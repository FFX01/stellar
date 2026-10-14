// Package lexer turns Stellar source text into tokens.
//
// Statements are terminated by newlines. Like Go, the lexer inserts a
// NEWLINE token at the end of a line only when the line's last token could
// end a statement: an identifier, a literal, `)`, `]`, `}`, or one of the
// keywords `return`, `break`, `continue`, `true`, `false`. A line ending in
// an operator or `,` therefore continues onto the next line. Consecutive
// blank lines produce at most one NEWLINE.
package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"stellar/internal/diag"
	"stellar/internal/token"
)

const eof = -1

// Lexer produces tokens from a source buffer.
type Lexer struct {
	src   []byte
	off   int // byte offset of ch
	width int // byte width of ch
	ch    rune
	line  int
	col   int

	// lastKind is the kind of the most recently returned token, used to
	// decide whether a newline terminates a statement.
	lastKind token.Kind
	errs     diag.List
}

// New returns a lexer positioned at the start of src.
func New(src []byte) *Lexer {
	l := &Lexer{src: src, line: 1, col: 1, lastKind: token.NEWLINE}
	l.readAt(0)
	return l
}

// Errors returns the diagnostics reported so far.
func (l *Lexer) Errors() diag.List { return l.errs }

// Tokenize lexes all of src, returning every token up to and including EOF.
func Tokenize(src []byte) ([]token.Token, diag.List) {
	l := New(src)
	var toks []token.Token
	for {
		t := l.Next()
		toks = append(toks, t)
		if t.Kind == token.EOF {
			return toks, l.Errors()
		}
	}
}

// readAt makes the rune at byte offset off the current rune.
func (l *Lexer) readAt(off int) {
	l.off = off
	if off >= len(l.src) {
		l.ch, l.width = eof, 0
		return
	}
	r, w := utf8.DecodeRune(l.src[off:])
	if r == utf8.RuneError && w == 1 {
		l.errorf(l.pos(), "invalid UTF-8 encoding")
	}
	l.ch, l.width = r, w
}

func (l *Lexer) advance() {
	if l.ch == eof {
		return
	}
	if l.ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	l.readAt(l.off + l.width)
}

func (l *Lexer) peek() rune {
	next := l.off + l.width
	if l.ch == eof || next >= len(l.src) {
		return eof
	}
	r, _ := utf8.DecodeRune(l.src[next:])
	return r
}

func (l *Lexer) pos() token.Pos { return token.Pos{Line: l.line, Col: l.col} }

func (l *Lexer) errorf(pos token.Pos, format string, args ...any) {
	l.errs.Add(pos, format, args...)
}

// terminates reports whether a newline after a token of kind k ends a
// statement.
func terminates(k token.Kind) bool {
	switch k {
	case token.IDENT, token.INT, token.FLOAT, token.STRING,
		token.RPAREN, token.RBRACKET, token.RBRACE,
		token.RETURN, token.BREAK, token.CONTINUE, token.TRUE, token.FALSE:
		return true
	}
	return false
}

// Next returns the next token.
func (l *Lexer) Next() token.Token {
	t := l.scan()
	l.lastKind = t.Kind
	return t
}

func (l *Lexer) scan() token.Token {
	space := l.off == 0 || l.lastKind == token.NEWLINE
	for {
		switch {
		case l.ch == ' ' || l.ch == '\t' || l.ch == '\r':
			l.advance()
		case l.ch == '\n':
			pos := l.pos()
			l.advance()
			if terminates(l.lastKind) {
				return token.Token{Kind: token.NEWLINE, Pos: pos}
			}
		case l.ch == '/' && l.peek() == '/':
			for l.ch != '\n' && l.ch != eof {
				l.advance()
			}
		case l.ch == '/' && l.peek() == '*':
			pos := l.pos()
			if l.skipBlockComment() && terminates(l.lastKind) {
				return token.Token{Kind: token.NEWLINE, Pos: pos}
			}
		default:
			t := l.scanToken()
			t.SpaceBefore = space
			return t
		}
		space = true
	}
}

