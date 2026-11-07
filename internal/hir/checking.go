package hir

import "fmt"

func identical(a, b Type) error {
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
		bt, ok := b.(*EnumT)
		if !ok || at != bt {
			return defaultErr
		}
		return nil
	default:
		return fmt.Errorf("unknown type `%T`", a)
	}
}
