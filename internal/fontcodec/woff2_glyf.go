package fontcodec

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Per W3C WOFF2 spec §5.1: the glyf table is transformed into a fixed
// header + 7 sub-streams (nContour, nPoints, flag, glyph, composite,
// bbox, instruction) plus an optional overlapSimpleBitmap. This file
// reverses that transform to produce standard SFNT glyf + loca bytes.

// tripletEntry is one row of the 128-entry triplet encoding table per
// WOFF2 spec §5.2. byteCount includes the flag byte itself.
type tripletEntry struct {
	byteCount uint8
	xBits     uint8
	yBits     uint8
	deltaX    int16
	deltaY    int16
	xSign     int8 // -1, 0 (when xBits==0), +1
	ySign     int8
}

// woff2TripletTable is built from the spec's structured pattern rather
// than typed out by hand — the rows fall into five regimes
// (8-bit-Y / 8-bit-X / 4+4 / 8+8 / 12+12 / 16+16) with predictable
// delta + sign cycling.
var woff2TripletTable = func() [128]tripletEntry {
	var t [128]tripletEntry
	signsXY := func(i int) (xs, ys int8) {
		switch i {
		case 0:
			return -1, -1
		case 1:
			return 1, -1
		case 2:
			return -1, 1
		case 3:
			return 1, 1
		}
		return
	}
	// 0..9: xBits=0, yBits=8, deltaY = 0,256,512,768,1024 (paired -/+)
	for i := 0; i < 10; i++ {
		t[i] = tripletEntry{
			byteCount: 2, xBits: 0, yBits: 8,
			deltaY: int16((i / 2) * 256),
			ySign:  []int8{-1, 1}[i%2],
		}
	}
	// 10..19: xBits=8, yBits=0, deltaX = 0,256,512,768,1024 (paired -/+)
	for i := 0; i < 10; i++ {
		t[10+i] = tripletEntry{
			byteCount: 2, xBits: 8, yBits: 0,
			deltaX: int16((i / 2) * 256),
			xSign:  []int8{-1, 1}[i%2],
		}
	}
	// 20..83: xBits=4, yBits=4, delta in {1,17,33,49} for both axes,
	// with 4-way sign cycling within each delta cell.
	for i := 0; i < 64; i++ {
		idx := i / 4
		dY := int16((idx/4)*16 + 1)
		dX := int16((idx%4)*16 + 1)
		xs, ys := signsXY(i % 4)
		t[20+i] = tripletEntry{
			byteCount: 2, xBits: 4, yBits: 4,
			deltaX: dX, deltaY: dY,
			xSign: xs, ySign: ys,
		}
	}
	// 84..119: xBits=8, yBits=8, delta in {1,257,513}, 4-way signs.
	for i := 0; i < 36; i++ {
		idx := i / 4
		dY := int16((idx/3)*256 + 1)
		dX := int16((idx%3)*256 + 1)
		xs, ys := signsXY(i % 4)
		t[84+i] = tripletEntry{
			byteCount: 3, xBits: 8, yBits: 8,
			deltaX: dX, deltaY: dY,
			xSign: xs, ySign: ys,
		}
	}
	// 120..123: xBits=12, yBits=12, no delta.
	for i := 0; i < 4; i++ {
		xs, ys := signsXY(i)
		t[120+i] = tripletEntry{
			byteCount: 4, xBits: 12, yBits: 12,
			xSign: xs, ySign: ys,
		}
	}
	// 124..127: xBits=16, yBits=16, no delta.
	for i := 0; i < 4; i++ {
		xs, ys := signsXY(i)
		t[124+i] = tripletEntry{
			byteCount: 5, xBits: 16, yBits: 16,
			xSign: xs, ySign: ys,
		}
	}
	return t
}()

// Composite glyph component flag bits (OpenType glyf format).
const (
	cmpArg1And2AreWords    = 0x0001
	cmpArgsAreXYValues     = 0x0002
	cmpRoundXYToGrid       = 0x0004
	cmpWeHaveAScale        = 0x0008
	cmpMoreComponents      = 0x0020
	cmpWeHaveAnXAndYScale  = 0x0040
	cmpWeHaveATwoByTwo     = 0x0080
	cmpWeHaveInstructions  = 0x0100
)

