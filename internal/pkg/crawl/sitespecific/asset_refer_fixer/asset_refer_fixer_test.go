package asset_refer_fixer

import (
	"net/http"
	"testing"
)

func TestIsNeedDelRefer(t *testing.T) {
	if !IsNeedDelRefer("http://no.referer.test.zeno.local/ab/c.jpg") {
		t.Error("IsNeedDelRefer failed")
	}

	if IsNeedDelRefer("https://donot-delete.referer.test.zeno.local/test.jpg") {
		t.Error("IsNeedDelRefer failed")
	}
}
func TestDelRefererHeader(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://no.referer.test.zeno.local/test.jpg", nil)
	req.Header.Set("Referer", "https://fake1.anotherwebsite.local/")
	DelReferer(req)
	if req.Header.Get("Referer") != "" {
		t.Error("DelReferer failed")
	}
}
