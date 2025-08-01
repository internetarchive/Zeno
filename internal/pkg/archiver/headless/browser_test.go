package headless

import (
	"testing"
)

func Test_queryLatestChromiumRevision(t *testing.T) {
	latestRev, err := queryLatestChromiumRevision()
	if err != nil {
		t.Fatalf("failed to get latest Chromium revision: %v", err)
	}
	if latestRev > 0 {
		t.Logf("Latest Chromium revision: %d", latestRev)
	} else {
		t.Error("Expected a positive revision number, got 0 or negative")
	}
}
