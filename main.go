package main

import (
	"fmt"
	"log"

	"github.com/pipe01/poodle/internal/lexer"
)

func main() {
	l := lexer.New([]byte(`
div nice @my_var asd
	what
`), "myfile.poo")

	for {
		t, err := l.Next()
		if err != nil {
			log.Fatalf("failed to get token: %s", err)
		}

		fmt.Printf("%#v\n", *t)
	}
}
