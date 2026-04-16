package fontcodec

import (
	"encoding/binary"
	"fmt"

	pb "openformat/gen/go/openformat/v1"
)

// populateCmapSubtable reads the subtable body at `b` (first two bytes
// are the format uint16) and, for recognised formats, sets the matching
// oneof wrapper on `rec.ParsedSubtable`. Unrecognised formats leave the
// field unset so callers can fall back to the opaque subtable_bodies
// map. Done as a direct field assignment because the oneof interface
// (isCmapEncodingRecord_ParsedSubtable) is unexported in the generated
// Go and can't be returned to an outside package.
func populateCmapSubtable(rec *pb.CmapEncodingRecord, b []byte) error {
	if len(b) < 2 {
		return errTableTooShort
	}
	format := binary.BigEndian.Uint16(b[0:2])
	switch format {
	case 0:
		s, err := parseCmapFormat0(b)
		if err != nil {
			return err
		}
		rec.ParsedSubtable = &pb.CmapEncodingRecord_Format0{Format0: s}
	case 4:
		s, err := parseCmapFormat4(b)
		if err != nil {
			return err
		}
		rec.ParsedSubtable = &pb.CmapEncodingRecord_Format4{Format4: s}
	case 6:
		s, err := parseCmapFormat6(b)
		if err != nil {
			return err
		}
		rec.ParsedSubtable = &pb.CmapEncodingRecord_Format6{Format6: s}
	case 10:
		s, err := parseCmapFormat10(b)
		if err != nil {
			return err
		}
		rec.ParsedSubtable = &pb.CmapEncodingRecord_Format10{Format10: s}
	case 12:
		s, err := parseCmapFormat12(b)
		if err != nil {
			return err
		}
		rec.ParsedSubtable = &pb.CmapEncodingRecord_Format12{Format12: s}
	case 13:
		s, err := parseCmapFormat13(b)
		if err != nil {
			return err
		}
		rec.ParsedSubtable = &pb.CmapEncodingRecord_Format13{Format13: s}
	case 14:
		s, err := parseCmapFormat14(b)
		if err != nil {
			return err
		}
		rec.ParsedSubtable = &pb.CmapEncodingRecord_Format14{Format14: s}
	}
	return nil
}

func parseCmapFormat0(b []byte) (*pb.CmapSubtableFormat0, error) {
	if len(b) < 6+256 {
		return nil, fmt.Errorf("cmap fmt0: body too short (%d)", len(b))
	}
	return &pb.CmapSubtableFormat0{
		Length:       uint32(binary.BigEndian.Uint16(b[2:4])),
		Language:     uint32(binary.BigEndian.Uint16(b[4:6])),
		GlyphIdArray: append([]byte(nil), b[6:6+256]...),
	}, nil
}

func parseCmapFormat4(b []byte) (*pb.CmapSubtableFormat4, error) {
	if len(b) < 14 {
		return nil, fmt.Errorf("cmap fmt4: header too short (%d)", len(b))
	}
	length := uint32(binary.BigEndian.Uint16(b[2:4]))
	language := uint32(binary.BigEndian.Uint16(b[4:6]))
	segCountX2 := uint32(binary.BigEndian.Uint16(b[6:8]))
	searchRange := uint32(binary.BigEndian.Uint16(b[8:10]))
	entrySelector := uint32(binary.BigEndian.Uint16(b[10:12]))
	rangeShift := uint32(binary.BigEndian.Uint16(b[12:14]))
	if segCountX2%2 != 0 {
		return nil, fmt.Errorf("cmap fmt4: odd segCountX2 %d", segCountX2)
	}
	segCount := int(segCountX2 / 2)
	// Layout: endCode[segCount] reservedPad[1] startCode[segCount]
	// idDelta[segCount] idRangeOffset[segCount] glyphIdArray[*]
	need := 14 + 2*segCount + 2 + 2*segCount + 2*segCount + 2*segCount
	if len(b) < need {
		return nil, fmt.Errorf("cmap fmt4: segments overrun buffer (need %d, have %d)", need, len(b))
	}
	cursor := 14
	endCode := readUint16Array(b, cursor, segCount)
	cursor += 2 * segCount
	cursor += 2 // reservedPad
	startCode := readUint16Array(b, cursor, segCount)
	cursor += 2 * segCount
	idDelta := make([]int32, segCount)
	for i := 0; i < segCount; i++ {
		idDelta[i] = int32(int16(binary.BigEndian.Uint16(b[cursor : cursor+2])))
		cursor += 2
	}
	idRangeOffset := readUint16Array(b, cursor, segCount)
	cursor += 2 * segCount
	// Remainder is glyphIdArray. The declared subtable length bounds it;
	// prefer that over len(b) to avoid pulling in neighbouring tables.
	end := int(length)
	if end <= 0 || end > len(b) {
		end = len(b)
	}
	glyphIdArray := []uint32{}
	for cursor+2 <= end {
		glyphIdArray = append(glyphIdArray, uint32(binary.BigEndian.Uint16(b[cursor:cursor+2])))
		cursor += 2
	}
	return &pb.CmapSubtableFormat4{
		Length:        length,
		Language:      language,
		SegCountX2:    segCountX2,
		SearchRange:   searchRange,
		EntrySelector: entrySelector,
		RangeShift:    rangeShift,
		EndCode:       endCode,
		StartCode:     startCode,
		IdDelta:       idDelta,
		IdRangeOffset: idRangeOffset,
		GlyphIdArray:  glyphIdArray,
	}, nil
}

