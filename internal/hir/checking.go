package hir

import (
	"fmt"
	"log"
)

func identical(a, b Type) error {
	if a == nil || b == nil {
		return fmt.Errorf("nil type")
	}
	defaultErr := fmt.Errorf("type `%s` != type `%s`", a.getName(), b.getName())

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
		log.Printf("%T %T", at, bt)
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
		case isVariant && at != vt:
			return defaultErr
		case isEnum:
			return identical(at.Enum, et)
		case !isEnum && !isVariant:
			return defaultErr
		}
		return nil
	default:
		return fmt.Errorf("unknown type `%T`", a)
	}
}