// reverseWoff2GlyfTransform takes the transformed glyf bytes (as they
// appear in the brotli-decompressed WOFF2 body) and returns standard
// SFNT glyf + loca bytes plus the indexFormat used (0=short, 1=long).
func reverseWoff2GlyfTransform(buf []byte) (glyf, loca []byte, indexFormat uint16, err error) {
	if len(buf) < 36 {
		return nil, nil, 0, fmt.Errorf("woff2 glyf: header too short (%d)", len(buf))
	}
	// reserved (uint16) is required to be 0; we just skip it.
	optionFlags := binary.BigEndian.Uint16(buf[2:4])
	numGlyphs := binary.BigEndian.Uint16(buf[4:6])
	indexFormat = binary.BigEndian.Uint16(buf[6:8])

	streamSizes := [7]uint32{}
	for i := 0; i < 7; i++ {
		streamSizes[i] = binary.BigEndian.Uint32(buf[8+i*4 : 12+i*4])
	}

	cursor := uint32(36)
	end := uint32(len(buf))
	carve := func(sz uint32, name string) ([]byte, error) {
		if cursor+sz > end {
			return nil, fmt.Errorf("woff2 glyf: %s stream overruns buffer (need %d, have %d)", name, sz, end-cursor)
		}
		s := buf[cursor : cursor+sz]
		cursor += sz
		return s, nil
	}
	nContourBuf, err := carve(streamSizes[0], "nContour")
	if err != nil {
		return nil, nil, 0, err
	}
	nPointsBuf, err := carve(streamSizes[1], "nPoints")
	if err != nil {
		return nil, nil, 0, err
	}
	flagBuf, err := carve(streamSizes[2], "flag")
	if err != nil {
		return nil, nil, 0, err
	}
	glyphBuf, err := carve(streamSizes[3], "glyph")
	if err != nil {
		return nil, nil, 0, err
	}
	compBuf, err := carve(streamSizes[4], "composite")
	if err != nil {
		return nil, nil, 0, err
	}
	bboxBuf, err := carve(streamSizes[5], "bbox")
	if err != nil {
		return nil, nil, 0, err
	}
	instBuf, err := carve(streamSizes[6], "instruction")
	if err != nil {
		return nil, nil, 0, err
	}
	var overlapBuf []byte
	if optionFlags&1 != 0 {
		// 4 * floor((numGlyphs+31)/32) bytes per spec — same packing as
		// the bbox bitmap but for OVERLAP_SIMPLE.
		ovLen := uint32(4 * ((uint32(numGlyphs) + 31) / 32))
		overlapBuf, err = carve(ovLen, "overlapSimple")
		if err != nil {
			return nil, nil, 0, err
		}
	}

	// Per spec §5.1: bboxBitmap is 4 * ceil(numGlyphs/32) bytes (padded
	// to a 32-bit boundary), NOT just ceil(numGlyphs/8).
	bboxBitmapLen := int(4 * ((uint32(numGlyphs) + 31) / 32))
	if bboxBitmapLen > len(bboxBuf) {
		return nil, nil, 0, fmt.Errorf("woff2 glyf: bbox bitmap (%d) larger than bbox stream (%d)", bboxBitmapLen, len(bboxBuf))
	}
	bboxBitmap := bboxBuf[:bboxBitmapLen]
	bboxValues := bboxBuf[bboxBitmapLen:]

	cur := streamCursors{
		nPointsBuf:  nPointsBuf,
		flagBuf:     flagBuf,
		glyphBuf:    glyphBuf,
		compBuf:     compBuf,
		bboxValues:  bboxValues,
		instBuf:     instBuf,
		bboxBitmap:  bboxBitmap,
		overlapBmp:  overlapBuf,
	}

	glyphs := make([][]byte, numGlyphs)
	for i := uint16(0); i < numGlyphs; i++ {
		if int(i)*2+2 > len(nContourBuf) {
			return nil, nil, 0, fmt.Errorf("woff2 glyf: nContour stream too short for glyph %d", i)
		}
		nContours := int16(binary.BigEndian.Uint16(nContourBuf[i*2 : i*2+2]))
		switch {
		case nContours == 0:
			glyphs[i] = nil
		case nContours > 0:
			g, err := decodeSimpleGlyph(int(nContours), int(i), &cur)
			if err != nil {
				return nil, nil, 0, fmt.Errorf("glyph %d (simple): %w", i, err)
			}
			glyphs[i] = g
		case nContours == -1:
			g, err := decodeCompositeGlyph(&cur)
			if err != nil {
				return nil, nil, 0, fmt.Errorf("glyph %d (composite): %w", i, err)
			}
			glyphs[i] = g
		default:
			return nil, nil, 0, fmt.Errorf("woff2 glyf: glyph %d has nContours=%d", i, nContours)
		}
	}

	// Pad each glyph and emit loca offsets. Short loca uses offset/2,
	// so simple-glyph bodies are padded to a 2-byte boundary; long loca
	// has no alignment requirement, but fonttools (and most encoders)
	// still pad to 4 bytes for cache friendliness — the spec does not
	// mandate this, so we use 2/4 to match the indexFormat exactly.
	align := 2
	if indexFormat == 1 {
		align = 4
	}
	var glyfBuf bytes.Buffer
	offsets := make([]uint32, 0, int(numGlyphs)+1)
	for _, g := range glyphs {
		offsets = append(offsets, uint32(glyfBuf.Len()))
		if g != nil {
			glyfBuf.Write(g)
			for glyfBuf.Len()%align != 0 {
				glyfBuf.WriteByte(0)
			}
		}
	}
	offsets = append(offsets, uint32(glyfBuf.Len()))

	var locaBuf bytes.Buffer
	if indexFormat == 0 {
		for _, off := range offsets {
			if off%2 != 0 || off/2 > 0xffff {
				return nil, nil, 0, fmt.Errorf("woff2 glyf: short loca offset %d unrepresentable", off)
			}
			_ = binary.Write(&locaBuf, binary.BigEndian, uint16(off/2))
		}
	} else {
		for _, off := range offsets {
			_ = binary.Write(&locaBuf, binary.BigEndian, off)
		}
	}

	return glyfBuf.Bytes(), locaBuf.Bytes(), indexFormat, nil
}

