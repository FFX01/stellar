package analyzer

import "stellar/internal/ast"

func (a *Analyzer) collectTypeDeclStmt(stmt *ast.TypeDeclStmt) {
	sym := &TypeSymbol{symbolBase: symbolBase{
		Name:     stmt.Name,
		Node:     stmt,
		IsPublic: stmt.IsPublic,
		State:    Unresolved,
	}}
	switch stmt.Definition.(type) {
	case *ast.StructType:
		sym.Type = &StructType{Name: stmt.Name}
	case *ast.EnumType:
		sym.Type = &StructType{Name: stmt.Name}
	case *ast.InterfaceType:
		sym.Type = &StructType{Name: stmt.Name}
	}
	a.declare(sym)
}

func (a *Analyzer) collectTypeAliasStmt(stmt *ast.TypeAliasStmt) {
	sym := &TypeSymbol{symbolBase: symbolBase{
		Name:     stmt.Name,
		Node:     stmt,
		IsPublic: stmt.IsPublic,
		State:    Unresolved,
	}}
	a.declare(sym)
}

func (a *Analyzer) collectFunctionStmt(stmt *ast.FunctionStmt) {
	sym := &FuncSymbol{symbolBase: symbolBase{
		Name:     stmt.Signature.Name,
		Node:     stmt,
		IsPublic: stmt.IsPublic,
		State:    Unresolved,
	}}
	a.declare(sym)
}

func (a *Analyzer) collectVarStmt(stmt *ast.VarStmt) {
	sym := &VarSymbol{symbolBase: symbolBase{
		Name:     stmt.Name.Literal,
		Node:     stmt,
		IsPublic: stmt.IsPublic,
		State:    Unresolved,
	}}
	if stmt.IsMut {
		a.addError(stmt, "top-level `mut` is not allowed")
	} else {
		a.declare(sym)
	}
}

func (a *Analyzer) collectImportStmt(stmt *ast.ImportStmt) {
	sym := &ModuleSymbol{symbolBase: symbolBase{
		Name:  stmt.Path[len(stmt.Path)-1].Literal,
		Node:  stmt,
		State: Resolved,
	}}
	a.declare(sym)
}

func (a *Analyzer) collectImplDeclStmt(stmt *ast.ImplDeclStmt) {
	a.Impls = append(a.Impls, stmt)
}

func (a *Analyzer) collect(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		switch t := stmt.(type) {
		case *ast.TypeDeclStmt:
			a.collectTypeDeclStmt(t)
		case *ast.TypeAliasStmt:
			a.collectTypeAliasStmt(t)
		case *ast.FunctionStmt:
			a.collectFunctionStmt(t)
		case *ast.VarStmt:
			a.collectVarStmt(t)
		case *ast.ImportStmt:
			a.collectImportStmt(t)
		}
	}
}
