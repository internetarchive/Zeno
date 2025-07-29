package extractor

import (
	"bytes"
	"io"
	"testing"
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
