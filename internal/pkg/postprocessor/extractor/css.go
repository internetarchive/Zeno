package extractor

import (
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
	"go.baoshuo.dev/csslexer"
)

// The logger also used in the HTML extractor for CSS related logs.
var cssLogger = log.NewFieldedLogger(&log.Fields{
	"component": "postprocessor.extractor.css",
})

// Assuming the input [data] is already trimmed and does not contain any leading
// or trailing whitespace, quotes, "url(", or ")".
// If [isString] is true, the [data] is from a <string-token>, else it is from a <url-token>.
//
// Don't feed <bad-url-token> or <bad-string-token> data to this function, as it will not handle them correctly.
//
// returns: The unescaped data value
//
// References:
// <https://www.w3.org/TR/css-syntax-3/#url-token-diagram>
// <https://www.w3.org/TR/css-syntax-3/#consume-url-token>
//
// <https://www.w3.org/TR/css-syntax-3/#string-token-diagram>
// <https://www.w3.org/TR/css-syntax-3/#consume-string-token>
//
// <https://www.w3.org/TR/css-syntax-3/#consume-escaped-code-point>
// <https://www.w3.org/International/questions/qa-escapes#cssescapes>
func parseStringOrURLTokenData(data []rune, isString bool) string {
	// In this function, all the "qouted" comments are references to the CSS spec.

	// Fast path for unescaped URLs/strings
	if len(data) == 0 || !slices.Contains(data, '\\') {
		return string(data)
	}

	var value strings.Builder
	value.Grow(len(data))

	// Slow path for escaped URLs/strings
	pos := 0
	for pos < len(data) {
		c := data[pos]

		if c == '\\' { // backslash escape start
			pos++ // advance to the next code point

			// 1. "If the next input code point is EOF, do nothing. "
			if pos >= len(data) {
				break
			}

			// For <url-token>: https://www.w3.org/TR/css-syntax-3/#check-if-two-code-points-are-a-valid-escape
			// ... "if the second code point is a newline, return false" (<bad-url-token>)...
			// For <string-token>: https://www.w3.org/TR/css-syntax-3/#consume-string-token
			// ... "if the next input code point is a newline, consume it" ...
			//
			// Yes, this is the only difference between <url-token> and <string-token> handling.
			//
			// 2. "Otherwise, if the next input code point is a newline, consume it."
			if isString && isNewline(data[pos]) {
				pos++
				continue
			}

			// 3. "Otherwise, (the stream starts with a valid escape) consume an escaped
			// code point and append the returned code point to the <string-token>’s value."
			// Since we are not handling <bad-url-token> or <bad-string-token>,
			// we can assume that the next code point is a valid escape.

			// 3.1.1  "Consume as many hex digits as possible, but no more than 5. Note that
			// this means 1-6 hex digits have been consumed in total"
			hexDigits := make([]rune, 0, 6)
			for pos < len(data) && len(hexDigits) < 6 { // max 6 hex digits
				hc := data[pos]

				if (hc >= '0' && hc <= '9') || (hc >= 'a' && hc <= 'f') || (hc >= 'A' && hc <= 'F') {
					hexDigits = append(hexDigits, hc)
					pos++
				} else {
					// not a hex digit, break without advancing
					break
				}
			}

			if len(hexDigits) != 0 { // hex digits found after the backslash
				// 3.1.2 "Interpret the hex digits as a hexadecimal number.
				// If this number is zero, or is for a surrogate, or is greater than
				// the maximum allowed code point, return U+FFFD REPLACEMENT CHARACTER (�).
				// Otherwise, return the code point with that value. "
				value.WriteRune(sanitizeRune(hexToRune(hexDigits)))

				if pos >= len(data) { // EOF after hex digits
					break
				} else if isWhitespace(data[pos]) { // whitespace after hex digits
					// 3.1.1 "If the next input code point is whitespace, consume it as well"
					// Bonus: If you wander why do not append the whitespace to the value:
					// https://www.w3.org/International/questions/qa-escapes#cssescapes
					// "Because any white-space following the hexadecimal number is swallowed up as part of the escape"
					pos++
				}
			} else { // no hex digits after the backslash
				if pos >= len(data) { // EOF after backslash, this is a edge case for <string-token>.
					// <https://github.com/w3c/csswg-drafts/issues/3182>,
					// <https://lists.w3.org/Archives/Public/www-style/2013Jun/0683.html>
					// <https://wpt.live/css/css-syntax/escaped-eof.html>
					// "the correct way to handle an "escaped EOF" (that is, a stylesheet ending in a \)
					// is to produce a U+FFFD, except in strings, where it is ignored"
					// This will never happen for <url-token> as if it happens it will produce a <bad-url-token>.

					// For <string-token>, let's just ignore it, as the spec says.
					break
				} else {
					// 3.3 "anything else: Return the current input code point. "
					//  Append the current input code point to the  value.
					value.WriteRune(data[pos])
					pos++
				}
			}
		} else {
			value.WriteRune(c)
			pos++
		}
	}

	return value.String()
}

