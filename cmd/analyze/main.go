package main

import "stellar/internal/analyzer"
import "stellar/internal/lexer"
import "stellar/internal/parser"
import "log"
import "os"

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
	a := analyzer.Run(program)
	a.PrintTree()
}
