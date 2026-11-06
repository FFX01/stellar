package hir

import (
	"fmt"
	"regexp"
	"stellar/internal/ast"
	"stellar/internal/errors"

	"github.com/sanity-io/litter"
)

type Scope struct {
	Name     string
	Parent   *Scope
	Children []*Scope
	Types    map[string]Type
	Values   map[string]Value
}

func newScope(parent *Scope, name string) *Scope {
	s := &Scope{Name: name, Parent: parent}
	s.Types = map[string]Type{}
	s.Values = map[string]Value{}
	if parent != nil {
		parent.Children = append(parent.Children, s)
	}
	return s
}
func (s *Scope) delcareType(name string, typ Type) error {
	if _, ok := s.Types[name]; ok {
		return fmt.Errorf("redaclaration of `%s`", name)
	}
	s.Types[name] = typ
	return nil
}
func (s *Scope) declareValue(name string, val Value) error {
	if _, ok := s.Values[name]; ok {
		return fmt.Errorf("redeclaration of `%s`", name)
	}
	s.Values[name] = val
	return nil
}
func (s *Scope) lookupType(name string) (Type, error) {
	if t, ok := s.Types[name]; ok {
		if t.GetState() != Resolved {
			return nil, fmt.Errorf("type `%s` not fully resolved", name)
		}
		return t, nil
	}
	if s.Parent != nil {
		return s.Parent.lookupType(name)
	}
	return nil, fmt.Errorf("type `%s` not found", name)
}
func (s *Scope) lookupValue(name string) (Value, error) {
	if v, ok := s.Values[name]; ok {
		return v, nil
	}
	if s.Parent != nil {
		return s.Parent.lookupValue(name)
	}
	return nil, fmt.Errorf("value `%s` not found", name)
}

type ResolutionState int

const (
	Unresolved ResolutionState = iota
	Resolved
)

type Operator int

const (
	OpAdd Operator = iota
	OpSub
	OpMul
	OpDiv
	OpMod
	OpEq
	OpNeq
	OpLt
	OpLTE
	OpGt
	OpGte
)

var OpMap = map[string]Operator{
	"+":  OpAdd,
	"-":  OpSub,
	"*":  OpMul,
	"/":  OpDiv,
	"%":  OpMod,
	"==": OpEq,
	"!=": OpNeq,
	"<":  OpLt,
	"<=": OpLTE,
	">":  OpGt,
	">=": OpGte,
}

type Type interface {
	typ()
	GetState() ResolutionState
	getName() string
}

type StructField struct {
	Node *ast.Field
	Name string
	Type Type
}

type StructT struct {
	Node   *ast.TypeDeclStmt
	Name   string
	State  ResolutionState
	Fields map[string]StructField
}

func (st *StructT) typ()                      {}
func (st *StructT) GetState() ResolutionState { return st.State }
func (st *StructT) getName() string           { return st.Name }

type PrimitiveType struct {
	Name string
}

func (pt *PrimitiveType) typ()                      {}
func (pt *PrimitiveType) GetState() ResolutionState { return Resolved }
func (pt *PrimitiveType) getName() string           { return pt.Name }

var (
	IntT     = &PrimitiveType{Name: "int"}
	FloatT   = &PrimitiveType{Name: "float"}
	StrT     = &PrimitiveType{Name: "str"}
	BoolT    = &PrimitiveType{Name: "bool"}
	VoidT    = &PrimitiveType{Name: "void"}
	InvalidT = &PrimitiveType{Name: "invalid"}
	SelfT    = &PrimitiveType{Name: "self"}
	PrintT   = &FunctionT{
		Name: "print",
		Params: []FParam{
			{Name: "value", Type: StrT},
		},
		ReturnType: VoidT,
		RState:     Resolved,
	}
)

type FParam struct {
	Node ast.Node
	Name string
	Type Type
}

type FunctionT struct {
	Node       ast.Node
	Name       string
	Params     []FParam
	ReturnType Type
	RState     ResolutionState
}

func (ft *FunctionT) typ()                      {}
func (ft *FunctionT) GetState() ResolutionState { return ft.RState }
func (ft *FunctionT) getName() string           { return ft.Name }

