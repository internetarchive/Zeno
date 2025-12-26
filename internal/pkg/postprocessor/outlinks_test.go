package postprocessor

import (
	_ "embed"
	"os"
	"testing"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

func TestFilterURLsByProtocol(t *testing.T) {
	outlinks := []*models.URL{
		{Raw: "http://example.com"},
		{Raw: "tel:12312313"},
		{Raw: "MAILTO:someone@archive.org"},
		{Raw: "file:/tmp/data.dat"},
	}

	filtered := filterURLsByProtocol(outlinks)
	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered, got %d", len(filtered))
	}
}

func TestFilterMaxOutlinks(t *testing.T) {
	outlinks := []*models.URL{
		{Raw: "http://e1.com"},
		{Raw: "http://e2.com"},
		{Raw: "http://e3.com"},
	}

	config.InitConfig()

	// no limit by default
	outlinks2 := filterMaxOutlinks(outlinks)
	if len(outlinks2) != 3 {
		t.Errorf("expected 3 outlinks and no filtering, got %d", len(outlinks2))
	}

	// set limit = 1
	config.Get().MaxOutlinks = 1
	outlinks3 := filterMaxOutlinks(outlinks)
	if len(outlinks3) != 1 {
		t.Errorf("expected 1 outlink, got %d", len(outlinks3))
	}
}

//go:embed testdata/wikipedia_IA.txt
var wikitext []byte

//go:embed testdata/Q27536592.html.gz
var q27536592HTMLGZ []byte

func TestExtractLinksFromPage(t *testing.T) {
	spf := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	_, _ = spf.Write(wikitext)
	URL := &models.URL{Raw: "https://en.wikipedia.org/wiki/Internet_Archive"}
	URL.SetBody(spf)
	URL.Parse()

	config.InitConfig()
	config.Get().StrictRegex = false
	links := extractLinksFromPage(URL)
	if len(links) != 430 {
		t.Errorf("expected 430 links, got %d", len(links))
	}

	config.Get().StrictRegex = true
	links = extractLinksFromPage(URL)
	if len(links) != 449 {
		t.Errorf("expected 449 links, got %d", len(links))
	}
}

func TestExtractLinksFromPageWithLongLines(t *testing.T) {
	spf := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	_, _ = spf.Write(utils.MustDecompressGzippedBytes(q27536592HTMLGZ))

	URL := &models.URL{Raw: "https://www.wikidata.org/wiki/Q27536592"}
	URL.SetBody(spf)
	URL.Parse()

	config.InitConfig()
	config.Get().StrictRegex = false

	links := extractLinksFromPage(URL)
	if len(links) != 72 {
		t.Errorf("expected 72 links, got %d", len(links))
	}
}

func BenchmarkExtractLinksFromPageRelax(b *testing.B) {
	spf := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	_, _ = spf.Write(wikitext)
	URL := &models.URL{Raw: "https://en.wikipedia.org/wiki/Internet_Archive"}
	URL.SetBody(spf)
	URL.Parse()

	config.InitConfig()
	config.Get().StrictRegex = false

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractLinksFromPage(URL)
	}

	// KiB/s metric
	totalSize := len(wikitext) * b.N
	kiB := float64(totalSize) / 1024.0
	seconds := b.Elapsed().Seconds()
	b.ReportMetric(kiB/seconds, "KiB/s")
}

func BenchmarkExtractLinksFromPageStrict(b *testing.B) {
	spf := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	_, _ = spf.Write(wikitext)
	URL := &models.URL{Raw: "https://en.wikipedia.org/wiki/Internet_Archive"}
	URL.SetBody(spf)
	URL.Parse()

	config.InitConfig()
	config.Get().StrictRegex = true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractLinksFromPage(URL)
	}

	totalSize := len(wikitext) * b.N
	kiB := float64(totalSize) / 1024.0
	seconds := b.Elapsed().Seconds()
	b.ReportMetric(kiB/seconds, "KiB/s")
}
