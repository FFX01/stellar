package parser

import (
	"fmt"
	"log"
	"slices"
	"stellar/internal/ast"
	"stellar/internal/lexer"
	"strconv"
	"strings"
)

type (
	prefixParseFunc func() ast.Expression
	infixParseFunc  func(left ast.Expression) ast.Expression
)

type Precedence int

const (
	_ Precedence = iota
	PLowest
	PAssign
	POr
	PAnd
	PEquals
	PRelational
	PSum
	PProduct
	PUnary
	PCall
	PIndex
	PHighest
)

var PMap = map[lexer.T_Token]Precedence{
	lexer.OR:     POr,
	lexer.AND:    PAnd,
	lexer.EQ:     PEquals,
	lexer.NEQ:    PEquals,
	lexer.GT:     PRelational,
	lexer.LT:     PRelational,
	lexer.GTE:    PRelational,
	lexer.LTE:    PRelational,
	lexer.PLUS:   PSum,
	lexer.MINUS:  PSum,
	lexer.AST:    PProduct,
	lexer.DIV:    PProduct,
	lexer.MOD:    PProduct,
	lexer.LPAREN: PCall,
	lexer.LBRACK: PCall,
	lexer.DOT:    PCall,
}

type Error struct {
	Position lexer.Position
	Message  string
}

func (e *Error) String() string {
	return fmt.Sprintf("%s: %s", e.Position.String(), e.Message)
}

type Parser struct {
	tokens []lexer.Token
	pos    int
	errors []Error

	prefixFuncs map[lexer.T_Token]prefixParseFunc
	infixFuncs  map[lexer.T_Token]infixParseFunc
}

func (p *Parser) PrintErrors() {
	for _, e := range p.errors {
		fmt.Println(e.String())
	}
}

func (p *Parser) cur() lexer.Token {
	if p.pos >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos]
}
func (p *Parser) peek(n int) lexer.Token {
	if p.pos+n < len(p.tokens) {
		return p.tokens[p.pos+n]
	}
	return p.tokens[len(p.tokens)-1]
}
func (p *Parser) recall(n int) lexer.Token {
	if p.pos-n > -1 {
		return p.tokens[p.pos-n]
	}
	return p.tokens[0]
}
func (p *Parser) recallCheck(n int, t lexer.T_Token) bool {
	return p.recall(n).Type == t
}
func (p *Parser) error(t lexer.Token, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	msg = fmt.Sprintf(
		"%d:%d %s %s: ",
		p.cur().Position.Line,
		p.cur().Position.Column,
		p.cur().Literal,
		p.peek(1).Literal,
	) + msg
	log.Print(msg)
	p.errors = append(p.errors, Error{Position: t.Position, Message: msg})
}
func (p *Parser) advance() lexer.Token {
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return p.cur()
}
func (p *Parser) accept(t lexer.T_Token) bool {
	if p.check(t) {
		p.advance()
		return true
	}
	return false
}
func (p *Parser) expectOneOf(opts ...lexer.T_Token) lexer.Token {
	tok := p.cur()
	if !slices.Contains(opts, tok.Type) {
		var typestrs []string
		for _, t := range opts {
			typestrs = append(typestrs, string(t))
		}
		p.error(
			tok,
			"Expected on of [%s], got %s",
			strings.Join(typestrs, ", "),
			tok.Type,
		)
	}
	p.advance()
	return tok
}

func (p *Parser) expect(t lexer.T_Token) lexer.Token {
	tok := p.cur()
	if tok.Type != t {
		p.error(tok, "Expected %s, got %s", t, tok.Type)
	}
	p.advance()
	return tok
}
func (p *Parser) check(t lexer.T_Token) bool { return p.cur().Type == t }
func (p *Parser) peekCheck(n int, t lexer.T_Token) bool {
	return p.peek(n).Type == t
}

func (p *Parser) parseBoolLiteralExpr() ast.Expression {
	n := &ast.BoolLiteralExpr{Token: p.cur()}
	if p.check(lexer.TRUE) {
		n.Value = true
	}
	if p.check(lexer.FALSE) {
		n.Value = false
	}
	p.advance()
	return n
}

