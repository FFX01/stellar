# Analyzer implementation plan

Target: the three-pass analyzer from our discussion, built in small steps that each
end with something you can run. Package: `internal/analyzer`.

| Pass | What it does | File (already exist, empty) |
|---|---|---|
| 1. collect | create a symbol for every top-level declaration | `collection.go` |
| 2. resolve | fill in the types of those symbols | `evaluation.go` |
| 3. check | walk function bodies and expressions | `validation.go` |

Also split the data definitions out of `main.go` into `types.go`, `symbols.go`,
`scope.go`, and `universe.go`. Expression typing (`typeOf`) can live in `evaluation.go`
or its own `expr.go`.

## Step 0 — Decisions (answer before writing code)

This plan **assumes** the answers in bold. Tell me if any is wrong and I'll change
the plan.

1. **One symbol table** with a kind per symbol, instead of separate Types and Symbols
   tables. This contradicts the "separate namespace" note in `docs/design.md`
   (Lexing). If you agree, update that note.
2. **Shadowing is allowed** in inner scopes. A redeclaration in the *same* scope is an
   error.
3. **Top-level `var` without an annotation is allowed.** Its type is resolved on demand
   (Step 8), with cycle detection. Your `simple.solr` already relies on this.
4. **`Symbol` is an interface**, with one struct per kind (`VarSymbol`, `FuncSymbol`,
   `TypeSymbol`, `ModuleSymbol`) sharing an embedded `symbolBase`. Kind-specific fields
   (such as `Mutable`) then exist only where they apply.

## Step 1 — Types (`types.go`)

A `Type` describes what a value is. Every type is a Go **struct**. There are no
constants and no strings standing in for types.

```go
type Type interface{ String() string }
```

Builtin types are **package-level singleton pointers**. Because there is exactly one
`*BasicType` per builtin, comparing two basic types with `==` is correct.

```go
type BasicType struct{ Name string }
var (
    IntT   = &BasicType{"int"}
    FloatT = &BasicType{"float"}
    StrT   = &BasicType{"str"}
    BoolT  = &BasicType{"bool"}
)
```

Two special singletons:
- `Invalid`: the type of anything that failed to check. Absorbing: an operator, call
  or assignment with an `Invalid` operand returns `Invalid` **without** reporting an
  error. Never put it in a table where user code can name it.
- `Void`: the unit type, written `void`. It is the return type of a function with no
  `-> T`, and it is a real type (`void!` is a result with no payload). It is **not** a null
  that other types can hold; absence is `Maybe`. Register it in the universe scope under
  the name `void`.

Composite types, each a struct:

| Struct | Fields |
|---|---|
| `SliceType` | `Elem Type` |
| `ResultType` | `Value Type` (this is `T!`) |
| `MaybeType` | `Value Type` (this is `T?`, i.e. `Maybe[T]`; variants `Some(x)` and `None()`) |
| `FuncType` | `Params []Param`, `Return Type` (`Void` if none) |
| `ObjectType` | `Name`, `Fields []FieldInfo`, `Methods` (map plus order slice) |
| `EnumType` | `Name`, `Members []MemberInfo` |
| `InterfaceType` | `Name`, `Methods` (name to `*FuncType`) |
| `TypeParam` | `Name` (used when generics are added) |

with these helper structs:

```go
type Param struct {
    Name string
    Type Type
}
type FieldInfo struct {
    Name   string
    Type   Type
    Public bool
}
type MemberInfo struct { // one enum variant
    Name   string
    Fields []FieldInfo // payload fields, e.g. Shape.Rect(w=10, h=20). Empty if none.
}
type MethodInfo struct {
    Name   string
    Type   *FuncType // full signature, with `self` as Params[0]
    Public bool
    Node   *ast.FunctionStmt // for pass 3, which checks the body
}
```

`ObjectType.Methods` is `map[string]*MethodInfo` plus an `order []string` slice, for the
dump. When typing a method call such as `lexer.cur()`, skip `Params[0]` (the receiver).

