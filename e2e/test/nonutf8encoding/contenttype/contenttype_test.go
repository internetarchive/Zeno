package contenttype

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/internetarchive/Zeno/e2e"
	"github.com/internetarchive/Zeno/e2e/test/nonutf8encoding"
)

func TestNonUTF8ContentType(t *testing.T) {
	server := nonutf8encoding.SetupServer()
	serverURL := strings.Replace(server.URL, "127.0.0.1", "127.0.0.1.nip.io", 1)
	defer server.Close()

	os.RemoveAll("jobs")

	tempSocketPath := path.Join(os.TempDir(), fmt.Sprintf("zeno-%d.sock", os.Getpid()))
	defer os.Remove(tempSocketPath)

	shouldStopCh := make(chan struct{})
	rm := &nonutf8encoding.RecordMatcher{}
	wg := &sync.WaitGroup{}

	wg.Add(2)

	go e2e.StartHandleLogRecord(t, wg, rm, tempSocketPath, shouldStopCh)
	go e2e.ExecuteCmdZenoGetURL(t, wg, tempSocketPath, []string{serverURL + "/raw"})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)
}
