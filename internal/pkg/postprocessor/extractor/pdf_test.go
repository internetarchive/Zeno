package extractor

import (
	"bytes"
	_ "embed"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/pkg/models"
)

//go:embed testdata/InternetArchiveDeveloperPortal.pdf
var DeveloperPortalPDF []byte

//go:embed testdata/corrupt.pdf
var CorruptPDF []byte

func TestPDF(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBuffer(DeveloperPortalPDF)),
	}

	var URL = new(models.URL)
	URL.SetResponse(resp)

	err := archiver.ProcessBody(URL, false, false, 0, os.TempDir())
	if err != nil {
		t.Errorf("ProcessBody() error = %v", err)
	}

	start := time.Now()
	outlinks, err := PDF(URL)
	if err != nil {
		t.Error(err)
		return
	}

	want := 19
	if len(outlinks) != want {
		t.Errorf("PDF() got = %v, want %v", len(outlinks), want)
	}
	t.Logf("PDF extraction took %v", time.Since(start))
}

// must fail gracefully with corrupt files.
func TestCorruptPDF(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBuffer(CorruptPDF)),
	}

	var URL = new(models.URL)
	URL.SetResponse(resp)

	err := archiver.ProcessBody(URL, false, false, 0, os.TempDir())
	if err != nil {
		t.Errorf("ProcessBody() error = %v", err)
	}

	outlinks, err := PDF(URL)
	if err == nil {
		t.Error("Corrupt PDF must raise an error")
	}
	if len(outlinks) != 0 {
		t.Error("Cannot extract outlinks from corrupt PDF")
	}
}
