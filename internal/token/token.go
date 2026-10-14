// Package token defines the lexical tokens of Stellar and source positions.
package token

import "fmt"

// Pos is a location in a source file. Line and Col are 1-based; Col counts
// runes, not bytes. The zero Pos means "no position".
type Pos struct {
	Line int
	Col  int
}

func (p Pos) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

// IsValid reports whether p refers to a real source location.
func (p Pos) IsValid() bool {
	return p.Line > 0
}

// Kind identifies the type of a token.
type Kind int

const (
	ILLEGAL Kind = iota
	EOF
	NEWLINE // statement terminator, inserted by the lexer (see lexer docs)

	literalBegin
	IDENT
	INT
	FLOAT
	STRING
	literalEnd

	operatorBegin
	PLUS    // +
	MINUS   // -
	STAR    // *
	SLASH   // /
	PERCENT // %

	EQ     // ==
	NEQ    // !=
	LT     // <
	LTE    // <=
	GT     // >
	GTE    // >=
	ASSIGN // =

	LPAREN   // (
	RPAREN   // )
	LBRACE   // {
	RBRACE   // }
	LBRACKET // [
	RBRACKET // ]

	COMMA    // ,
	COLON    // :
	DOT      // .
	DOTDOT   // ..
	ARROW    // ->
	FATARROW // =>
	PIPE     // |
	QUESTION // ?
	operatorEnd

	keywordBegin
	AND
	AS
	BREAK
	CONTINUE
	ELSE
	ENUM
	FALSE
	FN
	FOR
	IF
	IMPL
	IMPORT
	IN
	INTERFACE
	IS
	LET
	MATCH
	MUT
	NOT
	OBJECT
	OR
	PUB
	RETURN
	TRUE
	TRY
	TYPE
	VAR
	WHILE
	keywordEnd
)

var kindNames = map[Kind]string{
	ILLEGAL: "ILLEGAL",
	EOF:     "end of file",
	NEWLINE: "newline",

	IDENT:  "identifier",
	INT:    "integer literal",
	FLOAT:  "float literal",
	STRING: "string literal",

	PLUS:    "+",
	MINUS:   "-",
	STAR:    "*",
	SLASH:   "/",
	PERCENT: "%",

	EQ:     "==",
	NEQ:    "!=",
	LT:     "<",
	LTE:    "<=",
	GT:     ">",
	GTE:    ">=",
	ASSIGN: "=",

	LPAREN:   "(",
	RPAREN:   ")",
	LBRACE:   "{",
	RBRACE:   "}",
	LBRACKET: "[",
	RBRACKET: "]",

	COMMA:    ",",
	COLON:    ":",
	DOT:      ".",
	DOTDOT:   "..",
	ARROW:    "->",
	FATARROW: "=>",
	PIPE:     "|",
	QUESTION: "?",

	AND:       "and",
	AS:        "as",
	BREAK:     "break",
	CONTINUE:  "continue",
	ELSE:      "else",
	ENUM:      "enum",
	FALSE:     "false",
	FN:        "fn",
	FOR:       "for",
	IF:        "if",
	IMPL:      "impl",
	IMPORT:    "import",
	IN:        "in",
	INTERFACE: "interface",
	IS:        "is",
	LET:       "let",
	MATCH:     "match",
	MUT:       "mut",
	NOT:       "not",
	OBJECT:    "object",
	OR:        "or",
	PUB:       "pub",
	RETURN:    "return",
	TRUE:      "true",
	TRY:       "try",
	TYPE:      "type",
	VAR:       "var",
	WHILE:     "while",
}

func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// IsLiteral reports whether k is an identifier or a literal.
func (k Kind) IsLiteral() bool { return literalBegin < k && k < literalEnd }

// IsOperator reports whether k is an operator or delimiter.
func (k Kind) IsOperator() bool { return operatorBegin < k && k < operatorEnd }

// IsKeyword reports whether k is a reserved word.
func (k Kind) IsKeyword() bool { return keywordBegin < k && k < keywordEnd }

var keywords = func() map[string]Kind {
	m := make(map[string]Kind)
	for k := keywordBegin + 1; k < keywordEnd; k++ {
		m[kindNames[k]] = k
	}
	return m
}()

// Lookup maps an identifier to its keyword kind, or IDENT if it is not a
// keyword.
func Lookup(ident string) Kind {
	if k, ok := keywords[ident]; ok {
		return k
	}
	return IDENT
}

// Token is a single lexical token. Lit holds the source text for
// identifiers, literals and ILLEGAL tokens; it is empty otherwise.
// SpaceBefore reports whether whitespace, a comment, or the start of the
// file precedes the token; it decides whether `Name{` is an object literal.
type Token struct {
	Kind        Kind
	Lit         string
	Pos         Pos
	SpaceBefore bool
}

func (t Token) String() string {
	switch {
	case t.Kind == IDENT || t.Kind.IsLiteral():
		return fmt.Sprintf("%s %q", t.Kind, t.Lit)
	case t.Kind == ILLEGAL:
		return fmt.Sprintf("illegal %q", t.Lit)
	default:
		return t.Kind.String()
	}
}
