package hir

import (
	"fmt"
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
	default:
		return fmt.Errorf("unknown type `%T`", a)
	}
}
