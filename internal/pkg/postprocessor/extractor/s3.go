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
	Prefix []string `xml:"Prefix"`
}

// IsS3 checks if the response is from an S3 server
func IsS3(URL *models.URL) bool {
	return utils.StringContainsSliceElements(URL.GetResponse().Header.Get("Server"), validS3Servers)
}

// S3 decides which helper to call based on the query param: old style (no list-type=2) vs. new style (list-type=2)
func S3(URL *models.URL) ([]*models.URL, error) {
	defer URL.RewindBody()

	// Decode XML result
	var result S3ListBucketResult
	if err := xml.NewDecoder(URL.GetBody()).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding S3 XML: %v", err)
	}

	// Prepare base data
	reqURL := URL.GetRequest().URL
	listType := reqURL.Query().Get("list-type")

	// Build https://<host> as the base for direct file links
	baseStr := fmt.Sprintf("https://%s", reqURL.Host)
	parsedBase, err := url.Parse(baseStr)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %v", err)
	}

	var outlinkStrings []string

	// Delegate to old style or new style
	if listType != "2" {
		// Old style S3 listing, uses marker
		outlinkStrings = s3Legacy(reqURL, parsedBase, result)
	} else {
		// New style listing (list-type=2), uses continuation token and/or CommonPrefixes
		outlinkStrings = s3V2(reqURL, parsedBase, result)
	}

	// Convert from []string -> []*models.URL
	var outlinks []*models.URL
	for _, link := range outlinkStrings {
		outlinks = append(outlinks, &models.URL{Raw: link})
	}
	return outlinks, nil
}

// s3Legacy handles the old ListObjects style, which uses `marker` for pagination.
func s3Legacy(reqURL *url.URL, parsedBase *url.URL, result S3ListBucketResult) []string {
	var outlinks []string

	// If there are objects in <Contents>, create a "next page" URL using `marker`
	if len(result.Contents) > 0 {
		lastKey := result.Contents[len(result.Contents)-1].Key
		nextURL := *reqURL
		q := nextURL.Query()
		q.Set("marker", lastKey)
		nextURL.RawQuery = q.Encode()
		outlinks = append(outlinks, nextURL.String())
	}

	// Produce direct file links for each object
	for _, obj := range result.Contents {
		if obj.Size > 0 {
			fileURL := *parsedBase
			fileURL.Path += "/" + obj.Key
			outlinks = append(outlinks, fileURL.String())
		}
	}

	return outlinks
}

// s3V2 handles the new ListObjectsV2 style, which uses `continuation-token` and can return CommonPrefixes.
func s3V2(reqURL *url.URL, parsedBase *url.URL, result S3ListBucketResult) []string {
	var outlinks []string

	// If we have common prefixes => "subfolders"
	if len(result.CommonPrefixes) > 0 {
		for _, prefix := range result.CommonPrefixes {
			// Create a URL for each common prefix (subfolder)
			for _, p := range prefix.Prefix {
				nextURL := *reqURL
				q := nextURL.Query()
				q.Set("prefix", p)
				nextURL.RawQuery = q.Encode()
				outlinks = append(outlinks, nextURL.String())
			}
		}
	} else {
		// Otherwise, we have actual objects in <Contents>
		for _, obj := range result.Contents {
			if obj.Size > 0 {
				fileURL := *parsedBase
				fileURL.Path += "/" + obj.Key
				outlinks = append(outlinks, fileURL.String())
			}
		}
	}

	// If truncated => add a link with continuation-token
	if result.IsTruncated && result.NextContinuationToken != "" {
		nextURL := *reqURL
		q := nextURL.Query()
		q.Set("continuation-token", result.NextContinuationToken)
		nextURL.RawQuery = q.Encode()
		outlinks = append(outlinks, nextURL.String())
	}

	return outlinks
}
