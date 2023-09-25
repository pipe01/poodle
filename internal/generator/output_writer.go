package generator

import (
	"bufio"
	"fmt"
	"html"
	"io"
	"strings"
)

type outputWriter struct {
	w           io.Writer
	indentation int
}

func (w *outputWriter) indent(delta int) {
	w.indentation += delta
}

func (w *outputWriter) writeIndentation() {
	fmt.Fprint(w.w, strings.Repeat("\t", w.indentation))
}

func (w *outputWriter) WriteFileHeader(pkg string, imports []string) {
	fmt.Fprintf(w.w, `package %s

import (
`, pkg)

	for _, i := range imports {
		fmt.Fprintf(w.w, "	%s\n", i)
	}

	fmt.Fprint(w.w, ")\n\n")
}

func (w *outputWriter) WriteFuncHeader(name string, args []string) {
	fmt.Fprintf(w.w, "func %s(w *bufio.Writer", name)

	if len(args) > 0 {
		fmt.Fprintf(w.w, ", %s", strings.Join(args, ", "))
	}

	fmt.Fprint(w.w, ") {\n")

	w.indent(1)
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

func (w *outputWriter) WriteStatementStart(indent bool, keyword string, arg string) {
	if indent {
		w.writeIndentation()
	}

	if arg == "" {
		fmt.Fprintf(w.w, "%s {\n", keyword)
	} else {
		fmt.Fprintf(w.w, "%s %s {\n", keyword, arg)
	}

	w.indent(1)
}

func (w *outputWriter) WriteVariable(name string, value string) {
	w.writeIndentation()
	fmt.Fprintf(w.w, "%s := %s\n", name, value)

	w.writeIndentation()
	fmt.Fprintf(w.w, "_ = %s\n", name)
}

func (w *outputWriter) WriteBlockStart() {
	w.writeIndentation()
	fmt.Fprint(w.w, "{\n")
	w.indent(1)
}

func (w *outputWriter) WriteBlockEnd(newLine bool) {
	w.indent(-1)
	w.writeIndentation()

	if newLine {
		fmt.Fprint(w.w, "}\n")
	} else {
		fmt.Fprint(w.w, "} ")
	}
}

func (w *outputWriter) WriteGoBlock(contents string) {
	sc := bufio.NewScanner(strings.NewReader(contents))

	for sc.Scan() {
		w.writeIndentation()
		w.w.Write(sc.Bytes())
		w.w.Write([]byte{'\n'})
	}
}

func (w *outputWriter) WriteFuncVariableStart(name string, args string) {
	w.writeIndentation()
	fmt.Fprintf(w.w, "%s := func(%s) {\n", name, args)
	w.indent(1)
}
