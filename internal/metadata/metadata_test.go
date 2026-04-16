package metadata

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixturesDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "data", "metadata")
}

func TestParseInline(t *testing.T) {
	// Minimal METADATA.pb stripped of `required` fields we don't need for
	// the parser test. prototext with DiscardUnknown accepts inline
	// comments ('# ...').
	src := `name: "Example Sans"
designer: "Example Foundry"
license: "OFL"
# this is a comment
category: "SANS_SERIF"
date_added: "2024-01-02"
fonts {
  name: "Example Sans"
  style: "normal"
  weight: 400
  filename: "ExampleSans-Regular.ttf"
  post_script_name: "ExampleSans-Regular"
  full_name: "Example Sans Regular"
}
axes {
  tag: "wght"
  min_value: 100.0
  max_value: 900.0
}
subsets: "latin"
subsets: "latin-ext"
is_noto: false
`
	fam, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := fam.GetName(); got != "Example Sans" {
		t.Errorf("name = %q", got)
	}
	if len(fam.GetFonts()) != 1 {
		t.Fatalf("fonts = %d, want 1", len(fam.GetFonts()))
	}
	if got := fam.GetFonts()[0].GetFilename(); got != "ExampleSans-Regular.ttf" {
		t.Errorf("fonts[0].filename = %q", got)
	}
	if len(fam.GetAxes()) != 1 || fam.GetAxes()[0].GetTag() != "wght" {
		t.Errorf("axes = %+v", fam.GetAxes())
	}
	if issues := Validate(fam); len(issues) != 0 {
		t.Errorf("unexpected issues: %v", issues)
	}
}

func TestValidateCatchesMissing(t *testing.T) {
	fam, err := Parse([]byte(`name: ""
designer: ""
license: ""
date_added: "not a date"
fonts {
  name: "x"
  style: "normal"
  weight: 400
  filename: ""
  post_script_name: "x"
  full_name: "x"
}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	issues := Validate(fam)
	if len(issues) < 4 {
		t.Errorf("expected at least 4 issues, got %d: %v", len(issues), issues)
	}
}

// TestRealWorldFixtures walks every METADATA.pb fetched by setup.sh and
// asserts that (a) the file parses, (b) Validate returns no issues. Skips
// if setup.sh has not been run.
func TestRealWorldFixtures(t *testing.T) {
	dir := fixturesDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no fixtures directory at %s (run ./setup.sh)", dir)
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".METADATA.pb") {
			continue
		}
		count++
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			fam, err := ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := fam.GetName(); got == "" {
				t.Error("name is empty after parse")
			}
			if got := fam.GetLicense(); got == "" {
				t.Error("license is empty after parse")
			}
			if len(fam.GetFonts()) == 0 {
				t.Error("fonts[] is empty")
			}
			if issues := Validate(fam); len(issues) > 0 {
				t.Errorf("validate: %v", issues)
			}
		})
	}
	if count == 0 {
		t.Skip("no *.METADATA.pb fixtures found; run ./setup.sh")
	}
}

// TestReadFS exercises the directory-walker. Uses the fixture dir from
// setup.sh; skipped when absent.
func TestReadFS(t *testing.T) {
	dir := fixturesDir(t)
	if _, err := os.Stat(dir); err != nil {
		t.Skip("no metadata fixtures")
	}
	got, errs := ReadFS(dir)
	if len(errs) != 0 {
		t.Errorf("parse errors: %v", errs)
	}
	if len(got) == 0 {
		t.Skip("no fixtures present")
	}
	// All fixtures we check in are named `<family>.METADATA.pb`.
	for rel := range got {
		if !strings.HasSuffix(rel, ".METADATA.pb") {
			t.Errorf("unexpected fixture path %q", rel)
		}
	}
}

// TestFullCorpusMetadata parses every METADATA.pb under data/fonts/gfonts
// (the opt-in google/fonts clone). Off by default; set FULL_CORPUS=1 to
// opt in. Useful for catching schema drift we haven't seen locally.
func TestFullCorpusMetadata(t *testing.T) {
	if os.Getenv("FULL_CORPUS") == "" {
		t.Skip("FULL_CORPUS not set; skipping full google/fonts sweep")
	}
	_, file, _, _ := runtime.Caller(0)
	corpus := filepath.Join(filepath.Dir(file), "..", "..", "data", "fonts", "gfonts")
	if _, err := os.Stat(corpus); err != nil {
		t.Skipf("no gfonts clone at %s (run SETUP_FULL=1 ./setup.sh)", corpus)
	}
	got, errs := ReadFS(corpus)
	t.Logf("corpus metadata: %d parsed, %d parse errors", len(got), len(errs))
	for rel, err := range errs {
		t.Errorf("%s: %v", rel, err)
	}
	var issueFiles int
	for rel, fam := range got {
		if issues := Validate(fam); len(issues) > 0 {
			issueFiles++
			if issueFiles <= 10 {
				t.Errorf("%s: %v", rel, issues)
			}
		}
	}
	if issueFiles > 10 {
		t.Errorf("... and %d more files with validation issues", issueFiles-10)
	}
	t.Logf("corpus metadata: %d families with issues", issueFiles)
}
