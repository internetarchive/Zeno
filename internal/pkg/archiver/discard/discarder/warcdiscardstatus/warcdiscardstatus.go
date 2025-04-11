package warcdiscardstatus

import (
	"net/http"
	"slices"

	"github.com/internetarchive/Zeno/internal/pkg/config"
)

var InWARCDiscardStatus = "Response status code in --warc-discard-status"

// WARCDiscardStatusHook discards responses based on the --warc-discard-status cli flag.
func WARCDiscardStatusHook(resp *http.Response) (bool, string) {
	if len(config.Get().WARCDiscardStatus) > 0 && slices.Contains(config.Get().WARCDiscardStatus, resp.StatusCode) {
		return true, InWARCDiscardStatus
	}
	return false, ""
}
