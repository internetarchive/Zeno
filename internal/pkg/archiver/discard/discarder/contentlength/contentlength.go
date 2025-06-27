package contentlength

import (
	"net/http"

	"github.com/internetarchive/Zeno/internal/pkg/config"
)

var ContentLengthExceeded = "Response content-length exceeds configured limit"

func ContentLengthHook(resp *http.Response) (bool, string) {
	if resp.ContentLength > int64(config.Get().MaxContentLengthMiB)*1024*1024 {
		return true, ContentLengthExceeded
	}
	return false, ""
}
