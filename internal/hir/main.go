package hir

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
	name() string
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
	Name string
	Type Type
}

type Function struct {
	Node       ast.Node
	Name       string
	Receiver   Value
	Params     []Param
	ReturnType Type
	Body       *Scope
	state      ResolutionState
}

func (f *Function) typ()                   {}
func (f *Function) val()                   {}
func (f *Function) name() string           { return f.Name }
func (f *Function) getType() Type          { return f }
func (f *Function) State() ResolutionState { return f.state }
func (f *Function) getParamByName(name string) *Param {
	for idx, p := range f.Params {
		if p.Name == name {
			return &f.Params[idx]
		}
	}
	return nil
}

type FunctionCall struct {
	Node       ast.Node
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

type Literal struct {
	Node ast.Node
	Type Type
}

func (l *Literal) val()          {}
func (l *Literal) getType() Type { return l.Type }

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
	Name   string
	Fields map[string]Field
	state  ResolutionState
}

func (st *StructType) typ()                   {}
func (st *StructType) name() string           { return st.Name }
func (st *StructType) State() ResolutionState { return st.state }

type PrimitiveType struct {
	Name string
}

func (pt *PrimitiveType) typ()                   {}
func (pt *PrimitiveType) name() string           { return pt.Name }
func (pt *PrimitiveType) State() ResolutionState { return Resolved }

var (
	IntT     = &PrimitiveType{Name: "int"}
	FloatT   = &PrimitiveType{Name: "float"}
	StrT     = &PrimitiveType{Name: "str"}
	BoolT    = &PrimitiveType{Name: "bool"}
	VoidT    = &PrimitiveType{Name: "void"}
	InvalidT = &PrimitiveType{Name: "invalid"}
)

func newUniverse() *Scope {
	u := &Scope{Name: "universe"}
	u.Types = map[string]Type{
		"int":   IntT,
		"float": FloatT,
		"str":   StrT,
		"bool":  BoolT,
		"void":  VoidT,
	}
	u.Values = map[string]Value{}
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
	defaultErr := fmt.Errorf("type `%s` != type `%s`", a.name(), b.name())

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

func (a *Analyzer) analyzeFunctionCallExpr(fn *Function, expr *ast.CallExpr) Value {
	var requiredNames []string
	for _, p := range fn.Params {
		requiredNames = append(requiredNames, p.Name)
	}
	var satisfiedNames []string
	for idx, arg := range expr.Args {
		var param *Param
		argValue := a.analyzeExpr(arg.Value)
		if idx >= len(fn.Params) {
			a.addError(
				arg,
				fmt.Errorf(
					"function `%s` takes %d args, but %d were given",
					fn.Name, len(fn.Params), len(expr.Args),
				),
			)
			return &InvalidValue{Node: expr}
		}
		if idx > 0 && arg.Name == "" && expr.Args[idx-1].Name != "" {
			err := fmt.Errorf("postional args cannot come after named args")
			a.addError(arg, err)
			return &InvalidValue{Node: arg}
		}
		if arg.Name != "" {
			param = fn.getParamByName(arg.Name)
			if param == nil {
				err := fmt.Errorf("function `%s` has no param `%s`", fn.Name, arg.Name)
				a.addError(arg, err)
				return &InvalidValue{expr}
			}
		} else {
			param = &fn.Params[idx]
		}

		if err := identical(argValue.getType(), param.Type); err != nil {
			a.addError(arg, err)
			return &InvalidValue{Node: arg}
		}
		satisfiedNames = append(satisfiedNames, param.Name)
	}

	if len(satisfiedNames) < len(fn.Params) {
		err := fmt.Errorf("function `%s` takes %d args, but %d were given",
			fn.Name, len(fn.Params), len(satisfiedNames))
		a.addError(expr, err)
		return &InvalidValue{Node: expr}
	}

	return &FunctionCall{Node: expr, ReturnType: fn.ReturnType}
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
				st.Name, len(st.Fields), len(expr.Args))
			a.addError(arg, err)
			continue
		}
		field, ok := st.Fields[arg.Name]
		if !ok {
			err := fmt.Errorf("struct `%s` has no field `%s`", st.Name, arg.Name)
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
	case *Function:
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
		return &Literal{Node: t, Type: IntT}
	case *ast.FloatLiteralExpr:
		return &Literal{Node: t, Type: FloatT}
	case *ast.StringLiteralExpr:
		return &Literal{Node: t, Type: StrT}
	case *ast.BoolLiteralExpr:
		return &Literal{Node: t, Type: BoolT}
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

func (a *Analyzer) analyzeVarStmt(stmt *ast.VarStmt) {
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
	if err := identical(v.DeclaredType, v.Value.getType()); err != nil {
		a.addError(stmt, err)
		v.ResolvedType = InvalidT
	} else {
		v.ResolvedType = v.Value.getType()
	}
	if err := a.declareValue(v.Name, v); err != nil {
		a.addError(stmt, err)
	}
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
	f, ok := ft.(*Function)
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

	if err := a.declareValue(f.Name, f); err != nil {
		a.addError(stmt, err)
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

	v := &StructConstructor{Node: stmt, Name: n.Name, Type: n}
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

func (a *Analyzer) analyzeTypeOrFunctionDeclStmt(stmt ast.Statement) {
	switch t := stmt.(type) {
	case *ast.FunctionStmt:
		a.analyzeFunctionStmt(t)
	case *ast.TypeDeclStmt:
		a.analyzeTypeDeclStmt(t)
	default:
		err := fmt.Errorf("unknown statement: `%T`", stmt)
		a.addError(stmt, err)
	}
}

func (a *Analyzer) analyzeProgram(prog *ast.Program) {
	for _, s := range prog.Statements {
		if _, ok := s.(*ast.VarStmt); !ok {
			a.analyzeTypeOrFunctionDeclStmt(s)
		}
	}
	for _, s := range prog.Statements {
		if t, ok := s.(*ast.VarStmt); ok {
			a.analyzeVarStmt(t)
		}
	}
}

func (a *Analyzer) collectStructType(stmt *ast.TypeDeclStmt) {
	n := &StructType{Node: stmt, Name: stmt.Name, state: Unresolved}
	if err := a.declareType(n.Name, n); err != nil {
		err := fmt.Errorf("redeclaration of type `%s`", n.Name)
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
	n := &Function{Node: stmt, Name: stmt.Signature.Name, state: Unresolved}
	if err := a.declareType(n.Name, n); err != nil {
		err := fmt.Errorf("redeclaration of function `%s`", n.Name)
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

func (a *Analyzer) Run(prog *ast.Program) {
	a.collectFunctionsAndTypes(prog)
	a.analyzeProgram(prog)
}
