/*
Package hir
*/
package hirold

import (
	"fmt"
	"regexp"
	"slices"
	"stellar/internal/ast"
	"stellar/internal/errors"
	"strings"

	"github.com/sanity-io/litter"
)

type ResolutionState int

const (
	Unresolved ResolutionState = iota
	Resolved
)

type Type interface {
	typ()
	Name() string
	State() ResolutionState
}
type Value interface {
	val()
	getType() Type
}

type Scope struct {
	Name     string
	Parent   *Scope
	Children []*Scope
	Types    map[string]Type
	Values   map[string]Value
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
func (s *Scope) lookupType(name string) Type {
	if t, ok := s.Types[name]; ok {
		return t
	}
	if s.Parent != nil {
		return s.Parent.lookupType(name)
	}
	return nil
}
func (s *Scope) lookupValue(name string) Value {
	if v, ok := s.Values[name]; ok {
		return v
	}
	if s.Parent != nil {
		return s.Parent.lookupValue(name)
	}
	return nil
}

type Param struct {
	Name     string
	Type     Type
	Variable *Variable
}

type ExpressionStmt struct {
	Value Value
}

func (es *ExpressionStmt) val()          {}
func (es *ExpressionStmt) getType() Type { return es.Value.getType() }

type FunctionT struct {
	Node       ast.Node
	name       string
	Params     []Param
	ReturnType Type
	state      ResolutionState
	Builtin    bool
}

func (ft *FunctionT) typ()                   {}
func (ft *FunctionT) Name() string           { return ft.name }
func (ft *FunctionT) State() ResolutionState { return ft.state }
func (ft *FunctionT) getParamByName(name string) *Param {
	for idx, p := range ft.Params {
		if p.Name == name {
			return &ft.Params[idx]
		}
	}
	return nil
}

type FunctionV struct {
	Node ast.Node
	Name string
	Type *FunctionT
	Body []Value
}

func (fv *FunctionV) val()          {}
func (fv *FunctionV) getType() Type { return fv.Type }

type FunctionCall struct {
	Node       ast.Node
	Receiver   Type
	Callee     *FunctionV
	Args       []Value
	ReturnType Type
}

func (fc *FunctionCall) val()          {}
func (fc *FunctionCall) getType() Type { return fc.ReturnType }

type StructConstructorCall struct {
	Node       ast.Node
	ReturnType *StructType
}

func (scc *StructConstructorCall) val()          {}
func (scc *StructConstructorCall) getType() Type { return scc.ReturnType }

type StrLiteral struct {
	Node  ast.Node
	Value string
}

func (l *StrLiteral) val()          {}
func (l *StrLiteral) getType() Type { return StrT }

type BoolLiteral struct {
	Node  ast.Node
	Value bool
}

func (l *BoolLiteral) val()          {}
func (l *BoolLiteral) getType() Type { return BoolT }

type IntLiteral struct {
	Node  ast.Node
	Value int
}

func (l *IntLiteral) val()          {}
func (l *IntLiteral) getType() Type { return IntT }

type FloatLiteral struct {
	Node  ast.Node
	Value float64
}

func (l *FloatLiteral) val()          {}
func (l *FloatLiteral) getType() Type { return FloatT }

type Producer struct {
	Node  ast.Node
	Type  Type
	Fresh bool
}

func (p *Producer) val()          {}
func (p *Producer) getType() Type { return p.Type }

type StructConstructor struct {
	Node ast.Node
	Name string
	Type *StructType
}

func (sc *StructConstructor) val()          {}
func (sc *StructConstructor) getType() Type { return sc.Type }

type ReturnStmt struct {
	Node  ast.Node
	Value Value
}

func (rs *ReturnStmt) val() {}
func (rs *ReturnStmt) getType() Type {
	if rs.Value != nil {
		return rs.Value.getType()
	}
	return VoidT
}

type Variable struct {
	Node         *ast.VarStmt
	Name         string
	Value        Value
	DeclaredType Type
	InferredType Type
	ResolvedType Type
	Mutable      bool
}

func (v *Variable) val()          {}
func (v *Variable) stmt()         {}
func (v *Variable) getType() Type { return v.ResolvedType }

type InvalidValue struct {
	Node ast.Node
}

func (i *InvalidValue) val()          {}
func (i *InvalidValue) getType() Type { return InvalidT }

type Field struct {
	Name   string
	Type   Type
	Public bool
}

type StructType struct {
	Node   ast.Node
	name   string
	Fields map[string]Field
	state  ResolutionState
}

func (st *StructType) typ()                   {}
func (st *StructType) Name() string           { return st.name }
func (st *StructType) State() ResolutionState { return st.state }

type PrimitiveType struct {
	name string
}

func (pt *PrimitiveType) typ()                   {}
func (pt *PrimitiveType) Name() string           { return pt.name }
func (pt *PrimitiveType) State() ResolutionState { return Resolved }

var (
	IntT     = &PrimitiveType{name: "int"}
	FloatT   = &PrimitiveType{name: "float"}
	StrT     = &PrimitiveType{name: "str"}
	BoolT    = &PrimitiveType{name: "bool"}
	VoidT    = &PrimitiveType{name: "void"}
	InvalidT = &PrimitiveType{name: "invalid"}
)

// builtin functions
var (
	StPrint = &FunctionT{
		name: "print",
		Params: []Param{
			{Name: "value", Type: StrT},
		},
		Builtin:    true,
		ReturnType: VoidT,
		state:      Resolved,
	}
)

func newUniverse() *Scope {
	u := &Scope{Name: "universe"}
	u.Types = map[string]Type{
		"int":   IntT,
		"float": FloatT,
		"str":   StrT,
		"bool":  BoolT,
		"void":  VoidT,
		"print": StPrint,
	}
	return u
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

type Analyzer struct {
	Universe     *Scope
	Package      *Scope
	CurrentScope *Scope
	Errors       []errors.Error
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
	if len(a.Errors) > 0 {
		fmt.Println("--- Errors ---")
		for _, e := range a.Errors {
			fmt.Println(e.String())
		}
	}
}

func (a *Analyzer) addError(node ast.Node, e error) {
	se := errors.Error{Position: node.Pos(), Message: e.Error()}
	a.Errors = append(a.Errors, se)
}

func NewAnalyzer() *Analyzer {
	universe := newUniverse()
	pkg := newScope(universe, "package")
	a := &Analyzer{
		Universe:     universe,
		Package:      pkg,
		CurrentScope: pkg,
	}
	return a
}

func (a *Analyzer) declareType(name string, typ Type) error {
	return a.CurrentScope.delcareType(name, typ)
}

func (a *Analyzer) declareValue(name string, val Value) error {
	return a.CurrentScope.declareValue(name, val)
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

func identical(a, b Type) error {
	defaultErr := fmt.Errorf("type `%s` != type `%s`", a.Name(), b.Name())

	switch at := a.(type) {
	case *PrimitiveType:
		bt, ok := b.(*PrimitiveType)
		if !ok {
			return defaultErr
		}
		if at != bt {
			return defaultErr
		}
		return nil
	case *StructType:
		bt, ok := b.(*StructType)
		if !ok {
			return defaultErr
		}
		if at != bt {
			return defaultErr
		}
		return nil
	default:
		return fmt.Errorf("unknown type `%T`", a)
	}
}

func (a *Analyzer) analyzeInfixExpr(expr *ast.InfixExpr) Value {
	v := &Producer{Node: expr, Fresh: true}
	left := a.analyzeExpr(expr.Left)
	right := a.analyzeExpr(expr.Right)
	if err := identical(left.getType(), right.getType()); err != nil {
		a.addError(expr, err)
		return v
	}
	v.Type = left.getType()
	return v
}

func (a *Analyzer) analyzeFunctionCallExpr(fn *FunctionT, expr *ast.CallExpr) Value {
	fc := &FunctionCall{Node: expr, ReturnType: fn.ReturnType, Callee: fn}
	var satisfiedNames []string
	for idx, param := range fn.Params {
		var arg *ast.Argument
		for _, a := range expr.Args {
			if a.Name == param.Name {
				arg = a
				break
			}
		}
		if arg == nil {
			if idx < len(expr.Args) {
				arg = expr.Args[idx]
			} else {
				err := fmt.Errorf("function `%s` takes %d args, but %d were given",
					fn.Name(), len(fn.Params), len(expr.Args))
				a.addError(expr, err)
				return &InvalidValue{Node: expr}
			}
		}
		argValue := a.analyzeExpr(arg.Value)
		if idx > 0 && arg.Name == "" && expr.Args[idx-1].Name != "" {
			err := fmt.Errorf("postional args cannot come after named args")
			a.addError(arg, err)
			return &InvalidValue{Node: arg}
		}

		if err := identical(argValue.getType(), param.Type); err != nil {
			a.addError(arg, err)
			return &InvalidValue{Node: arg}
		}
		satisfiedNames = append(satisfiedNames, param.Name)
		fc.Args = append(fc.Args, argValue)
	}

	if len(satisfiedNames) < len(fn.Params) {
		err := fmt.Errorf("function `%s` takes %d args, but %d were given",
			fn.Name(), len(fn.Params), len(satisfiedNames))
		a.addError(expr, err)
		return &InvalidValue{Node: expr}
	}
	return fc
}

func hasZeroValue(t Type) bool {
	switch t.(type) {
	case *PrimitiveType:
		return true
	case *StructType:
		return false
	}
	return false
}

func (a *Analyzer) analyzeStructConstructorCall(
	st *StructType, expr *ast.CallExpr,
) Value {
	n := &StructConstructorCall{Node: expr, ReturnType: st}
	var filledFields []string
	for idx, arg := range expr.Args {
		if arg.Name == "" {
			err := fmt.Errorf("constructor args must be named")
			a.addError(arg, err)
			continue
		}
		if idx >= len(st.Fields) {
			err := fmt.Errorf("`%s` takes at most %d args, but got %d",
				st.Name(), len(st.Fields), len(expr.Args))
			a.addError(arg, err)
			continue
		}
		field, ok := st.Fields[arg.Name]
		if !ok {
			err := fmt.Errorf("struct `%s` has no field `%s`", st.Name(), arg.Name)
			a.addError(arg, err)
			continue
		}
		argValue := a.analyzeExpr(arg.Value)
		if err := identical(field.Type, argValue.getType()); err != nil {
			a.addError(arg, err)
			continue
		}
		filledFields = append(filledFields, arg.Name)
	}

	var missingFields []string
	for k, v := range st.Fields {
		if !slices.Contains(filledFields, k) && !hasZeroValue(v.Type) {
			missingFields = append(missingFields, k)
		}
	}
	if len(missingFields) > 0 {
		err := fmt.Errorf("missing fields in constructor call: `%s`",
			strings.Join(missingFields, ", "))
		a.addError(expr, err)
	}
	return n
}

func (a *Analyzer) analyzeCallExpr(expr *ast.CallExpr) Value {
	leftValue := a.analyzeExpr(expr.Left)
	switch t := leftValue.getType().(type) {
	case *FunctionT:
		return a.analyzeFunctionCallExpr(t, expr)
	case *StructType:
		return a.analyzeStructConstructorCall(t, expr)
	default:
		return &InvalidValue{Node: expr}
	}
}

func (a *Analyzer) analyzeIdentifierExpr(expr *ast.IdentifierExpr) Value {
	id := expr.Value
	val := a.CurrentScope.lookupValue(id)
	if val == nil {
		err := fmt.Errorf("value `%s` not found", id)
		a.addError(expr, err)
		return &InvalidValue{Node: expr}
	}
	return val
}

func (a *Analyzer) analyzeExpr(expr ast.Expression) Value {
	switch t := expr.(type) {
	case *ast.IntegerLiteralExpr:
		return &IntLiteral{Node: t, Value: int(t.Value)}
	case *ast.FloatLiteralExpr:
		return &FloatLiteral{Node: t, Value: t.Value}
	case *ast.StringLiteralExpr:
		return &StrLiteral{Node: t, Value: t.Value}
	case *ast.BoolLiteralExpr:
		return &BoolLiteral{Node: t, Value: t.Value}
	case *ast.InfixExpr:
		return a.analyzeInfixExpr(t)
	case *ast.CallExpr:
		return a.analyzeCallExpr(t)
	case *ast.IdentifierExpr:
		return a.analyzeIdentifierExpr(t)
	default:
		err := fmt.Errorf("invalid expression: `%s`", expr.String())
		a.addError(expr, err)
		return &InvalidValue{Node: expr}
	}
}

func (a *Analyzer) analyzeVarStmt(stmt *ast.VarStmt) Value {
	v := &Variable{Node: stmt, Name: stmt.Name.Literal}
	if stmt.Type != nil {
		typeName := getTypeName(stmt.Type)
		resolvedType := a.CurrentScope.lookupType(typeName)
		if resolvedType != nil {
			v.DeclaredType = resolvedType
		}
	}
	v.Value = a.analyzeExpr(stmt.Value)
	v.InferredType = v.Value.getType()
	if v.DeclaredType == nil {
		v.ResolvedType = v.InferredType
	} else if err := identical(v.DeclaredType, v.InferredType); err != nil {
		a.addError(stmt, err)
		v.ResolvedType = InvalidT
	} else {
		v.ResolvedType = v.InferredType
	}
	if err := a.declareValue(v.Name, v); err != nil {
		a.addError(stmt, err)
	}
	return v
}

func (a *Analyzer) analyzeFunctionParam(
	p *ast.Parameter, stmt *ast.FunctionStmt,
) Param {
	n := Param{Name: p.Name}
	typeName := getTypeName(p.Type)
	t := a.CurrentScope.lookupType(typeName)
	if t != nil {
		n.Type = t
	} else {
		a.addError(p, fmt.Errorf("type `%s` not found", typeName))
		n.Type = InvalidT
	}
	return n
}

func (a *Analyzer) analyzeFunctionStmt(stmt *ast.FunctionStmt) {
	ft := a.CurrentScope.lookupType(stmt.Signature.Name)
	if ft == nil {
		err := fmt.Errorf("function `%s` not found", stmt.Signature.Name)
		a.addError(stmt, err)
		return
	}
	if ft.State() == Resolved {
		return
	}
	f, ok := ft.(*FunctionT)
	if !ok {
		err := fmt.Errorf("invalid type for function")
		a.addError(stmt, err)
		return
	}
	rtName := getTypeName(stmt.Signature.ReturnType)
	rt := a.CurrentScope.lookupType(rtName)
	if rt == nil {
		a.addError(stmt.Signature.ReturnType,
			fmt.Errorf("unknown type: `%s`", rtName))
		f.ReturnType = InvalidT
	} else {
		f.ReturnType = rt
	}

	for _, p := range stmt.Signature.Parameters {
		f.Params = append(f.Params, a.analyzeFunctionParam(p, stmt))
	}
	f.state = Resolved

	fv := &FunctionV{Node: stmt.Body, Name: f.Name(), Type: f}
	if err := a.declareValue(fv.Name, fv); err != nil {
		a.addError(stmt.Body, err)
	}
}

func (a *Analyzer) analyzeStructDeclStmt(def *ast.StructType, stmt *ast.TypeDeclStmt) {
	maybeN := a.CurrentScope.lookupType(stmt.Name)
	if maybeN == nil {
		err := fmt.Errorf("`%s` not found", stmt.Name)
		a.addError(stmt, err)
		return
	}
	if maybeN.State() == Resolved {
		return
	}
	n, ok := maybeN.(*StructType)
	if !ok {
		a.addError(stmt, fmt.Errorf("unknown type: `%T`", maybeN))
	}
	n.Fields = map[string]Field{}
	for _, f := range def.Fields {
		newF := Field{Name: f.Name, Public: f.IsPublic}
		typ := a.CurrentScope.lookupType(getTypeName(f.Type))
		if typ == nil {
			err := fmt.Errorf("type `%s` not found", getTypeName(f.Type))
			a.addError(f, err)
		} else {
			newF.Type = typ
			n.Fields[newF.Name] = newF
		}
	}
	n.state = Resolved

	v := &StructConstructor{Node: stmt, Name: n.Name(), Type: n}
	if err := a.declareValue(v.Name, v); err != nil {
		a.addError(stmt, err)
	}
}

func (a *Analyzer) analyzeTypeDeclStmt(stmt *ast.TypeDeclStmt) {
	switch t := stmt.Definition.(type) {
	case *ast.StructType:
		a.analyzeStructDeclStmt(t, stmt)
	default:
		err := fmt.Errorf("unknown type declaration: `%T`", t)
		a.addError(stmt, err)
	}
}

func (a *Analyzer) analyzePass1(stmt ast.Statement) {
	switch t := stmt.(type) {
	case *ast.FunctionStmt:
		a.analyzeFunctionStmt(t)
	case *ast.TypeDeclStmt:
		a.analyzeTypeDeclStmt(t)
	}
}

// func (a *Analyzer) analyzeFunctionParam(
// 	p *ast.Parameter, stmt *ast.FunctionStmt,
// ) Param {
// 	n := Param{Name: p.Name}
// 	typeName := getTypeName(p.Type)
// 	t := a.CurrentScope.lookupType(typeName)
// 	if t != nil {
// 		n.Type = t
// 	} else {
// 		a.addError(p, fmt.Errorf("type `%s` not found", typeName))
// 		n.Type = InvalidT
// 	}
// 	return n
// }

func (a *Analyzer) analyzeStructMethod(s *StructType, m *ast.FunctionStmt) {
	name := fmt.Sprintf("%s.%s", s.Name(), m.Signature.Name)
	fn := Function{Node: m, name: name, ReceiverType: s,
		Builtin: false, state: Unresolved}
	rtName := getTypeName(m.Signature.ReturnType)
	rt := a.CurrentScope.lookupType(rtName)
	if rt == nil {
		a.addError(m.Signature.ReturnType,
			fmt.Errorf("invalid return type: `%s`", rtName))
		return
	}
	fn.ReturnType = rt
	for idx, p := range m.Signature.Parameters {
		switch {
		case idx == 0 && p.Name != "self":
			a.addError(p,
				fmt.Errorf("methods must always take `self` as their first parameter"))
			return
		case p.Name == "self" && p.Type != nil:
			a.addError(p,
				fmt.Errorf("`self` takes no type annotation"))
			return
		case p.Name == "self":
			fn.Params = append(fn.Params, Param{Name: "self", Type: s})
		}
	}
}

func (a *Analyzer) analyzeImplDeclStmt(stmt *ast.ImplDeclStmt) {
	astTarget, ok := stmt.Target.(*ast.NamedType)
	if !ok {
		a.addError(stmt,
			fmt.Errorf("cannot implement a non struct or enum type"))
		return
	}
	target := a.Package.lookupType(astTarget.Name)
	if target == nil {
		a.addError(stmt,
			fmt.Errorf("impl target `%s` not found", astTarget.Name))
		return
	}
}

func (a *Analyzer) analyzePass2(stmt ast.Statement) {
	switch t := stmt.(type) {
	case *ast.ImplDeclStmt:
		a.analyzeImplDeclStmt(t)
	}
}

func (a *Analyzer) analyzePass3(stmt ast.Statement) {
	switch t := stmt.(type) {
	case *ast.VarStmt:
		a.analyzeVarStmt(t)
	}
}

func (a *Analyzer) analyzeProgram(prog *ast.Program) {
	for _, s := range prog.Statements {
		a.analyzePass1(s)
	}
	for _, s := range prog.Statements {
		a.analyzePass2(s)
	}
	for _, s := range prog.Statements {
		a.analyzePass3(s)
	}
}

func (a *Analyzer) collectStructType(stmt *ast.TypeDeclStmt) {
	n := &StructType{Node: stmt, name: stmt.Name, state: Unresolved}
	n.Methods = map[string]Function{}
	if err := a.declareType(n.name, n); err != nil {
		err := fmt.Errorf("redeclaration of type `%s`", n.Name())
		a.addError(stmt, err)
	}
}

func (a *Analyzer) collectTypeDeclStmt(stmt *ast.TypeDeclStmt) {
	switch stmt.Definition.(type) {
	case *ast.StructType:
		a.collectStructType(stmt)
	}
}

func (a *Analyzer) collectFunctionStmt(stmt *ast.FunctionStmt) {
	n := &FunctionT{Node: stmt, name: stmt.Signature.Name, state: Unresolved}
	if err := a.declareType(n.Name(), n); err != nil {
		err := fmt.Errorf("redeclaration of function `%s`", n.Name())
		a.addError(stmt, err)
	}
}

func (a *Analyzer) collectFunctionsAndTypes(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		switch t := stmt.(type) {
		case *ast.TypeDeclStmt:
			a.collectTypeDeclStmt(t)
		case *ast.FunctionStmt:
			a.collectFunctionStmt(t)
		}
	}
}

func (a *Analyzer) declareFunctionParam(fn *Function, p *Param) {
	v := &Variable{Name: p.Name, DeclaredType: p.Type,
		InferredType: p.Type, ResolvedType: p.Type, Mutable: false}
	if err := a.declareValue(v.Name, v); err != nil {
		a.addError(fn.Node, err)
	}
	p.Variable = v
}

func (a *Analyzer) analyzeReturnStmt(fn *Function, stmt *ast.ReturnStmt) {
	v := &ReturnStmt{Node: stmt}
	if stmt.Value != nil {
		v.Value = a.analyzeExpr(stmt.Value)
	}

	switch {
	case fn.ReturnType == VoidT && v.Value != nil:
		a.addError(stmt,
			fmt.Errorf("function`%s` returns no value", fn.Name()))
	case fn.ReturnType != VoidT && v.Value == nil:
		a.addError(stmt,
			fmt.Errorf("function `%s` must return `%s`", fn.Name(), fn.ReturnType.Name()))
	case v.Value != nil:
		if err := identical(v.Value.getType(), fn.ReturnType); err != nil {
			a.addError(stmt, err)
		}
	}
	fn.Body = append(fn.Body, v)
}

func (a *Analyzer) analyzeFunctionBodyStmt(fn *Function, stmt ast.Statement) {
	switch t := stmt.(type) {
	case *ast.VarStmt:
		v := a.analyzeVarStmt(t)
		fn.Body = append(fn.Body, v)
	case *ast.ExpressionStatement:
		v := a.analyzeExpr(t.Expression)
		es := &ExpressionStmt{Value: v}
		fn.Body = append(fn.Body, es)
	case *ast.ReturnStmt:
		a.analyzeReturnStmt(fn, t)
	}
}

func (a *Analyzer) analyzeFunctionStmtBody(fn *Function, body ast.Expression) {
	block, ok := body.(*ast.BlockExpr)
	if !ok {
		err := fmt.Errorf("expected block body but got %T", body)
		a.addError(body, err)
		return
	}
	for _, stmt := range block.Statements {
		a.analyzeFunctionBodyStmt(fn, stmt)
	}

	if fn.ReturnType != VoidT {
		if _, ok := fn.Body[len(fn.Body)-1].(*ReturnStmt); !ok {
			a.addError(body,
				fmt.Errorf("function `%s` must return a value", fn.Name()))
		}
	}
}

func (a *Analyzer) analyzeFunctionBody(fn *Function) {
	a.enterScope(fn.Name())
	for idx := range fn.Params {
		a.declareFunctionParam(fn, &fn.Params[idx])
	}
	switch t := fn.Node.(type) {
	case *ast.FunctionStmt:
		a.analyzeFunctionStmtBody(fn, t.Body)
	}
	a.exitScope()
}

func (a *Analyzer) analyzeFunctionBodies() {
	for _, sym := range a.Package.Values {
		if fn, ok := sym.(*Function); ok {
			a.analyzeFunctionBody(fn)
		}
	}
}

func (a *Analyzer) Run(prog *ast.Program) {
	a.collectFunctionsAndTypes(prog)
	a.analyzeProgram(prog)
	a.analyzeFunctionBodies()
}