// streamCursors tracks read positions across all the per-table streams
// so the per-glyph decoders can advance them sequentially.
type streamCursors struct {
	nPointsBuf, flagBuf, glyphBuf, compBuf, bboxValues, instBuf []byte
	nPointsCur, flagCur, glyphCur, compCur, bboxValCur, instCur int
	bboxBitmap, overlapBmp                                      []byte
}

// bitSet checks bit `i` in a packed bitmap where bit 0 of glyph 0 is
// the high (MSB) bit of byte 0.
func bitSet(bmp []byte, i int) bool {
	if bmp == nil {
		return false
	}
	byteIdx := i >> 3
	if byteIdx >= len(bmp) {
		return false
	}
	bitIdx := 7 - (i & 7)
	return bmp[byteIdx]&(1<<bitIdx) != 0
}

func decodeSimpleGlyph(nContours, glyphIdx int, c *streamCursors) ([]byte, error) {
	// Read endpoints from nPointsStream (one 255UInt16 per contour).
	endPts := make([]uint16, nContours)
	totalPoints := 0
	for i := 0; i < nContours; i++ {
		v, n, err := read255UShort(c.nPointsBuf[c.nPointsCur:])
		if err != nil {
			return nil, fmt.Errorf("nPoints contour %d: %w", i, err)
		}
		c.nPointsCur += n
		totalPoints += int(v)
		if totalPoints == 0 || totalPoints > 0x10000 {
			return nil, fmt.Errorf("invalid totalPoints=%d", totalPoints)
		}
		endPts[i] = uint16(totalPoints - 1)
	}

	// Read flagStream + glyphStream into per-point dx/dy + on-curve flag.
	if c.flagCur+totalPoints > len(c.flagBuf) {
		return nil, fmt.Errorf("flagStream truncated (need %d, have %d)", totalPoints, len(c.flagBuf)-c.flagCur)
	}
	pointFlags := c.flagBuf[c.flagCur : c.flagCur+totalPoints]
	c.flagCur += totalPoints

	dxs := make([]int16, totalPoints)
	dys := make([]int16, totalPoints)
	onCurves := make([]bool, totalPoints)
	for p := 0; p < totalPoints; p++ {
		fb := pointFlags[p]
		onCurves[p] = (fb & 0x80) == 0
		idx := fb & 0x7f
		entry := woff2TripletTable[idx]
		need := int(entry.byteCount - 1)
		if c.glyphCur+need > len(c.glyphBuf) {
			return nil, fmt.Errorf("glyphStream truncated at point %d (need %d, have %d)", p, need, len(c.glyphBuf)-c.glyphCur)
		}
		val := c.glyphBuf[c.glyphCur : c.glyphCur+need]
		c.glyphCur += need

		var dx, dy int16
		switch {
		case entry.xBits == 0 && entry.yBits == 8:
			dy = int16(int16(val[0])+entry.deltaY) * int16(entry.ySign)
		case entry.xBits == 8 && entry.yBits == 0:
			dx = int16(int16(val[0])+entry.deltaX) * int16(entry.xSign)
		case entry.xBits == 4 && entry.yBits == 4:
			hi := int16(val[0] >> 4)
			lo := int16(val[0] & 0x0f)
			dx = (hi + entry.deltaX) * int16(entry.xSign)
			dy = (lo + entry.deltaY) * int16(entry.ySign)
		case entry.xBits == 8 && entry.yBits == 8:
			dx = (int16(val[0]) + entry.deltaX) * int16(entry.xSign)
			dy = (int16(val[1]) + entry.deltaY) * int16(entry.ySign)
		case entry.xBits == 12 && entry.yBits == 12:
			packed := uint32(val[0])<<16 | uint32(val[1])<<8 | uint32(val[2])
			dx = int16(packed>>12) * int16(entry.xSign)
			dy = int16(packed&0xfff) * int16(entry.ySign)
		case entry.xBits == 16 && entry.yBits == 16:
			dx = int16(binary.BigEndian.Uint16(val[0:2])) * int16(entry.xSign)
			dy = int16(binary.BigEndian.Uint16(val[2:4])) * int16(entry.ySign)
		default:
			return nil, fmt.Errorf("triplet idx %d has unhandled bit sizes (%d/%d)", idx, entry.xBits, entry.yBits)
		}
		dxs[p] = dx
		dys[p] = dy
	}

	// Instruction length (255UInt16 from glyphStream) + bytes from instStream.
	instLen, n, err := read255UShort(c.glyphBuf[c.glyphCur:])
	if err != nil {
		return nil, fmt.Errorf("simple-glyph instLen: %w", err)
	}
	c.glyphCur += n
	if c.instCur+int(instLen) > len(c.instBuf) {
		return nil, fmt.Errorf("instructionStream truncated (need %d)", instLen)
	}
	instructions := c.instBuf[c.instCur : c.instCur+int(instLen)]
	c.instCur += int(instLen)

	// Bbox: from bboxValues if the bitmap bit is set, else compute.
	var xMin, yMin, xMax, yMax int16
	if bitSet(c.bboxBitmap, glyphIdx) {
		if c.bboxValCur+8 > len(c.bboxValues) {
			return nil, fmt.Errorf("bbox stream truncated for glyph %d", glyphIdx)
		}
		xMin = int16(binary.BigEndian.Uint16(c.bboxValues[c.bboxValCur : c.bboxValCur+2]))
		yMin = int16(binary.BigEndian.Uint16(c.bboxValues[c.bboxValCur+2 : c.bboxValCur+4]))
		xMax = int16(binary.BigEndian.Uint16(c.bboxValues[c.bboxValCur+4 : c.bboxValCur+6]))
		yMax = int16(binary.BigEndian.Uint16(c.bboxValues[c.bboxValCur+6 : c.bboxValCur+8]))
		c.bboxValCur += 8
	} else {
		ax, ay := int16(0), int16(0)
		first := true
		for i := 0; i < totalPoints; i++ {
			ax += dxs[i]
			ay += dys[i]
			if first {
				xMin, xMax, yMin, yMax = ax, ax, ay, ay
				first = false
				continue
			}
			if ax < xMin {
				xMin = ax
			}
			if ax > xMax {
				xMax = ax
			}
			if ay < yMin {
				yMin = ay
			}
			if ay > yMax {
				yMax = ay
			}
		}
	}

	// Whether this glyph's first point should carry OVERLAP_SIMPLE.
	overlap := bitSet(c.overlapBmp, glyphIdx)

	return packSimpleGlyph(int16(nContours), xMin, yMin, xMax, yMax,
		endPts, instructions, dxs, dys, onCurves, overlap)
}

