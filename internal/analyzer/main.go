package analyzer

// Todos:
// - make struct fields a map for O(1) lookups

import (
	"fmt"
	"slices"
	"stellar/internal/ast"
	"strings"
)

var UNRESOLVED string = "<unresolved>"

type Type interface {
	typ()
	Name() string
}

type PrimitiveType struct {
	Id string
}

func (pt *PrimitiveType) typ()         {}
func (pt *PrimitiveType) Name() string { return pt.Id }

var (
	IntT     = &PrimitiveType{Id: "int"}
	FloatT   = &PrimitiveType{Id: "float"}
	StrT     = &PrimitiveType{Id: "str"}
	BoolT    = &PrimitiveType{Id: "bool"}
	InvalidT = &PrimitiveType{Id: "invalid"}
	VoidT    = &PrimitiveType{Id: "void"}
	SelfT    = &PrimitiveType{Id: "self"}

	// Temporary for imports
	AnyT = &PrimitiveType{Id: "any"}
)

type Param struct {
	Node *ast.Parameter
	Id   string
	Type Type
}

type FunctionType struct {
	Node       *ast.FunctionSignature
	Receiver   *Param
	Params     []Param
	ReturnType Type
}

func (ft *FunctionType) typ() {}
func (ft *FunctionType) Name() string {
	return fmt.Sprintf("fn() -> %s",
		ft.ReturnType.Name())
}

type InterfaceType struct {
	Node    *ast.InterfaceType
	Id      string
	Methods map[string]*FunctionType
}

func (it *InterfaceType) typ()         {}
func (it *InterfaceType) Name() string { return it.Id }
func (it *InterfaceType) GetMethod(id string) *FunctionType {
	if m, ok := it.Methods[id]; ok {
		return m
	}
	return nil
}

type Field struct {
	Node     *ast.Field
	Id       string
	Type     Type
	IsPublic bool
}

type EnumMember struct {
	Node   *ast.EnumMember
	Id     string
	Fields map[string]Field
}

type EnumType struct {
	Node    *ast.EnumType
	Id      string
	Members []EnumMember
	Methods map[string]*FunctionSymbol
}

func (et *EnumType) typ()         {}
func (et *EnumType) Name() string { return et.Id }
func (et *EnumType) GetMember(id string) *EnumMember {
	for idx, m := range et.Members {
		if m.Id == id {
			return &et.Members[idx]
		}
	}
	return nil
}
func (et *EnumType) GetMethod(id string) *FunctionSymbol {
	if m, ok := et.Methods[id]; ok {
		return m
	}
	return nil
}

type SliceType struct {
	Node        *ast.SliceType
	ElementType Type
}

func (st *SliceType) typ() {}
func (st *SliceType) Name() string {
	return fmt.Sprintf("[]%s", st.ElementType.Name())
}

type StructType struct {
	Node    *ast.StructType
	Id      string
	Fields  map[string]Field
	Methods map[string]*FunctionSymbol
}

func (st *StructType) typ()         {}
func (st *StructType) Name() string { return st.Id }
func (st *StructType) FieldNames() []string {
	var out []string
	for k, _ := range st.Fields {
		out = append(out, k)
	}
	return out
}
func (st *StructType) GetMethod(id string) *FunctionSymbol {
	if m, ok := st.Methods[id]; ok {
		return m
	}
	return nil
}
func (st *StructType) GetField(id string) *Field {
	if f, ok := st.Fields[id]; ok {
		return &f
	}
	return nil
}

type ResolutionState int

const (
	Unresolved ResolutionState = iota
	Resolving
	Resolved
)

type OperandMode int

const (
	Invalid OperandMode = iota
	TypeExpr
	Variable
	Value
	Module
	VariantConstructor
)

type Operand struct {
	Mode   OperandMode
	Type   Type
	Symbol Symbol
	Member *EnumMember
}

func invalidOperand(sym Symbol) Operand {
	return Operand{Mode: Invalid, Type: InvalidT, Symbol: sym}
}

type Symbol interface {
	sym()
	Name() string
	State() ResolutionState
	GetType() Type
}

type TypeSymbol struct {
	Node  *ast.TypeDeclStmt
	Id    string
	Type  Type
	state ResolutionState
}

func (ts *TypeSymbol) sym()                   {}
func (ts *TypeSymbol) Name() string           { return ts.Id }
func (ts *TypeSymbol) State() ResolutionState { return ts.state }
func (ts *TypeSymbol) GetType() Type          { return ts.Type }