func (p *Parser) parseName() *ast.Name {
	tok := p.expect(lexer.IDENT)
	return &ast.Name{Token: tok, Value: tok.Literal}
}

func (p *Parser) parseBlockExpr() ast.Expression {
	tok := p.expect(lexer.LBRACE)
	n := &ast.BlockExpr{Token: tok}
	for !p.check(lexer.RBRACE) && !p.check(lexer.EOF) {
		n.Statements = append(n.Statements, p.parseStatement())
		if p.check(lexer.RBRACE) {
			break
		}
	}
	p.expect(lexer.RBRACE)
	return n
}

func (p *Parser) parseSwitchArm() *ast.SwitchArm {
	tok := p.cur()
	n := &ast.SwitchArm{Token: tok}
	n.Case = p.parseExpression(PLowest)
	p.expect(lexer.FAT_ARROW)
	if p.check(lexer.LBRACE) {
		n.Value = p.parseBlockExpr()
		p.accept(lexer.COMMA)
	} else {
		n.Value = p.parseExpression(PLowest)
		p.expect(lexer.COMMA)
	}
	return n
}

func (p *Parser) parseSwitchExpr() ast.Expression {
	tok := p.expect(lexer.SWITCH)
	n := &ast.SwitchExpr{Token: tok}
	n.Subject = p.parseExpression(PLowest)
	p.expect(lexer.LBRACE)
	for !p.check(lexer.RBRACE) && !p.check(lexer.EOF) {
		n.Arms = append(n.Arms, p.parseSwitchArm())
		if p.check(lexer.RBRACE) {
			break
		}
	}
	p.expect(lexer.RBRACE)
	return n
}

func (p *Parser) parseTryExpr() ast.Expression {
	tok := p.expect(lexer.TRY)
	n := &ast.TryExpr{Token: tok}
	n.Value = p.parseExpression(PLowest)
	if p.check(lexer.ELSE) {
		p.advance()
		if p.check(lexer.PIPE) {
			p.advance()
			n.ErrName = p.expect(lexer.IDENT).Literal
			p.expect(lexer.PIPE)
		}
		if p.check(lexer.LBRACE) {
			n.ElseValue = p.parseBlockExpr()
		} else {
			n.ElseValue = p.parseExpression(PLowest)
		}
	}
	return n
}

func (p *Parser) parseStringLiteralExpr() ast.Expression {
	tok := p.expect(lexer.STRING)
	n := &ast.StringLiteralExpr{Token: tok, Value: tok.Literal}
	return n
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	t := p.cur()
	p.advance()
	n := new(ast.IntegerLiteralExpr)
	n.Token = t

	value, err := strconv.ParseInt(t.Literal, 0, 64)
	if err != nil {
		p.error(t, "Unable to parse int: %s", t.Literal)
		return nil
	}
	n.Value = value
	return n
}

func (p *Parser) parseForInExpr() ast.Expression {
	tok := p.expect(lexer.FOR)
	n := &ast.ForInExpr{Token: tok}
	n.IndexBinding = p.expect(lexer.IDENT)
	p.expect(lexer.COMMA)
	n.ValueBinding = p.expect(lexer.IDENT)
	p.expect(lexer.IN)
	n.Container = p.parseExpression(PLowest)
	n.Body = p.parseBlockExpr()
	return n
}

func (p *Parser) parseForWhileExpr() ast.Expression {
	tok := p.expect(lexer.FOR)
	n := &ast.ForWhileExpr{Token: tok}
	if !p.check(lexer.LBRACE) {
		n.Condition = p.parseExpression(PLowest)
	}
	n.Body = p.parseBlockExpr()
	return n
}

func (p *Parser) parseForExpr() ast.Expression {
	if p.peekCheck(1, lexer.IDENT) && p.peekCheck(2, lexer.COMMA) {
		return p.parseForInExpr()
	} else {
		return p.parseForWhileExpr()
	}
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	t := p.expect(lexer.FLOAT)
	v, err := strconv.ParseFloat(t.Literal, 64)
	if err != nil {
		p.error(t, "unable to parse float: %s", t.Literal)
		return nil
	}
	return &ast.FloatLiteralExpr{Token: t, Value: v}
}

