package nonutf8encoding

import (
	_ "embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type RecordMatcher struct {
	url1Archived    bool
	url2Archived    bool
	url3Archived    bool
	unexpectedError bool
}

func (rm *RecordMatcher) Match(record map[string]string) {
	if record["level"] == "ERROR" {
		rm.unexpectedError = true
	}

	if record["msg"] == "url archived" {
		if record["status"] != "200" {
			fmt.Printf("Unexpected status for archived URL: %s\n", record["status"])
			rm.unexpectedError = true
			return
		}
		URL, _ := url.Parse(record["url"])
		switch URL.Path {
		case "/1111你好":
			rm.url1Archived = true
		case "/2222你好":
			rm.url2Archived = true
		case "/3333你好":
			rm.url3Archived = true
		case "/raw", "/meta_decl":
		default:
			fmt.Printf("Unexpected URL archived: %s\n", record["url"])
			rm.unexpectedError = true
		}
	}
}

func (rm *RecordMatcher) Assert(t *testing.T) {
	if !(rm.url1Archived && rm.url2Archived && rm.url3Archived) {
		t.Errorf("Not all URLs were archived: url1Archived=%v, url2Archived=%v, url3Archived=%v",
			rm.url1Archived, rm.url2Archived, rm.url3Archived)
	}
	if rm.unexpectedError {
		t.Error("An unexpected error was logged during the test")
	}
}

func (rm *RecordMatcher) ShouldStop() bool {
	return (rm.url1Archived && rm.url2Archived && rm.url3Archived) || rm.unexpectedError
}

//go:embed testdata/gbk_raw.html
var gbkRawPayload []byte

//go:embed testdata/gbk_meta_charset.html
var gbkMetaCharsetPayload []byte

func SetupServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Received request: %s %s\n", r.Method, r.URL.Path)
		switch r.URL.Path {
		case "/1111你好", "/2222你好", "/3333你好":
			if strings.Contains(r.URL.RawQuery, "%CA%C0%BD%E7=%D4%D9%BC%FB") { // >>> '世界=再见'.encode('gbk') = b'\xca\xc0\xbd\xe7=\xd4\xd9\xbc\xfb'
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte("OK"))
			} else {
				http.Error(w, "Bad Request - bad query", http.StatusBadRequest)
			}
		case "/raw":
			w.Header().Set("Content-Type", "text/html; charset=gbk") // Declare GBK encoding
			w.Write(gbkRawPayload)
		case "/meta_decl":
			w.Header().Set("Content-Type", "text/html") //
			w.Write(gbkMetaCharsetPayload)
		default:
			http.NotFound(w, r)
		}
	}))
}
