package main

import (
	"fmt"
	"log"

	"github.com/pipe01/poodle/internal/lexer"
)

func main() {
	l := lexer.New([]byte(`
.test(id="nice") hello
`), "myfile.poo")

	for {
		t, err := l.Next()
		if err != nil {
			log.Fatalf("failed to get token: %s", err)
		}

		fmt.Printf("% 20s %#v\n", t.Type.String(), *t)
	}
}
