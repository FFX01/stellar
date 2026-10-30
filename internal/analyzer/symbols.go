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
	Node     ast.Node
	Type     Type
	State    ResolveState
	IsPublic bool
}

func (sb *symbolBase) base() *symbolBase { return sb }

type Symbol interface {
	base() *symbolBase
}

// VarStmt, Parameter, bindings in for in/try else: store ForInExpr/TryExpr
type VarSymbol struct {
	symbolBase
	IsMut bool
}

// ast.FunctionStmt
type FuncSymbol struct{ symbolBase }

// TypeDeclStmt, TypeAliasStmt
type TypeSymbol struct{ symbolBase }

// ImportStmt
type ModuleSymbol struct{ symbolBase }
