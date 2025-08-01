package extractor

import (
	"bytes"
	"io"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
	"golang.org/x/text/encoding/htmlindex"
)

func Test_charsetNewReader(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		input       string
		wantOutput  string
		wantErr     bool
		wantEnc     string
	}{
		{
			name:        "UTF-8 content",
			contentType: "text/html; charset=utf-8",
			input:       "<html><body>Test</body></html>",
			wantOutput:  "<html><body>Test</body></html>",
			wantEnc:     "utf-8",
			wantErr:     false,
		},
		{
			name:        "ISO-8859-1 content",
			contentType: "text/html; charset=iso-8859-1",
			input:       "<html><body>Test</body></html>",
			wantOutput:  "<html><body>Test</body></html>",
			wantEnc:     "windows-1252",
			wantErr:     false,
		},
		{
			name:        "Invalid content-type with ascii html",
			contentType: "text/html; charset=invalid-charset",
			input:       "<html><body>Test</body></html>",
			wantOutput:  "<html><body>Test</body></html>",
			wantEnc:     "windows-1252",
			wantErr:     false,
		},
		{
			name:        "Invalid content-type with utf8 html",
			contentType: "text/html; charset=invalid-charset",
			input:       "<html><body>你好</body></html>",
			wantOutput:  "<html><body>你好</body></html>",
			wantEnc:     "utf-8",
			wantErr:     false,
		},
		{
			name:        "GBK html without content-type",
			contentType: "",
			input:       "<html><head><meta charset=\"gbk\"></head><body>\xc4\xe3\xba\xc3</body></html>",
			wantOutput:  "<html><head><meta charset=\"gbk\"></head><body>你好</body></html>",
			wantEnc:     "gbk",
			wantErr:     false,
		},
		{
			name:        "GBK html with content-type hint",
			contentType: "text/html; charset=gbk",
			input:       "<html><body>\xc4\xe3\xba\xc3</body></html>",
			wantOutput:  "<html><body>你好</body></html>",
			wantEnc:     "gbk",
			wantErr:     false,
		},
		{
			name:        "GBK html with wrong content-type",
			contentType: "text/html; charset=utf-8",
			input:       "<html><head><meta charset=\"gbk\"></head><body>\xc4\xe3\xba\xc3</body></html>",
			wantOutput:  "<html><head><meta charset=\"gbk\"></head><body>���</body></html>",
			wantEnc:     "utf-8",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader([]byte(tt.input))
			output, err, _, encName, _ := charsetNewReader(r, tt.contentType)

			if (err != nil) != tt.wantErr {
				t.Errorf("charsetNewReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if output == nil {
				t.Errorf("charsetNewReader() output is nil")
				return
			}

			if encName != tt.wantEnc {
				t.Errorf("charsetNewReader() encoding = %v, want %v", encName, tt.wantEnc)
			}

			buf := new(bytes.Buffer)
			if _, err := io.Copy(buf, output); err != nil {
				t.Errorf("io.Copy() error = %v", err)
				return
			}
			if buf.String() != tt.wantOutput {
				t.Errorf("charsetNewReader() got = %v, want %v", buf.String(), tt.wantOutput)
			}
		})
	}
}

func Test_encodeNonUTF8QueryURLs(t *testing.T) {
	tests := []struct {
		name     string
		encName  string
		urls     []string
		wantUrls []string
	}{
		{
			name:     "UTF-8 URLs passthrough encoding",
			encName:  "utf-8",
			urls:     []string{"http://example.com/ABC你好?q=测试", "http://example.com/?q=hello"},
			wantUrls: []string{"http://example.com/ABC你好?q=测试", "http://example.com/?q=hello"},
		},
		{
			name:     "GBK URLs",
			encName:  "gbk",
			urls:     []string{"http://example.com/ABC你好?q=测试", "http://example.com/?q=hello"},
			wantUrls: []string{"http://example.com/ABC%E4%BD%A0%E5%A5%BD?q=%B2%E2%CA%D4", "http://example.com/?q=hello"},
		},
		{
			name:     "Shift_JIS URLs",
			encName:  "shift_jis",
			urls:     []string{"http://example.com/ABCまんが?q=アニメ", "http://example.com/?q=hello"},
			wantUrls: []string{"http://example.com/ABC%E3%81%BE%E3%82%93%E3%81%8C?q=%83A%83j%83%81", "http://example.com/?q=hello"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var urls []*models.URL
			for _, raw := range tt.urls {
				url := &models.URL{Raw: raw}
				urls = append(urls, url)
			}

			enc, _ := htmlindex.Get(tt.encName)
			if enc == nil {
				t.Errorf("charset.Lookup() not found")
				return
			}
			gotUrls := encodeNonUTF8QueryURLs(urls, enc)

			for i, gotUrl := range gotUrls {
				if gotUrl.Raw != tt.wantUrls[i] {
					t.Errorf("encodeNonUTF8QueryURLs() got = %v, want %v", gotUrl.Raw, tt.wantUrls[i])
				}
			}
		})
	}
}
