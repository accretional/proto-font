package fuzz_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"openformat/internal/fontcodec"
)

func fixtureSeeds(f *testing.F) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "data", "fonts")
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Skip the opt-in google/fonts clone; thousands of seeds
			// slow the fuzz loop to a crawl.
			if filepath.Base(path) == "gfonts" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".ttf", ".otf", ".woff", ".woff2", ".ttc", ".otc":
			if b, rerr := os.ReadFile(path); rerr == nil {
				f.Add(b)
			}
		}
		return nil
	})
}

// FuzzDecode exercises Decode with arbitrary bytes. It must never panic.
func FuzzDecode(f *testing.F) {
	fixtureSeeds(f)
	f.Add([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	f.Add([]byte("wOFF"))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = fontcodec.Decode(b)
	})
}

// FuzzRoundTrip starts from each seed and asserts that after Decode+Encode
// the raw bytes still equal the input. Only runs on inputs that decode
// cleanly — malformed inputs are out of scope for round-trip equality.
func FuzzRoundTrip(f *testing.F) {
	fixtureSeeds(f)
	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := fontcodec.Decode(b)
		if err != nil {
			return
		}
		out, err := fontcodec.Encode(m)
		if err != nil {
			return
		}
		if !bytes.Equal(out, b) {
			t.Fatalf("round-trip mismatch: len got=%d want=%d", len(out), len(b))
		}
	})
}
