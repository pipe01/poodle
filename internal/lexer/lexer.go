package lexer

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"unicode"
)

const (
	interpolationChar = '@'
	debugPrint        = false
)

type LexerError struct {
	Inner    error
	Location Location
}

func (e *LexerError) Error() string {
	return fmt.Sprintf("%s at %s", e.Inner, &e.Location)
}

type UnexpectedRuneError struct {
	Got      rune
	Expected string
}

func (e *UnexpectedRuneError) Error() string {
	return fmt.Sprintf("expected %s, found %q", e.Expected, e.Got)
}

type stateFunc func() stateFunc

type Lexer struct {
	fileName string
	file     []byte

	tokens chan Token
	r      *bufio.Reader

	str      []rune
	strStart Location

	byteIndex, runeIndex int
	line, col            int
	lastLineCol          int
	lastRuneSize         int
	depth                int

	err *LexerError
}

func New(file []byte, fileName string) *Lexer {
	tks := make(chan Token, 1)

	lexer := &Lexer{
		tokens:   tks,
		file:     file,
		fileName: fileName,
		r:        bufio.NewReader(bytes.NewReader(file)),
		str:      make([]rune, 0, 200),
		strStart: Location{
			File: fileName,
		},
	}

	go func() {
		defer close(tks)

		state := lexer.lexIndentation()
		for state != nil {
			state = state()

			if lexer.err != nil {
				return
			}
		}

		tks <- Token{
			Type: TokenEOF,
			Start: Location{
				File:   lexer.fileName,
				Line:   lexer.line,
				Column: lexer.col + 1,
			},
		}
	}()

	return lexer
}

func (l *Lexer) Next() (*Token, error) {
	t, ok := <-l.tokens
	if !ok {
		return nil, l.err
	}

	return &t, nil
}

func (l *Lexer) take() (rune, bool) {
	r, size, err := l.r.ReadRune()
	if err != nil {
		return 0, true
	}

	l.str = append(l.str, r)
	l.lastRuneSize = size

	l.col++
	l.runeIndex++
	l.byteIndex += size

	if r == '\n' {
		l.line++
		l.lastLineCol = l.col
		l.col = 0
	}

	if debugPrint {
		fmt.Printf("take %q\n", r)
	}

	return r, false
}

func (l *Lexer) takeRune(exp rune) bool {
	r, eof := l.take()
	if eof {
		return false
	}
	if r != exp {
		l.lexUnexpected(r, fmt.Sprintf("%q", exp))
		return false
	}

	return true
}

func (l *Lexer) rewindRune() {
	err := l.r.UnreadRune()
	if err != nil {
		panic("cannot unread rune")
	}

	l.str = l.str[:len(l.str)-1]

	l.byteIndex -= l.lastRuneSize
	l.runeIndex--

	if l.col == 0 {
		l.line--
		l.col = l.lastLineCol
	} else {
		l.col--
	}

	if debugPrint {
		fmt.Println("rewind")
	}
}

func (l *Lexer) emit(typ TokenType) {
	l.tokens <- Token{
		Type:     typ,
		Start:    l.strStart,
		Contents: string(l.str),
		Depth:    l.depth,
	}

	l.discard()
}

func (l *Lexer) discard() {
	l.strStart = Location{
		File:   l.fileName,
		Line:   l.line,
		Column: l.col,
	}
	l.str = l.str[:0]
}

func (l *Lexer) skipWhitespace() {
	for {
		r, eof := l.take()
		if eof {
			return
		}

		switch r {
		case ' ', '\t':
		default:
			l.rewindRune()
			return
		}
	}
}

func (l *Lexer) isEmpty() bool {
	return len(l.str) == 0
}

func (l *Lexer) lexError(err error) stateFunc {
	return func() stateFunc {
		l.err = &LexerError{
			Inner:    err,
			Location: l.strStart,
		}
		return nil
	}
}

func (l *Lexer) lexUnexpected(got rune, expected string) stateFunc {
	return l.lexError(&UnexpectedRuneError{
		Got:      got,
		Expected: expected,
	})
}

