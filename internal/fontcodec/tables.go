package fontcodec

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf16"

	pb "openformat/gen/go/openformat/v1"
)

var errTableTooShort = errors.New("fontcodec: table body too short")

// ---------------- head -------------------------------------------------------

func parseHead(b []byte) (*pb.HeadTable, error) {
	if len(b) < 54 {
		return nil, errTableTooShort
	}
	return &pb.HeadTable{
		MajorVersion:        uint32(binary.BigEndian.Uint16(b[0:2])),
		MinorVersion:        uint32(binary.BigEndian.Uint16(b[2:4])),
		FontRevisionFixed:   int64(int32(binary.BigEndian.Uint32(b[4:8]))),
		CheckSumAdjustment:  binary.BigEndian.Uint32(b[8:12]),
		MagicNumber:         binary.BigEndian.Uint32(b[12:16]),
		Flags:               uint32(binary.BigEndian.Uint16(b[16:18])),
		UnitsPerEm:          uint32(binary.BigEndian.Uint16(b[18:20])),
		Created:             int64(binary.BigEndian.Uint64(b[20:28])),
		Modified:            int64(binary.BigEndian.Uint64(b[28:36])),
		XMin:                int32(int16(binary.BigEndian.Uint16(b[36:38]))),
		YMin:                int32(int16(binary.BigEndian.Uint16(b[38:40]))),
		XMax:                int32(int16(binary.BigEndian.Uint16(b[40:42]))),
		YMax:                int32(int16(binary.BigEndian.Uint16(b[42:44]))),
		MacStyle:            uint32(binary.BigEndian.Uint16(b[44:46])),
		LowestRecPpem:       uint32(binary.BigEndian.Uint16(b[46:48])),
		FontDirectionHint:   int32(int16(binary.BigEndian.Uint16(b[48:50]))),
		IndexToLocFormat:    int32(int16(binary.BigEndian.Uint16(b[50:52]))),
		GlyphDataFormat:     int32(int16(binary.BigEndian.Uint16(b[52:54]))),
	}, nil
}

// ---------------- hhea -------------------------------------------------------

func parseHhea(b []byte) (*pb.HheaTable, error) {
	if len(b) < 36 {
		return nil, errTableTooShort
	}
	return &pb.HheaTable{
		MajorVersion:         uint32(binary.BigEndian.Uint16(b[0:2])),
		MinorVersion:         uint32(binary.BigEndian.Uint16(b[2:4])),
		Ascender:             int32(int16(binary.BigEndian.Uint16(b[4:6]))),
		Descender:            int32(int16(binary.BigEndian.Uint16(b[6:8]))),
		LineGap:              int32(int16(binary.BigEndian.Uint16(b[8:10]))),
		AdvanceWidthMax:      uint32(binary.BigEndian.Uint16(b[10:12])),
		MinLeftSideBearing:   int32(int16(binary.BigEndian.Uint16(b[12:14]))),
		MinRightSideBearing:  int32(int16(binary.BigEndian.Uint16(b[14:16]))),
		XMaxExtent:           int32(int16(binary.BigEndian.Uint16(b[16:18]))),
		CaretSlopeRise:       int32(int16(binary.BigEndian.Uint16(b[18:20]))),
		CaretSlopeRun:        int32(int16(binary.BigEndian.Uint16(b[20:22]))),
		CaretOffset:          int32(int16(binary.BigEndian.Uint16(b[22:24]))),
		// 4 reserved int16 at [24:32]
		MetricDataFormat:     int32(int16(binary.BigEndian.Uint16(b[32:34]))),
		NumberOfHMetrics:     uint32(binary.BigEndian.Uint16(b[34:36])),
	}, nil
}

// ---------------- maxp -------------------------------------------------------

