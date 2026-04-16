package fontcodec

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	pb "openformat/gen/go/openformat/v1"
)

// Minimal valid SFNT with a single fake "TEST" table. We hand-craft this
// because it's small and exercises every header-layout code path.
func buildMinimalSfnt(t *testing.T) []byte {
	t.Helper()
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE}
	// Pad table to multiple of 4 for the *checksum* step; the codec handles
	// tail padding in the file itself.
	padded := append([]byte(nil), payload...)
	for len(padded)%4 != 0 {
		padded = append(padded, 0)
	}
	checksum := uint32(0)
	for i := 0; i < len(padded); i += 4 {
		checksum += binary.BigEndian.Uint32(padded[i : i+4])
	}

	out := make([]byte, 12+16)
	binary.BigEndian.PutUint32(out[0:4], 0x00010000)
	binary.BigEndian.PutUint16(out[4:6], 1)
	binary.BigEndian.PutUint16(out[6:8], 16)
	binary.BigEndian.PutUint16(out[8:10], 0)
	binary.BigEndian.PutUint16(out[10:12], 0)
	// directory entry
	copy(out[12:16], []byte("TEST"))
	binary.BigEndian.PutUint32(out[16:20], checksum)
	binary.BigEndian.PutUint32(out[20:24], 28) // offset
	binary.BigEndian.PutUint32(out[24:28], uint32(len(payload)))
	out = append(out, payload...)
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	return out
}

func TestRoundTripMinimalSfnt(t *testing.T) {
	raw := buildMinimalSfnt(t)
	m, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := m.File.GetSfnt().GetNumTables(); got != 1 {
		t.Errorf("numTables = %d, want 1", got)
	}
	tbl := m.File.GetSfnt().Tables[0]
	if tbl.Tag != "TEST" {
		t.Errorf("tag = %q, want TEST", tbl.Tag)
	}
	out, err := Encode(m)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(out, raw) {
		t.Errorf("round-trip mismatch (raw_bytes path)")
	}
	// Now null raw_bytes to exercise the synthesis path.
	m.RawBytes = nil
	out2, err := Encode(m)
	if err != nil {
		t.Fatalf("Encode (synth): %v", err)
	}
	if !bytes.Equal(out2, raw) {
		t.Errorf("synthesis round-trip mismatch:\n got %x\nwant %x", out2, raw)
	}
}

// TestWOFF2DecodeStructured verifies the WOFF2 decoder populates the
// structured table directory + decompressed per-table bytes (not just the
// header + opaque compressed stream). Driven by the committed
// data/fonts/handwritten/TestWOFF2.woff2 fixture.
func TestWOFF2DecodeStructured(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	path := filepath.Join(repoRoot, "data", "fonts", "handwritten", "TestWOFF2.woff2")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	m, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	w := m.File.GetWoff2()
	if w == nil {
		t.Fatal("no Woff2 body")
	}
	if got := uint32(len(w.TableDirectory)); got != w.NumTables {
		t.Fatalf("TableDirectory len=%d, want NumTables=%d", got, w.NumTables)
	}
	// TestWOFF2.woff2 is a TrueType-flavoured font, so we expect head + glyf.
	tags := map[string]*int{"head": nil, "glyf": nil, "loca": nil}
	for i, e := range w.TableDirectory {
		if e.TagStr == "" || e.Tag == 0 {
			t.Errorf("entry %d missing tag (str=%q raw=%#x)", i, e.TagStr, e.Tag)
		}
		stored := e.OrigLength
		if e.Transformed {
			stored = e.TransformLength
		}
		if uint32(len(e.Data)) != stored {
			t.Errorf("entry %d (%s): data len=%d, want stored=%d",
				i, e.TagStr, len(e.Data), stored)
		}
		if _, want := tags[e.TagStr]; want {
			i := i
			tags[e.TagStr] = &i
		}
	}
	for tag, ptr := range tags {
		if ptr == nil {
			t.Errorf("expected tag %q in directory", tag)
		}
	}
	// glyf and loca should be in transformed form (no transform-version-3
	// fonttools fixture).
	for _, e := range w.TableDirectory {
		if (e.TagStr == "glyf" || e.TagStr == "loca") && !e.Transformed {
			t.Errorf("%s entry not flagged as transformed", e.TagStr)
		}
	}
}

