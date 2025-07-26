package extractor

import (
	"net/http"
	"strings"
	"testing"
)

func TestS3Compatible(t *testing.T) {
	extractor := ObjectStorageOutlinkExtractor{}
	// This subtest shows a scenario of a valid XML with a single object,
	// and list-type != 2 => "marker" logic should be used.
	t.Run("Valid XML with single object, no list-type=2 => marker next link", func(t *testing.T) {
		xmlBody := `
<ListBucketResult>
	<Contents>
		<Key>file1.txt</Key>
		<LastModified>2021-01-01T12:00:00.000Z</LastModified>
		<Size>123</Size>
	</Contents>
	<IsTruncated>false</IsTruncated>
</ListBucketResult>`

		URLObj := buildTestObjectStorageURLObj("https://example.com/?someparam=1", xmlBody, http.Header{"Server": []string{"AmazonS3"}})
		outlinks, err := s3Compatible(URLObj)
		outlinks2, err2 := extractor.Extract(URLObj) // indirectly call, for coverage testing
		if err != nil || err2 != nil {
			t.Fatalf("S3() returned unexpected error: %v, err2: %v", err, err2)
		}

		if len(outlinks) != 2 || len(outlinks2) != 2 {
			t.Fatalf("expected 2 outlinks, got %d and %d", len(outlinks), len(outlinks2))
		}
		expectedOutlinks := []string{
			"https://example.com/?marker=file1.txt&someparam=1",
			"https://example.com/file1.txt",
		}
		for i, outlink := range outlinks {
			if outlink.Raw != expectedOutlinks[i] {
				t.Errorf("expected %s, got %s", expectedOutlinks[i], outlink.Raw)
			}
		}
	})

	// Another subtest example: common prefixes => subfolder links for list-type=2
	t.Run("Valid XML with common prefixes => subfolder links (list-type=2)", func(t *testing.T) {
		xmlBody := `
<ListBucketResult>
    <IsTruncated>false</IsTruncated>
    <CommonPrefixes>
        <Prefix>folder1/</Prefix>
        <Prefix>folder2/</Prefix>
    </CommonPrefixes>
</ListBucketResult>`

		URLObj := buildTestObjectStorageURLObj("https://example.com/?list-type=2", xmlBody, http.Header{"Server": []string{"AmazonS3"}})
		outlinks, err := s3Compatible(URLObj)
		outlinks2, err2 := extractor.Extract(URLObj) // indirectly call, for coverage testing
		if err != nil || err2 != nil {
			t.Fatalf("s3Compatible() returned unexpected error: %v, err2: %v", err, err2)
		}

		if len(outlinks) != 2 || len(outlinks2) != 2 {
			t.Fatalf("expected 2 outlinks, got %d and %d", len(outlinks), len(outlinks2))
		}

		if !strings.Contains(outlinks[0].Raw, "prefix=folder1%2F") {
			t.Errorf("expected prefix=folder1/ in outlink, got %s", outlinks[0].Raw)
		}
		if !strings.Contains(outlinks[1].Raw, "prefix=folder2%2F") {
			t.Errorf("expected prefix=folder2/ in outlink, got %s", outlinks[1].Raw)
		}
	})

	// Example for invalid XML
	t.Run("Invalid XML => error", func(t *testing.T) {
		xmlBody := `<ListBucketResult><BadTag`

		URLObj := buildTestObjectStorageURLObj("https://example.com/?list-type=2", xmlBody, http.Header{"Server": []string{"AmazonS3"}})
		outlinks, err := s3Compatible(URLObj)
		outlinks2, err2 := extractor.Extract(URLObj) // indirectly call, for coverage testing
		if err == nil || err2 == nil {
			t.Fatalf("expected error for invalid XML, got none")
		}

		if len(outlinks) != 0 || len(outlinks2) != 0 {
			t.Errorf("expected no outlinks on error, got %d and %d", len(outlinks), len(outlinks2))
		}
	})
}
