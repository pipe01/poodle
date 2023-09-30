package lexer

import "fmt"

type TokenType int

const (
	TokenIdentifier TokenType = iota
	TokenNewLine
	TokenInlineText
	TokenImportPath

	TokenParenOpen
	TokenParenClose

	TokenEquals
	TokenDot
	TokenComma
	TokenPlus
	TokenHashtag
	TokenColon
	TokenInterpolationStart
	TokenQuestionMark
	TokenExclamationPoint
	TokenPipe

	TokenCommentStart
	TokenCommentStartBuffered
	TokenCommentText

	TokenClassName
	TokenID

	TokenKeyword
	TokenAttributeName
	TokenQuotedString

	TokenGoExpr
	TokenGoBlock

	TokenEOF
)

func (t TokenType) String() string {
	switch t {
	case TokenIdentifier:
		return "Identifier"
	case TokenNewLine:
		return "Newline"
	case TokenInlineText:
		return "Inline text"
	case TokenImportPath:
		return "Import path"

	case TokenParenOpen:
		return "Parentheses open"
	case TokenParenClose:
		return "Parentheses close"

	case TokenEquals:
		return "Equals"
	case TokenDot:
		return "Dot"
	case TokenComma:
		return "Comma"
	case TokenPlus:
		return "Plus"
	case TokenHashtag:
		return "Hashtag"
	case TokenColon:
		return "Colon"
	case TokenInterpolationStart:
		return "Interpolation start"
	case TokenQuestionMark:
		return "Question mark"
	case TokenExclamationPoint:
		return "Exclamation point"
	case TokenPipe:
		return "Pipe"

	case TokenCommentStart:
		return "Comment start"
	case TokenCommentStartBuffered:
		return "Comment start buffered"
	case TokenCommentText:
		return "Comment text"

	case TokenClassName:
		return "Class name"
	case TokenID:
		return "ID"

	case TokenKeyword:
		return "Keyword"
	case TokenAttributeName:
		return "Attribute name"
	case TokenQuotedString:
		return "Quoted string"

	case TokenGoExpr:
		return "Go expression"
	case TokenGoBlock:
		return "Go block"

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
