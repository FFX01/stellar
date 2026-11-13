/* Package hir */
package hir

import (
	"cmp"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"

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

func (s *Scope) declareType(name string, typ Type) error {
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
	OpAnd
	OpOr
)

var OpMap = map[string]Operator{
	"+":   OpAdd,
	"-":   OpSub,
	"*":   OpMul,
	"/":   OpDiv,
	"%":   OpMod,
	"==":  OpEq,
	"!=":  OpNeq,
	"<":   OpLt,
	"<=":  OpLTE,
	">":   OpGt,
	">=":  OpGte,
	"and": OpAnd,
	"or":  OpOr,
}

type Type interface {
	typ()
	GetState() ResolutionState
	getName() string
	GetMethod(string) (*FunctionT, error)
}

type ErrorType struct{}

func (et *ErrorType) typ()                      {}
func (et *ErrorType) getName() string           { return "Error" }
func (et *ErrorType) GetState() ResolutionState { return Resolved }
func (et *ErrorType) GetMethod(n string) (*FunctionT, error) {
	return nil, fmt.Errorf("`%s`, does not implement method `%s`",
		et.getName(), n,
	)
}
func (et *ErrorType) genError() *FunctionT {
	return &FunctionT{
		Name:       "error",
		Layer:      Builtin,
		RState:     Resolved,
		ReturnType: StrT,
		Params: []FParam{
			{Name: "self", Type: ErrorT},
		},
	}
}
func (et *ErrorType) genWrap() *FunctionT {
	return &FunctionT{
		Name:       "wrap",
		Layer:      Builtin,
		RState:     Resolved,
		ReturnType: ErrorT,
		Params: []FParam{
			{Name: "self", Type: ErrorT},
			{Name: "cause", Type: ErrorT},
		},
	}
}

type VariantT struct {
	Node   *ast.EnumMember
	Name   string
	Enum   *EnumT
	Tag    int
	State  ResolutionState
	Fields []Field
}

func (vt *VariantT) typ()                      {}
func (vt *VariantT) getName() string           { return vt.Enum.Name + "." + vt.Name }
func (vt *VariantT) GetState() ResolutionState { return vt.State }
func (vt *VariantT) GetMethod(n string) (*FunctionT, error) {
	return nil, fmt.Errorf("not implemented")
}

func (vt *VariantT) GetField(name string) (int, *Field, error) {
	for idx, f := range vt.Fields {
		if f.Name == name {
			return idx, &vt.Fields[idx], nil
		}
	}
	return 0, nil, fmt.Errorf("`%s` has no field `%s`", vt.Name, name)
}

type EnumT struct {
	Node     *ast.TypeDeclStmt
	Name     string
	State    ResolutionState
	Variants []*VariantT
	Methods  map[string]*FunctionT
}

func (et *EnumT) typ()                      {}
func (et *EnumT) GetState() ResolutionState { return et.State }
func (et *EnumT) getName() string           { return et.Name }
func (et *EnumT) GetVariant(name string) (int, *VariantT, error) {
	for idx, v := range et.Variants {
		if v.Name == name {
			return idx, et.Variants[idx], nil
		}
	}
	return 0, nil, fmt.Errorf("enum `%s` has no member `%s`", et.Name, name)
}

func (et *EnumT) GetMethod(n string) (*FunctionT, error) {
	if m, ok := et.Methods[n]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("not implemented")
}

type MaybeT struct {
	ElemType Type
}

func (mt *MaybeT) typ()                      {}
func (mt *MaybeT) GetState() ResolutionState { return Resolved }
func (mt *MaybeT) getName() string {
	return "Maybe"
}
func (mt *MaybeT) GetMethod(n string) (*FunctionT, error) {
	return nil, fmt.Errorf("`Maybe` has no method `%s`", n)
}

type ResultT struct {
	ElemType Type
}

func (rt *ResultT) typ()                      {}
func (rt *ResultT) GetState() ResolutionState { return Resolved }
func (rt *ResultT) getName() string           { return "Result" }
func (rt *ResultT) GetMethod(n string) (*FunctionT, error) {
	return nil, fmt.Errorf("`Result` has no method `%s`", n)
}

type SliceT struct {
	ElementType Type
}

func (st *SliceT) typ()                      {}
func (st *SliceT) GetState() ResolutionState { return Resolved }
func (st *SliceT) getName() string           { return "[]" + st.ElementType.getName() }
func (st *SliceT) GetMethod(name string) (*FunctionT, error) {
	switch name {
	case "append":
		return st.genAppend(), nil
	case "replace":
		return st.genReplace(), nil
	case "get":
		return st.genGet(), nil
	case "prepend":
		return st.genPrepend(), nil
	case "contains":
		return st.genContains(), nil
	default:
		return nil, fmt.Errorf("slice type has no method `%s`", name)
	}
}
func (st *SliceT) genContains() *FunctionT {
	return &FunctionT{
		Name:   "contains",
		Layer:  Builtin,
		RState: Resolved,
		Params: []FParam{
			{Name: "self", Type: st},
			{Name: "elem", Type: st.ElementType},
		},
		ReturnType: BoolT,
	}
}
func (st *SliceT) genPrepend() *FunctionT {
	return &FunctionT{
		Name:   "prepend",
		Layer:  Builtin,
		RState: Resolved,
		Params: []FParam{
			{Name: "self", Type: st},
			{Name: "elem", Type: st.ElementType},
		},
		ReturnType: st,
	}
}

func (st *SliceT) genAppend() *FunctionT {
	return &FunctionT{
		Name:       "append",
		Layer:      Builtin,
		ReturnType: st,
		RState:     Resolved,
		Params: []FParam{
			{Name: "self", Type: SelfT},
			{Name: "elem", Type: st.ElementType},
		},
	}
}
func (st *SliceT) genGet() *FunctionT {
	return &FunctionT{
		Name:       "get",
		Layer:      Builtin,
		ReturnType: &MaybeT{ElemType: st.ElementType},
		RState:     Resolved,
		Params: []FParam{
			{Name: "self", Type: st},
			{Name: "idx", Type: IntT},
		},
	}
}
func (st *SliceT) genReplace() *FunctionT {
	return &FunctionT{
		Name:       "replace",
		Layer:      Builtin,
		ReturnType: st,
		RState:     Resolved,
		Params: []FParam{
			{Name: "self", Type: SelfT},
			{Name: "idx", Type: IntT},
			{Name: "elem", Type: st.ElementType},
		},
	}
}

type Field struct {
	Node *ast.Field
	Name string
	Type Type
}

type StructT struct {
	Node    *ast.TypeDeclStmt
	Name    string
	State   ResolutionState
	Fields  []Field
	Methods map[string]*FunctionT
}

func (st *StructT) typ()                      {}
func (st *StructT) GetState() ResolutionState { return st.State }
func (st *StructT) getName() string           { return st.Name }
func (st *StructT) GetField(name string) (int, *Field, error) {
	if len(st.Fields) < 1 {
		return 0, nil, fmt.Errorf("struct `%s` has no fields", st.Name)
	}
	for idx, f := range st.Fields {
		if f.Name == name {
			return idx, &st.Fields[idx], nil
		}
	}
	return 0, nil, fmt.Errorf("struct `%s` has no field `%s`", st.Name, name)
}

func (st *StructT) GetMethod(n string) (*FunctionT, error) {
	// user defined
	if m, ok := st.Methods[n]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("`%s` has no method `%s`",
		st.Name, n,
	)
}

type PrimitiveType struct {
	Name string
}

func (pt *PrimitiveType) typ()                      {}
func (pt *PrimitiveType) GetState() ResolutionState { return Resolved }
func (pt *PrimitiveType) getName() string           { return pt.Name }
func (pt *PrimitiveType) GetMethod(name string) (*FunctionT, error) {
	switch name {
	case "string":
		ft := &FunctionT{
			Name:       fmt.Sprintf("%s.%s", pt.Name, "string"),
			ReturnType: StrT,
			RState:     Resolved,
			Layer:      Builtin,
			Params: []FParam{
				{Name: "self", Type: SelfT},
			},
		}
		return ft, nil
	}
	return nil, fmt.Errorf("not implemented")
}

var stringableMap = map[string]*FunctionT{
	"str": {
		Name:       "str.string",
		RState:     Resolved,
		ReturnType: StrT,
		Layer:      Builtin,
		Params: []FParam{
			{Name: "self", Type: SelfT},
		},
	},
}

var (
	IntT     = &PrimitiveType{Name: "int"}
	FloatT   = &PrimitiveType{Name: "float"}
	StrT     = &PrimitiveType{Name: "str"}
	BoolT    = &PrimitiveType{Name: "bool"}
	VoidT    = &PrimitiveType{Name: "void"}
	InvalidT = &PrimitiveType{Name: "invalid"}
	TypeRefT = &PrimitiveType{Name: "typeRef"}
	SelfT    = &TypeParamT{Name: "Self"}
	AnyT     = &PrimitiveType{Name: "any"}
	ErrorT   = &ErrorType{}

	printTp = &TypeParamT{Name: "T", Bound: StringableI}
	PrintT  = &FunctionT{
		Name:       "print",
		Layer:      Builtin,
		TypeParams: []*TypeParamT{printTp},
		Params: []FParam{
			{Name: "value", Type: printTp},
		},
		ReturnType: VoidT,
		RState:     Resolved,
	}

	LenT = &FunctionT{
		Name:       "len",
		Layer:      Builtin,
		ReturnType: IntT,
		RState:     Resolved,
		Params: []FParam{
			{Name: "value", Type: AnyT},
		},
	}
)

type FParam struct {
	Node ast.Node
	Name string
	Type Type
}

type InterfaceT struct {
	Node    *ast.TypeDeclStmt
	Name    string
	Methods map[string]*FunctionT
	State   ResolutionState
	Layer   ProgramLayer
}

func (it *InterfaceT) typ()                      {}
func (it *InterfaceT) GetState() ResolutionState { return it.State }
func (it *InterfaceT) getName() string           { return it.Name }
func (it *InterfaceT) GetMethod(n string) (*FunctionT, error) {
	if m, ok := it.Methods[n]; ok {
		return m, nil
	}
	return nil, fmt.Errorf(
		"interface `%s` has no method `%s",
		it.Name,
		n,
	)
}

var (
	StringableI = &InterfaceT{
		Name:  "Stringable",
		Layer: Builtin,
		State: Resolved,
		Methods: map[string]*FunctionT{
			"string": {Name: "string",
				ReturnType: StrT,
				Layer:      Builtin,
				RState:     Resolved,
				Params: []FParam{
					{Name: "self", Type: SelfT},
				},
			},
		},
	}
)

type TypeParamT struct {
	Name  string
	Bound *InterfaceT
}

func (tpt *TypeParamT) typ()                      {}
func (tpt *TypeParamT) GetState() ResolutionState { return Resolved }
func (tpt *TypeParamT) getName() string           { return tpt.Name }
func (tpt *TypeParamT) GetMethod(n string) (*FunctionT, error) {
	if tpt.Bound == nil {
		return nil, fmt.Errorf("`%s` has no method `%s`", tpt.Name, n)
	}
	return tpt.Bound.GetMethod(n)
}

type ProgramLayer int

const (
	Builtin ProgramLayer = iota
	prelude
	User
)

type FunctionT struct {
	Node       ast.Node
	Name       string
	TypeParams []*TypeParamT
	Params     []FParam
	ReturnType Type
	RState     ResolutionState
	Layer      ProgramLayer
}

func (ft *FunctionT) typ()                      {}
func (ft *FunctionT) GetState() ResolutionState { return ft.RState }
func (ft *FunctionT) getName() string {
	if ft.Name != "" {
		return ft.Name
	}
	var ptypes []string
	for _, p := range ft.Params {
		ptypes = append(ptypes, p.Type.getName())
	}
	return fmt.Sprintf(
		"fn(%s) -> %s",
		strings.Join(ptypes, ", "),
		ft.ReturnType.getName(),
	)
}
func (ft *FunctionT) GetMethod(n string) (*FunctionT, error) {
	return nil, fmt.Errorf("not implemented")
}

type Value interface {
	val()
	GetType() Type
}

var (
	PrintV   = &FunctionV{Name: "print", Type: PrintT}
	LenV     = &FunctionV{Name: "len", Type: LenT}
	InvalidV = &InvalidValue{}
	VoidV    = &VoidValue{}
)

type WithV struct {
	Receiver Value
	Type     Type
}

func (wv *WithV) val()          {}
func (wv *WithV) GetType() Type { return wv.Type }

type ErrorV struct {
	Message string
	Cause   *ErrorV
}

func (ev *ErrorV) val()          {}
func (ev *ErrorV) GetType() Type { return ErrorT }

type TryV struct {
	Node  *ast.TryExpr
	Value Value // must be Result or Maybe
	Err   *CaseBinding
	Else  *Block
	Type  Type
}

func (tv *TryV) val()          {}
func (tv *TryV) GetType() Type { return tv.Type }

type WrapperKind int

const (
	WrapOk WrapperKind = iota
	WrapErr
	WrapSome
	WrapNone
)

type WrapperV struct {
	Node  ast.Expression
	Kind  WrapperKind
	Value Value
	Type  Type
}

func (wv *WrapperV) val()          {}
func (wv *WrapperV) GetType() Type { return wv.Type }

type SliceRange struct {
	Node     *ast.SliceExpr
	StartArg Value
	EndArg   Value
}

func (sv *SliceRange) val()          {}
func (sv *SliceRange) GetType() Type { return VoidT }

type SpreadValue struct {
	Node  *ast.SpreadExpr
	Value Value
}

func (sv *SpreadValue) val() {}
func (sv *SpreadValue) GetType() Type {
	if sv.Value == nil {
		return InvalidT
	}
	return sv.Value.GetType()
}

type VariantConstruction struct {
	Type *VariantT
	Base Value
	Args []NamedArg
}

func (vc *VariantConstruction) val()          {}
func (vc *VariantConstruction) GetType() Type { return vc.Type }

type NamedArg struct {
	Node  *ast.Argument
	Name  string
	Value Value
}

type VariantV struct {
	Name string
	Type *VariantT
}

func (vv *VariantV) val()          {}
func (vv *VariantV) GetType() Type { return vv.Type }

type EnumV struct {
	Type *EnumT
	Name string
}

func (ev *EnumV) val()          {}
func (ev *EnumV) GetType() Type { return ev.Type }

type ZeroV struct {
	Type Type
}

func (zv *ZeroV) val()          {}
func (zv *ZeroV) GetType() Type { return zv.Type }

type CopyV struct {
	Node  *ast.CopyExpr
	Value Value
}

func (cv *CopyV) val()          {}
func (cv *CopyV) GetType() Type { return cv.Value.GetType() }

type StructConstruction struct {
	Node   *ast.CallExpr
	Callee *StructT
	Base   Value
	Args   []NamedArg
}

func (sc *StructConstruction) val()          {}
func (sc *StructConstruction) GetType() Type { return sc.Callee }

type InterfaceV struct {
	Name string
	Type *InterfaceT
}

func (iv *InterfaceV) val()          {}
func (iv *InterfaceV) GetType() Type { return TypeRefT }

type StructV struct {
	Name string
	Type *StructT
}

func (sv *StructV) val()          {}
func (sv *StructV) GetType() Type { return TypeRefT }

type MethodV struct {
	Name     string
	Type     *FunctionT
	Receiver Value
}

func (mv *MethodV) val()          {}
func (mv *MethodV) GetType() Type { return mv.Type }

type FunctionLit struct {
	Node *ast.FunctionExpr
	Body *FunctionBody
}

func (fl *FunctionLit) val()          {}
func (fl *FunctionLit) GetType() Type { return fl.Body.Type }

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
	Node     *ast.CallExpr
	Receiver Value
	Callee   *FunctionT
	CalleeV  Value
	Args     []FunctionCallArgument
	TypeArgs []Type
	Type     Type
}

