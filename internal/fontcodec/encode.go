package fontcodec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"github.com/andybalholm/brotli"

	pb "openformat/gen/go/openformat/v1"
)

// Encode serialises a FontFileWithMetadata back to container bytes.
//
// When `file.RawBytes` is non-empty it is returned verbatim — this is the
// normal round-trip path after Decode and guarantees byte-exact output
// without depending on every table serializer being perfect.
//
// When `file.RawBytes` is empty (freshly constructed proto) Encode rebuilds
// the container from the structured fields. For SFNT this means writing an
// offset table, a directory, and the per-table raw bodies. Only the
// structural round-trip is supported in the synthesis path; structured
// per-table fields (HeadTable, OS2Table, etc.) are advisory and are NOT
// re-serialised — set SfntTable.RawData to the intended bytes.
func Encode(m *pb.FontFileWithMetadata) ([]byte, error) {
	if m == nil {
		return nil, errors.New("fontcodec: nil FontFileWithMetadata")
	}
	if len(m.RawBytes) > 0 {
		return append([]byte(nil), m.RawBytes...), nil
	}
	if m.File == nil {
		return nil, errors.New("fontcodec: FontFile missing and no RawBytes")
	}
	switch body := m.File.Body.(type) {
	case *pb.FontFile_Sfnt:
		return encodeSFNT(body.Sfnt)
	case *pb.FontFile_Woff1:
		return encodeWOFF1(body.Woff1)
	case *pb.FontFile_Woff2:
		return encodeWOFF2(body.Woff2)
	case *pb.FontFile_Collection:
		return encodeTTC(body.Collection)
	case *pb.FontFile_Eot:
		return encodeEOT(body.Eot)
	default:
		return nil, fmt.Errorf("fontcodec: cannot synthesise flavor %v without RawBytes", m.File.Flavor)
	}
}

// --- SFNT synthesis ----------------------------------------------------------

