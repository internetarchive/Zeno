package extractor

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"
)

func TestXML(t *testing.T) {
	tests := []struct {
		name     string
		xmlBody  string
		wantURLs []*url.URL
		wantErr  bool
	}{
		{
			name: "Valid XML with URLs",
			xmlBody: `
				<root>
					<item>http://example.com</item>
					<nested>
						<url>https://example.org</url>
					</nested>
					<noturl>just some text</noturl>
				</root>`,
			wantURLs: []*url.URL{
				{Scheme: "http", Host: "example.com"},
				{Scheme: "https", Host: "example.org"},
			},
			wantErr: false,
		},
		{
			name:     "Empty XML",
			xmlBody:  `<root></root>`,
			wantURLs: nil,
			wantErr:  false,
		},
		{
			name:     "Invalid XML",
			xmlBody:  `<root><unclosed>`,
			wantURLs: nil,
			wantErr:  true,
		},
		{
			name: "XML with invalid URL",
			xmlBody: `
				<root>
					<item>http://example.com</item>
					<item>not a valid url</item>
				</root>`,
			wantURLs: []*url.URL{
				{Scheme: "http", Host: "example.com"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Body: io.NopCloser(bytes.NewBufferString(tt.xmlBody)),
			}

			gotURLs, err := XML(resp)

			if (err != nil) != tt.wantErr {
				t.Errorf("XML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !compareURLs(gotURLs, tt.wantURLs) {
				t.Errorf("XML() gotURLs = %v, want %v", gotURLs, tt.wantURLs)
			}
		})
	}
}

func TestXMLBodyReadError(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader([]byte{})), // Empty reader to simulate EOF
	}
	resp.Body.Close() // Close the body to simulate a read error

	_, err := XML(resp)
	if err == nil {
		t.Errorf("XML() expected error, got nil")
	}
}
