package benchmarks_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"openformat/internal/fontcodec"
)

func allFontFiles() []string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return nil
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "data", "fonts")
	includeCorpus := os.Getenv("FULL_CORPUS") != ""
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if !includeCorpus && filepath.Base(path) == "gfonts" {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".ttf", ".otf", ".woff", ".woff2", ".ttc", ".otc":
			out = append(out, path)
		}
		return nil
	})
	return out
}

func BenchmarkDecode(b *testing.B) {
	files := allFontFiles()
	if len(files) == 0 {
		b.Skip("no fixtures")
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		b.Run(filepath.Base(f), func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			for i := 0; i < b.N; i++ {
				if _, err := fontcodec.Decode(raw); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkEncode(b *testing.B) {
	files := allFontFiles()
	if len(files) == 0 {
		b.Skip("no fixtures")
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		m, err := fontcodec.Decode(raw)
		if err != nil {
			continue
		}
		b.Run(filepath.Base(f), func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			for i := 0; i < b.N; i++ {
				if _, err := fontcodec.Encode(m); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
