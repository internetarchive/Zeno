package models

import (
	"sync"

	"github.com/gabriel-vasile/mimetype"
)

var mimeExtendOnce sync.Once

func extendMimetype() {
	mimeExtendOnce.Do(func() {
		mimePlaceholderFunc := func(raw []byte, limit uint32) bool {
			return false
		}

		// https://github.com/gabriel-vasile/mimetype/pull/113
		// gabriel-vasile/mimetype does not support CSS detection yet, so
		// we have to extend a placeholder for our Content-Type lookup.
		mimetype.Lookup("text/plain").Extend(mimePlaceholderFunc, "text/css", ".css")
	})
}