// TestWOFF2GlyfTransformReversal runs reverseWoff2GlyfTransform against
// the transformed glyf bytes carried on the TestWOFF2.woff2 fixture and
// asserts byte-exact match against reference glyf + loca captures
// produced with fonttools.
func TestWOFF2GlyfTransformReversal(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	fixtures := filepath.Join(repoRoot, "data", "fonts", "handwritten")
	raw, err := os.ReadFile(filepath.Join(fixtures, "TestWOFF2.woff2"))
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	wantGlyf, err := os.ReadFile(filepath.Join(fixtures, "TestWOFF2.expected.glyf.bin"))
	if err != nil {
		t.Fatalf("expected glyf: %v", err)
	}
	wantLoca, err := os.ReadFile(filepath.Join(fixtures, "TestWOFF2.expected.loca.bin"))
	if err != nil {
		t.Fatalf("expected loca: %v", err)
	}

	m, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	var glyfData []byte
	for _, e := range m.File.GetWoff2().TableDirectory {
		if e.TagStr == "glyf" {
			glyfData = e.Data
			break
		}
	}
	if glyfData == nil {
		t.Fatal("no glyf entry in woff2 directory")
	}

	gotGlyf, gotLoca, _, err := reverseWoff2GlyfTransform(glyfData)
	if err != nil {
		t.Fatalf("reverseWoff2GlyfTransform: %v", err)
	}
	if !bytes.Equal(gotGlyf, wantGlyf) {
		t.Errorf("glyf mismatch:\n got (%d bytes) %x\nwant (%d bytes) %x",
			len(gotGlyf), gotGlyf, len(wantGlyf), wantGlyf)
	}
	if !bytes.Equal(gotLoca, wantLoca) {
		t.Errorf("loca mismatch:\n got (%d bytes) %x\nwant (%d bytes) %x",
			len(gotLoca), gotLoca, len(wantLoca), wantLoca)
	}

	// And verify the decoder wires reversed bytes onto the entries.
	var glyfEntry, locaEntry *struct{ data []byte }
	for _, e := range m.File.GetWoff2().TableDirectory {
		if e.TagStr == "glyf" {
			glyfEntry = &struct{ data []byte }{e.UntransformedData}
		}
		if e.TagStr == "loca" {
			locaEntry = &struct{ data []byte }{e.UntransformedData}
		}
	}
	if glyfEntry == nil || !bytes.Equal(glyfEntry.data, wantGlyf) {
		t.Errorf("glyf entry untransformed_data not wired")
	}
	if locaEntry == nil || !bytes.Equal(locaEntry.data, wantLoca) {
		t.Errorf("loca entry untransformed_data not wired")
	}
}

// TestHeadCheckSumAdjustmentRecompute verifies that encodeSFNT
// recomputes head.checkSumAdjustment per OpenType §5.head in the
// synthesis path: zeroing the field and summing the whole file must
// equal 0xB1B0AFBA.
func TestHeadCheckSumAdjustmentRecompute(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	path := filepath.Join(repoRoot, "data", "fonts", "noto", "NotoSans-VF.ttf")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	m, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	m.RawBytes = nil
	out, err := Encode(m)
	if err != nil {
		t.Fatalf("Encode synth: %v", err)
	}
	// Locate head table's offset in the re-emitted file.
	s := m.File.GetSfnt()
	if s == nil {
		t.Fatal("no Sfnt body")
	}
	// Parse the output's directory to find head.
	numTables := int(binary.BigEndian.Uint16(out[4:6]))
	var headOff uint32
	for i := 0; i < numTables; i++ {
		base := 12 + 16*i
		tag := string(out[base : base+4])
		if tag == "head" {
			headOff = binary.BigEndian.Uint32(out[base+8 : base+12])
			break
		}
	}
	if headOff == 0 {
		t.Fatal("head table not found in encoded output")
	}
	// Per spec: zero checkSumAdjustment, sum whole file, result must be 0xB1B0AFBA.
	scratch := append([]byte(nil), out...)
	binary.BigEndian.PutUint32(scratch[headOff+8:headOff+12], 0)
	var sum uint32
	for i := 0; i+4 <= len(scratch); i += 4 {
		sum += binary.BigEndian.Uint32(scratch[i : i+4])
	}
	// Handle any trailing unaligned bytes.
	if rem := len(scratch) % 4; rem != 0 {
		var tail [4]byte
		copy(tail[:], scratch[len(scratch)-rem:])
		sum += binary.BigEndian.Uint32(tail[:])
	}
	adj := binary.BigEndian.Uint32(out[headOff+8 : headOff+12])
	if sum+adj != 0xB1B0AFBA {
		t.Errorf("checkSumAdjustment invariant broken: sum=%#x adj=%#x sum+adj=%#x want %#x",
			sum, adj, sum+adj, uint32(0xB1B0AFBA))
	}
}