func (p *Parser) parseArgument() *ast.Argument {
	n := &ast.Argument{Token: p.cur()}
	if p.check(lexer.IDENT) && p.peekCheck(1, lexer.ASSIGN) {
		n.Name = p.expect(lexer.IDENT).Literal
		p.expect(lexer.ASSIGN)
	}
	n.Value = p.parseExpression(PLowest)
	return n
}

func (p *Parser) parseMemberExpr(left ast.Expression) ast.Expression {
	op := p.expect(lexer.DOT)
	n := &ast.MemberExpr{Token: op, Container: left}
	n.Member = p.expect(lexer.IDENT).Literal
	return n
}

func (p *Parser) parseCallExpr(left ast.Expression) ast.Expression {
	op := p.expect(lexer.LPAREN)
	n := &ast.CallExpr{Token: op, Left: left}
	if !p.check(lexer.RPAREN) && !p.check(lexer.EOF) {
		n.Args = append(n.Args, p.parseArgument())
		for p.check(lexer.COMMA) {
			p.advance()
			n.Args = append(n.Args, p.parseArgument())
		}
	}
	p.expect(lexer.RPAREN)
	return n
}

func (p *Parser) parseSequenceLiteralExpr() ast.Expression {
	op := p.expect(lexer.LBRACK)
	n := &ast.SequenceLiteral{Token: op}
	if !p.check(lexer.RBRACK) {
		n.Elements = append(n.Elements, p.parseExpression(PLowest))
		for p.check(lexer.COMMA) {
			p.advance()
			n.Elements = append(n.Elements, p.parseExpression(PLowest))
		}
	}
	p.expect(lexer.RBRACK)
	return n
}

func (p *Parser) parseNotExpr() ast.Expression {
	tok := p.expect(lexer.NOT)
	n := &ast.NotExpr{Token: tok}
	n.Value = p.parseExpression(PUnary)
	return n
}

func (p *Parser) parseNegativeExpr() ast.Expression {
	tok := p.expect(lexer.MINUS)
	n := &ast.NegativeExpr{Token: tok}
	n.Value = p.parseExpression(PUnary)
	return n
}

func (p *Parser) parseRefExpr() ast.Expression {
	tok := p.expect(lexer.REF)
	n := &ast.RefExpr{Token: tok}
	n.Value = p.parseExpression(PUnary)
	return n
}

func (p *Parser) parseIfExpr() ast.Expression {
	tok := p.expect(lexer.IF)
	n := &ast.IfExpr{Token: tok}
	n.Condition = p.parseExpression(PLowest)
	n.Then = p.parseBlockExpr()
	if p.check(lexer.ELSE) {
		p.advance()
		n.Else = p.parseExpression(PLowest)
	}
	return n
}

func (p *Parser) parseType() ast.Type {
	switch p.cur().Type {
	case lexer.LBRACK:
		tok := p.expect(lexer.LBRACK)
		if p.check(lexer.RBRACK) {
			n := &ast.SliceType{Token: tok}
			p.advance()
			n.ElementType = p.parseType()
			return n
		} else if p.check(lexer.INT) {
			n := &ast.ArrayType{Token: tok}
			v, err := strconv.ParseInt(p.expect(lexer.INT).Literal, 10, 64)
			if err != nil {
				n.Length = v
			}
			p.expect(lexer.RBRACK)
			n.ElementType = p.parseType()
			return n
		}
	case lexer.AST:
		tok := p.expect(lexer.AST)
		n := &ast.PointerType{Token: tok}
		n.ElementType = p.parseType()
		return n
	case lexer.IDENT:
		tok := p.expect(lexer.IDENT)
		n := &ast.NamedType{Token: tok}
		if p.check(lexer.DOT) {
			n.Path = append(n.Path, tok.Literal)
			for p.check(lexer.DOT) {
				p.advance()
				if p.peekCheck(1, lexer.DOT) {
					n.Path = append(n.Path, p.expect(lexer.IDENT).Literal)
				}
			}
			n.Name = p.expect(lexer.IDENT).Literal
		} else {
			n.Name = tok.Literal
		}
		if p.cur().Type == lexer.LBRACK {
			p.advance()
			n.Args = append(n.Args, p.parseType())
			for p.cur().Type == lexer.COMMA {
				p.advance()
				n.Args = append(n.Args, p.parseType())
			}
			p.expect(lexer.RBRACK)
		}
		return n
	case lexer.FUNC:
		return p.parseFunctionType()
	default:
		p.error(p.cur(), "unexpected token for type: %s", p.cur().Literal)
		return nil
	}
	return nil
}

