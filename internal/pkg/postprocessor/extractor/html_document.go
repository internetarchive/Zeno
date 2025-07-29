package extractor

import (
	"bytes"
	"io"
	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

var htmldocLogger = log.NewFieldedLogger(&log.Fields{
	"component": "postprocessor.extractor.html_document",
})

// This function is a modified version of the charset.NewReader() function
// Returns additional [enc, name, certain] so we can get the [enc] encoding info to encode url query components correctly
func charsetNewReader(r io.Reader, contentType string) (io.Reader, error, encoding.Encoding, string, bool) {
	preview := make([]byte, 1024)
	n, err := io.ReadFull(r, preview)
	switch {
	case err == io.ErrUnexpectedEOF:
		preview = preview[:n]
		r = bytes.NewReader(preview)
	case err != nil:
		return nil, err, nil, "", false
	default:
		r = io.MultiReader(bytes.NewReader(preview), r)
	}

	enc, name, certain := charset.DetermineEncoding(preview, contentType)
	if enc != encoding.Nop {
		r = transform.NewReader(r, enc.NewDecoder())
	}

	return r, nil, enc, name, certain
}

// TransformDocument transforms the document of a URL by detecting its encoding and creating a utf-8 goquery document.
func TransformDocument(u *models.URL) (doc *goquery.Document, err error) {
	u.DocumentTransfromMux.Lock()
	defer u.DocumentTransfromMux.Unlock()

	// debug: reset cache
	// u.SetDocumentCache(nil)

	if u.GetDocumentCache() == nil {
		// We need to rewind the body, reason:
		// 1. charset.NewReader() will read the first 1024 bytes to detect the encoding.
		// 2. goquery will read until EOF
		defer u.RewindBody()

		contentType := u.GetResponse().Header.Get("Content-Type")

		htmldocLogger.Debug("Transforming document step 1", "url", u.String(), "content_type", contentType)
		transformReader, err, enc, encName, certain := charsetNewReader(u.GetBody(), contentType)
		if err != nil {
			return nil, err
		}
		htmldocLogger.Debug("Transforming document step 2", "url", u.String(), "enc", enc, "enc_name", encName, "certain", certain)

		// Create the document from the converted reader
		document, err := goquery.NewDocumentFromReader(transformReader)
		if err != nil {
			return nil, err
		}

		u.SetDocumentCache(document)
		u.SetDocumentEncoding(enc)
		htmldocLogger.Warn("Document transformed", "url", u.String(), "encoding", encName, "certain", certain)

	}

	return u.GetDocumentCache(), nil
}

func encodeNonUTF8QueryURLs(urls []*models.URL, enc encoding.Encoding) []*models.URL {
	if enc == nil || enc == encoding.Nop {
		return urls
	}

	for _, URL := range urls {
		if URL == nil {
			continue
		}

		parsedURL, err := url.Parse(URL.Raw)
		if err != nil {
			htmldocLogger.Warn("unable to parse URL, keeping original URL", "err", err.Error(), "url", URL.Raw)
			continue
		}
		// According to the URL spec, we only need to encode the query part.
		// The path part should be left as utf8, we don't need to encode it.
		query := parsedURL.Query()
		newQuery := url.Values{}
		for key, values := range query {
			for _, value := range values {
				var encodedKey, encodedValue string
				if !isValidUTF8(key) {
					// If the key is not valid UTF-8, we do not encode it.
					encodedKey = key
				} else {
					encodedKey, err = enc.NewEncoder().String(key)
					if err != nil {
						htmldocLogger.Warn("unable to encode URL key", "err", err.Error(), "key", key, "url", URL.Raw)
						continue
					}
				}
				if !isValidUTF8(value) {
					encodedValue = value
				} else {
					encodedValue, err = enc.NewEncoder().String(value)
					if err != nil {
						htmldocLogger.Warn("unable to encode URL value", "err", err.Error(), "value", value, "url", URL.Raw)
						continue
					}
				}
				newQuery.Add(encodedKey, encodedValue)
			}
		}
		htmldocLogger.Warn("Encoded URL query", "url", URL.Raw, "enc", enc, "query", query, "new_query", newQuery, "new_query_string", newQuery.Encode())
		parsedURL.RawQuery = newQuery.Encode()
		URL.Raw = parsedURL.String()
	}

	return urls
}

func isValidUTF8(s string) bool {
	// Check if the string contains any invalid UTF-8 characters
	for i := 0; i < len(s); i++ {
		if s[i] < 0 || s[i] > 127 {
			return false
		}
	}
	return true
}
