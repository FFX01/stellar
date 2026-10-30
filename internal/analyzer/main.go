package analyzer

import (
	"cmp"
	"fmt"
	"slices"
	"stellar/internal/ast"
	"stellar/internal/errors"
)

var (
	IntSym = &TypeSymbol{symbolBase{Name: "int",
		Type: IntT, State: Resolved}}
	StrSym = &TypeSymbol{symbolBase{Name: "str",
		Type: StrT, State: Resolved}}
	FloatSym = &TypeSymbol{symbolBase{Name: "float",
		Type: FloatT, State: Resolved}}
	BoolSym = &TypeSymbol{symbolBase{Name: "bool",
		Type: BoolT, State: Resolved}}
	VoidSym = &TypeSymbol{symbolBase{Name: "void",
		Type: VoidT, State: Resolved}}
	ErrorSym = &TypeSymbol{symbolBase{Name: "Error",
		Type: ErrorT, State: Resolved}}
	LenSym = &FuncSymbol{symbolBase{Name: "len",
		Type: LenT, State: Resolved}}
	ResultSym = &TypeSymbol{symbolBase{Name: "Result",
		State: Resolved}}
	MaybeSym = &TypeSymbol{symbolBase{Name: "Maybe",
		State: Resolved}}
)

type Analyzer struct {
	Universe, Package, CurrentScope *Scope
	Errors                          []errors.Error
	ExprTypes                       map[ast.Expression]Type
	Impls                           []*ast.ImplDeclStmt
	Bodies                          []*pendingBody
	inPass3                         bool
	// function state, saved and restored for every function body
	curFunc   *FunctionType
	loopDepth int
}

type pendingBody struct {
	funcType *FunctionType
	params   []*ast.Parameter
	body     ast.Expression
	scope    *Scope
	name     string
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

func (a *Analyzer) enterScope(kind ScopeKind, name string) *Scope {
	s := NewScope(a.CurrentScope, kind, name)
	a.CurrentScope = s
	return s
}

func (a *Analyzer) exitScope() {
	if a.CurrentScope.Parent != nil {
		a.CurrentScope = a.CurrentScope.Parent
	}
}

func (a *Analyzer) declare(sym Symbol) bool {
	if sym.base().Name == "-" {
		return true
	}
	if a.Universe.LookupLocal(sym.base().Name) != nil {
		a.addError(sym.base().Node,
			"`%s` is a builtin and can't be redeclared",
			sym.base().Name)
		return false
	}
	if existing := a.CurrentScope.Declare(sym); existing != nil {
		a.addError(sym.base().Node,
			"`%s` redeclared (first declared at %s)",
			existing.base().Name,
			existing.base().Node.Pos().String())
		return false
	}
	return true
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		ExprTypes: map[ast.Expression]Type{},
	}
}

func (a *Analyzer) PrintErrors() {
	for _, e := range a.Errors {
		fmt.Println(e.String())
	}
}

func Run(prog *ast.Program) *Analyzer {
	a := NewAnalyzer()
	a.Universe = newUniverse()
	a.Package = NewScope(a.Universe, PackageScope, "package")
	a.CurrentScope = a.Package
	a.collect(prog)
	a.resolve()
	a.checkBodies()

	// sort errors by line, col
	slices.SortFunc(a.Errors, func(a, b errors.Error) int {
		if n := cmp.Compare(a.Position.Line, b.Position.Line); n != 0 {
			return n
		}
		return cmp.Compare(a.Position.Column, b.Position.Column)
	})

	return a
}
