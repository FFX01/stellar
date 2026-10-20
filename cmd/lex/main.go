package main

import "stellar/internal/lexer"
import "fmt"
import "os"
import "log"

func main() {
	if len(os.Args) < 2 {
		log.Fatal("No file provided")
	}

	filePath := os.Args[1]
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal("File does not exist")
	}

	l := lexer.NewLexer(string(content))
	fmt.Println("Now running lexer...")
	out := l.Run()
	var errors []lexer.Token
	for _, t := range out {
		if t.Type == lexer.ERR {
			errors = append(errors, t)
		}
		fmt.Println(t.String())
	}
	fmt.Println("")
	for _, e := range errors {
		fmt.Printf("ERROR: %s\n", e.String())
	}
}
