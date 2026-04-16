package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"

	gfaxes "openformat/gen/go/googlefonts/axes"
)

// LoadAxisRegistry walks a directory of per-axis `.textproto` records
// (as shipped by googlefonts/axisregistry) and returns one AxisProto
// per file keyed by the record's tag (e.g. "wght", "CASL"). dataDir
// typically looks like
// `<gfonts-clone>/axisregistry/Lib/axisregistry/data/`.
//
// Siblings that are not `.textproto` files (svg illustrations, README)
// are ignored. Unknown fields in the textproto are tolerated so the
// loader keeps working when upstream adds new axis attributes.
func LoadAxisRegistry(dataDir string) (map[string]*gfaxes.AxisProto, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("metadata: empty dataDir")
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, err
	}
	out := map[string]*gfaxes.AxisProto{}
	opts := prototext.UnmarshalOptions{DiscardUnknown: true}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".textproto") {
			continue
		}
		path := filepath.Join(dataDir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("metadata: read %s: %w", e.Name(), err)
		}
		var ax gfaxes.AxisProto
		if err := opts.Unmarshal(b, &ax); err != nil {
			return nil, fmt.Errorf("metadata: parse %s: %w", e.Name(), err)
		}
		tag := ax.GetTag()
		if tag == "" {
			// Fall back to the filename stem so we still surface the entry.
			tag = strings.TrimSuffix(e.Name(), ".textproto")
		}
		out[tag] = &ax
	}
	return out, nil
}

// AxisRegistryTags returns the sorted set of axis tags present in a
// loaded registry. Handy for assertions in tests and for gfapi helpers
// that want to whitelist known axes.
func AxisRegistryTags(reg map[string]*gfaxes.AxisProto) []string {
	tags := make([]string, 0, len(reg))
	for t := range reg {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}