func (p *Parser) parseBinaryExpr(left ast.Expression) ast.Expression {
	op := p.cur()
	p.advance()
	right := p.parseExpression(PMap[op.Type] + 1)
	return &ast.InfixExpr{
		Token:    op,
		Left:     left,
		Operator: op.Literal,
		Right:    right,
	}
}
func (p *Parser) parseGroupingExpr() ast.Expression {
	p.expect(lexer.LPAREN)
	inner := p.parseExpression(PLowest)
	p.expect(lexer.RPAREN)
	return inner
}

func (p *Parser) parseField() *ast.Field {
	pub := false
	if p.check(lexer.PUB) {
		p.advance()
		pub = true
	}
	tok := p.expect(lexer.IDENT)
	n := &ast.Field{Token: tok, Name: tok.Literal, IsPublic: pub}
	p.expect(lexer.COLON)
	n.Type = p.parseType()
	return n
}

func (p *Parser) parseInterfaceType() ast.Type {
	tok := p.expect(lexer.INTERFACE)
	n := &ast.InterfaceType{Token: tok}
	p.expect(lexer.LBRACE)
	if !p.check(lexer.RBRACE) {
		p.expect(lexer.FUNC)
		n.Methods = append(n.Methods, p.parseFunctionSignature())
		for p.check(lexer.FUNC) {
			p.advance()
			n.Methods = append(n.Methods, p.parseFunctionSignature())
			if p.check(lexer.RBRACE) {
				break
			}
		}
	}
	p.expect(lexer.RBRACE)
	return n
}

func (p *Parser) parseObjectType() ast.Type {
	tok := p.expect(lexer.OBJ)
	n := &ast.ObjectType{Token: tok}
	p.expect(lexer.LBRACE)
	if !p.check(lexer.RBRACE) {
		n.Fields = append(n.Fields, p.parseField())
		for p.check(lexer.COMMA) {
			p.advance()
			if p.check(lexer.RBRACE) {
				break
			}
			n.Fields = append(n.Fields, p.parseField())
		}
	}
	p.expect(lexer.RBRACE)
	return n
}

func (p *Parser) parseEnumMember() *ast.EnumMember {
	tok := p.expect(lexer.IDENT)
	n := &ast.EnumMember{Token: tok, Name: tok.Literal}
	if p.check(lexer.LPAREN) {
		p.advance()
		if !p.check(lexer.RPAREN) {
			n.Fields = append(n.Fields, p.parseField())
			for p.check(lexer.COMMA) {
				p.advance()
				if p.check(lexer.RPAREN) {
					break
				}
				n.Fields = append(n.Fields, p.parseField())
			}
		}
		p.expect(lexer.RPAREN)
	}
	return n
}

func (p *Parser) parseEnumType() ast.Type {
	tok := p.expect(lexer.ENUM)
	n := &ast.EnumType{Token: tok}
	p.expect(lexer.LBRACE)
	if !p.check(lexer.RBRACE) {
		n.Members = append(n.Members, p.parseEnumMember())
		for p.check(lexer.COMMA) {
			p.advance()
			if p.check(lexer.RBRACE) {
				break
			}
			n.Members = append(n.Members, p.parseEnumMember())
		}
	}
	p.expect(lexer.RBRACE)
	return n
}

