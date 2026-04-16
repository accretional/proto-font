package fontcodec

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

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

	// Directory entries.
	for i, t := range dirOrder {
		base := 12 + 16*i
		binary.BigEndian.PutUint32(out[base:base+4], tagRaw(t.Tag))
		checksum := t.Checksum
		if checksum == 0 && len(t.RawData) > 0 {
			checksum = sfntTableChecksum(t.RawData)
		}
		binary.BigEndian.PutUint32(out[base+4:base+8], checksum)
		binary.BigEndian.PutUint32(out[base+8:base+12], offsets[t.Tag])
		binary.BigEndian.PutUint32(out[base+12:base+16], lengths[t.Tag])
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

// --- WOFF 2.0 synthesis (placeholder) ----------------------------------------

func encodeWOFF2(w *pb.Woff2Font) ([]byte, error) {
	return nil, errors.New("fontcodec: WOFF2 synthesis without RawBytes is not supported yet")
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
