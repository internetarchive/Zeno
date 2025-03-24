package extractor

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/pkg/models"
)

func TestHTMLOutlinks(t *testing.T) {
	config.InitConfig()
	body := `
<html>
 <head></head>
 <body>
  <a href="http://example.com">ex</a>
  <a href="http://archive.org">ar</a>
  <p>test</p>
  <a href="https://web.archive.org">wa</a>
 </body>
</html>
`

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBufferString(body)),
	}
	newURL := &models.URL{Raw: "http://ex.com"}
	newURL.SetResponse(resp)
	err := archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
	if err != nil {
		t.Errorf("ProcessBody() error = %v", err)
	}
	item := models.NewItem("test", newURL, "")

	outlinks, err := HTMLOutlinks(item)
	if err != nil {
		t.Errorf("Error extracting HTML outlinks %s", err)
	}

	expectedURLs := map[string]bool{
		"http://example.com":      false,
		"http://archive.org":      false,
		"https://web.archive.org": false,
	}

	for _, link := range outlinks {
		urlString := link.Raw
		if _, exists := expectedURLs[urlString]; exists {
			expectedURLs[urlString] = true
		} else {
			t.Errorf("Unexpected outlink found: %s", urlString)
		}
	}

	for url, found := range expectedURLs {
		if !found {
			t.Errorf("Expected outlink not found: %s", url)
		}
	}
}

// Test <audio> and <video> src extraction
func TestHTMLAssetsAudioVideo(t *testing.T) {
	config.InitConfig()
	audioVideoBody := `
<html>
 <head></head>
 <body>
  <video src="http://f1.com"></video>
  <p>test</p>
  <audio src="http://f2.com"></audio>
 </body>
</html>
`

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBufferString(audioVideoBody)),
	}
	newURL := &models.URL{Raw: "http://ex.com"}
	newURL.SetResponse(resp)
	err := archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
	if err != nil {
		t.Errorf("ProcessBody() error = %v", err)
	}
	item := models.NewItem("test", newURL, "")

	assets, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("HTMLAssets error = %v", err)
	}

	expectedAssets := map[string]bool{
		"http://f1.com": false,
		"http://f2.com": false,
	}

	if len(assets) != len(expectedAssets) {
		t.Errorf("Expected %d assets but got %d", len(expectedAssets), len(assets))
	}

	for _, asset := range assets {
		if _, exists := expectedAssets[asset.Raw]; exists {
			expectedAssets[asset.Raw] = true
		} else {
			t.Errorf("Unexpected asset found: %s", asset.Raw)
		}
	}

	for url, found := range expectedAssets {
		if !found {
			t.Errorf("Expected asset not found: %s", url)
		}
	}
}

// Test [data-item], [style], [data-preview] attribute extraction
func TestHTMLAssetsAttributes(t *testing.T) {
	config.InitConfig()
	html := `
<html>
 <head></head>
 <body>
  <div style="background: url('http://something.com/data.jpg')"></div>
   <div data-preview="http://archive.org">...</div>
  <p>test</p>
  <div data-item='{"id": 123, "name": "Sample Item", "image": "https://example.com/image.jpg"}'>
     Click here for details
  </div>
 </body>
</html>
`

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBufferString(html)),
	}
	newURL := &models.URL{Raw: "http://ex.com"}
	newURL.SetResponse(resp)
	err := archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
	if err != nil {
		t.Errorf("ProcessBody() error = %v", err)
	}
	item := models.NewItem("test", newURL, "")

	assets, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("HTMLAssets error = %v", err)
	}

	expectedAssets := map[string]bool{
		"http://something.com/data.jpg": false,
		"http://archive.org":            false,
		"https://example.com/image.jpg": false,
	}

	if len(assets) != len(expectedAssets) {
		t.Errorf("Expected %d assets but got %d", len(expectedAssets), len(assets))
	}

	for _, asset := range assets {
		if _, exists := expectedAssets[asset.Raw]; exists {
			expectedAssets[asset.Raw] = true
		} else {
			t.Errorf("Unexpected asset found: %s", asset.Raw)
		}
	}

	for url, found := range expectedAssets {
		if !found {
			t.Errorf("Expected asset not found: %s", url)
		}
	}
}

