package preprocessor

import (
	"testing"

	"github.com/internetarchive/Zeno/internal/pkg/config"
)

func TestHasPathLoop(t *testing.T) {
	config.Set(&config.Config{MaxSegmentRepetition: 3, MaxSegmentRepetitionThreshold: 2})
	tests := []struct {
		name   string
		path   string
		search string
		want   bool
	}{
		// Path segment tests
		{
			name: "normal path",
			path: "/css/styles.css",
			want: false,
		},
		{
			name: "path with 3 repeated segments (at threshold)",
			path: "/a/b/a/b/a/b/file.css",
			want: false, // 3 is the limit, not exceeded
		},
		{
			name: "path with 4 repeated segments (exceeds threshold)",
			path: "/a/b/a/b/a/b/a/b/file.css",
			want: true,
		},
		{
			name: "crawler trap - lms style",
			path: "/theme/styles.php/synergycustom/1770128580/all/DataTables-1.11.3/images/Dats/bootstrap/DataTables-1.11.3/fonts/fonts/fonts/bootstrap/DataTables-1.11.3/images/sort_asc.png",
			want: true,
		},
		{
			name: "single segment repeated 4 times",
			path: "/fonts/fonts/fonts/fonts/file.woff2",
			want: true,
		},
		{
			name: "no path",
			path: "",
			want: false,
		},
		{
			name: "root path only",
			path: "/",
			want: false,
		},
		{
			name: "deep but non-repeating path",
			path: "/a/b/c/d/e/f/g/h/i/j/k.css",
			want: false,
		},
		// Query parameter tests
		{
			name:   "query param repeated 4 times - YouTube style",
			path:   "/watch",
			search: "?v=abc&feature=applinks&feature=applinks&feature=applinks&feature=applinks",
			want:   true,
		},
		{
			name:   "query param repeated 3 times (at threshold)",
			path:   "/watch",
			search: "?v=abc&feature=applinks&feature=applinks&feature=applinks",
			want:   false,
		},
		{
			name:   "different query params not repeated",
			path:   "/page",
			search: "?a=1&b=2&c=3&d=4",
			want:   false,
		},
		{
			name:   "same key different values not flagged",
			path:   "/page",
			search: "?tag=a&tag=b&tag=c&tag=d",
			want:   false,
		},
		{
			name:   "URL with query string - path repeated",
			path:   "/a/a/a/a",
			search: "?page=1",
			want:   true,
		},
		{
			name:   "encoded query params in redirect URL",
			path:   "/Login",
			search: "?continue=https%3A%2F%2Fyoutube.com&feature=applinks&feature=applinks&feature=applinks&feature=applinks",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasPathLoop(tt.path, tt.search)
			if got != tt.want {
				t.Errorf("hasPathLoop(%q, %q) = %v, want %v", tt.path, tt.search, got, tt.want)
			}
		})
	}
}
