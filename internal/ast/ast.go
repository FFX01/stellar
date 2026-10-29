package ast

import (
	"fmt"
	"stellar/internal/lexer"
	"strings"
)

func sliceToString[T Node](nodes []T) string {
	var l []string
	for _, n := range nodes {
		l = append(l, n.String())
	}
	s := strings.Join(l, ", ")
	return fmt.Sprintf("[%s]", s)
}

func pointerToString(node Node) string {
	if node != nil {
		return node.String()
	}
	return "missing"
}

type Node interface {
	TokenLiteral() string
	String() string
	// Pos returns the source position where the node starts.
	Pos() lexer.Position
}

// startPos returns the start position of n, or the fallback token's position
// when n is missing.
func startPos(n Node, fallback lexer.Token) lexer.Position {
	if n == nil {
		return fallback.Position
	}
	return n.Pos()
}

type Statement interface {
	Node
	stmt()
}

type Expression interface {
	Node
	expr()
}

type Type interface {
	Node
	typ()
}

type ExpressionStatement struct {
	Token      lexer.Token
	Expression Expression
}

func (e *ExpressionStatement) TokenLiteral() string { return e.Token.Literal }
func (e *ExpressionStatement) Pos() lexer.Position  { return e.Token.Position }
func (e *ExpressionStatement) stmt()                {}
func (e *ExpressionStatement) String() string {
	return fmt.Sprintf(
		"ExpressionStatement (%s)",
		pointerToString(e.Expression),
	)
}

type IdentifierExpr struct {
	Token lexer.Token
	Value string
}

func (i *IdentifierExpr) TokenLiteral() string { return i.Token.Literal }
func (i *IdentifierExpr) Pos() lexer.Position  { return i.Token.Position }
func (i *IdentifierExpr) expr()                {}
func (i *IdentifierExpr) String() string       { return i.Value }

type SequenceLiteral struct {
	Token    lexer.Token
	Elements []Expression
}

func (s *SequenceLiteral) TokenLiteral() string { return s.Token.Literal }
func (s *SequenceLiteral) Pos() lexer.Position  { return s.Token.Position }
func (s *SequenceLiteral) expr()                {}
func (s *SequenceLiteral) String() string {
	return fmt.Sprintf("SequenceLiteral (%s)", sliceToString(s.Elements))
}

type BlockExpr struct {
	Token      lexer.Token
	Statements []Statement
}

func (b *BlockExpr) TokenLiteral() string { return b.Token.Literal }
func (b *BlockExpr) Pos() lexer.Position  { return b.Token.Position }
func (b *BlockExpr) expr()                {}
func (b *BlockExpr) String() string {
	return fmt.Sprintf(
		"BlockExpr(%s)",
		sliceToString(b.Statements),
	)
}

type MustExpr struct {
	Token lexer.Token
	Value Expression
}

func (m *MustExpr) TokenLiteral() string { return m.Token.Literal }
func (m *MustExpr) Pos() lexer.Position  { return m.Token.Position }
func (m *MustExpr) expr()                {}
func (m *MustExpr) String() string {
	return fmt.Sprintf(
		"MustExpr(%s)",
		pointerToString(m.Value),
	)
}

type TryExpr struct {
	Token     lexer.Token
	Value     Expression
	ErrName   string
	ElseValue Expression
}

func (t *TryExpr) TokenLiteral() string { return t.Token.Literal }
func (t *TryExpr) Pos() lexer.Position  { return t.Token.Position }
func (t *TryExpr) expr()                {}
func (t *TryExpr) String() string {
	return fmt.Sprintf(
		"TryExpr(%s, %s, %s)",
		pointerToString(t.Value),
		t.ErrName,
		pointerToString(t.ElseValue),
	)
}

type StringLiteralExpr struct {
	Token lexer.Token
	Value string
}