// packSimpleGlyph emits a standard SFNT simple glyph entry: header,
// endpoints, instructions, then run-length-encoded flags and packed
// X/Y coordinate streams (smallest encoding per axis: zero, signed
// uint8 with sign-bit, or signed int16).
func packSimpleGlyph(nContours, xMin, yMin, xMax, yMax int16,
	endPts []uint16, instructions []byte,
	dxs, dys []int16, onCurves []bool, overlap bool,
) ([]byte, error) {
	totalPoints := len(dxs)

	// Choose flag + per-point coord encoding for each axis.
	flags := make([]byte, totalPoints)
	xCoords := make([]byte, 0, totalPoints*2)
	yCoords := make([]byte, 0, totalPoints*2)
	for i := 0; i < totalPoints; i++ {
		var f byte
		if onCurves[i] {
			f |= 0x01
		}
		if overlap && i == 0 {
			f |= 0x40
		}
		dx := dxs[i]
		switch {
		case dx == 0:
			f |= 0x10 // X_IS_SAME (X_SHORT clear) → no X bytes
		case dx > 0 && dx <= 255:
			f |= 0x02 | 0x10 // X_SHORT + positive
			xCoords = append(xCoords, byte(dx))
		case dx < 0 && dx >= -255:
			f |= 0x02 // X_SHORT + negative
			xCoords = append(xCoords, byte(-dx))
		default:
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], uint16(dx))
			xCoords = append(xCoords, b[0], b[1])
		}
		dy := dys[i]
		switch {
		case dy == 0:
			f |= 0x20
		case dy > 0 && dy <= 255:
			f |= 0x04 | 0x20
			yCoords = append(yCoords, byte(dy))
		case dy < 0 && dy >= -255:
			f |= 0x04
			yCoords = append(yCoords, byte(-dy))
		default:
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], uint16(dy))
			yCoords = append(yCoords, b[0], b[1])
		}
		flags[i] = f
	}

	// Run-length compress consecutive identical flags using REPEAT_FLAG.
	var flagsOut bytes.Buffer
	for i := 0; i < totalPoints; {
		j := i + 1
		for j < totalPoints && flags[j] == flags[i] && j-i < 256 {
			j++
		}
		run := j - i
		if run >= 2 {
			flagsOut.WriteByte(flags[i] | 0x08)
			flagsOut.WriteByte(byte(run - 1))
		} else {
			flagsOut.WriteByte(flags[i])
		}
		i = j
	}

	var out bytes.Buffer
	_ = binary.Write(&out, binary.BigEndian, nContours)
	_ = binary.Write(&out, binary.BigEndian, xMin)
	_ = binary.Write(&out, binary.BigEndian, yMin)
	_ = binary.Write(&out, binary.BigEndian, xMax)
	_ = binary.Write(&out, binary.BigEndian, yMax)
	for _, ep := range endPts {
		_ = binary.Write(&out, binary.BigEndian, ep)
	}
	_ = binary.Write(&out, binary.BigEndian, uint16(len(instructions)))
	out.Write(instructions)
	out.Write(flagsOut.Bytes())
	out.Write(xCoords)
	out.Write(yCoords)
	return out.Bytes(), nil
}

