// Package ast declares the syntax tree produced by the parser.
//
// The tree is purely syntactic: names are not resolved and nothing is
// type-checked. Parentheses written in the source are kept as ParenExpr so
// the printer can reproduce them.
package ast

import "stellar/internal/token"

// Node is implemented by every syntax tree node.
type Node interface {
	Pos() token.Pos
}

// Decl is a top-level declaration.
type Decl interface {
	Node
	declNode()
}

// Stmt is a statement inside a block.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is an expression.
type Expr interface {
	Node
	exprNode()
}

// TypeExpr is a type written in source.
type TypeExpr interface {
	Node
	typeNode()
}

// Pattern is the left-hand side of a match arm.
type Pattern interface {
	Node
	patternNode()
}

// ---------------------------------------------------------------------------
// Files and declarations

// File is one source file, which is also one module.
type File struct {
	Imports []*Import
	Decls   []Decl
}

// Import is `import "path"`.
type Import struct {
	At   token.Pos
	Path *StringLit
}

func (n *Import) Pos() token.Pos { return n.At }

// FuncDecl is a named function, at top level or inside an impl block.
type FuncDecl struct {
	At     token.Pos
	Pub    bool
	Name   *Ident
	Params []*Param
	Result TypeExpr // nil if the function produces no value
	Body   *Block
}

// Param is a function parameter. A receiver is written `self` or
// `mut self`; it has IsSelf set and a nil Type.
type Param struct {
	At      token.Pos
	Name    *Ident
	Type    TypeExpr
	IsSelf  bool
	MutSelf bool
}

func (n *Param) Pos() token.Pos { return n.At }

// ObjectDecl is `type Name object { fields }`.
type ObjectDecl struct {
	At     token.Pos
	Pub    bool
	Name   *Ident
	Fields []*Field
}

// Field is an object field or an enum variant field.
type Field struct {
	At   token.Pos
	Pub  bool
	Name *Ident
	Type TypeExpr
}

func (n *Field) Pos() token.Pos { return n.At }

// InterfaceDecl is `type Name interface { method signatures }`.
type InterfaceDecl struct {
	At      token.Pos
	Pub     bool
	Name    *Ident
	Methods []*FuncSig
}

// FuncSig is a method signature inside an interface.
type FuncSig struct {
	At     token.Pos
	Name   *Ident
	Params []*Param
	Result TypeExpr
}

func (n *FuncSig) Pos() token.Pos { return n.At }

// EnumDecl is `type Name enum { variants }`.
type EnumDecl struct {
	At       token.Pos
	Pub      bool
	Name     *Ident
	Variants []*Variant
}

// Variant is one case of an enum. Fields is nil for a variant without data.
type Variant struct {
	At     token.Pos
	Name   *Ident
	Fields []*Field
}

func (n *Variant) Pos() token.Pos { return n.At }

// ImplDecl is `impl Interface for Target { ... }`, or `impl Target { ... }`
// for inherent methods, in which case Interface is nil.
type ImplDecl struct {
	At        token.Pos
	Interface TypeExpr
	Target    TypeExpr
	Methods   []*FuncDecl
}

func (n *FuncDecl) Pos() token.Pos      { return n.At }
func (n *ObjectDecl) Pos() token.Pos    { return n.At }
func (n *InterfaceDecl) Pos() token.Pos { return n.At }
func (n *EnumDecl) Pos() token.Pos      { return n.At }
func (n *ImplDecl) Pos() token.Pos      { return n.At }

func (*FuncDecl) declNode()      {}
func (*ObjectDecl) declNode()    {}
func (*InterfaceDecl) declNode() {}
func (*EnumDecl) declNode()      {}
func (*ImplDecl) declNode()      {}

// ---------------------------------------------------------------------------
// Types

