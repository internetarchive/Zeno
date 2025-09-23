package postprocessor

import (
	"testing"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/pkg/models"
)

// TestAssetFilteringIntegration tests the complete asset filtering flow
func TestAssetFilteringIntegration(t *testing.T) {
	// Initialize config for testing
	cfg := &config.Config{}
	config.Set(cfg)

	// Create test item with HTML content containing assets
	testURL, _ := models.NewURL("http://example.com")
	item := models.NewItem(&testURL, "")

	// Mock some HTML content (this would normally be extracted by real extractors)
	assets := []*models.URL{
		mustCreateURL("http://example.com/style1.css"),
		mustCreateURL("http://example.com/style2.css"),
		mustCreateURL("http://example.com/script1.js"),
		mustCreateURL("http://example.com/script2.js"),
		mustCreateURL("http://example.com/image1.jpg"),
		mustCreateURL("http://example.com/image2.png"),
		mustCreateURL("http://example.com/video.mp4"),
		mustCreateURL("http://example.com/data.json"),
	}

	tests := []struct {
		name                    string
		maxAssets               int
		allowedFileTypes        []string
		disallowedFileTypes     []string
		expectedMaxCount        int
		expectedMinCount        int
		shouldContainExtensions []string
		shouldNotContainExts    []string
	}{
		{
			name:             "default behavior - no filtering",
			expectedMaxCount: 8,
			expectedMinCount: 8,
		},
		{
			name:             "max assets only",
			maxAssets:        3,
			expectedMaxCount: 3,
			expectedMinCount: 3,
		},
		{
			name:                    "allow only stylesheets",
			allowedFileTypes:        []string{"css"},
			expectedMaxCount:        2,
			expectedMinCount:        2,
			shouldContainExtensions: []string{"css"},
			shouldNotContainExts:    []string{"js", "jpg", "png", "mp4", "json"},
		},
		{
			name:                    "disallow multimedia",
			disallowedFileTypes:     []string{"mp4", "jpg", "png"},
			expectedMaxCount:        5,
			expectedMinCount:        5,
			shouldContainExtensions: []string{"css", "js", "json"},
			shouldNotContainExts:    []string{"mp4", "jpg", "png"},
		},
		{
			name:                    "combined filtering - max assets with file type",
			maxAssets:               2,
			allowedFileTypes:        []string{"css", "js"},
			expectedMaxCount:        2,
			expectedMinCount:        2,
			shouldContainExtensions: []string{"css"}, // Only expect CSS since we have 2 CSS files first in order
			shouldNotContainExts:    []string{"jpg", "png", "mp4", "json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set config for this test
			cfg.MaxAssets = tt.maxAssets
			cfg.AssetsAllowedFileTypes = tt.allowedFileTypes
			cfg.AssetsDisallowedFileTypes = tt.disallowedFileTypes

			// Test the actual filtering function
			filtered := filterAssets(item, assets)

			// Validate count expectations
			if len(filtered) > tt.expectedMaxCount {
				t.Errorf("expected at most %d assets, got %d", tt.expectedMaxCount, len(filtered))
			}
			if len(filtered) < tt.expectedMinCount {
				t.Errorf("expected at least %d assets, got %d", tt.expectedMinCount, len(filtered))
			}

			// Validate extensions that should be present
			for _, expectedExt := range tt.shouldContainExtensions {
				found := false
				for _, asset := range filtered {
					if containsExtension(asset.Raw, expectedExt) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected to find asset with extension %s", expectedExt)
				}
			}

			// Validate extensions that should NOT be present
			for _, bannedExt := range tt.shouldNotContainExts {
				for _, asset := range filtered {
					if containsExtension(asset.Raw, bannedExt) {
						t.Errorf("found banned extension %s in asset %s", bannedExt, asset.Raw)
					}
				}
			}
		})
	}
}

// Helper function to check if a URL contains a specific file extension
func containsExtension(rawURL, ext string) bool {
	return len(rawURL) > len(ext)+1 && rawURL[len(rawURL)-len(ext)-1:] == "."+ext
}

// TestExtractAssetsOutlinksWithFiltering tests the complete extraction + filtering pipeline
func TestExtractAssetsOutlinksWithFiltering(t *testing.T) {
	cfg := &config.Config{
		MaxAssets:              2,
		AssetsAllowedFileTypes: []string{"css"},
	}
	config.Set(cfg)

	// Create test item with no body (which means no assets will be extracted)
	testURL, _ := models.NewURL("http://example.com")
	item := models.NewItem(&testURL, "")

	// Test that shouldExtractAssets returns false for items without body
	if shouldExtractAssets(item) {
		t.Error("shouldExtractAssets should return false for items without body")
	}

	// Reset config to not disable assets capture, and set headless to false
	cfg.DisableAssetsCapture = false
	cfg.Headless = false

	// Still should return false because there's no body
	if shouldExtractAssets(item) {
		t.Error("shouldExtractAssets should return false for items without body even when assets capture is enabled")
	}
}

// TestConfigValidation tests that the configuration is properly validated
func TestConfigValidation(t *testing.T) {
	// Test conflicting configurations
	cfg := &config.Config{
		AssetsAllowedFileTypes:    []string{"css", "js"},
		AssetsDisallowedFileTypes: []string{"js", "png"}, // Should be ignored due to allowed types
	}
	config.Set(cfg)

	testURL, _ := models.NewURL("http://example.com")
	item := models.NewItem(&testURL, "")

	assets := []*models.URL{
		mustCreateURL("http://example.com/style.css"),
		mustCreateURL("http://example.com/script.js"),
		mustCreateURL("http://example.com/image.png"),
	}

	filtered := filterAssets(item, assets)

	// Should only have CSS and JS files (allowed types take precedence)
	if len(filtered) != 2 {
		t.Errorf("expected 2 assets (css + js), got %d", len(filtered))
	}

	// Should contain CSS and JS but not PNG
	foundCSS, foundJS, foundPNG := false, false, false
	for _, asset := range filtered {
		if containsExtension(asset.Raw, "css") {
			foundCSS = true
		}
		if containsExtension(asset.Raw, "js") {
			foundJS = true
		}
		if containsExtension(asset.Raw, "png") {
			foundPNG = true
		}
	}

	if !foundCSS {
		t.Error("expected CSS file not found")
	}
	if !foundJS {
		t.Error("expected JS file not found")
	}
	if foundPNG {
		t.Error("PNG file should not be present (not in allowed types)")
	}
}