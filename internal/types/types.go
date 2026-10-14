// Package types implements the Stellar type checker.
//
// The checker resolves names, computes the type of every expression, and
// enforces the language's static rules: strict typing, literal typing from
// context, deep immutability, error handling, exhaustive matching and the
// strict discard rule. Its output, Info, records what the interpreter needs
// to run the program.
package types

import (
	"fmt"
	"strings"

	"stellar/internal/ast"
)

// Type is a Stellar type.
type Type interface {
	String() string
}

// BasicKind identifies a primitive type.
type BasicKind int

const (
	Int8 BasicKind = iota
	Int16
	Int32
	Int64
	Uint8
	Uint16
	Uint32
	Uint64
	Float32
	Float64
	Bool
	Str
)

// Basic is a primitive type.
type Basic struct {
	Kind BasicKind
	Name string
}

func (t *Basic) String() string { return t.Name }

// IsInteger reports whether t is a signed or unsigned integer type.
func (t *Basic) IsInteger() bool { return t.Kind <= Uint64 }

// IsSigned reports whether t is a signed integer type.
func (t *Basic) IsSigned() bool { return t.Kind <= Int64 }

// IsFloat reports whether t is a floating-point type.
func (t *Basic) IsFloat() bool { return t.Kind == Float32 || t.Kind == Float64 }

// IsNumeric reports whether t is an integer or floating-point type.
func (t *Basic) IsNumeric() bool { return t.IsInteger() || t.IsFloat() }

// Bits returns the size of a numeric type in bits.
func (t *Basic) Bits() int {
	switch t.Kind {
	case Int8, Uint8:
		return 8
	case Int16, Uint16:
		return 16
	case Int32, Uint32, Float32:
		return 32
	default:
		return 64
	}
}

// Primitive types.
var (
	TInt8    = &Basic{Int8, "int8"}
	TInt16   = &Basic{Int16, "int16"}
	TInt32   = &Basic{Int32, "int32"}
	TInt64   = &Basic{Int64, "int64"}
	TUint8   = &Basic{Uint8, "uint8"}
	TUint16  = &Basic{Uint16, "uint16"}
	TUint32  = &Basic{Uint32, "uint32"}
	TUint64  = &Basic{Uint64, "uint64"}
	TFloat32 = &Basic{Float32, "float32"}
	TFloat64 = &Basic{Float64, "float64"}
	TBool    = &Basic{Bool, "bool"}
	TStr     = &Basic{Str, "str"}
)

var basics = []*Basic{TInt8, TInt16, TInt32, TInt64, TUint8, TUint16, TUint32, TUint64, TFloat32, TFloat64, TBool, TStr}

// voidType is the "type" of an expression that produces no value, such as
// a call to a function without a result. It is never written in source.
type voidType struct{}

func (voidType) String() string { return "no value" }

// neverType is the type of an expression that never completes normally,
// such as a block ending in `return`. It is assignable to every type.
type neverType struct{}

func (neverType) String() string { return "never" }

// invalidType is the type of an expression that already failed to check.
// Operations on it report nothing further, which prevents cascades.
type invalidType struct{}

func (invalidType) String() string { return "invalid" }

// selfType stands for the implementing type inside an interface
// declaration. It is replaced by a concrete type with Subst.
type selfType struct{}

func (selfType) String() string { return "Self" }

// Singleton internal types.
var (
	Void    Type = voidType{}
	Never   Type = neverType{}
	Invalid Type = invalidType{}
	SelfT   Type = selfType{}
)

// Field is an object field or enum variant field.
type Field struct {
	Name string
	Type Type
	Pub  bool
}

// Method is a function attached to a named type, either inherent or from
// an interface impl. Self is nil for a static function.
type Method struct {
	Name      string
	Owner     Type       // the type the method belongs to
	Interface *Interface // nil for an inherent method
	Self      *SelfParam
	Sig       *Func // excludes the receiver
	Pub       bool
	Decl      *ast.FuncDecl // nil for methods of interface values
}

