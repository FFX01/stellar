// Package parser builds a syntax tree from Stellar source.
//
// The parser is a hand-written recursive-descent parser. On a syntax error
// it records a diagnostic, abandons the current statement or declaration
// member, and resynchronises at the next line, so one mistake produces one
// diagnostic instead of a cascade.
package parser

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"stellar/internal/ast"
	"stellar/internal/diag"
	"stellar/internal/lexer"
	"stellar/internal/token"
)

// maxErrors bounds how many diagnostics a single parse reports.
const maxErrors = 10

// bailout unwinds the parser to the nearest recovery point.
type bailout struct{}

// tooManyErrors unwinds the whole parse once maxErrors is reached.
type tooManyErrors struct{}

type parser struct {
	toks []token.Token
	i    int
	tok  token.Token

	errs diag.List
}

// ParseFile parses a complete source file.
func ParseFile(src []byte) (*ast.File, diag.List) {
	p := newParser(src)
	file := &ast.File{}
	func() {
		defer p.catchTooMany()
		p.parseFile(file)
	}()
	p.errs.Sort()
	return file, p.errs
}

// ParseExpr parses a single expression. It is intended for tests and tools.
func ParseExpr(src []byte) (ast.Expr, diag.List) {
	p := newParser(src)
	var x ast.Expr
	func() {
		defer p.catchTooMany()
		defer p.catchBailout()
		x = p.parseExpr()
		p.accept(token.NEWLINE)
		if !p.at(token.EOF) {
			p.fail(p.tok.Pos, "unexpected %s after expression", p.tok)
		}
	}()
	p.errs.Sort()
	return x, p.errs
}

func newParser(src []byte) *parser {
	toks, lexErrs := lexer.Tokenize(src)
	p := &parser{toks: toks, errs: lexErrs}
	p.tok = p.toks[0]
	return p
}

// ---------------------------------------------------------------------------
// Token helpers

func (p *parser) next() {
	if p.i < len(p.toks)-1 {
		p.i++
	}
	p.tok = p.toks[p.i]
}

func (p *parser) peek() token.Token {
	if p.i+1 < len(p.toks) {
		return p.toks[p.i+1]
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser) at(k token.Kind) bool { return p.tok.Kind == k }

func (p *parser) atIdent(name string) bool {
	return p.tok.Kind == token.IDENT && p.tok.Lit == name
}

func (p *parser) accept(k token.Kind) bool {
	if p.at(k) {
		p.next()
		return true
	}
	return false
}

func (p *parser) expect(k token.Kind) token.Token {
	t := p.tok
	if t.Kind != k {
		p.fail(t.Pos, "expected %s, found %s", k, t)
	}
	p.next()
	return t
}

func (p *parser) expectIdent() *ast.Ident {
	t := p.expect(token.IDENT)
	return &ast.Ident{At: t.Pos, Name: t.Lit}
}

func (p *parser) skipNewlines() {
	for p.at(token.NEWLINE) {
		p.next()
	}
}

// expectTerminator requires the end of a statement or member: a newline, or
// a following `}` or end of file, which are left in place.
func (p *parser) expectTerminator() {
	switch p.tok.Kind {
	case token.NEWLINE:
		p.next()
	case token.RBRACE, token.EOF:
	default:
		p.fail(p.tok.Pos, "expected newline, found %s", p.tok)
	}
}

// ---------------------------------------------------------------------------
// Errors and recovery

func (p *parser) errorf(pos token.Pos, format string, args ...any) {
	// Report at most one error per line; later ones are usually cascades.
	if n := len(p.errs); n > 0 && p.errs[n-1].Pos.Line == pos.Line {
		return
	}
	p.errs.Add(pos, format, args...)
	if len(p.errs) >= maxErrors {
		panic(tooManyErrors{})
	}
}

// fail reports an error and abandons the current construct. Errors at an
// ILLEGAL token were already reported by the lexer.
func (p *parser) fail(pos token.Pos, format string, args ...any) {
	if !p.at(token.ILLEGAL) {
		p.errorf(pos, format, args...)
	}
	panic(bailout{})
}

func (p *parser) catchTooMany() {
	if r := recover(); r != nil {
		if _, ok := r.(tooManyErrors); !ok {
			panic(r)
		}
	}
}

func (p *parser) catchBailout() {
	if r := recover(); r != nil {
		if _, ok := r.(bailout); !ok {
			panic(r)
		}
	}
}

// guarded runs parse, a parser for one statement or member of a braced
// body. If it fails, guarded skips to the end of the line or to the `}`
// closing the body, whichever comes first, so parsing can continue.
func (p *parser) guarded(parse func()) {
	start := p.i
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if _, ok := r.(bailout); !ok {
			panic(r)
		}
		p.syncLine()
		if p.i == start && !p.at(token.RBRACE) && !p.at(token.EOF) {
			p.next() // guarantee progress
		}
	}()
	parse()
}