- **Mutability is not part of a type.** Parameters are never mutable, so a `FuncType` has
  no `mut` anywhere, and `mut` appears only on a variable declaration
  (`VarSymbol.Mutable`, Step 2). `@x` has the same type as `x`.
- **Aliases have no struct.** `type A = B` just makes `A`'s symbol point at `B`'s type.
- Replace `typesMatch` with `identical(a, b Type) bool`: basics by `==`, slices and
  results and maybes recursively, functions by parameter types and return type (parameter
  names ignored), named types (object, enum, interface) by pointer identity. `void` is a
  singleton, so it compares with `==`.
- Delete `TypeTable`, `MissingT`, and the `Name()` and `typ()` methods. `String()` is
  all the interface needs.

**Check:** the package compiles. Nothing else changes yet.

## Step 2 — Symbols (`symbols.go`)

A `Symbol` is a name that something declared.

```go
type ResolveState int
const ( Unresolved ResolveState = iota; Resolving; Resolved )

// Fields every kind of symbol has.
type symbolBase struct {
    Name   string
    Type   Type          // nil until pass 2 fills it in
    Node   ast.Node      // declaring node, for positions. nil for builtins.
    State  ResolveState
    Public bool
}

// The interface exposes the shared fields through a pointer to the embedded struct,
// so shared code reads sym.base().State and needs no getter per field.
type Symbol interface{ base() *symbolBase }

func (b *symbolBase) base() *symbolBase { return b }

type VarSymbol struct {
    symbolBase
    Mutable bool // declared with `mut`. Parameters, `var` bindings and `for` bindings are
                 // false. See "Mutability" below.
}

// These kinds have no fields of their own yet. They still exist as separate types so a
// type switch can tell them apart.
type FuncSymbol struct{ symbolBase }
type TypeSymbol struct{ symbolBase }
type ModuleSymbol struct{ symbolBase }
```

There is no `SymbolKind` enum. Use a type switch (`switch s := sym.(type)`) wherever the
kind matters.

- `Node` is the **only** place the declaring node is stored. It lets an error say "first
  declared at 3:5", and pass 2 reads the declaration from it. What it holds, by kind:

  | Symbol | `Node` |
  |---|---|
  | `VarSymbol`, top-level or local | `*ast.VarStmt` |
  | `VarSymbol`, parameter | `*ast.Parameter` |
  | `VarSymbol`, `for` binding | the `*ast.ForInExpr` (its bindings are bare tokens) |
  | `FuncSymbol` | `*ast.FunctionStmt` |
  | `TypeSymbol` | `*ast.TypeDeclStmt` or `*ast.TypeAliasStmt` |
  | `ModuleSymbol` | `*ast.ImportStmt` |

  The symbol's own type tells you which node it is, so pass 2 asserts it once at the top
  of each case, for example `decl := s.Node.(*ast.FunctionStmt)`.
- Builtin symbols have a nil `Node`. Use a helper such as
  `func posOf(s Symbol) string` that returns `"builtin"` when `Node` is nil, in every
  message that mentions a declaration.
- `State` drives on-demand resolution and cycle detection (Step 8).
- For a `TypeSymbol`, `Type` is the type it names. For an object, enum or interface that
  is the named struct created in pass 1, whose body is empty until pass 2.

### Mutability

Rules from `docs/value-semantics.md`:
- `var x = ...` is an immutable binding. It can be **rebound** (`x = e`), but its contents
  can't be changed.
- `mut x = ...` can be rebound **and** changed in place (`x.f = v`, `x.xs.push(v)`,
  `x[i] = v`, `+=`).
- Parameters, `for` bindings and package-level `var`s are never `Mutable`.
- A closure can neither change nor rebind a variable of its parent function.

So there are two separate checks, and both read the root variable of the target. The root
of `a.b[i].c` is `a`.