// SelfParam describes a method receiver.
type SelfParam struct {
	Mut bool
}

// Object is a user-declared object type.
type Object struct {
	Name    string
	Fields  []*Field
	Methods map[string]*Method // inherent methods and static functions
	Impls   map[*Interface]map[string]*Method
}

func (t *Object) String() string { return t.Name }

// Field returns the named field, or nil.
func (t *Object) Field(name string) *Field {
	for _, f := range t.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// Enum is a user-declared enum type.
type Enum struct {
	Name     string
	Variants []*Variant
	Methods  map[string]*Method
	Impls    map[*Interface]map[string]*Method
}

func (t *Enum) String() string { return t.Name }

// Variant returns the named variant, or nil.
func (t *Enum) Variant(name string) *Variant {
	for _, v := range t.Variants {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// Variant is one case of an enum. Fields is nil for a variant without data.
type Variant struct {
	Name   string
	Enum   *Enum
	Index  int
	Fields []*Field
}

// Field returns the named field, or nil.
func (v *Variant) Field(name string) *Field {
	for _, f := range v.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// Interface is a user-declared or built-in interface. Impls maps each
// implementing type to its methods.
type Interface struct {
	Name    string
	Methods []*IfaceMethod
	Impls   map[Type]map[string]*Method
}

func (t *Interface) String() string { return t.Name }

// Method returns the named method, or nil.
func (t *Interface) Method(name string) *IfaceMethod {
	for _, m := range t.Methods {
		if m.Name == name {
			return m
		}
	}
	return nil
}

// IfaceMethod is a method signature in an interface. Every interface method
// has a receiver.
type IfaceMethod struct {
	Name    string
	MutSelf bool
	Sig     *Func // excludes the receiver
}

// Func is a function type. A parameter that the function may change in the
// caller is a *Mut; Mut appears nowhere else.
type Func struct {
	Params []Type
	Result Type // Void if the function produces no value
}

func (t *Func) String() string {
	var b strings.Builder
	b.WriteString("fn(")
	for i, p := range t.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.String())
	}
	b.WriteString(")")
	if t.Result != Void {
		b.WriteString(" -> ")
		b.WriteString(t.Result.String())
	}
	return b.String()
}

// Array is `[]T`.
type Array struct{ Elem Type }

func (t *Array) String() string { return "[]" + t.Elem.String() }

// Map is `Map[K, V]`.
type Map struct{ Key, Value Type }

func (t *Map) String() string { return fmt.Sprintf("Map[%s, %s]", t.Key, t.Value) }

// Result is `Result[T]`, or bare `Result` when Ok is Void.
type Result struct{ Ok Type }

func (t *Result) String() string {
	if t.Ok == Void {
		return "Result"
	}
	return fmt.Sprintf("Result[%s]", t.Ok)
}

// Maybe is `Maybe[T]`, written `?T`.
type Maybe struct{ Elem Type }

func (t *Maybe) String() string { return "?" + t.Elem.String() }

// Ref is `Ref[T]`, a shared reference to a mutable target. Copying a Ref
// copies the reference, and the target is writable through any copy.
type Ref struct{ Elem Type }

func (t *Ref) String() string { return fmt.Sprintf("Ref[%s]", t.Elem) }

// Mut is `mut T`, the type of a parameter that the function may change in
// the caller. It only appears in Func.Params.
type Mut struct{ Elem Type }

func (t *Mut) String() string { return "mut " + t.Elem.String() }

// ---------------------------------------------------------------------------
// Type relations

// Identical reports whether a and b are the same type.
func Identical(a, b Type) bool {
	if a == b {
		return true
	}
	switch a := a.(type) {
	case *Func:
		b, ok := b.(*Func)
		if !ok || len(a.Params) != len(b.Params) || !Identical(a.Result, b.Result) {
			return false
		}
		for i := range a.Params {
			if !Identical(a.Params[i], b.Params[i]) {
				return false
			}
		}
		return true
	case *Array:
		b, ok := b.(*Array)
		return ok && Identical(a.Elem, b.Elem)
	case *Map:
		b, ok := b.(*Map)
		return ok && Identical(a.Key, b.Key) && Identical(a.Value, b.Value)
	case *Result:
		b, ok := b.(*Result)
		return ok && Identical(a.Ok, b.Ok)
	case *Maybe:
		b, ok := b.(*Maybe)
		return ok && Identical(a.Elem, b.Elem)
	case *Mut:
		b, ok := b.(*Mut)
		return ok && Identical(a.Elem, b.Elem)
	case *Ref:
		b, ok := b.(*Ref)
		return ok && Identical(a.Elem, b.Elem)
	}
	return false
}

// Assignable reports whether a value of type from can be used where a value
// of type to is expected. Untyped literals are handled separately.
//
// Beyond identity: Never goes anywhere; a type goes to an interface it
// implements; and arrays, Maybe and Result are covariant, which is sound
// because they are values that nothing else can write to.
func Assignable(from, to Type) bool {
	if from == Never || Identical(from, to) {
		return true
	}
	switch to := to.(type) {
	case *Interface:
		return Implements(from, to)
	case *Array:
		from, ok := from.(*Array)
		return ok && Assignable(from.Elem, to.Elem)
	case *Maybe:
		from, ok := from.(*Maybe)
		return ok && Assignable(from.Elem, to.Elem)
	case *Result:
		from, ok := from.(*Result)
		return ok && Assignable(from.Ok, to.Ok)
	}
	return false
}

// Implements reports whether t has an impl of iface. An interface trivially
// implements itself.
func Implements(t Type, iface *Interface) bool {
	if t == iface {
		return true
	}
	_, ok := iface.Impls[t]
	return ok
}

// Comparable reports whether values of t can be compared with == and !=.
// Refs compare by identity, so they are always comparable.
func Comparable(t Type) bool {
	switch t := t.(type) {
	case *Ref:
		return true
	case *Func:
		return false
	case *Array:
		return Comparable(t.Elem)
	case *Map:
		return Comparable(t.Value)
	case *Maybe:
		return Comparable(t.Elem)
	case *Result:
		return t.Ok == Void || Comparable(t.Ok)
	case *Object:
		for _, f := range t.Fields {
			if !Comparable(f.Type) {
				return false
			}
		}
	case *Enum:
		for _, v := range t.Variants {
			for _, f := range v.Fields {
				if !Comparable(f.Type) {
					return false
				}
			}
		}
	}
	return true
}

// Ordered reports whether values of t can be compared with < <= > >=.
func Ordered(t Type) bool {
	b, ok := t.(*Basic)
	return ok && (b.IsNumeric() || b.Kind == Str)
}

// Subst replaces Self with self throughout t.
func Subst(t, self Type) Type {
	switch t := t.(type) {
	case selfType:
		return self
	case *Mut:
		return &Mut{Elem: Subst(t.Elem, self)}
	case *Ref:
		return &Ref{Elem: Subst(t.Elem, self)}
	case *Maybe:
		return &Maybe{Elem: Subst(t.Elem, self)}
	case *Result:
		if t.Ok == Void {
			return t
		}
		return &Result{Ok: Subst(t.Ok, self)}
	case *Array:
		return &Array{Elem: Subst(t.Elem, self)}
	case *Map:
		return &Map{Key: Subst(t.Key, self), Value: Subst(t.Value, self)}
	case *Func:
		f := &Func{Result: Subst(t.Result, self)}
		for _, p := range t.Params {
			f.Params = append(f.Params, Subst(p, self))
		}
		return f
	}
	return t
}

// methodString renders a method signature including its receiver.
func methodString(mutSelf bool, f *Func) string {
	parts := []string{"self"}
	if mutSelf {
		parts[0] = "mut self"
	}
	for _, p := range f.Params {
		parts = append(parts, p.String())
	}
	s := "fn(" + strings.Join(parts, ", ") + ")"
	if f.Result != Void {
		s += " -> " + f.Result.String()
	}
	return s
}
