package lexer

type TokenType int

const (
	TokenTagName TokenType = iota
	TokenNewLine
	TokenTagInlineText

	TokenParenOpen
	TokenParenClose
	TokenEquals
	TokenDot
	TokenHashTag

	TokenClassName
	TokenID

	TokenAttributeName
	TokenQuotedString

	TokenEOF
)

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