func (p *parser) syncLine() {
	depth := 0
	for !p.at(token.EOF) {
		switch p.tok.Kind {
		case token.LBRACE, token.LPAREN, token.LBRACKET:
			depth++
		case token.RPAREN, token.RBRACKET:
			if depth > 0 {
				depth--
			}
		case token.RBRACE:
			if depth == 0 {
				return
			}
			depth--
		case token.NEWLINE:
			if depth == 0 {
				p.next()
				return
			}
		}
		p.next()
	}
}

// syncDecl skips to the next token that can start a top-level declaration
// at the beginning of a line. start is the token index where the failed
// declaration began; syncDecl always moves past it.
func (p *parser) syncDecl(start int) {
	for !p.at(token.EOF) {
		if p.i > start && p.tok.Pos.Col == 1 && startsDecl(p.tok.Kind) {
			return
		}
		p.next()
	}
}

func startsDecl(k token.Kind) bool {
	switch k {
	case token.PUB, token.FN, token.TYPE, token.IMPL, token.IMPORT:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Files and declarations

func (p *parser) parseFile(file *ast.File) {
	p.skipNewlines()
	for !p.at(token.EOF) {
		p.parseTopLevel(file)
		p.skipNewlines()
	}
}

func (p *parser) parseTopLevel(file *ast.File) {
	start := p.i
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			p.syncDecl(start)
		}
	}()

	if p.at(token.IMPORT) {
		if len(file.Decls) > 0 {
			p.fail(p.tok.Pos, "imports must come before declarations")
		}
		file.Imports = append(file.Imports, p.parseImport())
		return
	}
	file.Decls = append(file.Decls, p.parseDecl())
}

func (p *parser) parseImport() *ast.Import {
	at := p.expect(token.IMPORT).Pos
	path := p.parseStringLit()
	p.expectTerminator()
	return &ast.Import{At: at, Path: path}
}

func (p *parser) parseDecl() ast.Decl {
	at := p.tok.Pos
	pub := p.accept(token.PUB)
	var d ast.Decl
	switch p.tok.Kind {
	case token.FN:
		d = p.parseFuncDecl(at, pub)
	case token.TYPE:
		d = p.parseTypeDecl(at, pub)
	case token.IMPL:
		if pub {
			p.fail(at, "impl blocks cannot be pub")
		}
		d = p.parseImpl()
	default:
		p.fail(p.tok.Pos, "expected declaration (fn, type or impl), found %s", p.tok)
	}
	p.expectTerminator()
	return d
}

func (p *parser) parseFuncDecl(at token.Pos, pub bool) *ast.FuncDecl {
	p.expect(token.FN)
	name := p.expectIdent()
	params := p.parseParams()
	result := p.parseResult()
	body := p.parseBlock()
	return &ast.FuncDecl{At: at, Pub: pub, Name: name, Params: params, Result: result, Body: body}
}

