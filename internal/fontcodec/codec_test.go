package fontcodec

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