func parseCmapFormat6(b []byte) (*pb.CmapSubtableFormat6, error) {
	if len(b) < 10 {
		return nil, fmt.Errorf("cmap fmt6: header too short (%d)", len(b))
	}
	entryCount := uint32(binary.BigEndian.Uint16(b[8:10]))
	need := 10 + 2*int(entryCount)
	if len(b) < need {
		return nil, fmt.Errorf("cmap fmt6: entries overrun (need %d, have %d)", need, len(b))
	}
	return &pb.CmapSubtableFormat6{
		Length:       uint32(binary.BigEndian.Uint16(b[2:4])),
		Language:     uint32(binary.BigEndian.Uint16(b[4:6])),
		FirstCode:    uint32(binary.BigEndian.Uint16(b[6:8])),
		EntryCount:   entryCount,
		GlyphIdArray: readUint16Array(b, 10, int(entryCount)),
	}, nil
}

func parseCmapFormat10(b []byte) (*pb.CmapSubtableFormat10, error) {
	if len(b) < 20 {
		return nil, fmt.Errorf("cmap fmt10: header too short (%d)", len(b))
	}
	numChars := binary.BigEndian.Uint32(b[16:20])
	need := 20 + 2*int(numChars)
	if len(b) < need {
		return nil, fmt.Errorf("cmap fmt10: entries overrun (need %d, have %d)", need, len(b))
	}
	return &pb.CmapSubtableFormat10{
		Length:        binary.BigEndian.Uint32(b[4:8]),
		Language:      binary.BigEndian.Uint32(b[8:12]),
		StartCharCode: binary.BigEndian.Uint32(b[12:16]),
		NumChars:      numChars,
		GlyphIdArray:  readUint16Array(b, 20, int(numChars)),
	}, nil
}

func parseCmapFormat12(b []byte) (*pb.CmapSubtableFormat12, error) {
	if len(b) < 16 {
		return nil, fmt.Errorf("cmap fmt12: header too short (%d)", len(b))
	}
	numGroups := binary.BigEndian.Uint32(b[12:16])
	need := 16 + 12*int(numGroups)
	if len(b) < need {
		return nil, fmt.Errorf("cmap fmt12: groups overrun (need %d, have %d)", need, len(b))
	}
	groups := make([]*pb.CmapSequentialMapGroup, numGroups)
	for i := uint32(0); i < numGroups; i++ {
		off := 16 + 12*int(i)
		groups[i] = &pb.CmapSequentialMapGroup{
			StartCharCode: binary.BigEndian.Uint32(b[off : off+4]),
			EndCharCode:   binary.BigEndian.Uint32(b[off+4 : off+8]),
			StartGlyphId:  binary.BigEndian.Uint32(b[off+8 : off+12]),
		}
	}
	return &pb.CmapSubtableFormat12{
		Length:   binary.BigEndian.Uint32(b[4:8]),
		Language: binary.BigEndian.Uint32(b[8:12]),
		Groups:   groups,
	}, nil
}

func parseCmapFormat13(b []byte) (*pb.CmapSubtableFormat13, error) {
	if len(b) < 16 {
		return nil, fmt.Errorf("cmap fmt13: header too short (%d)", len(b))
	}
	numGroups := binary.BigEndian.Uint32(b[12:16])
	need := 16 + 12*int(numGroups)
	if len(b) < need {
		return nil, fmt.Errorf("cmap fmt13: groups overrun (need %d, have %d)", need, len(b))
	}
	groups := make([]*pb.CmapConstantMapGroup, numGroups)
	for i := uint32(0); i < numGroups; i++ {
		off := 16 + 12*int(i)
		groups[i] = &pb.CmapConstantMapGroup{
			StartCharCode: binary.BigEndian.Uint32(b[off : off+4]),
			EndCharCode:   binary.BigEndian.Uint32(b[off+4 : off+8]),
			GlyphId:       binary.BigEndian.Uint32(b[off+8 : off+12]),
		}
	}
	return &pb.CmapSubtableFormat13{
		Length:   binary.BigEndian.Uint32(b[4:8]),
		Language: binary.BigEndian.Uint32(b[8:12]),
		Groups:   groups,
	}, nil
}

