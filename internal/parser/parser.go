package parser

import "github.com/pipe01/poodle/internal/lexer"

type parser struct {
	instrs chan Instruction
	tokens <-chan lexer.Token
}

func Parse(tokens <-chan lexer.Token) {
	instrs := make(chan Instruction)

	p := parser{
		instrs: instrs,
		tokens: tokens,
	}
}

func (p *parser) parseDocument() {

}
