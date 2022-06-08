package crawl

import (
	"path"

	"github.com/CorentinB/warc"
)

// dumpResponseToFile is like httputil.DumpResponse but dumps the response directly
// to a file and return its path
/*func (c *Crawl) dumpResponseToFile(resp *http.Response) (string, error) {
	var err error

	// Generate a file on disk with a unique name
	UUID := uuid.NewV4()
	filePath := filepath.Join(c.JobPath, "temp", UUID.String()+".temp")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Write the response to the file directly
	err = resp.Write(file)
	if err != nil {
		os.Remove(filePath)
		return "", err
	}

	return filePath, nil
}*/

func (c *Crawl) initWARCRotatorSettings() *warc.RotatorSettings {
	var rotatorSettings = warc.NewRotatorSettings()

	rotatorSettings.OutputDirectory = path.Join(c.JobPath, "warcs")
	rotatorSettings.Compression = "GZIP"
	rotatorSettings.Prefix = c.WARCPrefix
	rotatorSettings.WarcinfoContent.Set("software", "Zeno")
	if len(c.WARCOperator) > 0 {
		rotatorSettings.WarcinfoContent.Set("operator", c.WARCOperator)
	}

	return rotatorSettings
}