var urlTokenPrefix = []rune("url(")

// Trim leading and trailing whitespace
func TrimSpace(data []rune) []rune {
	start := 0
	for start < len(data) && isWhitespace(data[start]) {
		start++
	}

	end := len(data)
	for end > start && isWhitespace(data[end-1]) {
		end--
	}

	return data[start:end]
}

func parseURLTokenData(data []rune) string {
	return parseStringOrURLTokenData(TrimSpace(data[len(urlTokenPrefix):len(data)-1]), false)
}

func parseStringTokenData(data []rune) string {
	return parseStringOrURLTokenData(TrimSpace(data[1:len(data)-1]), true)
}

func hexToRune(hexDigits []rune) rune {
	if len(hexDigits) == 0 {
		panic("no hex digits provided") // never happen
	}

	uPoint, err := strconv.ParseUint(string(hexDigits), 16, 32)
	if uPoint > unicode.MaxRune || err != nil {
		return '\ufffd'
	}

	return rune(uPoint)
}

// If this number is zero, or is for a surrogate, or is greater than the maximum allowed code point, return U+FFFD REPLACEMENT CHARACTER
// https://www.w3.org/TR/css-syntax-3/#consume-escaped-code-point
func sanitizeRune(r rune) rune {
	if r == 0 || (r >= 0xD800 && r <= 0xDBFF) || (r >= 0xDC00 && r <= 0xDFFF) || r > unicode.MaxRune {
		return '\ufffd'
	}
	return r
}

var allowedPrecedeAtRules = [][]rune{
	[]rune("@charset"),
	[]rune("@layer"),
}

var atImportRule = []rune("@import")

type atRuleStateManager struct {
	inOKArea        bool // Whether the current area is allowed to contain @import rules
	inAt            bool // Whether the current state is in an @-rule
	inValidATImport bool // Whether the current state is in an @import rule
}

func newAtRuleStateMnager() *atRuleStateManager {
	return &atRuleStateManager{
		inOKArea:        true,  // Initially, the area is allowed to contain @import rules
		inAt:            false, // Initially, we are not in an @-rule
		inValidATImport: false, // Initially, we are not in an @import rule
	}
}

func (self *atRuleStateManager) Feed(tt csslexer.TokenType, data []rune) {
	if !self.inOKArea {
		self.Done()
		return
	}

	self.inOKArea = true

	// empty @layer definitions:
	// <https://www.w3.org/TR/css-cascade-5/#layer-empty>
	if tt == csslexer.AtKeywordToken {
		self.inAt = true
		for _, rule := range allowedPrecedeAtRules {
			if equalFold(data, rule) {
				if self.inValidATImport {
					self.Done() // must not have any other valid at-rules or style rules between it and previous @import rules
					return
				}
				return
			}
		}
		if equalFold(data, atImportRule) {
			self.inValidATImport = true // @import rule
			return
		}
	}

	if self.inAt {
		// NOTE: This is NOT an empty @layer definition:
		// @layer default {
		//   audio[controls] {
		//     display: block;
		//   }
		// }
		if tt == csslexer.LeftBraceToken {
			self.inOKArea = false
			return
		}
	}
}

func (self *atRuleStateManager) Done() {
	self.inOKArea, self.inAt, self.inValidATImport = false, false, false
}