// TestHTMLOutlinksWithNonUTF8Encodings tests the HTMLOutlinks function with various encodings
func TestHTMLOutlinksWithNonUTF8Encodings(t *testing.T) {
	config.InitConfig()

	iso8859_1_cafe := []byte("caf\xE9.com")   // café.com in ISO-8859-1
	windows1252_euro := []byte("\x80uro.com") // €uro.com in Windows-1252

	testCases := []struct {
		name          string
		contentType   string
		body          []byte
		expectedLinks map[string]bool
		expectError   bool // Add expectation for error
	}{
		{
			name:        "UTF-8 Encoding",
			contentType: "text/html; charset=utf-8",
			body:        []byte(`<html><body><a href="http://example.com">Example</a><a href="http://café.com">Café</a></body></html>`),
			expectedLinks: map[string]bool{
				"http://example.com": false,
				"http://café.com":    false,
			},
			expectError: false,
		},
		{
			name:        "ISO-8859-1 Encoding",
			contentType: "text/html; charset=ISO-8859-1",
			body:        []byte(fmt.Sprintf(`<html><body><a href="http://example.com">Example</a><a href="http://%s">Café</a></body></html>`, iso8859_1_cafe)),
			expectedLinks: map[string]bool{
				"http://example.com": false,
				"http://café.com":    false,
			},
			expectError: false,
		},
		{
			name:        "Windows-1252 Encoding",
			contentType: "text/html; charset=windows-1252",
			body:        []byte(fmt.Sprintf(`<html><body><a href="http://example.com">Example</a><a href="http://%s">Euro</a></body></html>`, windows1252_euro)),
			expectedLinks: map[string]bool{
				"http://example.com": false,
				"http://€uro.com":    false,
			},
			expectError: false,
		},
		{
			name:        "Missing Charset in Content-Type",
			contentType: "text/html",
			// ISO-8859-1 encoded HTML with meta charset tag
			body: []byte(fmt.Sprintf(`<html><head><meta http-equiv="Content-Type" content="text/html; charset=ISO-8859-1"></head><body><a href="http://example.com">Example</a><a href="http://%s">Café</a></body></html>`, iso8859_1_cafe)),
			expectedLinks: map[string]bool{
				"http://example.com": false,
				"http://café.com":    false,
			},
			expectError: false,
		},
		{
			name:        "Conflicting Charset",
			contentType: "text/html; charset=utf-8",
			// ISO-8859-1 encoded HTML with meta charset tag that conflicts with Content-Type
			body: []byte(fmt.Sprintf(`<html><head><meta http-equiv="Content-Type" content="text/html; charset=ISO-8859-1"></head><body><a href="http://example.com">Example</a><a href="http://%s">Café</a></body></html>`, iso8859_1_cafe)),
			expectedLinks: map[string]bool{
				"http://example.com": false,
				"http://café.com":    false,
			},
			expectError: false,
		},
		{
			// New test case for HTML5 meta charset tag
			name:        "HTML5 Meta Charset",
			contentType: "text/html; charset=utf-8",
			body:        []byte(fmt.Sprintf(`<html><head><meta charset="ISO-8859-1"></head><body><a href="http://example.com">Example</a><a href="http://%s">Café</a></body></html>`, iso8859_1_cafe)),
			expectedLinks: map[string]bool{
				"http://example.com": false,
				"http://café.com":    false,
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bodyBuffer := bytes.NewBuffer(tc.body)
			resp := &http.Response{
				Body: io.NopCloser(bodyBuffer),
				Header: http.Header{
					"Content-Type": []string{tc.contentType},
				},
			}

			newURL := &models.URL{Raw: "http://ex.com"}
			newURL.SetResponse(resp)
			err := archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
			if err != nil {
				t.Errorf("ProcessBody() error = %v", err)
			}

			// Reset the body by creating a new reader
			resp.Body = io.NopCloser(bytes.NewReader(bodyBuffer.Bytes()))
			item := models.NewItem("test", newURL, "")
			outlinks, err := HTMLOutlinks(item)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("HTMLOutlinks error: %v", err)
			}

			// Check if all expected links are found
			if len(outlinks) != len(tc.expectedLinks) {
				t.Errorf("Expected %d outlinks, got %d", len(tc.expectedLinks), len(outlinks))
				for i, link := range outlinks {
					t.Logf("  Outlink %d: %s", i+1, link.Raw)
				}
			}

			// Mark expected links as found
			for _, link := range outlinks {
				if _, exists := tc.expectedLinks[link.Raw]; exists {
					tc.expectedLinks[link.Raw] = true
				} else {
					t.Errorf("Unexpected outlink found: %s", link.Raw)
				}
			}

			// Check if any expected links are missing
			for url, found := range tc.expectedLinks {
				if !found {
					t.Errorf("Expected outlink not found: %s", url)
				}
			}
		})
	}
}

