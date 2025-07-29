package extractor

import (
	"bytes"
	"io"
	"net/url"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
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

	var enc encoding.Encoding
	htmlEnc, name, certain := charset.DetermineEncoding(preview, contentType)
	if htmlEnc != nil && htmlEnc != encoding.Nop {
		// The htmlEnc will use HTML escape sequences for runes that are not supported by the character set.
		// To get the original unwrapped encoding, we need to use htmlindex.
		enc, err = htmlindex.Get(name)
		if err != nil { // This should never happen
			htmldocLogger.Error("unable to get encoding from htmlindex", "err", err.Error(), "name", name)
			return r, err, nil, "", false
		}

		r = transform.NewReader(r, enc.NewDecoder())
	}

	return r, nil, enc, name, certain
}

// TransformDocument transforms the document of a URL by detecting its encoding and creating a utf-8 goquery document.
func TransformDocument(u *models.URL) (doc *goquery.Document, err error) {
	u.DocumentTransformMux.Lock()
	defer u.DocumentTransformMux.Unlock()

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
			htmldocLogger.Error("Transforming document step 1 failed", "url", u.String(), "err", err.Error())
			return nil, err
		}
		htmldocLogger.Debug("Transforming document step 2", "url", u.String(), "enc", enc, "enc_name", encName, "certain", certain)

		// Create the document from the converted reader
		document, err := goquery.NewDocumentFromReader(transformReader)
		if err != nil {
			htmldocLogger.Error("Transforming document step 2 failed", "url", u.String(), "err", err.Error())
			return nil, err
		}

		u.SetDocumentCache(document)
		u.SetDocumentEncoding(enc)
		htmldocLogger.Debug("Document transformed", "url", u.String(), "enc_name", encName)

	}

	return u.GetDocumentCache(), nil
}

// encodeNonUTF8QueryURLs encodes the query parameters of the given URLs using the specified encoding.
func encodeNonUTF8QueryURLs(urls []*models.URL, enc encoding.Encoding) []*models.URL {
	if enc == nil || enc == encoding.Nop {
		return urls
	}

	for _, URL := range urls {
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
				// If the key/value is not valid UTF-8, we do not encode it since it may already be encoded.
				// If encoding fails, we keep the original key/value.

				if !utf8.ValidString(key) {
					encodedKey = key
				} else {
					encodedKey, err = enc.NewEncoder().String(key)
					if err != nil {
						htmldocLogger.Warn("unable to encode query key", "err", err.Error(), "key", key, "url", URL.Raw)
						encodedKey = key
					}
				}
				if !utf8.ValidString(value) {
					encodedValue = value
				} else {
					encodedValue, err = enc.NewEncoder().String(value)
					if err != nil {
						htmldocLogger.Warn("unable to encode query value", "err", err.Error(), "value", value, "url", URL.Raw)
						encodedValue = value
					}
				}
				newQuery.Add(encodedKey, encodedValue)
			}
		}

		newQueryEncoded := newQuery.Encode()
		htmldocLogger.Debug("encoded URL query", "raw_url", URL.Raw, "enc", enc, "raw_query", parsedURL.RawQuery, "new_query", newQueryEncoded)

		parsedURL.RawQuery = newQueryEncoded
		URL.Raw = parsedURL.String()
	}

	return urls
}
