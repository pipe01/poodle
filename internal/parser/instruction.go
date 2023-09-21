package parser

import "github.com/pipe01/poodle/internal/lexer"

type pos lexer.Location

func (p pos) Position() lexer.Location {
	return lexer.Location(p)
}

type Instruction interface {
	Position() lexer.Location
}

type InstructionWriteRaw struct {
	pos
	Text string
}
