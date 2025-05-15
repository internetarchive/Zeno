package extractor

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
)

// Azure Blob Storage
var azureServers = []string{
	// Windows-Azure-Blob/1.0 Microsoft-HTTPAPI/2.0
	"Windows-Azure-Blob",
	// Blob Service Version 1.0 Microsoft-HTTPAPI/2.0
	"Blob Service Version",
	// emulator, https://github.com/Azure/Azurite
	"Azurite-Blob",
}

// AZureBlobEnumerationResults represents the XML structure of an AZure Blob Storage listing
// <https://learn.microsoft.com/en-us/rest/api/storageservices/enumerating-blob-resources>
type AZureBlobEnumerationResults struct {
	XMLName    xml.Name    `xml:"EnumerationResults"`
	Prefix     string      `xml:"Prefix"`
	Marker     string      `xml:"Marker"`
	Blobs      []AZureBlob `xml:"Blobs>Blob"`
	NextMarker string      `xml:"NextMarker"`
}

type AZureBlob struct {
	Name         string `xml:"Name"` // path/to/file.txt, no leading slash
	LastModified string `xml:"Properties>Last-Modified"`
	Size         int64  `xml:"Properties>Content-Length"`
}

var azureLogger = log.NewFieldedLogger(&log.Fields{
	"component": "postprocessor.extractor.object_storage_azure",
})

func azure(URL *models.URL) ([]*models.URL, error) {
	defer URL.RewindBody()

	// Decode XML result
	var result AZureBlobEnumerationResults
	if err := xml.NewDecoder(URL.GetBody()).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding AZureBlobEnumerationResults XML: %w", err)
	}

	reqURL := URL.GetRequest().URL

	var outlinks []string

	if result.NextMarker != "" {
		nextURL := *reqURL
		q := nextURL.Query()
		q.Set("marker", result.NextMarker)
		nextURL.RawQuery = q.Encode()
		outlinks = append(outlinks, nextURL.String())
	}

	// Build base url for files
	//
	// reqURL: "https://{endpoint_host}/{account}/{bucket}?..."
	// ->
	// baseURL: "https://{endpoint_host}/{account}/{bucket}/
	baseURL := *reqURL
	baseURL.RawQuery = ""
	baseURL.ForceQuery = false
	if !strings.HasSuffix(baseURL.Path, "/") {
		baseURL.Path = baseURL.Path + "/"
	}

	for _, blob := range result.Blobs {
		fileURL := baseURL
		if strings.HasPrefix(blob.Name, "/") {
			azureLogger.Warn("invalid blob name: it starts with a leading slash", "blob_name", blob.Name)
			continue
		}
		fileURL.Path = baseURL.Path + blob.Name
		outlinks = append(outlinks, fileURL.String())
	}

	return toURLs(outlinks), nil
}
