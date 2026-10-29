package analyzer

import (
	"stellar/internal/ast"
)

type Type interface {
	typ()
}

type PrimitiveType struct {
	Id string
}

func (pt *PrimitiveType) typ() {}

var (
	IntT     = &PrimitiveType{Id: "int"}
	FloatT   = &PrimitiveType{Id: "float"}
	StrT     = &PrimitiveType{Id: "str"}
	BoolT    = &PrimitiveType{Id: "bool"}
	InvalidT = &PrimitiveType{Id: "invalid"}
	VoidT    = &PrimitiveType{Id: "void"}
)

type SliceType struct {
	ElemType Type
}

func (st *SliceType) typ() {}

type ResultType struct {
	ValueType Type
}

func (rt *ResultType) typ() {}

type Param struct {
	Name  string
	Type  Type
	IsMut bool
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

type FunctionType struct {
	Params     []Param
	ReturnType Type
}

func (ft *FunctionType) typ() {}

type ObjectType struct {
	Name         string
	Fields       []FieldInfo
	MethodsOrder []string
	Methods      map[string]*MethodInfo
}

func (ot *ObjectType) typ() {}

type EnumType struct {
	Name    string
	Members []MemberInfo
}

func (et *EnumType) typ() {}

type InterfaceType struct {
	Name        string
	MethodOrder []string
	Methods     map[string]*FunctionType
}

func (it *InterfaceType) typ() {}

type TypeParamType struct {
	Name string
}

func identicalParam(a, b Param) bool {
	if a.IsMut != b.IsMut {
		return false
	}
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
		if a_T.ValueType != nil {
			return identical(a_T.ValueType, b_T.ValueType)
		}
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
		return a_T.Id == b_T.Id
	case *ObjectType, *EnumType, *InterfaceType:
		return a == b
	}
	return false
}