// synthesizeWoff2Glyf is the inverse of reverseWoff2GlyfTransform: given
// plain SFNT glyf + loca bytes, it emits the WOFF2 §5.1 transformed
// representation (header + seven sub-streams + optional overlap bitmap).
// Byte-for-byte parity with a specific encoder (e.g. fonttools) is NOT
// guaranteed — triplet selection and bbox-bitmap policy are encoder
// choices — but round-trip through reverseWoff2GlyfTransform returns
// the original bytes.
func synthesizeWoff2Glyf(glyfBytes, locaBytes []byte, numGlyphs, indexFormat uint16) ([]byte, error) {
	offsets := make([]uint32, numGlyphs+1)
	if indexFormat == 0 {
		if len(locaBytes) < int(numGlyphs+1)*2 {
			return nil, fmt.Errorf("woff2 synth: short loca for %d glyphs (short)", numGlyphs)
		}
		for i := range offsets {
			offsets[i] = uint32(binary.BigEndian.Uint16(locaBytes[i*2:i*2+2])) * 2
		}
	} else {
		if len(locaBytes) < int(numGlyphs+1)*4 {
			return nil, fmt.Errorf("woff2 synth: short loca for %d glyphs (long)", numGlyphs)
		}
		for i := range offsets {
			offsets[i] = binary.BigEndian.Uint32(locaBytes[i*4 : i*4+4])
		}
	}

	var (
		nContourStream  bytes.Buffer
		nPointsStream   bytes.Buffer
		flagStream      bytes.Buffer
		glyphStream     bytes.Buffer
		compositeStream bytes.Buffer
		bboxValues      bytes.Buffer
		instStream      bytes.Buffer
	)
	bitmapLen := 4 * ((int(numGlyphs) + 31) / 32)
	bboxBitmap := make([]byte, bitmapLen)
	var overlapBitmap []byte
	setBit := func(bmp []byte, i int) {
		bmp[i>>3] |= 1 << (7 - (i & 7))
	}

	for i := uint16(0); i < numGlyphs; i++ {
		start := offsets[i]
		end := offsets[i+1]
		if end < start || int(end) > len(glyfBytes) {
			return nil, fmt.Errorf("woff2 synth: glyph %d offsets out of range", i)
		}
		body := glyfBytes[start:end]

		if len(body) == 0 {
			_ = binary.Write(&nContourStream, binary.BigEndian, int16(0))
			continue
		}
		if len(body) < 10 {
			return nil, fmt.Errorf("woff2 synth: glyph %d body too short (%d)", i, len(body))
		}
		nContours := int16(binary.BigEndian.Uint16(body[0:2]))
		xMin := int16(binary.BigEndian.Uint16(body[2:4]))
		yMin := int16(binary.BigEndian.Uint16(body[4:6]))
		xMax := int16(binary.BigEndian.Uint16(body[6:8]))
		yMax := int16(binary.BigEndian.Uint16(body[8:10]))
		_ = binary.Write(&nContourStream, binary.BigEndian, nContours)

		switch {
		case nContours > 0:
			endPts, insts, dxs, dys, onCurves, overlap, err := parseSfntSimpleGlyph(body)
			if err != nil {
				return nil, fmt.Errorf("woff2 synth: glyph %d: %w", i, err)
			}
			// nPointsStream: one 255UInt16 per contour, count of points.
			prev := -1
			for _, e := range endPts {
				count := int(e) - prev
				prev = int(e)
				nPointsStream.Write(write255UShort(uint32(count)))
			}
			// Encode bbox: include explicit bbox only if computed doesn't
			// match header (fonttools-compatible policy).
			if !simpleBboxMatches(dxs, dys, xMin, yMin, xMax, yMax) {
				setBit(bboxBitmap, int(i))
				_ = binary.Write(&bboxValues, binary.BigEndian, xMin)
				_ = binary.Write(&bboxValues, binary.BigEndian, yMin)
				_ = binary.Write(&bboxValues, binary.BigEndian, xMax)
				_ = binary.Write(&bboxValues, binary.BigEndian, yMax)
			}
			for p := 0; p < len(dxs); p++ {
				idx, tbytes, err := encodeTriplet(dxs[p], dys[p])
				if err != nil {
					return nil, fmt.Errorf("woff2 synth: glyph %d point %d: %w", i, p, err)
				}
				fb := byte(idx & 0x7f)
				if !onCurves[p] {
					fb |= 0x80
				}
				flagStream.WriteByte(fb)
				glyphStream.Write(tbytes)
			}
			glyphStream.Write(write255UShort(uint32(len(insts))))
			instStream.Write(insts)
			if overlap {
				if overlapBitmap == nil {
					overlapBitmap = make([]byte, bitmapLen)
				}
				setBit(overlapBitmap, int(i))
			}
		case nContours == -1:
			setBit(bboxBitmap, int(i))
			_ = binary.Write(&bboxValues, binary.BigEndian, xMin)
			_ = binary.Write(&bboxValues, binary.BigEndian, yMin)
			_ = binary.Write(&bboxValues, binary.BigEndian, xMax)
			_ = binary.Write(&bboxValues, binary.BigEndian, yMax)
			compBytes, insts, haveInst, err := parseSfntCompositeGlyph(body)
			if err != nil {
				return nil, fmt.Errorf("woff2 synth: glyph %d composite: %w", i, err)
			}
			compositeStream.Write(compBytes)
			if haveInst {
				glyphStream.Write(write255UShort(uint32(len(insts))))
				instStream.Write(insts)
			}
		default:
			return nil, fmt.Errorf("woff2 synth: glyph %d has unexpected nContours=%d", i, nContours)
		}
	}

	optionFlags := uint16(0)
	if overlapBitmap != nil {
		optionFlags = 1
	}
	bboxStreamLen := uint32(len(bboxBitmap)) + uint32(bboxValues.Len())

	var out bytes.Buffer
	_ = binary.Write(&out, binary.BigEndian, uint16(0)) // reserved
	_ = binary.Write(&out, binary.BigEndian, optionFlags)
	_ = binary.Write(&out, binary.BigEndian, numGlyphs)
	_ = binary.Write(&out, binary.BigEndian, indexFormat)
	_ = binary.Write(&out, binary.BigEndian, uint32(nContourStream.Len()))
	_ = binary.Write(&out, binary.BigEndian, uint32(nPointsStream.Len()))
	_ = binary.Write(&out, binary.BigEndian, uint32(flagStream.Len()))
	_ = binary.Write(&out, binary.BigEndian, uint32(glyphStream.Len()))
	_ = binary.Write(&out, binary.BigEndian, uint32(compositeStream.Len()))
	_ = binary.Write(&out, binary.BigEndian, bboxStreamLen)
	_ = binary.Write(&out, binary.BigEndian, uint32(instStream.Len()))

	out.Write(nContourStream.Bytes())
	out.Write(nPointsStream.Bytes())
	out.Write(flagStream.Bytes())
	out.Write(glyphStream.Bytes())
	out.Write(compositeStream.Bytes())
	out.Write(bboxBitmap)
	out.Write(bboxValues.Bytes())
	out.Write(instStream.Bytes())
	if overlapBitmap != nil {
		out.Write(overlapBitmap)
	}
	return out.Bytes(), nil
}

