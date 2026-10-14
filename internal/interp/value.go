// Package interp is a tree-walking interpreter for type-checked Stellar
// programs.
//
// # Values
//
// Primitives are represented by the Go type of the same name (int32 by
// int32, str by string, ...), so a value's runtime type can always be
// recovered. Compound values are pointers: *Object, *Array, *MapVal and
// *EnumVal. Maybe and Result values are *Some / none and *OkVal / *ErrVal.
//
// # Value semantics
//
// Stellar values behave as copies. The interpreter implements this with
// copy-on-write: whenever a value is stored somewhere (bound to a name,
// placed in a field or element, passed as an argument, returned), it is
// marked shared. Writing through a place first replaces a shared value with
// a shallow copy, whose children are in turn marked shared. A value that
// was never shared is written in place, so building up a local array or
// object costs nothing extra.
package interp

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"stellar/internal/types"
)

// Value is a runtime value. See the package documentation.
type Value any

// Object is an instance of an object type.
type Object struct {
	Type   *types.Object
	Fields []Value
	shared bool
}

// Array is an array value.
type Array struct {
	Elems  []Value
	shared bool
}

// MapVal is a map value. Keys keeps insertion order for iteration.
type MapVal struct {
	Keys   []Value
	Vals   map[Value]Value
	shared bool
}

// EnumVal is an enum value. Enum values are never written in place.
type EnumVal struct {
	Type    *types.Enum
	Variant *types.Variant
	Fields  []Value
}

// RefVal is a Ref[T]. Copying a Ref copies this pointer, so every copy
// reaches the same target; == compares the pointers.
type RefVal struct{ V Value }

// Some is a present Maybe value.
type Some struct{ V Value }

// noneVal is the absent Maybe value.
type noneVal struct{}

// None is the single absent Maybe value.
var None Value = noneVal{}

// OkVal is a successful Result. V is nil for a bare Result.
type OkVal struct{ V Value }

// ErrVal is a failed Result. E is an Error value.
type ErrVal struct{ E Value }

// ErrorVal is the hidden built-in error type made by Err("msg") and wrap().
type ErrorVal struct {
	Msg   string
	Cause Value // the wrapped error, or nil
}

// markShared records that v is now held in more than one place. Wrappers
// that are never written in place (Maybe, Result, enums) pass the mark to
// their contents, which can be extracted and written later. A Ref is shared
// by design; its target is marked when stored into it.
func markShared(v Value) {
	switch v := v.(type) {
	case *Object:
		v.shared = true
	case *Array:
		v.shared = true
	case *MapVal:
		v.shared = true
	case *Some:
		markShared(v.V)
	case *OkVal:
		markShared(v.V)
	case *ErrVal:
		markShared(v.E)
	case *EnumVal:
		for _, f := range v.Fields {
			markShared(f)
		}
	}
}

// unshare returns v ready to be written in place: v itself if it is not
// shared, otherwise a shallow copy whose children are marked shared.
func unshare(v Value) Value {
	switch v := v.(type) {
	case *Object:
		if !v.shared {
			return v
		}
		c := &Object{Type: v.Type, Fields: append([]Value(nil), v.Fields...)}
		for _, f := range c.Fields {
			markShared(f)
		}
		return c
	case *Array:
		if !v.shared {
			return v
		}
		c := &Array{Elems: append([]Value(nil), v.Elems...)}
		for _, e := range c.Elems {
			markShared(e)
		}
		return c
	case *MapVal:
		if !v.shared {
			return v
		}
		c := &MapVal{Keys: append([]Value(nil), v.Keys...), Vals: make(map[Value]Value, len(v.Vals))}
		for k, val := range v.Vals {
			markShared(val)
			c.Vals[k] = val
		}
		return c
	}
	return v
}

// equal reports structural equality, as defined by ==.
func equal(a, b Value) bool {
	switch a := a.(type) {
	case *Object:
		b, ok := b.(*Object)
		return ok && a.Type == b.Type && equalSlices(a.Fields, b.Fields)
	case *Array:
		b, ok := b.(*Array)
		return ok && equalSlices(a.Elems, b.Elems)
	case *MapVal:
		b, ok := b.(*MapVal)
		if !ok || len(a.Vals) != len(b.Vals) {
			return false
		}
		for k, v := range a.Vals {
			w, ok := b.Vals[k]
			if !ok || !equal(v, w) {
				return false
			}
		}
		return true
	case *EnumVal:
		b, ok := b.(*EnumVal)
		return ok && a.Variant == b.Variant && equalSlices(a.Fields, b.Fields)
	case *Some:
		b, ok := b.(*Some)
		return ok && equal(a.V, b.V)
	case *OkVal:
		b, ok := b.(*OkVal)
		return ok && equal(a.V, b.V)
	case *ErrVal:
		b, ok := b.(*ErrVal)
		return ok && equal(a.E, b.E)
	case *ErrorVal:
		b, ok := b.(*ErrorVal)
		return ok && a.Msg == b.Msg && equal(a.Cause, b.Cause)
	}
	return a == b
}

func equalSlices(a, b []Value) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

// runtimeType returns the concrete type of a value held in an interface.
func runtimeType(v Value) types.Type {
	switch v := v.(type) {
	case *Object:
		return v.Type
	case *EnumVal:
		return v.Type
	case int8:
		return types.TInt8
	case int16:
		return types.TInt16
	case int32:
		return types.TInt32
	case int64:
		return types.TInt64
	case uint8:
		return types.TUint8
	case uint16:
		return types.TUint16
	case uint32:
		return types.TUint32
	case uint64:
		return types.TUint64
	case float32:
		return types.TFloat32
	case float64:
		return types.TFloat64
	case bool:
		return types.TBool
	case string:
		return types.TStr
	}
	return nil
}

// formatPrimitive renders a primitive as str(x) does.
func formatPrimitive(v Value) string {
	switch v := v.(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float32:
		return formatFloat(float64(v), 32)
	case float64:
		return formatFloat(v, 64)
	}
	return fmt.Sprint(v)
}

// formatFloat always shows a decimal point for finite whole numbers, so
// str(2.0) is "2.0", not "2".
func formatFloat(f float64, bits int) string {
	s := strconv.FormatFloat(f, 'g', -1, bits)
	if !math.IsInf(f, 0) && !math.IsNaN(f) && !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}
