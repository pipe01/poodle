package lexer

import (
	"fmt"
	"go/scanner"
	"go/token"
)

func (l *Lexer) setupGoScanner() (*scanner.Scanner, *token.File) {
	startByteIndex := l.byteIndex
	data := l.file[startByteIndex:]

	var scan scanner.Scanner

	fileSet := token.NewFileSet()
	f := fileSet.AddFile(l.filename, 1, len(data))

	scan.Init(f, data, func(pos token.Position, msg string) {
		col := pos.Column - 1
		if pos.Line == 1 {
			col += l.col
		}

		l.err = &LexerError{
			Inner: fmt.Errorf("scan Go code: %s", msg),
			Location: Location{
				File:   l.filename,
				Line:   l.line + pos.Line - 1,
				Column: col,
			},
		}
	}, 0)

	return &scan, f
}