// NamedType is a possibly module-qualified type name with optional
// bracketed arguments: `int32`, `Book`, `lib.Book`, `Result[int32]`,
// `Map[str, int32]`.
type NamedType struct {
	Module *Ident // nil if unqualified
	Name   *Ident
	Args   []TypeExpr
}

// MutType is `mut T`. It is only valid as the type of a function parameter
// or of a parameter in a function type.
type MutType struct {
	At   token.Pos
	Elem TypeExpr
}

// OptionalType is `?T`, shorthand for `Maybe[T]`.
type OptionalType struct {
	At   token.Pos
	Elem TypeExpr
}

// ArrayType is `[]T`.
type ArrayType struct {
	At   token.Pos
	Elem TypeExpr
}

// FuncType is `fn(T, U) -> R`.
type FuncType struct {
	At     token.Pos
	Params []TypeExpr
	Result TypeExpr // nil if the function produces no value
}

func (n *NamedType) Pos() token.Pos {
	if n.Module != nil {
		return n.Module.Pos()
	}
	return n.Name.Pos()
}
func (n *MutType) Pos() token.Pos      { return n.At }
func (n *OptionalType) Pos() token.Pos { return n.At }
func (n *ArrayType) Pos() token.Pos    { return n.At }
func (n *FuncType) Pos() token.Pos     { return n.At }

func (*NamedType) typeNode()    {}
func (*MutType) typeNode()      {}
func (*OptionalType) typeNode() {}
func (*ArrayType) typeNode()    {}
func (*FuncType) typeNode()     {}

// ---------------------------------------------------------------------------
// Statements

// Block is `{ stmts }`. It is also an expression: in value position its
// value is its last expression.
type Block struct {
	At    token.Pos
	Stmts []Stmt
}

// LetStmt is `let name: T = value` or, with Var set, `var ...`.
type LetStmt struct {
	At    token.Pos
	Var   bool
	Name  *Ident // may be `_`
	Type  TypeExpr
	Value Expr
}

// AssignStmt is `target = value`.
type AssignStmt struct {
	Target Expr
	Value  Expr
}

// ReturnStmt is `return` or `return value`.
type ReturnStmt struct {
	At    token.Pos
	Value Expr // nil for a bare return
}

// BreakStmt is `break`.
type BreakStmt struct{ At token.Pos }

// ContinueStmt is `continue`.
type ContinueStmt struct{ At token.Pos }

// WhileStmt is `while cond { body }`.
type WhileStmt struct {
	At   token.Pos
	Cond Expr
	Body *Block
}

// ForStmt is `for name in iter { body }`.
type ForStmt struct {
	At   token.Pos
	Var  *Ident
	Iter Expr
	Body *Block
}

// ExprStmt is an expression used as a statement.
type ExprStmt struct {
	X Expr
}

func (n *Block) Pos() token.Pos        { return n.At }
func (n *LetStmt) Pos() token.Pos      { return n.At }
func (n *AssignStmt) Pos() token.Pos   { return n.Target.Pos() }
func (n *ReturnStmt) Pos() token.Pos   { return n.At }
func (n *BreakStmt) Pos() token.Pos    { return n.At }
func (n *ContinueStmt) Pos() token.Pos { return n.At }
func (n *WhileStmt) Pos() token.Pos    { return n.At }
func (n *ForStmt) Pos() token.Pos      { return n.At }
func (n *ExprStmt) Pos() token.Pos     { return n.X.Pos() }

func (*LetStmt) stmtNode()      {}
func (*AssignStmt) stmtNode()   {}
func (*ReturnStmt) stmtNode()   {}
func (*BreakStmt) stmtNode()    {}
func (*ContinueStmt) stmtNode() {}
func (*WhileStmt) stmtNode()    {}
func (*ForStmt) stmtNode()      {}
func (*ExprStmt) stmtNode()     {}

// ---------------------------------------------------------------------------
// Expressions

// Ident is a name.
type Ident struct {
	At   token.Pos
	Name string
}