// parseResult parses an optional `-> Type`.
func (p *parser) parseResult() ast.TypeExpr {
	if p.accept(token.ARROW) {
		return p.parseType()
	}
	return nil
}

func (p *parser) parseParams() []*ast.Param {
	p.expect(token.LPAREN)
	var params []*ast.Param
	p.skipNewlines()
	for !p.at(token.RPAREN) {
		params = append(params, p.parseParam())
		p.skipNewlines()
		if !p.accept(token.COMMA) {
			break
		}
		p.skipNewlines()
	}
	p.expect(token.RPAREN)
	return params
}

func (p *parser) parseParam() *ast.Param {
	at := p.tok.Pos
	if p.at(token.MUT) && p.peek().Kind == token.IDENT && p.peek().Lit == "self" {
		p.next()
		name := p.expectIdent()
		return &ast.Param{At: at, Name: name, IsSelf: true, MutSelf: true}
	}
	if p.atIdent("self") {
		name := p.expectIdent()
		return &ast.Param{At: at, Name: name, IsSelf: true}
	}
	name := p.expectIdent()
	p.expect(token.COLON)
	typ := p.parseParamType()
	return &ast.Param{At: at, Name: name, Type: typ}
}

// parseParamType parses a parameter type, the one place `mut T` is allowed.
func (p *parser) parseParamType() ast.TypeExpr {
	if p.at(token.MUT) {
		at := p.tok.Pos
		p.next()
		return &ast.MutType{At: at, Elem: p.parseType()}
	}
	return p.parseType()
}

func (p *parser) parseTypeDecl(at token.Pos, pub bool) ast.Decl {
	p.expect(token.TYPE)
	name := p.expectIdent()
	if !isPascalCase(name.Name) {
		p.errorf(name.At, "type name %s must be PascalCase", name.Name)
	}
	switch p.tok.Kind {
	case token.OBJECT:
		p.next()
		return &ast.ObjectDecl{At: at, Pub: pub, Name: name, Fields: p.parseObjectBody()}
	case token.INTERFACE:
		p.next()
		return &ast.InterfaceDecl{At: at, Pub: pub, Name: name, Methods: p.parseInterfaceBody()}
	case token.ENUM:
		p.next()
		return &ast.EnumDecl{At: at, Pub: pub, Name: name, Variants: p.parseEnumBody()}
	default:
		p.fail(p.tok.Pos, "expected object, interface or enum, found %s", p.tok)
		return nil
	}
}

// parseBody parses `{ member \n member ... }`, calling member once per
// newline-separated member with error recovery between members.
func (p *parser) parseBody(member func()) {
	p.expect(token.LBRACE)
	p.skipNewlines()
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		p.guarded(func() {
			member()
			p.expectTerminator()
		})
		p.skipNewlines()
	}
	p.expect(token.RBRACE)
}

func (p *parser) parseObjectBody() []*ast.Field {
	var fields []*ast.Field
	p.parseBody(func() {
		at := p.tok.Pos
		pub := p.accept(token.PUB)
		name := p.expectIdent()
		p.expect(token.COLON)
		fields = append(fields, &ast.Field{At: at, Pub: pub, Name: name, Type: p.parseType()})
	})
	return fields
}

func (p *parser) parseInterfaceBody() []*ast.FuncSig {
	var methods []*ast.FuncSig
	p.parseBody(func() {
		at := p.tok.Pos
		if p.at(token.PUB) {
			p.fail(at, "interface methods take the interface's visibility; remove pub")
		}
		p.expect(token.FN)
		name := p.expectIdent()
		params := p.parseParams()
		result := p.parseResult()
		methods = append(methods, &ast.FuncSig{At: at, Name: name, Params: params, Result: result})
	})
	return methods
}

func (p *parser) parseEnumBody() []*ast.Variant {
	var variants []*ast.Variant
	p.parseBody(func() {
		name := p.expectIdent()
		v := &ast.Variant{At: name.At, Name: name}
		if p.at(token.LPAREN) {
			v.Fields = p.parseVariantFields()
		}
		variants = append(variants, v)
	})
	return variants
}

