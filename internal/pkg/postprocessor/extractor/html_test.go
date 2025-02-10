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
