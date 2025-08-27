package headless

import "github.com/internetarchive/Zeno/internal/pkg/log"

var logger = log.NewFieldedLogger(&log.Fields{
	"component": "archiver.headless",
})
