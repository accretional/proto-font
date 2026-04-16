package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pb "openformat/gen/go/openformat/v1"
	fp "openformat/gen/go/googlefonts/fonts_public"
)

// CheckFilenames compares a METADATA.pb's fonts[*].filename list against
// the actual font binaries in the family directory. Returns a
// FamilyFilenameCheck with three lists:
//
//   - resolved: entries declared in METADATA that are present on disk
//   - missing:  declared in METADATA but NOT found
//   - orphan:   .ttf/.otf on disk that no METADATA entry references
//
// Read-only; doesn't touch other files in the directory.
func CheckFilenames(familyDir string, fam *fp.FamilyProto) (*pb.FamilyFilenameCheck, error) {
	if familyDir == "" {
		return nil, fmt.Errorf("metadata: empty familyDir")
	}
	if fam == nil {
		return nil, fmt.Errorf("metadata: nil FamilyProto")
	}
	entries, err := os.ReadDir(familyDir)
	if err != nil {
		return nil, err
	}
	onDisk := map[string]os.DirEntry{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".ttf") || strings.HasSuffix(lower, ".otf") {
			onDisk[name] = e
		}
	}

	out := &pb.FamilyFilenameCheck{FamilyDir: familyDir}
	declared := map[string]bool{}
	for _, f := range fam.GetFonts() {
		name := f.GetFilename()
		if name == "" {
			continue
		}
		declared[name] = true
		e, ok := onDisk[name]
		if !ok {
			out.Missing = append(out.Missing, name)
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("metadata: stat %s: %w", name, err)
		}
		out.Resolved = append(out.Resolved, &pb.ResolvedBinary{
			Filename:  name,
			SizeBytes: info.Size(),
		})
	}
	for name := range onDisk {
		if !declared[name] {
			out.Orphan = append(out.Orphan, name)
		}
	}
	sort.Strings(out.Missing)
	sort.Strings(out.Orphan)
	sort.Slice(out.Resolved, func(i, j int) bool {
		return out.Resolved[i].Filename < out.Resolved[j].Filename
	})
	return out, nil
}

// CheckFamilyDir is a convenience wrapper: load METADATA.pb from the
// directory, parse it, and run the filename cross-check.
func CheckFamilyDir(familyDir string) (*pb.FamilyFilenameCheck, *fp.FamilyProto, error) {
	fam, err := ReadFile(filepath.Join(familyDir, "METADATA.pb"))
	if err != nil {
		return nil, nil, err
	}
	chk, err := CheckFilenames(familyDir, fam)
	if err != nil {
		return nil, fam, err
	}
	return chk, fam, nil
}