func NewParser(input []lexer.Token) *Parser {
	p := new(Parser)
	p.prefixFuncs = make(map[lexer.T_Token]prefixParseFunc)
	p.infixFuncs = make(map[lexer.T_Token]infixParseFunc)
	p.tokens = input

	p.prefixFuncs[lexer.INT] = p.parseIntegerLiteral
	p.prefixFuncs[lexer.FLOAT] = p.parseFloatLiteral
	p.prefixFuncs[lexer.IDENT] = p.parseIdentifierExpr
	p.prefixFuncs[lexer.LPAREN] = p.parseGroupingExpr
	p.prefixFuncs[lexer.LBRACK] = p.parseSequenceLiteralExpr
	p.prefixFuncs[lexer.STRING] = p.parseStringLiteralExpr
	p.prefixFuncs[lexer.TRUE] = p.parseBoolLiteralExpr
	p.prefixFuncs[lexer.FALSE] = p.parseBoolLiteralExpr
	p.prefixFuncs[lexer.TRY] = p.parseTryExpr
	p.prefixFuncs[lexer.LBRACE] = p.parseBlockExpr
	p.prefixFuncs[lexer.SWITCH] = p.parseSwitchExpr
	p.prefixFuncs[lexer.IF] = p.parseIfExpr
	p.prefixFuncs[lexer.NOT] = p.parseNotExpr
	p.prefixFuncs[lexer.MINUS] = p.parseNegativeExpr
	p.prefixFuncs[lexer.FOR] = p.parseForExpr
	p.prefixFuncs[lexer.REF] = p.parseRefExpr

	p.infixFuncs[lexer.LPAREN] = p.parseCallExpr
	p.infixFuncs[lexer.DOT] = p.parseMemberExpr
	p.infixFuncs[lexer.LBRACK] = p.parseSubscriptExpr

	for _, t := range []lexer.T_Token{
		lexer.PLUS, lexer.MINUS, lexer.DIV, lexer.AST, lexer.EQ,
		lexer.NEQ, lexer.LT, lexer.LTE, lexer.GT, lexer.GTE,
		lexer.AND, lexer.OR, lexer.MOD,
	} {
		p.infixFuncs[t] = p.parseBinaryExpr
	}
	return p
}

func (p *Parser) parseVarStmt(pub bool) *ast.VarStmt {
	n := new(ast.VarStmt)
	n.IsPublic = pub
	n.Token = p.expect(lexer.VAR)
	n.Name = p.expect(lexer.IDENT)
	if p.cur().Type == lexer.COLON {
		p.advance()
		n.Type = p.parseType()
	}
	p.expect(lexer.ASSIGN)
	n.Value = p.parseExpression(PLowest)
	p.expect(lexer.END)
	return n
}

func (p *Parser) parseSubscriptExpr(left ast.Expression) ast.Expression {
	tok := p.expect(lexer.LBRACK)
	n := &ast.SubscriptExpr{Token: tok, Receiver: left}
	n.Argument = p.parseExpression(PLowest)
	p.expect(lexer.RBRACK)
	return n
}

func (p *Parser) parseIdentifierExpr() ast.Expression {
	tok := p.expect(lexer.IDENT)
	return &ast.IdentifierExpr{Token: tok, Value: tok.Literal}
}

func (p *Parser) parseExpression(minPrec Precedence) ast.Expression {
	prefix, ok := p.prefixFuncs[p.cur().Type]
	if !ok {
		p.error(p.cur(), "expected expression, got %s", p.cur().Literal)
		return nil
	}
	left := prefix()

	for {
		prec, isInfix := PMap[p.cur().Type]
		if !isInfix || prec < minPrec {
			break
		}
		infixF, ok := p.infixFuncs[p.cur().Type]
		if ok {
			left = infixF(left)
		} else {
			p.error(p.cur(), "No infix function for %s", p.cur().Literal)
			break
		}
	}
	return left
}

func (p *Parser) parseStatement() ast.Statement {
	var stmt ast.Statement

	pub := p.accept(lexer.PUB)

	switch p.cur().Type {
	case lexer.VAR:
		stmt = p.parseVarStmt(pub)
	case lexer.RETURN:
		stmt = p.parseReturnStatement()
	case lexer.EMIT:
		stmt = p.parseEmitStatement()
	case lexer.BREAK:
		stmt = &ast.BreakStmt{Token: p.expect(lexer.BREAK)}
		p.expect(lexer.END)
	case lexer.CONT:
		stmt = &ast.ContinueStmt{Token: p.expect(lexer.CONT)}
		p.expect(lexer.END)
	default:
		stmt = p.parseOtherStmt()
	}
	return stmt
}

