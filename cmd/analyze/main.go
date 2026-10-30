package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"stellar/internal/analyzer"
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
		for _, e := range lexerErrors {
			fmt.Println(e.Message)
		}
		log.Fatal("Stoping due to lexer errors.")
	}

	p := parser.NewParser(tokens)
	program := p.Parse()
	if p.HasErrors() {
		p.PrintErrors()
		log.Fatal("Parser errors, stopping...")
	}

	a := analyzer.Run(program)
	if i, crossed := a.Package.Lookup("int"); i != nil {
		fmt.Println("Found int")
		fmt.Println("crossed: ", crossed)
	}
	if i := a.Package.LookupLocal("int"); i != nil {
		fmt.Println("Found int in local. this is a problem")
	}
	file, err := os.Create("log.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	a.Dump(writer)
	writer.Flush()
}
