package lexer

import (
	"fmt"
)

type T_Token string

const (
	IDENT        T_Token = "IDENT"
	INT          T_Token = "NUM"
	FLOAT        T_Token = "FLOAT"
	ERR          T_Token = "ERR"
	LPAREN       T_Token = "LPAREN"
	RPAREN       T_Token = "RPAREN"
	LBRACK       T_Token = "LBRACK"
	RBRACK       T_Token = "RBRACK"
	COMMA        T_Token = "COMMA"
	LBRACE       T_Token = "LBRACE"
	RBRACE       T_Token = "RBRACE"
	EOF          T_Token = "EOF"
	ASSIGN       T_Token = "ASSIGN"
	NOT          T_Token = "NOT"
	EQ           T_Token = "EQ"
	NEQ          T_Token = "NEQ"
	PLUS         T_Token = "PLUS"
	PLUS_ASSIGN  T_Token = "PLUS_ASSIGN"
	MINUS        T_Token = "MINUS"
	MINUS_ASSIGN T_Token = "MINUS_ASSIGN"
	DIV          T_Token = "DIV"
	DIV_ASSIGN   T_Token = "DIV_ASSIGN"
	MUL          T_Token = "MUL"
	MUL_ASSIGN   T_Token = "MUL_ASSIGN"
	DOT          T_Token = "DOT"
	GRANT        T_Token = "GRANT"
	COLON        T_Token = "COLON"
	DCOLON       T_Token = "DCOLON"
	GT           T_Token = "GT"
	GTE          T_Token = "GTE"
	LT           T_Token = "LT"
	LTE          T_Token = "LTE"
	PIPE         T_Token = "PIPE"
	STRING       T_Token = "STRING"
	IF           T_Token = "IF"
	ELSE         T_Token = "ELSE"
	RETURN       T_Token = "RETURN"
	TRY          T_Token = "TRY"
	MUST         T_Token = "MUST"
	FUNC         T_Token = "FUNC"
	TYPE         T_Token = "TYPE"
	IMPL         T_Token = "IMPL"
	RETURN_ARROW T_Token = "RETURN_ARROW"
	FAT_ARROW    T_Token = "FAT_ARROW"
	SWITCH       T_Token = "SWITCH"
	FOR          T_Token = "FOR"
	IN           T_Token = "IN"
	VAR          T_Token = "VAR"
	BREAK        T_Token = "BREAK"
	CONT         T_Token = "CONT"
	IMPORT       T_Token = "IMPORT"
	PUB          T_Token = "PUB"
	INTERFACE    T_Token = "INTERFACE"
	ENUM         T_Token = "ENUM"
	OBJ          T_Token = "OBJ"
	AND          T_Token = "AND"
	OR           T_Token = "OR"
	TRUE         T_Token = "TRUE"
	FALSE        T_Token = "FALSE"
	END          T_Token = "END"
	EMIT         T_Token = "EMIT"
	MOD          T_Token = "MOD"
	MOD_ASSIGN   T_Token = "MOD_ASSIGN"
	MUT          T_Token = "MUT"
)

var keywords = map[string]T_Token{
	"if":        IF,
	"else":      ELSE,
	"return":    RETURN,
	"try":       TRY,
	"must":      MUST,
	"fn":        FUNC,
	"type":      TYPE,
	"impl":      IMPL,
	"switch":    SWITCH,
	"for":       FOR,
	"in":        IN,
	"var":       VAR,
	"break":     BREAK,
	"continue":  CONT,
	"import":    IMPORT,
	"pub":       PUB,
	"interface": INTERFACE,
	"enum":      ENUM,
	"object":    OBJ,
	"and":       AND,
	"or":        OR,
	"true":      TRUE,
	"false":     FALSE,
	"mut":       MUT,
	";":         END,
}

func lookupIdent(ident string) T_Token {
	if ttok, isKeyword := keywords[ident]; isKeyword {
		return ttok
	}
	return IDENT
}

type Position struct {
	Line   int
	Column int
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

type Token struct {
	Type     T_Token
	Literal  string
	Position Position
}

func (t Token) String() string {
	return fmt.Sprintf(
		"{type: %s, literal: %q, pos: %s}",
		t.Type,
		t.Literal,
		t.Position.String(),
	)
}

type Lexer struct {
	input         string
	pos           int
	readPos       int
	ch            byte
	currentLine   int
	currentColumn int
}

func NewLexer(input string) *Lexer {
	l := new(Lexer)
	l.input = input
	l.currentLine = 1
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
		l.currentColumn++
	}
	l.pos = l.readPos
	l.readPos++
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' || l.ch == '\n' {
		switch l.ch {
		case ' ', '\t':
			for l.peek() == ' ' || l.peek() == '\t' {
				l.readChar()
			}
			l.readChar()
		case '\r':
			if l.peek() == '\n' {
				l.readChar()
			}
			l.readChar()
			l.currentLine++
			l.currentColumn = 1
		case '\n':
			l.readChar()
			l.currentLine++
			l.currentColumn = 1
		}
	}
}