// TestWOFF2GlyfSynthesisRoundTrip runs the encoder's
// synthesizeWoff2Glyf over the fonttools reference glyf + loca and
// verifies reverseWoff2GlyfTransform returns the same bytes. This is
// the tightest loop for guarding the transform synthesiser.
func TestWOFF2GlyfSynthesisRoundTrip(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	fixtures := filepath.Join(repoRoot, "data", "fonts", "handwritten")
	glyfIn, err := os.ReadFile(filepath.Join(fixtures, "TestWOFF2.expected.glyf.bin"))
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	locaIn, err := os.ReadFile(filepath.Join(fixtures, "TestWOFF2.expected.loca.bin"))
	if err != nil {
		t.Fatalf("expected loca: %v", err)
	}
	// TestWOFF2.woff2 has 6 glyphs and short (indexFormat=0) loca.
	const numGlyphs uint16 = 6
	const indexFormat uint16 = 0
	if want := int(numGlyphs+1) * 2; len(locaIn) != want {
		t.Fatalf("loca sanity: got %d want %d", len(locaIn), want)
	}

	transformed, err := synthesizeWoff2Glyf(glyfIn, locaIn, numGlyphs, indexFormat)
	if err != nil {
		t.Fatalf("synthesizeWoff2Glyf: %v", err)
	}
	glyfOut, locaOut, idxFmt, err := reverseWoff2GlyfTransform(transformed)
	if err != nil {
		t.Fatalf("reverseWoff2GlyfTransform on synth output: %v", err)
	}
	if idxFmt != indexFormat {
		t.Errorf("indexFormat round-trip: got %d want %d", idxFmt, indexFormat)
	}
	if !bytes.Equal(glyfOut, glyfIn) {
		t.Errorf("glyf round-trip mismatch:\n got (%d) %x\nwant (%d) %x",
			len(glyfOut), glyfOut, len(glyfIn), glyfIn)
	}
	if !bytes.Equal(locaOut, locaIn) {
		t.Errorf("loca round-trip mismatch:\n got (%d) %x\nwant (%d) %x",
			len(locaOut), locaOut, len(locaIn), locaIn)
	}
}

// TestWOFF2EncodeRoundTrip decodes the fixture, clears raw_bytes,
// re-encodes via the synthesis path, then re-decodes and verifies
// structural equivalence. Byte-exact is not a goal because brotli is
// non-deterministic across encoders.
func TestWOFF2EncodeRoundTrip(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	path := filepath.Join(repoRoot, "data", "fonts", "handwritten", "TestWOFF2.woff2")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	m, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	m.RawBytes = nil
	out, err := Encode(m)
	if err != nil {
		t.Fatalf("Encode synth: %v", err)
	}
	m2, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode of synth output: %v", err)
	}
	w1 := m.File.GetWoff2()
	w2 := m2.File.GetWoff2()
	if w1 == nil || w2 == nil {
		t.Fatalf("missing Woff2 body after round-trip")
	}
	if w1.NumTables != w2.NumTables {
		t.Fatalf("NumTables: got %d want %d", w2.NumTables, w1.NumTables)
	}
	if w1.Flavor != w2.Flavor {
		t.Errorf("Flavor: got %#x want %#x", w2.Flavor, w1.Flavor)
	}
	origByTag := map[string]*pb.Woff2TableDirectoryEntry{}
	for _, e := range w1.TableDirectory {
		origByTag[e.TagStr] = e
	}
	for _, e := range w2.TableDirectory {
		want, ok := origByTag[e.TagStr]
		if !ok {
			t.Errorf("unexpected tag in re-encoded directory: %s", e.TagStr)
			continue
		}
		if e.OrigLength != want.OrigLength {
			t.Errorf("%s OrigLength: got %d want %d", e.TagStr, e.OrigLength, want.OrigLength)
		}
		if e.Transformed != want.Transformed {
			t.Errorf("%s Transformed: got %v want %v", e.TagStr, e.Transformed, want.Transformed)
		}
		if e.Transformed && e.TagStr != "loca" {
			// Transform byte streams must match: encoder is deterministic
			// for the transform itself (unlike brotli).
			if !bytes.Equal(e.Data, want.Data) {
				t.Errorf("%s transformed data mismatch (len got=%d want=%d)",
					e.TagStr, len(e.Data), len(want.Data))
			}
		}
		if want.Transformed && (want.TagStr == "glyf" || want.TagStr == "loca") {
			if !bytes.Equal(e.UntransformedData, want.UntransformedData) {
				t.Errorf("%s UntransformedData mismatch (len got=%d want=%d)",
					e.TagStr, len(e.UntransformedData), len(want.UntransformedData))
			}
		}
	}
}

