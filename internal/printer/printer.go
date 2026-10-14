// Package printer renders a syntax tree as canonical Stellar source.
//
// Output uses four-space indentation and one declaration per paragraph.
// Comments are not part of the syntax tree, so they are not reproduced.
package printer

import (
	"fmt"
	"strings"

	"stellar/internal/ast"
	"stellar/internal/token"
)

const indentUnit = "    "

// Options controls rendering.
type Options struct {
	// Parenthesize wraps every operator expression in parentheses, making
	// precedence and associativity visible. Intended for tests.
	Parenthesize bool
}

// File renders a whole file.
func File(f *ast.File) string {
	return FileWith(f, Options{})
}

// FileWith renders a whole file using opts.
func FileWith(f *ast.File, opts Options) string {
	p := &printer{opts: opts}
	p.file(f)
	return p.b.String()
}

// Expr renders a single expression.
func Expr(x ast.Expr) string {
	return ExprWith(x, Options{})
}

// ExprWith renders a single expression using opts.
func ExprWith(x ast.Expr, opts Options) string {
	p := &printer{opts: opts}
	p.expr(x)
	return p.b.String()
}

type printer struct {
	b      strings.Builder
	indent int
	opts   Options
}

func (p *printer) print(parts ...string) {
	for _, s := range parts {
		p.b.WriteString(s)
	}
}

func (p *printer) printf(format string, args ...any) {
	fmt.Fprintf(&p.b, format, args...)
}

// newline ends the current line and indents the next one.
func (p *printer) newline() {
	p.b.WriteByte('\n')
	p.b.WriteString(strings.Repeat(indentUnit, p.indent))
}

// ---------------------------------------------------------------------------
// Declarations

func (p *printer) file(f *ast.File) {
	for _, imp := range f.Imports {
		p.print("import ", imp.Path.Raw, "\n")
	}
	for i, d := range f.Decls {
		if i > 0 || len(f.Imports) > 0 {
			p.print("\n")
		}
		p.decl(d)
		p.print("\n")
	}
}

func (p *printer) pub(pub bool) {
	if pub {
		p.print("pub ")
	}
}

func (p *printer) decl(d ast.Decl) {
	switch d := d.(type) {
	case *ast.FuncDecl:
		p.funcDecl(d)
	case *ast.ObjectDecl:
		p.pub(d.Pub)
		p.print("type ", d.Name.Name, " object {")
		p.indent++
		for _, f := range d.Fields {
			p.newline()
			p.pub(f.Pub)
			p.print(f.Name.Name, ": ")
			p.typ(f.Type)
		}
		p.closeBrace(len(d.Fields) == 0)
	case *ast.InterfaceDecl:
		p.pub(d.Pub)
		p.print("type ", d.Name.Name, " interface {")
		p.indent++
		for _, m := range d.Methods {
			p.newline()
			p.print("fn ", m.Name.Name)
			p.signature(m.Params, m.Result)
		}
		p.closeBrace(len(d.Methods) == 0)
	case *ast.EnumDecl:
		p.pub(d.Pub)
		p.print("type ", d.Name.Name, " enum {")
		p.indent++
		for _, v := range d.Variants {
			p.newline()
			p.print(v.Name.Name)
			if v.Fields != nil {
				p.print("(")
				for i, f := range v.Fields {
					if i > 0 {
						p.print(", ")
					}
					p.print(f.Name.Name, ": ")
					p.typ(f.Type)
				}
				p.print(")")
			}
		}
		p.closeBrace(len(d.Variants) == 0)
	case *ast.ImplDecl:
		p.print("impl ")
		if d.Interface != nil {
			p.typ(d.Interface)
			p.print(" for ")
		}
		p.typ(d.Target)
		p.print(" {")
		p.indent++
		for i, m := range d.Methods {
			if i > 0 {
				p.print("\n")
			}
			p.newline()
			p.funcDecl(m)
		}
		p.closeBrace(len(d.Methods) == 0)
	default:
		panic(fmt.Sprintf("printer: unexpected declaration %T", d))
	}
}

