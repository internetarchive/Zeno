package extractor

import "testing"

func TestCSSURL(t *testing.T) {
	tests := []struct {
		name          string
		CSS           string
		expectedLinks []string
		inline        bool
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
		},
		{
			name:          "bare declaration URL inline CSS",
			CSS:           `url("https://example.com/style.css");`,
			expectedLinks: []string{"https://example.com/style.css"},
			inline:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := CSS(tt.CSS, tt.inline)
			if len(links) != len(tt.expectedLinks) {
				t.Errorf("Expected %d links, got %d", len(tt.expectedLinks), len(links))
				return
			}
			for i, link := range links {
				if link != tt.expectedLinks[i] {
					t.Errorf("Expected link %s, got %s", tt.expectedLinks[i], link)
				}
			}
		})
	}
}