func (l *Lexer) lexIndentation() stateFunc {
	l.depth = 0

	for {
		r, eof := l.take()
		if eof {
			return nil
		}

		switch r {
		case ' ':
			return l.lexError(errors.New("spaces indentation is not allowed"))
		case '\t':
			l.depth++
		case '\n':
			l.discard()
			l.depth = 0
		default:
			l.rewindRune()
			l.discard()
			return l.lexLineStart
		}
	}
}

func (l *Lexer) lexNewLine() stateFunc {
	for {
		r, eof := l.take()
		if eof {
			return nil
		}

		if !isNewLine(r) {
			l.rewindRune()
			l.emit(TokenNewLine)
			return l.lexIndentation
		}
	}
}

func (l *Lexer) lexForcedNewLine() stateFunc {
	foundSome := false

	for {
		r, eof := l.take()
		if eof {
			return nil
		}

		if !isNewLine(r) {
			if !foundSome {
				return l.lexUnexpected(r, "a new line")
			}

			l.rewindRune()
			l.emit(TokenNewLine)
			return l.lexIndentation
		}

		foundSome = true
	}
}

func (l *Lexer) lexLineStart() stateFunc {
	r, eof := l.take()
	if eof {
		return nil
	}
	switch r {
	case interpolationChar:
		l.emit(TokenAtSign)
		return l.lexInterpolation(l.lexForcedNewLine)

	case '.': // Shortcut div with class
		l.emit(TokenDot)
		return l.lexClassName

	case '#': // Shortcut div with ID
		l.emit(TokenHashtag)
		return l.lexID

	case '|':
		l.emit(TokenPipe)
		return l.lexWhitespacedInlineContent
	}

	for {
		r, eof := l.take()
		if eof {
			return nil
		}

		if !(isASCIIDigit(r) || isASCIILetter(r) || r == '-' || r == '_') {
			l.rewindRune()

			if l.isEmpty() {
				return l.lexUnexpected(r, "a tag name")
			}

			break
		}
	}

	l.emit(TokenTagName)
	return l.lexAfterTag
}

func (l *Lexer) lexAfterTag() stateFunc {
	r, eof := l.take()
	if eof {
		return nil
	}

	switch r {
	case ' ':
		l.rewindRune()
		return l.lexWhitespacedInlineContent

	case '(':
		l.emit(TokenParenOpen)
		return l.lexAttributeName

	case '.':
		l.emit(TokenDot)
		return l.lexClassName

	case '#':
		l.emit(TokenHashtag)
		return l.lexID

	default:
		if isNewLine(r) {
			l.emit(TokenNewLine)
			return l.lexIndentation
		}
	}

	return l.lexUnexpected(r, "valid tag qualifiers, content or a newline")
}

func (l *Lexer) lexClassName() stateFunc {
	r, eof := l.take()
	if eof {
		return nil
	}
	if !isASCIILetter(r) && r != '-' && r != '_' {
		return l.lexUnexpected(r, "a valid CSS name first character")
	}

	for {
		r, eof := l.take()
		if eof {
			return nil
		}

		if !isASCIILetter(r) && !isASCIIDigit(r) && r != '-' && r != '_' {
			l.rewindRune()
			l.emit(TokenClassName)
			break
		}
	}

	return l.lexAfterTag
}

func (l *Lexer) lexID() stateFunc {
	for {
		r, eof := l.take()
		if eof {
			return nil
		}

		if !isASCIILetter(r) && !isASCIIDigit(r) && r != '-' && r != '_' {
			l.rewindRune()

			if l.isEmpty() {
				return l.lexUnexpected(r, "an ID")
			}

			l.emit(TokenID)
			break
		}
	}

	return l.lexAfterTag
}

func (l *Lexer) lexTagInlineContent() stateFunc {
	for {
		r, eof := l.take()
		if eof {
			return nil
		}

		if r == interpolationChar {
			// Rewind and emit pending inline text, if any
			l.rewindRune()
			if !l.isEmpty() {
				l.emit(TokenTagInlineText)
			}

			// Take first interpolation char again
			l.take()

			// Check if the next char if also the interpolation char
			r, eof := l.take()
			if eof {
				return nil
			}
			if r == interpolationChar {
				// If it is, we rewind back to the first one, discard it, then take
				// the second one again and continue the loop in order to include it
				// in the next inline text emit
				l.rewindRune()
				l.discard()
				l.take()
				continue
			}

			l.rewindRune()
			l.emit(TokenAtSign)
			return l.lexInterpolation(l.lexTagInlineContent)
		}

		if isNewLine(r) {
			l.rewindRune()
			if !l.isEmpty() {
				l.emit(TokenTagInlineText)
			}
			return l.lexNewLine
		}
	}
}

