package analyzer

import (
	"log"
	"stellar/internal/ast"
	"stellar/internal/lexer"
)

type SymbolT int

const (
	VAR SymbolT = iota
	TYPE
	FUNC
)

type Symbol struct {
	Name  string
	Token lexer.Token
	Kind  SymbolT
}

type Env map[string]Symbol

type Scope struct {
	Parent   *Scope
	Children []*Scope
	Env      *Env
}

func Run(program ast.Program) {
	for _, stmt := range program.Statements {
		log.Print(stmt.String())
	}
}