func encodeSFNT(s *pb.SfntFont) ([]byte, error) {
	if s == nil {
		return nil, errors.New("fontcodec: nil SfntFont")
	}
	n := len(s.Tables)

	// Directory order: use TableDirectoryOrder when present, else tag-sorted.
	dirOrder := make([]*pb.SfntTable, 0, n)
	byTag := map[string]*pb.SfntTable{}
	for _, t := range s.Tables {
		byTag[t.Tag] = t
	}
	if len(s.TableDirectoryOrder) == len(s.Tables) {
		for _, tag := range s.TableDirectoryOrder {
			if t, ok := byTag[tag]; ok {
				dirOrder = append(dirOrder, t)
			}
		}
	}
	if len(dirOrder) != n {
		dirOrder = append(dirOrder[:0], s.Tables...)
		sort.SliceStable(dirOrder, func(i, j int) bool {
			return dirOrder[i].Tag < dirOrder[j].Tag
		})
	}

	// Body order: use TableBodyOrder when present, else same as directory.
	bodyOrder := make([]*pb.SfntTable, 0, n)
	if len(s.TableBodyOrder) == len(s.Tables) {
		for _, tag := range s.TableBodyOrder {
			if t, ok := byTag[tag]; ok {
				bodyOrder = append(bodyOrder, t)
			}
		}
	}
	if len(bodyOrder) != n {
		bodyOrder = append([]*pb.SfntTable(nil), dirOrder...)
	}

	headerSize := 12 + 16*n
	// Assign body offsets.
	offsets := map[string]uint32{}
	lengths := map[string]uint32{}
	pos := headerSize
	out := make([]byte, headerSize)

	// First pass: decide offsets. Each body is 4-byte aligned. Recorded
	// `PostTablePadding` overrides the implicit zero-padding, otherwise we
	// zero-pad to the next 4-byte boundary so both the next body *and* EOF
	// land aligned per OpenType §5.
	for _, t := range bodyOrder {
		padTo4(&pos)
		offsets[t.Tag] = uint32(pos)
		lengths[t.Tag] = uint32(len(t.RawData))
		pos += len(t.RawData)
		if pad, ok := s.PostTablePadding[t.Tag]; ok {
			pos += len(pad)
		} else {
			padTo4(&pos)
		}
	}
	out = append(out, make([]byte, pos-headerSize)...)

	// Fill body bytes (zero-padded gaps stay zero).
	for _, t := range bodyOrder {
		o := int(offsets[t.Tag])
		copy(out[o:], t.RawData)
		if pad, ok := s.PostTablePadding[t.Tag]; ok {
			copy(out[o+len(t.RawData):], pad)
		}
	}

	// Offset table header.
	ver := s.SfntVersion
	if ver == 0 {
		ver = magicTrueType
	}
	binary.BigEndian.PutUint32(out[0:4], ver)
	binary.BigEndian.PutUint16(out[4:6], uint16(n))
	searchRange, entrySelector, rangeShift := searchRangeFor(uint16(n))
	if s.SearchRange != 0 {
		searchRange = uint16(s.SearchRange)
	}
	if s.EntrySelector != 0 {
		entrySelector = uint16(s.EntrySelector)
	}
	if s.RangeShift != 0 {
		rangeShift = uint16(s.RangeShift)
	}
	binary.BigEndian.PutUint16(out[6:8], searchRange)
	binary.BigEndian.PutUint16(out[8:10], entrySelector)
	binary.BigEndian.PutUint16(out[10:12], rangeShift)

	// Directory entries. For the head table the directory checksum must
	// be computed with checkSumAdjustment zeroed (OpenType §5.head).
	for i, t := range dirOrder {
		base := 12 + 16*i
		binary.BigEndian.PutUint32(out[base:base+4], tagRaw(t.Tag))
		checksum := t.Checksum
		if checksum == 0 && len(t.RawData) > 0 {
			if t.Tag == "head" && len(t.RawData) >= 12 {
				scratch := append([]byte(nil), t.RawData...)
				binary.BigEndian.PutUint32(scratch[8:12], 0)
				checksum = sfntTableChecksum(scratch)
			} else {
				checksum = sfntTableChecksum(t.RawData)
			}
		}
		binary.BigEndian.PutUint32(out[base+4:base+8], checksum)
		binary.BigEndian.PutUint32(out[base+8:base+12], offsets[t.Tag])
		binary.BigEndian.PutUint32(out[base+12:base+16], lengths[t.Tag])
	}

	// Whole-file checkSumAdjustment (OpenType §5.head): zero the field,
	// sum every uint32 word across the entire file, then write
	// 0xB1B0AFBA − sum. We always recompute in the synthesis path so that
	// any caller-side edits stay consistent; the raw_bytes short-circuit
	// in Encode() preserves bit-exact output for decoded files.
	if headOff, ok := offsets["head"]; ok && int(headOff)+12 <= len(out) {
		binary.BigEndian.PutUint32(out[headOff+8:headOff+12], 0)
		sum := sfntTableChecksum(out)
		adj := uint32(0xB1B0AFBA) - sum
		binary.BigEndian.PutUint32(out[headOff+8:headOff+12], adj)
	}

	return out, nil
}

func padTo4(p *int) {
	if r := *p % 4; r != 0 {
		*p += 4 - r
	}
}

func searchRangeFor(num uint16) (uint16, uint16, uint16) {
	if num == 0 {
		return 0, 0, 0
	}
	pow2 := uint16(1)
	log2 := uint16(0)
	for pow2*2 <= num {
		pow2 *= 2
		log2++
	}
	searchRange := pow2 * 16
	rangeShift := num*16 - searchRange
	return searchRange, log2, rangeShift
}

// --- WOFF 1.0 synthesis ------------------------------------------------------

