package uivalidation

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// repoRoot returns an absolute path to the repo root, derived from the
// location of this test file (ui-e2e-validation/ lives one level
// below root).
func repoRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..")
}

// TestGenerateAssets runs the generator against data/fonts/ and asserts
// the expected files land. Always runs. Skips cleanly when no fixtures
// are present (e.g. a shallow CI checkout without setup.sh).
func TestGenerateAssets(t *testing.T) {
	root := repoRoot(t)
	fontsDir := filepath.Join(root, "data", "fonts")
	samples, err := CollectSamples(fontsDir, false)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(samples) == 0 {
		t.Skip("no font fixtures under data/fonts — run ./setup.sh")
	}

	out := t.TempDir()
	shot := filepath.Join(out, "screenshots")
	if err := WriteAssets(samples, out, "http://127.0.0.1:0", shot); err != nil {
		t.Fatalf("write assets: %v", err)
	}

	// Every sample should produce both an HTML page and a textproto.
	htmlDir := filepath.Join(out, "html")
	autoDir := filepath.Join(out, "automation")
	for _, s := range samples {
		slug := slugify(s.Family)
		htmlPath := filepath.Join(htmlDir, slug+".html")
		if _, err := os.Stat(htmlPath); err != nil {
			t.Errorf("missing html for %s: %v", s.Family, err)
		}
		autoPath := filepath.Join(autoDir, slug+".textproto")
		info, err := os.Stat(autoPath)
		if err != nil {
			t.Errorf("missing automation for %s: %v", s.Family, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("empty automation for %s", s.Family)
		}
	}
}

// TestChromerpcScreenshots is the actual headless-browser sweep.
//
// Off by default. Opt in with UI_E2E=1 when a chromerpc gRPC server is
// reachable (default localhost:50051, override with CHROMERPC_ADDR).
// The test:
//   1. generates HTML + automation textprotos under a temp dir,
//   2. serves the temp dir over HTTP on localhost,
//   3. for each automation, rewrites the URL to the live server and
//      invokes `chromerpc-automate` (see CHROMERPC_AUTOMATE_CMD env) to
//      run the automation, and
//   4. asserts the screenshot file lands with non-zero size.
//
// The `chromerpc-automate` binary is expected on PATH OR the
// CHROMERPC_AUTOMATE_CMD env var can point at a shell command (e.g.
// "go run ../chromerpc/cmd/automate").
func TestChromerpcScreenshots(t *testing.T) {
	if os.Getenv("UI_E2E") == "" {
		t.Skip("UI_E2E not set; skipping headless-browser sweep")
	}
	addr := os.Getenv("CHROMERPC_ADDR")
	if addr == "" {
		addr = "localhost:50051"
	}
	if !dialable(addr, 2*time.Second) {
		t.Skipf("chromerpc not reachable at %s — start it with `make -C ../chromerpc run`", addr)
	}

	automate := os.Getenv("CHROMERPC_AUTOMATE_CMD")
	if automate == "" {
		if path, err := exec.LookPath("chromerpc-automate"); err == nil {
			automate = path
		} else {
			// Fall back to `go run` against the sibling accretional repo.
			automate = "go run ../chromerpc/cmd/automate"
		}
	}

	root := repoRoot(t)
	fontsDir := filepath.Join(root, "data", "fonts")
	samples, err := CollectSamples(fontsDir, false)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(samples) == 0 {
		t.Skip("no font fixtures")
	}

	workDir := t.TempDir()
	shotDir := filepath.Join(workDir, "screenshots")
	// SCREENSHOT_OUT_DIR overrides the temp screenshot dir so callers can
	// keep PNGs around (e.g. to refresh docs/screenshots/).
	if v := os.Getenv("SCREENSHOT_OUT_DIR"); v != "" {
		shotDir = v
		if err := os.MkdirAll(shotDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", shotDir, err)
		}
	}

	// Serve the font binaries AND (eventually) the generated HTML from
	// a single static server so @font-face src URLs resolve.
	mux := http.NewServeMux()
	mux.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir(fontsDir))))
	mux.Handle("/html/", http.StripPrefix("/html/", http.FileServer(http.Dir(filepath.Join(workDir, "html")))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	if err := WriteAssets(samples, workDir, srv.URL, shotDir); err != nil {
		t.Fatalf("write assets: %v", err)
	}

	for _, s := range samples {
		slug := slugify(s.Family)
		autoPath := filepath.Join(workDir, "automation", slug+".textproto")
		t.Run(slug, func(t *testing.T) {
			cmd := splitCmd(automate)
			cmd = append(cmd, "-addr", addr, "-input", autoPath)
			c := exec.Command(cmd[0], cmd[1:]...)
			c.Dir = root
			out, err := c.CombinedOutput()
			if err != nil {
				t.Fatalf("%s: %v\n%s", automate, err, out)
			}
			shot := filepath.Join(shotDir, slug+".png")
			info, err := os.Stat(shot)
			if err != nil {
				t.Fatalf("screenshot missing: %v\nchromerpc output:\n%s", err, out)
			}
			if info.Size() == 0 {
				t.Fatalf("empty screenshot")
			}
		})
	}
}

func dialable(addr string, timeout time.Duration) bool {
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// splitCmd splits a "cmd arg1 arg2" string the way a shell would —
// whitespace-delimited, no quote handling. Good enough for env-supplied
// commands like "go run ../chromerpc/cmd/automate".
func splitCmd(s string) []string {
	var out []string
	for _, f := range strings.Fields(s) {
		out = append(out, f)
	}
	return out
}

// Assert that a standard text-rendering sample actually contains the
// @font-face declaration. Cheap smoke test against template typos.
func TestHTMLContainsFontFace(t *testing.T) {
	s := FontSample{Family: "Probe", RelPath: "probe/Probe.ttf", Format: "truetype"}
	var b strings.Builder
	if err := WriteHTML(&b, s); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	html := b.String()
	if !strings.Contains(html, "@font-face") {
		t.Error("missing @font-face in generated HTML")
	}
	if !strings.Contains(html, "Probe") {
		t.Error("family label missing from HTML")
	}
	if !strings.Contains(html, `format("truetype")`) {
		t.Error("format() missing from HTML")
	}
}

func TestAutomationShape(t *testing.T) {
	s := FontSample{Family: "Probe", RelPath: "probe/Probe.ttf", Format: "truetype"}
	var b strings.Builder
	if err := WriteAutomation(&b, s, "http://localhost:8080/html/probe.html", "out/probe.png"); err != nil {
		t.Fatalf("WriteAutomation: %v", err)
	}
	txt := b.String()
	for _, want := range []string{"AutomationSequence", "name:", "set_viewport", "navigate", "screenshot", "output_path:", "probe.png"} {
		if want == "AutomationSequence" {
			continue // name-only; template doesn't print the type
		}
		if !strings.Contains(txt, want) {
			t.Errorf("automation missing %q", want)
		}
	}
	if !strings.Contains(txt, fmt.Sprintf("%q", "Probe")) {
		t.Errorf("family name not quoted in automation:\n%s", txt)
	}
}