func (p *parser) parseVariantFields() []*ast.Field {
	p.expect(token.LPAREN)
	fields := []*ast.Field{}
	p.skipNewlines()
	for !p.at(token.RPAREN) {
		name := p.expectIdent()
		p.expect(token.COLON)
		fields = append(fields, &ast.Field{At: name.At, Name: name, Type: p.parseType()})
		p.skipNewlines()
		if !p.accept(token.COMMA) {
			break
		}
		p.skipNewlines()
	}
	p.expect(token.RPAREN)
	return fields
}

func (p *parser) parseImpl() *ast.ImplDecl {
	at := p.expect(token.IMPL).Pos
	d := &ast.ImplDecl{At: at}
	first := p.parseType()
	if p.accept(token.FOR) {
		d.Interface = first
		d.Target = p.parseType()
	} else {
		d.Target = first
	}
	p.parseBody(func() {
		mat := p.tok.Pos
		pub := p.accept(token.PUB)
		if !p.at(token.FN) {
			p.fail(p.tok.Pos, "expected method, found %s", p.tok)
		}
		d.Methods = append(d.Methods, p.parseFuncDecl(mat, pub))
	})
	return d
}

// ---------------------------------------------------------------------------
// Types

func (p *parser) parseType() ast.TypeExpr {
	at := p.tok.Pos
	switch p.tok.Kind {
	case token.MUT:
		p.fail(at, "mut is only allowed on function parameters; use var for a variable that changes")
	case token.QUESTION:
		p.next()
		return &ast.OptionalType{At: at, Elem: p.parseType()}
	case token.LBRACKET:
		p.next()
		p.expect(token.RBRACKET)
		return &ast.ArrayType{At: at, Elem: p.parseType()}
	case token.FN:
		return p.parseFuncType()
	case token.IDENT:
		return p.parseNamedType()
	default:
		p.fail(at, "expected type, found %s", p.tok)
	}
	return nil
}

func (p *parser) parseNamedType() *ast.NamedType {
	t := &ast.NamedType{Name: p.expectIdent()}
	if p.accept(token.DOT) {
		t.Module, t.Name = t.Name, p.expectIdent()
	}
	if p.accept(token.LBRACKET) {
		for {
			t.Args = append(t.Args, p.parseType())
			if !p.accept(token.COMMA) {
				break
			}
		}
		p.expect(token.RBRACKET)
	}
	return t
}

func (p *parser) parseFuncType() *ast.FuncType {
	at := p.expect(token.FN).Pos
	p.expect(token.LPAREN)
	t := &ast.FuncType{At: at}
	p.skipNewlines()
	for !p.at(token.RPAREN) {
		t.Params = append(t.Params, p.parseParamType())
		p.skipNewlines()
		if !p.accept(token.COMMA) {
			break
		}
		p.skipNewlines()
	}
	p.expect(token.RPAREN)
	t.Result = p.parseResult()
	return t
}

// ---------------------------------------------------------------------------
// Statements

func (p *parser) parseBlock() *ast.Block {
	at := p.expect(token.LBRACE).Pos
	b := &ast.Block{At: at}
	p.skipNewlines()
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		p.guarded(func() {
			b.Stmts = append(b.Stmts, p.parseStmt())
		})
		p.skipNewlines()
	}
	p.expect(token.RBRACE)
	return b
}

