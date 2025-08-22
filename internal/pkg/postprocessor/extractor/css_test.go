package extractor

import (
	_ "embed"
	"os"
	"testing"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

func TestCSSParser(t *testing.T) {
	tests := []struct {
		name                  string
		CSS                   string
		expectedLinks         []string
		expectedAtImportLinks []string
		inline                bool
		err                   bool
	}{
		{
			name:          "Valid String URL",
			CSS:           `background-image: url("https://example.com/image.png");`,
			expectedLinks: []string{"https://example.com/image.png"},
			inline:        true,
		},
		{
			name:          "Valid Multiple String URLs with Leading and Trailing Spaces",
			CSS:           `background-image: url(  "//example.com/image1.png"), url("//example.com/image2.png"  ); ccc: {--foo: url(  "//example.com/image3.png"   );}`,
			expectedLinks: []string{"//example.com/image1.png", "//example.com/image2.png", "//example.com/image3.png"},
			inline:        true,
		},
		{
			name:          "Valid String URL with Single Quotes",
			CSS:           `background-image: url('//example.com/image.png');`,
			expectedLinks: []string{"//example.com/image.png"},
			inline:        true,
		},
		{
			name:          "Valid URL with No Quotes",
			CSS:           `background-image: url(//example.com/image.png);`,
			expectedLinks: []string{"//example.com/image.png"},
			inline:        true,
		},
		{
			name:          "Valid URL with Escaped HEX Characters",
			CSS:           `background-image: url(   //\ example.com/imag\E9.png  );`,
			expectedLinks: []string{"// example.com/imagé.png"},
			inline:        true,
		},
		{
			name:          "Valid URL with Escaped HEX Characters Followed by Space",
			CSS:           `background-image: url(   //\ example.com/imag\E9 .png  );`,
			expectedLinks: []string{"// example.com/imagé.png"},
			inline:        true,
		},
		{
			name:          "Valid String URL with Escaped Non-HEX Character",
			CSS:           "background-image: url(\"//example.com/image\\.png\");",
			expectedLinks: []string{"//example.com/image.png"},
			inline:        true,
		},
		{
			name:          "Valid String URL with Escaped Newline",
			CSS:           "background-image: url(\"//example.com/image\\\n.png\");",
			expectedLinks: []string{"//example.com/image.png"},
			inline:        true,
		},
		{
			name:          "Valid String URL with Early Escape EOF",
			CSS:           `background-image: url("//example.com/image\`,
			expectedLinks: []string{"//example.com/image"},
			inline:        true,
		},
		{
			name:          "Valid URL with Non-ASCII Characters",
			CSS:           `background-image: url("//example.com/你好.png"), url("//example.com/世界.png");`,
			expectedLinks: []string{"//example.com/你好.png", "//example.com/世界.png"},
			inline:        true,
		},
		{
			// https://developer.mozilla.org/en-US/docs/Web/CSS/@font-face
			name: "Valid Font Face inline CSS",
			CSS: `  font-family: "Trickster";
					src:
						local("Trickster"),
						url("trickster-COLRv1.otf") format("opentype") tech(color-COLRv1),
						url("trickster-outline.otf") format("opentype"),
						url("trickster-outline.woff") format("woff");`,
			expectedLinks: []string{"trickster-COLRv1.otf", "trickster-outline.otf", "trickster-outline.woff"},
			inline:        true,
		},
		{
			name: "Valid Font Face separate CSS",
			CSS: `@font-face {
					font-family: "Trickster";
					src:
						local("Trickster"),
						url("trickster-COLRv1.otf") format("opentype") tech(color-COLRv1),
						url("trickster-outline.otf") format("opentype"),
						url("trickster-outline.woff") format("woff");
					}`,
			expectedLinks: []string{"trickster-COLRv1.otf", "trickster-outline.otf", "trickster-outline.woff"},
		},
		{
			name:          "bare declaration URL separete CSS",
			CSS:           `url("https://example.com/style.css");`,
			expectedLinks: []string{"https://example.com/style.css"},
		},
		{
			name:          "bare declaration URL inline CSS",
			CSS:           `url("https://example.com/style.css");`,
			expectedLinks: []string{"https://example.com/style.css"},
			inline:        true,
		},
		{
			name: "At-Import Rules",
			CSS: `
				/* comment A */
				@charset "UTF-8";
				/* comment B */


				@layer any;
				@layer default, theme, components;
				@import "1.css";
				@import url("2.css");
				@import url("3.css") print;
				@import url("4.css") print, screen;
				@import "5.css" screen;
				/* comment C */
				@import url("6.css") screen and (orientation: landscape);
				@import url("7.css") supports(display: grid) screen and (max-width: 400px);
				@import url("8.css") supports((not (display: grid)) and (display: flex))
				screen and (max-width: 400px);
				@import url("9.css")
				supports((selector(h2 > p)) and (font-tech(color-COLRv1)));

				/* and must not have any other valid at-rules or style rules between it and previous @import rules */
				@layer IBreakAfterImports;
				@import url("invalid.css"); /* this is a invalid @import rule */

				div {
					background-image: url("image.png");
				}`,
			expectedLinks: []string{"image.png"},
			// no "invalid.css" because it's not a valid @import rule
			expectedAtImportLinks: []string{"1.css", "2.css", "3.css", "4.css", "5.css", "6.css", "7.css", "8.css", "9.css"},
		},
		{
			name: "At-Import Rules after layer block",
			CSS: `
				@layer reset {
					audio[controls] {
						display: abc;
					}
				}
				@import "1.css";

				a {
					background-image: url("image.png");
				}`,
			expectedLinks:         []string{"image.png"},
			expectedAtImportLinks: []string{},
		},
		{
			name:          "bare declaration URL at start of a line",
			CSS:           `url("https://example.com/style.css");`,
			expectedLinks: []string{"https://example.com/style.css"},
		},
		{
			name: "Complex CSS",
			CSS: `
				@charset "UTF-8";
				@import "1.css";
				@import uRl("2.css" );
				@import url( "3.css") print;
				@import url(  "4.css"  ) print, screen;
				@import "5.css" screen;
				/* comment C */
				@import url("6.css") screen and (orientation: landscape);
				@import url("7.css") supports(display: grid) screen and (max-width: 400px);
				@import url("8.css") supports((not (display: grid)) and (display: flex))
				screen and (max-width: 400px);
				@import url("9.css")
				supports((selector(h2 > p)) and (font-tech(color-COLRv1)));

				/* and must not have any other valid at-rules or style rules between it and previous @import rules */
				@layer IBreakAfterImports;
				@import url("invalid.css"); /* this is a invalid @import rule */

				div {
					background-image: url("image1.png");
					background-image: uRl(  image2.png  );
					background-image: u\72 l(  i\(mage3.png  );
				}
			`,
			expectedLinks:         []string{"image1.png", "image2.png", "i(mage3.png"},
			expectedAtImportLinks: []string{"1.css", "2.css", "3.css", "4.css", "5.css", "6.css", "7.css", "8.css", "9.css"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links, atImportLinks := ExtractFromStringCSS(tt.CSS, tt.inline)
			if len(links) != len(tt.expectedLinks) {
				t.Errorf("Expected %d links, got %d", len(tt.expectedLinks), len(links))
				return
			}
			if len(atImportLinks) != len(tt.expectedAtImportLinks) {
				t.Errorf("Expected %d at-import links, got %d", len(tt.expectedAtImportLinks), len(atImportLinks))
				return
			}
			for i, link := range links {
				if link != tt.expectedLinks[i] {
					t.Errorf("Expected link %s, got %s", tt.expectedLinks[i], link)
				}
			}
			for i, atImportLink := range atImportLinks {
				if atImportLink != tt.expectedAtImportLinks[i] {
					t.Errorf("Expected at-import link %s, got %s", tt.expectedAtImportLinks[i], atImportLink)
				}
			}
		})
	}
}

//go:embed testdata/font-awesome-all.css.gz
var fontAwesomeCSSGZ []byte

func BenchmarkExtractFromURLCSS(b *testing.B) {
	url := &models.URL{
		Raw: "http://test.css",
	}
	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write(utils.MustDecompressGzippedBytes(fontAwesomeCSSGZ))
	url.SetBody(spooledTempFile)
	url.Parse()
	started := time.Now()

	for b.Loop() {
		r1, r2, err := ExtractFromURLCSS(url)
		if err != nil {
			b.Errorf("Error extracting CSS: %v", err)
		}
		if len(r1) != 18 {
			b.Errorf("Expected 18 links, got %d", len(r1))
		}
		if len(r2) != 0 {
			b.Errorf("Expected 0 at-import links, got %d", len(r2))
		}
	}
	b.StopTimer()

	totalBytes := len(utils.MustDecompressGzippedBytes(fontAwesomeCSSGZ)) * b.N
	elapsed := time.Since(started)
	totalKiB := totalBytes / 1024
	b.ReportMetric(float64(totalKiB)/elapsed.Seconds(), "kB/s")
}
