package extractor

import (
	"bytes"
	_ "embed"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	generalarchiver "github.com/internetarchive/Zeno/internal/pkg/archiver/general"
	"github.com/internetarchive/Zeno/pkg/models"
)

//go:embed testdata/InternetArchiveDeveloperPortal.pdf
var DeveloperPortalPDF []byte

//go:embed testdata/corrupt.pdf
var CorruptPDF []byte

func TestPDF(t *testing.T) {
	resp := &http.Response{
		Body:   io.NopCloser(bytes.NewBuffer(DeveloperPortalPDF)),
		Header: make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/pdf")

	var URL = new(models.URL)
	URL.SetResponse(resp)

	err := generalarchiver.ProcessBody(URL, false, false, 0, os.TempDir(), nil)
	if err != nil {
		t.Errorf("ProcessBody() error = %v", err)
	}

	start := time.Now()
	extractor := PDFOutlinkExtractor{}
	outlinks, err := extractor.Extract(URL)
	if err != nil {
		t.Error(err)
		return
	}

	want := 19
	if len(outlinks) != want {
		t.Errorf("PDFOutlinkExtractor.Extract() got = %v, want %v", len(outlinks), want)
	}
	t.Logf("PDF extraction took %v", time.Since(start))
}

// must fail gracefully with corrupt files.
func TestCorruptPDF(t *testing.T) {
	resp := &http.Response{
		Body:   io.NopCloser(bytes.NewBuffer(CorruptPDF)),
		Header: make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/pdf")

	var URL = new(models.URL)
	URL.SetResponse(resp)

	err := generalarchiver.ProcessBody(URL, false, false, 0, os.TempDir(), nil)
	if err != nil {
		t.Errorf("ProcessBody() error = %v", err)
	}

	extractor := PDFOutlinkExtractor{}
	outlinks, err := extractor.Extract(URL)
	if err == nil {
		t.Error("Corrupt PDF must raise an error")
	}
	if len(outlinks) != 0 {
		t.Error("Cannot extract outlinks from corrupt PDF")
	}
}