func (p *parser) parseStmt() ast.Stmt {
	at := p.tok.Pos
	var s ast.Stmt
	switch p.tok.Kind {
	case token.LET, token.VAR:
		s = p.parseLet()
	case token.RETURN:
		p.next()
		r := &ast.ReturnStmt{At: at}
		if !p.at(token.NEWLINE) && !p.at(token.RBRACE) && !p.at(token.EOF) {
			r.Value = p.parseExpr()
		}
		s = r
	case token.BREAK:
		p.next()
		s = &ast.BreakStmt{At: at}
	case token.CONTINUE:
		p.next()
		s = &ast.ContinueStmt{At: at}
	case token.WHILE:
		p.next()
		cond := p.parseExpr()
		s = &ast.WhileStmt{At: at, Cond: cond, Body: p.parseBlock()}
	case token.FOR:
		p.next()
		v := p.expectIdent()
		p.expect(token.IN)
		iter := p.parseExpr()
		s = &ast.ForStmt{At: at, Var: v, Iter: iter, Body: p.parseBlock()}
	default:
		x := p.parseExpr()
		if p.at(token.ASSIGN) {
			switch x.(type) {
			case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr:
			default:
				p.fail(x.Pos(), "cannot assign to this expression")
			}
			p.next()
			s = &ast.AssignStmt{Target: x, Value: p.parseExpr()}
		} else {
			s = &ast.ExprStmt{X: x}
		}
	}
	p.expectTerminator()
	return s
}

func (p *parser) parseLet() *ast.LetStmt {
	s := &ast.LetStmt{At: p.tok.Pos, Var: p.at(token.VAR)}
	p.next()
	s.Name = p.expectIdent()
	if p.accept(token.COLON) {
		s.Type = p.parseType()
	}
	if !p.at(token.ASSIGN) {
		p.fail(p.tok.Pos, "expected = and an initial value, found %s", p.tok)
	}
	p.next()
	s.Value = p.parseExpr()
	return s
}

// ---------------------------------------------------------------------------
// Expressions
//
// Precedence, lowest to highest:
//
//	or
//	and
//	not
//	== != < <= > >= is      (non-associative)
//	..                      (non-associative)
//	+ -
//	* / %
//	unary -  try
//	postfix: call, selector, index, object literal

func (p *parser) parseExpr() ast.Expr {
	return p.parseOr()
}

func (p *parser) parseOr() ast.Expr {
	x := p.parseAnd()
	for p.at(token.OR) {
		pos := p.tok.Pos
		p.next()
		p.skipNewlines()
		x = &ast.BinaryExpr{OpPos: pos, Op: token.OR, X: x, Y: p.parseAnd()}
	}
	return x
}

func (p *parser) parseAnd() ast.Expr {
	x := p.parseNot()
	for p.at(token.AND) {
		pos := p.tok.Pos
		p.next()
		p.skipNewlines()
		x = &ast.BinaryExpr{OpPos: pos, Op: token.AND, X: x, Y: p.parseNot()}
	}
	return x
}

func (p *parser) parseNot() ast.Expr {
	if p.at(token.NOT) {
		at := p.tok.Pos
		p.next()
		return &ast.UnaryExpr{At: at, Op: token.NOT, X: p.parseNot()}
	}
	return p.parseComparison()
}

func isComparison(k token.Kind) bool {
	switch k {
	case token.EQ, token.NEQ, token.LT, token.LTE, token.GT, token.GTE:
		return true
	}
	return false
}

func (p *parser) parseComparison() ast.Expr {
	x := p.parseRange()
	switch {
	case isComparison(p.tok.Kind):
		pos, op := p.tok.Pos, p.tok.Kind
		p.next()
		p.skipNewlines()
		x = &ast.BinaryExpr{OpPos: pos, Op: op, X: x, Y: p.parseRange()}
	case p.at(token.IS):
		p.next()
		is := &ast.IsExpr{X: x, Type: p.parseType()}
		if p.accept(token.AS) {
			is.As = p.expectIdent()
		}
		x = is
	default:
		return x
	}
	if isComparison(p.tok.Kind) || p.at(token.IS) {
		p.fail(p.tok.Pos, "comparisons cannot be chained; use 'and'")
	}
	return x
}