func (self *atRuleStateManager) Report() (inOKArea, inAt, inValidATImport bool) {
	return self.inOKArea, self.inAt, self.inValidATImport
}

type cssParser struct {
	lexer          *csslexer.Lexer
	atManager      *atRuleStateManager
	inURLFunction  bool
	inAtImportRule bool
	links          []string
	atImportLinks  []string
}

func newCSSParser(css []rune, inline bool) *cssParser {
	p := &cssParser{
		lexer:         csslexer.NewLexer(csslexer.NewInputRunes(css)),
		atManager:     newAtRuleStateMnager(),
		links:         make([]string, 0, 16),
		atImportLinks: make([]string, 0, 4),
	}

	if inline {
		p.atManager.Done() // disable @import for inline CSS
	}

	return p
}

func (p *cssParser) processFunctionToken(traw []rune) {
	if hasPrefixFold(traw, urlTokenPrefix) { // trailing space may be present
		p.inURLFunction = true
	}
}

func (p *cssParser) processAtKeywordToken(traw []rune) {
	if equalFold(traw, atImportRule) {
		p.inAtImportRule = true
	}
}

func (p *cssParser) processSemicolonToken() {
	p.inAtImportRule = false
}

func (p *cssParser) processRightParenthesisToken() {
	p.inURLFunction = false // end of url() function
}

func (p *cssParser) processStringToken(traw []rune) {
	_, _, inValidATImportRule := p.atManager.Report()

	if p.inAtImportRule {
		if !inValidATImportRule {
			return // skip invalid @import rules
		}
		p.atImportLinks = append(p.atImportLinks, parseStringTokenData(traw))
	} else if p.inURLFunction {
		p.links = append(p.links, parseStringTokenData(traw))
	}
}

func (p *cssParser) processUrlToken(traw []rune) {
	_, _, inValidATImportRule := p.atManager.Report()

	if p.inAtImportRule {
		if !inValidATImportRule {
			return // skip invalid @import rules
		}
		p.atImportLinks = append(p.atImportLinks, parseURLTokenData(traw))
	} else {
		p.links = append(p.links, parseURLTokenData(traw))
	}
}

func (p *cssParser) processToken(tt csslexer.TokenType, traw []rune) {
	switch tt {
	case csslexer.FunctionToken:
		p.processFunctionToken(traw)
	case csslexer.AtKeywordToken:
		p.processAtKeywordToken(traw)
	case csslexer.SemicolonToken:
		p.processSemicolonToken()
	case csslexer.RightParenthesisToken:
		p.processRightParenthesisToken()
	case csslexer.StringToken:
		p.processStringToken(traw)
	case csslexer.UrlToken:
		p.processUrlToken(traw)
	}
}

func (p *cssParser) parse() ([]string, []string) {
	for {
		tok := p.lexer.Next()
		if tok.Type == csslexer.WhitespaceToken || tok.Type == csslexer.CommentToken {
			continue // skip whitespace and comments
		}

		p.atManager.Feed(tok.Type, tok.Data)

		if tok.Type == csslexer.EOFToken {
			return p.links, p.atImportLinks
		}

		p.processToken(tok.Type, tok.Data)
	}
}

// https://html.spec.whatwg.org/multipage/links.html#link-type-stylesheet:process-the-linked-resource
// According to the spec, we should only check the Content-Type header if the resource is came from a HTTP(S) request.
func IsCSS(URL *models.URL) bool {
	return URL.GetMIMEType().Is("text/css")
}

func ExtractFromStringCSS(css string, inline bool) (links []string, atImportLinks []string) {
	parser := newCSSParser([]rune(css), inline)
	return parser.parse()
}

// ExtractFromURLCSS extracts URLs from a CSS URL
func ExtractFromURLCSS(URL *models.URL) (links []*models.URL, atImportLinks []*models.URL, err error) {
	defer URL.RewindBody()
	cssBody := strings.Builder{}
	if _, err := io.Copy(&cssBody, URL.GetBody()); err != nil {
		return nil, nil, err
	}
	sLinks, sAtImportLinks := ExtractFromStringCSS(cssBody.String(), false)
	return toURLs(sLinks), toURLs(sAtImportLinks), nil
}