var (
	IntS   = &TypeSymbol{Id: "int", Type: IntT, state: Resolved}
	FloatS = &TypeSymbol{Id: "float", Type: FloatT, state: Resolved}
	StrS   = &TypeSymbol{Id: "str", Type: StrT, state: Resolved}
	BoolS  = &TypeSymbol{Id: "bool", Type: BoolT, state: Resolved}
)

type ImportSymbol struct {
	Node  *ast.ImportStmt
	Id    string
	state ResolutionState
}

func (is *ImportSymbol) sym()                   {}
func (is *ImportSymbol) Name() string           { return is.Id }
func (is *ImportSymbol) State() ResolutionState { return is.state }
func (is *ImportSymbol) GetType() Type          { return AnyT }

type FunctionSymbol struct {
	Node     *ast.FunctionStmt
	Id       string
	Type     Type
	IsPublic bool
	state    ResolutionState
}

func (fs *FunctionSymbol) sym()                   {}
func (fs *FunctionSymbol) Name() string           { return fs.Id }
func (fs *FunctionSymbol) State() ResolutionState { return fs.state }
func (fs *FunctionSymbol) GetType() Type          { return fs.Type }

type VarSymbol struct {
	Node     *ast.VarStmt
	Id       string
	Type     Type
	IsPublic bool
	IsMut    bool
	state    ResolutionState
}

func (vs *VarSymbol) sym()                   {}
func (vs *VarSymbol) Name() string           { return vs.Id }
func (vs *VarSymbol) State() ResolutionState { return vs.state }
func (vs *VarSymbol) GetType() Type          { return vs.Type }

type Scope struct {
	Id          string
	Parent      *Scope
	Children    []*Scope
	symbolOrder []string
	symbols     map[string]Symbol
}

func NewScope(parent *Scope, id string) *Scope {
	s := &Scope{Id: id, Parent: parent}
	s.symbols = map[string]Symbol{}
	if parent != nil {
		parent.Children = append(parent.Children, s)
	}
	return s
}
func (s *Scope) lookup(name string) (Symbol, bool) {
	if s, ok := s.symbols[name]; ok {
		return s, false
	}
	if s.Parent != nil {
		sym, crossed := s.Parent.lookup(name)
		if sym != nil {
			return sym, true || crossed
		}
	}
	return nil, false
}
func (s *Scope) lookupLocal(name string) Symbol {
	if symbol, ok := s.symbols[name]; ok {
		return symbol
	}
	return nil
}
func (s *Scope) declare(sym Symbol) error {
	if _, ok := s.symbols[sym.Name()]; ok {
		return fmt.Errorf("redeclaration: %s", sym.Name())
	}
	s.symbolOrder = append(s.symbolOrder, sym.Name())
	s.symbols[sym.Name()] = sym
	return nil
}
func (s *Scope) Symbols() []Symbol {
	var out []Symbol
	for _, name := range s.symbolOrder {
		out = append(out, s.symbols[name])
	}
	return out
}

type AnalyzerError struct {
	Node ast.Node
	Msg  string
}

func (ae *AnalyzerError) String() string {
	return fmt.Sprintf(
		"%s Error: %s",
		ae.Node.Pos().String(),
		ae.Msg,
	)
}

type Analyzer struct {
	Universe     *Scope
	Package      *Scope
	CurrentScope *Scope
	Errors       []AnalyzerError
}

func (a *Analyzer) addErrorF(node ast.Node, msg string, args ...any) {
	fmtMsg := fmt.Sprintf(msg, args...)
	e := AnalyzerError{
		Node: node,
		Msg:  fmtMsg,
	}
	a.Errors = append(a.Errors, e)
}

func (a *Analyzer) addError(node ast.Node, err error) {
	e := AnalyzerError{
		Msg:  err.Error(),
		Node: node,
	}
	a.Errors = append(a.Errors, e)
}

func (a *Analyzer) declare(sym Symbol) error {
	return a.CurrentScope.declare(sym)
}

func NewAnalyzer() *Analyzer {
	// setup universe and package scopes
	a := &Analyzer{
		Universe: newUniverse(),
	}
	a.Package = NewScope(a.Universe, "package")
	a.CurrentScope = a.Package

	return a
}

