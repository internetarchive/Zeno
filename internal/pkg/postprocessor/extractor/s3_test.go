package extractor

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

// TestIsS3 checks the Server header for known S3 strings.
func TestIsS3(t *testing.T) {
	tests := []struct {
		name   string
		server string
		want   bool
	}{
		{"AmazonS3", "AmazonS3", true},
		{"WasabiS3", "WasabiS3", true},
		{"AliyunOSS", "AliyunOSS", true},
		{"No match", "Apache", false},
		{"Partial match", "Amazon", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a *models.URL with the response Server header set
			URLObj := &models.URL{}

			URLObj.SetResponse(&http.Response{
				Header: http.Header{
					"Server":       []string{tt.server},
					"Content-Type": []string{"text/xml"},
				},
			})

			got := IsS3(URLObj)
			if got != tt.want {
				t.Errorf("IsS3(server=%q) = %v, want %v", tt.server, got, tt.want)
			}
		})
	}
}

func TestS3(t *testing.T) {
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

		// Build an http.Request with a query param that is NOT list-type=2
		reqURL, _ := url.Parse("https://example.com/?someparam=1")

		// Create your models.URL instance.
		URLObj := &models.URL{}
		URLObj.SetRequest(&http.Request{URL: reqURL})

		// Likewise, set the HTTP response header using SetResponse.
		// We want to simulate an S3 server for these tests.
		URLObj.SetResponse(&http.Response{
			Header: http.Header{
				"Server": []string{"AmazonS3"},
			},
		})

		spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
		spooledTempFile.Write([]byte(xmlBody))

		URLObj.SetBody(spooledTempFile)

		outlinks, err := S3(URLObj)
		if err != nil {
			t.Fatalf("S3() returned unexpected error: %v", err)
		}

		if len(outlinks) != 2 {
			t.Fatalf("expected 2 outlinks, got %d", len(outlinks))
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

		reqURL, _ := url.Parse("https://example.com/?list-type=2")

		URLObj := &models.URL{}
		URLObj.SetRequest(&http.Request{URL: reqURL})
		URLObj.SetResponse(&http.Response{
			Header: http.Header{
				"Server": []string{"AmazonS3"},
			},
		})

		spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
		spooledTempFile.Write([]byte(xmlBody))

		URLObj.SetBody(spooledTempFile)

		outlinks, err := S3(URLObj)
		if err != nil {
			t.Fatalf("S3() returned unexpected error: %v", err)
		}

		if len(outlinks) != 2 {
			t.Fatalf("expected 2 outlinks, got %d", len(outlinks))
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

		reqURL, _ := url.Parse("https://example.com/?list-type=2")

		URLObj := &models.URL{}
		URLObj.SetRequest(&http.Request{URL: reqURL})
		URLObj.SetResponse(&http.Response{
			Header: http.Header{
				"Server": []string{"AmazonS3"},
			},
		})

		spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
		spooledTempFile.Write([]byte(xmlBody))

		URLObj.SetBody(spooledTempFile)

		outlinks, err := S3(URLObj)
		if err == nil {
			t.Fatalf("expected error for invalid XML, got none")
		}

		if len(outlinks) != 0 {
			t.Errorf("expected no outlinks on error, got %v", outlinks)
		}
	})
}
