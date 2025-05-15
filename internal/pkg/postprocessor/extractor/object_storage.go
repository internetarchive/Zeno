package extractor

import (
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
)

// All the supported object storage servers
var ObjectStorageServers = func() (s []string) {
	s = append(s, s3CompatibleServers...)
	s = append(s, azureServers...)
	return s
}()

// IsObjectStorage checks if the response is from an object storage server
func IsObjectStorage(URL *models.URL) bool {
	return utils.StringContainsSliceElements(URL.GetResponse().Header.Get("Server"), ObjectStorageServers) &&
		strings.Contains(URL.GetResponse().Header.Get("Content-Type"), "/xml") // tricky match both application/xml and text/xml
}

// ObjectStorage decides which helper to call based on the object storage server
func ObjectStorage(URL *models.URL) ([]*models.URL, error) {
	defer URL.RewindBody()

	server := URL.GetResponse().Header.Get("Server")
	if utils.StringContainsSliceElements(server, s3CompatibleServers) {
		return s3Compatible(URL)
	} else if utils.StringContainsSliceElements(server, azureServers) {
		return azure(URL)
	} else {
		return nil, nil
	}
}