// closeBrace ends an indented body opened with "{".
func (p *printer) closeBrace(empty bool) {
	p.indent--
	if empty {
		p.print("}")
		return
	}
	p.newline()
	p.print("}")
}

func (p *printer) funcDecl(d *ast.FuncDecl) {
	p.pub(d.Pub)
	p.print("fn ", d.Name.Name)
	p.signature(d.Params, d.Result)
	p.print(" ")
	p.block(d.Body)
}

func (p *printer) signature(params []*ast.Param, result ast.TypeExpr) {
	p.print("(")
	for i, prm := range params {
		if i > 0 {
			p.print(", ")
		}
		switch {
		case prm.MutSelf:
			p.print("mut self")
		case prm.IsSelf:
			p.print("self")
		default:
			p.print(prm.Name.Name, ": ")
			p.typ(prm.Type)
		}
	}
	p.print(")")
	if result != nil {
		p.print(" -> ")
		p.typ(result)
	}
}

// ---------------------------------------------------------------------------
// Types

func (p *printer) typ(t ast.TypeExpr) {
	switch t := t.(type) {
	case *ast.NamedType:
		if t.Module != nil {
			p.print(t.Module.Name, ".")
		}
		p.print(t.Name.Name)
		if len(t.Args) > 0 {
			p.print("[")
			for i, a := range t.Args {
				if i > 0 {
					p.print(", ")
				}
				p.typ(a)
			}
			p.print("]")
		}
	case *ast.MutType:
		p.print("mut ")
		p.typ(t.Elem)
	case *ast.OptionalType:
		p.print("?")
		p.typ(t.Elem)
	case *ast.ArrayType:
		p.print("[]")
		p.typ(t.Elem)
	case *ast.FuncType:
		p.print("fn(")
		for i, prm := range t.Params {
			if i > 0 {
				p.print(", ")
			}
			p.typ(prm)
		}
		p.print(")")
		if t.Result != nil {
			p.print(" -> ")
			p.typ(t.Result)
		}
	default:
		panic(fmt.Sprintf("printer: unexpected type %T", t))
	}
}

// ---------------------------------------------------------------------------
// Statements

func (p *printer) block(b *ast.Block) {
	p.print("{")
	if len(b.Stmts) == 0 {
		p.print("}")
		return
	}
	p.indent++
	for _, s := range b.Stmts {
		p.newline()
		p.stmt(s)
	}
	p.indent--
	p.newline()
	p.print("}")
}

func (p *printer) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.LetStmt:
		if s.Var {
			p.print("var ")
		} else {
			p.print("let ")
		}
		p.print(s.Name.Name)
		if s.Type != nil {
			p.print(": ")
			p.typ(s.Type)
		}
		p.print(" = ")
		p.expr(s.Value)
	case *ast.AssignStmt:
		p.expr(s.Target)
		p.print(" = ")
		p.expr(s.Value)
	case *ast.ReturnStmt:
		p.print("return")
		if s.Value != nil {
			p.print(" ")
			p.expr(s.Value)
		}
	case *ast.BreakStmt:
		p.print("break")
	case *ast.ContinueStmt:
		p.print("continue")
	case *ast.WhileStmt:
		p.print("while ")
		p.expr(s.Cond)
		p.print(" ")
		p.block(s.Body)
	case *ast.ForStmt:
		p.print("for ", s.Var.Name, " in ")
		p.expr(s.Iter)
		p.print(" ")
		p.block(s.Body)
	case *ast.ExprStmt:
		p.expr(s.X)
	default:
		panic(fmt.Sprintf("printer: unexpected statement %T", s))
	}
}

// ---------------------------------------------------------------------------
// Expressions

var opText = map[token.Kind]string{
	token.OR: "or", token.AND: "and", token.NOT: "not",
	token.EQ: "==", token.NEQ: "!=", token.LT: "<", token.LTE: "<=", token.GT: ">", token.GTE: ">=",
	token.PLUS: "+", token.MINUS: "-", token.STAR: "*", token.SLASH: "/", token.PERCENT: "%",
	token.DOTDOT: "..",
}

