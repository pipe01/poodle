package generator

import (
	"fmt"
	"html"
	"io"
	"strings"
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
	fmt.Fprintf(w.w, "w.WriteString(%s)\n", str)
}

func (w *outputWriter) WriteGoEscaped(str string) {
	w.writeIndentation()
	fmt.Fprintf(w.w, "w.WriteString(html.EscapeString(%s))\n", str)
}