func encodeWOFF1(w *pb.WoffFont) ([]byte, error) {
	if w == nil {
		return nil, errors.New("fontcodec: nil WoffFont")
	}
	headerSize := 44
	dirSize := 20 * len(w.Tables)
	out := make([]byte, headerSize+dirSize)
	// Fill table bodies + metadata + priv sequentially after directory.
	pos := headerSize + dirSize
	offsets := make([]uint32, len(w.Tables))
	for i, t := range w.Tables {
		offsets[i] = uint32(pos)
		out = append(out, t.StoredData...)
		pos += len(t.StoredData)
		// 4-byte align after each compressed block per spec.
		for pos%4 != 0 {
			out = append(out, 0)
			pos++
		}
	}
	var metaOffset, privOffset uint32
	if len(w.MetadataCompressed) > 0 {
		metaOffset = uint32(pos)
		out = append(out, w.MetadataCompressed...)
		pos += len(w.MetadataCompressed)
		for pos%4 != 0 {
			out = append(out, 0)
			pos++
		}
	}
	if len(w.PrivateData) > 0 {
		privOffset = uint32(pos)
		out = append(out, w.PrivateData...)
		pos += len(w.PrivateData)
	}

	// Header.
	sig := w.Signature
	if sig == 0 {
		sig = magicWOFF1
	}
	binary.BigEndian.PutUint32(out[0:4], sig)
	binary.BigEndian.PutUint32(out[4:8], w.Flavor)
	length := w.Length
	if length == 0 {
		length = uint32(pos)
	}
	binary.BigEndian.PutUint32(out[8:12], length)
	binary.BigEndian.PutUint16(out[12:14], uint16(len(w.Tables)))
	binary.BigEndian.PutUint16(out[14:16], uint16(w.Reserved))
	binary.BigEndian.PutUint32(out[16:20], w.TotalSfntSize)
	binary.BigEndian.PutUint16(out[20:22], uint16(w.MajorVersion))
	binary.BigEndian.PutUint16(out[22:24], uint16(w.MinorVersion))
	mo := w.MetaOffset
	if mo == 0 {
		mo = metaOffset
	}
	binary.BigEndian.PutUint32(out[24:28], mo)
	binary.BigEndian.PutUint32(out[28:32], w.MetaLength)
	binary.BigEndian.PutUint32(out[32:36], w.MetaOrigLength)
	po := w.PrivOffset
	if po == 0 {
		po = privOffset
	}
	binary.BigEndian.PutUint32(out[36:40], po)
	binary.BigEndian.PutUint32(out[40:44], w.PrivLength)

	// Directory entries.
	for i, t := range w.Tables {
		base := headerSize + 20*i
		binary.BigEndian.PutUint32(out[base:base+4], tagRaw(t.Tag))
		o := t.Offset
		if o == 0 {
			o = offsets[i]
		}
		binary.BigEndian.PutUint32(out[base+4:base+8], o)
		cl := t.CompLength
		if cl == 0 {
			cl = uint32(len(t.StoredData))
		}
		binary.BigEndian.PutUint32(out[base+8:base+12], cl)
		binary.BigEndian.PutUint32(out[base+12:base+16], t.OrigLength)
		binary.BigEndian.PutUint32(out[base+16:base+20], t.OrigChecksum)
	}
	return out, nil
}

// --- WOFF 2.0 synthesis ------------------------------------------------------

