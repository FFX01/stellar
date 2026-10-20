package main

import (
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

	p := parser.NewParser(tokens)
	program := p.Parse()
	p.PrintErrors()
	ast.Dump(os.Stdout, program)
}