func (s *StringLiteralExpr) TokenLiteral() string { return s.Token.Literal }
func (s *StringLiteralExpr) Pos() lexer.Position  { return s.Token.Position }
func (s *StringLiteralExpr) expr()                {}
func (s *StringLiteralExpr) String() string {
	return fmt.Sprintf("StringLiteralExpr (%s)", s.Value)
}

type MemberExpr struct {
	Token     lexer.Token
	Container Expression
	Member    string
}

func (m *MemberExpr) TokenLiteral() string { return m.Token.Literal }
func (m *MemberExpr) Pos() lexer.Position  { return startPos(m.Container, m.Token) }
func (m *MemberExpr) expr()                {}
func (m *MemberExpr) String() string {
	return fmt.Sprintf(
		"MemberExpr(%s, %s)",
		pointerToString(m.Container),
		m.Member,
	)
}

type BoolLiteralExpr struct {
	Token lexer.Token
	Value bool
}

func (b *BoolLiteralExpr) TokenLiteral() string { return b.Token.Literal }
func (b *BoolLiteralExpr) Pos() lexer.Position  { return b.Token.Position }
func (b *BoolLiteralExpr) expr()                {}
func (b *BoolLiteralExpr) String() string {
	return fmt.Sprintf("BoolLiteralExpr (%v)", b.Value)
}

type ForInExpr struct {
	Token        lexer.Token
	IndexBinding lexer.Token
	ValueBinding lexer.Token
	Container    Expression
	Body         Expression
}

func (f *ForInExpr) TokenLiteral() string { return f.Token.Literal }
func (f *ForInExpr) Pos() lexer.Position  { return f.Token.Position }
func (f *ForInExpr) expr()                {}
func (f *ForInExpr) String() string {
	return fmt.Sprintf(
		"ForInExpr(%s, %s, %s, %s)",
		f.IndexBinding.Literal,
		f.ValueBinding.Literal,
		pointerToString(f.Container),
		pointerToString(f.Body),
	)
}

type ForWhileExpr struct {
	Token     lexer.Token
	Condition Expression
	Body      Expression
}

func (f *ForWhileExpr) TokenLiteral() string { return f.Token.Literal }
func (f *ForWhileExpr) Pos() lexer.Position  { return f.Token.Position }
func (f *ForWhileExpr) expr()                {}
func (f *ForWhileExpr) String() string {
	return fmt.Sprintf(
		"ForWhileExpr(%s, %s)",
		pointerToString(f.Condition),
		pointerToString(f.Body),
	)
}

type IntegerLiteralExpr struct {
	Token lexer.Token
	Value int64
}

func (i *IntegerLiteralExpr) TokenLiteral() string { return i.Token.Literal }
func (i *IntegerLiteralExpr) Pos() lexer.Position  { return i.Token.Position }
func (i *IntegerLiteralExpr) expr()                {}
func (i *IntegerLiteralExpr) String() string       { return i.Token.Literal }

type FloatLiteralExpr struct {
	Token lexer.Token
	Value float64
}

func (f *FloatLiteralExpr) TokenLiteral() string { return f.Token.Literal }
func (f *FloatLiteralExpr) Pos() lexer.Position  { return f.Token.Position }
func (f *FloatLiteralExpr) expr()                {}
func (f *FloatLiteralExpr) String() string       { return f.Token.Literal }

type Argument struct {
	Token lexer.Token
	Name  string
	Value Expression
}

func (a *Argument) TokenLiteral() string { return a.Token.Literal }
func (a *Argument) Pos() lexer.Position  { return a.Token.Position }
func (a *Argument) String() string {
	return fmt.Sprintf(
		"Argument (%s, %s)",
		a.Name,
		pointerToString(a.Value),
	)
}

type MaybeExpr struct {
	Token  lexer.Token
	IsNone bool
	Value  Expression
}