// IntLit is an integer literal; Raw is its source text.
type IntLit struct {
	At  token.Pos
	Raw string
}

// FloatLit is a float literal; Raw is its source text.
type FloatLit struct {
	At  token.Pos
	Raw string
}

// StringLit is a string literal. Raw includes the quotes; Value is decoded.
type StringLit struct {
	At    token.Pos
	Raw   string
	Value string
}

// BoolLit is `true` or `false`.
type BoolLit struct {
	At    token.Pos
	Value bool
}

// BinaryExpr is `X op Y`, including the range operator `..`.
type BinaryExpr struct {
	OpPos token.Pos
	Op    token.Kind
	X, Y  Expr
}

// UnaryExpr is `-X` or `not X`.
type UnaryExpr struct {
	At token.Pos
	Op token.Kind
	X  Expr
}

// ParenExpr is `(X)`.
type ParenExpr struct {
	At token.Pos
	X  Expr
}

// CallExpr is `Fun(args)`.
type CallExpr struct {
	Fun  Expr
	Args []*Arg
}

// Arg is a call argument, optionally named: `r: 1.5`.
type Arg struct {
	Name  *Ident // nil for a positional argument
	Value Expr
}

func (n *Arg) Pos() token.Pos {
	if n.Name != nil {
		return n.Name.Pos()
	}
	return n.Value.Pos()
}

// SelectorExpr is `X.Sel`.
type SelectorExpr struct {
	X   Expr
	Sel *Ident
}

// IndexExpr is `X[Index]`.
type IndexExpr struct {
	X     Expr
	Index Expr
}

// ArrayLit is `[a, b, c]`.
type ArrayLit struct {
	At    token.Pos
	Elems []Expr
}

// MapLit is `[k: v, ...]`, or `[:]` for an empty map.
type MapLit struct {
	At      token.Pos
	Entries []*MapEntry
}

// MapEntry is one `key: value` pair of a map literal.
type MapEntry struct {
	Key   Expr
	Value Expr
}

func (n *MapEntry) Pos() token.Pos { return n.Key.Pos() }

// ObjectLit is `Type{ field: value, ... }`. Type is an Ident or a
// module-qualified SelectorExpr.
type ObjectLit struct {
	Type   Expr
	Fields []*FieldInit
}

// FieldInit is one `name: value` entry of an object literal.
type FieldInit struct {
	Name  *Ident
	Value Expr
}

func (n *FieldInit) Pos() token.Pos { return n.Name.Pos() }

// FuncLit is an anonymous function: `fn(x: int32) -> int32 { ... }`.
type FuncLit struct {
	At     token.Pos
	Params []*Param
	Result TypeExpr
	Body   *Block
}

// IfExpr is `if cond { ... } else ...`. Else is nil, a *Block, or an
// *IfExpr for `else if`.
type IfExpr struct {
	At   token.Pos
	Cond Expr
	Then *Block
	Else Expr
}

// MatchExpr is `match subject { arms }`.
type MatchExpr struct {
	At      token.Pos
	Subject Expr
	Arms    []*MatchArm
}

// MatchArm is `pattern => body`. Body may be a *Block.
type MatchArm struct {
	Pattern Pattern
	Body    Expr
}

func (n *MatchArm) Pos() token.Pos { return n.Pattern.Pos() }

// TryExpr is `try X`, optionally followed by `else Else` or
// `else |ErrName| Else`. Else may be a *Block.
type TryExpr struct {
	At      token.Pos
	X       Expr
	ErrName *Ident // nil unless the else clause binds the error
	Else    Expr   // nil if there is no else clause
}

// MutExpr is `mut X`, marking that X may be changed: an argument passed to
// a mut parameter (`rename(mut b, "x")`) or the receiver of a mut self
// method call (`mut b.retitle("x")`, where X is the call).
type MutExpr struct {
	At token.Pos
	X  Expr
}