// TestHTMLAssetsWithNonUTF8Encodings tests the HTMLAssets function with various encodings
func TestHTMLAssetsWithNonUTF8Encodings(t *testing.T) {
	config.InitConfig()

	iso8859_1_cafe := []byte("caf\xE9.com") // café.com in ISO-8859-1

	testCases := []struct {
		name          string
		contentType   string
		body          []byte
		expectedLinks []string
		expectError   bool
	}{
		{
			name:        "Complex HTML with Multiple Assets",
			contentType: "text/html; charset=ISO-8859-1",
			body: []byte(fmt.Sprintf(`
            <html>
            <head>
            <link href="http://example.com/style.css" rel="stylesheet">
            <script src="http://example.com/script.js"></script>
            </head>
            <body>
                <img src="http://example.com/image.jpg">
                <div style="background-image: url('http://%s/bg.jpg')"></div>
                <video src="http://example.com/video.mp4"></video>
                <audio src="http://example.com/audio.mp3"></audio>
                <source srcset="http://example.com/img.jpg 1x, http://example.com/img@2x.jpg 2x">
                <div data-preview="http://example.com/preview.jpg"></div>
            </body>
            </html>`, iso8859_1_cafe)),
			expectedLinks: []string{
				"http://example.com/style.css",
				"http://example.com/script.js",
				"http://example.com/image.jpg",
				"http://café.com/bg.jpg",
				"http://example.com/video.mp4",
				"http://example.com/audio.mp3",
				"http://example.com/img.jpg",
				"http://example.com/img@2x.jpg",
				"http://example.com/preview.jpg",
			},
			expectError: false,
		},
		{
			// Add test case for complex srcset attribute
			name:        "Complex srcset Attribute",
			contentType: "text/html; charset=utf-8",
			body: []byte(`
            <html>
            <head></head>
            <body>
                <img srcset="/image.jpg 480w, 
                            /large.jpg 800w 2x, 
                            /xlarge.jpg 1200w">
                <picture>
                    <source srcset="http://example.com/img-1x.jpg 1x, 
                                   http://example.com/img-2x.jpg 2x, 
                                   http://example.com/img-3x.jpg 3x">
                    <img src="http://example.com/fallback.jpg">
                </picture>
            </body>
            </html>`),
			expectedLinks: []string{
				"/image.jpg",
				"/large.jpg",
				"/xlarge.jpg",
				"http://example.com/img-1x.jpg",
				"http://example.com/img-2x.jpg",
				"http://example.com/img-3x.jpg",
				"http://example.com/fallback.jpg",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bodyBuffer := bytes.NewBuffer(tc.body)
			resp := &http.Response{
				Body: io.NopCloser(bodyBuffer),
				Header: http.Header{
					"Content-Type": []string{tc.contentType},
				},
			}
			newURL := &models.URL{Raw: "http://ex.com"}
			newURL.SetResponse(resp)
			err := archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
			if err != nil {
				t.Errorf("ProcessBody() error = %v", err)
			}

			// Reset the body by creating a new reader
			resp.Body = io.NopCloser(bytes.NewReader(bodyBuffer.Bytes()))
			item := models.NewItem("test", newURL, "")
			assets, err := HTMLAssets(item)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("HTMLAssets error: %v", err)
			}

			// Check that we have the expected number of assets
			if len(assets) != len(tc.expectedLinks) {
				t.Errorf("Expected %d assets, got %d", len(tc.expectedLinks), len(assets))
				for i, asset := range assets {
					t.Logf("  Asset %d: %s", i+1, asset.Raw)
				}
			}

			// Create a map of expected links for easier lookup
			expectedLinksMap := make(map[string]bool)
			for _, link := range tc.expectedLinks {
				expectedLinksMap[link] = false
			}

			// Check that each asset's Raw URL is in our expected list
			for _, asset := range assets {
				assetURL := asset.Raw
				if _, exists := expectedLinksMap[assetURL]; !exists {
					t.Errorf("Unexpected asset URL: %s", assetURL)
				} else {
					expectedLinksMap[assetURL] = true
				}
			}

			// Check that all expected links were found
			for link, found := range expectedLinksMap {
				if !found {
					t.Errorf("Expected asset not found: %s", link)
				}
			}
		})
	}
}

