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
	item := models.NewItem("test", newURL, "", false)
	config.InitConfig()

	outlinks, err := HTMLOutlinks(item)
	if err != nil {
		t.Errorf("Error extracting HTML outlinks %s", err)
	}
	if len(outlinks) != 3 {
		t.Errorf("We couldn't extract all HTML outlinks.")
	}
}

// Test <audio> and <video> src extraction
func TestHTMLAssetsAudioVideo(t *testing.T) {
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
	item := models.NewItem("test", newURL, "", false)

	assets, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("HTMLAssets error = %v", err)
	}
	if len(assets) != 2 {
		t.Errorf("We couldn't extract all audio/video assets.")
	}
}

// Test [data-item], [style], [data-preview] attribute extraction
func TestHTMLAssetsAttributes(t *testing.T) {
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
	item := models.NewItem("test", newURL, "", false)

	assets, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("HTMLAssets error = %v", err)
	}
	if len(assets) != 3 {
		t.Errorf("We couldn't extract all [data-item], [style], [data-preview] attribute assets. %d", len(assets))
	}
}