// TestCmapSubtableParsing runs decode on a real Noto font and verifies
// its cmap encoding records carry structured ParsedSubtable payloads
// with format-consistent contents. Round-trip byte-exactness is NOT
// affected — subtable_bodies is still the source of truth for Encode.
func TestCmapSubtableParsing(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	path := filepath.Join(repoRoot, "data", "fonts", "noto", "NotoSans-VF.ttf")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	m, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	var cmap *pb.CmapTable
	for _, tb := range m.File.GetSfnt().Tables {
		if tb.Tag == "cmap" {
			cmap = tb.GetCmap()
			break
		}
	}
	if cmap == nil || len(cmap.EncodingRecords) == 0 {
		t.Fatal("no cmap table or no encoding records")
	}
	seenFormats := map[uint32]bool{}
	for i, r := range cmap.EncodingRecords {
		seenFormats[r.SubtableFormat] = true
		switch r.SubtableFormat {
		case 4:
			f4 := r.GetFormat4()
			if f4 == nil {
				t.Errorf("record %d: format 4 not wired", i)
				break
			}
			segCount := int(f4.SegCountX2 / 2)
			if len(f4.EndCode) != segCount || len(f4.StartCode) != segCount ||
				len(f4.IdDelta) != segCount || len(f4.IdRangeOffset) != segCount {
				t.Errorf("record %d fmt4 segment array mismatch: end=%d start=%d delta=%d range=%d want %d",
					i, len(f4.EndCode), len(f4.StartCode), len(f4.IdDelta), len(f4.IdRangeOffset), segCount)
			}
			if segCount > 0 && f4.EndCode[segCount-1] != 0xFFFF {
				t.Errorf("record %d fmt4 last endCode=%#x want 0xFFFF", i, f4.EndCode[segCount-1])
			}
		case 12:
			f12 := r.GetFormat12()
			if f12 == nil {
				t.Errorf("record %d: format 12 not wired", i)
				break
			}
			if len(f12.Groups) == 0 {
				t.Errorf("record %d fmt12 has no groups", i)
			}
			for j, g := range f12.Groups {
				if g.EndCharCode < g.StartCharCode {
					t.Errorf("record %d fmt12 group %d: end<start", i, j)
				}
			}
		case 0, 6, 10, 13, 14:
			// Just confirm something was wired for these rarer formats when present.
			if r.GetParsedSubtable() == nil {
				t.Errorf("record %d: format %d body not wired", i, r.SubtableFormat)
			}
		}
	}
	if !seenFormats[4] {
		t.Error("expected at least one format-4 subtable in NotoSans-VF")
	}
}

func TestDetectFlavor(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		want string
	}{
		{"truetype", []byte{0x00, 0x01, 0x00, 0x00}, "FONT_CONTAINER_TRUETYPE"},
		{"otto", []byte("OTTO"), "FONT_CONTAINER_OPENTYPE_CFF"},
		{"woff1", []byte("wOFF"), "FONT_CONTAINER_WOFF1"},
		{"woff2", []byte("wOF2"), "FONT_CONTAINER_WOFF2"},
		{"ttc", []byte("ttcf"), "FONT_CONTAINER_COLLECTION"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := detectFlavor(tc.head)
			if err != nil {
				t.Fatalf("detectFlavor: %v", err)
			}
			if f.String() != tc.want {
				t.Errorf("got %s, want %s", f.String(), tc.want)
			}
		})
	}
}
