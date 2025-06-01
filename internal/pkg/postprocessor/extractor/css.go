package extractor

import (
	"bytes"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

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
func parseStringOrURLTokenData(data []byte, isString bool) []byte {
	// In this function, all the "qouted" comments are references to the CSS spec.

	// Fast path for unescaped URLs/strings
	if len(data) == 0 || !bytes.Contains(data, []byte{'\\'}) {
		return data
	}

	// Slow path for escaped URLs/strings
	value := make([]byte, 0, len(data))
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
			if isString && parse.IsNewline(data[pos]) {
				pos++
				continue
			}

			// 3. "Otherwise, (the stream starts with a valid escape) consume an escaped
			// code point and append the returned code point to the <string-token>’s value."
			// Since we are not handling <bad-url-token> or <bad-string-token>,
			// we can assume that the next code point is a valid escape.

			// 3.1.1  "Consume as many hex digits as possible, but no more than 5. Note that
			// this means 1-6 hex digits have been consumed in total"
			hexDigits := make([]byte, 0, 6)
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
				value = append(value, runeToBytes(sanitizeRune(hexToRune(hexDigits)))...)

				if pos >= len(data) { // EOF after hex digits
					break
				} else if parse.IsWhitespace(data[pos]) { // whitespace after hex digits
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
					value = append(value, data[pos])
					pos++
				}
			}
		} else {
			value = append(value, c)
			pos++
		}
	}

	return value
}

func parseURLTokenData(data []byte) []byte {
	return parseStringOrURLTokenData(data, false)
}

func parseStringTokenData(data []byte) []byte {
	return parseStringOrURLTokenData(data, true)
}

func runeToBytes(r rune) []byte {
	buf := make([]byte, utf8.UTFMax)
	n := utf8.EncodeRune(buf, r)
	return buf[:n]
}

func hexToRune(hexDigits []byte) rune {
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

func urlTokenToValue(t css.Token) string {
	if t.TokenType != css.URLToken {
		return ""
	}
	end := len(t.Data) - 1
	if t.Data[len(t.Data)-1] != ')' { // closing parenthesis
		end = len(t.Data)
	}
	data := parse.TrimWhitespace(t.Data[4:end]) // remove "url(" and ")"

	delim := byte(0)
	if len(data) > 0 && (data[0] == '\'' || data[0] == '"') { // quoted url
		delim = data[0]
		end = len(data) - 1
		if data[end] != delim { // unclosed quote
			end = len(data)
		}
		data = data[1:end] // remove the quotes
	}

	if delim == byte(0) { // unquoted url
		return string(parseURLTokenData(data))
	} else if delim == '\'' || delim == '"' { // quoted url
		return string(parseStringTokenData(data))
	} else {
		panic("invalid delimiter") // never happen
	}
}

// TODO: "@import" rule
func stringTokenToValue(t css.Token) string {
	if t.TokenType != css.StringToken {
		return ""
	}
	end := len(t.Data) - 1
	if t.Data[len(t.Data)-1] != t.Data[0] { // closing quote
		end = len(t.Data)
	}
	return string(parseStringOrURLTokenData(t.Data[1:end], true)) // remove the quotes
}

func CSS(cssBody string, inline bool) []string {
	// TODO: "@import" rule
	// TODO: separate CSS file

	var links []string
	p := css.NewParser(parse.NewInput(bytes.NewBufferString(cssBody)), inline)
	for {
		gt, tt, data := p.Next()
		if tt == css.URLToken {
			links = append(links, urlTokenToValue(css.Token{TokenType: tt, Data: data}))
		}

		if gt == css.ErrorGrammar {
			break
		} else if gt == css.AtRuleGrammar || gt == css.BeginAtRuleGrammar || gt == css.BeginRulesetGrammar || gt == css.DeclarationGrammar {
			for _, tk := range p.Values() {
				if tk.TokenType == css.URLToken {
					links = append(links, urlTokenToValue(tk))
				}
			}
		} else {
		}
	}
	return links
}
