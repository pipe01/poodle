package lexer

import "fmt"

type TokenType int

const (
	TokenTagName TokenType = iota
	TokenNewLine
	TokenTagInlineText

	TokenParenOpen
	TokenParenClose
	TokenBraceOpen
	TokenBraceClose

	TokenEquals
	TokenDot
	TokenHashtag
	TokenAtSign

	TokenClassName
	TokenID

	TokenAttributeName
	TokenQuotedString

	TokenGoExpr
)

func (t TokenType) String() string {
	switch t {
	case TokenTagName:
		return "Tag name"
	case TokenNewLine:
		return "\\n"
	case TokenTagInlineText:
		return "Inline text"

	case TokenParenOpen:
		return "Parentheses open"
	case TokenParenClose:
		return "Parentheses close"
	case TokenBraceOpen:
		return "Brace open"
	case TokenBraceClose:
		return "Brace close"

	case TokenEquals:
		return "Equals"
	case TokenDot:
		return "Dot"
	case TokenHashtag:
		return "Hashtag"
	case TokenAtSign:
		return "At sign"

	case TokenClassName:
		return "Class name"
	case TokenID:
		return "ID"

	case TokenAttributeName:
		return "Attribute name"
	case TokenQuotedString:
		return "ID"

	case TokenGoExpr:
		return "Quoted string"
	}

	return "<unknown>"
}

type Token struct {
	Type     TokenType
	Start    Location
	Depth    int
	Contents string
}

type Location struct {
	File string

	// 0-based
	Line, Column int
}

func (l *Location) String() string {
	return fmt.Sprintf("%s:%d:%d", l.File, l.Line+1, l.Column+1)
}
