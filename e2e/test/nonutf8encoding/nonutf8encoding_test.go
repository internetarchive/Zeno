package boot

import (
	_ "embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/internetarchive/Zeno/e2e"
)

type recordMatcher struct {
	url1Archived    bool
	url2Archived    bool
	url3Archived    bool
	unexpectedError bool
}

func (rm *recordMatcher) Match(record map[string]string) {
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
		case "/":
		default:
			fmt.Printf("Unexpected URL archived: %s\n", record["url"])
			rm.unexpectedError = true
		}
	}
}

func (rm *recordMatcher) Assert(t *testing.T) {
	if !(rm.url1Archived && rm.url2Archived && rm.url3Archived) {
		t.Errorf("Not all URLs were archived: url1Archived=%v, url2Archived=%v, url3Archived=%v",
			rm.url1Archived, rm.url2Archived, rm.url3Archived)
	}
	if rm.unexpectedError {
		t.Error("An unexpected error was logged during the test")
	}
}

func (rm *recordMatcher) ShouldStop() bool {
	return (rm.url1Archived && rm.url2Archived && rm.url3Archived) || rm.unexpectedError
}

//go:embed testdata/gbk.html
var gbkPayload []byte

func setupServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Received request: %s %s\n", r.Method, r.URL.Path)
		switch r.URL.Path {
		case "/1111你好", "/2222你好", "/3333你好":
			if strings.Contains(r.URL.RawQuery, "%CA%C0%BD%E7=%D4%D9%BC%FB") { // >>> '世界=再见'.encode('gbk') = b'\xca\xc0\xbd\xe7=\xd4\xd9\xbc\xfb'
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte("OK"))
				return
			} else {
				http.Error(w, "Bad Request - bad query", http.StatusBadRequest)
				return
			}
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=gbk") // Declare GBK encoding
			w.Write(gbkPayload)
			return
		}
		http.NotFound(w, r)
	}))
}

func TestNonUTF8Encoding(t *testing.T) {
	server := setupServer()
	serverURL := strings.Replace(server.URL, "127.0.0.1", "127.0.0.1.nip.io", 1)
	defer server.Close()

	os.RemoveAll("jobs")

	tempSocketPath := path.Join(os.TempDir(), fmt.Sprintf("zeno-%d.sock", os.Getpid()))
	defer os.Remove(tempSocketPath)

	shouldStopCh := make(chan struct{})
	rm := &recordMatcher{}
	wg := &sync.WaitGroup{}

	wg.Add(2)

	go e2e.StartHandleLogRecord(t, wg, rm, tempSocketPath, shouldStopCh)
	go e2e.ExecuteCmdZenoGetURL(t, wg, tempSocketPath, []string{serverURL})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)
}