func newUniverse() *Scope {
	scope := NewScope(nil, "universe")

	// decalre builtins
	scope.declare(IntS)
	scope.declare(FloatS)
	scope.declare(StrS)
	scope.declare(BoolS)

	return scope
}

func (a *Analyzer) collectVarStmt(stmt *ast.VarStmt) {
	s := &VarSymbol{
		Node: stmt, Id: stmt.Name.Literal,
		IsPublic: stmt.IsPublic, IsMut: stmt.IsMut,
		state: Unresolved,
	}
	if err := a.declare(s); err != nil {
		a.addError(stmt, err)
	}
}

func (a *Analyzer) collectTypeDeclStmt(stmt *ast.TypeDeclStmt) {
	s := &TypeSymbol{Node: stmt, Id: stmt.Name, state: Unresolved}
	switch typ := stmt.Definition.(type) {
	case *ast.StructType:
		s.Type = &StructType{Node: typ, Id: stmt.Name}
	case *ast.EnumType:
		s.Type = &EnumType{Node: typ, Id: stmt.Name}
	case *ast.InterfaceType:
		s.Type = &InterfaceType{Node: typ, Id: stmt.Name}
	}
	if err := a.declare(s); err != nil {
		a.addError(stmt, err)
	}
	s.state = Resolved
}

func (a *Analyzer) collectFunctionStmt(stmt *ast.FunctionStmt) {
	s := &FunctionSymbol{Node: stmt,
		Id: stmt.Signature.Name, IsPublic: stmt.IsPublic,
		state: Unresolved}
	if err := a.declare(s); err != nil {
		a.addError(stmt, err)
	}
}

func (a *Analyzer) collectImportStmt(stmt *ast.ImportStmt) {
	s := &ImportSymbol{Node: stmt, state: Resolved}
	s.Id = stmt.Path[len(stmt.Path)-1].Literal
	if err := a.declare(s); err != nil {
		a.addError(stmt, err)
	}
}

func (a *Analyzer) collectStmt(stmt ast.Statement) {
	switch ty := stmt.(type) {
	case *ast.VarStmt:
		a.collectVarStmt(ty)
	case *ast.TypeDeclStmt:
		a.collectTypeDeclStmt(ty)
	case *ast.FunctionStmt:
		a.collectFunctionStmt(ty)
	case *ast.ImportStmt:
		a.collectImportStmt(ty)
	}
}

func (a *Analyzer) collectPackage(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		a.collectStmt(stmt)
	}
}

func (a *Analyzer) resolveStructSymbol(symT *StructType) {
	symT.Fields = map[string]Field{}
	symT.Methods = map[string]*FunctionSymbol{}
	for _, f := range symT.Node.Fields {
		sf := Field{Id: f.Name, IsPublic: f.IsPublic,
			Node: f, Type: a.resolveType(f.Type)}
		symT.Fields[f.Name] = sf
	}
}

func (a *Analyzer) resolveEnumMember(enum *EnumType, member *ast.EnumMember) {
	em := EnumMember{Node: member, Id: member.Name}
	em.Fields = map[string]Field{}
	for _, f := range member.Fields {
		mf := Field{Node: f, Id: f.Name, Type: a.resolveType(f.Type)}
		em.Fields[f.Name] = mf
	}
	enum.Members = append(enum.Members, em)
}

func (a *Analyzer) resolveEnumSymbol(st *EnumType) {
	st.Methods = map[string]*FunctionSymbol{}
	for _, m := range st.Node.Members {
		a.resolveEnumMember(st, m)
	}
}

func (a *Analyzer) resolveInterfaceSymbol(st *InterfaceType) {
	st.Methods = map[string]*FunctionType{}
	for _, method := range st.Node.Methods {
		ft := &FunctionType{Node: method,
			ReturnType: a.resolveType(method.ReturnType)}
		for _, p := range method.Parameters {
			a.resolveParam(p, ft, SelfT)
		}
		st.Methods[method.Name] = ft
	}
}

func (a *Analyzer) resolveTypDeclSymbol(sym *TypeSymbol) {
	switch t := sym.Type.(type) {
	case *StructType:
		a.resolveStructSymbol(t)
	case *EnumType:
		a.resolveEnumSymbol(t)
	case *InterfaceType:
		a.resolveInterfaceSymbol(t)
	}
}