| Operation | Allowed when |
|---|---|
| rebind: `x = e` | `x` is declared in the current function. Never across a function boundary. |
| change contents: `x.f = e`, `x[i] = e`, `x.push(e)`, `x += e` | `x` is `Mutable` **and** declared in the current function. |

Symbols do **not** store their scope. `Lookup` starts at the current scope and climbs
through the parents. Each time it leaves a `FunctionScope` **without having found the name
in it**, it has crossed a function boundary, and it returns that as a second value:

```
Lookup(name) (sym Symbol, crossedFunction bool)
```

Have one analyzer helper, `a.lookupVar(name) (v *VarSymbol, crossed bool)`, that calls
`Lookup` and returns the variable. The two checks become:

```
canRebind(v, crossed)  = !crossed
canMutate(v, crossed)  = v.Mutable && !crossed
```

Blocks (`if`, `for`, `switch` arms) are **not** boundaries. Function scopes are. So an `if`
or `for` body inside a function can rebind and change that function's variables.

A closure reads its parent's variables from a copy taken when the closure is created, so
reading a crossed variable is always fine. Only writing is an error.

Consequences:
- Calling `x.advance()`, where `advance` takes `self`, never mutates `x`. A method can't
  change its receiver, so there is nothing to check at the call.
- `mut r = @param;` is how a function gets a changeable copy of a parameter. `@x` is the
  copy operator (see Step 9, item 10).
- `mut x = y;` without `@` is an error. `var x = @y;` is allowed, if pointless.
- A top-level `var` is immutable inside every function, and can't be rebound from one.

Why no `Scope` field on the symbol: a stored scope can be set wrongly (a parameter
declared against the enclosing scope, for example), and it makes the symbol and the
scope point at each other. The walk uses the real scope structure, so there is nothing
to get wrong.

## Step 3 — Scope (`scope.go`)

```go
type ScopeKind int
const ( UniverseScope ScopeKind = iota; PackageScope; FunctionScope; BlockScope )

type Scope struct {
    Parent   *Scope
    Children []*Scope
    Kind     ScopeKind
    Name     string               // "lex", "if", ... for the dump
    symbols  map[string]Symbol
    order    []string             // declaration order, used by the dump
}
```

Methods:
- `Declare(sym Symbol) Symbol`: if the name already exists **in this scope** (don't walk
  parents), return the existing symbol and change nothing. Otherwise insert it, append
  to `order`, and return nil. The caller reports the error, because the scope has no
  node to attach it to.
- `LookupLocal(name)`: this scope only.
- `Lookup(name) (Symbol, bool)`: this scope, then parents. The bool is `crossedFunction`
  (see "Mutability is relative to a scope" in Step 2). Set it when moving from a scope
  to its parent, if the scope being left is a `FunctionScope` that did not contain the
  name. Most callers ignore it.
- `Symbols()`: the symbols in `order`.

**Check:** a small Go test: declare twice, confirm the second call returns the first
symbol and `order` has one entry.

## Step 4 — Analyzer struct and errors (`main.go`)

```go
type Analyzer struct {
    Universe, Package, Current *Scope
    Errors    []errors.Error
    ExprTypes map[ast.Expression]Type  // pass 3 records every expression's type
    Impls     []*ast.ImplDeclStmt      // filled in pass 1, used in pass 2
    Bodies    []pendingBody            // function bodies waiting for pass 3
    inPass3   bool
    curFn     *FuncType                // enclosing function, for return/try checks
    loopDepth int
}
type pendingBody struct{ fn *FuncType; params []*ast.Parameter; body ast.Expression; scope *Scope }
```

- Keep `addError(node, format, args...)`. After all passes, **sort `Errors` by
  position**.
- Remove `a.Types`, `addType`, and `lookupType`. Everything goes through `Scope`.

## Step 5 — Universe (`universe.go`)

This answers "what are the builtins". They are **symbols in the universe scope**,
the parent of the package scope, so every lookup reaches them.

