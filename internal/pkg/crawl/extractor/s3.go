package extractor

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/CorentinB/warc"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

var validS3Servers = []string{
	"AmazonS3",
}

// S3ListBucketResult represents the XML structure of an S3 bucket listing
type S3ListBucketResult struct {
	XMLName               xml.Name       `xml:"ListBucketResult"`
	Name                  string         `xml:"Name"`
	Prefix                string         `xml:"Prefix"`
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
func IsS3(resp *http.Response) bool {
	return utils.StringContainsSliceElements(resp.Header.Get("Server"), validS3Servers)
}

// S3 takes an initial response and custom HTTP client, returns all file URLs
func S3(resp *http.Response, c *warc.CustomHTTPClient) ([]*url.URL, error) {
	stringPtrs, err := S3CollectURLs(resp, c)
	if err != nil {
		return nil, err
	}

	// Convert []*string to []string
	strings := make([]string, len(stringPtrs))
	for i, ptr := range stringPtrs {
		strings[i] = *ptr
	}

	// Convert to []*url.URL using the utility function
	return utils.StringSliceToURLSlice(strings), nil
}

// S3CollectURLs collects all URLs as string pointers
func S3CollectURLs(resp *http.Response, c *warc.CustomHTTPClient) ([]*string, error) {
	var allFiles []*string

	// Extract base URL from the response URL
	reqURL := resp.Request.URL
	baseURL := fmt.Sprintf("https://%s", reqURL.Host)

	// Parse the base URL
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %v", err)
	}

	// Process the response
	result, err := S3ProcessResponse(resp)
	if err != nil {
		return nil, err
	}

	// Add initial files
	for _, obj := range result.Contents {
		if obj.Size > 0 {
			fileURL := *parsedBase
			fileURL.Path += "/" + obj.Key
			urlStr := fileURL.String()
			allFiles = append(allFiles, &urlStr)
		}
	}

	// Process all common prefixes
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(result.CommonPrefixes))
	sem := make(chan struct{}, 4) // Limit to 4 concurrent goroutines

	for _, prefix := range result.CommonPrefixes {
		wg.Add(1)
		go func(prefix string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			files, err := S3ExplorePrefix(prefix, parsedBase, c)
			if err != nil {
				errChan <- fmt.Errorf("error exploring prefix %s: %v", prefix, err)
				return
			}

			mu.Lock()
			allFiles = append(allFiles, files...)
			mu.Unlock()
		}(prefix.Prefix)
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return nil, <-errChan
	}

	return allFiles, nil
}

// S3ProcessResponse parses an HTTP response into an S3ListBucketResult
func S3ProcessResponse(resp *http.Response) (*S3ListBucketResult, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}
	defer resp.Body.Close()

	var result S3ListBucketResult
	if err := xml.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error parsing XML: %v", err)
	}

	return &result, nil
}

// S3ExplorePrefix explores a single prefix and returns all file URLs found
func S3ExplorePrefix(prefix string, baseURL *url.URL, c *warc.CustomHTTPClient) ([]*string, error) {
	var files []*string

	// Create the request URL for this prefix
	requestURL := *baseURL
	q := requestURL.Query()
	q.Set("list-type", "2")
	q.Set("prefix", prefix)
	q.Set("delimiter", "/")
	requestURL.RawQuery = q.Encode()

	// Make the HTTP request using the custom client
	resp, err := c.Get(requestURL.String())
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request: %v", err)
	}

	result, err := S3ProcessResponse(resp)
	if err != nil {
		return nil, err
	}

	// Process files in this prefix
	for _, obj := range result.Contents {
		if obj.Size > 0 {
			fileURL := *baseURL
			fileURL.Path += "/" + obj.Key
			urlStr := fileURL.String()
			files = append(files, &urlStr)
		}
	}

	// Recursively explore all sub-prefixes
	if len(result.CommonPrefixes) > 0 {
		for _, subPrefix := range result.CommonPrefixes {
			subFiles, err := S3ExplorePrefix(subPrefix.Prefix, baseURL, c)
			if err != nil {
				return nil, err
			}
			files = append(files, subFiles...)
		}
	}

	return files, nil
}