func parseMaxp(b []byte) (*pb.MaxpTable, error) {
	if len(b) < 6 {
		return nil, errTableTooShort
	}
	version := binary.BigEndian.Uint32(b[0:4])
	m := &pb.MaxpTable{
		Version:   version,
		NumGlyphs: uint32(binary.BigEndian.Uint16(b[4:6])),
	}
	if version == 0x00010000 {
		if len(b) < 32 {
			return nil, errTableTooShort
		}
		m.MaxPoints = uint32(binary.BigEndian.Uint16(b[6:8]))
		m.MaxContours = uint32(binary.BigEndian.Uint16(b[8:10]))
		m.MaxCompositePoints = uint32(binary.BigEndian.Uint16(b[10:12]))
		m.MaxCompositeContours = uint32(binary.BigEndian.Uint16(b[12:14]))
		m.MaxZones = uint32(binary.BigEndian.Uint16(b[14:16]))
		m.MaxTwilightPoints = uint32(binary.BigEndian.Uint16(b[16:18]))
		m.MaxStorage = uint32(binary.BigEndian.Uint16(b[18:20]))
		m.MaxFunctionDefs = uint32(binary.BigEndian.Uint16(b[20:22]))
		m.MaxInstructionDefs = uint32(binary.BigEndian.Uint16(b[22:24]))
		m.MaxStackElements = uint32(binary.BigEndian.Uint16(b[24:26]))
		m.MaxSizeOfInstructions = uint32(binary.BigEndian.Uint16(b[26:28]))
		m.MaxComponentElements = uint32(binary.BigEndian.Uint16(b[28:30]))
		m.MaxComponentDepth = uint32(binary.BigEndian.Uint16(b[30:32]))
	}
	return m, nil
}

// ---------------- OS/2 -------------------------------------------------------

func parseOS2(b []byte) (*pb.OS2Table, error) {
	if len(b) < 78 {
		return nil, errTableTooShort
	}
	o := &pb.OS2Table{
		Version:              uint32(binary.BigEndian.Uint16(b[0:2])),
		XAvgCharWidth:        int32(int16(binary.BigEndian.Uint16(b[2:4]))),
		UsWeightClass:        uint32(binary.BigEndian.Uint16(b[4:6])),
		UsWidthClass:         uint32(binary.BigEndian.Uint16(b[6:8])),
		FsType:               uint32(binary.BigEndian.Uint16(b[8:10])),
		YSubscriptXSize:      int32(int16(binary.BigEndian.Uint16(b[10:12]))),
		YSubscriptYSize:      int32(int16(binary.BigEndian.Uint16(b[12:14]))),
		YSubscriptXOffset:    int32(int16(binary.BigEndian.Uint16(b[14:16]))),
		YSubscriptYOffset:    int32(int16(binary.BigEndian.Uint16(b[16:18]))),
		YSuperscriptXSize:    int32(int16(binary.BigEndian.Uint16(b[18:20]))),
		YSuperscriptYSize:    int32(int16(binary.BigEndian.Uint16(b[20:22]))),
		YSuperscriptXOffset:  int32(int16(binary.BigEndian.Uint16(b[22:24]))),
		YSuperscriptYOffset:  int32(int16(binary.BigEndian.Uint16(b[24:26]))),
		YStrikeoutSize:       int32(int16(binary.BigEndian.Uint16(b[26:28]))),
		YStrikeoutPosition:   int32(int16(binary.BigEndian.Uint16(b[28:30]))),
		SFamilyClass:         int32(int16(binary.BigEndian.Uint16(b[30:32]))),
		Panose:               append([]byte(nil), b[32:42]...),
		UlUnicodeRange:       append([]byte(nil), b[42:58]...),
		AchVendId:            string(b[58:62]),
		FsSelection:          uint32(binary.BigEndian.Uint16(b[62:64])),
		UsFirstCharIndex:     uint32(binary.BigEndian.Uint16(b[64:66])),
		UsLastCharIndex:      uint32(binary.BigEndian.Uint16(b[66:68])),
		STypoAscender:        int32(int16(binary.BigEndian.Uint16(b[68:70]))),
		STypoDescender:       int32(int16(binary.BigEndian.Uint16(b[70:72]))),
		STypoLineGap:         int32(int16(binary.BigEndian.Uint16(b[72:74]))),
		UsWinAscent:          uint32(binary.BigEndian.Uint16(b[74:76])),
		UsWinDescent:         uint32(binary.BigEndian.Uint16(b[76:78])),
	}
	trailingStart := 78
	if o.Version >= 1 && len(b) >= 86 {
		o.UlCodePageRange = append([]byte(nil), b[78:86]...)
		trailingStart = 86
	}
	if o.Version >= 2 && len(b) >= 96 {
		o.SxHeight = int32(int16(binary.BigEndian.Uint16(b[86:88])))
		o.SCapHeight = int32(int16(binary.BigEndian.Uint16(b[88:90])))
		o.UsDefaultChar = uint32(binary.BigEndian.Uint16(b[90:92]))
		o.UsBreakChar = uint32(binary.BigEndian.Uint16(b[92:94]))
		o.UsMaxContext = uint32(binary.BigEndian.Uint16(b[94:96]))
		trailingStart = 96
	}
	if o.Version >= 5 && len(b) >= 100 {
		o.UsLowerOpticalPointSize = uint32(binary.BigEndian.Uint16(b[96:98]))
		o.UsUpperOpticalPointSize = uint32(binary.BigEndian.Uint16(b[98:100]))
		trailingStart = 100
	}
	if trailingStart < len(b) {
		o.TrailingRaw = append([]byte(nil), b[trailingStart:]...)
	}
	return o, nil
}