func (l *Lexer) lexAttributeName() stateFunc {
	l.skipWhitespace()
	l.discard()

	for {
		r, eof := l.take()
		if eof {
			return nil
		}

		if !unicode.IsLetter(r) {
			l.rewindRune()

			if l.isEmpty() {
				return l.lexAfterAttributes
			}

			l.emit(TokenAttributeName)
			return l.lexAttributeEqual
		}
	}
}

func (l *Lexer) lexAttributeEqual() stateFunc {
	l.skipWhitespace()
	l.discard()

	if !l.takeRune('=') {
		return nil
	}

	l.emit(TokenEquals)
	return l.lexAttributeValue
}

func (l *Lexer) lexAttributeValue() stateFunc {
	l.skipWhitespace()
	l.discard()

	if !l.takeRune('"') {
		return nil
	}

	for {
		r, eof := l.take()
		if eof {
			return nil
		}
		if r == '"' {
			break
		}
	}

	l.emit(TokenQuotedString)

	return l.lexAttributeName
}

func (l *Lexer) lexAfterAttributes() stateFunc {
	if !l.takeRune(')') {
		return nil
	}

	l.emit(TokenParenClose)
	return l.lexAfterTag
}

func (l *Lexer) lexInterpolation(returnTo stateFunc) stateFunc {
	return func() stateFunc {
		r, eof := l.take()
		if eof {
			return nil
		}

		if r == '{' {
			l.emit(TokenBraceOpen)
			return l.lexInterpolationBlock(returnTo)
		}

		l.rewindRune()
		return l.lexInterpolationExpr(returnTo)
	}
}

func (l *Lexer) lexInterpolationExpr(returnTo stateFunc) stateFunc {
	return func() stateFunc {
		r, eof := l.take()
		if eof {
			return nil
		}
		if r != '_' && !unicode.IsLetter(r) {
			return l.lexUnexpected(r, "a valid Go identifier first character")
		}

		for {
			r, eof := l.take()
			if eof {
				return nil
			}
			if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				l.rewindRune()
				break
			}
		}

		l.emit(TokenGoExpr)
		return returnTo
	}
}

// This function uses the built-in Go token scanner to intelligently find
// the closing right brace, avoiding things like braces inside strings
func (l *Lexer) lexInterpolationBlock(returnTo stateFunc) stateFunc {
	return func() stateFunc {
		startByteIndex := l.byteIndex
		textStart := l.file[startByteIndex:]

		var scan scanner.Scanner

		fileSet := token.NewFileSet()
		f := fileSet.AddFile(l.fileName, 1, len(textStart))

		scan.Init(f, textStart, func(pos token.Position, msg string) {
			col := pos.Column - 1
			if pos.Line == 1 {
				col += l.col
			}

			l.err = &LexerError{
				Inner: fmt.Errorf("scan Go code: %s", msg),
				Location: Location{
					File:   l.fileName,
					Line:   l.line + pos.Line - 1,
					Column: col,
				},
			}
		}, 0)

		bracesCount := 1
		var rbracePos int

	loop:
		for {
			pos, tok, _ := scan.Scan()

			switch tok {
			case token.LBRACE:
				bracesCount++

			case token.RBRACE:
				bracesCount--
				if bracesCount == 0 {
					rbracePos = int(pos) - f.Base()
					break loop
				}

			case token.EOF:
				return l.lexError(errors.New("cannot find expression end brace"))
			}
		}

		for l.byteIndex < startByteIndex+rbracePos {
			l.take()
		}
		l.emit(TokenGoExpr)

		if !l.takeRune('}') {
			return nil
		}
		l.emit(TokenBraceClose)

		return returnTo
	}
}

func (l *Lexer) lexWhitespacedInlineContent() stateFunc {
	r, eof := l.take()
	if eof {
		return nil
	}
	if r == ' ' {
		l.discard()
	}

	return l.lexTagInlineContent
}

func isASCIILetter(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isNewLine(r rune) bool {
	return r == '\r' || r == '\n'
}
