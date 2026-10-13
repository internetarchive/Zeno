package extractor

import (
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/internetarchive/Zeno/v2/internal/pkg/log"
	"github.com/internetarchive/Zeno/v2/pkg/models"
)

var (
	// https://tc39.es/ecma262/#table-line-terminator-code-points
	isECMALineBreak = func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\u2028' || r == '\u2029'
	}
	// https://tc39.es/ecma262/#sec-white-space
	isECMAWhiteSpace = func(r rune) bool {
		switch r {
		case '\t', '\v', '\f', '\uFEFF':
			return true
		default:
			// category "Space Separator" (https://www.compart.com/en/unicode/category/Zs)
			return unicode.Is(unicode.Zs, r)
		}
	}
	sourceMapURL = regexp.MustCompile(`[@#]\s*sourceMappingURL\s*=\s*(\S+)`)
)

// 7.1.2.1. Extraction methods for JavaScript sources without parsing (https://tc39.es/ecma426/2024/#extraction-javascript)
// also supports CSS (see 7.1.2.2)
func extractSourceMapURL(src string) string {
	lines := strings.FieldsFunc(src, isECMALineBreak)

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimLeftFunc(lines[i], isECMAWhiteSpace)

		if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
			// opt. clear the */ suffix for multiline comments/CSS
			line = strings.TrimSuffix(line, "*/")
			if m := sourceMapURL.FindStringSubmatch(line); m != nil {
				return m[1]
			}
		}
	}
	return ""
}

func IsJSOrCss(URL *models.URL) bool {
	extractor := SourceMapExtractor{}
	return extractor.Match(URL)
}

// unified interface for AssetExtractors for later refactoring (https://github.com/internetarchive/Zeno/pull/551)
type SourceMapExtractor struct{}

func (SourceMapExtractor) Match(URL *models.URL) bool {
	return URL.GetMIMEType().Is("text/javascript") || URL.GetMIMEType().Is("text/css")
}

func (SourceMapExtractor) Extract(item *models.Item) (assets, outlinks []*models.URL, err error) {
	URL := item.GetURL()
	defer URL.RewindBody()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.extractor.SourceMap",
	})

	var rawSourceMappingURL string

	// try reading sourcemap || x-sourcemap from response headers,
	// takes precedence over inline source map annotations
	if response := URL.GetResponse(); response != nil { // headless items have a nil response
		for _, key := range []string{"SourceMap", "X-SourceMap"} {
			if rawSourceMappingURL = response.Header.Get(key); rawSourceMappingURL != "" {
				break
			}
		}
	}

	if rawSourceMappingURL == "" {
		content, err := io.ReadAll(URL.GetBody())
		if err != nil {
			logger.Error("unable to ReadAll item body: %s", err)
			return assets, outlinks, err
		}

		rawSourceMappingURL = extractSourceMapURL(string(content))
	}

	if rawSourceMappingURL == "" {
		return assets, outlinks, nil
	}

	resolvedURL, err := resolveURL(rawSourceMappingURL, URL)
	if err != nil {
		var baseURL string
		if URL.GetBase() != nil {
			baseURL = URL.GetBase().String()
		}

		logger.Debug("unable to resolve URL", "error", err, "base_url", baseURL, "target", rawSourceMappingURL)
		return assets, outlinks, nil
	}

	assets = append(assets, &models.URL{Raw: resolvedURL})
	return assets, outlinks, nil
}