// encodeWOFF2 rebuilds a WOFF2 container from the structured proto. Each
// directory entry must carry either `Data` (wire-form bytes — what the
// decoder produced) or `UntransformedData` for glyf/loca (in which case
// we synthesise the transform on-the-fly). All other tables are stored
// verbatim.
//
// Byte-for-byte parity with the original file is NOT a goal: brotli is
// non-deterministic across encoders, so we aim for structural
// round-trip — decode(encode(p)) must match p.
func encodeWOFF2(w *pb.Woff2Font) ([]byte, error) {
	if w == nil {
		return nil, errors.New("fontcodec: nil Woff2Font")
	}
	if w.Flavor == magicTTC {
		return nil, errors.New("fontcodec: WOFF2 collection synthesis not supported yet")
	}

	// Per-entry wire-form bytes: use Data if populated; otherwise
	// synthesise glyf from untransformed_data. loca in WOFF2 is stored
	// empty when transformed (its bytes live in the glyf transform), so
	// Data for loca is expected to be zero-length.
	type prepared struct {
		entry *pb.Woff2TableDirectoryEntry
		data  []byte
	}
	prep := make([]prepared, len(w.TableDirectory))
	var glyfSynth, locaSynth []byte
	for i, e := range w.TableDirectory {
		prep[i] = prepared{entry: e, data: e.Data}
	}
	// When glyf is transformed but Data is empty and UntransformedData is
	// present, synthesise the transform from SFNT glyf + loca.
	glyfIdx, locaIdx := -1, -1
	for i, e := range w.TableDirectory {
		switch e.TagStr {
		case "glyf":
			glyfIdx = i
		case "loca":
			locaIdx = i
		}
	}
	if glyfIdx >= 0 && prep[glyfIdx].entry.Transformed && len(prep[glyfIdx].data) == 0 {
		g := prep[glyfIdx].entry
		if len(g.UntransformedData) == 0 {
			return nil, errors.New("fontcodec: WOFF2 glyf entry missing both Data and UntransformedData")
		}
		if locaIdx < 0 {
			return nil, errors.New("fontcodec: WOFF2 glyf transform requires a loca entry")
		}
		loca := prep[locaIdx].entry.UntransformedData
		// numGlyphs and indexFormat come from head/maxp; we rediscover
		// indexFormat from loca length vs numGlyphs. Callers must set
		// UntransformedData on loca consistently.
		numGlyphs, indexFormat, err := deriveGlyphCount(w.TableDirectory, loca)
		if err != nil {
			return nil, fmt.Errorf("fontcodec: WOFF2 glyf synth: %w", err)
		}
		xformed, err := synthesizeWoff2Glyf(g.UntransformedData, loca, numGlyphs, indexFormat)
		if err != nil {
			return nil, fmt.Errorf("fontcodec: WOFF2 glyf synth: %w", err)
		}
		glyfSynth = xformed
		prep[glyfIdx].data = glyfSynth
		locaSynth = nil // WOFF2 stores transformed loca as zero-length
		prep[locaIdx].data = locaSynth
	}

	// Directory: flags + optional 4-byte explicit tag + UIntBase128
	// origLength + optional UIntBase128 transformLength.
	var dir bytes.Buffer
	for _, p := range prep {
		e := p.entry
		flags := byte(e.Flags)
		tagIdx := flags & 0x3f
		transformVer := flags >> 6
		dir.WriteByte(flags)
		if tagIdx == 63 {
			var tag [4]byte
			binary.BigEndian.PutUint32(tag[:], e.Tag)
			dir.Write(tag[:])
		}
		origLen := e.OrigLength
		if origLen == 0 {
			// When synthesising we need a correct origLength; prefer
			// UntransformedData length, falling back to Data length for
			// non-transformed tables.
			if len(e.UntransformedData) > 0 {
				origLen = uint32(len(e.UntransformedData))
			} else {
				origLen = uint32(len(p.data))
			}
		}
		dir.Write(writeUIntBase128(origLen))
		if hasWoff2Transform(e.TagStr, transformVer) {
			tlen := e.TransformLength
			if tlen == 0 {
				tlen = uint32(len(p.data))
			}
			dir.Write(writeUIntBase128(tlen))
		}
	}

	// Concat per-entry data and brotli-compress.
	var bodies bytes.Buffer
	for _, p := range prep {
		bodies.Write(p.data)
	}
	var compressed bytes.Buffer
	bw := brotli.NewWriterLevel(&compressed, brotli.BestCompression)
	if _, err := bw.Write(bodies.Bytes()); err != nil {
		return nil, fmt.Errorf("fontcodec: brotli write: %w", err)
	}
	if err := bw.Close(); err != nil {
		return nil, fmt.Errorf("fontcodec: brotli close: %w", err)
	}

	// Layout: header (48) + directory + compressed stream + metadata + private.
	headerSize := 48
	dirBytes := dir.Bytes()
	compressedBytes := compressed.Bytes()
	// Pad compressed stream to 4-byte boundary before metadata / private.
	tail := headerSize + len(dirBytes) + len(compressedBytes)
	padLen := 0
	if r := tail % 4; r != 0 && (len(w.MetadataCompressed) > 0 || len(w.PrivateData) > 0) {
		padLen = 4 - r
	}
	metaOffset := uint32(0)
	privOffset := uint32(0)
	if len(w.MetadataCompressed) > 0 {
		metaOffset = uint32(tail + padLen)
	}
	if len(w.PrivateData) > 0 {
		off := tail + padLen + len(w.MetadataCompressed)
		// Metadata block is also padded to 4-byte boundary before private.
		if r := off % 4; r != 0 {
			off += 4 - r
		}
		privOffset = uint32(off)
	}

	total := tail + padLen + len(w.MetadataCompressed)
	if privOffset > 0 {
		total = int(privOffset) + len(w.PrivateData)
	}

	out := make([]byte, headerSize, total)
	sig := w.Signature
	if sig == 0 {
		sig = magicWOFF2
	}
	binary.BigEndian.PutUint32(out[0:4], sig)
	binary.BigEndian.PutUint32(out[4:8], w.Flavor)
	length := w.Length
	if length == 0 {
		length = uint32(total)
	} else {
		length = uint32(total) // always use computed length for synthesis
	}
	binary.BigEndian.PutUint32(out[8:12], length)
	binary.BigEndian.PutUint16(out[12:14], uint16(len(w.TableDirectory)))
	binary.BigEndian.PutUint16(out[14:16], uint16(w.Reserved))
	// totalSfntSize is the reconstructed SFNT size — sum of orig lengths
	// plus 12-byte offset table + 16 bytes per directory entry plus
	// 4-byte alignment per table. For round-trip we trust the decoded
	// value when present.
	total_sfnt := w.TotalSfntSize
	if total_sfnt == 0 {
		total_sfnt = computeTotalSfntSize(w.TableDirectory)
	}
	binary.BigEndian.PutUint32(out[16:20], total_sfnt)
	binary.BigEndian.PutUint32(out[20:24], uint32(len(compressedBytes)))
	binary.BigEndian.PutUint16(out[24:26], uint16(w.MajorVersion))
	binary.BigEndian.PutUint16(out[26:28], uint16(w.MinorVersion))
	binary.BigEndian.PutUint32(out[28:32], metaOffset)
	metaLen := uint32(len(w.MetadataCompressed))
	binary.BigEndian.PutUint32(out[32:36], metaLen)
	binary.BigEndian.PutUint32(out[36:40], w.MetaOrigLength)
	binary.BigEndian.PutUint32(out[40:44], privOffset)
	binary.BigEndian.PutUint32(out[44:48], uint32(len(w.PrivateData)))

	out = append(out, dirBytes...)
	out = append(out, compressedBytes...)
	if padLen > 0 {
		out = append(out, make([]byte, padLen)...)
	}
	if len(w.MetadataCompressed) > 0 {
		out = append(out, w.MetadataCompressed...)
		if len(w.PrivateData) > 0 {
			for len(out)%4 != 0 {
				out = append(out, 0)
			}
		}
	}
	if len(w.PrivateData) > 0 {
		out = append(out, w.PrivateData...)
	}
	return out, nil
}