// TestHTMLWithMalformedContent tests the HTML extraction with malformed content
func TestHTMLWithMalformedContent(t *testing.T) {
	config.InitConfig()

	testCases := []struct {
		name        string
		contentType string
		body        []byte
		expectError bool
		comment     string // Add explanations for expected behavior
	}{
		{
			name:        "Malformed HTML - Missing Closing Tags",
			contentType: "text/html; charset=utf-8",
			body:        []byte(`<html><body><a href="http://example.com">Example</a><div>`), // Missing closing div and body tags
			expectError: false,
			comment:     "goquery should handle missing closing tags gracefully",
		},
		{
			name:        "Malformed HTML - Mismatched Tags",
			contentType: "text/html; charset=utf-8",
			body:        []byte(`<html><body><div><p>Text</div></body></html>`),
			expectError: false,
			comment:     "goquery should handle mismatched tags gracefully",
		},
		{
			name:        "Malformed HTML - Unexpected Characters",
			contentType: "text/html; charset=utf-8",
			body:        []byte(`<html <body><a href="http://example.com">Example</a></body></html>`),
			expectError: false,
			comment:     "HTML parsers typically handle unexpected characters within tag boundaries",
		},
		{
			name:        "Invalid Charset",
			contentType: "text/html; charset=invalid-charset",
			body:        []byte(`<html><body><a href="http://example.com">Example</a></body></html>`),
			expectError: false,
			comment:     "Should fall back to UTF-8 when charset is invalid",
		},
		{
			name:        "Malformed HTML - Text with non-ASCII",
			contentType: "text/html",
			body:        []byte("<html><body>This is some text with non-ASCII characters like éèà.</body></html>"),
			expectError: false,
			comment:     "Non-ASCII characters should be handled correctly in UTF-8 encoded content",
		},
		{
			name:        "Truncated Attribute Value",
			contentType: "text/html; charset=utf-8",
			body:        []byte(`<html><body><a href="http://example.com`), // Truncated in the middle of attribute value
			expectError: false,
			comment:     "HTML parser should handle truncated attribute values gracefully",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing case: %s - %s", tc.name, tc.comment)

			bodyBuffer := bytes.NewBuffer(tc.body)
			resp := &http.Response{
				Body: io.NopCloser(bodyBuffer),
				Header: http.Header{
					"Content-Type": []string{tc.contentType},
				},
			}

			newURL := &models.URL{Raw: "http://ex.com"}
			newURL.SetResponse(resp)
			err := archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
			if err != nil {
				// We expect ProcessBody to work even with malformed content
				t.Logf("ProcessBody() error = %v", err)
			}

			// Reset the body by creating a new reader
			resp.Body = io.NopCloser(bytes.NewReader(bodyBuffer.Bytes()))

			item := models.NewItem("test", newURL, "")

			outlinks, err := HTMLOutlinks(item)
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for malformed content but got none")
				} else {
					t.Logf("HTMLOutlinks returned error: %v", err)
				}
			} else if err != nil { // Log unexpected errors
				t.Errorf("HTMLOutlinks returned unexpected error: %v", err)
			} else {
				// Optionally, verify that outlinks were processed correctly
				t.Logf("Found %d outlinks", len(outlinks))
			}

			// Reset the body again for HTMLAssets
			resp.Body = io.NopCloser(bytes.NewReader(bodyBuffer.Bytes()))

			assets, err := HTMLAssets(item)
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for malformed content but got none")
				} else {
					t.Logf("HTMLAssets returned error: %v", err)
				}
			} else if err != nil { // Log unexpected errors.
				t.Errorf("HTMLAssets returned unexpected error: %v", err)
			} else {
				// Optionally, verify that assets were processed correctly
				t.Logf("Found %d assets", len(assets))
			}
		})
	}
}