func (a *Analyzer) resolveTypeDeclSymbols(s *Scope) {
	for _, tds := range s.Symbols() {
		if t, ok := tds.(*TypeSymbol); ok {
			a.resolveTypDeclSymbol(t)
		}
	}
}

func (a *Analyzer) resolveImpl(node *ast.ImplDeclStmt) {
	namedType, ok := node.Target.(*ast.NamedType)
	if !ok {
		a.addErrorF(node, "cannot implement %T", node.Target)
		return
	}
	sym := a.CurrentScope.lookupLocal(namedType.Name)
	if sym == nil {
		a.addErrorF(node, "type %s not found in this scope", namedType.Name)
		return
	}
	typeSymbol, ok := sym.(*TypeSymbol)
	if !ok {
		a.addErrorF(node, "%s is not a type", sym.Name())
		return
	}
	for _, method := range node.Methods {
		fSym := &FunctionSymbol{Node: method,
			Id: method.Signature.Name, IsPublic: method.IsPublic,
			state: Unresolved}
		a.resolveFunctionSymbol(fSym, typeSymbol)
		switch symT := typeSymbol.Type.(type) {
		case *StructType:
			symT.Methods[method.Signature.Name] = fSym
		case *EnumType:
			symT.Methods[method.Signature.Name] = fSym
		default:
			a.addErrorF(method, "cannot implement method on type %T", typeSymbol.Type)
		}
	}
	typeSymbol.state = Resolved
}

func (a *Analyzer) resolvePackageImpls(prog *ast.Program) {
	for _, node := range prog.Statements {
		if stmt, ok := node.(*ast.ImplDeclStmt); ok {
			a.resolveImpl(stmt)
		}
	}
}

func (a *Analyzer) resolveNamedType(typ *ast.NamedType) Type {
	sym, _ := a.CurrentScope.lookup(typ.Name)
	if sym == nil {
		a.addErrorF(typ, "type `%s` not found", typ.Name)
		return InvalidT
	}
	if sym.State() != Resolved {
		a.addErrorF(typ, "type `%s` not resolved", typ.Name)
		return InvalidT
	}
	return sym.GetType()
}

func (a *Analyzer) resolveSliceType(typ *ast.SliceType) Type {
	st := &SliceType{Node: typ}
	st.ElementType = a.resolveType(typ.ElementType)
	return st
}

func (a *Analyzer) resolveType(typ ast.Type) Type {
	switch t := typ.(type) {
	case nil:
		return VoidT
	case *ast.NamedType:
		return a.resolveNamedType(t)
	case *ast.SliceType:
		return a.resolveSliceType(t)
	}
	return InvalidT
}

func (a *Analyzer) inferInfixExprOp(expr *ast.InfixExpr) Operand {
	leftOp := a.inferExprOp(expr.Left)
	rightOp := a.inferExprOp(expr.Right)
	if !identical(leftOp.Type, rightOp.Type) {
		a.addErrorF(expr, "type `%s` != `%s`",
			leftOp.Type.Name(), rightOp.Type.Name())
		return invalidOperand(nil)
	}
	return leftOp
}

func findParamById(params []Param, id string) *Param {
	for _, p := range params {
		if p.Id == id {
			return &p
		}
	}
	return nil
}

func (a *Analyzer) checkFunctionArgTypes(
	ft *FunctionType, params []Param, args []*ast.Argument,
	callExpr *ast.CallExpr,
) bool {
	var requiredNames []string
	for _, p := range params {
		requiredNames = append(requiredNames, p.Id)
	}

	var filledNames []string
	for idx, arg := range args {
		if idx > len(params)-1 {
			a.addErrorF(arg, "too many arguments to function `%s`", ft.Node.Name)
			return false
		}
		if idx > 0 && arg.Name == "" {
			if args[idx-1].Name != "" {
				a.addErrorF(args[idx-1],
					"named arguments cannot come before positional arguments")
				return false
			}
		}
		var param *Param
		if arg.Name != "" {
			param = findParamById(params, arg.Name)
			if param == nil {
				a.addErrorF(arg, "function `%s` has no param named `%s`",
					ft.Node.Name, arg.Name)
				return false
			}
		} else {
			param = &params[idx]
		}

		if slices.Contains(filledNames, param.Id) {
			a.addErrorF(arg, "duplicate argument for paramter `%s`", param.Id)
			return false
		}

		argType := a.inferExprOp(arg.Value).Type
		if !identical(param.Type, argType) {
			a.addErrorF(arg, "type `%s` != `%s`",
				argType.Name(), param.Type.Name())
			return false
		}
		filledNames = append(filledNames, param.Id)
		requiredNames = slices.DeleteFunc(requiredNames, func(s string) bool {
			return s == param.Id
		})
	}
	if len(requiredNames) > 0 {
		a.addErrorF(callExpr, "missing value(s) for argument(s) `%s`",
			strings.Join(requiredNames, ", "))
		return false
	}
	return true
}

