package crawl

import (
	"path"

	"github.com/CorentinB/warc"
	"github.com/sirupsen/logrus"
)

func (c *Crawl) initWARCWriter() {
	var rotatorSettings = warc.NewRotatorSettings()
	var err error

	rotatorSettings.OutputDirectory = path.Join(c.JobPath, "warcs")
	rotatorSettings.Compression = "GZIP"
	rotatorSettings.Prefix = c.WARCPrefix
	rotatorSettings.WarcinfoContent.Set("software", "Zeno")
	if len(c.WARCOperator) > 0 {
		rotatorSettings.WarcinfoContent.Set("operator", c.WARCOperator)
	}

	c.WARCWriter, c.WARCWriterFinish, err = rotatorSettings.NewWARCRotator()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("Error when initialize WARC writer")
	}
}
