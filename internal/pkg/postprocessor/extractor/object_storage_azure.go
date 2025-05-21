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

// AzureBlobEnumerationResults represents the XML structure of an Azure Blob Storage listing
// <https://learn.microsoft.com/en-us/rest/api/storageservices/enumerating-blob-resources>
type AzureBlobEnumerationResults struct {
	XMLName    xml.Name    `xml:"EnumerationResults"`
	Prefix     string      `xml:"Prefix"`
	Marker     string      `xml:"Marker"`
	Blobs      []AzureBlob `xml:"Blobs>Blob"`
	NextMarker string      `xml:"NextMarker"`
}

type AzureBlob struct {
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
	var result AzureBlobEnumerationResults
	if err := xml.NewDecoder(URL.GetBody()).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding AzureBlobEnumerationResults XML: %w", err)
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
	baseURL := *reqURL
	baseURL.RawQuery = ""
	baseURL.ForceQuery = false

	for _, blob := range result.Blobs {
		if strings.HasPrefix(blob.Name, "/") {
			azureLogger.Warn("invalid blob name: it starts with a leading slash", "blob_name", blob.Name)
			continue
		}
		fileURL := baseURL.JoinPath(blob.Name)
		outlinks = append(outlinks, fileURL.String())
	}

	return toURLs(outlinks), nil
}
