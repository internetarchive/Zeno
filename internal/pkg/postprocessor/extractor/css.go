package extractor

import (
	"io"

	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
	"go.baoshuo.dev/csslexer"
)

// The logger also used in the HTML extractor for CSS related logs.
var cssLogger = log.NewFieldedLogger(&log.Fields{
	"component": "postprocessor.extractor.css",
})

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

func (self *atRuleStateManager) Feed(tt csslexer.TokenType, v string) {
	if !self.inOKArea {
		self.Done()
		return
	}

	self.inOKArea = true

	// empty @layer definitions:
	// <https://www.w3.org/TR/css-cascade-5/#layer-empty>
	if tt == csslexer.AtKeywordToken {
		self.inAt = true
		switch v {
		case "charset", "layer":
			if self.inValidATImport {
				self.Done() // must not have any other valid at-rules or style rules between it and previous @import rules
				return
			}
			return
		case "import":
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

func (p *cssParser) processStringToken(v string) {
	_, _, inValidATImportRule := p.atManager.Report()

	if p.inAtImportRule {
		if !inValidATImportRule {
			return // skip invalid @import rules
		}
		p.atImportLinks = append(p.atImportLinks, v)
	} else if p.inURLFunction {
		p.links = append(p.links, v)
	}
}

func (p *cssParser) processUrlToken(v string) {
	_, _, inValidATImportRule := p.atManager.Report()

	if p.inAtImportRule {
		if !inValidATImportRule {
			return // skip invalid @import rules
		}
		p.atImportLinks = append(p.atImportLinks, v)
	} else {
		p.links = append(p.links, v)
	}
}

func (p *cssParser) processToken(t csslexer.Token) {
	switch t.Type {
	case csslexer.FunctionToken:
		if t.Value == "url" {
			p.inURLFunction = true
		}
	case csslexer.AtKeywordToken:
		if t.Value == "import" {
			p.inAtImportRule = true
		}
	case csslexer.SemicolonToken:
		p.inAtImportRule = false
	case csslexer.RightParenthesisToken:
		p.inURLFunction = false // end of url() function
	case csslexer.StringToken:
		p.processStringToken(t.Value)
	case csslexer.UrlToken:
		p.processUrlToken(t.Value)
	}
}

func (p *cssParser) parse() ([]string, []string) {
	for {
		tok := p.lexer.Next()
		if tok.Type == csslexer.EOFToken {
			return p.links, p.atImportLinks
		}
		if tok.Type == csslexer.WhitespaceToken || tok.Type == csslexer.CommentToken {
			continue // skip whitespace and comments
		}

		p.atManager.Feed(tok.Type, tok.Value)

		p.processToken(tok)
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
	bodyBytes, err := io.ReadAll(URL.GetBody())
	if err != nil {
		return nil, nil, err
	}
	sLinks, sAtImportLinks := newCSSParser([]rune(string(bodyBytes)), false).parse()
	return toURLs(sLinks), toURLs(sAtImportLinks), nil
}
