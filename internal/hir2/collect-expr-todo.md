# AST nodes that `collectExpr` does not handle

This list compares `internal/ast/ast.go` and `parseStatement` in `internal/parser/parser.go`
with `collectExpr` in `internal/hir2/main.go`. The status is on 2026-11-25.
For each node in the list, `collectExpr` returns `InvalidExprID`.

## Expressions
- `ast.MustExpr` (`must e`).
- `ast.ResultExpr` (`Ok(x)`, `Err(e)`).
- `ast.MaybeExpr` (`Some(x)`, `None()`).
- `ast.CopyExpr` (`@x`). The language does not have this operator now, and the lexer rejects
  `@`. Thus no source text can make this node.

Parentheses (`(e)`) do not make a node. The parser returns the inner expression.

## Defect in a handled case
`ast.FloatLiteralExpr` stores `int(math.Float64frombits(uint64(t.Value)))`. The value is
incorrect: `uint64(1.5)` is `1`, and the result is `0`. To keep the bits of the value, use
`int(math.Float64bits(t.Value))`.
