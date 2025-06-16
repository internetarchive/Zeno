package extractor

import "testing"

func TestCSSURL(t *testing.T) {
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
			inline:        false,
		},
		{
			name:          "bare declaration URL separete CSS",
			CSS:           `url("https://example.com/style.css");`,
			expectedLinks: []string{},
			inline:        false,
			err:           true, // got unexpected token in declaration
		},
		{
			name:          "bare declaration URL inline CSS",
			CSS:           `url("https://example.com/style.css");`,
			expectedLinks: []string{"https://example.com/style.css"},
			inline:        true,
			err:           true, // got unexpected token in declaration
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
			inline:                false,
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
			inline:                false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links, atImportLinks, err := ExtracFromStringCSS(tt.CSS, tt.inline)
			if (err != nil) != tt.err {
				t.Errorf("Expected error %v, got %v", tt.err, err)
			}
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
