package crawl

import (
	"fmt"
	"path"

	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/CorentinB/warc"
)

func (c *Crawl) initWARCRotatorSettings() *warc.RotatorSettings {
	var rotatorSettings = warc.NewRotatorSettings()

	rotatorSettings.OutputDirectory = path.Join(c.JobPath, "warcs")
	rotatorSettings.Compression = "GZIP"
	rotatorSettings.Prefix = c.WARCPrefix
	rotatorSettings.WarcinfoContent.Set("software", fmt.Sprintf("Zeno %s", utils.GetVersion().Version))

	if len(c.WARCOperator) > 0 {
		rotatorSettings.WarcinfoContent.Set("operator", c.WARCOperator)
	}

	return rotatorSettings
}
