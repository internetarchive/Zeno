package extractor

import (
	"net/http"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
)

func TestShouldMatchM3U8URL(t *testing.T) {
	cases := []struct {
		url      string
		mimeType string
		expected bool
	}{
		{"https://sub.example.com/test.m3u8", "application/vnd.apple.mpegurl", true},
		{"https://sub.example.com/test2.m3u8", "application/x-mpegURL", true}, // will be fixed by PRhttps://github.com/gabriel-vasile/mimetype/pull/755
		{"https://sub.example.com/test3.m3u8", "application/json", false},
		{"https://sub.example.com/example.html", "text/html", false},
		{"https://sub.example.com/m3u8.txt", "text/plain", false},
		{"https://sub.example.com/example.mp4", "application/octet-stream", false},
		{"https://sub.example.com/example.form", "application/x-www-form-urlencoded", false},
	}

	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			url, err := models.NewURL(c.url)
			if err != nil {
				t.Fatalf("failed to create URL: %v", err)
			}
			resp := &http.Response{
				Header:     make(http.Header),
				Body:       nil,
				StatusCode: 200,
			}
			resp.Header.Set("Content-Type", c.mimeType)
			url.SetResponse(resp)

			// call match, returns bool
			matched := M3U8Extractor{}.Match(&url)
			if matched != c.expected {
				t.Errorf("M3U8Extractor.Match(%q) = %v, want %v: mimetype=%q", c.url, matched, c.expected, url.GetMIMEType())
			}
		})
	}
}

// TODO: Add test for Extract()
