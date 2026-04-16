package metadata

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// gfontsCorpus returns the path to the full google/fonts clone. Tests
// using it should Skip when the directory is absent (SETUP_FULL=1 not
// run).
func gfontsCorpus(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "data", "fonts", "gfonts")
}

func TestLoadDescriptionPlainText(t *testing.T) {
	html := []byte(`<p>A &amp; B <em>italic</em> &#39;q&#39; <a href="x">link</a>.</p>
<p>More text.</p>`)
	path := filepath.Join(t.TempDir(), "DESCRIPTION.en_us.html")
	if err := os.WriteFile(path, html, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	d, err := LoadDescription(path)
	if err != nil {
		t.Fatalf("LoadDescription: %v", err)
	}
	if d == nil {
		t.Fatal("nil description from existing file")
	}
	if string(d.RawHtml) != string(html) {
		t.Error("raw_html not preserved verbatim")
	}
	// Tag strip + entity decode + whitespace collapse.
	want := "A & B italic 'q' link . More text."
	if d.PlainText != want {
		t.Errorf("plain_text:\n got  %q\n want %q", d.PlainText, want)
	}
}

func TestLoadDescriptionMissingFile(t *testing.T) {
	d, err := LoadDescription(filepath.Join(t.TempDir(), "no-such-file.html"))
	if err != nil {
		t.Errorf("missing file should not error, got: %v", err)
	}
	if d != nil {
		t.Errorf("missing file should return nil, got: %+v", d)
	}
}

// TestLoadDescriptionCorpus reads DESCRIPTION from a real google/fonts
// family directory. Skips when the full corpus isn't present.
func TestLoadDescriptionCorpus(t *testing.T) {
	path := filepath.Join(gfontsCorpus(t), "ofl", "notosans", "DESCRIPTION.en_us.html")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("gfonts clone absent (%v)", err)
	}
	d, err := LoadDescription(path)
	if err != nil {
		t.Fatalf("LoadDescription: %v", err)
	}
	if d == nil {
		t.Fatal("nil description")
	}
	if len(d.RawHtml) == 0 {
		t.Error("raw_html empty")
	}
	if !strings.Contains(d.PlainText, "Noto") {
		t.Errorf("plain_text missing 'Noto': %q", d.PlainText)
	}
}

func TestCheckFilenames(t *testing.T) {
	dir := t.TempDir()
	// Lay down a METADATA.pb declaring two files; put only one of them on
	// disk and add one orphan.
	meta := `name: "X"
designer: "x"
license: "OFL"
date_added: "2024-01-02"
fonts { filename: "Present.ttf" name: "x" style: "normal" weight: 400 post_script_name: "x" full_name: "x" }
fonts { filename: "Missing.ttf" name: "x" style: "normal" weight: 400 post_script_name: "x" full_name: "x" }
`
	if err := os.WriteFile(filepath.Join(dir, "METADATA.pb"), []byte(meta), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Present.ttf"), []byte("fakefont"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Orphan.otf"), []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}
	chk, _, err := CheckFamilyDir(dir)
	if err != nil {
		t.Fatalf("CheckFamilyDir: %v", err)
	}
	if len(chk.Resolved) != 1 || chk.Resolved[0].Filename != "Present.ttf" {
		t.Errorf("resolved = %+v", chk.Resolved)
	}
	if len(chk.Resolved) == 1 && chk.Resolved[0].SizeBytes != int64(len("fakefont")) {
		t.Errorf("resolved size = %d, want %d", chk.Resolved[0].SizeBytes, len("fakefont"))
	}
	if len(chk.Missing) != 1 || chk.Missing[0] != "Missing.ttf" {
		t.Errorf("missing = %v", chk.Missing)
	}
	if len(chk.Orphan) != 1 || chk.Orphan[0] != "Orphan.otf" {
		t.Errorf("orphan = %v", chk.Orphan)
	}
}

// TestCheckFilenamesCorpus runs the filename cross-check against a real
// family directory. Noto Sans should come back clean (no missing, no
// orphans) when the clone is intact.
func TestCheckFilenamesCorpus(t *testing.T) {
	dir := filepath.Join(gfontsCorpus(t), "ofl", "notosans")
	if _, err := os.Stat(filepath.Join(dir, "METADATA.pb")); err != nil {
		t.Skipf("gfonts clone absent (%v)", err)
	}
	chk, fam, err := CheckFamilyDir(dir)
	if err != nil {
		t.Fatalf("CheckFamilyDir: %v", err)
	}
	if fam.GetName() != "Noto Sans" {
		t.Errorf("family name = %q, want 'Noto Sans'", fam.GetName())
	}
	if len(chk.Missing) != 0 {
		t.Errorf("missing binaries in notosans: %v", chk.Missing)
	}
	if len(chk.Resolved) == 0 {
		t.Error("no resolved binaries in notosans")
	}
}

// TestLoadAxisRegistry walks the axisregistry data dir and verifies a
// handful of well-known axes are present with their expected tags.
func TestLoadAxisRegistry(t *testing.T) {
	dataDir := filepath.Join(gfontsCorpus(t),
		"axisregistry", "Lib", "axisregistry", "data")
	if _, err := os.Stat(dataDir); err != nil {
		t.Skipf("axisregistry absent (%v)", err)
	}
	reg, err := LoadAxisRegistry(dataDir)
	if err != nil {
		t.Fatalf("LoadAxisRegistry: %v", err)
	}
	if len(reg) == 0 {
		t.Fatal("empty axis registry")
	}
	// Well-known axes that MUST exist in the registry.
	for _, tag := range []string{"wght", "wdth", "opsz", "slnt", "ital"} {
		if _, ok := reg[tag]; !ok {
			t.Errorf("registry missing %q axis", tag)
		}
	}
	// Spot-check wght values.
	if wght, ok := reg["wght"]; ok {
		if wght.GetMinValue() <= 0 || wght.GetMaxValue() <= wght.GetMinValue() {
			t.Errorf("wght range looks off: min=%v max=%v",
				wght.GetMinValue(), wght.GetMaxValue())
		}
		if wght.GetDisplayName() == "" {
			t.Error("wght display_name empty")
		}
	}
}