// ---------------- post -------------------------------------------------------

func parsePost(b []byte) (*pb.PostTable, error) {
	if len(b) < 32 {
		return nil, errTableTooShort
	}
	p := &pb.PostTable{
		VersionFixed:       int64(int32(binary.BigEndian.Uint32(b[0:4]))),
		ItalicAngleFixed:   int64(int32(binary.BigEndian.Uint32(b[4:8]))),
		UnderlinePosition:  int32(int16(binary.BigEndian.Uint16(b[8:10]))),
		UnderlineThickness: int32(int16(binary.BigEndian.Uint16(b[10:12]))),
		IsFixedPitch:       binary.BigEndian.Uint32(b[12:16]),
		MinMemType_42:      binary.BigEndian.Uint32(b[16:20]),
		MaxMemType_42:      binary.BigEndian.Uint32(b[20:24]),
		MinMemType_1:       binary.BigEndian.Uint32(b[24:28]),
		MaxMemType_1:       binary.BigEndian.Uint32(b[28:32]),
	}
	if len(b) > 32 {
		p.TrailingRaw = append([]byte(nil), b[32:]...)
	}
	return p, nil
}

// ---------------- name -------------------------------------------------------

func parseName(b []byte) (*pb.NameTable, error) {
	if len(b) < 6 {
		return nil, errTableTooShort
	}
	n := &pb.NameTable{
		Version:       uint32(binary.BigEndian.Uint16(b[0:2])),
		Count:         uint32(binary.BigEndian.Uint16(b[2:4])),
		StorageOffset: uint32(binary.BigEndian.Uint16(b[4:6])),
	}
	if len(b) < 6+12*int(n.Count) {
		return nil, errTableTooShort
	}
	n.Records = make([]*pb.NameRecord, n.Count)
	for i := uint32(0); i < n.Count; i++ {
		base := 6 + 12*int(i)
		r := &pb.NameRecord{
			PlatformId:   uint32(binary.BigEndian.Uint16(b[base : base+2])),
			EncodingId:   uint32(binary.BigEndian.Uint16(b[base+2 : base+4])),
			LanguageId:   uint32(binary.BigEndian.Uint16(b[base+4 : base+6])),
			NameId:       uint32(binary.BigEndian.Uint16(b[base+6 : base+8])),
			Length:       uint32(binary.BigEndian.Uint16(b[base+8 : base+10])),
			StringOffset: uint32(binary.BigEndian.Uint16(b[base+10 : base+12])),
		}
		n.Records[i] = r
	}
	afterRecs := 6 + 12*int(n.Count)
	// Version 1 has a language tag record block between records and storage.
	if n.Version >= 1 && afterRecs+2 <= int(n.StorageOffset) {
		ltCount := int(binary.BigEndian.Uint16(b[afterRecs : afterRecs+2]))
		ltEnd := afterRecs + 2 + 4*ltCount
		if ltEnd <= int(n.StorageOffset) && ltEnd <= len(b) {
			n.V1LangTagData = append([]byte(nil), b[afterRecs:ltEnd]...)
		}
	}
	if int(n.StorageOffset) <= len(b) {
		n.StringStorage = append([]byte(nil), b[n.StorageOffset:]...)
	}
	for _, r := range n.Records {
		start := int(r.StringOffset)
		end := start + int(r.Length)
		if end <= len(n.StringStorage) {
			r.Decoded = decodeNameString(r.PlatformId, n.StringStorage[start:end])
		}
	}
	return n, nil
}