type Value interface {
	val()
	GetType() Type
}

var (
	printV = &FunctionV{Name: "print", Type: PrintT}
)

type StructConstructionArg struct {
	Node  *ast.Argument
	Name  string
	Value Value
}

type StructConstruction struct {
	Node   *ast.CallExpr
	Callee *StructT
	Args   []StructConstructionArg
}

func (sc *StructConstruction) val()          {}
func (sc *StructConstruction) GetType() Type { return sc.Callee }

type StructV struct {
	Name string
	Type *StructT
}

func (sv *StructV) val()          {}
func (sv *StructV) GetType() Type { return sv.Type }

type FunctionV struct {
	Name string
	Type *FunctionT
}

func (fv *FunctionV) val()          {}
func (fv *FunctionV) GetType() Type { return fv.Type }

type FunctionCallArgument struct {
	Node  *ast.Argument
	Value Value
}

func (a *FunctionCallArgument) val()          {}
func (a *FunctionCallArgument) GetType() Type { return a.Value.GetType() }

type FunctionCall struct {
	Node   *ast.CallExpr
	Callee *FunctionT
	Args   []FunctionCallArgument
	Type   Type
}

func (fc *FunctionCall) val()          {}
func (fc *FunctionCall) GetType() Type { return fc.Type }

type Variable struct {
	Node         *ast.VarStmt
	Name         string
	DeclaredType Type
	InferredType Type
	ResolvedType Type
	Value        Value
}

func (v *Variable) val()          {}
func (v *Variable) GetType() Type { return v.ResolvedType }

type BinaryValue struct {
	Node  *ast.InfixExpr
	Op    Operator
	Left  Value
	Right Value
	Type  Type
}

func (bv *BinaryValue) val()          {}
func (bv *BinaryValue) GetType() Type { return bv.Type }

var (
	InvalidV = &InvalidValue{}
)

type InvalidValue struct{}

func (iv *InvalidValue) val()          {}
func (iv *InvalidValue) GetType() Type { return InvalidT }

type IntLiteral struct {
	Node  *ast.IntegerLiteralExpr
	Value int
}

func (il *IntLiteral) val()          {}
func (il *IntLiteral) GetType() Type { return IntT }

type FloatLiteral struct {
	Node  *ast.FloatLiteralExpr
	Value float64
}

func (fl *FloatLiteral) val()          {}
func (fl *FloatLiteral) GetType() Type { return FloatT }

type BoolLiteral struct {
	Node  *ast.BoolLiteralExpr
	Value bool
}

func (bl *BoolLiteral) val()          {}
func (bl *BoolLiteral) GetType() Type { return BoolT }

type StrLiteral struct {
	Node  *ast.StringLiteralExpr
	Value string
}

func (sl *StrLiteral) val()          {}
func (sl *StrLiteral) GetType() Type { return StrT }

type FunctionParamVar struct {
	Node  ast.Node
	Name  string
	Index int
	Type  Type
}

func (fpv *FunctionParamVar) val()          {}
func (fpv *FunctionParamVar) GetType() Type { return fpv.Type }

type Statement interface {
	GetNode() ast.Node
	stmt()
}

type ExpressionStatement struct {
	Node  ast.Expression
	Value Value
}

func (es *ExpressionStatement) stmt()             {}
func (es *ExpressionStatement) GetNode() ast.Node { return es.Node }

type ReturnStatement struct {
	Node  *ast.ReturnStmt
	Value Value
}

func (rs *ReturnStatement) stmt()             {}
func (rs *ReturnStatement) GetNode() ast.Node { return rs.Node }

type FunctionBody struct {
	Node      ast.Node
	Type      *FunctionT
	ParamVars []*FunctionParamVar
	Body      []Statement
}

func (fb *FunctionBody) val() {}

type Analyzer struct {
	Universe       *Scope
	Package        *Scope
	CurrentScope   *Scope
	Errors         []errors.Error
	functionBodies map[string]*FunctionBody
}