func (a *Analyzer) inferStructCallExprOp(st *StructType, expr *ast.CallExpr) Operand {
	ok := a.inferConstructorArgsExprOp(st.Fields, expr)
	if ok {
		return Operand{Mode: Value, Type: st}
	}
	return invalidOperand(nil)
}

func (a *Analyzer) inferEnumVariantCallExprOp(
	et *EnumType, em *EnumMember, expr *ast.CallExpr,
) Operand {
	ok := a.inferConstructorArgsExprOp(em.Fields, expr)
	if ok {
		return Operand{Mode: Value, Type: et, Member: em}
	}
	return invalidOperand(nil)
}

func (a *Analyzer) inferConstructorArgsExprOp(
	fields map[string]Field, expr *ast.CallExpr,
) bool {
	var filledNames []string
	for _, arg := range expr.Args {
		if arg.Name == "" {
			a.addErrorF(arg, "constructors only accept named arguments")
			return false
		}
		field, ok := fields[arg.Name]
		if !ok {
			a.addErrorF(arg, "`%s` has no field `%s`",
				exprPath(expr.Left), arg.Name)
			return false
		}
		if slices.Contains(filledNames, arg.Name) {
			a.addErrorF(arg, "duplicate field name `%s` in constructor call",
				arg.Name)
			return false
		}
		filledNames = append(filledNames, arg.Name)

		argOp := a.inferExprOp(arg.Value)

		if !identical(field.Type, argOp.Type) {
			a.addErrorF(arg, "type `%s` != `%s`",
				argOp.Type.Name(), field.Type.Name())
			return false
		}
	}

	var missingFields []string
	for k, v := range fields {
		if !slices.Contains(filledNames, k) && !hasZeroValue(v.Type) {
			missingFields = append(missingFields, k)
		}
	}
	if len(missingFields) > 0 {
		a.addErrorF(expr, "missing value(s) for field(s): %s",
			strings.Join(missingFields, ", "))
		return false
	}
	return true
}

func (a *Analyzer) inferFunctionCallExprOp(
	ft *FunctionType, expr *ast.CallExpr,
) Operand {
	if !a.checkFunctionArgTypes(ft, ft.Params, expr.Args, expr) {
		return invalidOperand(nil)
	}
	return Operand{Mode: Value, Type: ft.ReturnType}
}

func getFieldById(fields []Field, id string) *Field {
	for _, f := range fields {
		if f.Id == id {
			return &f
		}
	}
	return nil
}

func hasZeroValue(t Type) bool {
	switch t.(type) {
	case *PrimitiveType, *SliceType:
		return true
	case *StructType, *EnumType, *InterfaceType, *FunctionType:
		return false
	}
	return false
}

func exprPath(expr ast.Expression) string {
	switch t := expr.(type) {
	case *ast.IdentifierExpr:
		return t.Value
	case *ast.MemberExpr:
		return exprPath(t.Container) + "." + t.Member
	case *ast.CallExpr:
		return exprPath(t.Left) + "()"
	default:
		return "expression"
	}
}

func (a *Analyzer) inferCallExprOp(expr *ast.CallExpr) Operand {
	leftOp := a.inferExprOp(expr.Left)

	switch leftOp.Mode {
	case Invalid:
		return leftOp
	case TypeExpr:
		switch t := leftOp.Type.(type) {
		case *StructType:
			return a.inferStructCallExprOp(t, expr)
		default:
			a.addErrorF(expr.Left, "cannot construct `%s`", leftOp.Symbol.Name())
			return invalidOperand(leftOp.Symbol)
		}
	case Value, Variable:
		switch t := leftOp.Type.(type) {
		case *FunctionType:
			return a.inferFunctionCallExprOp(t, expr)
		default:
			a.addErrorF(expr.Left, "cannot call `%s` of type `%s`",
				exprPath(expr.Left), t.Name())
			return invalidOperand(leftOp.Symbol)
		}
	case Module:
		a.addErrorF(expr.Left, "module calls not yet implmented")
		return invalidOperand(leftOp.Symbol)
	case VariantConstructor:
		switch t := leftOp.Type.(type) {
		case *EnumType:
			return a.inferEnumVariantCallExprOp(t, leftOp.Member, expr)
		default:
			a.addErrorF(expr.Left, "constructor call on non variant")
			return invalidOperand(leftOp.Symbol)
		}
	default:
		a.addErrorF(expr, "unknown expression")
		return invalidOperand(leftOp.Symbol)
	}
}

