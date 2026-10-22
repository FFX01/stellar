package ast

import (
	"fmt"
	"stellar/internal/lexer"
	"strconv"
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
func (i *IdentifierExpr) expr()                {}
func (i *IdentifierExpr) String() string       { return i.Value }

type SequenceLiteral struct {
	Token    lexer.Token
	Elements []Expression
}

func (s *SequenceLiteral) TokenLiteral() string { return s.Token.Literal }
func (s *SequenceLiteral) expr()                {}
func (s *SequenceLiteral) String() string {
	return fmt.Sprintf("SequenceLiteral (%s)", sliceToString(s.Elements))
}

type Name struct {
	Token lexer.Token
	Value string
}

func (n *Name) TokenLiteral() string { return n.Token.Literal }
func (n *Name) String() string {
	return fmt.Sprintf(
		"Name(%s)",
		n.Value,
	)
}

type BlockExpr struct {
	Token      lexer.Token
	Statements []Statement
}

func (b *BlockExpr) TokenLiteral() string { return b.Token.Literal }
func (b *BlockExpr) expr()                {}
func (b *BlockExpr) String() string {
	return fmt.Sprintf(
		"BlockExpr(%s)",
		sliceToString(b.Statements),
	)
}

type TryExpr struct {
	Token     lexer.Token
	Value     Expression
	ErrName   string
	ElseValue Expression
}

func (t *TryExpr) TokenLiteral() string { return t.Token.Literal }
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
func (i *IntegerLiteralExpr) expr()                {}
func (i *IntegerLiteralExpr) String() string       { return i.Token.Literal }

type FloatLiteralExpr struct {
	Token lexer.Token
	Value float64
}

func (f *FloatLiteralExpr) TokenLiteral() string { return f.Token.Literal }
func (f *FloatLiteralExpr) expr()                {}
func (f *FloatLiteralExpr) String() string       { return f.Token.Literal }

type Argument struct {
	Token lexer.Token
	Name  string
	Value Expression
}

func (a *Argument) TokenLiteral() string { return a.Token.Literal }
func (a *Argument) String() string {
	return fmt.Sprintf(
		"Argument (%s, %s)",
		a.Name,
		pointerToString(a.Value),
	)
}

type CallExpr struct {
	Token lexer.Token
	Left  Expression
	Args  []*Argument
}

func (c *CallExpr) TokenLiteral() string { return c.Token.Literal }
func (c *CallExpr) expr()                {}
func (c *CallExpr) String() string {
	return fmt.Sprintf(
		"CallExpr (%s, %s)",
		pointerToString(c.Left),
		sliceToString(c.Args),
	)
}

type InfixExpr struct {
	Token    lexer.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (i *InfixExpr) TokenLiteral() string { return i.Token.Literal }
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
func (s *SwitchExpr) expr()                {}
func (s *SwitchExpr) String() string {
	return fmt.Sprintf(
		"SwitchExpr(%s, [%s])",
		pointerToString(s.Subject),
		sliceToString(s.Arms),
	)
}

type NewExpr struct {
	Token lexer.Token
	Type  Type
}

func (n *NewExpr) TokenLiteral() string { return n.Token.Literal }
func (n *NewExpr) expr()                {}
func (n *NewExpr) String() string {
	return fmt.Sprintf(
		"NewExpr(%s)",
		pointerToString(n.Type),
	)
}

type IfExpr struct {
	Token     lexer.Token
	Condition Expression
	Then      Expression
	Else      Expression
}

func (i *IfExpr) TokenLiteral() string { return i.Token.Literal }
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
func (s *SliceType) typ()                 {}
func (s *SliceType) String() string {
	return fmt.Sprintf("SliceType (%s)", pointerToString(s.ElementType))
}

type ArrayType struct {
	Token       lexer.Token
	ElementType Type
	Length      int64
}

func (a *ArrayType) TokenLiteral() string { return a.Token.Literal }
func (a *ArrayType) typ()                 {}
func (a *ArrayType) String() string {
	v := strconv.FormatInt(a.Length, 10)
	return fmt.Sprintf("ArrayType (%s, %s)", pointerToString(a.ElementType), v)
}

type PointerType struct {
	Token       lexer.Token
	ElementType Type
}

func (p *PointerType) TokenLiteral() string { return p.Token.Literal }
func (p *PointerType) typ()                 {}
func (p *PointerType) String() string {
	return fmt.Sprintf("PointerType (%s)", pointerToString(p.ElementType))
}

type NotExpr struct {
	Token lexer.Token
	Value Expression
}

func (n *NotExpr) TokenLiteral() string { return n.Token.Literal }
func (n *NotExpr) expr()                {}
func (n *NotExpr) String() string {
	return fmt.Sprintf(
		"NotExpr(%s)",
		pointerToString(n.Value),
	)
}

type NegativeExpr struct {
	Token lexer.Token
	Value Expression
}

func (n *NegativeExpr) TokenLiteral() string { return n.Token.Literal }
func (n *NegativeExpr) expr()                {}
func (n *NegativeExpr) String() string {
	return fmt.Sprintf(
		"NegativeExpr(%s)",
		pointerToString(n.Value),
	)
}

type NamedType struct {
	Token lexer.Token
	Path  []string
	Name  string
	Args  []Type
}

func (n *NamedType) TokenLiteral() string { return n.Token.Literal }
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
	Token     lexer.Token
	Interface Type
	Target    Type
	Methods   []*FunctionStmt
}

func (i *ImplDeclStmt) TokenLiteral() string { return i.Token.Literal }
func (i *ImplDeclStmt) stmt()                {}
func (i *ImplDeclStmt) String() string {
	return fmt.Sprintf(
		"ImplDeclStmt(%s -> %s, %s)",
		pointerToString(i.Interface),
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
func (e *EnumMember) String() string {
	return fmt.Sprintf("EnumMember (%s, %s)", e.Name, sliceToString(e.Fields))
}

type EnumType struct {
	Token   lexer.Token
	Members []*EnumMember
}

func (e *EnumType) TokenLiteral() string { return e.Token.Literal }
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
	IsPublic bool
}

func (v *VarStmt) TokenLiteral() string { return v.Token.Literal }
func (v *VarStmt) stmt()                {}
func (v *VarStmt) String() string {
	return fmt.Sprintf(
		"VarStmt (%v, %s, %s, %s)",
		v.IsPublic,
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
func (f *FunctionSignature) String() string {
	return fmt.Sprintf(
		"FunctionSignature (%s, %s, %s, %s)",
		f.Name,
		pointerToString(f.ReturnType),
		sliceToString(f.Parameters),
		sliceToString(f.TypeParams),
	)
}

type FunctionStmt struct {
	Signature *FunctionSignature
	Body      Expression
	IsPublic  bool
}

func (f *FunctionStmt) TokenLiteral() string { return f.Signature.Token.Literal }
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
func (b *BreakStmt) stmt()                {}
func (b *BreakStmt) String() string {
	return "BreakStmt"
}

type ContinueStmt struct {
	Token lexer.Token
}

func (c *ContinueStmt) TokenLiteral() string { return c.Token.Literal }
func (c *ContinueStmt) stmt()                {}
func (c *ContinueStmt) String() string       { return "ContinueStmt" }

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}
func (p *Program) String() string {
	builder := new(strings.Builder)
	for _, stmt := range p.Statements {
		builder.WriteString(stmt.String())
		builder.WriteString("\n")
	}
	return builder.String()
}