func (a *Analyzer) storeFunctionBody(typeName string, fb *FunctionBody) {
	namespace := typeName
	if namespace == "" {
		namespace = "_"
	}
	fullName := fmt.Sprintf("%s.%s", namespace, fb.Type.Name)
	if _, ok := a.functionBodies[fullName]; ok {
		a.errorF(fb.Node, "redaclarion of function body for %s", fullName)
	}
	a.functionBodies[fullName] = fb
}

func (a *Analyzer) enterScope(name string) {
	s := newScope(a.CurrentScope, name)
	a.CurrentScope = s
}

func (a *Analyzer) exitScope() {
	if a.CurrentScope.Parent != nil {
		a.CurrentScope = a.CurrentScope.Parent
	}
}

func (a *Analyzer) Dump() {
	litter.Config.FieldExclusions = regexp.MustCompile(`^(Node)$`)
	litter.Dump(a.Universe)
	litter.Dump(a.functionBodies)
	if len(a.Errors) > 0 {
		fmt.Println("--- Errors ---")
		for _, e := range a.Errors {
			fmt.Println(e.String())
		}
	}
}

func (a *Analyzer) errorF(node ast.Node, format string, args ...any) {
	err := fmt.Errorf(format, args...)
	a.addError(node, err)
}

func (a *Analyzer) addError(node ast.Node, e error) {
	se := errors.Error{Position: node.Pos(), Message: e.Error()}
	a.Errors = append(a.Errors, se)
}

func newUniverse() *Scope {
	u := &Scope{Name: "universe"}
	u.Types = map[string]Type{
		"int":   IntT,
		"float": FloatT,
		"str":   StrT,
		"bool":  BoolT,
		"void":  VoidT,
		"print": PrintT,
	}
	u.Values = map[string]Value{
		"print": printV,
	}
	return u
}

func (a *Analyzer) collectFunctionType(n *ast.FunctionStmt) {
	ft := &FunctionT{Node: n, Name: n.Signature.Name, RState: Unresolved}
	fv := &FunctionV{Name: ft.Name, Type: ft}
	if err := a.CurrentScope.delcareType(ft.Name, ft); err != nil {
		a.addError(n, err)
	}
	if err := a.CurrentScope.declareValue(fv.Name, fv); err != nil {
		a.addError(n, err)
	}
}

func (a *Analyzer) collectStructType(st *ast.StructType, stmt *ast.TypeDeclStmt) {
	t := &StructT{Node: stmt, Name: stmt.Name, State: Resolved}
	t.Fields = map[string]StructField{}
	if err := a.CurrentScope.delcareType(t.Name, t); err != nil {
		a.addError(stmt, err)
	}
	v := &StructV{Name: t.Name, Type: t}
	if err := a.CurrentScope.declareValue(v.Name, v); err != nil {
		a.addError(stmt, err)
	}
}

func (a *Analyzer) collectTypeDeclStmt(stmt *ast.TypeDeclStmt) {
	switch t := stmt.Definition.(type) {
	case *ast.StructType:
		a.collectStructType(t, stmt)
	}
}

func (a *Analyzer) pass1(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		switch t := stmt.(type) {
		case *ast.FunctionStmt:
			a.collectFunctionType(t)
		case *ast.TypeDeclStmt:
			a.collectTypeDeclStmt(t)
		}
	}
}

func (a *Analyzer) resolveFunctionParam(t *FunctionT, p *ast.Parameter) {
	fp := FParam{Node: p, Name: p.Name}
	tname := getTypeName(p.Type)
	typ, err := a.CurrentScope.lookupType(tname)
	if err != nil {
		a.addError(p, err)
		return
	}
	fp.Type = typ
	t.Params = append(t.Params, fp)
}

func (a *Analyzer) resolveFunctionT(t *FunctionT) {
	stmt, ok := t.Node.(*ast.FunctionStmt)
	if !ok {
		return
	}
	rtName := getTypeName(stmt.Signature.ReturnType)
	rt, err := a.CurrentScope.lookupType(rtName)
	if err != nil {
		a.addError(stmt.Signature.ReturnType, err)
		return
	}
	t.ReturnType = rt
	for _, p := range stmt.Signature.Parameters {
		a.resolveFunctionParam(t, p)
	}
	t.RState = Resolved
}

