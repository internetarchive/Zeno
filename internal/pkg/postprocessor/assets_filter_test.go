package postprocessor

import (
	"testing"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/pkg/models"
)

func TestFilterAssets(t *testing.T) {
	// Initialize config for testing
	cfg := &config.Config{}
	config.Set(cfg)

	// Create test item
	testURL, _ := models.NewURL("http://example.com")
	item := models.NewItem(&testURL, "")

	// Create test assets with different file types
	assets := []*models.URL{
		mustCreateURL("http://example.com/image.jpg"),
		mustCreateURL("http://example.com/script.js"),
		mustCreateURL("http://example.com/style.css"),
		mustCreateURL("http://example.com/page.html"),
		mustCreateURL("http://example.com/data.json"),
		mustCreateURL("http://example.com/video.mp4"),
	}

	tests := []struct {
		name                       string
		maxAssets                  int
		allowedFileTypes          []string
		disallowedFileTypes       []string
		expectedCount             int
		expectedURLs              []string
	}{
		{
			name:          "no filtering - return all assets",
			maxAssets:     0,
			expectedCount: 6,
		},
		{
			name:          "max assets limit",
			maxAssets:     3,
			expectedCount: 3,
		},
		{
			name:              "allowed file types only",
			allowedFileTypes:  []string{"jpg", "css"},
			expectedCount:     2,
			expectedURLs:      []string{"http://example.com/image.jpg", "http://example.com/style.css"},
		},
		{
			name:                "disallowed file types",
			disallowedFileTypes: []string{"js", "mp4"},
			expectedCount:       4,
		},
		{
			name:              "allowed types override disallowed",
			allowedFileTypes:  []string{"jpg"},
			disallowedFileTypes: []string{"jpg", "css"}, // Should be ignored
			expectedCount:     1,
			expectedURLs:      []string{"http://example.com/image.jpg"},
		},
		{
			name:              "max assets with file type filtering",
			maxAssets:         2,
			allowedFileTypes:  []string{"jpg", "css", "js"},
			expectedCount:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set config for this test
			cfg.MaxAssets = tt.maxAssets
			cfg.AssetsAllowedFileTypes = tt.allowedFileTypes
			cfg.AssetsDisallowedFileTypes = tt.disallowedFileTypes

			// Filter assets
			filtered := filterAssets(item, assets)

			// Check count
			if len(filtered) != tt.expectedCount {
				t.Errorf("expected %d assets, got %d", tt.expectedCount, len(filtered))
			}

			// Check specific URLs if provided
			if len(tt.expectedURLs) > 0 {
				for _, expectedURL := range tt.expectedURLs {
					found := false
					for _, asset := range filtered {
						if asset.Raw == expectedURL {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected URL %s not found in filtered assets", expectedURL)
					}
				}
			}
		})
	}
}

func TestFilterAssetsWithNilAssets(t *testing.T) {
	cfg := &config.Config{}
	config.Set(cfg)

	testURL, _ := models.NewURL("http://example.com")
	item := models.NewItem(&testURL, "")

	// Test with nil assets
	assets := []*models.URL{
		nil,
		mustCreateURL("http://example.com/image.jpg"),
		nil,
		mustCreateURL("http://example.com/script.js"),
		nil,
	}

	cfg.MaxAssets = 0
	cfg.AssetsAllowedFileTypes = []string{"jpg"}

	filtered := filterAssets(item, assets)

	if len(filtered) != 1 {
		t.Errorf("expected 1 asset after filtering, got %d", len(filtered))
	}

	if filtered[0].Raw != "http://example.com/image.jpg" {
		t.Errorf("expected image.jpg, got %s", filtered[0].Raw)
	}
}

func TestFilterAssetsWithInvalidURLs(t *testing.T) {
	cfg := &config.Config{
		AssetsAllowedFileTypes: []string{"jpg"},
	}
	config.Set(cfg)

	testURL, _ := models.NewURL("http://example.com")
	item := models.NewItem(&testURL, "")

	// Create asset with invalid URL
	invalidAsset, _ := models.NewURL("not-a-valid-url")
	
	assets := []*models.URL{
		mustCreateURL("http://example.com/image.jpg"),
		&invalidAsset,
		mustCreateURL("http://example.com/script.js"),
	}

	filtered := filterAssets(item, assets)

	// Should have only the jpg asset (invalid URL has no extension so is filtered out, js is filtered out)
	if len(filtered) != 1 {
		t.Errorf("expected 1 asset after filtering (only jpg), got %d", len(filtered))
	}
	
	if len(filtered) > 0 && filtered[0].Raw != "http://example.com/image.jpg" {
		t.Errorf("expected image.jpg, got %s", filtered[0].Raw)
	}
}

// Helper function to create URLs for testing
func mustCreateURL(rawURL string) *models.URL {
	url, err := models.NewURL(rawURL)
	if err != nil {
		panic(err)
	}
	return &url
}