func (me *MaybeExpr) TokenLiteral() string { return me.Token.Literal }
func (me *MaybeExpr) expr()                {}
func (me *MaybeExpr) Pos() lexer.Position  { return me.Token.Position }
func (me *MaybeExpr) String() string {
	return fmt.Sprintf(
		"MaybeExpr(%v, %s)",
		me.IsNone,
		pointerToString(me.Value),
	)
}

type ResultExpr struct {
	Token   lexer.Token
	IsError bool
	Value   Expression
}

func (r *ResultExpr) TokenLiteral() string { return r.Token.Literal }
func (r *ResultExpr) expr()                {}
func (r *ResultExpr) Pos() lexer.Position  { return r.Token.Position }
func (r *ResultExpr) String() string {
	wrapper := "Ok"
	if r.IsError {
		wrapper = "Err"
	}
	return fmt.Sprintf(
		"ResultExpr(%s(%s))",
		wrapper,
		pointerToString(r.Value),
	)
}

type CallExpr struct {
	Token lexer.Token
	Left  Expression
	Args  []*Argument
}

func (c *CallExpr) TokenLiteral() string { return c.Token.Literal }
func (c *CallExpr) Pos() lexer.Position  { return startPos(c.Left, c.Token) }
func (c *CallExpr) expr()                {}
func (c *CallExpr) String() string {
	return fmt.Sprintf(
		"CallExpr (%s, %s)",
		pointerToString(c.Left),
		sliceToString(c.Args),
	)
}

type SliceExpr struct {
	Token lexer.Token
	Start Expression
	End   Expression
}

func (s *SliceExpr) TokenLiteral() string { return s.Token.Literal }
func (s *SliceExpr) Pos() lexer.Position  { return startPos(s.Start, s.Token) }
func (s *SliceExpr) expr()                {}
func (s *SliceExpr) String() string {
	return fmt.Sprintf(
		"SliceExpr(%s, %s)",
		pointerToString(s.Start),
		pointerToString(s.End),
	)
}

type CopyExpr struct {
	Token lexer.Token
	Value Expression
}

func (g *CopyExpr) TokenLiteral() string { return g.Token.Literal }
func (g *CopyExpr) Pos() lexer.Position  { return g.Token.Position }
func (g *CopyExpr) expr()                {}
func (g *CopyExpr) String() string {
	return fmt.Sprintf(
		"GrantExpr(%s)",
		pointerToString(g.Value),
	)
}

