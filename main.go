package main

import (
	"fmt"
	"log"

	"github.com/pipe01/poodle/lexer"
)

func main() {
	l := lexer.New([]byte(`hello.class-hello.my-class#asd(id="hello") Nice
	what.nice Cock
`), "myfile.poo")

	for {
		t, err := l.Next()
		if err != nil {
			log.Fatalf("failed to get token: %s", err)
		}

		fmt.Printf("%#v\n", *t)
	}
}
