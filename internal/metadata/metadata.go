// Package metadata reads the `METADATA.pb` text-proto files that live in
// each family directory under https://github.com/google/fonts (e.g.
// `ofl/notosans/METADATA.pb`).
//
// These files use Protocol Buffers text format (not JSON) and conform to
// gftools' `fonts_public.proto` schema. We consume the vendored copy of
// that schema from `proto/googlefonts/v1/fonts_public.proto`.
package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"

	fp "openformat/gen/go/googlefonts/fonts_public"
)

// Parse decodes a single METADATA.pb blob. Unknown fields are tolerated
// because the upstream schema evolves without bumping any version — new
// fields appear in real-world files before the .proto catches up.
func Parse(b []byte) (*fp.FamilyProto, error) {
	var fam fp.FamilyProto
	opts := prototext.UnmarshalOptions{
		DiscardUnknown: true,
	}
	if err := opts.Unmarshal(b, &fam); err != nil {
		return nil, fmt.Errorf("metadata: prototext: %w", err)
	}
	return &fam, nil
}

// ReadFile reads and parses a METADATA.pb at the given path.
func ReadFile(path string) (*fp.FamilyProto, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// ReadFS parses every METADATA.pb found under root. The key in the returned
// map is the path relative to root (e.g. `ofl/notosans/METADATA.pb`). Both
// the real google/fonts layout (`.../METADATA.pb`) and a flat fixture
// layout (`notosans.METADATA.pb`) are accepted.
// Parse errors are collected in the second return value so a single bad
// file doesn't stop the walk.
func ReadFS(root string) (map[string]*fp.FamilyProto, map[string]error) {
	out := map[string]*fp.FamilyProto{}
	errs := map[string]error{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if base != "METADATA.pb" && !strings.HasSuffix(base, ".METADATA.pb") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		fam, perr := ReadFile(path)
		if perr != nil {
			errs[rel] = perr
			return nil
		}
		out[rel] = fam
		return nil
	})
	return out, errs
}

// Validate applies a minimal set of invariants on top of protobuf decoding:
//
//   - `name`, `designer`, `license` must be non-empty (they are `required`
//     in the schema but proto3/prototext will happily decode absent
//     required fields; we re-check explicitly).
//   - Every entry in `fonts` must have a `filename` (used downstream to
//     locate the actual binary in the same directory).
//   - `date_added` must parse as YYYY-MM-DD when present.
//
// Returns a slice of issues; empty means the file is structurally sound.
// `Validate` does NOT enforce Font-Bakery-level constraints — that belongs
// in a separate suite.
func Validate(fam *fp.FamilyProto) []string {
	var issues []string
	if fam.GetName() == "" {
		issues = append(issues, "name is empty")
	}
	if fam.GetDesigner() == "" {
		issues = append(issues, "designer is empty")
	}
	if fam.GetLicense() == "" {
		issues = append(issues, "license is empty")
	}
	for i, f := range fam.GetFonts() {
		if f.GetFilename() == "" {
			issues = append(issues, fmt.Sprintf("fonts[%d]: filename is empty", i))
		}
	}
	if d := fam.GetDateAdded(); d != "" {
		if !looksLikeISODate(d) {
			issues = append(issues, fmt.Sprintf("date_added %q is not YYYY-MM-DD", d))
		}
	}
	return issues
}

func looksLikeISODate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	for i, r := range s {
		if i == 4 || i == 7 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

