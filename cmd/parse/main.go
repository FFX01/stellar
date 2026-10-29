package main

import (
	"fmt"
	"log"
	"os"
	"stellar/internal/ast"
	"stellar/internal/lexer"
	"stellar/internal/parser"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("No file provided")
	}

	filePath := os.Args[1]
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal("File does not exist")
	}

	lexer := lexer.NewLexer(string(content))
	tokens := lexer.Run()
	lexerErrors := lexer.Errors()
	if len(lexerErrors) > 0 {
		fmt.Println("--- Lexer Errors ---")
		for _, e := range lexerErrors {
			fmt.Println(e.Message)
		}
	}

	p := parser.NewParser(tokens)
	program := p.Parse()
	p.PrintErrors()
	ast.Dump(os.Stdout, program)
}