1. For each of `int`, `float`, `str`, `bool`: declare
   `&TypeSymbol{symbolBase: symbolBase{Name: "int", Type: IntT, State: Resolved}}`.
2. **`int(x)`, `float(x)`, `str(x)` are not function symbols.** `int` is the type
   symbol from step 1. When `typeOf` sees a call whose callee resolves to a `*TypeSymbol`,
   the call is a conversion (basic type) or a construction (object type). That is how
   `Token(kind=...)` and `int(x)` both work, and it means there is nothing extra to
   register. What each conversion accepts is TBD in the spec, so start by accepting
   any basic type.
3. **`Result` isn't registered.** `T!` is parsed into `ast.ResultType`, and `Ok(...)`
   and `Err(...)` are parsed into `ast.ResultExpr`, so none of them is a name lookup.
   Add a `Result[T]` symbol only when generic types exist.
4. **Other builtin functions** (`len`, `print`, …) are a later addition. When you add
   one, it's a `*FuncSymbol` in the universe scope with a normal `FuncType`.
5. **Builtin methods** such as `str.get`, `contains`, and slice `push` need a separate
   member table keyed by receiver type. Not needed until Step 14.

**Check:** `NewAnalyzer()` creates Universe, then Package as its child, and
`Lookup("int")` from the package scope finds the symbol.

## Step 6 — Pass 1: collect (`collection.go`)

`collect(prog *ast.Program)` loops over `prog.Statements` once. It looks at
declaration headers only, never bodies. For each statement:

| Statement | Action |
|---|---|
| `TypeDeclStmt` | create the empty named type from `Definition`: `&ObjectType{Name}`, `&EnumType{Name}` or `&InterfaceType{Name}`. Symbol: `TypeSymbol`, that type, `Unresolved`. |
| `TypeAliasStmt` | `TypeSymbol`, `Type: nil`, `Unresolved` |
| `FunctionStmt` | `FuncSymbol`, `Type: nil` |
| `VarStmt` | `VarSymbol`, `Type: nil`, `Mutable: false`. Top-level `mut` is an error (mutation is local to a function). |
| `ImportStmt` | `ModuleSymbol` named after the last path element |
| `ImplDeclStmt` | no symbol. Append to `a.Impls`. |
| `BadStmt` | skip |

Always set `Node` to the statement. If `Declare` returns an
existing symbol, report `X redeclared (first declared at L:C)` using
`existing.base().Node.Pos()` (via `posOf`), and discard the new
symbol.

**Check:** a file with two `var x` declarations and a `type x` shows one redeclaration
error each, and the dump lists each name once.

## Step 7 — Dump and tests (do this before pass 2)

Build the tooling first so every later step is checkable.

1. Replace `PrintTree` with `Dump(w io.Writer)`. Per scope print `kind name`, then one
   line per symbol in `order`: `name  kind  type`. Print `String()`, not `Name()`, so
   `Result[int]` stays intact.
2. `Dump` of errors separately, as `file:line:col: message`, to stderr.
3. Golden tests: `internal/analyzer/ref/<name>.solr` with a sibling `<name>.expected`,
   and one Go test that runs every pair and diffs the output.
4. In `cmd/analyze`: if the parser reported errors, print them and stop. Otherwise run
   the analyzer, print errors, and `os.Exit(1)` if there are any.

## Step 8 — Pass 2: resolve (`evaluation.go`)

### 8a. `resolveTypeExpr(t ast.Type) Type` (replaces `visitType`)
| AST node | Result |
|---|---|
| `NamedType` | `Lookup(name)`. It must exist and be a `*TypeSymbol`, otherwise report an error and return `Invalid`. Return `sym.Type`. For an alias, call `resolveSymbol` first. |
| `SliceType` | `&SliceType{Elem: resolveTypeExpr(...)}` |
| `ResultType` | `&ResultType{Value: ...}` |
| `MaybeType` | `&MaybeType{Value: ...}` (the parser has to produce this for `T?`) |
| `FunctionType` | build a `FuncType` |
| `MutableType` | error: "parameters cannot be `mut`". Remove this node from the parser (see "Parser impact" in `docs/value-semantics.md` §7). |
| `nil` | **not an error.** Callers decide what "no annotation" means. |

