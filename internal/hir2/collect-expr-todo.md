# AST nodes that `collectExpr` does not handle

This list compares `internal/ast/ast.go` and `parseStatement` in `internal/parser/parser.go`
with `collectExpr` in `internal/hir2/main.go`. The status is on 2026-11-25.
For each node in the list, `collectExpr` returns `InvalidExprID`.

## Statements in a block
`BlockExpr.Statements` holds `ast.Statement` values. `collectExpr` handles no statement type.

- `ast.ReturnStmt` (`return;`, `return e;`).
- `ast.EmitStmt` (`>> e;`).
- `ast.BreakStmt`.
- `ast.ContinueStmt`.

The parser cannot put `ast.FunctionStmt`, `ast.TypeDeclStmt`, `ast.TypeAliasStmt`,
`ast.ImplDeclStmt`, or `ast.ImportStmt` in a block. The parser does not make `ast.BadStmt`.

## Expressions
- `ast.CallExpr` (`f(a, b=c)`). Its arguments are `ast.Argument` values.
- `ast.MemberExpr` (`a.b`, `Action.Quit`).
- `ast.SubscriptExpr` (`xs[i]`).
- `ast.SliceExpr` (`xs[a:b]`, `xs[:b]`, `xs[a:]`).
- `ast.TupleAccessExpr` (`t.0`).
- `ast.SequenceLiteral` (`[1, 2]`, `[...xs, 3]`). `EKSliceLit` exists, but no case makes it.
- `ast.TupleLiteral` (`#(1, "a")`). `EKTupleLit` exists, but no case makes it.
- `ast.SpreadExpr` (`...xs`). It occurs in a sequence literal or in a call argument.
- `ast.SwitchExpr`, with its `ast.SwitchArm` values and the `as` bindings.
- `ast.ForInExpr` (`for i, x in xs { }`), with its two bindings.
- `ast.ForWhileExpr` (`for c { }`, and `for { }` with no condition).
- `ast.FunctionExpr` (`fn(x: int) -> int: x + 1`).
- `ast.TryExpr` (`try e`, `try e else v`, `try e else |err| { }`).
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