// skipBlockComment consumes a /* */ comment and reports whether it spanned
// a newline, in which case it acts as a line break.
func (l *Lexer) skipBlockComment() bool {
	start := l.pos()
	l.advance() // /
	l.advance() // *
	sawNewline := false
	for {
		switch {
		case l.ch == eof:
			l.errorf(start, "unterminated block comment")
			return sawNewline
		case l.ch == '*' && l.peek() == '/':
			l.advance()
			l.advance()
			return sawNewline
		case l.ch == '\n':
			sawNewline = true
		}
		l.advance()
	}
}

func (l *Lexer) scanToken() token.Token {
	pos := l.pos()
	ch := l.ch

	if ch == eof {
		if terminates(l.lastKind) {
			return token.Token{Kind: token.NEWLINE, Pos: pos}
		}
		return token.Token{Kind: token.EOF, Pos: pos}
	}
	if isLetter(ch) {
		return l.scanIdent(pos)
	}
	if isDigit(ch) {
		return l.scanNumber(pos)
	}
	if ch == '"' {
		return l.scanString(pos)
	}

	l.advance()
	kind := token.ILLEGAL
	switch ch {
	case '+':
		kind = token.PLUS
	case '-':
		kind = l.pick('>', token.ARROW, token.MINUS)
	case '*':
		kind = token.STAR
	case '/':
		kind = token.SLASH
	case '%':
		kind = token.PERCENT
	case '=':
		switch l.ch {
		case '=':
			l.advance()
			kind = token.EQ
		case '>':
			l.advance()
			kind = token.FATARROW
		default:
			kind = token.ASSIGN
		}
	case '!':
		if l.ch == '=' {
			l.advance()
			kind = token.NEQ
		}
	case '<':
		kind = l.pick('=', token.LTE, token.LT)
	case '>':
		kind = l.pick('=', token.GTE, token.GT)
	case '(':
		kind = token.LPAREN
	case ')':
		kind = token.RPAREN
	case '{':
		kind = token.LBRACE
	case '}':
		kind = token.RBRACE
	case '[':
		kind = token.LBRACKET
	case ']':
		kind = token.RBRACKET
	case ',':
		kind = token.COMMA
	case ':':
		kind = token.COLON
	case '.':
		kind = l.pick('.', token.DOTDOT, token.DOT)
	case '|':
		kind = token.PIPE
	case '?':
		kind = token.QUESTION
	}
	if kind == token.ILLEGAL {
		lit := string(ch)
		if ch == '!' {
			l.errorf(pos, "unexpected '!'; use 'not' for logical negation")
		} else {
			l.errorf(pos, "unexpected character %q", ch)
		}
		return token.Token{Kind: token.ILLEGAL, Lit: lit, Pos: pos}
	}
	return token.Token{Kind: kind, Pos: pos}
}

// pick consumes next and returns yes if the current rune is next, else no.
func (l *Lexer) pick(next rune, yes, no token.Kind) token.Kind {
	if l.ch == next {
		l.advance()
		return yes
	}
	return no
}

func (l *Lexer) scanIdent(pos token.Pos) token.Token {
	start := l.off
	for isLetter(l.ch) || isDigit(l.ch) {
		l.advance()
	}
	lit := string(l.src[start:l.off])
	kind := token.Lookup(lit)
	if kind != token.IDENT {
		return token.Token{Kind: kind, Pos: pos}
	}
	return token.Token{Kind: token.IDENT, Lit: lit, Pos: pos}
}

