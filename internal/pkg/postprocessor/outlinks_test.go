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
	var outlinks []*models.URL
	outlinks = append(outlinks, &models.URL{Raw: "http://example.com"})
	// skipped
	outlinks = append(outlinks, &models.URL{Raw: "tel:12312313"})
	outlinks = append(outlinks, &models.URL{Raw: "MAILTO:someone@archive.org"})
	outlinks = append(outlinks, &models.URL{Raw: "file:/tmp/data.dat"})

	filtered := filterURLsByProtocol(outlinks)

	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered, got %d", len(filtered))
	}
}

func TestFilterMaxOutlinks(t *testing.T) {
	var outlinks []*models.URL
	outlinks = append(outlinks, &models.URL{Raw: "http://e1.com"})
	outlinks = append(outlinks, &models.URL{Raw: "http://e2.com"})
	outlinks = append(outlinks, &models.URL{Raw: "http://e3.com"})

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
var wikitext []byte // CC BY-SA 4.0

//go:embed testdata/Q27536592.html.gz
var q27536592HTMLGZ []byte // CC BY-SA 4.0

func TestExtractLinksFromPage(t *testing.T) {
	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write(wikitext)
	URL := &models.URL{Raw: "https://en.wikipedia.org/wiki/Internet_Archive"}
	URL.SetBody(spooledTempFile)
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

// https://github.com/internetarchive/Zeno/issues/413
//
// There are 2 lines in the HTML that are longer than 64KiB, overflowing the default bufio.Scanner buffer size if we use line-by-line reading.
func TestExtractLinksFromPageWithLongLines(t *testing.T) {
	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write(utils.MustDecompressGzippedBytes(q27536592HTMLGZ))

	URL := &models.URL{Raw: "https://www.wikidata.org/wiki/Q27536592"}
	URL.SetBody(spooledTempFile)
	URL.Parse()

	config.InitConfig()
	config.Get().StrictRegex = false

	links := extractLinksFromPage(URL)
	if len(links) != 72 {
		t.Errorf("expected 72 links, got %d", len(links))
	}
}

func BenchmarkExtractLinksFromPageRelax(b *testing.B) {
	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write(wikitext)
	URL := &models.URL{Raw: "https://en.wikipedia.org/wiki/Internet_Archive"}
	URL.SetBody(spooledTempFile)
	URL.Parse()

	config.InitConfig()
	config.Get().StrictRegex = false

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractLinksFromPage(URL)
	}

	// add metric: KiB/s
	totalSize := len(wikitext) * b.N
	kiB := float64(totalSize) / 1024.0
	seconds := b.Elapsed().Seconds()
	kiBPerSecond := kiB / seconds
	b.ReportMetric(kiBPerSecond, "KiB/s")
}

func BenchmarkExtractLinksFromPageStrict(b *testing.B) {
	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write(wikitext)
	URL := &models.URL{Raw: "https://en.wikipedia.org/wiki/Internet_Archive"}
	URL.SetBody(spooledTempFile)
	URL.Parse()

	config.InitConfig()
	config.Get().StrictRegex = true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractLinksFromPage(URL)
	}

	// add metric: KiB/s
	totalSize := len(wikitext) * b.N
	kiB := float64(totalSize) / 1024.0
	seconds := b.Elapsed().Seconds()
	kiBPerSecond := kiB / seconds
	b.ReportMetric(kiBPerSecond, "KiB/s")
}