func (p *Parser) parseAssignStmt(left ast.Expression) ast.Statement {
	tok := p.expectOneOf(assigns()...)
	n := &ast.AssignStmt{Token: tok, Target: left}
	n.Value = p.parseExpression(PLowest)
	return n
}

func assigns() []lexer.T_Token {
	return []lexer.T_Token{
		lexer.ASSIGN, lexer.PLUS_ASSIGN, lexer.MINUS_ASSIGN,
		lexer.DIV_ASSIGN, lexer.MUL_ASSIGN, lexer.MOD_ASSIGN,
	}
}

func (p *Parser) parseOtherStmt() ast.Statement {
	var n ast.Statement
	tok := p.cur()
	left := p.parseExpression(PLowest)
	if slices.Contains(assigns(), p.cur().Type) {
		n = p.parseAssignStmt(left)
	} else {
		n = &ast.ExpressionStatement{Token: tok, Expression: left}
	}
	if !p.recallCheck(1, lexer.RBRACE) {
		p.expect(lexer.END)
	}
	return n
}

func (p *Parser) parseImportStmt() *ast.ImportStmt {
	tok := p.expect(lexer.IMPORT)
	n := &ast.ImportStmt{Token: tok}
	n.Path = append(n.Path, p.expect(lexer.IDENT))
	for p.check(lexer.DCOLON) {
		p.advance()
		n.Path = append(n.Path, p.expect(lexer.IDENT))
	}
	p.expect(lexer.END)
	return n
}

func (p *Parser) parseParam() *ast.Parameter {
	name := p.expect(lexer.IDENT)
	var paramType ast.Type
	if p.check(lexer.COLON) {
		p.advance()
		paramType = p.parseType()
	}
	return &ast.Parameter{Token: name, Name: name.Literal, Type: paramType}
}

func (p *Parser) parseFunctionSignature() *ast.FunctionSignature {
	tok := p.expect(lexer.IDENT)
	n := &ast.FunctionSignature{Token: tok, Name: tok.Literal}
	if p.check(lexer.LBRACK) {
		p.advance()
		n.TypeParams = append(n.TypeParams, p.parseTypeParam())
		for p.check(lexer.COMMA) {
			p.advance()
			n.TypeParams = append(n.TypeParams, p.parseTypeParam())
			if p.check(lexer.RBRACK) {
				break
			}
		}
		p.expect(lexer.RBRACK)
	}
	p.expect(lexer.LPAREN)
	if p.check(lexer.IDENT) {
		n.Parameters = append(n.Parameters, p.parseParam())
		for p.check(lexer.COMMA) {
			p.advance()
			n.Parameters = append(n.Parameters, p.parseParam())
		}
	}
	p.expect(lexer.RPAREN)
	if p.check(lexer.RETURN_ARROW) {
		p.advance()
		n.ReturnType = p.parseType()
	}
	return n
}

func (p *Parser) parseFunctionStmt(pub bool) *ast.FunctionStmt {
	p.expect(lexer.FUNC)
	n := &ast.FunctionStmt{IsPublic: pub}
	n.Signature = p.parseFunctionSignature()
	n.Body = p.parseBlockExpr()
	return n
}

func (p *Parser) parseFunctionTypeParam() *ast.Parameter {
	n := &ast.Parameter{Token: p.cur()}
	if p.check(lexer.IDENT) && p.peekCheck(1, lexer.COLON) {
		n.Name = p.expect(lexer.IDENT).Literal
		p.expect(lexer.COLON)
	}
	n.Type = p.parseType()
	return n
}

func (p *Parser) parseFunctionType() ast.Type {
	tok := p.expect(lexer.FUNC)
	n := &ast.FunctionType{Token: tok}
	p.expect(lexer.LPAREN)
	if !p.check(lexer.RPAREN) {
		n.Parameters = append(n.Parameters, p.parseFunctionTypeParam())
		for p.check(lexer.COMMA) {
			p.advance()
			n.Parameters = append(n.Parameters, p.parseFunctionTypeParam())
		}
	}
	p.expect(lexer.RPAREN)
	if p.check(lexer.RETURN_ARROW) {
		p.advance()
		n.ReturnType = p.parseType()
	}
	return n
}

