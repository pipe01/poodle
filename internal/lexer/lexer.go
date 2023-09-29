package lexer

import (
	"errors"
	"fmt"
	"go/token"
	"unicode"
	"unicode/utf8"
)

const (
	interpolationChar = '@'
	debugPrint        = false
)

type LexerError struct {
	Inner    error
	Location Location
}

func (e *LexerError) Unwrap() error {
	return e.Inner
}

func (e *LexerError) Error() string {
	return fmt.Sprintf("%s at %s", e.Inner, &e.Location)
}

func (e *LexerError) At() Location {
	return e.Location
}

type UnexpectedRuneError struct {
	Got      rune
	Expected string
}

func (e *UnexpectedRuneError) Error() string {
	return fmt.Sprintf("expected %s, found %q", e.Expected, e.Got)
}

var ErrMixedIndentation = errors.New("mixed spaces and tabs aren't allowed on the same line")

type stateFunc func() stateFunc

type state struct {
	str      []rune
	strStart Location

	byteIndex, runeIndex int
	line, col            int
	lastLineCol          int
	lastRuneSize         int
	depth                int
}

type Lexer struct {
	filename string
	file     []byte

	tokens chan Token

	spacesPerLevel int

	state
	stateStack []state

	err *LexerError
}

func New(file []byte, fileName string) *Lexer {
	tks := make(chan Token, 1)

	lexer := &Lexer{
		tokens:     tks,
		file:       file,
		filename:   fileName,
		stateStack: make([]state, 0, 10),
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
				File:   lexer.filename,
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

func (l *Lexer) Collect() ([]Token, error) {
	tks := []Token{}

	for t := range l.tokens {
		tks = append(tks, t)

		if t.Type == TokenEOF {
			break
		}
	}

	if l.err != nil {
		return nil, l.err
	}

	return tks, nil
}

func (l *Lexer) take() (r rune, eof bool) {
	if l.byteIndex >= len(l.file) {
		return 0, true
	}

	if l.file[l.byteIndex] == '\r' {
		l.byteIndex++
	}

	r, size := utf8.DecodeRune(l.file[l.byteIndex:])

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

func (l *Lexer) peek() (r rune, eof bool) {
	if l.byteIndex >= len(l.file) {
		return 0, true
	}

	idx := l.byteIndex
	if l.file[idx] == '\r' {
		idx++
	}

	r, _ = utf8.DecodeRune(l.file[idx:])
	return
}

func (l *Lexer) takeRune(exp rune) (taken bool) {
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

func (l *Lexer) takeMany(n int) (eof bool) {
	for i := 0; i < n; i++ {
		_, eof = l.take()
		if eof {
			return true
		}
	}

	return false
}

func (l *Lexer) takeUntilByteIndex(n int) (eof bool) {
	for l.byteIndex < n {
		_, eof = l.take()
		if eof {
			return true
		}
	}

	return false
}

func (l *Lexer) takeWhitespace() (took bool) {
	for {
		state := l.state

		r, eof := l.take()
		if eof {
			return false
		}

		if !isWhitespace(r) {
			l.state = state
			return took
		}

		took = true
	}
}

func (l *Lexer) takeUntilNewline() {
	for {
		state := l.state

		r, eof := l.take()
		if eof {
			return
		}

		if r == '\n' {
			l.state = state
			return
		}
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
		File:   l.filename,
		Line:   l.line,
		Column: l.col,
	}
	l.str = l.str[:0]
}

func (l *Lexer) isEmpty() bool {
	return len(l.str) == 0
}

func (l *Lexer) lexError(err error) stateFunc {
	l.err = &LexerError{
		Inner:    err,
		Location: l.strStart,
	}
	return nil
}

func (l *Lexer) lexUnexpected(got rune, expected string) stateFunc {
	return l.lexError(&UnexpectedRuneError{
		Got:      got,
		Expected: expected,
	})
}

func (l *Lexer) takeIndentation() (depth int) {
	spaces := 0

	for {
		state := l.state

		r, eof := l.take()
		if eof {
			return
		}

		switch r {
		case ' ':
			if depth != 0 {
				l.lexError(ErrMixedIndentation)
				return 0
			}

			spaces++

		case '\t':
			if spaces != 0 {
				l.lexError(ErrMixedIndentation)
				return 0
			}

			depth++

		case '\n':
			spaces = 0
			depth = 0

		default:
			if spaces > 0 {
				if l.spacesPerLevel == 0 {
					l.spacesPerLevel = spaces
				}

				depth = spaces / l.spacesPerLevel
			}

			l.state = state
			return
		}
	}
}

func (l *Lexer) takeIdentifier(expected string) (found bool) {
	for {
		r, eof := l.peek()
		if eof {
			return !l.isEmpty()
		}

		// This could be optimized, but it's more legible like this
		if l.isEmpty() {
			if !unicode.IsLetter(r) && r != '_' {
				l.lexUnexpected(r, expected)
				return false
			}
		} else {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return true
			}
		}

		l.take()
	}
}

func (l *Lexer) lexIndentation() stateFunc {
	l.depth = l.takeIndentation()
	l.discard()
	return l.lexLineStart
}

func (l *Lexer) lexNewLine() stateFunc {
	for {
		state := l.state

		r, eof := l.take()
		if eof {
			return nil
		}

		if r != '\n' {
			l.state = state
			l.emit(TokenNewLine)
			return l.lexIndentation
		}
	}
}

func (l *Lexer) lexForcedNewLine() stateFunc {
	foundSome := false

	for {
		state := l.state

		r, eof := l.take()
		if eof {
			return nil
		}

		if r != '\n' {
			if !foundSome {
				return l.lexUnexpected(r, "a new line")
			}

			l.state = state
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
	case interpolationChar: // Interpolation
		l.emit(TokenInterpolationStart)

		if r, eof := l.peek(); !eof && r == '\n' {
			l.take()
			l.discard()

			return l.lexInterpolationBlock(l.lexIndentation)
		}

		return l.lexInterpolationInline(l.lexForcedNewLine, true)

	case '.': // Shortcut div with class
		l.emit(TokenDot)
		return l.lexClassName

	case '#': // Shortcut div with ID
		l.emit(TokenHashtag)
		return l.lexID

	case '|': // Block text
		l.emit(TokenPipe)

		state := l.state

		r, eof := l.take()
		if eof {
			return nil
		}

		l.state = state

		switch {
		case isWhitespace(r):
			return l.lexWhitespacedInlineContent

		case r == '\n':
			return l.lexNewLine

		default:
			return l.lexTagInlineContent
		}

	case '/': // Comment
		return l.lexComment

	case '+': // Mixin call
		l.emit(TokenPlus)
		return l.lexMixinCall
	}

	for {
		state := l.state

		r, eof := l.take()
		if eof {
			return nil
		}

		if !(isASCIIDigit(r) || isASCIILetter(r) || r == '-' || r == '_') {
			l.state = state

			if l.isEmpty() {
				return l.lexUnexpected(r, "a tag name")
			}

			break
		}
	}

	tagName := string(l.str)
	if tagName == "include" {
		l.emit(TokenKeyword)

		l.takeWhitespace()
		l.discard()

		l.takeUntilNewline()
		l.emit(TokenImportPath)

		return l.lexNewLine
	}

	if l.depth == 0 {
		switch tagName {
		case "arg":
			l.emit(TokenKeyword)

			if !l.takeRune(' ') {
				return nil
			}
			l.discard()

			l.takeUntilNewline()
			l.emit(TokenTagInlineText)

			return l.lexNewLine

		case "import":
			l.emit(TokenKeyword)

			l.takeWhitespace()
			l.discard()

			l.takeUntilNewline()
			l.emit(TokenImportPath)

			return l.lexNewLine

		case "mixin":
			l.emit(TokenKeyword)

			l.takeWhitespace()
			l.discard()

			return l.lexMixinDef

		case "doctype":
			l.emit(TokenKeyword)

			l.takeWhitespace()
			l.discard()

			l.takeUntilNewline()
			l.emit(TokenTagInlineText)

			return l.lexNewLine
		}
	}

	l.emit(TokenIdentifier)
	return l.lexAfterTag
}

func (l *Lexer) lexComment() stateFunc {
	r, eof := l.take()
	if eof {
		return nil
	}

	if r != '/' {
		return l.lexUnexpected(r, "'/'")
	}

	if r, eof := l.peek(); !eof && r == '-' {
		l.take()
		l.emit(TokenCommentStart)
	} else {
		l.emit(TokenCommentStartBuffered)
	}

	for {
		state := l.state

		r, eof := l.take()
		if eof || r == '\n' {
			l.state = state
			l.emit(TokenCommentText)
			l.take()

			if eof {
				return nil
			}
			return l.lexIndentation
		}
	}
}

func (l *Lexer) lexAfterTag() stateFunc {
	state := l.state

	r, eof := l.take()
	if eof {
		return nil
	}

	switch r {
	case ' ':
		l.state = state
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
		if r == '\n' {
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
		state := l.state

		r, eof := l.take()
		if eof {
			return nil
		}

		if !isASCIILetter(r) && !isASCIIDigit(r) && r != '-' && r != '_' {
			l.state = state
			l.emit(TokenClassName)
			break
		}
	}

	return l.lexAfterTag
}

func (l *Lexer) lexID() stateFunc {
	for {
		state := l.state

		r, eof := l.take()
		if eof {
			return nil
		}

		if !isASCIILetter(r) && !isASCIIDigit(r) && r != '-' && r != '_' {
			l.state = state

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
		r, eof := l.peek()
		if eof {
			if !l.isEmpty() {
				l.emit(TokenTagInlineText)
			}
			return nil
		}

		switch {
		case r == interpolationChar:
			// Emit pending inline text, if any
			if !l.isEmpty() {
				l.emit(TokenTagInlineText)
			}

			// Take first interpolation char
			l.take()

			// Check if the next char if also the interpolation char
			if r, eof := l.peek(); !eof && r == interpolationChar {
				// If it is, we discard the first one, then take
				// the second one and continue the loop in order to include it
				// in the next inline text emit
				l.discard()
				l.take()
				continue
			}

			l.emit(TokenInterpolationStart)
			return l.lexInterpolationInline(l.lexTagInlineContent, false)

		case r == '\n':
			if !l.isEmpty() {
				l.emit(TokenTagInlineText)
			}
			return l.lexNewLine

		default:
			l.take()
		}
	}
}

func (l *Lexer) lexAttributeName() stateFunc {
	l.takeWhitespace()
	l.discard()

	for {
		state := l.state

		r, eof := l.take()
		if eof {
			return nil
		}

		if r == '\n' {
			l.takeWhitespace()
			l.discard()
			continue
		}

		if !unicode.IsLetter(r) {
			l.state = state

			if l.isEmpty() {
				return l.lexAfterAttributes
			}

			l.emit(TokenAttributeName)
			return l.lexAttributeEqual
		}
	}
}

func (l *Lexer) lexAttributeEqual() stateFunc {
	l.takeWhitespace()
	l.discard()

	if !l.takeRune('=') {
		return nil
	}

	l.emit(TokenEquals)
	return l.lexAttributeValue
}

func (l *Lexer) lexAttributeValue() stateFunc {
	state := l.state

	r, eof := l.take()
	if eof {
		return nil
	}
	if r != '"' {
		if isWhitespace(r) {
			return l.lexUnexpected(r, "an attribute value")
		}

		l.state = state
		return l.lexInterpolationInline(l.lexAttributeName, false)
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

func (l *Lexer) takeGoExpression(parseStmts bool, stopOn token.Token) {
	if r, eof := l.peek(); !eof && r == '!' {
		l.take()
		l.emit(TokenExclamationPoint)
	}

	startByteIndex := l.byteIndex
	scan, f := l.setupGoScanner()

	var parenCount int
	var endPos int
	startsIdent := false

loop:
	for {
		pos, tok, lit := scan.Scan()

		if parseStmts && pos == 1 {
			switch tok {
			// If the first token is "if", "else" or "for", emit the corresponding start token
			// and take the rest of the line as the expression after that statement
			case token.IF, token.ELSE, token.FOR:
				l.takeUntilByteIndex(startByteIndex + len(lit))
				l.emit(TokenKeyword)

				l.takeWhitespace()
				l.discard()

				l.takeUntilNewline()
				if tok == token.ELSE {
					l.discard()
				} else {
					l.emit(TokenGoExpr)
				}

				return
			}
		}

		switch tok {
		case token.ILLEGAL:
			if lit == string(interpolationChar) {
				l.err = nil
				break loop
			}

		case token.IDENT:
			if parenCount == 0 {
				if startsIdent {
					break loop
				}

				startsIdent = true
				endPos = int(pos) + len(lit)
			}

		case token.LPAREN:
			parenCount++
		case token.RPAREN:
			parenCount--

			if parenCount < 0 {
				endPos = int(pos)
				break loop
			}
			if parenCount == 0 {
				endPos = int(pos) + 1
				break loop
			}

		case stopOn:
			if parenCount == 0 {
				endPos = int(pos)
				break loop
			}

		case token.EOF:
			if parenCount != 0 {
				l.lexError(errors.New("unfinished Go expression"))
				return
			}
			break loop
		}
	}

	endIndex := int(endPos) - f.Base()

	for l.byteIndex < startByteIndex+endIndex {
		l.take()
	}
}

func (l *Lexer) lexInterpolationInline(returnTo stateFunc, parseStmts bool) stateFunc {
	return func() stateFunc {
		l.takeGoExpression(parseStmts, 0)
		l.emit(TokenGoExpr)

		return returnTo
	}
}

func (l *Lexer) lexInterpolationBlock(returnTo stateFunc) stateFunc {
	return func() stateFunc {
		startDepth := l.depth

		for {
			state := l.state
			depth := l.takeIndentation()

			if depth <= startDepth {
				l.state = state
				break
			}

			l.takeUntilNewline()
			l.take()
		}

		l.emit(TokenGoBlock)
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

func (l *Lexer) lexMixinDef() stateFunc {
	// Lex mixin name
	if !l.takeIdentifier("mixin name") {
		return nil
	}
	l.emit(TokenIdentifier)

	r, eof := l.take()
	if eof {
		return nil
	}
	if r == '\n' {
		l.emit(TokenNewLine)
		return l.lexIndentation
	}
	if r != '(' {
		return l.lexUnexpected(r, "newline or argument list")
	}
	l.emit(TokenParenOpen)

	// Lex mixin args
loop:
	for {
		l.takeWhitespace()
		l.discard()

		// Take argument name
		if !l.takeIdentifier("mixin argument name") {
			return nil
		}
		l.emit(TokenIdentifier)

		// Take at least one space
		if !l.takeRune(' ') {
			return nil
		}
		l.takeWhitespace()
		l.discard()

		// Take argument type
		for {
			r, eof := l.peek()
			if eof {
				return nil
			}

			if r == ',' || r == ')' {
				break
			}

			l.take()
		}
		l.emit(TokenIdentifier)

		// Skip whitespace
		l.takeWhitespace()
		l.discard()

		// Take comma or right parenthesis
		r, eof := l.take()
		if eof {
			return nil
		}

		switch r {
		case ')':
			l.emit(TokenParenClose)
			break loop
		case ',':
			l.emit(TokenComma)
		default:
			l.lexUnexpected(r, "comma or right parenthesis")
			return nil
		}
	}

	if !l.takeRune('\n') {
		return nil
	}

	return l.lexIndentation
}

func (l *Lexer) lexMixinCall() stateFunc {
	if !l.takeIdentifier("mixin name") {
		return nil
	}
	l.emit(TokenIdentifier)

	r, eof := l.take()
	if eof {
		return nil
	}
	if r == '\n' {
		l.emit(TokenNewLine)
		return l.lexIndentation
	}
	if r != '(' {
		return l.lexUnexpected(r, "newline or argument list")
	}
	l.emit(TokenParenOpen)

	l.takeWhitespace()
	l.discard()

loop:
	for {
		l.takeGoExpression(false, token.COMMA)

		if l.isEmpty() {
			break
		}

		l.emit(TokenGoExpr)

		r, eof := l.take()
		if eof {
			return nil
		}

		switch r {
		case ',':
			l.emit(TokenComma)

			l.takeWhitespace()
			l.discard()

		case ')':
			l.emit(TokenParenClose)
			break loop

		case '\n':
			return l.lexUnexpected(r, "an argument value")
		}
	}

	return l.lexForcedNewLine
}

func isASCIILetter(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t'
}
