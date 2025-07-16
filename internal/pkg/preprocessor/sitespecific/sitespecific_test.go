package sitespecific

import (
	"net/http"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
)

func TestPreprocessor(t *testing.T) {
	targetUrl := "https://www.npr.org/2025/07/16/nx-s1-5468967/npr-pbs-cuts-senate-recission"
	parsedURL, err := models.NewURL(targetUrl)
	if err != nil {
		t.Fatalf("Failed to parsed URL")
	}
	req, err := http.NewRequest("GET", targetUrl, nil)
	if err != nil {
		t.Fatal(err)
	}
	RunPreprocessors(&parsedURL, req)

	// Check if npr preprocessor added HTTP request headers.
	if req.Header.Get("Referer") != "https://www.npr.org/" {
		t.Errorf("HTTP request header wasn't updated %v", req.Header.Get("Referer"))
	}

	targetUrl2 := "https://www.tiktok.com/@otaldosamu2001/video/7291474492210056454?q=lixnu&t=1752663113132"
	parsedURL2, err2 := models.NewURL(targetUrl2)
	if err2 != nil {
		t.Fatalf("Failed to parsed URL")
	}
	req2, err2 := http.NewRequest("GET", targetUrl, nil)
	if err2 != nil {
		t.Fatal(err)
	}
	RunPreprocessors(&parsedURL2, req2)

	// Check if tiktok preprocessor added HTTP request headers.
	if req2.Header.Get("Authority") != "www.tiktok.com" {
		t.Errorf("HTTP request header wasn't updated %v", req2.Header.Get("Authority"))
	}
}
