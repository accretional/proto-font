package fontcodec

import (
	"bytes"
	"encoding/binary"
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
