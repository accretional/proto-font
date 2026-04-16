// Package uivalidation generates per-font HTML samples and matching
// chromerpc automation textprotos so we can drive headless Chrome to
// screenshot every font in data/fonts/.
//
// Usage:
//
//	samples, _ := uivalidation.CollectSamples(rootDir)
//	uivalidation.WriteAssets(samples, outDir)
//
// The test harness in this package (see e2e_test.go) ties CollectSamples
// and WriteAssets together with a local http.FileServer and, when
// UI_E2E=1 is set, shells out to the `chromerpc` CLI.
package uivalidation

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// FontSample describes a single font file we want to render in a
// browser: where it lives on disk, how it should be referenced from a
// stylesheet, and any variable-axis information worth sliding through.
type FontSample struct {
	// AbsPath is the filesystem path to the font binary.
	AbsPath string
	// RelPath is the path relative to the assets dir so both the HTML
	// and the served URL agree.
	RelPath string
	// Family is a human-facing label derived from the filename.
	Family string
	// Format is "truetype" or "opentype" — what goes in @font-face format().
	Format string
	// Variable is true for variable fonts (filename contains [ and ]).
	Variable bool
}

// CollectSamples walks root for supported font files and returns a
// deduplicated, sorted slice of FontSample. Files under `gfonts/` (the
// full google/fonts corpus) are skipped unless includeCorpus is true —
// otherwise the harness would generate thousands of HTML files.
func CollectSamples(root string, includeCorpus bool) ([]FontSample, error) {
	var out []FontSample
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if !includeCorpus && filepath.Base(path) == "gfonts" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		format := ""
		switch ext {
		case ".ttf":
			format = "truetype"
		case ".otf":
			format = "opentype"
		case ".woff":
			format = "woff"
		case ".woff2":
			format = "woff2"
		default:
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		base := strings.TrimSuffix(filepath.Base(path), ext)
		out = append(out, FontSample{
			AbsPath:  path,
			RelPath:  filepath.ToSlash(rel),
			Family:   base,
			Format:   format,
			Variable: strings.ContainsAny(base, "[]"),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out, nil
}

// htmlTmpl renders a sample page using @font-face. We include a
// representative character spread (Latin letters, digits, punctuation)
// plus a paragraph so line-height and metrics are visible. For
// variable fonts we wire up `font-variation-settings` sliders.
var htmlTmpl = template.Must(template.New("sample").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{{.Family}}</title>
<style>
  @font-face {
    font-family: "Sample";
    src: url("/fonts/{{.RelPath}}") format("{{.Format}}");
    font-display: block;
  }
  :root { color-scheme: light; }
  body { margin: 0; padding: 32px; font-family: system-ui, sans-serif;
         background: #fff; color: #111; }
  header { margin-bottom: 24px; font-size: 12px; color: #666;
           font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
  section.sample { font-family: "Sample"; }
  .row { margin: 16px 0; }
  .row.huge   { font-size: 96px; line-height: 1.0; }
  .row.large  { font-size: 48px; line-height: 1.1; }
  .row.medium { font-size: 24px; line-height: 1.3; }
  .row.small  { font-size: 14px; line-height: 1.5; max-width: 720px; }
  .glyphs { font-size: 48px; line-height: 1.2; letter-spacing: 0.05em; }
  .digits { font-size: 32px; letter-spacing: 0.1em;
            font-variant-numeric: tabular-nums; }
</style>
</head>
<body>
  <header>font: {{.Family}} | file: {{.RelPath}} | format: {{.Format}}{{if .Variable}} | variable{{end}}</header>
  <section class="sample">
    <div class="row huge">Hamburgefonstiv</div>
    <div class="row large">The quick brown fox jumps over the lazy dog.</div>
    <div class="row glyphs">ABCDEFGHIJKLMNOPQRSTUVWXYZ</div>
    <div class="row glyphs">abcdefghijklmnopqrstuvwxyz</div>
    <div class="row digits">0123456789 &middot; !?@#$%&amp;*()[]{}</div>
    <div class="row medium">&ldquo;Typography is the craft of endowing human language with a durable visual form.&rdquo; &mdash; Robert Bringhurst</div>
    <div class="row small">Pangrams exercise every letter; this sample also ranges over digits, quotation marks, and common punctuation so we can spot regressions in kerning, metrics, and glyph rendering.</div>
  </section>
</body>
</html>
`))

// WriteHTML writes a sample page for s to w.
func WriteHTML(w io.Writer, s FontSample) error {
	return htmlTmpl.Execute(w, s)
}

// automationTmpl renders a chromerpc AutomationSequence textproto that
// navigates to a URL, waits briefly for @font-face to resolve, and
// writes a PNG screenshot. The schema is described in the chromerpc
// README (https://github.com/accretional/chromerpc).
var automationTmpl = template.Must(template.New("auto").Parse(`# Generated by ui-e2e-validation. Do not edit by hand.
name: "{{.Family}}"
steps {
  label: "viewport"
  set_viewport {
    width: 1280
    height: 800
    device_scale_factor: 2
  }
}
steps {
  label: "navigate"
  navigate { url: "{{.URL}}" }
}
steps {
  label: "settle"
  wait { milliseconds: 500 }
}
steps {
  label: "shot"
  screenshot {
    output_path: "{{.OutputPath}}"
    format: "png"
    full_page: true
  }
}
`))

// AutomationInput is the data bound into the automation template.
type AutomationInput struct {
	Family     string
	URL        string
	OutputPath string
}

// WriteAutomation writes a textproto automation for the given sample.
// url is the URL the headless browser should open; outputPath is where
// the screenshot lands (relative to chromerpc's working directory).
func WriteAutomation(w io.Writer, s FontSample, url, outputPath string) error {
	return automationTmpl.Execute(w, AutomationInput{
		Family:     s.Family,
		URL:        url,
		OutputPath: outputPath,
	})
}

// WriteAssets generates HTML and automation textproto files for each
// sample, under assetsDir/html and assetsDir/automation respectively.
// serveBase is the URL prefix that will expose assetsDir/html over
// HTTP (e.g. "http://127.0.0.1:8080"). screenshotDir is where
// screenshots will land when chromerpc runs.
func WriteAssets(samples []FontSample, assetsDir, serveBase, screenshotDir string) error {
	htmlDir := filepath.Join(assetsDir, "html")
	autoDir := filepath.Join(assetsDir, "automation")
	for _, d := range []string{htmlDir, autoDir, screenshotDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	for _, s := range samples {
		slug := slugify(s.Family)
		htmlPath := filepath.Join(htmlDir, slug+".html")
		if err := writeFile(htmlPath, func(w io.Writer) error { return WriteHTML(w, s) }); err != nil {
			return err
		}
		shotPath := filepath.Join(screenshotDir, slug+".png")
		url := fmt.Sprintf("%s/html/%s.html", strings.TrimRight(serveBase, "/"), slug)
		autoPath := filepath.Join(autoDir, slug+".textproto")
		if err := writeFile(autoPath, func(w io.Writer) error {
			return WriteAutomation(w, s, url, shotPath)
		}); err != nil {
			return err
		}
	}
	return nil
}

func slugify(in string) string {
	var b strings.Builder
	for _, r := range in {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func writeFile(path string, body func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return body(f)
}
