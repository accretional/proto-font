package fontcodec

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"sort"

	pb "openformat/gen/go/openformat/v1"
)

// Decode parses a font container and returns the structured proto.
// FontFileWithMetadata.RawBytes is always populated with the verbatim input
// so Encode can round-trip byte-for-byte.
func Decode(raw []byte) (*pb.FontFileWithMetadata, error) {
	flavor, err := detectFlavor(raw)
	if err != nil {
		return nil, err
	}
	file := &pb.FontFile{Flavor: flavor}
	switch flavor {
	case pb.FontContainerFlavor_FONT_CONTAINER_TRUETYPE,
		pb.FontContainerFlavor_FONT_CONTAINER_OPENTYPE_CFF,
		pb.FontContainerFlavor_FONT_CONTAINER_TRUE_APPLE:
		sfnt, err := decodeSFNT(raw)
		if err != nil {
			return nil, err
		}
		file.Body = &pb.FontFile_Sfnt{Sfnt: sfnt}
	case pb.FontContainerFlavor_FONT_CONTAINER_WOFF1:
		w, err := decodeWOFF1(raw)
		if err != nil {
			return nil, err
		}
		file.Body = &pb.FontFile_Woff1{Woff1: w}
	case pb.FontContainerFlavor_FONT_CONTAINER_WOFF2:
		w, err := decodeWOFF2(raw)
		if err != nil {
			return nil, err
		}
		file.Body = &pb.FontFile_Woff2{Woff2: w}
	case pb.FontContainerFlavor_FONT_CONTAINER_COLLECTION:
		c, err := decodeTTC(raw)
		if err != nil {
			return nil, err
		}
		file.Body = &pb.FontFile_Collection{Collection: c}
	case pb.FontContainerFlavor_FONT_CONTAINER_EOT:
		e, err := decodeEOT(raw)
		if err != nil {
			return nil, err
		}
		file.Body = &pb.FontFile_Eot{Eot: e}
	default:
		return nil, fmt.Errorf("fontcodec: unsupported flavor %v", flavor)
	}
	sum := sha256.Sum256(raw)
	return &pb.FontFileWithMetadata{
		File: file,
		Provenance: &pb.FontProvenance{
			Sha256: hex.EncodeToString(sum[:]),
		},
		RawBytes: append([]byte(nil), raw...),
	}, nil
}

// --- SFNT ---------------------------------------------------------------------