func decodeNameString(platformID uint32, raw []byte) string {
	// Platform 0 (Unicode) and Platform 3 (Microsoft) store UTF-16BE. Platform
	// 1 (Macintosh) encoding varies by encodingID — MacRoman is dominant but
	// we only best-effort decode. Caller treats this as advisory.
	switch platformID {
	case 0, 3:
		if len(raw)%2 != 0 {
			return ""
		}
		u := make([]uint16, len(raw)/2)
		for i := range u {
			u[i] = binary.BigEndian.Uint16(raw[2*i : 2*i+2])
		}
		return string(utf16.Decode(u))
	case 1:
		// MacRoman: bytes < 0x80 match ASCII; preserve as-is for readability.
		out := make([]rune, 0, len(raw))
		for _, c := range raw {
			if c < 0x80 {
				out = append(out, rune(c))
			} else {
				out = append(out, 0xFFFD)
			}
		}
		return string(out)
	}
	return ""
}

// ---------------- cmap (directory only) --------------------------------------

func parseCmapDirectory(b []byte) (*pb.CmapTable, error) {
	if len(b) < 4 {
		return nil, errTableTooShort
	}
	c := &pb.CmapTable{
		Version:            uint32(binary.BigEndian.Uint16(b[0:2])),
		NumEncodingRecords: uint32(binary.BigEndian.Uint16(b[2:4])),
	}
	if len(b) < 4+8*int(c.NumEncodingRecords) {
		return nil, errTableTooShort
	}
	c.SubtableBodies = map[uint32][]byte{}
	c.EncodingRecords = make([]*pb.CmapEncodingRecord, c.NumEncodingRecords)
	for i := uint32(0); i < c.NumEncodingRecords; i++ {
		base := 4 + 8*int(i)
		off := binary.BigEndian.Uint32(b[base+4 : base+8])
		rec := &pb.CmapEncodingRecord{
			PlatformId:     uint32(binary.BigEndian.Uint16(b[base : base+2])),
			EncodingId:     uint32(binary.BigEndian.Uint16(b[base+2 : base+4])),
			SubtableOffset: off,
		}
		if int(off)+2 <= len(b) {
			rec.SubtableFormat = uint32(binary.BigEndian.Uint16(b[off : off+2]))
		}
		c.EncodingRecords[i] = rec
		// Best-effort copy of the subtable body. The length is format-
		// specific; for round-trip we only need the bytes to reassemble the
		// cmap, which come from the enclosing SfntTable.RawData anyway.
		if _, ok := c.SubtableBodies[off]; !ok {
			end := len(b)
			if int(off) < end {
				c.SubtableBodies[off] = append([]byte(nil), b[off:end]...)
			}
		}
	}
	return c, nil
}

// ---------------- Checksum helper -------------------------------------------

// sfntTableChecksum implements the head-aware OpenType checksum (OpenType
// Table Directory: sum of uint32 big-endian words, zero-padded to 4 bytes).
func sfntTableChecksum(b []byte) uint32 {
	var sum uint32
	n := len(b) / 4
	for i := 0; i < n; i++ {
		sum += binary.BigEndian.Uint32(b[i*4 : i*4+4])
	}
	if rem := len(b) % 4; rem != 0 {
		var tail [4]byte
		copy(tail[:], b[n*4:])
		sum += binary.BigEndian.Uint32(tail[:])
	}
	return sum
}

// fmt is imported by callers in decode.go; this no-op reference keeps it
// tree-shaken from THIS file cleanly when compiled standalone.
var _ = fmt.Sprint
