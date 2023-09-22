package lexer

import "fmt"

type TokenType int

const (
	TokenTagName TokenType = iota
	TokenNewLine
	TokenWhitespace
	TokenTagInlineText

	TokenParenOpen
	TokenParenClose
	TokenBraceOpen
	TokenBraceClose

	TokenEquals
	TokenDot
	TokenHashtag
	TokenAtSign
	TokenPipe

	TokenCommentStart
	TokenCommentText

	TokenClassName
	TokenID

	TokenAttributeName
	TokenQuotedString

	TokenGoExpr

	TokenEOF
)

func (t TokenType) String() string {
	switch t {
	case TokenTagName:
		return "Tag name"
	case TokenNewLine:
		return "Newline"
	case TokenWhitespace:
		return "Whitespace"
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
	case TokenPipe:
		return "Pipe"

	case TokenCommentStart:
		return "Comment start"
	case TokenCommentText:
		return "Comment text"

	case TokenClassName:
		return "Class name"
	case TokenID:
		return "ID"

	case TokenAttributeName:
		return "Attribute name"
	case TokenQuotedString:
		return "Quoted string"

	case TokenGoExpr:
		return "Go expression"

	case TokenEOF:
		return "EOF"
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
