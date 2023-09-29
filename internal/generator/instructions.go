package generator

import (
	"fmt"
	"io"
	"strings"
)

type Instruction interface {
	WriteTo(w io.Writer)
}

type InstructionIndentation struct {
	Depth int
}

func (i *InstructionIndentation) WriteTo(w io.Writer) {
	fmt.Fprint(w, strings.Repeat("\t", i.Depth))
}

type InstructionFileHeader struct {
	Package string
	Imports []string
}

func (i *InstructionFileHeader) WriteTo(w io.Writer) {
	fmt.Fprintf(w, `package %s

import (
`, i.Package)

	for _, i := range i.Imports {
		fmt.Fprintf(w, "	%s\n", i)
	}

	fmt.Fprint(w, ")\n\n")
}

type InstructionWriteFuncHeader struct {
	Name string
	Args []string
}

func (i *InstructionWriteFuncHeader) WriteTo(w io.Writer) {
	fmt.Fprintf(w, "func %s(w *bufio.Writer", i.Name)

	if len(i.Args) > 0 {
		fmt.Fprintf(w, ", %s", strings.Join(i.Args, ", "))
	}

	fmt.Fprint(w, ") {\n")
}

type InstructionLiteral struct {
	String string
}

func (i *InstructionLiteral) WriteTo(w io.Writer) {
	writeLiteral(w, i.String)
}

func writeLiteral(w io.Writer, str string) {
	fmt.Fprintf(w, "w.WriteString(%q)\n", str)
}

type InstructionGo struct {
	Value      string
	HTMLEscape bool
}

func (i *InstructionGo) WriteTo(w io.Writer) {
	if i.HTMLEscape {
		fmt.Fprintf(w, "w.WriteString(html.EscapeString(fmt.Sprint(%s)))\n", i.Value)
	} else {
		fmt.Fprintf(w, "w.WriteString(fmt.Sprint(%s))\n", i.Value)
	}
}

type InstructionStatementStart struct {
	Keyword string
	Arg     string
}

func (i *InstructionStatementStart) WriteTo(w io.Writer) {
	if i.Arg == "" {
		fmt.Fprintf(w, "%s {\n", i.Keyword)
	} else {
		fmt.Fprintf(w, "%s %s {\n", i.Keyword, i.Arg)
	}
}

type InstructionVariable struct {
	Name, Type, Value string
}

func (i *InstructionVariable) WriteTo(w io.Writer) {
	fmt.Fprintf(w, "var %s %s = %s; _ = %s\n", i.Name, i.Type, i.Value, i.Name)
}

type InstructionBlockStart struct {
}

func (i *InstructionBlockStart) WriteTo(w io.Writer) {
	fmt.Fprint(w, "{\n")
}

type InstructionBlockEnd struct {
	Newline bool
}

func (i *InstructionBlockEnd) WriteTo(w io.Writer) {
	if i.Newline {
		fmt.Fprint(w, "}\n")
	} else {
		fmt.Fprint(w, "} ")
	}
}

type InstructionGoLine struct {
	Content []byte
}

func (i *InstructionGoLine) WriteTo(w io.Writer) {
	w.Write(i.Content)
	w.Write([]byte{'\n'})
}
