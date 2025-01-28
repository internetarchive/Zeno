package extractor

import "testing"

func TestHasFileExtension(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "Simple JPG extension",
			input: "http://example.com/image.jpg",
			want:  true,
		},
		{
			name:  "Query param after extension",
			input: "https://example.org/dog.png?foo=bar",
			want:  true,
		},
		{
			name:  "Fragment after extension",
			input: "https://test.com/cat.gif#section1",
			want:  true,
		},
		{
			name:  "No extension at all",
			input: "http://example.com/foo",
			want:  false,
		},
		{
			name:  "Trailing slash after potential extension",
			input: "http://example.com/foo.txt/",
			want:  false, // The extension is not truly at the end
		},
		{
			name:  "Extension deeper in path",
			input: "http://example.com/data.txt/archive",
			want:  false, // The .txt is not the last segment
		},
		{
			name:  "Multiple dots, multiple segments",
			input: "http://example.net/backups/data.tar.gz?version=2",
			want:  true,
		},
		{
			name:  "Hidden file style, no extension (e.g. .htaccess)",
			input: "https://example.com/.htaccess",
			want:  true,
		},
		{
			name:  "Dot at the end only (no extension)",
			input: "http://example.org/name.",
			want:  false, // There's no extension after the final dot
		},
		{
			name:  "Just a plain filename with extension, no slashes",
			input: "file.zip",
			want:  true,
		},
		{
			name:  "Filename with multiple dots in the last segment",
			input: "https://example.io/some.dir/my.file.name.txt",
			want:  true,
		},
		{
			name:  "Parameters but no dot in final segment",
			input: "https://example.com/paramCheck?this=that",
			want:  false,
		},
		{
			name:  "Multiple slashes near the end",
			input: "http://example.com/dir/subdir/.hidden/",
			want:  false,
		},
		{
			name:  "Dot in subdirectory name only",
			input: "http://example.com/dir.withdot/filename",
			want:  false,
		},
		{
			name:  "Extension is the last item plus fragment",
			input: "http://example.com/test.db#backup",
			want:  true,
		},
		{
			name:  "No slash, no dot, random string",
			input: "thisIsJustAString",
			want:  false,
		},
		{
			name:  "Multiple dots in final segment with a trailing query",
			input: "http://example.com/foo.bar.baz.qux?stuff=1",
			want:  true,
		},
		{
			name:  "Extension disguised with a slash in the query",
			input: "http://example.com/data.zip?path=/etc/passwd",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasFileExtension(tt.input)
			if got != tt.want {
				t.Errorf("hasFileExtension(%q) = %v; want %v", tt.input, got, tt.want)
			}
		})
	}
}