// parseCmapFormat14 parses a Unicode Variation Sequences subtable.
// Per OT §5.cmap fmt14 the header layout is:
//
//	uint16 format, uint32 length, uint32 numVarSelectorRecords,
//	[uint24 varSelector + uint32 defaultUVSOffset + uint32 nonDefaultUVSOffset] × N
//
// Both UVS tables are optional per-record (offset 0 means absent).
func parseCmapFormat14(b []byte) (*pb.CmapSubtableFormat14, error) {
	if len(b) < 10 {
		return nil, fmt.Errorf("cmap fmt14: header too short (%d)", len(b))
	}
	length := binary.BigEndian.Uint32(b[2:6])
	numRecords := binary.BigEndian.Uint32(b[6:10])
	end := int(length)
	if end <= 0 || end > len(b) {
		end = len(b)
	}
	recordLen := 11
	need := 10 + recordLen*int(numRecords)
	if need > end {
		return nil, fmt.Errorf("cmap fmt14: records overrun (need %d, have %d)", need, end)
	}
	vs := make([]*pb.CmapVariationSelector, numRecords)
	for i := uint32(0); i < numRecords; i++ {
		off := 10 + recordLen*int(i)
		varSelector := uint32(b[off])<<16 | uint32(b[off+1])<<8 | uint32(b[off+2])
		defaultOff := binary.BigEndian.Uint32(b[off+3 : off+7])
		nonDefaultOff := binary.BigEndian.Uint32(b[off+7 : off+11])
		rec := &pb.CmapVariationSelector{VarSelector: varSelector}
		if defaultOff != 0 {
			d, err := parseCmapDefaultUVS(b, int(defaultOff), end)
			if err != nil {
				return nil, fmt.Errorf("cmap fmt14: record %d defaultUVS: %w", i, err)
			}
			rec.DefaultUvs = d
		}
		if nonDefaultOff != 0 {
			n, err := parseCmapNonDefaultUVS(b, int(nonDefaultOff), end)
			if err != nil {
				return nil, fmt.Errorf("cmap fmt14: record %d nonDefaultUVS: %w", i, err)
			}
			rec.NonDefaultUvs = n
		}
		vs[i] = rec
	}
	return &pb.CmapSubtableFormat14{
		Length:        length,
		VarSelectors:  vs,
	}, nil
}

func parseCmapDefaultUVS(b []byte, off, end int) (*pb.CmapDefaultUVS, error) {
	if off+4 > end {
		return nil, fmt.Errorf("defaultUVS header OOB (off %d)", off)
	}
	numRanges := binary.BigEndian.Uint32(b[off : off+4])
	need := off + 4 + 4*int(numRanges)
	if need > end {
		return nil, fmt.Errorf("defaultUVS body OOB (need %d, have %d)", need, end)
	}
	out := &pb.CmapDefaultUVS{Ranges: make([]*pb.CmapUnicodeRange, numRanges)}
	for i := uint32(0); i < numRanges; i++ {
		p := off + 4 + 4*int(i)
		start := uint32(b[p])<<16 | uint32(b[p+1])<<8 | uint32(b[p+2])
		out.Ranges[i] = &pb.CmapUnicodeRange{
			StartUnicodeValue: start,
			AdditionalCount:   uint32(b[p+3]),
		}
	}
	return out, nil
}

func parseCmapNonDefaultUVS(b []byte, off, end int) (*pb.CmapNonDefaultUVS, error) {
	if off+4 > end {
		return nil, fmt.Errorf("nonDefaultUVS header OOB (off %d)", off)
	}
	numMappings := binary.BigEndian.Uint32(b[off : off+4])
	need := off + 4 + 5*int(numMappings)
	if need > end {
		return nil, fmt.Errorf("nonDefaultUVS body OOB (need %d, have %d)", need, end)
	}
	out := &pb.CmapNonDefaultUVS{Mappings: make([]*pb.CmapUVSMapping, numMappings)}
	for i := uint32(0); i < numMappings; i++ {
		p := off + 4 + 5*int(i)
		unicode := uint32(b[p])<<16 | uint32(b[p+1])<<8 | uint32(b[p+2])
		gid := uint32(binary.BigEndian.Uint16(b[p+3 : p+5]))
		out.Mappings[i] = &pb.CmapUVSMapping{UnicodeValue: unicode, GlyphId: gid}
	}
	return out, nil
}

func readUint16Array(b []byte, off, n int) []uint32 {
	out := make([]uint32, n)
	for i := 0; i < n; i++ {
		out[i] = uint32(binary.BigEndian.Uint16(b[off+2*i : off+2*i+2]))
	}
	return out
}
