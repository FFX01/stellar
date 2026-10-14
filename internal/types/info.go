package types

import (
	"stellar/internal/ast"
	"stellar/internal/token"
)

// Info records the results of type checking for later phases.
type Info struct {
	// Types holds the type of every checked expression. Untyped literals
	// have the type they took from context.
	Types map[ast.Expr]Type
	// TypeExprs holds the resolved type of every type written in source.
	TypeExprs map[ast.TypeExpr]Type
	// Defs maps declaring identifiers to what they declare.
	Defs map[*ast.Ident]Symbol
	// Uses maps referring identifiers to what they refer to.
	Uses map[*ast.Ident]Symbol
	// Selections describes every `X.Sel` that is not a plain module access.
	Selections map[*ast.SelectorExpr]*Selection
	// Variants maps enum match patterns to the variant they match.
	Variants map[*ast.VariantPattern]*Variant
	// Funcs maps top-level function declarations to their symbols.
	Funcs map[*ast.FuncDecl]*FuncSym
	// Methods maps method declarations inside impl blocks to their methods.
	Methods map[*ast.FuncDecl]*Method
	// Error is the built-in Error interface.
	Error *Interface
}

// SelectionKind classifies a selector expression.
type SelectionKind int

const (
	SelField      SelectionKind = iota // value.field
	SelMethod                          // value.method, bound to the receiver
	SelMethodExpr                      // Type.method; the receiver becomes the first argument
	SelStatic                          // Type.fn for a function without self
	SelVariant                         // Enum.Variant
	SelRefValue                        // r.value, the whole target of a Ref
)

// Selection describes what a selector expression refers to.
type Selection struct {
	Kind SelectionKind
	// Deref is set when a field or method is reached through a Ref, as in
	// r.firstName for r: Ref[Author].
	Deref   bool
	Field   *Field
	Method  *Method
	Variant *Variant
}

// Symbol is anything a name can refer to.
type Symbol interface {
	symbolName() string
}

// VarKind says how a variable was declared, which decides whether it can
// change.
type VarKind int

const (
	VarLet      VarKind = iota // let
	VarVar                     // var
	VarParam                   // ordinary parameter
	VarMutParam                // mut parameter
	VarSelf                    // self in a method taking self
	VarMutSelf                 // self in a method taking mut self
	VarLoop                    // for loop variable
	VarBinding                 // bound by a pattern, `as`, or `else |e|`
)

// Var is a local variable, parameter, loop variable or pattern binding.
type Var struct {
	Name string
	Type Type
	Kind VarKind
	Pos  token.Pos
	fn   *funcCtx
}

// Writable reports whether the variable can be reassigned or changed in
// place.
func (v *Var) Writable() bool {
	return v.Kind == VarVar || v.Kind == VarMutParam || v.Kind == VarMutSelf
}

// IsMutParam reports whether the variable is a mut parameter or mut self,
// which refer to the caller's data and only exist during the call.
func (v *Var) IsMutParam() bool {
	return v.Kind == VarMutParam || v.Kind == VarMutSelf
}

// FuncSym is a top-level function.
type FuncSym struct {
	Name string
	Sig  *Func
	Decl *ast.FuncDecl
}

// TypeSym is a named type: a primitive, a declared type, or Error.
type TypeSym struct {
	Name string
	Type Type
}

// GenericSym is one of the built-in parametric types: Result, Maybe, Map,
// Ref. Ref is also the constructor function Ref(x).
type GenericSym struct {
	Name string
}

// BuiltinID identifies a built-in function or constructor.
type BuiltinID int

const (
	BuiltinPrint BuiltinID = iota
	BuiltinPanic
	BuiltinLen
	BuiltinPush
	BuiltinWrap
	BuiltinCause
	BuiltinOk
	BuiltinErr
	BuiltinSome
	BuiltinNone
)

// BuiltinSym is a built-in function or Result/Maybe constructor.
type BuiltinSym struct {
	Name string
	ID   BuiltinID
}

func (s *Var) symbolName() string        { return s.Name }
func (s *FuncSym) symbolName() string    { return s.Name }
func (s *TypeSym) symbolName() string    { return s.Name }
func (s *GenericSym) symbolName() string { return s.Name }
func (s *BuiltinSym) symbolName() string { return s.Name }

// scope is one level of name bindings.
type scope struct {
	parent *scope
	syms   map[string]Symbol
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent, syms: map[string]Symbol{}}
}

func (s *scope) lookup(name string) Symbol {
	for ; s != nil; s = s.parent {
		if sym, ok := s.syms[name]; ok {
			return sym
		}
	}
	return nil
}

var builtinNames = map[string]BuiltinID{
	"print": BuiltinPrint,
	"panic": BuiltinPanic,
	"len":   BuiltinLen,
	"push":  BuiltinPush,
	"wrap":  BuiltinWrap,
	"cause": BuiltinCause,
	"Ok":    BuiltinOk,
	"Err":   BuiltinErr,
	"Some":  BuiltinSome,
	"None":  BuiltinNone,
}

// newUniverse builds the outermost scope. Each check gets its own, because
// the Error interface records the impls declared by the program.
func newUniverse() (*scope, *Interface) {
	u := newScope(nil)
	for _, b := range basics {
		u.syms[b.Name] = &TypeSym{Name: b.Name, Type: b}
	}
	errIface := &Interface{
		Name:    "Error",
		Methods: []*IfaceMethod{{Name: "message", Sig: &Func{Result: TStr}}},
		Impls:   map[Type]map[string]*Method{},
	}
	u.syms["Error"] = &TypeSym{Name: "Error", Type: errIface}
	for _, name := range []string{"Result", "Maybe", "Map", "Ref"} {
		u.syms[name] = &GenericSym{Name: name}
	}
	for name, id := range builtinNames {
		u.syms[name] = &BuiltinSym{Name: name, ID: id}
	}
	return u, errIface
}
