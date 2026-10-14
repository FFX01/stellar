# Stellar: project status

Last updated: 2026-10-15. Read this first when resuming work, then
`docs/design.md` (the language specification; §12 lists every decision and
open question).

## State

| Phase | Status |
|---|---|
| Lexer, parser, printer (`internal/lexer`, `parser`, `printer`) | Done |
| Type checker (`internal/types`) | Done for the whole design, including value semantics, `mut` markers, closures and `Ref[T]` |
| Tree-walking interpreter (`internal/interp`) | Done; runs `examples/library.st` |
| `Ref[T]` | Done (checker and interpreter) |
| Higher-order built-ins `map`, `filter`, `reduce`, `sortBy` | Not started (design §8.6, signatures provisional) |
| Modules and `import` | Not started (design §9) |
| Transpile to Go | Not started |

CLI: `stellar parse | check | run file.st`.

## Testing

```sh
go test ./...                                    # everything
go test ./internal/parser -update                # regenerate parser golden files
go test ./internal/interp -update                # regenerate run-output golden files
go test ./internal/types -run '^$' -fuzz FuzzCheck -fuzztime 30s
go test ./internal/parser -run '^$' -fuzz FuzzParseFile -fuzztime 30s
```

- Parser: `internal/parser/testdata/{ok,errors}/*.st` with `.golden` files.
- Checker: `internal/types/testdata/*.st`, expected errors written inline as
  `// ERROR "substring"`. A file without such comments must check cleanly.
- Interpreter: `internal/interp/testdata/*.st` and `examples/*.st`, each with
  a `.out` file holding stdout plus any panic or `error:` line.

## Key implementation choices

- Checker: bidirectional (`expr(x, want)`); literals take their type from
  `want` or from the other operand (`needsContext`, `operands`). Writable
  locations are "places" (`place.go`), used for assignments, `mut` arguments
  and marked receivers; overlap is checked per call.
- Interpreter: primitives are native Go types, so runtime types are
  recoverable. Value semantics by copy-on-write (`markShared` on every store,
  `unshare` before every write through an lvalue). `mut` arguments and
  `mut self` by copy-in/copy-out. A `Ref` is a `*RefVal` pointer.

## Working agreements

- Work in small steps; after each, run the tests, commit, and send a short
  update. Long silent stretches look like a hang.
- Discuss language-design questions before implementing them; record each
  decision in `docs/design.md` §12. Mark anything decided during
  implementation as provisional and raise it.
- Guiding rule for the language: data never changes unless the code that
  changes it is explicitly marked.

## Next steps

1. Confirm design §12 item 17 (calls through a `Ref` are still marked).
2. Higher-order built-ins: `map`, `filter`, `reduce`, `sortBy` (checker and
   interpreter); then consider the `Iterable` interface (§12 item 18).
3. Revisit the friction found while writing test programs: literals in
   `for` headers and conversions need a typed `let` first (`0..10`,
   `["a", "b"]`, `int32(3.99)`).
4. Add file names to panic traces.
5. Modules and `import`.