func (a *Analyzer) resolveStructT(t *StructT) {
	decl := t.Node.Definition
	stmt, ok := decl.(*ast.StructType)
	if !ok {
		a.errorF(stmt, "Invalid struct definition")
		return
	}
	for _, f := range stmt.Fields {
		field := StructField{Node: f, Name: f.Name}
		tname := getTypeName(f.Type)
		ft, err := a.CurrentScope.lookupType(tname)
		if err != nil {
			a.addError(f, err)
			field.Type = InvalidT
		} else {
			field.Type = ft
		}
		t.Fields[field.Name] = field
	}
}

func (a *Analyzer) pass2() {
	for _, t := range a.Package.Types {
		switch t := t.(type) {
		case *FunctionT:
			a.resolveFunctionT(t)
		case *StructT:
			a.resolveStructT(t)
		}
	}
}

func (a *Analyzer) resolveReturnStmt(fb *FunctionBody, stmt *ast.ReturnStmt) {
	s := &ReturnStatement{Node: stmt}
	if stmt.Value != nil {
		s.Value = a.resolveExpr(stmt.Value)
	}

	switch {
	case fb.Type.ReturnType == VoidT && s.Value != nil:
		a.errorF(stmt, "function `%s` returns no value", fb.Type.Name)
	case fb.Type.ReturnType != VoidT && s.Value == nil:
		a.errorF(stmt,
			"function `%s` must return `%s`", fb.Type.Name, fb.Type.ReturnType.getName())
	case s.Value != nil:
		if err := identical(s.Value.GetType(), fb.Type.ReturnType); err != nil {
			a.addError(stmt, err)
		}
	}
	fb.Body = append(fb.Body, s)
}

func (a *Analyzer) resolveFunctionBodyStmt(fb *FunctionBody, stmt ast.Statement) {
	switch t := stmt.(type) {
	case *ast.ReturnStmt:
		a.resolveReturnStmt(fb, t)
	}
}

func (a *Analyzer) resolveFunctionBodyExpr(fb *FunctionBody, expr ast.Expression) {
	v := a.resolveExpr(expr)
	es := &ExpressionStatement{Node: expr, Value: v}
	fb.Body = append(fb.Body, es)
}

func (a *Analyzer) resolveFunctionBody(t *FunctionT) {
	a.enterScope(t.Name)
	defer a.exitScope()

	stmt, ok := t.Node.(*ast.FunctionStmt)
	if !ok {
		a.errorF(stmt, "function expressions not yet supported")
	}
	fb := &FunctionBody{Node: stmt.Body, Type: t}

	for idx, p := range t.Params {
		pv := &FunctionParamVar{Node: p.Node, Name: p.Name, Index: idx, Type: p.Type}
		fb.ParamVars = append(fb.ParamVars, pv)
		if err := a.CurrentScope.declareValue(pv.Name, pv); err != nil {
			a.addError(p.Node, err)
		}
	}

	switch t := stmt.Body.(type) {
	case *ast.BlockExpr:
		for _, s := range t.Statements {
			a.resolveFunctionBodyStmt(fb, s)
		}
	default:
		a.resolveFunctionBodyExpr(fb, t)
	}

	a.storeFunctionBody("", fb)
}

func (a *Analyzer) pass4() {
	for _, t := range a.Package.Types {
		switch t := t.(type) {
		case *FunctionT:
			a.resolveFunctionBody(t)
		}
	}
}

func (a *Analyzer) resolveVarStmt(stmt *ast.VarStmt) {
	v := &Variable{Node: stmt, Name: stmt.Name.Literal}
	if stmt.Type != nil {
		tname := getTypeName(stmt.Type)
		t, err := a.CurrentScope.lookupType(tname)
		if err != nil {
			a.addError(stmt.Type, err)
			v.DeclaredType = InvalidT
		} else {
			v.DeclaredType = t
		}
	}
	val := a.resolveExpr(stmt.Value)
	if val == InvalidV {
		a.errorF(stmt.Value, "invalid expression")
		v.Value = InvalidV
		v.InferredType = InvalidT
	} else {
		v.Value = val
		v.InferredType = val.GetType()
	}
	switch {
	case v.DeclaredType == InvalidT || v.InferredType == InvalidT:
		v.ResolvedType = InvalidT
	case v.DeclaredType != nil:
		if err := identical(v.DeclaredType, v.InferredType); err != nil {
			a.addError(stmt, err)
			v.ResolvedType = InvalidT
		}
		v.ResolvedType = v.DeclaredType
	case v.DeclaredType == nil:
		v.ResolvedType = v.InferredType
	}
	if err := a.CurrentScope.declareValue(v.Name, v); err != nil {
		a.addError(stmt, err)
	}
}