func (l *Lexer) nextToken() Token {
	l.skipWhitespace()
	var t Token

	pos := Position{Line: l.currentLine, Column: l.currentColumn}
	switch l.ch {
	case '(':
		t = Token{Type: LPAREN, Literal: string(l.ch), Position: pos}
	case ')':
		t = Token{Type: RPAREN, Literal: string(l.ch), Position: pos}
	case '[':
		t = Token{Type: LBRACK, Literal: string(l.ch), Position: pos}
	case ']':
		t = Token{Type: RBRACK, Literal: string(l.ch), Position: pos}
	case '{':
		t = Token{Type: LBRACE, Literal: string(l.ch), Position: pos}
	case '}':
		t = Token{Type: RBRACE, Literal: string(l.ch), Position: pos}
	case '+':
		if l.peek() == '=' {
			t = Token{Type: PLUS_ASSIGN, Literal: "+=", Position: pos}
			l.readChar()
		} else {
			t = Token{Type: PLUS, Literal: string(l.ch), Position: pos}
		}
	case '-':
		if l.peek() == '>' {
			t = Token{Type: RETURN_ARROW, Literal: "->", Position: pos}
			l.readChar()
		} else if l.peek() == '=' {
			t = Token{Type: MINUS_ASSIGN, Literal: "-=", Position: pos}
			l.readChar()
		} else {
			t = Token{Type: MINUS, Literal: string(l.ch), Position: pos}
		}
	case '/':
		if l.peek() == '/' {
			l.discardComment()
			return l.nextToken()
		} else if l.peek() == '=' {
			t = Token{Type: DIV_ASSIGN, Literal: "/=", Position: pos}
			l.readChar()
		} else {
			t = Token{Type: DIV, Literal: string(l.ch), Position: pos}
		}
	case ',':
		t = Token{Type: COMMA, Literal: string(l.ch), Position: pos}
	case '*':
		if l.peek() == '=' {
			t = Token{Type: MUL_ASSIGN, Literal: "*=", Position: pos}
			l.readChar()
		} else {
			t = Token{Type: MUL, Literal: string(l.ch), Position: pos}
		}
	case '@':
		t = Token{Type: GRANT, Literal: string(l.ch), Position: pos}
	case ':':
		if l.peek() == ':' {
			t = Token{Type: DCOLON, Literal: "::", Position: pos}
			l.readChar()
		} else {
			t = Token{Type: COLON, Literal: string(l.ch), Position: pos}
		}
	case '<':
		if l.peek() == '=' {
			t = Token{Type: LTE, Literal: "<=", Position: pos}
			l.readChar()
		} else {
			t = Token{Type: LT, Literal: string(l.ch), Position: pos}
		}
	case '>':
		if l.peek() == '=' {
			t = Token{Type: GTE, Literal: ">=", Position: pos}
			l.readChar()
		} else if l.peek() == '>' {
			t = Token{Type: EMIT, Literal: ">>", Position: pos}
			l.readChar()
		} else {
			t = Token{Type: GT, Literal: string(l.ch), Position: pos}
		}
	case '!':
		if l.peek() == '=' {
			t = Token{Type: NEQ, Literal: "!=", Position: pos}
			l.readChar()
		} else {
			t = Token{Type: NOT, Literal: "!", Position: pos}
		}
	case '%':
		if l.peek() == '=' {
			t = Token{Type: MOD_ASSIGN, Literal: "%=", Position: pos}
			l.readChar()
		} else {
			t = Token{Type: MOD, Literal: string(l.ch), Position: pos}
		}
	case '=':
		if l.peek() == '=' {
			t = Token{Type: EQ, Literal: "==", Position: pos}
			l.readChar()
		} else if l.peek() == '>' {
			t = Token{Type: FAT_ARROW, Literal: "=>", Position: pos}
			l.readChar()
		} else {
			t = Token{Type: ASSIGN, Literal: string(l.ch), Position: pos}
		}
	case '.':
		t = Token{Type: DOT, Literal: string(l.ch), Position: pos}
	case '|':
		t = Token{Type: PIPE, Literal: string(l.ch), Position: pos}
	case 0:
		t = Token{Type: EOF, Literal: string(l.ch), Position: pos}
	case '"':
		t.Type = STRING
		t.Position = pos
		t.Literal = l.readString()
		return t
	case ';':
		t = Token{Type: END, Literal: string(l.ch), Position: pos}
	default:
		t.Position = pos
		if isLetter(l.ch) {
			t.Literal = l.readIdentifier()
			t.Type = lookupIdent(t.Literal)
			return t
		} else if isDigit(l.ch) {
			t.Literal = l.readNumber()
			if l.ch == '.' {
				if isDigit(l.peek()) {
					l.readChar()
					t.Type = FLOAT
					t.Literal += "."
					t.Literal += l.readNumber()
				} else {
					t.Type = ERR
				}
				return t
			}
			t.Type = INT
			return t
		} else {
			t.Literal = string(l.ch)
			t.Type = ERR
		}
	}

	l.readChar()
	return t
}

func (l *Lexer) peek() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *Lexer) readIdentifier() string {
	position := l.pos
	if isLetter(l.ch) {
		l.readChar()
	}
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.pos]
}

func (l *Lexer) readNumber() string {
	position := l.pos
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.pos]
}

func (l *Lexer) discardComment() {
	for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
		l.readChar()
	}
}

func (l *Lexer) readString() string {
	l.readChar()
	start := l.pos
	for {
		if l.ch == '"' || l.ch == 0 || l.ch == '\r' || l.ch == '\n' {
			break
		}
		l.readChar()
	}
	lit := l.input[start:l.pos]
	if l.ch == '"' {
		l.readChar()
	}
	return lit
}

func (l *Lexer) Run() []Token {
	var out []Token
	for {
		t := l.nextToken()
		out = append(out, t)
		if t.Type == EOF {
			break
		}
	}
	return out
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
