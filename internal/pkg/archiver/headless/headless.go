package headless

import "github.com/internetarchive/Zeno/v2/internal/pkg/log"

var logger = log.NewFieldedLogger(&log.Fields{
	"component": "archiver.headless",
})
