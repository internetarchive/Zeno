package log

import (
	"bytes"
	_ "embed"
	"testing"
)

//go:embed testdata/zeno.log.data
var zenoLog []byte

func TestParseLog(t *testing.T) {
	logCh := ParseLog(bytes.NewReader(zenoLog))
	ok := false
	for record := range logCh {
		if record["msg"] == "done, logs are flushing and will be closed" {
			ok = true
		}
	}
	if !ok {
		t.Error("expected log message not found")
	}
}
