package asset_refer_fixer

import (
	"net/http"
	"testing"
)

func TestIsImageURL(t *testing.T) {
	if !IsNeedDelRefer("http://no.referer.test.zeno.local/ab/c.jpg") {
		t.Error("IsImageURL failed")
	}
}

func TestDelRefererHeader(t *testing.T) {
	client := &http.Client{}

	req, _ := http.NewRequest("GET", "https://oscimg.oschina.net/oscnet/48abeebd67e46fb8861fd7f2fff930b5ec5.jpg", nil)

	req.Header.Set("Referer", "https://fake.anotherwebsite.local/")
	DelReferer(req)
	if req.Header.Get("Referer") != "" {
		t.Error("ImageAddHeader failed")
	}

	resp1, _ := client.Do(req)
	if resp1.StatusCode != 200 {
		t.Error("Status code not 200")
	}
	defer resp1.Body.Close()
}
