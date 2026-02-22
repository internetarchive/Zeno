package ina

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	_ "embed"

	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

func TestShouldMatchINAAPIURL(t *testing.T) {
	cases := []struct {
		url      string
		expected bool
	}{
		{"https://apipartner.ina.fr/assets/CAF94008902?sign=cba193fb1b0f088093cb933e551aeb57ca05aad8&partnerId=2", true},
		{"https://apipartner.ina.fr/partners/2/playerConfigurations.json", false},
		{"https://apipartner.ina.fr/assets/LXF01009183?sign=7b65ad9249d3727e8764f4fa98bbca900ba40dfc&partnerId=2", true},
		{"https://apipartner.ina.fr/partners/2/playerConfigurations.json", false},
		{"https://apipartner.ina.fr/assets/CPF86631442?sign=1c81c809cbc94b3472083f3c416c7f6e3c36a7fc&partnerId=2", true},
		{"https://apipartner.ina.fr/partners/2/playerConfigurations.json", false},
	}

	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			url, err := models.NewURL(c.url)
			if err != nil {
				t.Fatalf("failed to create URL: %v", err)
			}

			result := INAExtractor{}.Match(&url)
			if result != c.expected {
				t.Errorf("INAExtractor{}.Match(%q) = %v; want %v", c.url, result, c.expected)
			}
		})
	}

}

//go:embed testdata/ina.json
var rawJson []byte

func TestShouldExtractINAAPIURL(t *testing.T) {
	resp := &http.Response{
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBuffer(rawJson)),
		StatusCode: 200,
	}
	resp.Header.Set("Content-Type", "application/json")

	newURL, err := models.NewURL("https://apipartner.ina.fr/assets/CAF94008902?sign=cba193fb1b0f088093cb933e551aeb57ca05aad8&partnerId=2")
	if err != nil {
		t.Fatalf("failed to create URL: %v", err)
	}
	newURL.SetResponse(resp)

	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write(rawJson)

	newURL.SetBody(spooledTempFile)
	newURL.Parse()
	item := models.NewItem(&newURL, "")

	assets, _, err := INAExtractor{}.Extract(item)
	if err != nil {
		t.Fatalf("failed to extract assets: %v", err)
	}

	// check if urls are within assets
	expectedURLs := []string{
		"https://media-hub.ina.fr/video/TCTrhfFxdPsK05aEtOQZo21h9bs5knovVz3UlDlUiWugpJXwPaH2M5LEPFcMBUv2sxqpql3WsZx9UAKUZlxACWSXWW3DfDyGEPc2GrizPOCYemJ1BALeMMoG1HdyQ9G6flO2sauFpJQI1a8zkeUNWw==/sl_iv/2m+jStBk7YTkGuubVRRfcQ==/sl_hm/yu3+FJDLtzlrVZB7d60TIwYkjEOIRNk6NipAM252ri57pvRHRORKX58Bs1grMqCqTRrx81t936vdZ62TX+jXAA==/sl_e/1771006331",
		"https://cdn-hub.ina.fr/notice/690x517/09b/CPF86631442.jpeg",
		"https://player.ina.fr/embed/CPF86631442?pid=1c81c809cbc94b3472083f3c416c7f6e3c36a7fc&sign=c9cb04647e70ee3baaede8e4017131e827bcb496",
		"https://www.ina.fr/video/CPF86631442",
	}
	assetSet := make(map[string]struct{}, len(assets))
	for _, a := range assets {
		assetSet[a.Raw] = struct{}{}
	}

	for _, expected := range expectedURLs {
		if _, ok := assetSet[expected]; !ok {
			t.Errorf("expected URL not found in assets: %s", expected)
		}
	}

}