// open and close emit parentheses around operator expressions in
// Parenthesize mode.
func (p *printer) open() {
	if p.opts.Parenthesize {
		p.print("(")
	}
}

func (p *printer) close() {
	if p.opts.Parenthesize {
		p.print(")")
	}
}

func (p *printer) expr(x ast.Expr) {
	switch x := x.(type) {
	case *ast.Ident:
		p.print(x.Name)
	case *ast.IntLit:
		p.print(x.Raw)
	case *ast.FloatLit:
		p.print(x.Raw)
	case *ast.StringLit:
		p.print(x.Raw)
	case *ast.BoolLit:
		p.printf("%t", x.Value)
	case *ast.BinaryExpr:
		p.open()
		p.expr(x.X)
		if x.Op == token.DOTDOT {
			p.print("..")
		} else {
			p.print(" ", opText[x.Op], " ")
		}
		p.expr(x.Y)
		p.close()
	case *ast.UnaryExpr:
		p.open()
		if x.Op == token.NOT {
			p.print("not ")
		} else {
			p.print(opText[x.Op])
		}
		p.expr(x.X)
		p.close()
	case *ast.ParenExpr:
		p.print("(")
		p.expr(x.X)
		p.print(")")
	case *ast.CallExpr:
		p.expr(x.Fun)
		p.print("(")
		for i, a := range x.Args {
			if i > 0 {
				p.print(", ")
			}
			if a.Name != nil {
				p.print(a.Name.Name, ": ")
			}
			p.expr(a.Value)
		}
		p.print(")")
	case *ast.SelectorExpr:
		p.expr(x.X)
		p.print(".", x.Sel.Name)
	case *ast.IndexExpr:
		p.expr(x.X)
		p.print("[")
		p.expr(x.Index)
		p.print("]")
	case *ast.ArrayLit:
		p.print("[")
		for i, e := range x.Elems {
			if i > 0 {
				p.print(", ")
			}
			p.expr(e)
		}
		p.print("]")
	case *ast.MapLit:
		if len(x.Entries) == 0 {
			p.print("[:]")
			return
		}
		p.print("[")
		for i, e := range x.Entries {
			if i > 0 {
				p.print(", ")
			}
			p.expr(e.Key)
			p.print(": ")
			p.expr(e.Value)
		}
		p.print("]")
	case *ast.ObjectLit:
		p.expr(x.Type)
		if len(x.Fields) == 0 {
			p.print("{}")
			return
		}
		p.print("{ ")
		for i, f := range x.Fields {
			if i > 0 {
				p.print(", ")
			}
			p.print(f.Name.Name, ": ")
			p.expr(f.Value)
		}
		p.print(" }")
	case *ast.FuncLit:
		p.print("fn")
		p.signature(x.Params, x.Result)
		p.print(" ")
		p.block(x.Body)
	case *ast.Block:
		p.block(x)
	case *ast.IfExpr:
		p.ifExpr(x)
	case *ast.MatchExpr:
		p.matchExpr(x)
	case *ast.TryExpr:
		p.open()
		p.print("try ")
		p.expr(x.X)
		if x.Else != nil {
			p.print(" else ")
			if x.ErrName != nil {
				p.print("|", x.ErrName.Name, "| ")
			}
			p.expr(x.Else)
		}
		p.close()
	case *ast.MutExpr:
		p.print("mut ")
		p.expr(x.X)
	case *ast.IsExpr:
		p.open()
		p.expr(x.X)
		p.print(" is ")
		p.typ(x.Type)
		if x.As != nil {
			p.print(" as ", x.As.Name)
		}
		p.close()
	default:
		panic(fmt.Sprintf("printer: unexpected expression %T", x))
	}
}

