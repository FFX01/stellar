package analyzer

import (
	"fmt"
	"stellar/internal/ast"
	"strings"
)

type Type interface {
	typ()
	String() string
}

type PrimitiveType struct {
	Id string
}

func (pt *PrimitiveType) typ()           {}
func (pt *PrimitiveType) String() string { return pt.Id }

var (
	IntT     = &PrimitiveType{Id: "int"}
	FloatT   = &PrimitiveType{Id: "float"}
	StrT     = &PrimitiveType{Id: "str"}
	BoolT    = &PrimitiveType{Id: "bool"}
	InvalidT = &PrimitiveType{Id: "invalid"}
	VoidT    = &PrimitiveType{Id: "void"}
	NeverT   = &PrimitiveType{Id: "never"}
	SelfT    = &PrimitiveType{Id: "self"}
	LenT     = &BuiltInFunctionType{Name: "len"}
)

var ErrorT = &InterfaceType{
	Name:        "Error",
	MethodOrder: []string{"error"},
	Methods: map[string]*FunctionType{
		"error": &FunctionType{
			Params:     []Param{Param{Name: "self", Type: SelfT}},
			ReturnType: StrT,
		},
	},
}

type SliceType struct {
	ElemType Type
}

func (st *SliceType) typ() {}
func (st *SliceType) String() string {
	return fmt.Sprintf("[]%s", st.ElemType.String())
}

type ResultType struct {
	ValueType Type
}

func (rt *ResultType) typ() {}
func (rt *ResultType) String() string {
	return fmt.Sprintf("%s!", rt.ValueType.String())
}

type MaybeType struct {
	ValueType Type
}

func (mt *MaybeType) typ() {}
func (mt *MaybeType) String() string {
	return fmt.Sprintf("%s?", mt.ValueType.String())
}

type Param struct {
	Name string
	Type Type
}

type FieldInfo struct {
	Name     string
	Type     Type
	IsPublic bool
}

type MemberInfo struct {
	Name   string
	Fields []FieldInfo
}

type MethodInfo struct {
	Name     string
	Type     *FunctionType
	IsPublic bool
	Node     *ast.FunctionStmt
}

type MethodSet struct {
	order  []string
	byName map[string]*MethodInfo
}

func (ms *MethodSet) Add(info *MethodInfo) bool {
	if ms.byName == nil {
		ms.byName = map[string]*MethodInfo{}
	}
	if _, ok := ms.byName[info.Name]; ok {
		return false
	}
	ms.order = append(ms.order, info.Name)
	ms.byName[info.Name] = info
	return true
}

func (ms *MethodSet) Get(name string) *MethodInfo {
	if mi, ok := ms.byName[name]; ok {
		return mi
	}
	return nil
}

func (ms *MethodSet) All() []*MethodInfo {
	var out []*MethodInfo
	for _, name := range ms.order {
		if mi, ok := ms.byName[name]; ok {
			out = append(out, mi)
		}
	}
	return out
}

type FunctionType struct {
	Params     []Param
	ReturnType Type
}

func (ft *FunctionType) typ() {}
func (ft *FunctionType) String() string {
	var paramTypes []string
	for _, p := range ft.Params {
		paramTypes = append(paramTypes, p.Type.String())
	}
	return fmt.Sprintf(
		"fn(%s) -> %s",
		strings.Join(paramTypes, ", "),
		ft.ReturnType.String(),
	)

}

type StructType struct {
	Name    string
	Fields  []FieldInfo
	Methods MethodSet
}

func (st *StructType) typ() {}
func (st *StructType) String() string {
	return st.Name
}

type EnumType struct {
	Name    string
	Members []MemberInfo
	Methods MethodSet
}

func (et *EnumType) typ()           {}
func (et *EnumType) String() string { return et.Name }

type InterfaceType struct {
	Name        string
	MethodOrder []string
	Methods     map[string]*FunctionType
}

func (it *InterfaceType) typ()           {}
func (it *InterfaceType) String() string { return it.Name }

type TypeParamType struct {
	Name string
}

type BuiltInFunctionType struct {
	Name string
}

func (bift *BuiltInFunctionType) typ() {}
func (bift *BuiltInFunctionType) String() string {
	return fmt.Sprintf("builtin %s", bift.Name)
}

func identicalParam(a, b Param) bool {
	if !identical(a.Type, b.Type) {
		return false
	}
	return true
}

func identicalFunc(a, b *FunctionType) bool {
	if !identical(a.ReturnType, b.ReturnType) {
		return false
	}
	if len(a.Params) != len(b.Params) {
		return false
	}
	for idx, aParam := range a.Params {
		bParam := b.Params[idx]
		if !identicalParam(aParam, bParam) {
			return false
		}
	}
	return true
}

func identical(a, b Type) bool {
	switch a_T := a.(type) {
	case *ResultType:
		b_T, ok := b.(*ResultType)
		if !ok {
			return false
		}
		return identical(a_T.ValueType, b_T.ValueType)
	case *MaybeType:
		b_T, ok := b.(*MaybeType)
		if !ok {
			return false
		}
		return identical(a_T.ValueType, b_T.ValueType)
	case *SliceType:
		b_T, ok := b.(*SliceType)
		if !ok {
			return false
		}
		return identical(a_T.ElemType, b_T.ElemType)
	case *FunctionType:
		b_T, ok := b.(*FunctionType)
		if !ok {
			return false
		}
		return identicalFunc(a_T, b_T)
	case *PrimitiveType:
		b_T, ok := b.(*PrimitiveType)
		if !ok {
			return false
		}
		return a_T == b_T
	case *StructType, *EnumType, *InterfaceType, *BuiltInFunctionType:
		return a == b
	}
	return false
}
