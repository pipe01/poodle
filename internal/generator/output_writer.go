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

	instrs []Instruction
}

func (w *outputWriter) add(i Instruction) {
	w.instrs = append(w.instrs, i)
}

func (w *outputWriter) indent(delta int) {
	w.indentation += delta
}

func (w *outputWriter) writeIndentation() {
	w.add(&InstructionIndentation{
		Depth: w.indentation,
	})
}

func (w *outputWriter) Close() error {
	var litBuf strings.Builder

	for i := 0; i < len(w.instrs); i++ {
		inst := w.instrs[i]

		switch inst := inst.(type) {
		case *InstructionLiteral:
			litBuf.WriteString(inst.String)
			continue

		case *InstructionIndentation:
			if i == len(w.instrs)-1 {
				break
			}

			_, nextIsLit := w.instrs[i+1].(*InstructionLiteral)
			if nextIsLit {
				continue
			}
		}

		if litBuf.Len() > 0 {
			writeLiteral(w.w, litBuf.String())
			litBuf.Reset()
		}

		inst.WriteTo(w.w)
	}

	return nil
}

func (w *outputWriter) WriteFileHeader(pkg string, imports []string) {
	w.add(&InstructionFileHeader{
		Package: pkg,
		Imports: imports,
	})
}

func (w *outputWriter) WriteFuncHeader(name string, args []string) {
	w.add(&InstructionWriteFuncHeader{
		Name: name,
		Args: args,
	})

	w.indent(1)
}

func (w *outputWriter) WriteLiteralUnescaped(str string) {
	w.writeIndentation()
	w.add(&InstructionLiteral{String: str})
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
	w.add(&InstructionGo{
		Value:      str,
		HTMLEscape: false,
	})
}

func (w *outputWriter) WriteGoEscaped(str string) {
	w.writeIndentation()
	w.add(&InstructionGo{
		Value:      str,
		HTMLEscape: true,
	})
}

func (w *outputWriter) WriteStatementStart(indent bool, keyword string, arg string) {
	if indent {
		w.writeIndentation()
	}

	w.add(&InstructionStatementStart{
		Keyword: keyword,
		Arg:     arg,
	})

	w.indent(1)
}

func (w *outputWriter) WriteVariable(name, typ, value string) {
	w.writeIndentation()
	w.add(&InstructionVariable{
		Name:  name,
		Type:  typ,
		Value: value,
	})
}

func (w *outputWriter) WriteBlockStart() {
	w.writeIndentation()
	w.add(&InstructionBlockStart{})
	w.indent(1)
}

func (w *outputWriter) WriteBlockEnd(newLine bool) {
	w.indent(-1)
	w.writeIndentation()

	w.add(&InstructionBlockEnd{Newline: newLine})
}

func (w *outputWriter) WriteGoBlock(contents string) {
	sc := bufio.NewScanner(strings.NewReader(contents))

	for sc.Scan() {
		w.writeIndentation()
		w.add(&InstructionGoLine{
			Content: sc.Bytes(),
		})
	}
}