// TestHTMLOutlinksWithEncoding tests handling of different character encodings
func TestHTMLOutlinksWithEncoding(t *testing.T) {
	config.InitConfig()

	shiftJIS_Japanese := []byte{0x93, 0xfa, 0x96, 0x7b, 0x8c, 0xea} // Japanese text in Shift-JIS
	testCases := []struct {
		name          string
		contentType   string
		body          []byte
		expectedLinks int
		comment       string
	}{
		{
			name:        "ISO-8859-1 Encoding",
			contentType: "text/html; charset=ISO-8859-1",
			// This is an ISO-8859-1 encoded HTML with non-ASCII characters
			body:          []byte("<html><head><meta http-equiv=\"Content-Type\" content=\"text/html; charset=ISO-8859-1\"></head><body><a href=\"http://example.com\">Example</a><a href=\"http://caf\xe9.com\">Caf\xe9</a></body></html>"),
			expectedLinks: 2,
			comment:       "ISO-8859-1 encoding should handle é character correctly",
		},
		{
			name:        "Windows-1252 Encoding",
			contentType: "text/html; charset=windows-1252",
			// This is a Windows-1252 encoded HTML with non-ASCII characters
			body:          []byte("<html><head><meta http-equiv=\"Content-Type\" content=\"text/html; charset=windows-1252\"></head><body><a href=\"http://example.com\">Example</a><a href=\"http://caf\xe9.com\">Caf\xe9</a><a href=\"http://\x80uro.com\">\x80uro</a></body></html>"),
			expectedLinks: 3,
			comment:       "Windows-1252 encoding should handle € (0x80) and é characters correctly",
		},
		{
			name:        "Shift-JIS Encoding",
			contentType: "text/html; charset=Shift_JIS",
			// This is a simple Shift-JIS encoded HTML with Japanese characters
			body:          []byte(fmt.Sprintf("<html><head><meta http-equiv=\"Content-Type\" content=\"text/html; charset=Shift_JIS\"></head><body><a href=\"http://example.com\">%s</a><a href=\"http://japan.com\">Japan</a></body></html>", shiftJIS_Japanese)),
			expectedLinks: 2,
			comment:       "Shift-JIS encoding should handle Japanese characters correctly",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing case: %s - %s", tc.name, tc.comment)

			bodyBuffer := bytes.NewBuffer(tc.body)
			resp := &http.Response{
				Body: io.NopCloser(bodyBuffer),
				Header: http.Header{
					"Content-Type": []string{tc.contentType},
				},
			}

			newURL := &models.URL{Raw: "http://ex.com"}
			newURL.SetResponse(resp)
			err := archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
			if err != nil {
				t.Errorf("ProcessBody() error = %v", err)
			}

			// Reset the body by creating a new reader
			resp.Body = io.NopCloser(bytes.NewReader(bodyBuffer.Bytes()))

			item := models.NewItem("test", newURL, "")

			outlinks, err := HTMLOutlinks(item)
			if err != nil {
				t.Errorf("Error extracting HTML outlinks: %s", err)
			}
			if len(outlinks) != tc.expectedLinks {
				t.Errorf("Expected %d outlinks, got %d", tc.expectedLinks, len(outlinks))
			}

			// Log the found outlinks for debugging
			for i, link := range outlinks {
				t.Logf("Outlink %d: %s", i+1, link.Raw)
			}
		})
	}
}

