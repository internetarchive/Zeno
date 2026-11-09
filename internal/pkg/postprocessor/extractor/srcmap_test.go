package extractor

import (
	_ "embed"
	"testing"

	testutil "github.com/internetarchive/Zeno/v2/internal/pkg/postprocessor/extractor/testutil"
	"github.com/internetarchive/Zeno/v2/pkg/models"
)

func TestExtractSourceMapURL(t *testing.T) {
	var tests = []struct {
		name string
		src  string
		want string
	}{
		{
			name: "single-line comment",
			src:  "\n//# sourceMappingURL=main.mjs.map",
			want: "main.mjs.map",
		},
		{
			name: "single-line comment with leading whitespace",
			src:  "\n  //# sourceMappingURL=path-bool.umd.cjs.map",
			want: "path-bool.umd.cjs.map",
		},
		{
			name: "multiline comment",
			src:  "\n/*# sourceMappingURL=primer.css.map*/",
			want: "primer.css.map",
		},
		{
			name: "URL without hash",
			src:  "\n// sourceMappingURL=main.mjs.map",
			want: "",
		},
		{
			name: "URL on a new line inside multiline comment",
			src:  "\n/*# sourceMappingURL=primer.css.map\n */",
			want: "primer.css.map",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSourceMapURL(tt.src)
			if got != tt.want {
				t.Errorf("extractSourceMapURL() = %q, want %q in test %q", got, tt.want, tt.name)
			}
		})
	}
}

//go:embed testdata/css-srcmap.json
var cssSourceMapExample []byte

//go:embed testdata/js-srcmap.json
var jsSourceMapExample []byte

//go:embed testdata/header-srcmap.json
var headerSourceMapExample []byte

func TestSourceMap(t *testing.T) {
	tests := []struct {
		name        string
		expectedURL string
		item        *models.Item
	}{
		{"CSS", "https://www.leagueoflegends.com/_next/static/css/112495035b880485.css.map", testutil.HydrateItem(t, cssSourceMapExample)},
		{"JS", "https://github.githubassets.com/assets/ui-commands-internal-CGtmk2Nh.js.map", testutil.HydrateItem(t, jsSourceMapExample)},
		{"header", "https://en.wikipedia.org/w/load.php?lang=en&modules=startup&only=scripts&skin=vector-2022&sourcemap=1&version=hnw4h", testutil.HydrateItem(t, headerSourceMapExample)},
	}

	extractor := SourceMapExtractor{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assets, _, err := extractor.Extract(tt.item)
			if err != nil {
				t.Errorf("Extraxct() returned err %s", err)
			}
			if len(assets) != 1 {
				t.Errorf("Expected 1, got %d", len(assets))
			}
			if assets[0].Raw != tt.expectedURL {
				t.Errorf("Expected %s got %s", tt.expectedURL, assets[0].Raw)
			}
		})
	}
}