func (p *Parser) parseEmitStatement() ast.Statement {
	tok := p.expect(lexer.EMIT)
	n := &ast.EmitStmt{Token: tok}
	if !p.check(lexer.END) {
		n.Value = p.parseExpression(PLowest)
	}
	if !p.recallCheck(1, lexer.RBRACE) {
		p.expect(lexer.END)
	}
	return n
}

func (p *Parser) parseReturnStatement() *ast.ReturnStmt {
	n := &ast.ReturnStmt{Token: p.expect(lexer.RETURN)}
	if !p.check(lexer.END) {
		n.Value = p.parseExpression(PLowest)
	}
	if !p.recallCheck(1, lexer.RBRACE) {
		p.expect(lexer.END)
	}
	return n
}

func (p *Parser) parseTypeParam() *ast.TypeParameter {
	tok := p.expect(lexer.IDENT)
	return &ast.TypeParameter{Token: tok}
}

func (p *Parser) parseImplDeclStmt() ast.Statement {
	tok := p.expect(lexer.IMPL)
	n := &ast.ImplDeclStmt{Token: tok}
	interfaceOrTarget := p.parseType()
	if p.check(lexer.FOR) {
		n.Interface = interfaceOrTarget
		p.advance()
		n.Target = p.parseType()
	} else {
		n.Target = interfaceOrTarget
	}
	p.expect(lexer.LBRACE)
	if !p.check(lexer.RBRACE) && !p.check(lexer.EOF) {
		pub := p.accept(lexer.PUB)
		n.Methods = append(n.Methods, p.parseFunctionStmt(pub))
		for p.check(lexer.FUNC) || p.check(lexer.PUB) {
			pub2 := p.accept(lexer.PUB)
			n.Methods = append(n.Methods, p.parseFunctionStmt(pub2))
			if p.check(lexer.RBRACE) {
				break
			}
		}
	}
	p.expect(lexer.RBRACE)
	return n
}

func (p *Parser) parseTypeDecl(pub bool) ast.Statement {
	tok := p.expect(lexer.TYPE)
	ident := p.expect(lexer.IDENT)
	var parameters []*ast.TypeParameter
	if p.check(lexer.LBRACK) {
		p.advance()
		parameters = append(parameters, p.parseTypeParam())
		for p.check(lexer.COMMA) {
			p.advance()
			if p.check(lexer.RBRACK) {
				break
			}
			parameters = append(parameters, p.parseTypeParam())
		}
		p.expect(lexer.RBRACK)
	}
	if p.check(lexer.ASSIGN) {
		n := &ast.TypeAliasStmt{
			Token:      tok,
			Name:       ident.Literal,
			IsPublic:   pub,
			TypeParams: parameters,
		}
		p.advance()
		n.Value = p.parseType()
		p.expect(lexer.END)
		return n
	} else {
		n := &ast.TypeDeclStmt{
			Token:      tok,
			Name:       ident.Literal,
			IsPublic:   pub,
			TypeParams: parameters,
		}
		switch p.cur().Type {
		case lexer.OBJ:
			n.Definition = p.parseObjectType()
		case lexer.ENUM:
			n.Definition = p.parseEnumType()
		case lexer.INTERFACE:
			n.Definition = p.parseInterfaceType()
		default:
			p.error(p.cur(), "Invalid TypeDecl: %s", p.cur().Literal)
		}
		return n
	}
}

func (p *Parser) next() ast.Statement {
	pub := p.accept(lexer.PUB)

	switch p.cur().Type {
	case lexer.IMPORT:
		return p.parseImportStmt()
	case lexer.FUNC:
		return p.parseFunctionStmt(pub)
	case lexer.TYPE:
		return p.parseTypeDecl(pub)
	case lexer.IMPL:
		return p.parseImplDeclStmt()
	default:
		p.error(p.cur(), "Unexpected token: %s", p.cur().Literal)
		p.pos++
		return nil
	}
}

func (p *Parser) Parse() *ast.Program {
	program := new(ast.Program)

	for p.cur().Type != lexer.EOF {
		n := p.next()
		if n != nil {
			program.Statements = append(program.Statements, n)
		}
	}
	return program
}