func (p *parser) parseRange() ast.Expr {
	x := p.parseAdditive()
	if p.at(token.DOTDOT) {
		pos := p.tok.Pos
		p.next()
		x = &ast.BinaryExpr{OpPos: pos, Op: token.DOTDOT, X: x, Y: p.parseAdditive()}
		if p.at(token.DOTDOT) {
			p.fail(p.tok.Pos, "ranges cannot be chained")
		}
	}
	return x
}

func (p *parser) parseAdditive() ast.Expr {
	x := p.parseMultiplicative()
	for p.at(token.PLUS) || p.at(token.MINUS) {
		pos, op := p.tok.Pos, p.tok.Kind
		p.next()
		p.skipNewlines()
		x = &ast.BinaryExpr{OpPos: pos, Op: op, X: x, Y: p.parseMultiplicative()}
	}
	return x
}

func (p *parser) parseMultiplicative() ast.Expr {
	x := p.parseUnary()
	for p.at(token.STAR) || p.at(token.SLASH) || p.at(token.PERCENT) {
		pos, op := p.tok.Pos, p.tok.Kind
		p.next()
		p.skipNewlines()
		x = &ast.BinaryExpr{OpPos: pos, Op: op, X: x, Y: p.parseUnary()}
	}
	return x
}

func (p *parser) parseUnary() ast.Expr {
	switch p.tok.Kind {
	case token.MINUS:
		at := p.tok.Pos
		p.next()
		return &ast.UnaryExpr{At: at, Op: token.MINUS, X: p.parseUnary()}
	case token.TRY:
		return p.parseTry()
	}
	return p.parsePostfix()
}

// parseTry parses `try X [else [|e|] fallback]`. The fallback is a block or
// a full expression, so an else clause extends as far right as possible.
func (p *parser) parseTry() ast.Expr {
	t := &ast.TryExpr{At: p.expect(token.TRY).Pos}
	t.X = p.parseUnary()
	if !p.accept(token.ELSE) {
		return t
	}
	if p.accept(token.PIPE) {
		t.ErrName = p.expectIdent()
		p.expect(token.PIPE)
	}
	if p.at(token.LBRACE) {
		t.Else = p.parseBlock()
	} else {
		t.Else = p.parseExpr()
	}
	return t
}

func (p *parser) parsePostfix() ast.Expr {
	x := p.parsePrimary()
	for {
		switch p.tok.Kind {
		case token.LPAREN:
			x = &ast.CallExpr{Fun: x, Args: p.parseArgs()}
		case token.DOT:
			p.next()
			x = &ast.SelectorExpr{X: x, Sel: p.expectIdent()}
		case token.LBRACKET:
			p.next()
			p.skipNewlines()
			idx := p.parseExpr()
			p.skipNewlines()
			p.expect(token.RBRACKET)
			x = &ast.IndexExpr{X: x, Index: idx}
		case token.LBRACE:
			if !p.objectLitStart(x) {
				return x
			}
			x = p.parseObjectLit(x)
		default:
			return x
		}
	}
}

// objectLitStart decides whether the `{` at the current token begins an
// object literal for the expression x. A `{` written directly after a name,
// with no space (`Book{`), is an object literal; a `{` after a space opens a
// block, such as the body of an if or for.
//
// The two common mistakes, a space inside a literal and a missing space
// before a body, are reported but then parsed as what was evidently meant,
// so a single typo does not derail the rest of the parse.
func (p *parser) objectLitStart(x ast.Expr) bool {
	name, ok := typeName(x)
	if p.tok.SpaceBefore {
		// No block starts with `field:`, so this is a misspaced literal.
		if ok && isPascalCase(name) && p.fieldFollows() {
			p.errorf(p.tok.Pos, "object literal must not have a space before {; write %s{", name)
			return true
		}
		return false
	}
	if !ok {
		return false
	}
	if !isPascalCase(name) {
		p.errorf(p.tok.Pos, "%s{ is not an object literal because type names are PascalCase; add a space before { to start a block", name)
		return false
	}
	if p.fieldFollows() || p.peek().Kind == token.RBRACE {
		return true
	}
	p.errorf(p.tok.Pos, "%s{ must be followed by field: value; add a space before { to start a block", name)
	return false
}

