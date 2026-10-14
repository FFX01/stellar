package lexer_test

import (
	"strings"
	"testing"

	"stellar/internal/lexer"
	"stellar/internal/token"
)

// render describes tokens compactly: operators and keywords by their text,
// literals as kind(text), newlines as ";".
func render(toks []token.Token) string {
	parts := make([]string, 0, len(toks))
	for _, t := range toks {
		switch {
		case t.Kind == token.NEWLINE:
			parts = append(parts, ";")
		case t.Kind == token.EOF:
			parts = append(parts, "EOF")
		case t.Kind == token.IDENT:
			parts = append(parts, "id("+t.Lit+")")
		case t.Kind == token.INT:
			parts = append(parts, "int("+t.Lit+")")
		case t.Kind == token.FLOAT:
			parts = append(parts, "float("+t.Lit+")")
		case t.Kind == token.STRING:
			parts = append(parts, "str("+t.Lit+")")
		case t.Kind == token.ILLEGAL:
			parts = append(parts, "illegal("+t.Lit+")")
		default:
			parts = append(parts, t.Kind.String())
		}
	}
	return strings.Join(parts, " ")
}

func TestTokens(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"empty", "", "EOF"},
		{"operators", "+ - * / % == != < <= > >= = -> => .. . , : | ?",
			"+ - * / % == != < <= > >= = -> => .. . , : | ? EOF"},
		{"delimiters", "( ) [ ] { }", "( ) [ ] { } ; EOF"},
		{"keywords", "pub fn type object interface enum impl let var mut",
			"pub fn type object interface enum impl let var mut EOF"},
		{"word operators", "a and b or not c is T as x",
			"id(a) and id(b) or not id(c) is id(T) as id(x) ; EOF"},
		{"types are identifiers", "int32 str Result map self Self",
			"id(int32) id(str) id(Result) id(map) id(self) id(Self) ; EOF"},
		{"unicode identifier", "café", "id(café) ; EOF"},

		{"integers", "0 42 1_000 0xFF 0b1010 0o17", "int(0) int(42) int(1_000) int(0xFF) int(0b1010) int(0o17) ; EOF"},
		{"floats", "1.5 0.25 1e10 2.5E-3", "float(1.5) float(0.25) float(1e10) float(2.5E-3) ; EOF"},
		{"range is not a float", "0..n", "int(0) .. id(n) ; EOF"},
		{"method on int", "5.x", "int(5) . id(x) ; EOF"},

		{"string", `"hello"`, `str("hello") ; EOF`},
		{"string escapes", `"a\n\t\"\\\u{1F600}"`, `str("a\n\t\"\\\u{1F600}") ; EOF`},

		{"newline after identifier", "a\nb", "id(a) ; id(b) ; EOF"},
		{"newline after literal", "1\n\"s\"\n2.0\n", `int(1) ; str("s") ; float(2.0) ; EOF`},
		{"newline after closers", ")\n]\n}\n", ") ; ] ; } ; EOF"},
		{"newline after return/break/continue", "return\nbreak\ncontinue\n", "return ; break ; continue ; EOF"},
		{"newline after bool", "true\nfalse\n", "true ; false ; EOF"},
		{"no newline after operator", "a +\nb", "id(a) + id(b) ; EOF"},
		{"no newline after comma", "f(a,\nb)", "id(f) ( id(a) , id(b) ) ; EOF"},
		{"no newline after open brace", "{\na\n}", "{ id(a) ; } ; EOF"},
		{"blank lines collapse", "a\n\n\n\nb", "id(a) ; id(b) ; EOF"},
		{"leading newlines ignored", "\n\n\na", "id(a) ; EOF"},

		{"line comment", "a // comment\nb", "id(a) ; id(b) ; EOF"},
		{"line comment at eof", "a // comment", "id(a) ; EOF"},
		{"block comment inline", "a /* c */ b", "id(a) id(b) ; EOF"},
		{"block comment spanning lines ends statement", "a /* c\n */ b", "id(a) ; id(b) ; EOF"},
		{"block comment after operator", "a + /* c\n */ b", "id(a) + id(b) ; EOF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks, errs := lexer.Tokenize([]byte(tt.src))
			if len(errs) > 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
			if got := render(toks); got != tt.want {
				t.Errorf("tokens:\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestPositions(t *testing.T) {
	toks, _ := lexer.Tokenize([]byte("let x\n  = é + 1"))
	want := []struct {
		kind      token.Kind
		line, col int
	}{
		{token.LET, 1, 1},
		{token.IDENT, 1, 5},
		{token.NEWLINE, 1, 6},
		{token.ASSIGN, 2, 3},
		{token.IDENT, 2, 5},
		{token.PLUS, 2, 7}, // columns count runes, not bytes
		{token.INT, 2, 9},
	}
	for i, w := range want {
		got := toks[i]
		if got.Kind != w.kind || got.Pos.Line != w.line || got.Pos.Col != w.col {
			t.Errorf("token %d: got %s at %s, want %s at %d:%d", i, got, got.Pos, w.kind, w.line, w.col)
		}
	}
}

func TestSpaceBefore(t *testing.T) {
	tests := []struct {
		src  string
		want []bool // SpaceBefore of each token, excluding NEWLINE and EOF
	}{
		{"Book{", []bool{true, false}},
		{"Book {", []bool{true, true}},
		{"Book\t{", []bool{true, true}},
		{"Book/* c */{", []bool{true, true}},
		{"a{\nb", []bool{true, false, true}},
		{"f(x)", []bool{true, false, false, false}},
	}
	for _, tt := range tests {
		toks, _ := lexer.Tokenize([]byte(tt.src))
		var got []bool
		for _, tok := range toks {
			if tok.Kind != token.EOF && tok.Kind != token.NEWLINE {
				got = append(got, tok.SpaceBefore)
			}
		}
		if len(got) != len(tt.want) {
			t.Errorf("%q: got %d tokens %v, want %d", tt.src, len(got), got, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%q: token %d (%s) SpaceBefore = %v, want %v", tt.src, i, toks[i], got[i], tt.want[i])
			}
		}
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"illegal character", "a $ b", `1:3: unexpected character '$'`},
		{"bang", "!a", `1:1: unexpected '!'; use 'not' for logical negation`},
		{"unterminated string", "\"abc\nx", "1:1: unterminated string literal"},
		{"unknown escape", `"\q"`, `1:1: unknown escape sequence \q`},
		{"bad unicode escape", `"\u{zz}"`, `1:1: invalid unicode escape \u{zz}`},
		{"unterminated block comment", "a /* b", "1:3: unterminated block comment"},
		{"empty hex", "0x", "1:1: hexadecimal literal has no digits"},
		{"empty exponent", "1e", "1:1: exponent has no digits"},
		{"trailing underscore", "1_", "1:1: '_' must separate successive digits"},
		{"letter after number", "12abc", `1:3: unexpected 'a' after number`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := lexer.Tokenize([]byte(tt.src))
			if len(errs) == 0 {
				t.Fatalf("expected error %q, got none", tt.want)
			}
			if got := errs[0].Error(); got != tt.want {
				t.Errorf("error:\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestUnquote(t *testing.T) {
	tests := []struct{ lit, want string }{
		{`""`, ""},
		{`"abc"`, "abc"},
		{`"a\nb"`, "a\nb"},
		{`"\t\r\0"`, "\t\r\x00"},
		{`"\"\\"`, `"\`},
		{`"\u{41}\u{1F600}"`, "A😀"},
		{`"héllo"`, "héllo"},
	}
	for _, tt := range tests {
		got, err := lexer.Unquote(tt.lit)
		if err != nil {
			t.Errorf("Unquote(%s): %v", tt.lit, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Unquote(%s) = %q, want %q", tt.lit, got, tt.want)
		}
	}
}