func (a *Analyzer) pass3(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		switch t := stmt.(type) {
		case *ast.VarStmt:
			a.resolveVarStmt(t)
		}
	}
}

func (a *Analyzer) Run(prog *ast.Program) {
	a.pass1(prog) // collect top-level function and type names
	a.pass2()     // resolve function and method types
	a.pass3(prog) // resolve top-level vars
	a.pass4()     // resolve function and method bodies
}

func NewAnalyzer() *Analyzer {
	universe := newUniverse()
	pkg := newScope(universe, "package")
	a := &Analyzer{
		Universe:     universe,
		Package:      pkg,
		CurrentScope: pkg,
	}
	a.functionBodies = map[string]*FunctionBody{}
	return a
}

func getTypeName(typ ast.Type) string {
	switch t := typ.(type) {
	case nil:
		return "void"
	case *ast.NamedType:
		return t.Name
	}
	return ""
}

func (a *Analyzer) resolveIdentifierExpr(expr *ast.IdentifierExpr) Value {
	val, err := a.CurrentScope.lookupValue(expr.Value)
	if err != nil {
		a.addError(expr, err)
		return InvalidV
	}
	return val
}

func getBinaryType(op Operator, operand Type) (Type, error) {
	defaultErr := func(op, tname string) error {
		return fmt.Errorf("cannot use `%s` on type `%s`", op, tname)
	}
	switch op {
	case OpAdd:
		switch operand {
		case IntT, StrT, FloatT:
			return operand, nil
		default:
			return InvalidT, defaultErr("+", operand.getName())
		}
	case OpSub:
		switch operand {
		case IntT, FloatT:
			return operand, nil
		default:
			return InvalidT, defaultErr("-", operand.getName())
		}
	case OpMul:
		switch operand {
		case IntT, FloatT:
			return operand, nil
		default:
			return InvalidT, defaultErr("*", operand.getName())
		}
	case OpDiv:
		switch operand {
		case IntT, FloatT:
			return operand, nil
		default:
			return InvalidT, defaultErr("/", operand.getName())
		}
	case OpMod:
		switch operand {
		case IntT:
			return operand, nil
		default:
			return InvalidT, defaultErr("%", operand.getName())
		}
	case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLTE:
		return BoolT, nil
	}
	return InvalidT, fmt.Errorf("invalid operator")
}

func (a *Analyzer) resolveInfixExpr(expr *ast.InfixExpr) Value {
	bv := &BinaryValue{Node: expr}
	bv.Left = a.resolveExpr(expr.Left)
	bv.Right = a.resolveExpr(expr.Right)
	op, ok := OpMap[expr.Operator]
	if !ok {
		a.errorF(expr, "invalid operator `%s`", expr.Operator)
		return InvalidV
	}
	bv.Op = op

	if err := identical(bv.Left.GetType(), bv.Right.GetType()); err != nil {
		a.addError(expr, err)
		return InvalidV
	}
	if bv.Left.GetType() == InvalidT || bv.Right.GetType() == InvalidT {
		return InvalidV
	}
	bt, err := getBinaryType(op, bv.Left.GetType())
	if err != nil {
		a.addError(expr, err)
	}
	bv.Type = bt
	return bv
}

// func (a *Analyzer) analyzeCallExpr(expr *ast.CallExpr) Value {
// 	leftValue := a.analyzeExpr(expr.Left)
// 	switch t := leftValue.getType().(type) {
// 	case *FunctionT:
// 		return a.analyzeFunctionCallExpr(t, expr)
// 	case *StructType:
// 		return a.analyzeStructConstructorCall(t, expr)
// 	default:
// 		return &InvalidValue{Node: expr}
// 	}
// }