func simpleBboxMatches(dxs, dys []int16, xMin, yMin, xMax, yMax int16) bool {
	if len(dxs) == 0 {
		return xMin == 0 && yMin == 0 && xMax == 0 && yMax == 0
	}
	var cx, cy, mx, Mx, my, My int16
	cx, cy = dxs[0], dys[0]
	mx, Mx, my, My = cx, cx, cy, cy
	for i := 1; i < len(dxs); i++ {
		cx += dxs[i]
		cy += dys[i]
		if cx < mx {
			mx = cx
		}
		if cx > Mx {
			Mx = cx
		}
		if cy < my {
			my = cy
		}
		if cy > My {
			My = cy
		}
	}
	return mx == xMin && Mx == xMax && my == yMin && My == yMax
}

// parseSfntSimpleGlyph reads a standard SFNT simple glyph body into its
// per-point deltas. Used by synthesizeWoff2Glyf.
func parseSfntSimpleGlyph(body []byte) (endPts []uint16, insts []byte,
	dxs, dys []int16, onCurves []bool, overlap bool, err error) {
	if len(body) < 10 {
		err = fmt.Errorf("simple glyph: header too short")
		return
	}
	nContours := int16(binary.BigEndian.Uint16(body[0:2]))
	if nContours <= 0 {
		err = fmt.Errorf("simple glyph: nContours=%d", nContours)
		return
	}
	pos := 10
	endPts = make([]uint16, nContours)
	for i := range endPts {
		if pos+2 > len(body) {
			err = fmt.Errorf("simple glyph: endpts truncated")
			return
		}
		endPts[i] = binary.BigEndian.Uint16(body[pos : pos+2])
		pos += 2
	}
	totalPoints := int(endPts[nContours-1]) + 1
	if pos+2 > len(body) {
		err = fmt.Errorf("simple glyph: instLen truncated")
		return
	}
	instLen := int(binary.BigEndian.Uint16(body[pos : pos+2]))
	pos += 2
	if pos+instLen > len(body) {
		err = fmt.Errorf("simple glyph: instructions truncated")
		return
	}
	insts = append([]byte(nil), body[pos:pos+instLen]...)
	pos += instLen

	flags := make([]byte, 0, totalPoints)
	for len(flags) < totalPoints {
		if pos >= len(body) {
			err = fmt.Errorf("simple glyph: flags truncated")
			return
		}
		f := body[pos]
		pos++
		flags = append(flags, f)
		if f&0x08 != 0 {
			if pos >= len(body) {
				err = fmt.Errorf("simple glyph: repeat count missing")
				return
			}
			runLen := int(body[pos])
			pos++
			for j := 0; j < runLen && len(flags) < totalPoints; j++ {
				flags = append(flags, f)
			}
		}
	}
	dxs = make([]int16, totalPoints)
	for i := 0; i < totalPoints; i++ {
		f := flags[i]
		switch {
		case f&0x02 != 0:
			if pos >= len(body) {
				err = fmt.Errorf("simple glyph: x short truncated")
				return
			}
			v := int16(body[pos])
			pos++
			if f&0x10 != 0 {
				dxs[i] = v
			} else {
				dxs[i] = -v
			}
		case f&0x10 != 0:
			dxs[i] = 0
		default:
			if pos+2 > len(body) {
				err = fmt.Errorf("simple glyph: x long truncated")
				return
			}
			dxs[i] = int16(binary.BigEndian.Uint16(body[pos : pos+2]))
			pos += 2
		}
	}
	dys = make([]int16, totalPoints)
	for i := 0; i < totalPoints; i++ {
		f := flags[i]
		switch {
		case f&0x04 != 0:
			if pos >= len(body) {
				err = fmt.Errorf("simple glyph: y short truncated")
				return
			}
			v := int16(body[pos])
			pos++
			if f&0x20 != 0 {
				dys[i] = v
			} else {
				dys[i] = -v
			}
		case f&0x20 != 0:
			dys[i] = 0
		default:
			if pos+2 > len(body) {
				err = fmt.Errorf("simple glyph: y long truncated")
				return
			}
			dys[i] = int16(binary.BigEndian.Uint16(body[pos : pos+2]))
			pos += 2
		}
	}
	onCurves = make([]bool, totalPoints)
	for i, f := range flags {
		onCurves[i] = f&0x01 != 0
	}
	if len(flags) > 0 {
		overlap = flags[0]&0x40 != 0
	}
	return
}

