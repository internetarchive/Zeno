package asset_refer_fixer

import (
	"net/http"
	"strings"
)

var NoRefererHosts = []string{
	"no.referer.test.zeno.local", // Test case
	"oscimg.oschina.net",
	"img-blog.csdnimg.cn",
}

func IsNeedDelRefer(URL string) bool {
	for _, host := range NoRefererHosts {
		if strings.Contains(URL, host+"/") {
			return true
		}
	}
	return false
}

func DelReferer(req *http.Request) {
	if req.Header.Get("Referer") != "" {
		req.Header.Del("Referer")
	}
}