func getArgByName(args []*ast.Argument, name string) (int, *ast.Argument, error) {
	for idx, a := range args {
		if a.Name == name {
			return idx, a, nil
		}
	}
	return 0, nil, fmt.Errorf("argument with name `%s` not found", name)
}

func (a *Analyzer) resolveFunctionCall(ft *FunctionT, expr *ast.CallExpr) Value {
	fc := &FunctionCall{Node: expr, Callee: ft, Type: ft.ReturnType}
	for idx, p := range ft.Params {
		var arg *ast.Argument
		_, namedArg, err := getArgByName(expr.Args, p.Name)
		if err == nil {
			arg = namedArg
		} else {
			if idx >= len(expr.Args) {
				a.errorF(arg, "function`%s` expects %d args, but %d were passed",
					ft.Name, len(ft.Params), len(expr.Args))
				return fc
			}
			arg = expr.Args[idx]
			if idx > 0 && expr.Args[idx-1].Name != "" {
				a.errorF(arg, "positional arguments cannot come after named arguments")
				return fc
			}
		}
		argV := a.resolveExpr(arg.Value)
		if argV == InvalidV {
			a.errorF(arg, "unknown expression: `%T`", arg.Value)
			return fc
		}
		if err := identical(argV.GetType(), p.Type); err != nil {
			a.addError(arg, err)
			return fc
		}
		fc.Args = append(fc.Args, FunctionCallArgument{Node: arg, Value: argV})
	}
	return fc
}

func hasZeroValue(t Type) bool {
	switch t.(type) {
	case *PrimitiveType:
		return true
	case *StructT:
		return false
	}
	return false
}

func (a *Analyzer) resolveStructConstruction(st *StructT, expr *ast.CallExpr) Value {
	sc := &StructConstruction{Node: expr, Callee: st}
	if len(expr.Args) > len(st.Fields) {
		a.errorF(expr, "too many arguments to constructor for struct `%s`", st.Name)
		return InvalidV
	}
	for n, f := range st.Fields {
		_, arg, err := getArgByName(expr.Args, n)
		if err != nil {
			if hasZeroValue(f.Type) {
				continue
			}
			a.addError(expr, err)
			return InvalidV
		}
		argV := a.resolveExpr(arg.Value)
		if argV == InvalidV {
			a.errorF(arg, "unknown expression `%T`", arg.Value)
			return InvalidV
		}
		if err := identical(f.Type, argV.GetType()); err != nil {
			a.addError(arg, err)
			return InvalidV
		}
		scArg := StructConstructionArg{Node: arg, Name: n, Value: argV}
		sc.Args = append(sc.Args, scArg)
	}
	return sc
}

func (a *Analyzer) resolveCallExpr(expr *ast.CallExpr) Value {
	leftVal := a.resolveExpr(expr.Left)
	switch t := leftVal.GetType().(type) {
	case *FunctionT:
		return a.resolveFunctionCall(t, expr)
	case *StructT:
		return a.resolveStructConstruction(t, expr)
	}
	return InvalidV
}

func (a *Analyzer) resolveExpr(expr ast.Expression) Value {
	switch t := expr.(type) {
	case *ast.IntegerLiteralExpr:
		return &IntLiteral{Node: t, Value: int(t.Value)}
	case *ast.FloatLiteralExpr:
		return &FloatLiteral{Node: t, Value: t.Value}
	case *ast.StringLiteralExpr:
		return &StrLiteral{Node: t, Value: t.Value}
	case *ast.BoolLiteralExpr:
		return &BoolLiteral{Node: t, Value: t.Value}
	case *ast.IdentifierExpr:
		return a.resolveIdentifierExpr(t)
	case *ast.InfixExpr:
		return a.resolveInfixExpr(t)
	case *ast.CallExpr:
		return a.resolveCallExpr(t)
		// case *ast.IdentifierExpr:
		// 	return a.analyzeIdentifierExpr(t)
		// default:
		// 	err := fmt.Errorf("invalid expression: `%s`", expr.String())
		// 	a.addError(expr, err)
		// 	return &InvalidValue{Node: expr}
	}
	return InvalidV
}