// TestHTMLAssetsWithEncoding tests handling of assets in different character encodings
func TestHTMLAssetsWithEncoding(t *testing.T) {
	config.InitConfig()

	shiftJIS_Japanese := []byte{0x93, 0xfa, 0x96, 0x7b} // Japanese characters in Shift-JIS
	testCases := []struct {
		name           string
		contentType    string
		body           []byte
		expectedAssets int
		comment        string
	}{
		{
			name:        "ISO-8859-1 Encoding",
			contentType: "text/html; charset=ISO-8859-1",
			// ISO-8859-1 encoded HTML with image tags and style attributes
			body:           []byte("<html><head><meta http-equiv=\"Content-Type\" content=\"text/html; charset=ISO-8859-1\"></head><body><img src=\"http://example.com/image.jpg\"><div style=\"background-image: url('http://caf\xe9.com/bg.jpg')\"></div></body></html>"),
			expectedAssets: 2,
			comment:        "Should extract URLs with ISO-8859-1 encoded characters in style attributes",
		},
		{
			name:        "Windows-1252 Encoding",
			contentType: "text/html; charset=windows-1252",
			// Windows-1252 encoded HTML with script and link tags
			body:           []byte("<html><head><meta http-equiv=\"Content-Type\" content=\"text/html; charset=windows-1252\"><link href=\"http://example.com/style.css\"></head><body><script src=\"http://example.com/script.js\"></script><div data-preview=\"http://\x80uro.com/preview.jpg\"></div></body></html>"),
			expectedAssets: 3,
			comment:        "Should extract URLs with Windows-1252 encoded characters in data attributes",
		},
		{
			name:        "Shift-JIS Encoding",
			contentType: "text/html; charset=Shift_JIS",
			// Shift-JIS encoded HTML with video and source tags
			body:           []byte(fmt.Sprintf("<html><head><meta http-equiv=\"Content-Type\" content=\"text/html; charset=Shift_JIS\"></head><body><video src=\"http://example.com/video.mp4\"></video><source srcset=\"http://%s.com/img.jpg 1x, http://japan.com/img@2x.jpg 2x\"></body></html>", shiftJIS_Japanese)),
			expectedAssets: 3, // video src + 2 srcset images
			comment:        "Should extract URLs with Shift-JIS encoded characters in srcset attributes",
		},
		{
			name:           "Multiple Meta Charset Tags",
			contentType:    "text/html",
			body:           []byte("<html><head><meta charset=\"utf-8\"><meta http-equiv=\"Content-Type\" content=\"text/html; charset=ISO-8859-1\"></head><body><img src=\"http://example.com/image.jpg\"><div style=\"background-image: url('http://caf\xe9.com/bg.jpg')\"></div></body></html>"),
			expectedAssets: 2,
			comment:        "Should prioritize the first charset declaration in meta tags",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bodyBuffer := bytes.NewBuffer(tc.body)
			resp := &http.Response{
				Body: io.NopCloser(bodyBuffer),
				Header: http.Header{
					"Content-Type": []string{tc.contentType},
				},
			}

			newURL := &models.URL{Raw: "http://ex.com"}
			newURL.SetResponse(resp)
			err := archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
			if err != nil {
				t.Errorf("ProcessBody() error = %v", err)
			}

			// Reset the body by creating a new reader
			resp.Body = io.NopCloser(bytes.NewReader(bodyBuffer.Bytes()))

			item := models.NewItem("test", newURL, "")

			assets, err := HTMLAssets(item)
			if err != nil {
				t.Errorf("HTMLAssets error = %v", err)
			}

			t.Logf("Test Case: %s - Found %d assets", tc.name, len(assets))
			for i, asset := range assets {
				t.Logf("  Asset %d: %s", i+1, asset.Raw) // Log each asset using Raw field
			}

			if len(assets) != tc.expectedAssets {
				t.Errorf("Expected %d assets, got %d", tc.expectedAssets, len(assets))
			}
		})
	}
}