`Args` on a `NamedType` (generics): report "not supported yet" for now.

### 8b. `resolveSymbol(sym Symbol)` — the state machine
```
b := sym.base()
if b.State == Resolved: return
if b.State == Resolving: report cycle on b.Node; set b.Type = Invalid; return
b.State = Resolving
  switch s := sym.(type) { ...resolve by kind (below)... }
b.State = Resolved
```
This one function gives cycle detection for aliases and for top-level vars.

Each case of the type switch reads its declaration from `Node` with one assertion at the
top: `*VarSymbol` uses `*ast.VarStmt`, `*FuncSymbol` uses `*ast.FunctionStmt`,
`*TypeSymbol` uses `*ast.TypeDeclStmt` or `*ast.TypeAliasStmt` (switch on which), and
`*ModuleSymbol` uses `*ast.ImportStmt`.

### 8c. Type declarations (run first)
Loop over the package symbols that are `*TypeSymbol`:
- **Object:** for each `ast.Field`, resolve its type and append a `FieldInfo`. Duplicate
  field name is an error.
- **Enum:** for each `EnumMember`, append a `MemberInfo` with its fields. Duplicate
  member name is an error.
- **Interface:** build a `FuncType` for each method signature.
- **Alias:** `sym.Type = resolveTypeExpr(target)`.
- **Type parameters:** `[T]` is not supported yet. When added, enter a child scope with
  `T` declared as a `TypeSymbol` holding `TypeParam`, resolve the body inside it, then exit.

### 8d. `buildFuncType(sig *ast.FunctionSignature) *FuncType`
- For each parameter: a missing type is an error ("parameter x needs a type").
- A missing return type becomes `Void`.
- `self` is an ordinary first parameter, as in `fn cur(self: Lexer)`. It is never `mut`,
  so a method can't change its receiver. Nothing special for the receiver here.

### 8e. `impl` blocks
Loop over `a.Impls`:
1. Resolve `Target`. It must be an object or enum type declared in this package.
2. For each method: `buildFuncType`, then add it to `target.Methods`. A duplicate
   method name, or one that equals a field name, is an error.
3. Append a `pendingBody` for pass 3.

This comes after 8c because it needs the types to exist.

### 8f. Function symbols
Loop over `*FuncSymbol` symbols: `sym.Type = buildFuncType(...)`, append a `pendingBody`.

### 8g. Top-level vars
Loop over `*VarSymbol` symbols and call `resolveSymbol`. For a var:
1. `declared := resolveTypeExpr(node.Type)` (nil when there is no annotation).
2. `inferred := typeOf(node.Value)`.
3. If either is `Invalid`, skip the comparison silently. If `declared != nil` and they
   don't `identical`, report a mismatch on `node.Value`.
4. `sym.Type = declared` if there is one, otherwise `inferred`.

`typeOf` for an identifier that is an `Unresolved` package symbol calls `resolveSymbol`
on it first. That is how `var a = b` works when `b` is declared further down.

### 8h. Function expressions inside a top-level `var`
`typeOf(FunctionExpr)` builds the `FuncType` from the signature only. Because the body
can't be checked yet (pass 3 hasn't started), queue it as a `pendingBody` whose scope is
the package scope.

**Check:** `simple.solr` should show exactly three errors, on lines 4, 5 and 6 (the
bugs from the last run are gone), with one `var` per line in the dump.

## Step 9 — `typeOf` (expression typing)

`typeOf(e ast.Expression) Type`. Records `a.ExprTypes[e] = result` for every node.
Build it in this order, running a golden file after each group:

1. **Literals:** `IntegerLiteralExpr`, `FloatLiteralExpr`, `StringLiteralExpr`,
   `BoolLiteralExpr` return the singleton.
