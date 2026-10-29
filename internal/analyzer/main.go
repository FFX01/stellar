package analyzer

import (
	"fmt"
	"stellar/internal/ast"
	"stellar/internal/errors"
	"strings"
)

func (a *Analyzer) typeOf(expr ast.Expression) Type {
	switch n := expr.(type) {
	case *ast.IntegerLiteralExpr:
		return &BuiltInType{Id: "int"}
	case *ast.FloatLiteralExpr:
		return &BuiltInType{Id: "float"}
	case *ast.StringLiteralExpr:
		return &BuiltInType{Id: "str"}
	case *ast.BoolLiteralExpr:
		return &BuiltInType{Id: "bool"}
	case *ast.IdentifierExpr:
		if resolved := a.CurrentScope.lookupSymbol(n.Value); resolved != nil {
			return resolved.Type()
		} else {
			a.addError(n, "Undefined: %s", n.Value)
			return Invalid
		}
	case *ast.NegativeExpr:
		return a.typeOf(n.Value)
	case *ast.NotExpr:
		return a.typeOf(n.Value)
	case *ast.ResultExpr:
		return a.typeOfResultExpr(n)
	case *ast.InfixExpr:
		t1 := a.typeOf(n.Left)
		t2 := a.typeOf(n.Right)
		if !typesMatch(t1, t2) {
			a.addError(n, "Type mismatch: %s != %s", t1.Name(), t2.Name())
			return Invalid
		}
		return t1
	}
	return Invalid
}

func (a *Analyzer) typeOfResultExpr(node *ast.ResultExpr) Type {
	t := &ResultType{}
	if node.IsError {
		t.Kind = RTK_Err
	} else {
		t.Kind = RTK_Ok
	}
	t.ValueType = a.typeOf(node.Value)
	return t
}

func (a *Analyzer) visitResultType(node *ast.ResultType) Type {
	t := &ResultType{Kind: RTK_None}
	if node.Value != nil {
		t.ValueType = a.visitType(node.Value)
	}
	return t
}

func (a *Analyzer) visitType(node ast.Type) Type {
	switch t := node.(type) {
	case *ast.NamedType:
		bit, ok := builtInTypeTable[t.Name]
		if ok {
			return bit
		}
		// this is where we lookup custom types
		if typ := a.CurrentScope.lookupType(t.Name); typ != nil {
			return typ
		}
	case *ast.ResultType:
		return a.visitResultType(t)
	}
	return nil
}

func (a *Analyzer) visitTypeDeclStmt(node *ast.TypeDeclStmt) {
	switch nt := node.Definition.(type) {
	case *ast.EnumType:
		t := &EnumType{Id: node.Name}
		for _, m := range nt.Members {
			t.Members = append(t.Members, m.Name)
		}
		a.addType(t)
	}
}

func (a *Analyzer) visitVarStmt(node *ast.VarStmt) {
	s := &VarSymbol{Id: node.Name.Literal}
	declaredType := a.visitType(node.Type)
	inferredType := a.typeOf(node.Value)
	if inferredType == nil {
		a.addError(
			node,
			"Type could not be inferred for: %s",
			node.Value.TokenLiteral(),
		)
		return
	}
	if declaredType != nil {
		if !typesMatch(declaredType, inferredType) {
			a.addError(node, "Type %s != %s", declaredType.String(), inferredType.String())
		}
		s.ValueType = declaredType
	} else {
		s.ValueType = inferredType
	}
	a.addSymbol(s)
}

func (s *Scope) lookupSymbol(name string) Symbol {
	if sym := s.Symbols.get(name); sym != nil {
		return sym
	}
	if s.Parent != nil {
		return s.Parent.lookupSymbol(name)
	}
	return nil
}

func (s *Scope) lookupType(name string) Type {
	if typ := s.Types.get(name); typ != nil {
		return typ
	}
	if s.Parent != nil {
		return s.Parent.lookupType(name)
	}
	return Invalid
}

func NewScope(p *Scope) *Scope {
	s := &Scope{
		Parent:  p,
		Symbols: newSymbolTable(),
		Types:   newTypeTable(),
	}
	if p != nil {
		p.Children = append(p.Children, s)
	}
	return s
}

type Analyzer struct {
	CurrentScope *Scope
	Errors       []errors.Error
}

func (a *Analyzer) addError(n ast.Node, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	msg = fmt.Sprintf(
		"`%s`: %s",
		n.TokenLiteral(),
		msg,
	)
	a.Errors = append(a.Errors, errors.Error{Position: n.Pos(), Message: msg})
}

func (a *Analyzer) EnterScope() {
	a.CurrentScope = NewScope(a.CurrentScope)
}

func (a *Analyzer) ExitScope() {
	a.CurrentScope = a.CurrentScope.Parent
}

func (a *Analyzer) addSymbol(s Symbol) {
	a.CurrentScope.Symbols.insert(s)
}

func (a *Analyzer) addType(t Type) {
	a.CurrentScope.Types.insert(t)
}

func (a *Analyzer) analyze(node ast.Node) {
	switch nt := node.(type) {
	case *ast.VarStmt:
		a.visitVarStmt(nt)
	}
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		CurrentScope: NewScope(nil),
	}
}

func Run(p *ast.Program) *Analyzer {
	analyzer := NewAnalyzer()
	for _, stmt := range p.Statements {
		analyzer.analyze(stmt)
	}
	return analyzer
}

func (a *Analyzer) PrintErrors() {
	for _, e := range a.Errors {
		fmt.Println(e.String())
	}
}

// PrintTree prints the current scope and all its descendants recursively.
// Call this on your root/universe scope to print the entire program's layout.
func (s *Scope) printTree(indent string, isLast bool) {
	// 1. Format the visual tree lines (ASCII Art style)
	marker := "├── "
	if isLast {
		marker = "└── "
	}

	fmt.Print(indent)
	fmt.Print(marker)

	// 2. Print the current scope header and its local symbols
	symbolList := []string{}
	for name, sym := range s.Symbols.value {
		symbolList = append(symbolList, fmt.Sprintf(
			"%s: %s (%s)",
			name,
			sym.Name(),
			sym.Type().Name(),
		))
	}

	if len(symbolList) == 0 {
		fmt.Println("[Block Scope] (empty)")
	} else {
		fmt.Printf("[Block Scope] { %s }\n", strings.Join(symbolList, ", "))
	}

	// 3. Prepare indentation for the children
	childIndent := indent
	if isLast {
		childIndent += "    "
	} else {
		childIndent += "│   "
	}

	// 4. Recurse down into child blocks
	for i, child := range s.Children {
		isChildLast := i == len(s.Children)-1
		child.printTree(childIndent, isChildLast)
	}
}

func (a *Analyzer) PrintTree() {
	topScope := a.CurrentScope
	isSolo := true
	for topScope.Parent != nil {
		isSolo = false
		topScope = topScope.Parent
	}
	topScope.printTree("", isSolo)
	fmt.Println("-- Errors: --")
	a.PrintErrors()
}