func (a *Analyzer) inferIdentifierExprOp(expr *ast.IdentifierExpr) Operand {
	name := expr.Value
	sym, _ := a.CurrentScope.lookup(name)
	if sym == nil {
		a.addErrorF(expr, "`%s` not found", name)
		return invalidOperand(nil)
	}
	if sym, ok := sym.(*VarSymbol); ok {
		if sym.State() == Unresolved {
			a.resolveVarSymbol(sym)
		} else if sym.State() == Resolving {
			a.addErrorF(sym.Node, "init cycle: `%s` refers to itself", sym.Name())
			return invalidOperand(sym)
		}
	}

	switch sym.(type) {
	case *VarSymbol:
		return Operand{Mode: Variable, Type: sym.GetType(), Symbol: sym}
	case *TypeSymbol:
		return Operand{Mode: TypeExpr, Type: sym.GetType(), Symbol: sym}
	case *FunctionSymbol:
		return Operand{Mode: Value, Type: sym.GetType()}
	case *ImportSymbol:
		return Operand{Mode: Module, Type: sym.GetType(), Symbol: sym}
	}
	return invalidOperand(sym)
}

func (a *Analyzer) inferMemberExprType(expr *ast.MemberExpr) Operand {
	containerOp := a.inferExprOp(expr.Container)
	if containerOp.Type == InvalidT {
		return invalidOperand(containerOp.Symbol)
	}

	switch containerOp.Mode {
	case TypeExpr:
		switch t := containerOp.Type.(type) {
		case *EnumType:
			member := t.GetMember(expr.Member)
			if member == nil {
				a.addErrorF(expr.Container, "`%s` has no variant `%s`",
					exprPath(expr.Container), expr.Member)
				return invalidOperand(nil)
			}
			if len(member.Fields) < 1 {
				return Operand{Mode: Value, Type: t}
			}
			return Operand{Mode: VariantConstructor, Type: t, Member: member}
		default:
			a.addErrorF(expr, "invalid member access on `%s`", t.Name())
			return invalidOperand(nil)
		}
	case Variable, Value:
		switch t := containerOp.Type.(type) {
		case *StructType:
			field := t.GetField(expr.Member)
			if field != nil {
				return Operand{Mode: containerOp.Mode,
					Type: field.Type, Symbol: containerOp.Symbol}
			}
			method := t.GetMethod(expr.Member)
			if method != nil {
				return Operand{Mode: Value, Type: method.Type}
			}
			a.addErrorF(expr, "`%s` has no field or method `%s`",
				t.Name(), expr.Member)
			return invalidOperand(containerOp.Symbol)
		case *EnumType:
			method := t.GetMethod(expr.Member)
			if method != nil {
				return Operand{Mode: Value, Type: method.Type}
			}
			a.addErrorF(expr, "`%s` has no method `%s`",
				t.Name(), expr.Member)
			// return invalidOperand(containerOp.Symbol)
			return Operand{Mode: Value, Type: t}
		case *InterfaceType:
			method := t.GetMethod(expr.Member)
			if method != nil {
				return Operand{Mode: Value, Type: method}
			}
			a.addErrorF(expr, "`%s` has no method `%s`",
				t.Name(), expr.Member)
			return invalidOperand(containerOp.Symbol)
		default:
			a.addErrorF(expr, "`%s` has member `%s`",
				t.Name(), expr.Member)
			return invalidOperand(containerOp.Symbol)
		}
	case Module:
		a.addErrorF(expr, "module inference not yet implemented")
		return invalidOperand(containerOp.Symbol)
	default:
		return invalidOperand(containerOp.Symbol)
	}
}

