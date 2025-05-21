package extractor

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAzure(t *testing.T) {
	t.Run("Valid XML with single object and nextMarker", func(t *testing.T) {
		xmlBody := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<EnumerationResults  ContainerName="zeno">
    <Prefix />
    <Marker>dir/azure_files/test_10.txt</Marker>
    <MaxResults>1</MaxResults>
    <Blobs>
        <Blob>
            <Name>dir/azure_files/test_100.txt</Name>
            <Properties>
                <Creation-Time>Thu, 15 May 2025 08:02:20 GMT</Creation-Time>
                <Last-Modified>Thu, 15 May 2025 08:02:20 GMT</Last-Modified>
                <Etag>0x202A0424CF3AFA0</Etag>
                <Content-Length>4</Content-Length>
                <Content-Type>text/plain</Content-Type>
                <Content-MD5>kZ0ReVbTE1xMaD/wITUvXA==</Content-MD5>
                <BlobType>BlockBlob</BlobType>
                <LeaseStatus>unlocked</LeaseStatus>
                <LeaseState>available</LeaseState>
                <ServerEncrypted>true</ServerEncrypted>
                <AccessTier>Hot</AccessTier>
                <AccessTierInferred>true</AccessTierInferred>
                <AccessTierChangeTime>Thu, 15 May 2025 08:02:20 GMT</AccessTierChangeTime>
            </Properties>
        </Blob>
    </Blobs>
    <NextMarker>dir/azure_files/test_100.txt</NextMarker>
</EnumerationResults>`
		URLObj := buildTestObjectStorageURLObj("http://example.com/devstoreaccount1/zeno?restype=container&comp=list&maxresults=1", xmlBody, http.Header{"Server": []string{"Windows-Azure-Blob/1.0"}})
		outlinks, err := azure(URLObj)
		outlinks2, err2 := ObjectStorage(URLObj) // indirectly call, for coverage testing
		if err != nil || err2 != nil {
			t.Fatalf("azure() returned unexpected error: %v, err2: %v", err, err2)
		}

		if len(outlinks) != 2 || len(outlinks2) != 2 {
			t.Fatalf("expected 2 outlinks, got %d and %d", len(outlinks), len(outlinks2))
		}

		expectedOutlinks := func() []*url.URL {
			urls := []string{
				"http://example.com/devstoreaccount1/zeno?comp=list&marker=dir%2Fazure_files%2Ftest_100.txt&maxresults=1&restype=container",
				"http://example.com/devstoreaccount1/zeno/dir/azure_files/test_100.txt",
			}
			outlinks := make([]*url.URL, len(urls))
			for i, link := range urls {
				parsed, err := url.Parse(link)
				if err != nil {
					t.Fatalf("failed to parse expected outlink %s: %v", link, err)
				}
				outlinks[i] = parsed
			}
			return outlinks
		}()

		for i, outlink := range outlinks {
			outlink.Parse()
			if outlink.GetParsed().String() != expectedOutlinks[i].String() { // Compare parsed URLs, The order of URL query parameters is insensitive
				t.Errorf("expected %s, got %s", expectedOutlinks[i].String(), outlink.String())
			}
		}
	})

	t.Run("invalid XML", func(t *testing.T) {
		xmlBody := `<EnumerationResults><BadTag`
		URLObj := buildTestObjectStorageURLObj("http://example.com/devstoreaccount1/zeno?restype=container&comp=list&maxresults=1", xmlBody, http.Header{"Server": []string{"Windows-Azure-Blob/1.0"}})
		_, err := azure(URLObj)
		if err == nil {
			t.Fatalf("expected error for invalid XML, got none")
		}

		if !strings.Contains(err.Error(), "error decoding AzureBlobEnumerationResults XML") {
			t.Fatalf("expected error 'error decoding AzureBlobEnumerationResults XML', got %v", err)
		}
	})
}
