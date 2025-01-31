package extractor

import (
	"encoding/xml"
	"fmt"
	"net/url"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
)

var validS3Servers = []string{
	"AmazonS3",
	"WasabiS3",
	"UploadServer", // Google Cloud Storage
	"Windows-Azure-Blob",
	"AliyunOSS", // Alibaba Object Storage Service
}

// S3ListBucketResult represents the XML structure of an S3 bucket listing
type S3ListBucketResult struct {
	XMLName               xml.Name       `xml:"ListBucketResult"`
	Name                  string         `xml:"Name"`
	Prefix                string         `xml:"Prefix"`
	Marker                string         `xml:"Marker"`
	Contents              []S3Object     `xml:"Contents"`
	CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes"`
	IsTruncated           bool           `xml:"IsTruncated"`
	NextContinuationToken string         `xml:"NextContinuationToken"`
}

type S3Object struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	Size         int64  `xml:"Size"`
}

type CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

// IsS3 checks if the response is from an S3 server
func IsS3(URL *models.URL) bool {
	return utils.StringContainsSliceElements(URL.GetResponse().Header.Get("Server"), validS3Servers)
}

// S3 takes an initial response and returns URLs of either files or prefixes at the current level,
// plus continuation URL if more results exist
func S3(URL *models.URL) ([]*models.URL, error) {
	defer URL.RewindBody()

	var result S3ListBucketResult
	if err := xml.NewDecoder(URL.GetBody()).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding S3 XML: %v", err)
	}

	// Extract base URL from the response URL
	reqURL := URL.GetRequest().URL
	requestQuery := reqURL.Query()
	baseURL := fmt.Sprintf("https://%s", reqURL.Host)
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %v", err)
	}

	var URLs []string

	// Ensure we can add marker
	// ListObjects
	if requestQuery.Get("list-type") != "2" && len(result.Contents) > 0 {
		// If we can, iterate through S3 using the marker field
		nextURL := *reqURL
		q := nextURL.Query()
		q.Set("marker", result.Contents[len(result.Contents)-1].Key)
		nextURL.RawQuery = q.Encode()
		URLs = append(URLs, nextURL.String())
	}

	// If we are using list-type 2/ListObjectsV2
	if len(result.CommonPrefixes) > 0 {
		for _, prefix := range result.CommonPrefixes {
			nextURL := *reqURL
			q := nextURL.Query()
			q.Set("prefix", prefix.Prefix)
			nextURL.RawQuery = q.Encode()
			URLs = append(URLs, nextURL.String())
		}
	} else {
		// Otherwise return file URLs
		for _, obj := range result.Contents {
			if obj.Size > 0 {
				fileURL := *parsedBase
				fileURL.Path += "/" + obj.Key
				URLs = append(URLs, fileURL.String())
			}
		}
	}

	// If there's a continuation token, add the continuation URL
	if result.IsTruncated && result.NextContinuationToken != "" {
		nextURL := *reqURL
		q := nextURL.Query()
		q.Set("continuation-token", result.NextContinuationToken)
		nextURL.RawQuery = q.Encode()
		URLs = append(URLs, nextURL.String())
	}

	var outlinks []*models.URL
	for _, extractedURL := range URLs {
		outlinks = append(outlinks, &models.URL{
			Raw: extractedURL,
		})
	}

	return outlinks, nil
}
