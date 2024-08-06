package crawl

import (
	"fmt"
	"path"

	"github.com/CorentinB/warc"
	"github.com/internetarchive/Zeno/internal/utils"
)

func (c *Crawl) initWARCRotatorSettings() *warc.RotatorSettings {
	var rotatorSettings = warc.NewRotatorSettings()

	rotatorSettings.OutputDirectory = path.Join(c.JobPath, "warcs")
	rotatorSettings.Compression = "GZIP"
	rotatorSettings.Prefix = c.WARCPrefix
	rotatorSettings.WarcinfoContent.Set("software", fmt.Sprintf("Zeno %s", utils.GetVersion().Version))
	rotatorSettings.WARCWriterPoolSize = c.WARCPoolSize

	if len(c.WARCOperator) > 0 {
		rotatorSettings.WarcinfoContent.Set("operator", c.WARCOperator)
	}

	return rotatorSettings
}
