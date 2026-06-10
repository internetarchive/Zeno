package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

// ItemFixture is the JSON schema for a serialized models.Item test fixture.
// It can be embedded with go:embed and passed to HydrateItem.
type ItemFixture struct {
	URL        string            `json:"url"`
	StatusCode int               `json:"status_code,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	Source     string            `json:"source,omitempty"`
	Hops       int               `json:"hops,omitempty"`
}

// HydrateItem unmarshals a JSON fixture and builds a *models.Item for use in tests.
// Defaults: status_code=200, headers=empty, body=empty, source=unset, hops=0.
// Uses t.Helper() and t.Fatal on errors.
func HydrateItem(t *testing.T, data []byte) *models.Item {
	t.Helper()

	var f ItemFixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	if f.URL == "" {
		t.Fatal("fixture url is required")
	}

	statusCode := f.StatusCode
	if statusCode == 0 {
		statusCode = 200
	}

	resp := &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(f.Body)),
	}
	for k, v := range f.Headers {
		resp.Header.Set(k, v)
	}

	newURL, err := models.NewURL(f.URL)
	if err != nil {
		t.Fatalf("create URL: %v", err)
	}
	newURL.SetResponse(resp)

	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write([]byte(f.Body))
	newURL.SetBody(spooledTempFile)
	if err := newURL.Parse(); err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if f.Hops != 0 {
		newURL.SetHops(f.Hops)
	}

	item := models.NewItem(&newURL, "")
	if item == nil {
		t.Fatal("NewItem returned nil")
	}

	if f.Source != "" {
		src, err := parseItemSource(f.Source)
		if err != nil {
			t.Fatalf("invalid source %q: %v", f.Source, err)
		}
		if err := item.SetSource(src); err != nil {
			t.Fatalf("SetSource: %v", err)
		}
	}

	return item
}

// parseItemSource maps fixture source strings to models.ItemSource constants.
func parseItemSource(s string) (models.ItemSource, error) {
	switch s {
	case "insert":
		return models.ItemSourceInsert, nil
	case "queue":
		return models.ItemSourceQueue, nil
	case "hq":
		return models.ItemSourceHQ, nil
	case "postprocess":
		return models.ItemSourcePostprocess, nil
	case "feedback":
		return models.ItemSourceFeedback, nil
	default:
		return 0, fmt.Errorf("unknown source %q (use: insert, queue, hq, postprocess, feedback)", s)
	}
}
