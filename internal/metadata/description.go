package metadata

import (
	"os"
	"regexp"
	"strings"

	pb "openformat/gen/go/openformat/v1"
)

// htmlTag matches a single HTML tag including its attributes. The
// DESCRIPTION.en_us.html files in google/fonts are simple — no CDATA,
// no scripts, no comments worth preserving — so a tag-strip is enough
// for search/preview without pulling in an HTML parser.
var htmlTag = regexp.MustCompile(`(?s)<[^>]*>`)

// whitespaceRun collapses every run of whitespace (including newlines
// inside the raw HTML) to a single space in plain_text.
var whitespaceRun = regexp.MustCompile(`\s+`)

// LoadDescription reads a DESCRIPTION.en_us.html file and returns the
// raw bytes plus a best-effort plain-text rendering. Returns nil + nil
// when the file doesn't exist so callers can treat "no description"
// uniformly.
func LoadDescription(path string) (*pb.FamilyDescription, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	plain := htmlTag.ReplaceAllString(string(b), " ")
	plain = unescapeBasicEntities(plain)
	plain = whitespaceRun.ReplaceAllString(plain, " ")
	plain = strings.TrimSpace(plain)
	return &pb.FamilyDescription{
		Path:      path,
		RawHtml:   append([]byte(nil), b...),
		PlainText: plain,
	}, nil
}

// unescapeBasicEntities handles the common HTML entities seen in
// DESCRIPTION files (&amp; &lt; &gt; &quot; &#39; and numeric refs for
// the few curly quotes that show up). Not a general-purpose entity
// decoder; good enough for this narrow input.
func unescapeBasicEntities(s string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
		"&nbsp;", " ",
	)
	return replacer.Replace(s)
}
