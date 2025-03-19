package extractor

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/CorentinB/warc/pkg/spooledtempfile"
	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/pkg/models"
)

func TestEPUBOutlinks(t *testing.T) {
	config.InitConfig()
	epubFile := "testdata/my_epub.epub"
	epubContent, err := os.ReadFile(epubFile)
	if err != nil {
		t.Fatalf("Failed to read EPUB file: %v", err)
	}

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(epubContent)),
	}
	newURL := &models.URL{Raw: "http://example.com/my_epub.epub"}
	newURL.SetResponse(resp)

	// Create a temporary file and copy the epubContent into it.
	tempFile := spooledtempfile.NewSpooledTempFile("", os.TempDir(), -1, true, 0.5)
	if tempFile == nil {
		t.Fatalf("Failed to create temporary file")
	}
	defer tempFile.Close()
	_, err = io.Copy(tempFile, bytes.NewReader(epubContent))
	if err != nil {
		t.Fatalf("Failed to copy content to temporary file: %v", err)
	}
	newURL.SetBody(tempFile)

	err = archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
	if err != nil {
		t.Errorf("ProcessBody() error = %v", err)
	}
	item := models.NewItem("test", newURL, "")

	outlinks, err := EPUBOutlinks(item)
	if err != nil {
		t.Errorf("Error extracting EPUB outlinks: %v", err)
	}

	expectedOutlinks := []string{
		// "https://api.example.com/data", - This is present inside of a console.log() inside of a <script> tag in a .html file
		// in my_epub.epub. Not sure if this is supposed to be extracted
		"https://example.com/about-this-epub",
		"https://wikipedia.org",
		"https://www.example.com/external-link",
		"https://www.example.org/another-external",
	}

	if len(outlinks) != len(expectedOutlinks) {
		t.Errorf("We couldn't extract all EPUB outlinks. Expected %d, got %d", len(expectedOutlinks), len(outlinks))
		return
	}

	extractedOutlinks := make(map[string]bool)
	for _, outlink := range outlinks {
		extractedOutlinks[outlink.Raw] = true
	}

	for _, expected := range expectedOutlinks {
		if !extractedOutlinks[expected] {
			t.Errorf("Expected outlink not found: %s", expected)
		}
	}
}

func TestEPUBAssets(t *testing.T) {
	config.InitConfig()
	epubFile := "testdata/my_epub.epub"
	epubContent, err := os.ReadFile(epubFile)
	if err != nil {
		t.Fatalf("Failed to read EPUB file: %v", err)
	}

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(epubContent)),
	}
	newURL := &models.URL{Raw: "http://example.com/my_epub.epub"}
	newURL.SetResponse(resp)

	// Create a temporary file and copy the epubContent into it.
	tempFile := spooledtempfile.NewSpooledTempFile("", os.TempDir(), -1, true, 0.5)
	if tempFile == nil {
		t.Fatalf("Failed to create temporary file")
	}
	defer tempFile.Close()
	_, err = io.Copy(tempFile, bytes.NewReader(epubContent))
	if err != nil {
		t.Fatalf("Failed to copy content to temporary file: %v", err)
	}
	newURL.SetBody(tempFile)

	err = archiver.ProcessBody(newURL, false, false, 0, os.TempDir())
	if err != nil {
		t.Errorf("ProcessBody() error = %v", err)
	}
	item := models.NewItem("test", newURL, "")

	assets, err := EPUBAssets(item)
	if err != nil {
		t.Errorf("Error extracting EPUB assets: %v", err)
	}

	expectedAssets := []string{
		"OEBPS/images/chapter1_image.png",
		"OEBPS/images/iframe_image.png",
		"OEBPS/images/image1-large.png",
		"OEBPS/images/image1.png",
		"OEBPS/images/inline_bg.png",
		"OEBPS/images/inline_bg2.jpg",
		"OEBPS/images/inline_div_bg.jpg",
		"OEBPS/images/other_image.png",
		"OEBPS/assets/audio.mp3",
		"OEBPS/assets/video.mp4",
		"OEBPS/assets/font.woff2",
		"OEBPS/styles.css",
	}

	if len(assets) != len(expectedAssets) {
		t.Errorf("We couldn't extract all EPUB assets. Expected %d, got %d", len(expectedAssets), len(assets))
		t.Logf("Expected assets:\n%s", strings.Join(expectedAssets, "\n"))
		extractedAssetsList := make([]string, 0, len(assets))
		for _, asset := range assets {
			extractedAssetsList = append(extractedAssetsList, asset.Raw)
		}
		t.Logf("Extracted assets:\n%s", strings.Join(extractedAssetsList, "\n"))
		return
	}

	extractedAssets := make(map[string]bool)
	for _, asset := range assets {
		extractedAssets[asset.Raw] = true
	}

	for _, expected := range expectedAssets {
		if !extractedAssets[expected] {
			t.Errorf("Expected asset not found: %s", expected)
			t.Logf("Expected assets:\n%s", strings.Join(expectedAssets, "\n"))
			extractedAssetsList := make([]string, 0, len(assets))
			for _, asset := range assets {
				extractedAssetsList = append(extractedAssetsList, asset.Raw)
			}
			t.Logf("Extracted assets:\n%s", strings.Join(extractedAssetsList, "\n"))
		}
	}
}