// computeTotalSfntSize estimates the reconstructed SFNT byte length from
// the directory (offset table + directory + per-table origLength padded
// to 4 bytes).
func computeTotalSfntSize(dir []*pb.Woff2TableDirectoryEntry) uint32 {
	size := uint32(12 + 16*len(dir))
	for _, e := range dir {
		size += e.OrigLength
		if pad := size % 4; pad != 0 {
			size += 4 - pad
		}
	}
	return size
}

// deriveGlyphCount recovers numGlyphs + indexFormat from the maxp and
// head entries. Callers pre-load UntransformedData on glyf+loca but
// not necessarily on head/maxp, so we read from Data for those tables.
func deriveGlyphCount(dir []*pb.Woff2TableDirectoryEntry, loca []byte) (uint16, uint16, error) {
	var maxp, head []byte
	for _, e := range dir {
		switch e.TagStr {
		case "maxp":
			maxp = e.Data
		case "head":
			head = e.Data
		}
	}
	if len(maxp) < 6 {
		return 0, 0, errors.New("maxp missing or short")
	}
	if len(head) < 54 {
		return 0, 0, errors.New("head missing or short")
	}
	numGlyphs := binary.BigEndian.Uint16(maxp[4:6])
	indexFormat := binary.BigEndian.Uint16(head[50:52])
	// Sanity: loca length must match.
	expected := int(numGlyphs+1) * 2
	if indexFormat == 1 {
		expected = int(numGlyphs+1) * 4
	}
	if len(loca) != expected {
		return 0, 0, fmt.Errorf("loca length %d doesn't match numGlyphs=%d indexFormat=%d (expected %d)",
			len(loca), numGlyphs, indexFormat, expected)
	}
	return numGlyphs, indexFormat, nil
}

// --- TTC synthesis -----------------------------------------------------------

func encodeTTC(c *pb.FontCollection) ([]byte, error) {
	// Synthesising a TTC from nested SfntFont structs requires re-laying out
	// shared table bodies; we only support round-trip via RawBytes today.
	return nil, errors.New("fontcodec: TTC synthesis without RawBytes is not supported yet")
}

// --- EOT synthesis -----------------------------------------------------------

func encodeEOT(e *pb.Eot) ([]byte, error) {
	return nil, errors.New("fontcodec: EOT synthesis without RawBytes is not supported yet")
}
