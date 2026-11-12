package hir

import (
	"fmt"
	"slices"
)

func identical(a, b Type) error {
	if a == nil || b == nil {
		return fmt.Errorf("nil type")
	}
	defaultErr := fmt.Errorf("type `%s` != type `%s`", a.getName(), b.getName())

	if a == AnyT || b == AnyT {
		return nil
	}

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
	case *StructT:
		bt, ok := b.(*StructT)
		if !ok || at != bt {
			return defaultErr
		}
		return nil
	case *EnumT:
		bt, isEnum := b.(*EnumT)
		vt, isVariant := b.(*VariantT)
		switch {
		case isEnum && at != bt:
			return defaultErr
		case isVariant:
			return identical(at, vt.Enum)
		case !isEnum && !isVariant:
			return defaultErr
		}
		return nil
	case *VariantT:
		vt, isVariant := b.(*VariantT)
		et, isEnum := b.(*EnumT)
		switch {
		case isVariant && at.Enum == vt.Enum:
			return nil
		case isVariant && at != vt:
			return defaultErr
		case isEnum:
			return identical(at.Enum, et)
		case !isEnum && !isVariant:
			return defaultErr
		}
		return nil
	case *SliceT:
		bt, ok := b.(*SliceT)
		if !ok {
			return defaultErr
		}
		return identical(at.ElementType, bt.ElementType)
	case *TypeParamT:
		bt, ok := b.(*TypeParamT)
		if !ok || at != bt {
			return defaultErr
		} else {
			return nil
		}
	case *FunctionT:
		bt, ok := b.(*FunctionT)
		if !ok || !sameParams(at, bt, 0) {
			return defaultErr
		}
		return nil
	case *ResultT:
		bt, ok := b.(*ResultT)
		if !ok {
			return defaultErr
		}
		return identical(at.ElemType, bt.ElemType)
	case *MaybeT:
		bt, ok := b.(*MaybeT)
		if !ok {
			return defaultErr
		}
		return identical(at.ElemType, bt.ElemType)
	default:
		return fmt.Errorf("unknown type `%T`", a)
	}
}

func sameSignature(got, want *FunctionT, t Type) bool {
	if len(got.TypeParams) > 0 {
		return false
	}
	if len(got.Params) != len(want.Params) {
		return false
	}
	wantT := subGeneric(want, map[*TypeParamT]Type{SelfT: t}).(*FunctionT)
	return sameParams(got, wantT, 1)
}

func sameParams(a, b *FunctionT, from int) bool {
	if len(a.Params) != len(b.Params) {
		return false
	}
	for i := from; i < len(a.Params); i++ {
		if identical(a.Params[i].Type, b.Params[i].Type) != nil {
			return false
		}
	}
	return identical(a.ReturnType, b.ReturnType) == nil
}

func satisfies(t Type, iface *InterfaceT) error {
	for name, want := range iface.Methods {
		got, err := t.GetMethod(name)
		if err != nil {
			return fmt.Errorf(
				"`%s` does not satisfy `%s`; no method `%s`",
				t.getName(),
				iface.getName(),
				name,
			)
		}
		if !sameSignature(got, want, t) {
			return fmt.Errorf(
				"`%s` does not satisfy `%s`; method `%s` has wrong signature",
				t.getName(),
				iface.getName(),
				name,
			)
		}
	}
	return nil
}

func mismatch(want, got Type) error {
	return fmt.Errorf("type `%s` != type `%s`", got.getName(), want.getName())
}

func hasFreeTParams(
	t Type, free []*TypeParamT, m map[*TypeParamT]Type,
) bool {
	switch t := t.(type) {
	case *TypeParamT:
		_, known := m[t]
		return !known && slices.Contains(free, t)
	case *SliceT:
		return hasFreeTParams(t.ElementType, free, m)
	case *ResultT:
		return hasFreeTParams(t.ElemType, free, m)
	case *MaybeT:
		return hasFreeTParams(t.ElemType, free, m)
	case *FunctionT:
		for _, p := range t.Params {
			if hasFreeTParams(p.Type, free, m) {
				return true
			}
		}
		return hasFreeTParams(t.ReturnType, free, m)
	}
	return false
}
