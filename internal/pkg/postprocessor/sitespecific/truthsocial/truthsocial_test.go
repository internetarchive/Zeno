package truthsocial

import (
	"os"
	"testing"

	"github.com/internetarchive/Zeno/v2/internal/pkg/config"
	"github.com/internetarchive/Zeno/v2/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

// The outlinks found in the statuses API response must be returned
func TestExtractAssetsStatusesOutlinks(t *testing.T) {
	config.Set(&config.Config{})

	body := `{"id": "123", "url": "https://truthsocial.com/@user/123", "media_attachments": []}`

	newURL, err := models.NewURL("https://truthsocial.com/api/v1/statuses/123")
	if err != nil {
		t.Fatalf("error parsing URL: %v", err)
	}
	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write([]byte(body))
	newURL.SetBody(spooledTempFile)

	_, outlinks, err := ExtractAssets(models.NewItem(&newURL, ""))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(outlinks) != 1 || outlinks[0].Raw != "https://truthsocial.com/@user/123" {
		t.Errorf("expected https://truthsocial.com/@user/123 as outlink, got %v", outlinks)
	}
}