// IsExpr is `X is Type` or `X is Type as Name`.
type IsExpr struct {
	X    Expr
	Type TypeExpr
	As   *Ident // nil if no binding
}

func (n *Ident) Pos() token.Pos        { return n.At }
func (n *IntLit) Pos() token.Pos       { return n.At }
func (n *FloatLit) Pos() token.Pos     { return n.At }
func (n *StringLit) Pos() token.Pos    { return n.At }
func (n *BoolLit) Pos() token.Pos      { return n.At }
func (n *BinaryExpr) Pos() token.Pos   { return n.X.Pos() }
func (n *UnaryExpr) Pos() token.Pos    { return n.At }
func (n *ParenExpr) Pos() token.Pos    { return n.At }
func (n *CallExpr) Pos() token.Pos     { return n.Fun.Pos() }
func (n *SelectorExpr) Pos() token.Pos { return n.X.Pos() }
func (n *IndexExpr) Pos() token.Pos    { return n.X.Pos() }
func (n *ArrayLit) Pos() token.Pos     { return n.At }
func (n *MapLit) Pos() token.Pos       { return n.At }
func (n *ObjectLit) Pos() token.Pos    { return n.Type.Pos() }
func (n *FuncLit) Pos() token.Pos      { return n.At }
func (n *IfExpr) Pos() token.Pos       { return n.At }
func (n *MatchExpr) Pos() token.Pos    { return n.At }
func (n *TryExpr) Pos() token.Pos      { return n.At }
func (n *IsExpr) Pos() token.Pos       { return n.X.Pos() }
func (n *MutExpr) Pos() token.Pos      { return n.At }

func (*Block) exprNode()        {}
func (*Ident) exprNode()        {}
func (*IntLit) exprNode()       {}
func (*FloatLit) exprNode()     {}
func (*StringLit) exprNode()    {}
func (*BoolLit) exprNode()      {}
func (*BinaryExpr) exprNode()   {}
func (*UnaryExpr) exprNode()    {}
func (*ParenExpr) exprNode()    {}
func (*CallExpr) exprNode()     {}
func (*SelectorExpr) exprNode() {}
func (*IndexExpr) exprNode()    {}
func (*ArrayLit) exprNode()     {}
func (*MapLit) exprNode()       {}
func (*ObjectLit) exprNode()    {}
func (*FuncLit) exprNode()      {}
func (*IfExpr) exprNode()       {}
func (*MatchExpr) exprNode()    {}
func (*TryExpr) exprNode()      {}
func (*IsExpr) exprNode()       {}
func (*MutExpr) exprNode()      {}

// ---------------------------------------------------------------------------
// Patterns

// WildcardPattern is `_`.
type WildcardPattern struct{ At token.Pos }

// TypePattern is `is Type` or `is Type as Name`.
type TypePattern struct {
	At   token.Pos
	Type TypeExpr
	As   *Ident
}

// VariantPattern matches an enum variant or a built-in Result/Maybe case:
// `Shape.Empty`, `Shape.Rect(w, h)`, `Shape.Rect(w: width)`, `Ok(v)`,
// `None`. Name is an Ident or SelectorExpr. Bindings is nil when the pattern
// has no parentheses.
type VariantPattern struct {
	Name     Expr
	Bindings []*Binding
}

// Binding binds a variant field. Field is the field name (or, for the
// built-in single-value variants, the new variable); Name renames it and is
// nil when the variable takes the field's name.
type Binding struct {
	Field *Ident
	Name  *Ident
}

func (n *Binding) Pos() token.Pos { return n.Field.Pos() }

func (n *WildcardPattern) Pos() token.Pos { return n.At }
func (n *TypePattern) Pos() token.Pos     { return n.At }
func (n *VariantPattern) Pos() token.Pos  { return n.Name.Pos() }

func (*WildcardPattern) patternNode() {}
func (*TypePattern) patternNode()     {}
func (*VariantPattern) patternNode()  {}