// parseSfntCompositeGlyph walks component records in a composite glyph
// body and returns the raw component bytes, instruction bytes (if any),
// and whether WE_HAVE_INSTRUCTIONS was set on any component.
func parseSfntCompositeGlyph(body []byte) (components, instructions []byte, haveInstructions bool, err error) {
	if len(body) < 10 {
		err = fmt.Errorf("composite glyph: header truncated")
		return
	}
	pos := 10
	start := pos
	for {
		if pos+4 > len(body) {
			err = fmt.Errorf("composite glyph: component header truncated")
			return
		}
		flag := binary.BigEndian.Uint16(body[pos : pos+2])
		pos += 4
		argSize := 2
		if flag&cmpArg1And2AreWords != 0 {
			argSize = 4
		}
		scaleSize := 0
		switch {
		case flag&cmpWeHaveATwoByTwo != 0:
			scaleSize = 8
		case flag&cmpWeHaveAnXAndYScale != 0:
			scaleSize = 4
		case flag&cmpWeHaveAScale != 0:
			scaleSize = 2
		}
		if pos+argSize+scaleSize > len(body) {
			err = fmt.Errorf("composite glyph: component body truncated")
			return
		}
		pos += argSize + scaleSize
		if flag&cmpWeHaveInstructions != 0 {
			haveInstructions = true
		}
		if flag&cmpMoreComponents == 0 {
			break
		}
	}
	components = append([]byte(nil), body[start:pos]...)
	if haveInstructions {
		if pos+2 > len(body) {
			err = fmt.Errorf("composite glyph: instLen truncated")
			return
		}
		instLen := int(binary.BigEndian.Uint16(body[pos : pos+2]))
		pos += 2
		if pos+instLen > len(body) {
			err = fmt.Errorf("composite glyph: instructions truncated")
			return
		}
		instructions = append([]byte(nil), body[pos:pos+instLen]...)
	}
	return
}

