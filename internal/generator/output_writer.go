package generator

import (
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/pipe01/poodle/internal/lexer"
)

type outputWriter struct {
	w           io.Writer
	indentation int
}

func (w *outputWriter) Indent(delta int) {
	w.indentation += delta
}

func (w *outputWriter) writeIndentation() {
	fmt.Fprint(w.w, strings.Repeat("\t", w.indentation))
}

func (w *outputWriter) WriteFileHeader(pkg string) {
	fmt.Fprintf(w.w, `package %s

import (
	"bufio"
	"html"
)

`, pkg)
}

func (w *outputWriter) WriteFuncHeader(name string) {
	fmt.Fprintf(w.w, "func %s(w *bufio.Writer) {\n", name)

	w.Indent(1)
}

func (w *outputWriter) WriteFuncFooter() {
	w.Indent(-1)
	w.writeIndentation()

	fmt.Fprint(w.w, "}\n")
}

func (w *outputWriter) WriteLiteralUnescaped(str string) {
	w.writeIndentation()
	fmt.Fprintf(w.w, "w.WriteString(%q)\n", str)
}

func (w *outputWriter) WriteLiteralUnescapedf(format string, a ...any) {
	w.WriteLiteralUnescaped(fmt.Sprintf(format, a...))
}

func (w *outputWriter) WriteLiteralEscaped(str string) {
	w.WriteLiteralUnescaped(html.EscapeString(str))
}

func (w *outputWriter) WriteLiteralEscapedf(format string, a ...any) {
	w.WriteLiteralEscaped(fmt.Sprintf(format, a...))
}

func (w *outputWriter) WriteGoUnescaped(str string) {
	w.writeIndentation()
	fmt.Fprintf(w.w, "w.WriteString(fmt.Sprint(%s))\n", str)
}

func (w *outputWriter) WriteGoEscaped(str string) {
	w.writeIndentation()
	fmt.Fprintf(w.w, "w.WriteString(html.EscapeString(fmt.Sprint(%s)))\n", str)
}

func (w *outputWriter) WriteStatementStart(indent bool, keyword lexer.TokenType, arg string) {
	if indent {
		w.writeIndentation()
	}

	keywordName := ""
	switch keyword {
	case lexer.TokenStartIf:
		keywordName = "if"
	case lexer.TokenStartElse:
		keywordName = "else"
	case lexer.TokenStartFor:
		keywordName = "for"
	}

	if arg == "" {
		fmt.Fprintf(w.w, "%s {\n", keywordName)
	} else {
		fmt.Fprintf(w.w, "%s %s {\n", keywordName, arg)
	}

	w.Indent(1)
}

func (w *outputWriter) WriteStatementEnd(newLine bool) {
	w.Indent(-1)
	w.writeIndentation()

	if newLine {
		fmt.Fprint(w.w, "}\n")
	} else {
		fmt.Fprint(w.w, "} ")
	}
}