func (fc *FunctionCall) val()          {}
func (fc *FunctionCall) GetType() Type { return fc.Type }

type IndexAccess struct {
	Node     *ast.SubscriptExpr
	Receiver Value
	Argument Value
	Type     Type
}

func (ia *IndexAccess) val()          {}
func (ia *IndexAccess) GetType() Type { return ia.Type }

type Constant struct {
	Node  *ast.VarStmt
	Name  string
	Type  Type
	Value Value
}

func (c *Constant) val()          {}
func (c *Constant) GetType() Type { return c.Type }

type Variable struct {
	Node         *ast.VarStmt
	Name         string
	DeclaredType Type
	InferredType Type
	ResolvedType Type
	Value        Value
	Owner        *FunctionBody
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

type InvalidValue struct{}

func (iv *InvalidValue) val()          {}
func (iv *InvalidValue) GetType() Type { return InvalidT }

type VoidValue struct{}

func (vv *VoidValue) val()          {}
func (vv *VoidValue) GetType() Type { return VoidT }

type SeqLiteral struct {
	Node   *ast.SequenceLiteral
	Values []Value
	Type   Type
}

func (sl *SeqLiteral) val()          {}
func (sl *SeqLiteral) GetType() Type { return sl.Type }

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

type Block struct {
	Statements []Statement
	Type       Type
}

type CaseBinding struct {
	Node ast.Node
	Name string
	Type Type
	Case Value
}

func (cb *CaseBinding) val()          {}
func (cb *CaseBinding) GetType() Type { return cb.Type }

type SwitchArm struct {
	Node      *ast.SwitchArm
	Condition Value
	Binding   *CaseBinding
	Body      *Block
	IsDefault bool
}

type Switch struct {
	Node    *ast.SwitchExpr
	Subject Value
	Type    Type
	Arms    []SwitchArm
}

func (s *Switch) val()          {}
func (s *Switch) GetType() Type { return s.Type }

type If struct {
	Node      *ast.IfExpr
	Condition Value
	Then      *Block
	Else      *Block
	Type      Type
}

func (i *If) val()          {}
func (i *If) GetType() Type { return i.Type }

type VariableStmt struct {
	Node  *ast.VarStmt
	Value *Variable
}

func (vs *VariableStmt) stmt()             {}
func (vs *VariableStmt) GetNode() ast.Node { return vs.Node }

type FieldAccess struct {
	Node      *ast.MemberExpr
	Container Value
	Type      Type
	Index     int
}

func (fa *FieldAccess) val()          {}
func (fa *FieldAccess) GetType() Type { return fa.Type }

type AssignStmt struct {
	Node   *ast.AssignStmt
	Target Value
	Value  Value
}

func (as *AssignStmt) stmt()             {}
func (as *AssignStmt) GetNode() ast.Node { return as.Node }

type ExpressionStatement struct {
	Node  ast.Expression
	Value Value
}

func (es *ExpressionStatement) stmt()             {}
func (es *ExpressionStatement) GetNode() ast.Node { return es.Node }

type ReturnStatement struct {
	Node  ast.Node
	Value Value
}

func (rs *ReturnStatement) stmt()             {}
func (rs *ReturnStatement) GetNode() ast.Node { return rs.Node }

type YieldStmt struct {
	Node  ast.Node
	Value Value
}

func (ys *YieldStmt) stmt()             {}
func (ys *YieldStmt) GetNode() ast.Node { return ys.Node }

type FunctionBody struct {
	Node      ast.Node
	Type      *FunctionT
	ParamVars []*FunctionParamVar
	Body      *Block
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
		sorted := slices.Clone(a.Errors)
		slices.SortFunc(sorted, func(a, b errors.Error) int {
			aLine, bLine := a.Position.Line, b.Position.Line
			if aLine < bLine {
				return -1
			} else if bLine < aLine {
				return 1
			} else {
				return 0
			}
		})
		fmt.Println("--- Errors ---")
		for _, e := range sorted {
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
		"int":        IntT,
		"float":      FloatT,
		"str":        StrT,
		"bool":       BoolT,
		"void":       VoidT,
		"print":      PrintT,
		"len":        LenT,
		"Stringable": StringableI,
	}
	u.Values = map[string]Value{
		"print": PrintV,
		"len":   LenV,
	}
	return u
}

func (a *Analyzer) collectFunctionType(n *ast.FunctionStmt) {
	ft := &FunctionT{Node: n, Name: n.Signature.Name, RState: Unresolved,
		Layer: User,
	}
	fv := &FunctionV{Name: ft.Name, Type: ft}
	if err := a.CurrentScope.declareType(ft.Name, ft); err != nil {
		a.addError(n, err)
	}
	if err := a.CurrentScope.declareValue(fv.Name, fv); err != nil {
		a.addError(n, err)
	}
}

func (a *Analyzer) collectStructType(st *ast.StructType, stmt *ast.TypeDeclStmt) {
	t := &StructT{Node: stmt, Name: stmt.Name, State: Resolved}
	t.Methods = map[string]*FunctionT{}
	if err := a.CurrentScope.declareType(t.Name, t); err != nil {
		a.addError(stmt, err)
	}
	v := &StructV{Name: t.Name, Type: t}
	if err := a.CurrentScope.declareValue(v.Name, v); err != nil {
		a.addError(stmt, err)
	}
}

func (a *Analyzer) collectEnumT(et *ast.EnumType, stmt *ast.TypeDeclStmt) {
	t := &EnumT{Node: stmt, Name: stmt.Name, State: Resolved}
	t.Methods = map[string]*FunctionT{}
	if err := a.CurrentScope.declareType(t.Name, t); err != nil {
		a.addError(stmt, err)
	}
	v := &EnumV{Name: t.Name, Type: t}
	if err := a.CurrentScope.declareValue(v.Name, v); err != nil {
		a.addError(stmt, err)
	}
}

func (a *Analyzer) collectInterfaceType(def *ast.InterfaceType, stmt *ast.TypeDeclStmt) {
	t := &InterfaceT{Node: stmt, Name: stmt.Name, State: Resolved, Layer: User}
	t.Methods = map[string]*FunctionT{}
	v := &InterfaceV{Name: t.Name, Type: t}
	if err := a.CurrentScope.declareType(t.Name, t); err != nil {
		a.addError(stmt, err)
	}
	if err := a.CurrentScope.declareValue(v.Name, v); err != nil {
		a.addError(stmt, err)
	}
}

func (a *Analyzer) collectTypeDeclStmt(stmt *ast.TypeDeclStmt) {
	switch t := stmt.Definition.(type) {
	case *ast.StructType:
		a.collectStructType(t, stmt)
	case *ast.EnumType:
		a.collectEnumT(t, stmt)
	case *ast.InterfaceType:
		a.collectInterfaceType(t, stmt)
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

func (a *Analyzer) resolveFunctionParam(t *FunctionT, p *ast.Parameter, selfT Type) {
	if p.Name == "self" {
		if selfT == nil {
			a.errorF(p, "`self` is only valid in a method or interface method signature")
			return
		}
		fp := FParam{Node: p, Name: p.Name, Type: selfT}
		t.Params = append(t.Params, fp)
		return
	}

	fp := FParam{Node: p, Name: p.Name}
	typ := a.resolveType(p.Type)
	if typ == VoidT {
		a.errorF(p, "`void` is not a valid parameter type")
		return
	}
	fp.Type = typ
	t.Params = append(t.Params, fp)
}

func (a *Analyzer) resolveTypeParams(
	ft *FunctionT, params []*ast.TypeParameter,
) {
	for _, tp := range params {
		tpt := &TypeParamT{Name: tp.Token.Literal}
		if tp.Bound != nil {
			bound, ok := a.resolveType(tp.Bound).(*InterfaceT)
			if !ok {
				a.errorF(
					tp,
					"the bound of `%s` must be an interface",
					tpt.Name,
				)
			}
			tpt.Bound = bound
		}
		if err := a.CurrentScope.declareType(tpt.Name, tpt); err != nil {
			a.addError(tp, err)
		}
		ft.TypeParams = append(ft.TypeParams, tpt)
	}

}

func (a *Analyzer) resolveFunctionT(t *FunctionT) {
	stmt, ok := t.Node.(*ast.FunctionStmt)
	if !ok {
		return
	}

	a.enterScope("[sig]")
	defer a.exitScope()

	a.resolveTypeParams(t, stmt.Signature.TypeParams)

	rt := a.resolveType(stmt.Signature.ReturnType)
	t.ReturnType = rt
	for _, p := range stmt.Signature.Parameters {
		a.resolveFunctionParam(t, p, nil)
	}
	t.RState = Resolved
}

func (a *Analyzer) resolveFunctionTypeType(node *ast.FunctionType) Type {
	ft := &FunctionT{RState: Resolved, Layer: User}
	for _, p := range node.Parameters {
		pt := a.resolveType(p.Type)
		if pt == VoidT {
			a.errorF(p, "`void` is not a valid parameter type")
			pt = InvalidT
		}
		ft.Params = append(ft.Params, FParam{Node: p, Name: p.Name, Type: pt})
	}
	ft.ReturnType = a.resolveType(node.ReturnType)
	return ft
}

func (a *Analyzer) resolveNamedType(node *ast.NamedType) Type {
	switch node.Name {
	case "Result":
		rt := &ResultT{ElemType: VoidT}
		if len(node.Args) > 0 {
			rt.ElemType = a.resolveType(node.Args[0])
		}
		if len(node.Args) > 1 {
			a.errorF(node, "result can only contain a single value")
			return InvalidT
		}
		return rt
	case "Maybe":
		if len(node.Args) != 1 {
			a.errorF(node, "`Maybe` requires exactly one type arg")
			return InvalidT
		}
		return &MaybeT{ElemType: a.resolveType(node.Args[0])}
	default:
		rt, err := a.CurrentScope.lookupType(node.Name)
		if err != nil {
			a.addError(node, err)
			return InvalidT
		}
		return rt
	}
}

func (a *Analyzer) resolveType(node ast.Type) Type {
	switch t := node.(type) {
	case nil:
		return VoidT
	case *ast.NamedType:
		return a.resolveNamedType(t)
	case *ast.SliceType:
		rt := &SliceT{}
		rt.ElementType = a.resolveType(t.ElementType)
		return rt
	case *ast.FunctionType:
		return a.resolveFunctionTypeType(t)
	case *ast.ResultType:
		rt := &ResultT{}
		rt.ElemType = a.resolveType(t.Value)
		return rt
	case *ast.MaybeType:
		rt := &MaybeT{}
		rt.ElemType = a.resolveType(t.Value)
		return rt
	default:
		return InvalidT
	}
}

func (a *Analyzer) resolveStructT(t *StructT) {
	decl := t.Node.Definition
	stmt, ok := decl.(*ast.StructType)
	if !ok {
		a.errorF(stmt, "Invalid struct definition")
		return
	}
	for _, f := range stmt.Fields {
		field := Field{Node: f, Name: f.Name}
		ft := a.resolveType(f.Type)
		if ft == VoidT {
			a.errorF(f, "`void` is not a valid field type")
			continue
		} else {
			field.Type = ft
		}
		t.Fields = append(t.Fields, field)
	}
}

func (a *Analyzer) resolveEnumT(t *EnumT) {
	node := t.Node.Definition
	def, ok := node.(*ast.EnumType)
	if !ok {
		a.errorF(t.Node, "`%s` is not an enum", t.Node.Name)
	}

	for idx, mem := range def.Members {
		variantT := &VariantT{
			Node: mem, Name: mem.Name, Enum: t,
			Tag: idx, State: Unresolved,
		}
		for _, f := range mem.Fields {
			vfield := Field{Node: f, Name: f.Name}
			ft := a.resolveType(f.Type)
			if ft == VoidT {
				a.errorF(f, "`void` is not a valid field type")
				vfield.Type = InvalidT
			} else {
				vfield.Type = ft
			}
			variantT.Fields = append(variantT.Fields, vfield)
		}
		variantT.State = Resolved
		if err := a.CurrentScope.declareType(variantT.getName(), variantT); err != nil {
			a.addError(mem, err)
		}
		variantv := &VariantV{Name: variantT.getName(), Type: variantT}
		if err := a.CurrentScope.declareValue(variantv.Name, variantv); err != nil {
			a.addError(mem, err)
		}
		t.Variants = append(t.Variants, variantT)
	}
}

func (a *Analyzer) resolveInterfaceT(t *InterfaceT) {
	stmt, ok := t.Node.Definition.(*ast.InterfaceType)
	if !ok {
		a.errorF(t.Node.Definition, "Expected interface definition")
		return
	}

	for _, meth := range stmt.Methods {
		if len(meth.TypeParams) > 0 {
			a.errorF(meth, "interface methods cannot be generic")
			continue
		}
		if len(meth.Parameters) < 1 {
			a.errorF(meth, "all methods must take at least a `self` argument")
			continue
		}

		a.enterScope("[iface]")
		a.CurrentScope.declareType("Self", SelfT)

		fname := fmt.Sprintf("%s.%s", t.Name, meth.Name)
		ft := &FunctionT{
			Node: meth, Name: fname, RState: Resolved,
			Layer: User,
		}
		ft.ReturnType = a.resolveType(meth.ReturnType)

		for _, p := range meth.Parameters {
			a.resolveFunctionParam(ft, p, SelfT)
		}
		a.exitScope()

		t.Methods[meth.Name] = ft
	}
}

func (a *Analyzer) pass2() {
	for _, t := range a.Package.Types {
		switch t := t.(type) {
		case *FunctionT:
			a.resolveFunctionT(t)
		case *StructT:
			a.resolveStructT(t)
		case *InterfaceT:
			a.resolveInterfaceT(t)
		case *EnumT:
			a.resolveEnumT(t)
		}
	}
}

func (a *Analyzer) requireValue(node ast.Node, v Value) bool {
	if v.GetType() == VoidT {
		a.errorF(node, "expression has no value (type `void`) and cannot be used here")
		return false
	}
	return true
}

func (a *Analyzer) checkReturn(
	fb *FunctionBody, node ast.Node, v Value,
) *ReturnStatement {
	switch {
	case fb.Type.ReturnType != VoidT && v == nil:
		a.errorF(node,
			"function `%s` must return `%s`", fb.Type.Name, fb.Type.ReturnType.getName())
	case v != nil:
		if err := identical(v.GetType(), fb.Type.ReturnType); err != nil {
			a.addError(node, err)
		}
	}
	return &ReturnStatement{Node: node, Value: v}
}

func (a *Analyzer) resolveReturnStmt(
	fb *FunctionBody, block *Block, stmt *ast.ReturnStmt,
) {
	var v Value
	if stmt.Value != nil {
		v = a.resolveExpr(stmt.Value, fb, fb.Type.ReturnType)
	} else {
		v = nil
	}
	rs := a.checkReturn(fb, stmt, v)
	block.Statements = append(block.Statements, rs)
}

func (a *Analyzer) resolveAssignStmt(
	fb *FunctionBody, block *Block, stmt *ast.AssignStmt,
) {
	as := &AssignStmt{Node: stmt}
	target := a.resolveExpr(stmt.Target, fb, nil)
	switch t := target.(type) {
	case *Variable:
		as.Target = t
	case *CaseBinding:
		a.errorF(stmt.Target, "cannot rebind case binding")
		return
	default:
		a.errorF(stmt.Target, "invalid assign target: `%T`", target)
		return
	}

	val := a.resolveExpr(stmt.Value, fb, as.Target.GetType())
	if !a.requireValue(stmt.Value, val) {
		as.Value = InvalidV
	} else {
		as.Value = val
	}

	if as.Value.GetType() == InvalidT || as.Target.GetType() == InvalidT {
		return
	}
	if err := identical(as.Target.GetType(), as.Value.GetType()); err != nil {
		a.addError(stmt.Value, err)
		return
	}
	block.Statements = append(block.Statements, as)
}

func (a *Analyzer) resolveIf(
	expr *ast.IfExpr, fb *FunctionBody, asValue bool,
) *If {
	ifV := &If{Node: expr, Type: VoidT}

	ifV.Condition = a.resolveExpr(expr.Condition, fb, nil)
	if t := ifV.Condition.GetType(); t != InvalidT {
		if err := identical(t, BoolT); err != nil {
			a.addError(expr.Condition, err)
		}
	}

	if then, ok := expr.Then.(*ast.BlockExpr); ok {
		ifV.Then = a.resolveBlock(fb, then)
	} else {
		a.errorF(expr.Then, "then branch of if expression must be a block")
		ifV.Then = &Block{Type: VoidT}
	}

	ifV.Else = a.resolveBranch(fb, expr.Else, asValue, false)

	if asValue {
		ifV.Type = a.getIfType(ifV)
	}

	return ifV
}

func (a *Analyzer) getIfType(i *If) Type {
	tt, et := i.Then.Type, i.Else.Type
	switch {
	case tt == InvalidT || et == InvalidT:
		return InvalidT
	case terminates(i.Then):
		return et
	case terminates(i.Else):
		return tt
	}
	if err := identical(tt, et); err != nil {
		a.errorF(i.Node,
			"if branches have differing types `%s` != `%s`",
			tt.getName(), et.getName())
	}
	return tt
}

func (a *Analyzer) resolveBranch(
	fb *FunctionBody, expr ast.Expression, asValue bool,
	noScope bool,
) *Block {
	switch t := expr.(type) {
	case *ast.BlockExpr:
		if noScope {
			return a.resolveBlockIn(fb, t)
		} else {
			return a.resolveBlock(fb, t)
		}
	case *ast.IfExpr:
		return yieldBlock(t, a.resolveIf(t, fb, asValue))
	case *ast.SwitchExpr:
		return yieldBlock(t, a.resolveSwitch(t, fb, asValue))
	default:
		return yieldBlock(t, a.resolveExpr(t, fb, nil))
	}
}

func (a *Analyzer) getSwitchType(sw *Switch) Type {
	var want Type
	for _, arm := range sw.Arms {
		if terminates(arm.Body) {
			continue
		}
		t := arm.Body.Type
		if t == InvalidT {
			return InvalidT
		}
		if want == nil {
			want = t
		} else if err := identical(t, want); err != nil {
			a.addError(arm.Node, err)
			return InvalidT
		}
	}

	if want == nil {
		return VoidT
	}

	return want
}

func (a *Analyzer) resolveSwitchArm(
	sw *Switch, node *ast.SwitchArm, fb *FunctionBody,
	asValue bool,
) SwitchArm {
	arm := SwitchArm{Node: node}
	if id, ok := node.Case.(*ast.IdentifierExpr); ok && id.Value == "_" {
		arm.IsDefault = true
	} else {
		arm.Condition = a.resolveExpr(node.Case, fb, nil)
	}

	switch {
	case arm.IsDefault && node.Binding != nil && sw.Subject == nil:
		a.errorF(node.Binding, "invalid `as`; nothing to bind to")
		arm.Binding = &CaseBinding{
			Node: node.Binding,
			Name: node.Binding.Value,
			Type: InvalidT,
			Case: InvalidV,
		}
	case arm.IsDefault && node.Binding != nil:
		arm.Binding = &CaseBinding{
			Node: node.Binding,
			Name: node.Binding.Value,
			Type: sw.Subject.GetType(),
			Case: sw.Subject,
		}
	case node.Binding != nil:
		arm.Binding = &CaseBinding{
			Node: node.Binding,
			Name: node.Binding.Value,
			Type: arm.Condition.GetType(),
			Case: arm.Condition,
		}
	}

	a.enterScope("arm")
	defer a.exitScope()

	if arm.Binding != nil {
		if err := a.CurrentScope.declareValue(arm.Binding.Name, arm.Binding); err != nil {
			a.addError(node.Binding, err)
		}
	}

	arm.Body = a.resolveBranch(fb, node.Value, asValue, false)

	return arm
}

func (a *Analyzer) resolveSwitch(
	expr *ast.SwitchExpr, fb *FunctionBody, asValue bool,
) *Switch {
	sw := &Switch{Node: expr, Type: VoidT}
	if expr.Subject != nil {
		sw.Subject = a.resolveExpr(expr.Subject, fb, nil)
	}

	for idx, arm := range expr.Arms {
		armv := a.resolveSwitchArm(sw, arm, fb, asValue)
		if armv.IsDefault && idx < len(expr.Arms)-1 {
			a.errorF(arm, "`_` default arm must be at end of switch")
		}
		sw.Arms = append(sw.Arms, armv)
	}

	if asValue {
		sw.Type = a.getSwitchType(sw)
	}

	return sw
}

func (a *Analyzer) resolveExprStmt(
	fb *FunctionBody, block *Block, stmt *ast.ExpressionStatement,
) {
	switch t := stmt.Expression.(type) {
	case *ast.IfExpr:
		ifv := a.resolveIf(t, fb, false)
		block.Statements = append(block.Statements,
			&ExpressionStatement{Node: t, Value: ifv})
	case *ast.SwitchExpr:
		sv := a.resolveSwitch(t, fb, false)
		block.Statements = append(block.Statements,
			&ExpressionStatement{Node: t, Value: sv})
	default:
		v := a.resolveExpr(t, fb, nil)
		es := &ExpressionStatement{Node: stmt.Expression, Value: v}
		block.Statements = append(block.Statements, es)
	}
}

func exitsBlock(s Statement) bool {
	if _, ok := s.(*YieldStmt); ok {
		return true
	}
	return stmtTerminates(s)
}

func (a *Analyzer) resolveStmts(
	fb *FunctionBody, blk *Block, stmts []ast.Statement,
) {
	for i, s := range stmts {
		n := len(blk.Statements)
		a.resolveFunctionBodyStmt(fb, blk, s)
		if len(blk.Statements) > n && exitsBlock(blk.Statements[len(blk.Statements)-1]) {
			if i+1 < len(stmts) {
				a.errorF(stmts[i+1], "unreachable code")
			}
			return
		}
	}
}

func (a *Analyzer) resolveYieldStmt(
	fb *FunctionBody, block *Block, stmt *ast.EmitStmt,
) {
	v := a.resolveExpr(stmt.Value, fb, nil)
	block.Statements = append(block.Statements, &YieldStmt{Node: stmt.Value, Value: v})
	block.Type = v.GetType()
}

func (a *Analyzer) resolveFunctionBodyStmt(
	fb *FunctionBody, block *Block, stmt ast.Statement,
) {
	switch t := stmt.(type) {
	case *ast.ReturnStmt:
		a.resolveReturnStmt(fb, block, t)
	case *ast.VarStmt:
		a.resolveVarStmt(fb, block, t)
	case *ast.AssignStmt:
		a.resolveAssignStmt(fb, block, t)
	case *ast.ExpressionStatement:
		a.resolveExprStmt(fb, block, t)
	case *ast.EmitStmt:
		if block == fb.Body {
			a.errorF(t,
				"`>>` cannot be used in the top-level function scope; use `return` instead",
			)
		} else {
			a.resolveYieldStmt(fb, block, t)
		}
	default:
		a.errorF(stmt, "unknown statement `%T`", stmt)
	}
}

func (a *Analyzer) resolveFunctionBodyExpr(fb *FunctionBody, expr ast.Expression) {
	v := a.resolveExpr(expr, fb, nil)
	rs := a.checkReturn(fb, expr, v)
	fb.Body.Statements = append(fb.Body.Statements, rs)
}

func (a *Analyzer) resolveBody(
	ft *FunctionT, body ast.Expression,
) *FunctionBody {
	a.enterScope(ft.Name)
	defer a.exitScope()

	for _, tp := range ft.TypeParams {
		a.CurrentScope.declareType(tp.Name, tp)
	}

	fb := &FunctionBody{Node: body, Type: ft, Body: &Block{Type: VoidT}}

	for idx, p := range ft.Params {
		pv := &FunctionParamVar{Node: p.Node, Name: p.Name, Index: idx, Type: p.Type}
		fb.ParamVars = append(fb.ParamVars, pv)
		if err := a.CurrentScope.declareValue(pv.Name, pv); err != nil {
			a.addError(p.Node, err)
		}
	}

	switch t := body.(type) {
	case *ast.BlockExpr:
		a.resolveStmts(fb, fb.Body, t.Statements)
		if ft.ReturnType != VoidT && !terminates(fb.Body) {
			a.errorF(body, "function `%s` must return `%s` on every path",
				ft.getName(), ft.ReturnType.getName())
		}
	default:
		a.resolveFunctionBodyExpr(fb, t)
	}

	return fb
}

func (a *Analyzer) resolveFunctionBody(t *FunctionT) {
	stmt, ok := t.Node.(*ast.FunctionStmt)
	if !ok {
		panic(
			fmt.Sprintf(
				"internal error: `%s` in package scope has no FunctionStmt", t.Name,
			),
		)
	}

	fb := a.resolveBody(t, stmt.Body)
	a.storeFunctionBody("", fb)
}

func terminates(b *Block) bool {
	return slices.ContainsFunc(b.Statements, stmtTerminates)
}

func valueTerminates(v Value) bool {
	switch t := v.(type) {
	case *If:
		return t.Else != nil && terminates(t.Then) && terminates(t.Else)
	case *Switch:
		if len(t.Arms) < 1 {
			return false
		}
		for _, a := range t.Arms {
			if a.IsDefault {
				return true
			}
			if !terminates(a.Body) {
				return false
			}
		}
	}
	return false
}

func stmtTerminates(stmt Statement) bool {
	switch t := stmt.(type) {
	case *ReturnStatement:
		return true
	case *ExpressionStatement:
		return valueTerminates(t.Value)
	case *YieldStmt:
		return valueTerminates(t.Value)
	case *VariableStmt:
		return valueTerminates(t.Value.Value)
	}
	return false
}

func (a *Analyzer) pass4() {
	for _, t := range a.Package.Types {
		switch t := t.(type) {
		case *FunctionT:
			a.resolveFunctionBody(t)
		}
	}
}

func (a *Analyzer) resolveVarStmt(
	fb *FunctionBody, block *Block, stmt *ast.VarStmt,
) {
	v := &Variable{Node: stmt, Name: stmt.Name.Literal}
	if stmt.Type != nil {
		v.DeclaredType = a.resolveType(stmt.Type)
	}
	val := a.resolveExpr(stmt.Value, fb, v.DeclaredType)
	if val == InvalidV || !a.requireValue(stmt.Value, val) {
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
		} else {
			v.ResolvedType = v.DeclaredType
		}
	case v.DeclaredType == nil:
		v.ResolvedType = v.InferredType
	}
	if err := a.CurrentScope.declareValue(v.Name, v); err != nil {
		a.addError(stmt, err)
	}
	if fb != nil && block != nil {
		v.Owner = fb
		vs := &VariableStmt{Node: stmt, Value: v}
		block.Statements = append(block.Statements, vs)
	}
}

func (a *Analyzer) checkConstExpr(expr ast.Expression) bool {
	switch t := expr.(type) {
	case *ast.IntegerLiteralExpr, *ast.FloatLiteralExpr,
		*ast.StringLiteralExpr, *ast.BoolLiteralExpr, *ast.IdentifierExpr:
		return true
	case *ast.NegativeExpr:
		return a.checkConstExpr(t.Value)
	case *ast.NotExpr:
		return a.checkConstExpr(t.Value)
	case *ast.InfixExpr:
		return a.checkConstExpr(t.Left) && a.checkConstExpr(t.Right)
	case *ast.SequenceLiteral:
		for _, el := range t.Elements {
			if !a.checkConstExpr(el) {
				return false
			}
		}
		return true
	default:
		a.errorF(expr, "only primitives, containers of primtives and operators are allowed in package-level contants")
		return false
	}
}

func foldConst(v Value) (Value, error) {
	switch t := v.(type) {
	case *IntLiteral, *StrLiteral, *FloatLiteral, *BoolLiteral:
		return t, nil
	case *Constant:
		return t.Value, nil
	case *BinaryValue:
		l, err := foldConst(t.Left)
		if err != nil {
			return nil, err
		}
		r, err := foldConst(t.Right)
		if err != nil {
			return nil, err
		}
		return foldBinaryValue(t.Op, l, r)
	case *SeqLiteral:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported const value")
	}
}

func (a *Analyzer) resolveConstant(stmt *ast.VarStmt) {
	if !a.checkConstExpr(stmt.Value) {
		return
	}
	c := &Constant{Node: stmt, Name: stmt.Name.Literal}
	var declaredType Type
	if stmt.Type != nil {
		declaredType = a.resolveType(stmt.Type)
		if declaredType == VoidT {
			a.errorF(stmt.Type, "`void` is not a valid type here")
			declaredType = InvalidT
		}
	}
	val := a.resolveExpr(stmt.Value, nil, declaredType)
	if val == InvalidV {
		return
	}
	rv, err := foldConst(val)
	if err != nil {
		a.addError(stmt, err)
		c.Value = InvalidV
		c.Type = InvalidT
	} else {
		c.Value, c.Type = rv, rv.GetType()
	}

	if declaredType != nil && declaredType != InvalidT {
		if err := identical(declaredType, c.Type); err != nil {
			a.addError(stmt, err)
			c.Type = InvalidT
		}
	}

	if err := a.CurrentScope.declareValue(c.Name, c); err != nil {
		a.addError(stmt, err)
	}
}

func (a *Analyzer) resolveImplDecl(node *ast.ImplDeclStmt) {
	targetT := a.resolveType(node.Target)

	for _, meth := range node.Methods {
		if meth.Signature.Name == "with" {
			a.errorF(meth, "`with` is a builtin method and cannot be shadowed")
			continue
		}
		fn := fmt.Sprintf("%s.%s", targetT.getName(), meth.Signature.Name)
		ft := &FunctionT{
			Node: meth, Name: fn, RState: Resolved,
			Layer: User,
		}

		a.enterScope("[sig]")
		a.CurrentScope.declareType("Self", targetT)

		a.resolveTypeParams(ft, meth.Signature.TypeParams)

		ft.ReturnType = a.resolveType(meth.Signature.ReturnType)

		if len(meth.Signature.Parameters) < 1 {
			a.errorF(meth, "all methods must take argument `self`")
			a.exitScope()
			continue
		}
		for _, p := range meth.Signature.Parameters {
			a.resolveFunctionParam(ft, p, targetT)
		}
		if ft.Params[0].Name != "self" {
			a.errorF(meth.Signature.Parameters[0],
				"all methods must take `self` as the first argument",
			)
		}

		a.exitScope()

		if err := a.CurrentScope.declareType(ft.Name, ft); err != nil {
			a.addError(meth, err)
		}
		switch t := targetT.(type) {
		case *StructT:
			t.Methods[meth.Signature.Name] = ft
		case *EnumT:
			t.Methods[meth.Signature.Name] = ft
		}
	}
}

func (a *Analyzer) pass3(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		switch t := stmt.(type) {
		case *ast.VarStmt:
			a.resolveConstant(t)
		case *ast.ImplDeclStmt:
			a.resolveImplDecl(t)
		}
	}
}

func (a *Analyzer) Run(prog *ast.Program) {
	a.pass1(prog) // collect top-level function and type names
	a.pass2()     // resolve function and method types
	a.pass3(prog) // resolve top-level vars and impl blocks
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

func (a *Analyzer) resolveIdentifierExpr(
	expr *ast.IdentifierExpr, fb *FunctionBody,
) Value {
	if expr.Value == "_" {
		a.errorF(expr, "`_` is not a value")
		return InvalidV
	}
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
	case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLTE, OpAnd, OpOr:
		return BoolT, nil
	}
	return InvalidT, fmt.Errorf("invalid operator")
}

func (a *Analyzer) resolveInfixExpr(expr *ast.InfixExpr, fb *FunctionBody) Value {
	bv := &BinaryValue{Node: expr}
	bv.Left = a.resolveExpr(expr.Left, fb, nil)
	bv.Right = a.resolveExpr(expr.Right, fb, nil)
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

func (a *Analyzer) resolveFunctionCallArgs(
	ft *FunctionT, expr *ast.CallExpr, fb *FunctionBody,
	isMethod bool, m map[*TypeParamT]Type,
) ([]FunctionCallArgument, bool) {
	startAt := 0
	if isMethod {
		startAt = 1
	}

	want := len(ft.Params) - startAt
	if len(expr.Args) > want {
		a.errorF(expr,
			"`%s` expects %d args but %d were passed",
			ft.Name,
			want,
			len(expr.Args),
		)
		return nil, false
	}

	var args []FunctionCallArgument
	for idx, p := range ft.Params[startAt:] {
		var arg *ast.Argument
		if p.Name != "" {
			_, arg, _ = getArgByName(expr.Args, p.Name)
		}
		if arg == nil {
			if idx >= len(expr.Args) {
				a.errorF(expr, "`%s` expects %d args but %d were passed",
					ft.Name, want, len(expr.Args),
				)
				return nil, false
			}
			arg = expr.Args[idx]
			if idx > 0 && expr.Args[idx-1].Name != "" {
				a.errorF(arg, "positional arguments cannot come after named arguments")
				return nil, false
			}
		}

		target := p.Type
		if m != nil {
			target = subGeneric(p.Type, m)
			if hasFreeTParams(target, ft.TypeParams, m) {
				target = nil
			}
		}

		argV := a.resolveExpr(arg.Value, fb, target)
		if err := requireValidValue(argV); err != nil {
			a.addError(arg, err)
			return nil, false
		}

		var err error
		if m != nil {
			err = unifyGeneric(p.Type, argV.GetType(), m)
		} else {
			err = identical(p.Type, argV.GetType())
		}
		if err != nil {
			a.addError(arg, err)
			return nil, false
		}

		args = append(args, FunctionCallArgument{Node: arg, Value: argV})
	}
	return args, true
}

func (a *Analyzer) resolveGenericCall(
	ft *FunctionT, expr *ast.CallExpr, fb *FunctionBody,
	recv Value,
) Value {
	m := map[*TypeParamT]Type{}

	args, ok := a.resolveFunctionCallArgs(
		ft, expr, fb, recv != nil, m,
	)
	if !ok {
		return InvalidV
	}

	fc := &FunctionCall{Node: expr, Callee: ft, Args: args, Receiver: recv}
	for _, tp := range ft.TypeParams {
		t, known := m[tp]
		if !known {
			a.errorF(expr,
				"cannot infer type paramater `%s` of `%s`",
				tp.Name, ft.Name,
			)
			return InvalidV
		}
		if tp.Bound != nil {
			if err := satisfies(t, tp.Bound); err != nil {
				a.addError(expr, err)
				return InvalidV
			}
		}
		fc.TypeArgs = append(fc.TypeArgs, t)
	}
	fc.Type = subGeneric(ft.ReturnType, m)
	return fc
}

func (a *Analyzer) resolveMethodCall(
	m *MethodV, expr *ast.CallExpr, fb *FunctionBody,
) Value {
	if len(m.Type.TypeParams) > 0 {
		return a.resolveGenericCall(m.Type, expr, fb, m.Receiver)
	}
	fc := &FunctionCall{
		Node:     expr,
		Callee:   m.Type,
		Type:     m.Type.ReturnType,
		Receiver: m.Receiver,
	}
	args, ok := a.resolveFunctionCallArgs(m.Type, expr, fb, true, nil)
	if !ok {
		return InvalidV
	}
	fc.Args = args
	return fc
}

func (a *Analyzer) resolveFunctionCall(
	ft *FunctionT, expr *ast.CallExpr, fb *FunctionBody,
) Value {
	if len(ft.TypeParams) > 0 {
		return a.resolveGenericCall(ft, expr, fb, nil)
	}
	fc := &FunctionCall{Node: expr, Callee: ft, Type: ft.ReturnType}
	args, ok := a.resolveFunctionCallArgs(
		ft, expr, fb, false, nil,
	)
	if !ok {
		return InvalidV
	}
	fc.Args = args
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

func (a *Analyzer) resolveStructConstruction(
	st *StructT, expr *ast.CallExpr, fb *FunctionBody, base Value,
) Value {
	sc := &StructConstruction{Node: expr, Callee: st, Base: base}
	if len(expr.Args) > len(st.Fields) {
		a.errorF(expr, "too many arguments to constructor for struct `%s`", st.Name)
		return InvalidV
	}

	for _, arg := range expr.Args {
		if arg.Name == "" {
			a.errorF(arg, "positional args not allowed in struct construction")
			return InvalidV
		}
		if _, _, err := st.GetField(arg.Name); err != nil {
			a.addError(arg, err)
			return InvalidV
		}
	}

	for _, f := range st.Fields {
		_, arg, err := getArgByName(expr.Args, f.Name)
		if err != nil {
			if base != nil {
				continue
			}
			if hasZeroValue(f.Type) {
				sc.Args = append(sc.Args,
					NamedArg{Node: arg, Name: f.Name, Value: &ZeroV{Type: f.Type}})
				continue
			}
			a.addError(expr, err)
			return InvalidV
		}
		argV := a.resolveExpr(arg.Value, fb, f.Type)
		if !a.requireValue(arg, argV) {
			return InvalidV
		}
		if argV == InvalidV {
			a.errorF(arg, "unknown expression `%T`", arg.Value)
			return InvalidV
		}
		if err := identical(f.Type, argV.GetType()); err != nil {
			a.addError(arg, err)
			return InvalidV
		}
		scArg := NamedArg{Node: arg, Name: f.Name, Value: argV}
		sc.Args = append(sc.Args, scArg)
	}
	return sc
}

func (a *Analyzer) resolveVariantConstruction(
	vt *VariantT, expr *ast.CallExpr, fb *FunctionBody, base Value,
) Value {
	var args []NamedArg
	for _, f := range vt.Fields {
		_, arg, err := getArgByName(expr.Args, f.Name)
		if err != nil {
			if base != nil {
				continue
			}
			a.addError(expr, err)
			args = append(args, NamedArg{Name: f.Name, Value: InvalidV})
			continue
		}
		argValue := a.resolveExpr(arg.Value, fb, f.Type)
		if !a.requireValue(arg, argValue) {
			return InvalidV
		}
		args = append(args, NamedArg{Name: f.Name, Value: argValue})
	}
	return &VariantConstruction{Type: vt, Args: args}
}

func (a *Analyzer) resolveCallExpr(expr *ast.CallExpr, fb *FunctionBody) Value {
	leftVal := a.resolveExpr(expr.Left, fb, nil)
	switch t := leftVal.(type) {
	case *StructV:
		return a.resolveStructConstruction(t.Type, expr, fb, nil)
	case *WithV:
		switch t2 := t.Type.(type) {
		case *StructT:
			return a.resolveStructConstruction(t2, expr, fb, t.Receiver)
		case *VariantT:
			return a.resolveVariantConstruction(t2, expr, fb, t.Receiver)
		}
	case *MethodV:
		return a.resolveMethodCall(t, expr, fb)
	case *VariantV:
		return a.resolveVariantConstruction(t.Type, expr, fb, nil)
	case *FunctionV:
		return a.resolveFunctionCall(t.Type, expr, fb)
	}

	if leftVal.GetType() == InvalidT {
		return InvalidV
	}

	ft, ok := leftVal.GetType().(*FunctionT)
	if !ok {
		a.errorF(expr.Left, "`%s` is not callable", leftVal.GetType().getName())
		return InvalidV
	}
	args, ok := a.resolveFunctionCallArgs(ft, expr, fb, false, nil)
	if !ok {
		return InvalidV
	}
	return &FunctionCall{
		Node:    expr,
		Callee:  ft,
		CalleeV: leftVal,
		Args:    args,
		Type:    ft.ReturnType,
	}
}

func compare[T cmp.Ordered](op Operator, a, b T) (result, ok bool) {
	switch op {
	case OpEq:
		return a == b, true
	case OpNeq:
		return a != b, true
	case OpLt:
		return a < b, true
	case OpLTE:
		return a <= b, true
	case OpGt:
		return a > b, true
	case OpGte:
		return a >= b, true
	}
	return false, false
}

func foldInt(op Operator, a, b int) (Value, error) {
	if c, ok := compare(op, a, b); ok {
		return &BoolLiteral{Value: c}, nil
	}
	ofErr := fmt.Errorf("integer overflow")
	ufErr := fmt.Errorf("integer undeflow")

	switch op {
	case OpAdd:
		v, of := AddSafe(a, b)
		if of {
			return InvalidV, ofErr
		}
		return &IntLiteral{Value: v}, nil
	case OpSub:
		v, uf := SubSafe(a, b)
		if uf {
			return InvalidV, ufErr
		}
		return &IntLiteral{Value: v}, nil
	case OpMul:
		v, of := MulSafe(a, b)
		if of {
			return InvalidV, ofErr
		}
		return &IntLiteral{Value: v}, nil
	case OpDiv:
		v, uhoh := DivSafe(a, b)
		if uhoh {
			return InvalidV, fmt.Errorf("overflow or division by 0")
		}
		return &IntLiteral{Value: v}, nil
	case OpMod:
		v, uhoh := ModSafe(a, b)
		if uhoh {
			return InvalidV, fmt.Errorf("overflow or division by 0")
		}
		return &IntLiteral{Value: v}, nil
	}
	panic("unreachable")
}

func foldStr(op Operator, a, b string) (Value, error) {
	if c, ok := compare(op, a, b); ok {
		return &BoolLiteral{Value: c}, nil
	}
	switch op {
	case OpAdd:
		return &StrLiteral{Value: a + b}, nil
	}
	panic("invalid operand for string in binary expression")
}

func foldFloat(op Operator, a, b float64) (Value, error) {
	if c, ok := compare(op, a, b); ok {
		return &BoolLiteral{Value: c}, nil
	}
	switch op {
	case OpAdd:
		return &FloatLiteral{Value: a + b}, nil
	case OpSub:
		return &FloatLiteral{Value: a - b}, nil
	case OpMul:
		return &FloatLiteral{Value: a * b}, nil
	case OpDiv:
		if b == 0 {
			return nil, fmt.Errorf("division by 0")
		}
		return &FloatLiteral{Value: a / b}, nil
	case OpMod:
		if b == 0 {
			return nil, fmt.Errorf("moduleo op by 0")
		}
		return &FloatLiteral{Value: math.Mod(a, b)}, nil
	}
	panic("unsuppoorted operation on float type")
}

func foldBool(op Operator, l, r bool) (Value, error) {
	switch op {
	case OpEq:
		return &BoolLiteral{Value: l == r}, nil
	case OpNeq:
		return &BoolLiteral{Value: l != r}, nil
	case OpAnd:
		return &BoolLiteral{Value: l && r}, nil
	case OpOr:
		return &BoolLiteral{Value: l || r}, nil
	}
	return nil, fmt.Errorf("`%v` not valid for bool", op)
}

func foldBinaryValue(op Operator, l, r Value) (Value, error) {
	switch l := l.(type) {
	case *IntLiteral:
		return foldInt(op, l.Value, r.(*IntLiteral).Value)
	case *StrLiteral:
		return foldStr(op, l.Value, r.(*StrLiteral).Value)
	case *FloatLiteral:
		return foldFloat(op, l.Value, r.(*FloatLiteral).Value)
	case *BoolLiteral:
		return foldBool(op, l.Value, r.(*BoolLiteral).Value)
	}
	panic("unreachable: operands were type-checked")
}

func (a *Analyzer) resolveCopyExpr(expr *ast.CopyExpr, fb *FunctionBody) Value {
	cv := &CopyV{Node: expr}
	cv.Value = a.resolveExpr(expr.Value, fb, nil)
	return cv
}

func (a *Analyzer) resolveMemberExpr(expr *ast.MemberExpr, fb *FunctionBody) Value {
	container := a.resolveExpr(expr.Container, fb, nil)
	switch t := container.(type) {
	case *CaseBinding:
		switch c := t.Case.(type) {
		case *VariantV:
			variant := c.Type
			idx, field, err := variant.GetField(expr.Member)
			if err != nil {
				a.addError(expr.Container, err)
				return InvalidV
			} else {
				return &FieldAccess{Container: t, Index: idx, Type: field.Type}
			}
		default:
			a.errorF(expr.Container, "`%s` value has no fields", t.GetType().getName())
			return InvalidV
		}
	case *EnumV:
		enumT := t.Type
		_, variant, err := enumT.GetVariant(expr.Member)
		if err != nil {
			a.addError(expr, err)
			return InvalidV
		}
		if val, err := a.CurrentScope.lookupValue(variant.getName()); err != nil {
			a.addError(expr, err)
		} else {
			return val
		}
	}
	containerT := container.GetType()
	if containerT == InvalidT {
		return InvalidV
	}

	switch t := containerT.(type) {
	case *SliceT:
		meth, err := t.GetMethod(expr.Member)
		if err != nil {
			a.addError(expr, err)
			return InvalidV
		}
		return &MethodV{Name: meth.Name, Type: meth, Receiver: container}
	case *StructT:
		if idx, field, err := t.GetField(expr.Member); err == nil {
			return &FieldAccess{
				Node: expr, Container: container,
				Index: idx, Type: field.Type,
			}
		}
		// handle struct.with(...)
		if expr.Member == "with" {
			return &WithV{Receiver: container, Type: t}
		}
		if m, err := t.GetMethod(expr.Member); err == nil {
			return &MethodV{Name: m.Name, Type: m, Receiver: container}
		}
		a.errorF(expr, "`%s` has no field or method `%s`",
			t.getName(), expr.Member,
		)
		return InvalidV
	case *EnumT:
		a.errorF(expr.Container, "`%s` has no fields", t.Name)
		return InvalidV
	case *VariantT:
		if expr.Member == "with" {
			return &WithV{Receiver: container, Type: t}
		} else {
			return InvalidV
		}
	case *TypeParamT:
		meth, err := t.GetMethod(expr.Member)
		if err != nil {
			a.addError(expr, err)
			return InvalidV
		}
		meth = subGeneric(meth, map[*TypeParamT]Type{SelfT: t}).(*FunctionT)
		return &MethodV{Name: meth.Name, Type: meth, Receiver: container}
	default:
		a.errorF(expr.Container, "container value %T not currently supported", t)
		return InvalidV
	}
}

func (a *Analyzer) resolveBlockIn(
	fb *FunctionBody, expr *ast.BlockExpr,
) *Block {
	blk := &Block{Type: VoidT}
	a.resolveStmts(fb, blk, expr.Statements)
	return blk
}

func (a *Analyzer) resolveBlock(fb *FunctionBody, expr *ast.BlockExpr) *Block {
	a.enterScope("block")
	defer a.exitScope()

	return a.resolveBlockIn(fb, expr)
}

func yieldBlock(node ast.Expression, v Value) *Block {
	return &Block{
		Statements: []Statement{&YieldStmt{Node: node, Value: v}},
		Type:       v.GetType(),
	}
}

func (a *Analyzer) resolveSequenceLiteral(
	s *ast.SequenceLiteral, fb *FunctionBody, targetT Type,
) Value {
	var want Type
	st, ok := targetT.(*SliceT)
	if ok {
		want = st.ElementType
	}

	sl := &SeqLiteral{Node: s}
	if len(s.Elements) < 1 && st != nil {
		sl.Type = st
		return sl
	}

	for _, el := range s.Elements {
		var elval Value
		switch t := el.(type) {
		case *ast.SpreadExpr:
			elval = a.resolveSpreadExpr(t, fb)
		default:
			elval = a.resolveExpr(el, fb, want)
		}
		if err := requireValidValue(elval); err != nil {
			a.addError(el, err)
			return InvalidV
		}

		elt := elval.GetType()
		if sv, ok := elval.(*SpreadValue); ok {
			elt = sv.GetType().(*SliceT).ElementType
		}

		if want == nil {
			want = elt
		} else if err := identical(want, elt); err != nil {
			a.addError(el, err)
			return InvalidV
		}
		sl.Values = append(sl.Values, elval)
	}

	if want == nil {
		a.errorF(s, "unable to infer sequence literal type")
		return InvalidV
	} else {
		sl.Type = &SliceT{ElementType: want}
	}

	return sl
}

func (a *Analyzer) resolveSliceExpr(
	se *ast.SliceExpr, fb *FunctionBody,
) (Value, Value) {
	var start, end Value
	if se.Start != nil {
		startV := a.resolveExpr(se.Start, fb, nil)
		if err := requireValidValue(startV); err != nil {
			a.addError(se.Start, err)
			return InvalidV, InvalidV
		} else {
			start = startV
		}
	} else {
		start = &IntLiteral{Value: 0}
	}

	if se.End != nil {
		endV := a.resolveExpr(se.End, fb, nil)
		if err := requireValidValue(endV); err != nil {
			a.addError(se.End, err)
		} else {
			end = endV
		}
	}
	return start, end
}

func requireValidValue(v Value) error {
	switch {
	case v == InvalidV:
		return fmt.Errorf("`invalid` used as value")
	case v.GetType() == InvalidT:
		return fmt.Errorf("value has invalid type")
	case v.GetType() == VoidT:
		return fmt.Errorf("value has type `void` and cannot be used here")
	default:
		return nil
	}
}

func (a *Analyzer) resolveSubscriptExpr(
	ss *ast.SubscriptExpr, fb *FunctionBody,
) Value {
	receiverV := a.resolveExpr(ss.Receiver, fb, nil)

	var argumentV Value
	if sr, ok := ss.Argument.(*ast.SliceExpr); ok {
		var startV, endV Value
		startV, endV = VoidV, VoidV
		if sr.Start != nil {
			startV = a.resolveExpr(sr.Start, fb, nil)
		}
		if sr.End != nil {
			endV = a.resolveExpr(sr.End, fb, nil)
		}
		argumentV = &SliceRange{Node: sr, StartArg: startV, EndArg: endV}
	} else {
		argumentV = a.resolveExpr(ss.Argument, fb, nil)
	}

	if err := requireValidValue(receiverV); err != nil {
		a.addError(ss.Receiver, err)
		return InvalidV
	}
	if argumentV == InvalidV {
		a.errorF(ss.Argument, "invalid argument for subscript expr")
		return InvalidV
	}

	switch t := receiverV.GetType().(type) {
	case *SliceT:
		ia := &IndexAccess{
			Node: ss, Receiver: receiverV, Argument: argumentV,
			Type: t.ElementType,
		}
		if argumentV.GetType() == IntT {
			return ia
		} else if _, ok := argumentV.(*SliceRange); ok {
			ia.Type = &SliceT{ElementType: t.ElementType}
			return ia
		} else {
			a.errorF(ss.Argument, "invalid argument for subscript expr")
			return InvalidV
		}
	case *PrimitiveType:
		if t == StrT {
			return &IndexAccess{
				Node: ss, Receiver: receiverV, Argument: argumentV,
				Type: StrT,
			}
		} else {
			a.errorF(ss, "invalid subscript access on type `%s`",
				t.getName())
			return InvalidV
		}
	default:
		a.errorF(ss, "invalid subscript access on type `%s`",
			t.getName(),
		)
		return InvalidV
	}
}

func (a *Analyzer) resolveSpreadExpr(
	expr *ast.SpreadExpr, fb *FunctionBody,
) Value {
	sv := &SpreadValue{Node: expr}
	val := a.resolveExpr(expr.Value, fb, nil)
	if err := requireValidValue(val); err != nil {
		a.addError(expr.Value, err)
		return InvalidV
	}
	if _, ok := val.GetType().(*SliceT); !ok {
		a.errorF(expr.Value, "spread not valid on type `%s`",
			val.GetType().getName())
		return InvalidV
	}
	sv.Value = val
	return sv
}

func (a *Analyzer) resolveFunctionExpr(node *ast.FunctionExpr) Value {
	ft := &FunctionT{
		Node:   node,
		RState: Resolved,
		Layer:  User,
	}
	a.enterScope("[lit]")
	for _, p := range node.Signature.Parameters {
		a.resolveFunctionParam(ft, p, nil)
	}
	ft.ReturnType = a.resolveType(node.Signature.ReturnType)
	a.exitScope()
	return &FunctionLit{Node: node, Body: a.resolveBody(ft, node.Body)}
}

func (a *Analyzer) resolveResultExpr(
	node *ast.ResultExpr, fb *FunctionBody, targetT Type,
) Value {
	want, _ := targetT.(*ResultT) // nil when there is no expected Result type
	v := &WrapperV{Node: node, Kind: WrapOk}

	if node.IsError {
		v.Kind = WrapErr
		if want == nil {
			a.errorF(node, "cannot infer the type of `Err()`")
			return InvalidV
		}
		if v.Value = a.resolveExpr(node.Value, fb, nil); !a.requireValue(node.Value, v.Value) {
			return InvalidV
		}
		v.Type = want
		return v
	}

	// Ok() with no value is void!
	elem := Type(VoidT)
	if node.Value != nil {
		var target Type
		if want != nil {
			target = want.ElemType
		}
		v.Value = a.resolveExpr(node.Value, fb, target)
		if !a.requireValue(node.Value, v.Value) {
			return InvalidV
		}
		elem = v.Value.GetType()
	}
	if want != nil {
		if err := identical(want.ElemType, elem); err != nil {
			a.addError(node, err)
			return InvalidV
		}
	}
	v.Type = &ResultT{ElemType: elem}
	return v
}

func (a *Analyzer) resolveMaybeExpr(
	node *ast.MaybeExpr, fb *FunctionBody, targetT Type,
) Value {
	want, _ := targetT.(*MaybeT)
	v := &WrapperV{Node: node, Kind: WrapSome}

	if node.IsNone {
		v.Kind = WrapNone
		if want == nil {
			a.errorF(node, "cannot infer the type of `None()`")
			return InvalidV
		}
		v.Type = want
		return v
	}

	var target Type
	if want != nil {
		target = want.ElemType
	}
	v.Value = a.resolveExpr(node.Value, fb, target)
	if !a.requireValue(node.Value, v.Value) {
		return InvalidV
	}
	if want != nil {
		if err := identical(want.ElemType, v.Value.GetType()); err != nil {
			a.addError(node.Value, err)
			return InvalidV
		}
	}
	v.Type = &MaybeT{ElemType: v.Value.GetType()}
	return v
}

func (a *Analyzer) resolveTryExpr(
	expr *ast.TryExpr, fb *FunctionBody,
) Value {
	tv := &TryV{Node: expr}

	val := a.resolveExpr(expr.Value, fb, nil)
	if val.GetType() == InvalidT {
		return InvalidV
	}
	tv.Value = val
	if res, isResult := val.GetType().(*ResultT); isResult {
		tv.Type = res.ElemType
	} else if may, isMaybe := val.GetType().(*MaybeT); isMaybe {
		tv.Type = may.ElemType
	} else {
		a.errorF(expr.Value, "expected `Maybe` or `Result`, got %s",
			val.GetType().getName())
		return InvalidV
	}

	if expr.ElseValue == nil {
		ret := fb.Type.ReturnType
		_, valIsResult := val.GetType().(*ResultT)
		_, retIsResult := ret.(*ResultT)
		_, retIsMaybe := ret.(*MaybeT)
		if (valIsResult && !retIsResult) || (!valIsResult && !retIsMaybe) {
			a.errorF(expr, "`try` without `else` needs a function that returns `%s`",
				map[bool]string{true: "Result", false: "Maybe"}[valIsResult])
			return InvalidV
		}
		return tv
	}

	a.enterScope("[else]")
	defer a.exitScope()

	if expr.ErrName != "" {
		if _, ok := tv.Value.GetType().(*MaybeT); ok {
			a.errorF(expr, "cannot bind `None` to `%s`", expr.ErrName)
			return InvalidV
		}
		cb := &CaseBinding{Node: expr, Name: expr.ErrName, Type: ErrorT}
		tv.Err = cb
		if err := a.CurrentScope.declareValue(cb.Name, cb); err != nil {
			a.addError(expr, err)
		}
	}

	if expr.ElseValue != nil {
		blk := a.resolveBranch(fb, expr.ElseValue, true, true)
		if !terminates(blk) {
			if err := identical(tv.Type, blk.Type); err != nil {
				a.addError(expr.ElseValue, err)
				return InvalidV
			}
		}
		tv.Else = blk
	}

	return tv
}

func (a *Analyzer) resolveNegativeExpr(
	expr *ast.NegativeExpr, fb *FunctionBody,
) Value {
	val := a.resolveExpr(expr.Value, fb, nil)
	if lit, ok := val.(*IntLiteral); ok {
		lit.Value = -lit.Value
		return lit
	} else if lit, ok := val.(*FloatLiteral); ok {
		lit.Value = -lit.Value
		return lit
	} else {
		a.errorF(
			expr,
			"value of type `%s` cannot be negative",
			val.GetType().getName(),
		)
		return InvalidV
	}
}

func (a *Analyzer) resolveExpr(
	expr ast.Expression, fb *FunctionBody, targetT Type,
) Value {
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
		return a.resolveIdentifierExpr(t, fb)
	case *ast.InfixExpr:
		return a.resolveInfixExpr(t, fb)
	case *ast.CallExpr:
		return a.resolveCallExpr(t, fb)
	case *ast.CopyExpr:
		return a.resolveCopyExpr(t, fb)
	case *ast.MemberExpr:
		return a.resolveMemberExpr(t, fb)
	case *ast.IfExpr:
		return a.resolveIf(t, fb, true)
	case *ast.SwitchExpr:
		return a.resolveSwitch(t, fb, true)
	case *ast.SequenceLiteral:
		return a.resolveSequenceLiteral(t, fb, targetT)
	case *ast.SubscriptExpr:
		return a.resolveSubscriptExpr(t, fb)
	case *ast.FunctionExpr:
		return a.resolveFunctionExpr(t)
	case *ast.ResultExpr:
		return a.resolveResultExpr(t, fb, targetT)
	case *ast.MaybeExpr:
		return a.resolveMaybeExpr(t, fb, targetT)
	case *ast.TryExpr:
		return a.resolveTryExpr(t, fb)
	case *ast.NegativeExpr:
		return a.resolveNegativeExpr(t, fb)
	}
	return InvalidV
}

func subGeneric(t Type, m map[*TypeParamT]Type) Type {
	switch t := t.(type) {
	case *TypeParamT:
		if r, ok := m[t]; ok {
			return r
		}
		return t
	case *SliceT:
		return &SliceT{ElementType: subGeneric(t.ElementType, m)}
	case *ResultT:
		return &ResultT{ElemType: subGeneric(t.ElemType, m)}
	case *MaybeT:
		return &MaybeT{ElemType: subGeneric(t.ElemType, m)}
	case *FunctionT:
		ft := *t
		ft.Params = make([]FParam, len(t.Params))
		for idx, p := range t.Params {
			p.Type = subGeneric(p.Type, m)
			ft.Params[idx] = p
		}
		ft.ReturnType = subGeneric(t.ReturnType, m)
		return &ft
	}
	return t // primitives, non-generic structs and enums
}

func unifyGeneric(paramT, argT Type, m map[*TypeParamT]Type) error {
	switch pt := paramT.(type) {
	case *TypeParamT:
		if known, ok := m[pt]; ok {
			return identical(argT, known)
		}
		m[pt] = argT
		return nil
	case *SliceT:
		a, ok := argT.(*SliceT)
		if !ok {
			return mismatch(paramT, argT)
		}
		return unifyGeneric(pt.ElementType, a.ElementType, m)
	case *ResultT:
		a, ok := argT.(*ResultT)
		if !ok {
			return mismatch(paramT, argT)
		}
		return unifyGeneric(pt.ElemType, a.ElemType, m)
	case *MaybeT:
		a, ok := argT.(*MaybeT)
		if !ok {
			return mismatch(paramT, argT)
		}
		return unifyGeneric(pt.ElemType, a.ElemType, m)
	case *FunctionT:
		a, ok := argT.(*FunctionT)
		if !ok || len(pt.Params) != len(a.Params) {
			return mismatch(paramT, argT)
		}
		for idx := range pt.Params {
			if err := unifyGeneric(pt.Params[idx].Type, a.Params[idx].Type, m); err != nil {
				return err
			}
		}
		return unifyGeneric(pt.ReturnType, a.ReturnType, m)
	}
	return identical(argT, paramT)
}