func (a *Analyzer) inferExprOp(expr ast.Expression) Operand {
	switch et := expr.(type) {
	case *ast.IntegerLiteralExpr:
		return Operand{Mode: Value, Type: IntT}
	case *ast.FloatLiteralExpr:
		return Operand{Mode: Value, Type: FloatT}
	case *ast.StringLiteralExpr:
		return Operand{Mode: Value, Type: StrT}
	case *ast.BoolLiteralExpr:
		return Operand{Mode: Value, Type: BoolT}
	case *ast.InfixExpr:
		return a.inferInfixExprOp(et)
	case *ast.CallExpr:
		return a.inferCallExprOp(et)
	case *ast.IdentifierExpr:
		return a.inferIdentifierExprOp(et)
	case *ast.MemberExpr:
		return a.inferMemberExprType(et)
	}
	return invalidOperand(nil)
}

func identicalFunctionType(a, b *FunctionType) bool {
	if len(a.Params) != len(b.Params) {
		return false
	}
	if !identical(a.ReturnType, b.ReturnType) {
		return false
	}
	for idx, p := range a.Params {
		if !identical(p.Type, b.Params[idx].Type) {
			return false
		}
	}
	return true
}

func identical(a, b Type) bool {
	switch at := a.(type) {
	case *PrimitiveType:
		bt, ok := b.(*PrimitiveType)
		if !ok {
			return false
		}
		return at == bt
	case *StructType:
		bt, ok := b.(*StructType)
		if !ok {
			return false
		}
		return at == bt
	case *EnumType:
		bt, ok := b.(*EnumType)
		if !ok {
			return false
		}
		return at == bt
	case *SliceType:
		bt, ok := b.(*SliceType)
		if !ok {
			return false
		}
		return identical(at.ElementType, bt.ElementType)
	case *FunctionType:
		bt, ok := b.(*FunctionType)
		if !ok {
			return false
		}
		return identicalFunctionType(at, bt)
	}
	return false
}

func (a *Analyzer) resolveVarSymbol(sym *VarSymbol) {
	if sym.State() == Resolved {
		return
	}
	sym.state = Resolving
	declaredType := a.resolveType(sym.Node.Type)
	inferredOp := a.inferExprOp(sym.Node.Value)
	if declaredType != nil && !identical(declaredType, inferredOp.Type) {
		a.addErrorF(sym.Node, "type `%s` != `%s`", declaredType.Name(),
			inferredOp.Type.Name())
		sym.Type = InvalidT
	} else if declaredType != nil {
		sym.Type = declaredType
	} else {
		sym.Type = inferredOp.Type
	}
	sym.state = Resolved
}

func (a *Analyzer) resolveSymbol(sym Symbol) {
	switch symT := sym.(type) {
	case *VarSymbol:
		a.resolveVarSymbol(symT)
	}
}

func (a *Analyzer) resolveScope(s *Scope) {
	for _, sym := range s.Symbols() {
		a.resolveSymbol(sym)
	}
}

func (a *Analyzer) resolveParam(
	node *ast.Parameter, ft *FunctionType, target Type,
) {
	p := Param{Node: node, Id: node.Name}
	switch {
	case p.Id == "self" && target != nil:
		p.Type = target
		ft.Receiver = &p
		return
	case p.Id == "self" && target == nil:
		a.addErrorF(node, "only methods accept `self`")
		return
	default:
		p.Type = a.resolveType(node.Type)
	}
	ft.Params = append(ft.Params, p)
}

func (a *Analyzer) resolveFunctionSymbol(sym *FunctionSymbol, target *TypeSymbol) {
	ft := &FunctionType{Node: sym.Node.Signature}
	ft.ReturnType = a.resolveType(sym.Node.Signature.ReturnType)
	for _, p := range sym.Node.Signature.Parameters {
		if target != nil {
			a.resolveParam(p, ft, target.Type)
		} else {
			a.resolveParam(p, ft, nil)
		}
	}
	sym.Type = ft
	sym.state = Resolved
}

func (a *Analyzer) resolveFunctionSymbols(s *Scope) {
	for _, sym := range s.Symbols() {
		if fs, ok := sym.(*FunctionSymbol); ok {
			a.resolveFunctionSymbol(fs, nil)
		}
	}
}

func Run(prog *ast.Program) *Analyzer {
	a := NewAnalyzer()
	a.collectPackage(prog)
	a.resolveTypeDeclSymbols(a.Package)
	a.resolveFunctionSymbols(a.Package)
	a.resolvePackageImpls(prog)
	a.resolveScope(a.Package)

	return a
}