func (p *printer) ifExpr(x *ast.IfExpr) {
	if then, els, ok := inlineIf(x); ok {
		p.print("if ")
		p.expr(x.Cond)
		p.print(" { ")
		p.expr(then)
		p.print(" } else { ")
		p.expr(els)
		p.print(" }")
		return
	}
	p.print("if ")
	p.expr(x.Cond)
	p.print(" ")
	p.block(x.Then)
	if x.Else != nil {
		p.print(" else ")
		p.expr(x.Else)
	}
}

// inlineIf reports whether x is a simple value-producing if/else, such as
// `if c { 1 } else { 2 }`, that should stay on one line, and returns the
// two branch values.
func inlineIf(x *ast.IfExpr) (then, els ast.Expr, ok bool) {
	elseBlock, isBlock := x.Else.(*ast.Block)
	if !isBlock {
		return nil, nil, false
	}
	then, ok1 := singleSimpleExpr(x.Then)
	els, ok2 := singleSimpleExpr(elseBlock)
	return then, els, ok1 && ok2 && simpleExpr(x.Cond)
}

func singleSimpleExpr(b *ast.Block) (ast.Expr, bool) {
	if len(b.Stmts) != 1 {
		return nil, false
	}
	s, ok := b.Stmts[0].(*ast.ExprStmt)
	if !ok || !simpleExpr(s.X) {
		return nil, false
	}
	return s.X, true
}

// simpleExpr reports whether x prints on a single line.
func simpleExpr(x ast.Expr) bool {
	switch x := x.(type) {
	case *ast.Block, *ast.IfExpr, *ast.MatchExpr, *ast.FuncLit:
		return false
	case *ast.BinaryExpr:
		return simpleExpr(x.X) && simpleExpr(x.Y)
	case *ast.UnaryExpr:
		return simpleExpr(x.X)
	case *ast.ParenExpr:
		return simpleExpr(x.X)
	case *ast.CallExpr:
		for _, a := range x.Args {
			if !simpleExpr(a.Value) {
				return false
			}
		}
		return simpleExpr(x.Fun)
	case *ast.SelectorExpr:
		return simpleExpr(x.X)
	case *ast.IndexExpr:
		return simpleExpr(x.X) && simpleExpr(x.Index)
	case *ast.ArrayLit:
		for _, e := range x.Elems {
			if !simpleExpr(e) {
				return false
			}
		}
		return true
	case *ast.MapLit:
		for _, e := range x.Entries {
			if !simpleExpr(e.Key) || !simpleExpr(e.Value) {
				return false
			}
		}
		return true
	case *ast.ObjectLit:
		for _, f := range x.Fields {
			if !simpleExpr(f.Value) {
				return false
			}
		}
		return true
	case *ast.TryExpr:
		return simpleExpr(x.X) && (x.Else == nil || simpleExpr(x.Else))
	case *ast.IsExpr:
		return simpleExpr(x.X)
	case *ast.MutExpr:
		return simpleExpr(x.X)
	}
	return true
}

func (p *printer) matchExpr(x *ast.MatchExpr) {
	p.print("match ")
	p.expr(x.Subject)
	p.print(" {")
	p.indent++
	for _, arm := range x.Arms {
		p.newline()
		p.pattern(arm.Pattern)
		p.print(" => ")
		p.expr(arm.Body)
	}
	p.closeBrace(len(x.Arms) == 0)
}

func (p *printer) pattern(pat ast.Pattern) {
	switch pat := pat.(type) {
	case *ast.WildcardPattern:
		p.print("_")
	case *ast.TypePattern:
		p.print("is ")
		p.typ(pat.Type)
		if pat.As != nil {
			p.print(" as ", pat.As.Name)
		}
	case *ast.VariantPattern:
		p.expr(pat.Name)
		if pat.Bindings != nil {
			p.print("(")
			for i, b := range pat.Bindings {
				if i > 0 {
					p.print(", ")
				}
				p.print(b.Field.Name)
				if b.Name != nil {
					p.print(": ", b.Name.Name)
				}
			}
			p.print(")")
		}
	default:
		panic(fmt.Sprintf("printer: unexpected pattern %T", pat))
	}
}