type InfixExpr struct {
	Token    lexer.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (i *InfixExpr) TokenLiteral() string { return i.Token.Literal }
func (i *InfixExpr) Pos() lexer.Position  { return startPos(i.Left, i.Token) }
func (i *InfixExpr) expr()                {}
func (i *InfixExpr) String() string {
	return fmt.Sprintf(
		"InfixExpr (%s, %s, %s)",
		pointerToString(i.Left),
		i.Operator,
		pointerToString(i.Right),
	)
}

type SwitchArm struct {
	Token lexer.Token
	Case  Expression
	Value Expression
}

func (s *SwitchArm) TokenLiteral() string { return s.Token.Literal }
func (s *SwitchArm) Pos() lexer.Position  { return s.Token.Position }
func (s *SwitchArm) String() string {
	return fmt.Sprintf(
		"MatchArm(%s, %s)",
		pointerToString(s.Case),
		pointerToString(s.Value),
	)
}

type SwitchExpr struct {
	Token   lexer.Token
	Subject Expression
	Arms    []*SwitchArm
}

func (s *SwitchExpr) TokenLiteral() string { return s.Token.Literal }
func (s *SwitchExpr) Pos() lexer.Position  { return s.Token.Position }
func (s *SwitchExpr) expr()                {}
func (s *SwitchExpr) String() string {
	return fmt.Sprintf(
		"SwitchExpr(%s, [%s])",
		pointerToString(s.Subject),
		sliceToString(s.Arms),
	)
}

type IfExpr struct {
	Token     lexer.Token
	Condition Expression
	Then      Expression
	Else      Expression
}

func (i *IfExpr) TokenLiteral() string { return i.Token.Literal }
func (i *IfExpr) Pos() lexer.Position  { return i.Token.Position }
func (i *IfExpr) expr()                {}
func (i *IfExpr) String() string {
	return fmt.Sprintf(
		"IfExpr(%s, %s, %s)",
		pointerToString(i.Condition),
		pointerToString(i.Then),
		pointerToString(i.Else),
	)
}

type SubscriptExpr struct {
	Token    lexer.Token
	Receiver Expression
	Argument Expression
}

func (s *SubscriptExpr) TokenLiteral() string { return s.Token.Literal }
func (s *SubscriptExpr) Pos() lexer.Position  { return startPos(s.Receiver, s.Token) }
func (s *SubscriptExpr) expr()                {}
func (s *SubscriptExpr) String() string {
	return fmt.Sprintf(
		"SubscriptExpr(%s, %s)",
		pointerToString(s.Receiver),
		pointerToString(s.Argument),
	)
}

type SliceType struct {
	Token       lexer.Token
	ElementType Type
}

func (s *SliceType) TokenLiteral() string { return s.Token.Literal }
func (s *SliceType) Pos() lexer.Position  { return s.Token.Position }
func (s *SliceType) typ()                 {}
func (s *SliceType) String() string {
	return fmt.Sprintf("SliceType (%s)", pointerToString(s.ElementType))
}

type NotExpr struct {
	Token lexer.Token
	Value Expression
}

func (n *NotExpr) TokenLiteral() string { return n.Token.Literal }
func (n *NotExpr) Pos() lexer.Position  { return n.Token.Position }
func (n *NotExpr) expr()                {}
func (n *NotExpr) String() string {
	return fmt.Sprintf(
		"NotExpr(%s)",
		pointerToString(n.Value),
	)
}

type SpreadExpr struct {
	Token lexer.Token
	Value Expression
}

func (se *SpreadExpr) TokenLiteral() string { return se.Token.Literal }
func (se *SpreadExpr) expr()                {}
func (se *SpreadExpr) Pos() lexer.Position  { return se.Token.Position }
func (se *SpreadExpr) String() string {
	return fmt.Sprintf("SpreadExpr(%s)", pointerToString(se.Value))
}

type NegativeExpr struct {
	Token lexer.Token
	Value Expression
}

func (n *NegativeExpr) TokenLiteral() string { return n.Token.Literal }
func (n *NegativeExpr) Pos() lexer.Position  { return n.Token.Position }
func (n *NegativeExpr) expr()                {}
func (n *NegativeExpr) String() string {
	return fmt.Sprintf(
		"NegativeExpr(%s)",
		pointerToString(n.Value),
	)
}

type MaybeType struct {
	Token lexer.Token
	Value Type
}

func (mt *MaybeType) TokenLiteral() string { return mt.Token.Literal }
func (mt *MaybeType) Pos() lexer.Position  { return mt.Token.Position }
func (mt *MaybeType) typ()                 {}
func (mt *MaybeType) String() string {
	return fmt.Sprintf(
		"MaybeType(%s)",
		pointerToString(mt.Value),
	)
}

type ResultType struct {
	Token lexer.Token
	Value Type
}

func (r *ResultType) TokenLiteral() string { return r.Token.Literal }
func (r *ResultType) Pos() lexer.Position  { return r.Token.Position }
func (r *ResultType) typ()                 {}
func (r *ResultType) String() string {
	return fmt.Sprintf(
		"ResultType(%s)",
		pointerToString(r.Value),
	)
}

type NamedType struct {
	Token lexer.Token
	Path  []string
	Name  string
	Args  []Type
}

func (n *NamedType) TokenLiteral() string { return n.Token.Literal }
func (n *NamedType) Pos() lexer.Position  { return n.Token.Position }
func (n *NamedType) typ()                 {}
func (n *NamedType) String() string {
	return fmt.Sprintf(
		"NamedType ([%s], %s, %s)",
		strings.Join(n.Path, ", "),
		n.Name,
		sliceToString(n.Args),
	)
}

type ImplDeclStmt struct {
	Token   lexer.Token
	Target  Type
	Methods []*FunctionStmt
}

func (i *ImplDeclStmt) TokenLiteral() string { return i.Token.Literal }
func (i *ImplDeclStmt) Pos() lexer.Position  { return i.Token.Position }
func (i *ImplDeclStmt) stmt()                {}
func (i *ImplDeclStmt) String() string {
	return fmt.Sprintf(
		"ImplDeclStmt(%s, %s)",
		pointerToString(i.Target),
		sliceToString(i.Methods),
	)
}

type TypeDeclStmt struct {
	Token      lexer.Token
	Name       string
	IsPublic   bool
	Definition Type
	TypeParams []*TypeParameter
}

func (t *TypeDeclStmt) TokenLiteral() string { return t.Token.Literal }
func (t *TypeDeclStmt) Pos() lexer.Position  { return t.Token.Position }
func (t *TypeDeclStmt) stmt()                {}
func (t *TypeDeclStmt) String() string {
	return fmt.Sprintf(
		"TypeDeclStmt (%v, %s, %s, %s)",
		t.IsPublic,
		t.Name,
		pointerToString(t.Definition),
		sliceToString(t.TypeParams),
	)
}

type Field struct {
	Token    lexer.Token
	Name     string
	Type     Type
	IsPublic bool
}

func (f *Field) TokenLiteral() string { return f.Token.Literal }
func (f *Field) Pos() lexer.Position  { return f.Token.Position }
func (f *Field) String() string {
	typestr := "missing"
	if f.Type != nil {
		typestr = f.Type.String()
	}
	pubstr := ""
	if f.IsPublic {
		pubstr = "pub "
	}
	return fmt.Sprintf("Field %s(%s, %s)", pubstr, f.Name, typestr)
}

type InterfaceType struct {
	Token   lexer.Token
	Methods []*FunctionSignature
}

func (i *InterfaceType) TokenLiteral() string { return i.Token.Literal }
func (i *InterfaceType) Pos() lexer.Position  { return i.Token.Position }
func (i *InterfaceType) typ()                 {}
func (i *InterfaceType) String() string {
	return fmt.Sprintf("InterfaceType (%s)", sliceToString(i.Methods))
}

type EnumMember struct {
	Token  lexer.Token
	Name   string
	Fields []*Field
}

func (e *EnumMember) TokenLiteral() string { return e.Token.Literal }
func (e *EnumMember) Pos() lexer.Position  { return e.Token.Position }
func (e *EnumMember) String() string {
	return fmt.Sprintf("EnumMember (%s, %s)", e.Name, sliceToString(e.Fields))
}

type EnumType struct {
	Token   lexer.Token
	Members []*EnumMember
}

func (e *EnumType) TokenLiteral() string { return e.Token.Literal }
func (e *EnumType) Pos() lexer.Position  { return e.Token.Position }
func (e *EnumType) typ()                 {}
func (e *EnumType) String() string {
	return fmt.Sprintf(
		"EnumType (%s)",
		sliceToString(e.Members),
	)
}

type ObjectType struct {
	Token  lexer.Token
	Fields []*Field
}

func (o *ObjectType) TokenLiteral() string { return o.Token.Literal }
func (o *ObjectType) Pos() lexer.Position  { return o.Token.Position }
func (o *ObjectType) typ()                 {}
func (o *ObjectType) String() string {
	return fmt.Sprintf(
		"ObjectType (%s)",
		sliceToString(o.Fields),
	)
}

type FunctionType struct {
	Token      lexer.Token
	Parameters []*Parameter
	ReturnType Type
}

func (f *FunctionType) TokenLiteral() string { return f.Token.Literal }
func (f *FunctionType) Pos() lexer.Position  { return f.Token.Position }
func (f *FunctionType) typ()                 {}
func (f *FunctionType) String() string {
	return fmt.Sprintf(
		"FunctionType (%s, [%s])",
		pointerToString(f.ReturnType),
		sliceToString(f.Parameters),
	)
}

type TypeParameter struct {
	Token lexer.Token
}

func (t *TypeParameter) TokenLiteral() string { return t.Token.Literal }
func (t *TypeParameter) Pos() lexer.Position  { return t.Token.Position }
func (t *TypeParameter) String() string {
	return fmt.Sprintf("TypeParameter (%s)", t.Token.Literal)
}

type TypeAliasStmt struct {
	Token      lexer.Token
	Name       string
	Value      Type
	IsPublic   bool
	TypeParams []*TypeParameter
}

func (t *TypeAliasStmt) TokenLiteral() string { return t.Token.Literal }
func (t *TypeAliasStmt) Pos() lexer.Position  { return t.Token.Position }
func (t *TypeAliasStmt) stmt()                {}
func (t *TypeAliasStmt) String() string {
	return fmt.Sprintf(
		"TypeAliasStmt (%v, %s, %s, %s)",
		t.IsPublic,
		t.Name,
		pointerToString(t.Value),
		sliceToString(t.TypeParams),
	)
}

type AssignStmt struct {
	Token  lexer.Token
	Target Expression
	Value  Expression
}

func (a *AssignStmt) TokenLiteral() string { return a.Token.Literal }
func (a *AssignStmt) Pos() lexer.Position  { return startPos(a.Target, a.Token) }
func (a *AssignStmt) stmt()                {}
func (a *AssignStmt) String() string {
	return fmt.Sprintf(
		"AssignStmt(%s, %s)",
		pointerToString(a.Target),
		pointerToString(a.Value),
	)
}

type VarStmt struct {
	Token    lexer.Token
	Name     lexer.Token
	Value    Expression
	Type     Type
	IsMut    bool
	IsPublic bool
}

func (v *VarStmt) TokenLiteral() string { return v.Token.Literal }
func (v *VarStmt) Pos() lexer.Position  { return v.Token.Position }
func (v *VarStmt) stmt()                {}
func (v *VarStmt) String() string {
	return fmt.Sprintf(
		"VarStmt (%v, %v, %s, %s, %s)",
		v.IsPublic,
		v.IsMut,
		v.Name.Literal,
		pointerToString(v.Type),
		v.Value.String(),
	)
}

type Parameter struct {
	Token lexer.Token
	Name  string
	Type  Type
}

func (p *Parameter) TokenLiteral() string { return p.Token.Literal }
func (p *Parameter) Pos() lexer.Position  { return p.Token.Position }
func (p *Parameter) String() string {
	typeString := "none"
	if p.Type != nil {
		typeString = p.Type.String()
	}
	return fmt.Sprintf(
		"Parameter (%s, %s)",
		p.Name,
		typeString,
	)
}

type ImportStmt struct {
	Token lexer.Token
	Path  []lexer.Token
}

func (i *ImportStmt) TokenLiteral() string { return i.Token.Literal }
func (i *ImportStmt) Pos() lexer.Position  { return i.Token.Position }
func (i *ImportStmt) stmt()                {}
func (i *ImportStmt) String() string {
	var tokenStrings []string
	for _, t := range i.Path {
		tokenStrings = append(tokenStrings, t.Literal)
	}
	pathStr := strings.Join(tokenStrings, ", ")
	return fmt.Sprintf("ImportStmt (%s)", pathStr)
}

type FunctionSignature struct {
	Token      lexer.Token
	Name       string
	ReturnType Type
	Parameters []*Parameter
	TypeParams []*TypeParameter
}

func (f *FunctionSignature) TokenLiteral() string { return f.Token.Literal }
func (f *FunctionSignature) Pos() lexer.Position  { return f.Token.Position }
func (f *FunctionSignature) String() string {
	return fmt.Sprintf(
		"FunctionSignature (%s, %s, %s, %s)",
		f.Name,
		pointerToString(f.ReturnType),
		sliceToString(f.Parameters),
		sliceToString(f.TypeParams),
	)
}

type FunctionExpr struct {
	Signature *FunctionSignature
	Body      Expression
}

func (f *FunctionExpr) TokenLiteral() string { return f.Signature.TokenLiteral() }
func (f *FunctionExpr) Pos() lexer.Position  { return f.Signature.Pos() }
func (f *FunctionExpr) expr()                {}
func (f *FunctionExpr) String() string {
	return fmt.Sprintf(
		"FunctionExpr(%s, %s)",
		pointerToString(f.Signature),
		pointerToString(f.Body),
	)
}

type FunctionStmt struct {
	Signature *FunctionSignature
	Body      Expression
	IsPublic  bool
}

func (f *FunctionStmt) TokenLiteral() string { return f.Signature.Token.Literal }
func (f *FunctionStmt) Pos() lexer.Position  { return f.Signature.Pos() }
func (f *FunctionStmt) stmt()                {}
func (f *FunctionStmt) String() string {
	return fmt.Sprintf(
		"FunctionStmt (%v, %s, %s)",
		f.IsPublic,
		pointerToString(f.Signature),
		pointerToString(f.Body),
	)
}

type EmitStmt struct {
	Token lexer.Token
	Value Expression
}

func (e *EmitStmt) TokenLiteral() string { return e.Token.Literal }
func (e *EmitStmt) Pos() lexer.Position  { return e.Token.Position }
func (e *EmitStmt) stmt()                {}
func (e *EmitStmt) String() string {
	return fmt.Sprintf(
		"EmitStmt(%s)",
		pointerToString(e.Value),
	)
}

type ReturnStmt struct {
	Token lexer.Token
	Value Expression
}

func (r *ReturnStmt) TokenLiteral() string { return r.Token.Literal }
func (r *ReturnStmt) Pos() lexer.Position  { return r.Token.Position }
func (r *ReturnStmt) stmt()                {}
func (r *ReturnStmt) String() string {
	return fmt.Sprintf(
		"ReturnStmt (%s)",
		pointerToString(r.Value),
	)
}

type BreakStmt struct {
	Token lexer.Token
}

func (b *BreakStmt) TokenLiteral() string { return b.Token.Literal }
func (b *BreakStmt) Pos() lexer.Position  { return b.Token.Position }
func (b *BreakStmt) stmt()                {}
func (b *BreakStmt) String() string {
	return "BreakStmt"
}

type ContinueStmt struct {
	Token lexer.Token
}

func (c *ContinueStmt) TokenLiteral() string { return c.Token.Literal }
func (c *ContinueStmt) Pos() lexer.Position  { return c.Token.Position }
func (c *ContinueStmt) stmt()                {}
func (c *ContinueStmt) String() string       { return "ContinueStmt" }

type BadStmt struct {
	From lexer.Token
	To   lexer.Token
}

func (b *BadStmt) TokenLiteral() string { return b.From.Literal }
func (b *BadStmt) Pos() lexer.Position  { return b.From.Position }
func (b *BadStmt) stmt()                {}
func (b *BadStmt) String() string {
	return fmt.Sprintf(
		"BadStmt(%s, %s)",
		b.From.Literal,
		b.To.Literal,
	)
}

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}
func (p *Program) Pos() lexer.Position {
	if len(p.Statements) > 0 {
		return p.Statements[0].Pos()
	}
	return lexer.Position{}
}
func (p *Program) String() string {
	builder := new(strings.Builder)
	for _, stmt := range p.Statements {
		builder.WriteString(stmt.String())
		builder.WriteString("\n")
	}
	return builder.String()
}
