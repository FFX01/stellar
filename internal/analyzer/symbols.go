package analyzer

import "stellar/internal/ast"

type ResolveState int

const (
	Unresolved ResolveState = iota
	Resolving
	Resolved
)

type symbolBase struct {
	Name     string
	Node     *ast.Node
	Type     Type
	State    ResolveState
	IsPublic bool
}

func (sb *symbolBase) base() *symbolBase { return sb }

type Symbol interface {
	base() *symbolBase
}

type VarSymbol struct {
	symbolBase
	IsMut bool
}

type FuncSymbol struct{ symbolBase }
type ObjectSymbol struct{ symbolBase }
type ModuleSymbol struct{ symbolBase }

type Scope struct {
	Parent   *Scope
	Children []*Scope
	Symbols  map[string]Symbol
}