// fieldFollows reports whether the tokens after the current `{` are
// `name:`, ignoring newlines.
func (p *parser) fieldFollows() bool {
	j := p.i + 1
	for j < len(p.toks) && p.toks[j].Kind == token.NEWLINE {
		j++
	}
	return j+1 < len(p.toks) && p.toks[j].Kind == token.IDENT && p.toks[j+1].Kind == token.COLON
}

// typeName returns the type name that x spells, if it can name an object
// type: `Book` or `module.Book`.
func typeName(x ast.Expr) (string, bool) {
	switch x := x.(type) {
	case *ast.Ident:
		return x.Name, true
	case *ast.SelectorExpr:
		if _, ok := x.X.(*ast.Ident); ok {
			return x.Sel.Name, true
		}
	}
	return "", false
}

// isPascalCase reports whether name is a valid type name: it starts with an
// upper-case letter and contains no underscores.
func isPascalCase(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r) && !strings.Contains(name, "_")
}

// parseList parses a comma-separated list closed by end, allowing newlines
// between elements and a trailing comma.
func (p *parser) parseList(end token.Kind, elem func()) {
	p.skipNewlines()
	for !p.at(end) {
		elem()
		p.skipNewlines()
		if !p.accept(token.COMMA) {
			break
		}
		p.skipNewlines()
	}
	p.expect(end)
}

func (p *parser) parseArgs() []*ast.Arg {
	p.expect(token.LPAREN)
	args := []*ast.Arg{}
	p.parseList(token.RPAREN, func() {
		a := &ast.Arg{}
		if p.at(token.IDENT) && p.peek().Kind == token.COLON {
			a.Name = p.expectIdent()
			p.next()
		}
		a.Value = p.parseExpr()
		args = append(args, a)
	})
	return args
}

func (p *parser) parseObjectLit(typ ast.Expr) *ast.ObjectLit {
	p.expect(token.LBRACE)
	lit := &ast.ObjectLit{Type: typ, Fields: []*ast.FieldInit{}}
	p.parseList(token.RBRACE, func() {
		name := p.expectIdent()
		p.expect(token.COLON)
		lit.Fields = append(lit.Fields, &ast.FieldInit{Name: name, Value: p.parseExpr()})
	})
	return lit
}

func (p *parser) parsePrimary() ast.Expr {
	t := p.tok
	switch t.Kind {
	case token.IDENT:
		p.next()
		return &ast.Ident{At: t.Pos, Name: t.Lit}
	case token.INT:
		p.next()
		return &ast.IntLit{At: t.Pos, Raw: t.Lit}
	case token.FLOAT:
		p.next()
		return &ast.FloatLit{At: t.Pos, Raw: t.Lit}
	case token.STRING:
		return p.parseStringLit()
	case token.TRUE, token.FALSE:
		p.next()
		return &ast.BoolLit{At: t.Pos, Value: t.Kind == token.TRUE}
	case token.LPAREN:
		p.next()
		p.skipNewlines()
		x := p.parseExpr()
		p.skipNewlines()
		p.expect(token.RPAREN)
		return &ast.ParenExpr{At: t.Pos, X: x}
	case token.LBRACKET:
		return p.parseArrayOrMapLit()
	case token.FN:
		return p.parseFuncLit(t.Pos)
	case token.MUT:
		p.next()
		if p.at(token.FN) {
			p.fail(t.Pos, "mut fn no longer exists; closures are plain fn")
		}
		return &ast.MutExpr{At: t.Pos, X: p.parsePostfix()}
	case token.IF:
		return p.parseIf()
	case token.MATCH:
		return p.parseMatch()
	}
	p.fail(t.Pos, "expected expression, found %s", t)
	return nil
}

