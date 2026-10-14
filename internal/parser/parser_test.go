package parser_test

import (
	"testing"

	"stellar/internal/parser"
	"stellar/internal/printer"
)

func TestExprPrecedence(t *testing.T) {
	tests := []struct{ src, want string }{
		{"a + b * c", "(a + (b * c))"},
		{"a * b + c", "((a * b) + c)"},
		{"a - b - c", "((a - b) - c)"},
		{"a / b % c", "((a / b) % c)"},
		{"-a * b", "((-a) * b)"},
		{"- -a", "(-(-a))"},
		{"a + b == c * d", "((a + b) == (c * d))"},
		{"a < b and c > d", "((a < b) and (c > d))"},
		{"a or b and c", "(a or (b and c))"},
		{"a and b or c", "((a and b) or c)"},
		{"not a == b", "(not (a == b))"},
		{"not a and b", "((not a) and b)"},
		{"not not a", "(not (not a))"},
		{"0..n + 1", "(0..(n + 1))"},
		{"a..b == c", "((a..b) == c)"},
		{"x is Book", "(x is Book)"},
		{"x is Book as b", "(x is Book as b)"},
		{"x is Book and y", "((x is Book) and y)"},
		{"not x is Book", "(not (x is Book))"},
		{"(a + b) * c", "(((a + b)) * c)"}, // source parens are kept as ParenExpr

		{"try f()", "(try f())"},
		{"try f() + 1", "((try f()) + 1)"},
		{"try f() else 0", "(try f() else 0)"},
		{"try f() else a + b", "(try f() else (a + b))"},
		{"try f() else |e| g(e)", "(try f() else |e| g(e))"},
		{"try try f()", "(try (try f()))"},
		{"try xs[0]", "(try xs[0])"},

		{"a.b.c(d)[e]", "a.b.c(d)[e]"},
		{"Shape.Circle(r: 1.5)", "Shape.Circle(r: 1.5)"},
		{"f(a, b: c)", "f(a, b: c)"},
		{"Book{ title: t, isbn: i }", "Book{ title: t, isbn: i }"},
		{"lib.Book{}", "lib.Book{}"},
		{"[1, 2, 3]", "[1, 2, 3]"},
		{"[]", "[]"},
		{"[:]", "[:]"},
		{`["a": 1, "b": 2]`, `["a": 1, "b": 2]`},
		{`[k: v + 1]`, `[k: (v + 1)]`},
		{"[\n  \"a\": 1,\n  \"b\": 2,\n]", `["a": 1, "b": 2]`},
		{"f(\n  a,\n  b,\n)", "f(a, b)"},
		{"Book{\n  title: t,\n}", "Book{ title: t }"},
		{"a +\n  b", "(a + b)"},
		{"a and\n  b", "(a and b)"},
	}
	for _, tt := range tests {
		x, errs := parser.ParseExpr([]byte(tt.src))
		if len(errs) > 0 {
			t.Errorf("ParseExpr(%q): %v", tt.src, errs)
			continue
		}
		got := printer.ExprWith(x, printer.Options{Parenthesize: true})
		if got != tt.want {
			t.Errorf("ParseExpr(%q)\n got: %s\nwant: %s", tt.src, got, tt.want)
		}
	}
}

func TestExprErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		{"a < b < c", "1:7: comparisons cannot be chained; use 'and'"},
		{"a == b is T", "1:8: comparisons cannot be chained; use 'and'"},
		{"a..b..c", "1:5: ranges cannot be chained"},
		{"a +", "1:4: expected expression, found end of file"},
		{"f(a b)", "1:5: expected ), found identifier \"b\""},
		{"try f() else |e g(e)", "1:17: expected |, found identifier \"g\""},
		{"a b", "1:3: unexpected identifier \"b\" after expression"},
		{`[1, "a": 2]`, "1:8: unexpected : in array literal"},
		{`["a": 1, 2]`, "1:11: expected : after map key, found ]"},
		{"[:", "1:3: expected ], found end of file"},
	}
	for _, tt := range tests {
		_, errs := parser.ParseExpr([]byte(tt.src))
		if len(errs) == 0 {
			t.Errorf("ParseExpr(%q): expected error %q, got none", tt.src, tt.want)
			continue
		}
		if got := errs[0].Error(); got != tt.want {
			t.Errorf("ParseExpr(%q)\n got: %s\nwant: %s", tt.src, got, tt.want)
		}
	}
}