func decodeSFNT(raw []byte) (*pb.SfntFont, error) {
	if len(raw) < 12 {
		return nil, errShortInput
	}
	s := &pb.SfntFont{
		SfntVersion:   binary.BigEndian.Uint32(raw[0:4]),
		NumTables:     uint32(binary.BigEndian.Uint16(raw[4:6])),
		SearchRange:   uint32(binary.BigEndian.Uint16(raw[6:8])),
		EntrySelector: uint32(binary.BigEndian.Uint16(raw[8:10])),
		RangeShift:    uint32(binary.BigEndian.Uint16(raw[10:12])),
	}
	n := int(s.NumTables)
	if len(raw) < 12+16*n {
		return nil, fmt.Errorf("fontcodec: truncated sfnt directory")
	}

	type dirEntry struct {
		tagRaw          uint32
		tag             string
		checksum        uint32
		offset          uint32
		length          uint32
	}
	entries := make([]dirEntry, n)
	for i := 0; i < n; i++ {
		base := 12 + 16*i
		entries[i] = dirEntry{
			tagRaw:   binary.BigEndian.Uint32(raw[base : base+4]),
			checksum: binary.BigEndian.Uint32(raw[base+4 : base+8]),
			offset:   binary.BigEndian.Uint32(raw[base+8 : base+12]),
			length:   binary.BigEndian.Uint32(raw[base+12 : base+16]),
		}
		entries[i].tag = tagString(entries[i].tagRaw)
	}

	// Directory order: as written. Table body order: sort by offset.
	s.TableDirectoryOrder = make([]string, n)
	for i, e := range entries {
		s.TableDirectoryOrder[i] = e.tag
	}
	byOffset := append([]dirEntry(nil), entries...)
	sort.SliceStable(byOffset, func(i, j int) bool {
		return byOffset[i].offset < byOffset[j].offset
	})
	s.TableBodyOrder = make([]string, n)
	for i, e := range byOffset {
		s.TableBodyOrder[i] = e.tag
	}

	// Build SfntTable list in directory order.
	byTag := map[string]*pb.SfntTable{}
	s.Tables = make([]*pb.SfntTable, n)
	for i, e := range entries {
		end := int(e.offset) + int(e.length)
		if end > len(raw) {
			return nil, fmt.Errorf("fontcodec: table %q extends past EOF", e.tag)
		}
		t := &pb.SfntTable{
			Tag:      e.tag,
			TagRaw:   e.tagRaw,
			Checksum: e.checksum,
			Offset:   e.offset,
			Length:   e.length,
			RawData:  append([]byte(nil), raw[e.offset:end]...),
		}
		attachStructuredTable(t)
		s.Tables[i] = t
		byTag[e.tag] = t
	}

	// Per-table post-padding: bytes between end-of-body and start of next
	// body in byOffset order, and any trailing bytes after the last table.
	tableEnd := func(e dirEntry) int { return int(e.offset) + int(e.length) }
	s.PostTablePadding = map[string][]byte{}
	for i, e := range byOffset {
		var nextStart int
		if i+1 < len(byOffset) {
			nextStart = int(byOffset[i+1].offset)
		} else {
			nextStart = len(raw)
		}
		// Malformed fonts can declare overlapping table extents; skip
		// rather than panicking on a negative-length slice.
		if tableEnd(e) > nextStart {
			continue
		}
		pad := raw[tableEnd(e):nextStart]
		if len(pad) > 0 && !allZero(pad) {
			s.PostTablePadding[e.tag] = append([]byte(nil), pad...)
		}
	}
	return s, nil
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

func attachStructuredTable(t *pb.SfntTable) {
	switch t.Tag {
	case "head":
		if h, err := parseHead(t.RawData); err == nil {
			t.Parsed = &pb.SfntTable_Head{Head: h}
		}
	case "hhea":
		if h, err := parseHhea(t.RawData); err == nil {
			t.Parsed = &pb.SfntTable_Hhea{Hhea: h}
		}
	case "maxp":
		if m, err := parseMaxp(t.RawData); err == nil {
			t.Parsed = &pb.SfntTable_Maxp{Maxp: m}
		}
	case "OS/2":
		if o, err := parseOS2(t.RawData); err == nil {
			t.Parsed = &pb.SfntTable_Os2{Os2: o}
		}
	case "post":
		if p, err := parsePost(t.RawData); err == nil {
			t.Parsed = &pb.SfntTable_Post{Post: p}
		}
	case "name":
		if n, err := parseName(t.RawData); err == nil {
			t.Parsed = &pb.SfntTable_Name{Name: n}
		}
	case "cmap":
		if c, err := parseCmapDirectory(t.RawData); err == nil {
			t.Parsed = &pb.SfntTable_Cmap{Cmap: c}
		}
	}
}

// --- WOFF 1.0 -----------------------------------------------------------------

func decodeWOFF1(raw []byte) (*pb.WoffFont, error) {
	if len(raw) < 44 {
		return nil, errShortInput
	}
	w := &pb.WoffFont{
		Signature:      binary.BigEndian.Uint32(raw[0:4]),
		Flavor:         binary.BigEndian.Uint32(raw[4:8]),
		Length:         binary.BigEndian.Uint32(raw[8:12]),
		NumTables:      uint32(binary.BigEndian.Uint16(raw[12:14])),
		Reserved:       uint32(binary.BigEndian.Uint16(raw[14:16])),
		TotalSfntSize:  binary.BigEndian.Uint32(raw[16:20]),
		MajorVersion:   uint32(binary.BigEndian.Uint16(raw[20:22])),
		MinorVersion:   uint32(binary.BigEndian.Uint16(raw[22:24])),
		MetaOffset:     binary.BigEndian.Uint32(raw[24:28]),
		MetaLength:     binary.BigEndian.Uint32(raw[28:32]),
		MetaOrigLength: binary.BigEndian.Uint32(raw[32:36]),
		PrivOffset:     binary.BigEndian.Uint32(raw[36:40]),
		PrivLength:     binary.BigEndian.Uint32(raw[40:44]),
	}
	n := int(w.NumTables)
	if len(raw) < 44+20*n {
		return nil, fmt.Errorf("fontcodec: truncated WOFF table directory")
	}
	w.Tables = make([]*pb.WoffTable, n)
	for i := 0; i < n; i++ {
		base := 44 + 20*i
		tagR := binary.BigEndian.Uint32(raw[base : base+4])
		offset := binary.BigEndian.Uint32(raw[base+4 : base+8])
		compLen := binary.BigEndian.Uint32(raw[base+8 : base+12])
		origLen := binary.BigEndian.Uint32(raw[base+12 : base+16])
		origChk := binary.BigEndian.Uint32(raw[base+16 : base+20])
		end := int(offset) + int(compLen)
		if end > len(raw) {
			return nil, fmt.Errorf("fontcodec: WOFF table %d extends past EOF", i)
		}
		w.Tables[i] = &pb.WoffTable{
			Tag:           tagString(tagR),
			TagRaw:        tagR,
			Offset:        offset,
			CompLength:    compLen,
			OrigLength:    origLen,
			OrigChecksum:  origChk,
			StoredData:    append([]byte(nil), raw[offset:end]...),
			WasCompressed: compLen != origLen,
		}
	}
	if w.MetaOffset != 0 && w.MetaLength != 0 {
		end := int(w.MetaOffset) + int(w.MetaLength)
		if end <= len(raw) {
			w.MetadataCompressed = append([]byte(nil), raw[w.MetaOffset:end]...)
			if decoded, err := zlibDecode(w.MetadataCompressed); err == nil {
				w.MetadataXml = string(decoded)
			}
		}
	}
	if w.PrivOffset != 0 && w.PrivLength != 0 {
		end := int(w.PrivOffset) + int(w.PrivLength)
		if end <= len(raw) {
			w.PrivateData = append([]byte(nil), raw[w.PrivOffset:end]...)
		}
	}
	return w, nil
}

func zlibDecode(b []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// --- WOFF 2.0 ----------------------------------------------------------------

func decodeWOFF2(raw []byte) (*pb.Woff2Font, error) {
	if len(raw) < 48 {
		return nil, errShortInput
	}
	w := &pb.Woff2Font{
		Signature:           binary.BigEndian.Uint32(raw[0:4]),
		Flavor:              binary.BigEndian.Uint32(raw[4:8]),
		Length:              binary.BigEndian.Uint32(raw[8:12]),
		NumTables:           uint32(binary.BigEndian.Uint16(raw[12:14])),
		Reserved:            uint32(binary.BigEndian.Uint16(raw[14:16])),
		TotalSfntSize:       binary.BigEndian.Uint32(raw[16:20]),
		TotalCompressedSize: binary.BigEndian.Uint32(raw[20:24]),
		MajorVersion:        uint32(binary.BigEndian.Uint16(raw[24:26])),
		MinorVersion:        uint32(binary.BigEndian.Uint16(raw[26:28])),
		MetaOffset:          binary.BigEndian.Uint32(raw[28:32]),
		MetaLength:          binary.BigEndian.Uint32(raw[32:36]),
		MetaOrigLength:      binary.BigEndian.Uint32(raw[36:40]),
		PrivOffset:          binary.BigEndian.Uint32(raw[40:44]),
		PrivLength:          binary.BigEndian.Uint32(raw[44:48]),
	}

	// Walk the variable-length table directory immediately after the
	// header. Each entry is 1–9 bytes (1 flag + optional 4-byte tag +
	// 1–3 byte 255UInt16 origLength + 1–3 byte 255UInt16 transformLength
	// for transformed tables).
	dir, dirLen, err := parseWoff2Directory(raw[48:], w.NumTables)
	if err != nil {
		return nil, err
	}
	w.TableDirectory = dir
	streamStart := 48 + dirLen

	// CollectionDirectory follows when flavor == 'ttcf'. We don't ship a
	// WOFF2-collection fixture yet; bail explicitly so the gap is loud.
	if w.Flavor == magicTTC {
		return nil, fmt.Errorf("fontcodec: WOFF2 collections not yet supported")
	}

	// Compressed stream is exactly totalCompressedSize bytes when set,
	// otherwise it runs to the start of the metadata or private blocks.
	streamEnd := len(raw)
	if w.TotalCompressedSize != 0 && streamStart+int(w.TotalCompressedSize) <= len(raw) {
		streamEnd = streamStart + int(w.TotalCompressedSize)
	} else if w.MetaOffset != 0 && int(w.MetaOffset) <= len(raw) {
		streamEnd = int(w.MetaOffset)
	} else if w.PrivOffset != 0 && int(w.PrivOffset) <= len(raw) {
		streamEnd = int(w.PrivOffset)
	}
	if streamStart > streamEnd {
		return nil, fmt.Errorf("fontcodec: WOFF2 directory overruns compressed stream")
	}
	w.CompressedStream = append([]byte(nil), raw[streamStart:streamEnd]...)

	decompressed, err := brotliDecode(w.CompressedStream)
	if err != nil {
		return nil, fmt.Errorf("fontcodec: WOFF2 brotli decode: %w", err)
	}
	if err := sliceWoff2TableData(decompressed, w.TableDirectory); err != nil {
		return nil, err
	}

	// Reverse the glyf transform (spec §5.1) so callers can see standard
	// SFNT glyf + loca bytes without a second decoder. The transformed
	// bytes are preserved in `data` for round-trip fidelity; the
	// reconstructed bytes land in `untransformed_data` on both entries.
	var glyfEntry, locaEntry *pb.Woff2TableDirectoryEntry
	for _, e := range w.TableDirectory {
		switch e.TagStr {
		case "glyf":
			glyfEntry = e
		case "loca":
			locaEntry = e
		}
	}
	if glyfEntry != nil && glyfEntry.Transformed {
		glyf, loca, _, err := reverseWoff2GlyfTransform(glyfEntry.Data)
		if err != nil {
			return nil, fmt.Errorf("fontcodec: WOFF2 glyf transform reversal: %w", err)
		}
		glyfEntry.UntransformedData = glyf
		if locaEntry != nil {
			locaEntry.UntransformedData = loca
		}
	}

	if w.MetaOffset != 0 && w.MetaLength != 0 {
		end := int(w.MetaOffset) + int(w.MetaLength)
		if end <= len(raw) {
			w.MetadataCompressed = append([]byte(nil), raw[w.MetaOffset:end]...)
		}
	}
	if w.PrivOffset != 0 && w.PrivLength != 0 {
		end := int(w.PrivOffset) + int(w.PrivLength)
		if end <= len(raw) {
			w.PrivateData = append([]byte(nil), raw[w.PrivOffset:end]...)
		}
	}
	return w, nil
}

// --- TTC ----------------------------------------------------------------------

func decodeTTC(raw []byte) (*pb.FontCollection, error) {
	if len(raw) < 12 {
		return nil, errShortInput
	}
	c := &pb.FontCollection{
		TtcTag:       binary.BigEndian.Uint32(raw[0:4]),
		MajorVersion: uint32(binary.BigEndian.Uint16(raw[4:6])),
		MinorVersion: uint32(binary.BigEndian.Uint16(raw[6:8])),
	}
	numFonts := binary.BigEndian.Uint32(raw[8:12])
	if len(raw) < 12+4*int(numFonts) {
		return nil, fmt.Errorf("fontcodec: truncated TTC directory")
	}
	c.OffsetTableOffsets = make([]uint32, numFonts)
	c.Fonts = make([]*pb.SfntFont, numFonts)
	for i := 0; i < int(numFonts); i++ {
		off := binary.BigEndian.Uint32(raw[12+4*i : 16+4*i])
		c.OffsetTableOffsets[i] = off
		// Each nested SFNT shares table bodies with the TTC; we can still
		// decode the directory by parsing from `off`.
		s, err := decodeSFNTAtOffset(raw, int(off))
		if err != nil {
			return nil, fmt.Errorf("fontcodec: TTC font %d: %w", i, err)
		}
		c.Fonts[i] = s
	}
	// v2 signature fields live right after the offsets.
	if c.MajorVersion >= 2 {
		base := 12 + 4*int(numFonts)
		if len(raw) >= base+12 {
			c.DsigTag = binary.BigEndian.Uint32(raw[base : base+4])
			c.DsigLength = binary.BigEndian.Uint32(raw[base+4 : base+8])
			c.DsigOffset = binary.BigEndian.Uint32(raw[base+8 : base+12])
			if c.DsigOffset != 0 && c.DsigLength != 0 {
				end := int(c.DsigOffset) + int(c.DsigLength)
				if end <= len(raw) {
					c.DsigData = append([]byte(nil), raw[c.DsigOffset:end]...)
				}
			}
		}
	}
	return c, nil
}

func decodeSFNTAtOffset(raw []byte, off int) (*pb.SfntFont, error) {
	if off+12 > len(raw) {
		return nil, errShortInput
	}
	// Treat the slice starting at off as a standalone SFNT. Table offsets in
	// the directory are absolute to the file start (not to `off`), so we
	// re-resolve them against the original `raw` buffer.
	h := raw[off:]
	s := &pb.SfntFont{
		SfntVersion:   binary.BigEndian.Uint32(h[0:4]),
		NumTables:     uint32(binary.BigEndian.Uint16(h[4:6])),
		SearchRange:   uint32(binary.BigEndian.Uint16(h[6:8])),
		EntrySelector: uint32(binary.BigEndian.Uint16(h[8:10])),
		RangeShift:    uint32(binary.BigEndian.Uint16(h[10:12])),
	}
	n := int(s.NumTables)
	if off+12+16*n > len(raw) {
		return nil, fmt.Errorf("fontcodec: truncated sfnt directory in TTC")
	}
	s.Tables = make([]*pb.SfntTable, n)
	s.TableDirectoryOrder = make([]string, n)
	for i := 0; i < n; i++ {
		base := off + 12 + 16*i
		tagR := binary.BigEndian.Uint32(raw[base : base+4])
		tag := tagString(tagR)
		checksum := binary.BigEndian.Uint32(raw[base+4 : base+8])
		offset := binary.BigEndian.Uint32(raw[base+8 : base+12])
		length := binary.BigEndian.Uint32(raw[base+12 : base+16])
		end := int(offset) + int(length)
		if end > len(raw) {
			return nil, fmt.Errorf("fontcodec: TTC table %q extends past EOF", tag)
		}
		t := &pb.SfntTable{
			Tag:      tag,
			TagRaw:   tagR,
			Checksum: checksum,
			Offset:   offset,
			Length:   length,
			RawData:  append([]byte(nil), raw[offset:end]...),
		}
		attachStructuredTable(t)
		s.Tables[i] = t
		s.TableDirectoryOrder[i] = tag
	}
	return s, nil
}

// --- EOT ----------------------------------------------------------------------

func decodeEOT(raw []byte) (*pb.Eot, error) {
	if len(raw) < 36 {
		return nil, errShortInput
	}
	e := &pb.Eot{
		EotSize:      binary.LittleEndian.Uint32(raw[0:4]),
		FontDataSize: binary.LittleEndian.Uint32(raw[4:8]),
		Version:      binary.LittleEndian.Uint32(raw[8:12]),
		Flags:        binary.LittleEndian.Uint32(raw[12:16]),
		FontPanose:   append([]byte(nil), raw[16:26]...),
		Charset:      uint32(raw[26]),
		Italic:       uint32(raw[27]),
		Weight:       binary.LittleEndian.Uint32(raw[28:32]),
		FsType:       uint32(binary.LittleEndian.Uint16(raw[32:34])),
		MagicNumber:  uint32(binary.LittleEndian.Uint16(raw[34:36])),
	}
	// The remainder up to (EotSize - FontDataSize) is variable-length fields.
	headerEnd := int(e.EotSize) - int(e.FontDataSize)
	if headerEnd < 0 || headerEnd > len(raw) {
		headerEnd = len(raw)
	}
	if headerEnd > 36 {
		e.TrailingHeader = append([]byte(nil), raw[36:headerEnd]...)
	}
	if headerEnd < len(raw) {
		e.FontData = append([]byte(nil), raw[headerEnd:]...)
	}
	return e, nil
}
