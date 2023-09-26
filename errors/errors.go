package errors

import "github.com/pipe01/poodle/internal/lexer"

type SituatedErr interface {
	Unwrap() error
	At() lexer.Location
}