// parseArrayOrMapLit parses `[a, b]`, `[]`, `[k: v, ...]` or `[:]`. The
// first element decides which: a `:` after it makes the literal a map.
func (p *parser) parseArrayOrMapLit() ast.Expr {
	at := p.expect(token.LBRACKET).Pos
	if p.at(token.COLON) {
		p.next()
		p.expect(token.RBRACKET)
		return &ast.MapLit{At: at, Entries: []*ast.MapEntry{}}
	}
	p.skipNewlines()
	isMap := false
	array := &ast.ArrayLit{At: at, Elems: []ast.Expr{}}
	m := &ast.MapLit{At: at, Entries: []*ast.MapEntry{}}
	first := true
	p.parseList(token.RBRACKET, func() {
		x := p.parseExpr()
		if first {
			isMap = p.at(token.COLON)
			first = false
		}
		if !isMap {
			if p.at(token.COLON) {
				p.fail(p.tok.Pos, "unexpected : in array literal")
			}
			array.Elems = append(array.Elems, x)
			return
		}
		if !p.at(token.COLON) {
			p.fail(p.tok.Pos, "expected : after map key, found %s", p.tok)
		}
		p.next()
		m.Entries = append(m.Entries, &ast.MapEntry{Key: x, Value: p.parseExpr()})
	})
	if isMap {
		return m
	}
	return array
}

func (p *parser) parseStringLit() *ast.StringLit {
	t := p.expect(token.STRING)
	// The lexer has already reported malformed escapes.
	v, _ := lexer.Unquote(t.Lit)
	return &ast.StringLit{At: t.Pos, Raw: t.Lit, Value: v}
}

func (p *parser) parseFuncLit(at token.Pos) *ast.FuncLit {
	p.expect(token.FN)
	f := &ast.FuncLit{At: at}
	f.Params = p.parseParams()
	f.Result = p.parseResult()
	f.Body = p.parseBlock()
	return f
}

func (p *parser) parseIf() *ast.IfExpr {
	at := p.expect(token.IF).Pos
	x := &ast.IfExpr{At: at}
	x.Cond = p.parseExpr()
	x.Then = p.parseBlock()
	if p.accept(token.ELSE) {
		if p.at(token.IF) {
			x.Else = p.parseIf()
		} else {
			x.Else = p.parseBlock()
		}
	}
	return x
}

func (p *parser) parseMatch() *ast.MatchExpr {
	at := p.expect(token.MATCH).Pos
	m := &ast.MatchExpr{At: at, Subject: p.parseExpr()}
	p.parseBody(func() {
		arm := &ast.MatchArm{Pattern: p.parsePattern()}
		p.expect(token.FATARROW)
		if p.at(token.LBRACE) {
			arm.Body = p.parseBlock()
		} else {
			arm.Body = p.parseExpr()
		}
		m.Arms = append(m.Arms, arm)
	})
	return m
}

func (p *parser) parsePattern() ast.Pattern {
	at := p.tok.Pos
	switch {
	case p.atIdent("_"):
		p.next()
		return &ast.WildcardPattern{At: at}
	case p.at(token.IS):
		p.next()
		tp := &ast.TypePattern{At: at, Type: p.parseType()}
		if p.accept(token.AS) {
			tp.As = p.expectIdent()
		}
		return tp
	case p.at(token.IDENT):
		var name ast.Expr = p.expectIdent()
		for p.accept(token.DOT) {
			name = &ast.SelectorExpr{X: name, Sel: p.expectIdent()}
		}
		vp := &ast.VariantPattern{Name: name}
		if p.at(token.LPAREN) {
			p.next()
			vp.Bindings = []*ast.Binding{}
			p.parseList(token.RPAREN, func() {
				b := &ast.Binding{Field: p.expectIdent()}
				if p.accept(token.COLON) {
					b.Name = p.expectIdent()
				}
				vp.Bindings = append(vp.Bindings, b)
			})
		}
		return vp
	}
	p.fail(at, "expected pattern, found %s", p.tok)
	return nil
}