func (l *Lexer) scanNumber(pos token.Pos) token.Token {
	start := l.off
	kind := token.INT

	if l.ch == '0' && (l.peek() == 'x' || l.peek() == 'b' || l.peek() == 'o') {
		l.advance()
		base := l.ch
		l.advance()
		valid := map[rune]func(rune) bool{'x': isHex, 'b': isBinary, 'o': isOctal}[base]
		if !l.scanDigits(valid) {
			l.errorf(pos, "%s literal has no digits", map[rune]string{'x': "hexadecimal", 'b': "binary", 'o': "octal"}[base])
		}
	} else {
		l.scanDigits(isDigit)
		// A '.' followed by a digit makes a float; "0..n" is a range.
		if l.ch == '.' && isDigit(l.peek()) {
			kind = token.FLOAT
			l.advance()
			l.scanDigits(isDigit)
		}
		if l.ch == 'e' || l.ch == 'E' {
			kind = token.FLOAT
			l.advance()
			if l.ch == '+' || l.ch == '-' {
				l.advance()
			}
			if !l.scanDigits(isDigit) {
				l.errorf(pos, "exponent has no digits")
			}
		}
	}

	if isLetter(l.ch) {
		l.errorf(l.pos(), "unexpected %q after number", l.ch)
		for isLetter(l.ch) || isDigit(l.ch) {
			l.advance()
		}
	}
	lit := string(l.src[start:l.off])
	if strings.HasSuffix(lit, "_") || strings.Contains(lit, "__") {
		l.errorf(pos, "'_' must separate successive digits")
	}
	return token.Token{Kind: kind, Lit: lit, Pos: pos}
}

// scanDigits consumes digits accepted by valid, plus '_' separators, and
// reports whether at least one digit was consumed.
func (l *Lexer) scanDigits(valid func(rune) bool) bool {
	n := 0
	for valid(l.ch) || l.ch == '_' {
		if l.ch != '_' {
			n++
		}
		l.advance()
	}
	return n > 0
}

func (l *Lexer) scanString(pos token.Pos) token.Token {
	start := l.off
	l.advance() // opening quote
	for {
		switch l.ch {
		case '"':
			l.advance()
			lit := string(l.src[start:l.off])
			if _, err := Unquote(lit); err != nil {
				l.errorf(pos, "%s", err)
			}
			return token.Token{Kind: token.STRING, Lit: lit, Pos: pos}
		case '\\':
			l.advance()
			if l.ch != eof && l.ch != '\n' {
				l.advance()
			}
		case '\n', eof:
			l.errorf(pos, "unterminated string literal")
			return token.Token{Kind: token.STRING, Lit: string(l.src[start:l.off]) + `"`, Pos: pos}
		default:
			l.advance()
		}
	}
}

// Unquote decodes a double-quoted string literal, including its quotes.
// Supported escapes: \n \t \r \0 \\ \" and \u{XXXX}.
func Unquote(lit string) (string, error) {
	if len(lit) < 2 || lit[0] != '"' || lit[len(lit)-1] != '"' {
		return "", fmt.Errorf("malformed string literal")
	}
	body := lit[1 : len(lit)-1]
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(body) {
			return "", fmt.Errorf("unterminated escape sequence")
		}
		switch body[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '0':
			b.WriteByte(0)
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		case 'u':
			end := strings.IndexByte(body[i:], '}')
			if i+1 >= len(body) || body[i+1] != '{' || end < 0 {
				return "", fmt.Errorf(`invalid unicode escape; expected \u{XXXX}`)
			}
			hex := body[i+2 : i+end]
			var r rune
			if len(hex) == 0 || len(hex) > 6 {
				return "", fmt.Errorf(`invalid unicode escape \u{%s}`, hex)
			}
			for _, h := range hex {
				if !isHex(h) {
					return "", fmt.Errorf(`invalid unicode escape \u{%s}`, hex)
				}
				r = r*16 + hexVal(h)
			}
			if !utf8.ValidRune(r) {
				return "", fmt.Errorf(`invalid unicode code point \u{%s}`, hex)
			}
			b.WriteRune(r)
			i += end
		default:
			return "", fmt.Errorf(`unknown escape sequence \%c`, body[i])
		}
	}
	return b.String(), nil
}

func isLetter(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= utf8.RuneSelf && unicode.IsLetter(r)
}

func isDigit(r rune) bool  { return r >= '0' && r <= '9' }
func isBinary(r rune) bool { return r == '0' || r == '1' }
func isOctal(r rune) bool  { return r >= '0' && r <= '7' }
func isHex(r rune) bool {
	return isDigit(r) || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'
}

func hexVal(r rune) rune {
	switch {
	case isDigit(r):
		return r - '0'
	case r >= 'a' && r <= 'f':
		return r - 'a' + 10
	default:
		return r - 'A' + 10
	}
}