// encodeTriplet picks the smallest triplet encoding for (dx, dy) and
// returns the triplet flagIdx + coord bytes (byteCount-1 bytes).
func encodeTriplet(dx, dy int16) (int, []byte, error) {
	abs := func(v int16) int16 {
		if v < 0 {
			return -v
		}
		return v
	}
	// 2-byte encodings: idx 0-9 (dx==0) and 10-19 (dy==0).
	if dx == 0 {
		ady := abs(dy)
		for k := 0; k <= 4; k++ {
			d := int16(k * 256)
			if ady >= d && ady <= d+255 {
				val := byte(ady - d)
				sign := 0
				if dy >= 0 {
					sign = 1
				}
				return k*2 + sign, []byte{val}, nil
			}
		}
	}
	if dy == 0 && dx != 0 {
		adx := abs(dx)
		for k := 0; k <= 4; k++ {
			d := int16(k * 256)
			if adx >= d && adx <= d+255 {
				val := byte(adx - d)
				sign := 0
				if dx >= 0 {
					sign = 1
				}
				return 10 + k*2 + sign, []byte{val}, nil
			}
		}
	}
	// idx 20-83: 4+4 bits, both axes nonzero, |dx|,|dy| in [1, 64].
	if dx != 0 && dy != 0 && abs(dx) <= 64 && abs(dy) <= 64 {
		adx := int(abs(dx))
		ady := int(abs(dy))
		dxCell := (adx - 1) / 16
		dyCell := (ady - 1) / 16
		si := 0
		if dx > 0 {
			si |= 1
		}
		if dy > 0 {
			si |= 2
		}
		valX := byte(adx - (dxCell*16 + 1))
		valY := byte(ady - (dyCell*16 + 1))
		return 20 + (dyCell*4+dxCell)*4 + si, []byte{(valX << 4) | valY}, nil
	}
	// idx 84-119: 8+8 bits, |dx|,|dy| in [1, 768].
	if dx != 0 && dy != 0 && abs(dx) <= 768 && abs(dy) <= 768 {
		adx := int(abs(dx))
		ady := int(abs(dy))
		dxCell := (adx - 1) / 256
		dyCell := (ady - 1) / 256
		si := 0
		if dx > 0 {
			si |= 1
		}
		if dy > 0 {
			si |= 2
		}
		valX := byte(adx - (dxCell*256 + 1))
		valY := byte(ady - (dyCell*256 + 1))
		return 84 + (dyCell*3+dxCell)*4 + si, []byte{valX, valY}, nil
	}
	// idx 120-123: 12+12 bits packed.
	if abs(dx) <= 4095 && abs(dy) <= 4095 {
		adx := uint32(abs(dx))
		ady := uint32(abs(dy))
		si := 0
		if dx > 0 {
			si |= 1
		}
		if dy > 0 {
			si |= 2
		}
		packed := (adx&0xfff)<<12 | (ady & 0xfff)
		return 120 + si, []byte{byte(packed >> 16), byte(packed >> 8), byte(packed)}, nil
	}
	// idx 124-127: 16+16.
	si := 0
	if dx > 0 {
		si |= 1
	}
	if dy > 0 {
		si |= 2
	}
	adx := uint16(abs(dx))
	ady := uint16(abs(dy))
	buf := make([]byte, 4)
	binary.BigEndian.PutUint16(buf[0:2], adx)
	binary.BigEndian.PutUint16(buf[2:4], ady)
	return 124 + si, buf, nil
}

func decodeCompositeGlyph(c *streamCursors) ([]byte, error) {
	// Composite glyphs always carry an explicit bbox.
	if c.bboxValCur+8 > len(c.bboxValues) {
		return nil, fmt.Errorf("composite bbox truncated")
	}
	xMin := int16(binary.BigEndian.Uint16(c.bboxValues[c.bboxValCur : c.bboxValCur+2]))
	yMin := int16(binary.BigEndian.Uint16(c.bboxValues[c.bboxValCur+2 : c.bboxValCur+4]))
	xMax := int16(binary.BigEndian.Uint16(c.bboxValues[c.bboxValCur+4 : c.bboxValCur+6]))
	yMax := int16(binary.BigEndian.Uint16(c.bboxValues[c.bboxValCur+6 : c.bboxValCur+8]))
	c.bboxValCur += 8

	// Walk component records, copying their bytes verbatim. Track whether
	// any component asks for instructions.
	componentStart := c.compCur
	haveInstructions := false
	for {
		if c.compCur+4 > len(c.compBuf) {
			return nil, fmt.Errorf("composite component header truncated")
		}
		flag := binary.BigEndian.Uint16(c.compBuf[c.compCur : c.compCur+2])
		// flag (2) + glyphIndex (2)
		c.compCur += 4
		argSize := 2
		if flag&cmpArg1And2AreWords != 0 {
			argSize = 4
		}
		scaleSize := 0
		switch {
		case flag&cmpWeHaveATwoByTwo != 0:
			scaleSize = 8
		case flag&cmpWeHaveAnXAndYScale != 0:
			scaleSize = 4
		case flag&cmpWeHaveAScale != 0:
			scaleSize = 2
		}
		need := argSize + scaleSize
		if c.compCur+need > len(c.compBuf) {
			return nil, fmt.Errorf("composite component args truncated")
		}
		c.compCur += need
		if flag&cmpWeHaveInstructions != 0 {
			haveInstructions = true
		}
		if flag&cmpMoreComponents == 0 {
			break
		}
	}
	componentBytes := c.compBuf[componentStart:c.compCur]

	var instructions []byte
	if haveInstructions {
		instLen, n, err := read255UShort(c.glyphBuf[c.glyphCur:])
		if err != nil {
			return nil, fmt.Errorf("composite instLen: %w", err)
		}
		c.glyphCur += n
		if c.instCur+int(instLen) > len(c.instBuf) {
			return nil, fmt.Errorf("composite instructionStream truncated")
		}
		instructions = c.instBuf[c.instCur : c.instCur+int(instLen)]
		c.instCur += int(instLen)
	}

	var out bytes.Buffer
	_ = binary.Write(&out, binary.BigEndian, int16(-1))
	_ = binary.Write(&out, binary.BigEndian, xMin)
	_ = binary.Write(&out, binary.BigEndian, yMin)
	_ = binary.Write(&out, binary.BigEndian, xMax)
	_ = binary.Write(&out, binary.BigEndian, yMax)
	out.Write(componentBytes)
	if haveInstructions {
		_ = binary.Write(&out, binary.BigEndian, uint16(len(instructions)))
		out.Write(instructions)
	}
	return out.Bytes(), nil
}
