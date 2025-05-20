package postprocessor

import (
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
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
