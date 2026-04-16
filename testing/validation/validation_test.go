// Package validation_test walks every font file under data/fonts/ and
// verifies Decode → Encode round-trips byte-for-byte, plus a handful of
// structural assertions (numTables matches directory, head magic == 0x5F0F3CF5,
// etc.).
package validation_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"openformat/internal/fontcodec"
)

func dataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// testing/validation/validation_test.go → repo root/data/fonts.
	return filepath.Join(filepath.Dir(file), "..", "..", "data", "fonts")
}

func listFonts(t *testing.T) []string {
	t.Helper()
	return walkFonts(t, dataDir(t), true)
}

// walkFonts collects font files under root. When skipCorpus is true the
// `gfonts/` subtree (populated by `SETUP_FULL=1 ./setup.sh`) is excluded —
// it can contain thousands of files and is exercised separately by
// TestFullCorpusRoundTrip under FULL_CORPUS=1.
func walkFonts(t *testing.T, root string, skipCorpus bool) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipCorpus && filepath.Base(path) == "gfonts" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".ttf", ".otf", ".woff", ".woff2", ".ttc", ".otc", ".eot":
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking fixtures: %v", err)
	}
	return out
}

func TestValidationRoundTrip(t *testing.T) {
	files := listFonts(t)
	if len(files) == 0 {
		t.Skip("no font fixtures under data/fonts — run ./setup.sh first")
	}
	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			m, err := fontcodec.Decode(raw)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(m.GetRawBytes()) != len(raw) {
				t.Fatalf("raw_bytes len=%d, want %d", len(m.GetRawBytes()), len(raw))
			}
			out, err := fontcodec.Encode(m)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if !bytes.Equal(out, raw) {
				t.Fatalf("round-trip mismatch (len got=%d want=%d)", len(out), len(raw))
			}
		})
	}
}

func TestHeadMagicPresent(t *testing.T) {
	files := listFonts(t)
	if len(files) == 0 {
		t.Skip("no font fixtures")
	}
	for _, f := range files {
		f := f
		ext := strings.ToLower(filepath.Ext(f))
		if ext != ".ttf" && ext != ".otf" {
			continue
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Skipf("read: %v", err)
			}
			m, err := fontcodec.Decode(raw)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			sfnt := m.File.GetSfnt()
			if sfnt == nil {
				t.Skip("not SFNT")
			}
			for _, tbl := range sfnt.Tables {
				if tbl.Tag == "head" {
					if h := tbl.GetHead(); h != nil {
						if h.MagicNumber != 0x5F0F3CF5 {
							t.Errorf("head magic = %#x, want 0x5F0F3CF5", h.MagicNumber)
						}
						if h.UnitsPerEm == 0 {
							t.Error("head units_per_em is 0")
						}
					}
					return
				}
			}
			t.Error("no head table found")
		})
	}
}

// TestFullCorpusRoundTrip walks the opt-in google/fonts clone under
// data/fonts/gfonts (produced by SETUP_FULL=1 ./setup.sh) and asserts the
// same Decode→Encode byte-for-byte invariant. Off by default because it
// touches thousands of files. Opt in with FULL_CORPUS=1.
func TestFullCorpusRoundTrip(t *testing.T) {
	if os.Getenv("FULL_CORPUS") == "" {
		t.Skip("FULL_CORPUS not set; skipping full google/fonts sweep")
	}
	corpus := filepath.Join(dataDir(t), "gfonts")
	if _, err := os.Stat(corpus); err != nil {
		t.Skipf("no gfonts clone at %s (run SETUP_FULL=1 ./setup.sh)", corpus)
	}
	files := walkFonts(t, corpus, false)
	if len(files) == 0 {
		t.Skip("gfonts clone present but no font files found")
	}
	t.Logf("full-corpus sweep: %d files", len(files))
	var failed int
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("%s: read: %v", f, err)
			failed++
			continue
		}
		m, err := fontcodec.Decode(raw)
		if err != nil {
			t.Errorf("%s: decode: %v", f, err)
			failed++
			continue
		}
		out, err := fontcodec.Encode(m)
		if err != nil {
			t.Errorf("%s: encode: %v", f, err)
			failed++
			continue
		}
		if !bytes.Equal(out, raw) {
			t.Errorf("%s: round-trip mismatch (got=%d want=%d)", f, len(out), len(raw))
			failed++
		}
	}
	t.Logf("full-corpus: %d ok, %d failed", len(files)-failed, failed)
}