2. **Identifier:** `Lookup` (use `lookupVar` when it is a variable). Not found is an error, result `Invalid`. Found: resolve if
   needed and return `sym.Type`. A `*TypeSymbol` here is only valid as a call callee or
   `Kind.X` receiver. Otherwise report "type used as a value".
3. **Prefix:** `-` on `int` or `float`, `!` on `bool`.
4. **Infix**, as a rule table (data, not an `if` chain). Operand types must be
   identical (`docs/design.md` §7 and §8):

   | Operators | Operands | Result |
   |---|---|---|
   | `+ - * /` | `int, int` or `float, float` | same type |
   | `%` | `int, int` | `int` |
   | `+` | `str, str` | `str` |
   | `== !=` | identical types | `bool` |
   | `< <= > >=` | `int`, `float` or `str` pairs | `bool` |
   | `and or` | `bool, bool` | `bool` |

   If either side is `Invalid`, return `Invalid` with no error. Otherwise report on the
   `InfixExpr` node (its `Pos()` is the left operand).
5. **`Ok` and `Err` (`ResultExpr`):** keep your current handling. `Err` can't know its
   payload type from the expression alone, so give it a way to be compared against an
   expected type.
6. **Call:** type the callee, then branch on what it is:
   - a `*TypeSymbol` for a basic type: conversion. Result is that type.
   - a `*TypeSymbol` for an object: construction. Arguments must be named (#129), match the
     field names and types, and follow the omission rule: a field of primitive or
     container type may be omitted and takes its zero value, a `Maybe` field may be
     omitted and is `None()`, any other field must be given. Result is the object type.
   - a function type: check arity, then each argument against its parameter. Result is
     `Return`, even if an argument was wrong. There is no call-site marker: `f(@x)` is
     not a call form, so an `@` argument is an error (copy first, then pass the copy).
   - anything else: "not callable".
7. **Member (`a.b`):** the receiver's type decides. An object field gives the field
   type. A method gives its `FuncType`. An enum receiver (the type itself, e.g.
   `Kind.Eof`) gives the enum type. Builtin members wait for Step 14.
8. **Subscript, slice, sequence literal.** A sequence literal needs an expected type to
   type `[]`. Pass the annotation down from `var x: []T = []`.
9. **`try` / `must`:** operand must be a `ResultType`. Result is `.Value`. `try` also
   requires the enclosing function to return a `ResultType`, so it can return the `Err`.
10. **`@x` (`GrantExpr`), the copy operator:** the operand's type. It is only valid as
    the initializer of a `var` or `mut` binding (`mut t = @todo;`). Anywhere else it is an
    error. `mut x = y;` (no `@`) is an error, and `var x = @y;` is allowed. Rename the AST
    node to `CopyExpr` when convenient.
11. **`FunctionExpr`:** build the `FuncType` from its signature. Queue the body in pass 2,
    or check it immediately in pass 3.
12. **`if`, `switch`, block, `for`:** see Step 10.

## Step 10 — Pass 3: check bodies (`validation.go`)

Loop `for i := 0; i < len(a.Bodies); i++` (the slice can grow while you iterate). For
each body:

1. Set `a.inPass3 = true`. Enter a `FunctionScope` as a child of the body's recorded
   scope. Declare each parameter as a `VarSymbol` with `Mutable: false`, `Resolved`, with its resolved type.
2. Set `a.curFn` to the function's `FuncType`.
3. Check the statements **in source order**. **Bind locals as you reach them**, never
   up front, so use-before-declaration is naturally an "undefined" error.
   - `VarStmt`, including `mut`: type the initializer **first**, then declare. For a
     `mut` binding the initializer must be an `@` copy, a literal, or a call result (a
     fresh value), never a bare identifier or field. That stops `var x = x + 1`
     from seeing itself. Compare with the annotation if there is one. Always declare the
     symbol, with `Invalid` if the initializer was bad. A same-scope redeclaration is an
     error.
   - `AssignStmt`: the target must be a variable, field, or subscript. The value type
     must be `identical` to the target type. Compound operators use the operator table.
     Find the root variable of the target with `lookupVar`. If the target is the variable
     itself, it's a rebind, which needs `canRebind`. If the target is a field or
     subscript of it, it changes contents, which needs `canMutate`. Report the specific
     reason: "cannot assign to a field of immutable `x` (declare it with `mut`)", or
     "cannot assign to `x` from inside a closure".
   - `ReturnStmt`: compare with `curFn.Return`. A bare `return` needs `Void`.
   - `ExpressionStatement`: `typeOf` it.
   - `Break` / `Continue`: error if `a.loopDepth == 0`.
   - `EmitStmt` (`>>`): see below.
4. Child scopes. For `if`, `for`, `switch` arms and nested blocks: `EnterScope`, check
   the statements, `ExitScope`. Conditions must be `bool`. A `for` increments
   `loopDepth`. `ForInExpr` declares its two binding variables in the child scope from
   the container's type.
5. A `FunctionExpr` met while `inPass3` is true: check its body **right now**, in a child
   scope of the current one. Don't queue it, because the enclosing scope keeps changing
   as the walk continues.
6. `>>` (emit) and block values: a block's type is the type of its `>>` operand.
   Decide whether `if`, `switch` and bare blocks as expressions require every branch to
   have the same type. This is not in the spec yet. Report it as TBD and skip it first.

Afterwards: sort errors by position.

**Check:** write `ref/bodies.solr` with return mismatches, a bad `if` condition,
`break` outside a loop, an undefined variable, and a shadowed variable. Confirm each.

## Step 11 — Types in the dump
Add types to the scope dump, with object fields, enum members and methods on indented
lines, so the output shows a type's contents. Golden-test `ref/types.solr` with an enum,
an object, an alias, and an `impl`.

## Step 12 — Acceptance test
Run the analyzer on `reference_programs/lexer/main.solr`.
- **`reference_programs/lexer/main.solr` is written in the old `mut`-parameter model and
  no longer applies.** Use a lexer or todo program rewritten for the current model.
  `reference_programs/lexer/immut.solr` is a candidate once its syntax errors are fixed.
- It should exercise `impl`, `var` rebinding, `mut` with `@`, `.with`, `try … else`, `for`,
  and `switch`. Add negative cases: field assignment on a `var`, mutating from a closure,
  and `mut x = y` without `@`.
- Builtin members (`str.get`, `contains`, slice `push`, `l.input[start:l.pos]`) will
  show as errors until Step 14. That's expected.

## Step 13 — Mutation rules (after the above works)
Most of the mutability checks are already part of Steps 9 and 10 (`lookupVar`, `canRebind`,
`canMutate`, the `@` rules). What remains for this step:
- in-place builtin methods (`push`, `sort`, ...) require `canMutate` on the receiver's root
  variable, and non-mutating ones (`append`, `with`) don't;
- a `for` loop over a `mut` variable iterates a copy, so changing the variable in the
  body doesn't affect the iteration;
- a closure that rebinds or changes a parent variable is an error (this already falls out
  of `canRebind` and `canMutate`).

## Step 14 — Builtin members
Add a table keyed by receiver type. Examples from the lexer program: `str.get(int) ->
str!`, `[]T.contains(T) -> bool`, `[]T.push(T)`. `typeOf(MemberExpr)` consults it when
the receiver is a builtin type.

## Suggested checkpoints
| After step | Run |
|---|---|
| 1–5 | `go build ./...`, and the old `simple.solr` output still prints |
| 6–7 | a duplicate-declaration file, and the golden test passes |
| 8 | `simple.solr`: exactly three errors, lines 4–6 |
| 9 (1–4) | arithmetic and comparison cases, both valid and invalid |
| 10 | `ref/bodies.solr` |
| 11 | `ref/types.solr` |
| 12 | `reference_programs/lexer/main.solr` |
