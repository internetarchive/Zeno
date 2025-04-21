package extractor

import (
	"bytes"
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

	itemURLString := "http://ex.com/test/page1.html"

	newURL := &models.URL{Raw: itemURLString}

	err := newURL.Parse()
	if err != nil {
		t.Fatalf("Failed to parse URL string %s for test item: %v", itemURLString, err)
	}

	parsedReqURL := newURL.GetParsed()

	resp := &http.Response{
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Request:    &http.Request{URL: parsedReqURL},
		StatusCode: 200,
		Header:     make(http.Header),
	}

	newURL.SetResponse(resp)

	err = archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
	if err != nil {
		t.Fatalf("ProcessBody() error = %v", err)
	}

	item := models.NewItem("test", newURL, "")

	outlinks, err := HTMLOutlinks(item)
	if err != nil {
		t.Errorf("Error extracting HTML outlinks: %s", err)
	}

	expectedOutlinks := []string{
		"http://example.com",
		"http://archive.org",
		"https://web.archive.org",
	}

	if len(outlinks) != len(expectedOutlinks) {
		t.Errorf("We couldn't extract all HTML outlinks. Expected %d, got %d", len(expectedOutlinks), len(outlinks))
	} else {
		extractedMap := make(map[string]bool)
		for _, u := range outlinks {
			extractedMap[u.Raw] = true
		}
		for _, expected := range expectedOutlinks {
			if !extractedMap[expected] {
				t.Errorf("Missing expected outlink: %s", expected)
			}
		}
	}
}

func TestHTMLAssetsAudioVideo(t *testing.T) {
	config.InitConfig()
	audioVideoBody := `
  <html>
    <head></head>
    <body>
      <video src="http://f1.com/video.mp4"></video>
      <p>test</p>
      <audio src="/audio.mp3"></audio>
    </body>
  </html>
  `

	itemURLString := "http://ex.com/media/page.html"

	newURL := &models.URL{Raw: itemURLString}

	err := newURL.Parse()
	if err != nil {
		t.Fatalf("Failed to parse URL string %s for test item: %v", itemURLString, err)
	}

	parsedReqURL := newURL.GetParsed()
	resp := &http.Response{
		Body:       io.NopCloser(bytes.NewBufferString(audioVideoBody)),
		Request:    &http.Request{URL: parsedReqURL},
		StatusCode: 200,
		Header:     make(http.Header),
	}

	newURL.SetResponse(resp)

	err = archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
	if err != nil {
		t.Fatalf("ProcessBody() error = %v", err)
	}

	item := models.NewItem("test", newURL, "")

	assets, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("HTMLAssets error = %v", err)
	}

	expectedAssets := []string{
		"http://f1.com/video.mp4",
		"http://ex.com/audio.mp3",
	}

	if len(assets) != len(expectedAssets) {
		t.Errorf("We couldn't extract all audio/video assets. Expected %d, got %d", len(expectedAssets), len(assets))
	} else {
		extractedMap := make(map[string]bool)
		for _, u := range assets {
			extractedMap[u.Raw] = true
		}
		for _, expected := range expectedAssets {
			if !extractedMap[expected] {
				t.Errorf("Missing expected asset: %s", expected)
			}
		}
	}
}

func TestHTMLAssetsAttributes(t *testing.T) {
	config.InitConfig()
	html := `
  <html>
    <head></head>
    <body>
     <div style="background: url('http://something.com/data.jpg')"></div>
     <div data-preview="http://archive.org/preview.png">...</div>
      <p>test</p>
      <div data-item='{"id": 123, "name": "Sample Item", "image": "/images/item_image.jpg"}'>
        Click here for details
      </div>
      <link rel="stylesheet" href="../css/style.css">
    </body>
  </html>
  `

	itemURLString := "http://ex.com/items/detail/page.html"

	newURL := &models.URL{Raw: itemURLString}

	err := newURL.Parse()
	if err != nil {
		t.Fatalf("Failed to parse URL string %s for test item: %v", itemURLString, err)
	}
	
	parsedReqURL := newURL.GetParsed()
	resp := &http.Response{
		Body:       io.NopCloser(bytes.NewBufferString(html)),
		Request:    &http.Request{URL: parsedReqURL},
		StatusCode: 200,
		Header:     make(http.Header),
	}

	newURL.SetResponse(resp)

	err = archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
	if err != nil {
		t.Fatalf("ProcessBody() error = %v", err)
	}

	item := models.NewItem("test", newURL, "")

	assets, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("HTMLAssets error = %v", err)
	}

	expectedAssets := []string{
		"http://something.com/data.jpg",
		"http://archive.org/preview.png",
		"http://ex.com/images/item_image.jpg",
		"http://ex.com/items/css/style.css",
	}

	if len(assets) != len(expectedAssets) {
		t.Errorf("We couldn't extract all assets. Expected %d, got %d", len(expectedAssets), len(assets))
		t.Logf("Extracted assets: %v", assets)
	} else {
		extractedMap := make(map[string]bool)
		for _, u := range assets {
			extractedMap[u.Raw] = true
		}
		for _, expected := range expectedAssets {
			if !extractedMap[expected] {
				t.Errorf("Missing expected asset: %s", expected)
			}
		}
	}
}
