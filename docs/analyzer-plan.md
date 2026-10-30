# Analyzer implementation plan

A step-by-step guide to building the Stellar semantic analyzer (type checker) in
`internal/analyzer`. It replaces the earlier plan.

- **Part 1** explains the ideas the analyzer is built on. Read it once before starting.
- **Part 2** is the build, in order. Every step ends with something you can run.
- **Part 3** holds the rule tables that Part 2 refers to.
- **Part 4** lists what is left out of the first version, and the open decisions.

The language rules come from `docs/language-spec.md` (cited as "spec §N"). When this plan and
the spec disagree, the spec wins and this plan has a bug.

---

# Part 1 — Concepts

## 1.1 What the analyzer does

The parser checks *shape*: that `var x: int = "a";` is a well-formed declaration. The analyzer
checks *meaning*: that `"a"` is not an `int`. It answers three questions for every name and
expression:

1. **What does this name refer to?** (name resolution: symbols and scopes)
2. **What type does this expression have?** (type checking)
3. **Is this operation allowed here?** (rules: mutability, `try` placement, `break` outside a loop, …)

Its output is a list of errors, plus a table of the type of every expression (`ExprTypes`),
which the compiler will use later.

## 1.2 Types vs type expressions

Two different things are both called "type":

- An **AST type** (`ast.NamedType`, `ast.SliceType`, …) is what the user *wrote*:
  the text `[]Todo`.
- A **checker type** (`*SliceType{ElemType: <Todo>}`) is what that text *means*.

Pass 2's `resolveTypeExpr` turns the first into the second. After that, the checker only works
with checker types.

## 1.3 Symbols

A **symbol** is the record the checker keeps for a declared name: its kind (variable, function,
type, module), its type, and the AST node that declared it. `var x = 1;` creates one
`*VarSymbol` named `x` with type `int`.

## 1.4 Scopes

A **scope** maps names to symbols, and has a parent. `Lookup(name)` looks in the scope, then its
parent, and so on up to the top. That one rule gives you shadowing and nested visibility.

```
Universe        int float str bool void Error len         (builtins; never changes)
└─ Package      every top-level declaration of the file   (filled by pass 1)
   └─ fn main   parameters and locals of main             (created in pass 3)
      └─ block  locals of an `if`/`for`/`switch` arm body (created in pass 3)
```

- **Universe and Package are separate scopes** so that builtins are shared by every package
  once modules exist, `a.Universe.LookupLocal(name)` answers "is this a builtin?", and
  `a.Package.Symbols()` lists exactly the user's top-level declarations.
- **`CurrentScope` is a cursor**, not a level. It points at the scope the checker is in.
  `enterScope` moves it down, `exitScope` moves it back up.
- **Function scopes are boundaries.** `Lookup` also reports whether it walked out of a
  function scope to find a name (`crossedFunc`). That is how the checker knows a closure is
  touching its parent's variable, or a function its top-level `var` (Part 3, mutability).

## 1.5 Why three passes

Top-level declarations can be used before they are declared (spec §4.1):

```solr
fn main() { var p = origin(); }    // `origin` is declared below
fn origin() -> Point { ... }
type Point struct { x: int, y: int }
```

A single top-to-bottom walk can't check `main`, because `origin` and `Point` aren't known yet.
So the work is split:

| Pass | Job | Scopes touched |
|---|---|---|
| 1. collect | Create a symbol for every top-level name. No types yet. | Package only |
| 2. resolve | Give every top-level symbol its type: struct fields, function signatures, methods, top-level `var`s. | Package only |
| 3. check | Walk every function body: statements, expressions, local variables. | Creates all function and block scopes |

Passes 1 and 2 never look inside a function body. That is deliberate: **local** names are
*not* usable before their declaration (spec §4.4), so local scopes are filled in pass 3, in
source order, as the walk reaches each declaration. A use before the declaration then simply
fails to find the name, and the "undefined" error falls out for free.

## 1.6 `InvalidT`: report each error once

When something is wrong, its type is `InvalidT`. Every rule treats `InvalidT` as "already
reported": an operator, call, comparison or assignment involving `InvalidT` produces `InvalidT`
(or passes) **without** a new error. Without this, one typo produces a cascade of ten errors.

## 1.7 Expected types

Most expressions know their own type: `1 + 2` is `int`. A few can't: `[]`, `None()`, `Err(e)`
and `Ok()` don't say what `T` is. For those, the caller passes down the type it *expects*:

```go
func (a *Analyzer) typeOf(e ast.Expression, want Type) Type
```

`want` is a hint, and may be nil. Only the expressions that can't infer their own type use it.
Everything else ignores it, and the **caller** still compares the result with what it expects.
Callers that have an expectation pass it: a `var` annotation, a function's return type for
`return`, a parameter type for an argument, a field type for a constructor argument.

## 1.8 How the code is organised

- Every pass and helper is a **method on `*Analyzer`**, because they all need the scopes, the
  error list, and the current-function state.
- Walking the AST uses a Go **type switch** (`switch n := e.(type)`), not the visitor pattern.
  Pass 3 must control the order (declare after the initializer, enter a scope before a body),
  and the passes return different things, which a single `Visitor` interface handles badly.
- The trade-off is that a type switch doesn't tell you when you forgot a node. So every
  switch over AST nodes has a **loud `default`**:
  `a.addError(n, "not implemented: %T", n); return InvalidT`. Part 3.1 lists every node
  and the step that handles it.

## 1.9 Files

| File | Contents |
|---|---|
| `types.go` | checker types, `identical`, `assignable` |
| `symbols.go` | symbol kinds |
| `scope.go` | `Scope` |
| `universe.go` | builtin types, `Error`, `len` |
| `main.go` | `Analyzer` struct, `NewAnalyzer`, `Run`, errors, scope helpers |
| `dump.go` | `Dump` (scopes, symbols, types) for golden tests |
| `collection.go` | pass 1 |
| `resolution.go` | pass 2 (`resolveTypeExpr`, `resolveSymbol`, signatures, `impl`) |
| `expressions.go` | `typeOf` and its helpers |
| `validation.go` | pass 3 (bodies, statements, control flow, mutability) |
| `analyzer_test.go` | the golden test runner |
| `ref/*.solr` + `ref/*.expected` | golden test inputs and outputs |

---

# Part 2 — Build steps

Each step has a **Goal**, what to **Build**, and a **Check**. Don't start a step until the
previous check passes.

## Step 1 — Types (`types.go`)

**Goal:** the Go structs that represent what a type *is*.

**Build:**

```go
type Type interface {
    typ()
    String() string
}
```

Singletons (one pointer per builtin, so `==` compares them):

| Name | Meaning |
|---|---|
| `IntT`, `FloatT`, `StrT`, `BoolT` | `*PrimitiveType` |
| `VoidT` | `*PrimitiveType{"void"}`: the unit type, a real type (spec §3.1) |
| `InvalidT` | `*PrimitiveType{"invalid"}`: "already reported" (§1.6) |
| `NeverT` | `*PrimitiveType{"never"}`: the type of a block that always leaves with `return`, `break` or `continue`. Assignable to every type (Step 13). |
| `SelfT` | `*PrimitiveType{"self"}`: the type of the untyped `self` in an interface method signature. The language has no `Self` type (spec §3.7), so users never write it. It only marks the receiver slot. |

Composite types:

| Struct | Fields | `String()` |
|---|---|---|
| `SliceType` | `ElemType Type` | `[]int` |
| `ResultType` | `ValueType Type` (`VoidT` for `void!`, never nil) | `int!` |
| `MaybeType` | `ValueType Type` | `int?` |
| `FunctionType` | `Params []Param`, `ReturnType Type` (`VoidT` if none) | `fn(int, str) -> bool` |
| `StructType` | `Name`, `Fields []FieldInfo`, `Methods MethodSet` | its name |
| `EnumType` | `Name`, `Members []MemberInfo`, `Methods MethodSet` | its name |
| `InterfaceType` | `Name`, `MethodOrder []string`, `Methods map[string]*FunctionType` | its name |
| `BuiltinFuncType` | `Name string` | `builtin len` |

`BuiltinFuncType` is the type of builtin functions whose signature can't be written as a
`FunctionType`, such as `len`, which takes either a slice or a `str` (Step 16).

Helper structs:

```go
type Param struct { Name string; Type Type }
type FieldInfo struct { Name string; Type Type; IsPublic bool }
type MemberInfo struct { Name string; Fields []FieldInfo }   // one enum variant
type MethodInfo struct {
    Name     string
    Type     *FunctionType      // full signature, `self` is Params[0]
    IsPublic bool
    Node     *ast.FunctionStmt
}
type MethodSet struct {
    order  []string
    byName map[string]*MethodInfo
}
```

Structs and enums both get methods from `impl`, so both embed a `MethodSet` instead of
duplicating the map and the order slice. Write three methods on it:

| Method | Behavior |
|---|---|
| `func (m *MethodSet) Add(info *MethodInfo) bool` | If `byName` is nil, create it first. If `info.Name` is already there, change nothing and return false (the caller reports the duplicate, Step 8). Otherwise store it, append the name to `order`, return true. |
| `func (m *MethodSet) Get(name string) *MethodInfo` | The method, or nil. Reading a nil map is fine in Go, so no nil check is needed. |
| `func (m *MethodSet) All() []*MethodInfo` | The methods in `order`, for the dump. |

`Add` has to create the map itself because pass 1 creates `&StructType{Name: "Point"}` with a
zero `MethodSet`, whose map is nil, and writing to a nil map panics. The receivers are
pointers: `Add` changes the set, and `t.Methods.Add(...)` on a `*StructType` passes
`&t.Methods` automatically. Use `Get` and `All` everywhere outside these three methods (your
`dump.go` reads `order` and `byName` directly).

Used by: Step 8 (`impl` methods are added), Step 11 (`p.add(q)` looks a method up), Step 15
(interface satisfaction), and the dump.

`identical(a, b Type) bool` implements spec §3.10:
- primitives, `NeverT`, `SelfT`: pointer equality;
- slices, results, maybes: same kind and `identical` element types;
- functions: same number of parameters, `identical` parameter types (names ignored), and
  `identical` return types;
- structs, enums, interfaces, builtin functions: pointer equality (the same declaration).

Mutability is not part of any type. `@x` has the type of `x`.

**Check:** `go build ./internal/analyzer/` compiles. Write a tiny Go test: `identical` of two
separately built `&SliceType{IntT}` is true, and of `[]int` and `[]str` is false.
`String()` of `&FunctionType{Params: []Param{{"a", IntT}}, ReturnType: &ResultType{StrT}}` is
`fn(int) -> str!`.

## Step 2 — Symbols (`symbols.go`)

**Goal:** the record for each declared name.

```go
type ResolveState int
const ( Unresolved ResolveState = iota; Resolving; Resolved )

type symbolBase struct {
    Name     string
    Node     ast.Node      // the declaring node. nil for builtins.
    Type     Type          // nil until resolved
    State    ResolveState
    IsPublic bool
}
func (b *symbolBase) base() *symbolBase { return b }

type Symbol interface{ base() *symbolBase }

type VarSymbol    struct{ symbolBase; IsMut bool }
type FuncSymbol   struct{ symbolBase }
type TypeSymbol   struct{ symbolBase }
type ModuleSymbol struct{ symbolBase }
```

- Use a type switch (`switch s := sym.(type)`) wherever the kind matters. There is no kind
  enum.
- Shared code reads fields through `sym.base()`, for example `sym.base().Name`.
- What `Node` holds:

  | Symbol | `Node` |
  |---|---|
  | top-level or local `VarSymbol` | `*ast.VarStmt` |
  | parameter `VarSymbol` | `*ast.Parameter` |
  | `for` binding, `try … else \|err\|` binding | the `*ast.ForInExpr` / `*ast.TryExpr` |
  | `FuncSymbol` | `*ast.FunctionStmt` |
  | `TypeSymbol` | `*ast.TypeDeclStmt` or `*ast.TypeAliasStmt` |
  | `ModuleSymbol` | `*ast.ImportStmt` |

- Symbols don't store their scope. The scope structure itself is the source of truth.

**Check:** compiles.

## Step 3 — Scopes (`scope.go`)

**Goal:** name → symbol maps with parents.

```go
type ScopeKind int
const ( UniverseScope ScopeKind = iota; PackageScope; FunctionScope; BlockScope )

type Scope struct {
    Parent   *Scope
    Children []*Scope        // for the dump
    Kind     ScopeKind
    Name     string          // "main", "if", "for", ... for the dump
    symbols  map[string]Symbol
    order    []string        // declaration order, so the dump is deterministic
}
```

| Method | Behavior |
|---|---|
| `NewScope(parent *Scope, kind ScopeKind, name string) *Scope` | Creates the map, and appends the new scope to `parent.Children` when there is a parent. **Always create scopes through this.** A `Scope{}` literal has a nil map, and `Declare` on it panics. |
| `Declare(sym) Symbol` | If the name exists **in this scope** (don't walk parents), return the existing symbol and change nothing. Otherwise insert it, append to `order`, return nil. It doesn't report errors: the caller does (Step 4). |
| `LookupLocal(name) Symbol` | This scope only. |
| `Lookup(name) (Symbol, bool)` | This scope, then the parents. The bool is `crossedFunc`: true if the walk **left a `FunctionScope`** on the way to the symbol. |
| `Symbols() []Symbol` | In declaration order. |

`crossedFunc` has to survive the whole walk. Leaving a function scope sets it, and it stays set
however many more scopes the walk passes through. So the recursive case is "my own crossing,
**or** whatever the parent reported", never just one of them.

**Check:** Go unit tests (`scope_test.go`):
1. Declaring `x` twice returns the first symbol the second time, and `Symbols()` has one entry.
2. Universe → Package → Function(`f`) → Block(`if`) → Function(closure) → Block(`if`).
   Declare `x` in `f`'s scope. From the innermost block, `Lookup("x")` finds it with
   `crossedFunc == true`. From the first block, it finds it with `crossedFunc == false`.
3. From the `f` scope, `Lookup` of a package symbol returns `crossedFunc == true`.

## Step 4 — The analyzer core (`main.go`, `universe.go`)

**Goal:** an analyzer that builds its scopes and runs (empty) passes.

```go
type Analyzer struct {
    Universe, Package, CurrentScope *Scope
    Errors    []errors.Error
    ExprTypes map[ast.Expression]Type
    Impls     []*ast.ImplDeclStmt   // pass 1 → pass 2
    Bodies    []*pendingBody        // pass 2 → pass 3
    inPass3   bool
    // Steps 12 and 13 add the per-function state: curFunc, loopDepth, blocks.
}
type pendingBody struct {
    funcType *FunctionType
    params   []*ast.Parameter
    body     ast.Expression     // *ast.BlockExpr, or the expression of `fn f(): e;`
    scope    *Scope             // the scope the function is declared in
    name     string             // for the dump
}
```

Helpers:

| Helper | Behavior |
|---|---|
| `addError(n ast.Node, format string, args ...any)` | Appends `errors.Error{Position: n.Pos(), Message: ...}`. Keep messages plain, e.g. `` undefined: `x` ``. |
| `enterScope(kind, name) *Scope` / `exitScope()` | `CurrentScope = NewScope(CurrentScope, kind, name)` and back to `CurrentScope.Parent`. |
| `declare(sym Symbol) bool` | The only way user code adds a symbol: (1) if `a.Universe.LookupLocal(name)` exists, report `` `int` is a builtin and can't be redeclared ``; (2) if `CurrentScope.Declare(sym)` returns an existing symbol, report `` `x` redeclared (first declared at 3:5) `` using the existing symbol's `Node.Pos()`; return false in both cases, true otherwise. Skip `_`: it is never declared. |

`universe.go`, `newUniverse() *Scope` creates the **universe** scope and declares these in it
(with `Declare` directly, not `declare`):
- `TypeSymbol`s `int`, `float`, `str`, `bool`, `void`, all `Resolved`, with the singleton type;
- `TypeSymbol` `Error`, whose type is `ErrorT`: a Go global in `universe.go` (like `IntT` in
  `types.go`), `var ErrorT = &InterfaceType{Name: "Error", ...}`. The checker refers to
  `ErrorT` directly for the `Err(e)` payload, the `|err|` binding, and `Error("msg")`. Its one
  method is `fn error(self) -> str`: a `FunctionType` whose `Params[0]` is
  `{"self", SelfT}` and whose `ReturnType` is `StrT` (spec §8.1);
- `TypeSymbol`s `Result` and `Maybe`, `State: Resolved`, `Type: nil` (Step 7a explains why);
- `FuncSymbol` `len` with type `&BuiltinFuncType{Name: "len"}` (spec §10.1).

Builtins are never shadowed: `declare` rejects every user declaration with a builtin's name,
in any scope (spec §2.2). Fields and enum variants are not scope symbols, so a field may still
be called `str`.

**The three passes start as empty methods**, one per file, so `Run` can call them now and
never changes again. Later steps fill in the bodies:

| Method | File | Filled in by |
|---|---|---|
| `func (a *Analyzer) collect(prog *ast.Program)` | `collection.go` | Step 6 |
| `func (a *Analyzer) resolve()` | `resolution.go` | Steps 7, 8, 10 (one part each) |
| `func (a *Analyzer) checkBodies()` | `validation.go` | Step 12 |

(`collection.go` currently has `collect` as a plain function. Make it a method: it needs
`a.declare` and `a.Impls`.)

`Run(prog *ast.Program) *Analyzer` creates the analyzer (`Universe`, then `Package` as its child,
`CurrentScope = Package`), calls `a.collect(prog)`, `a.resolve()`, `a.checkBodies()`, then
**sorts `Errors` by line, then column**.

Delete the old code in `main.go`: `BuiltInType`, `typesMatch`, `visitType`, `visitVarStmt`,
`lookupSymbol`, `analyze`, the old `Scope.Symbols.get` calls.

**Check:** `go build ./...` passes (`validation.go` needs its `package analyzer` line).
`Run` on an empty program returns no errors, and `a.Package.Lookup("int")` finds the builtin
while `a.Package.LookupLocal("int")` doesn't.

## Step 5 — Dump and golden tests (`dump.go`, `analyzer_test.go`)

**Goal:** a way to see what the analyzer built, and to compare it automatically from now on.

A **golden test** runs the analyzer on an input file and compares the output with a saved
"expected" file. You review the expected file once by eye. After that the test catches every
change.

**`Dump(w io.Writer)`** prints every scope except the universe, depth-first through `Children`,
one symbol per line in declaration order:

```
package
  var greeting: str
  fn main: fn() -> void
  type Point: struct
    field x: int
    method add: fn(Point, Point) -> Point
  type Dir: enum
    variant North
    variant Add(title: str)
  module fmt
  fn main
    var p: Point
    block if
      var q: int
```

Print a symbol's type with `String()`, or `<unresolved>` while it is nil. Then print the errors,
one per line: `3:5: message`.

**`analyzer_test.go`:** for every `ref/*.solr`: lex, parse (fail the test if there are lexer or
parser errors: a golden file must be valid syntax), `Run`, `Dump` to a buffer, and compare with
`ref/<name>.expected`. Add a `-update` flag that writes the `.expected` files instead of
comparing (`go test ./internal/analyzer -update`). Run it, read the new files, commit them.

**`cmd/analyze`:** print lexer and parser errors and exit if there are any. Otherwise run the
analyzer, print the dump and errors, and exit 1 if there are errors.

**Check:** `ref/empty.solr` (an empty file) produces `package` and nothing else.

## Step 6 — Pass 1: collect (`collection.go`)

**Goal:** a symbol for every top-level name, in the package scope.

`func (a *Analyzer) collect(prog *ast.Program)` loops over `prog.Statements` once, looks only at
declaration headers, and calls `a.declare` (with `CurrentScope == a.Package`):

| Statement | Symbol (`Node` = the statement, `State: Unresolved` unless noted) |
|---|---|
| `TypeDeclStmt` | `TypeSymbol`. Its `Type` is created now, **empty**: `&StructType{Name}`, `&EnumType{Name}`, or `&InterfaceType{Name}`, chosen by `Definition`. |
| `TypeAliasStmt` | `TypeSymbol`, `Type: nil` (an alias's type is only known after resolving its target) |
| `FunctionStmt` | `FuncSymbol` |
| `VarStmt` | `VarSymbol`, `IsMut: false`. If `IsMut` is set on the statement, report "top-level `mut` is not allowed" (the parser rejects it today, so this is defensive). |
| `ImportStmt` | `ModuleSymbol` named after the last path element, `State: Resolved`, `Type: nil` |
| `ImplDeclStmt` | no symbol. Append to `a.Impls`. |

Why create struct/enum/interface types now, empty? Because their **identity** is all another
declaration needs. `type List struct { next: Node? }` and `type Node struct { list: List }`
refer to each other. With the pointers created in pass 1, pass 2 can fill in both bodies in
any order.

**Check:** `ref/collect.solr`: two `var x`, a `type x struct {}`, a `fn int() {}`, an `import
std::fmt;`. Expect one symbol per name, two "redeclared" errors pointing at the first `x`, and
one builtin error for `int`.

## Step 7 — Pass 2a: type declarations (`resolution.go`)

**Goal:** turn written types into checker types, and fill in the bodies of every `type`
declaration: struct fields, enum variants, interface methods, alias targets.

### Where things stand after pass 1

| Declaration | `Type` | `State` |
|---|---|---|
| builtins (`int`, `Error`, …) | the singleton | `Resolved` |
| `type P struct {...}` | `&StructType{Name: "P"}`, no fields yet | `Unresolved` |
| `type E enum {...}` | `&EnumType{Name: "E"}`, no members yet | `Unresolved` |
| `type I interface {...}` | `&InterfaceType{Name: "I"}`, no methods yet | `Unresolved` |
| `type A = ...;` | `nil` | `Unresolved` |

For a struct, enum or interface, `Type` is the type's **identity** (the pointer every
reference shares) and `State` says whether its **body** has been filled in. This step fills
the bodies in place, in the pointer that already exists. Never replace `Type` with a new
struct: every reference made so far points at the old one.

### 7a. `Result` and `Maybe` in the universe

`Result[int]` and `Maybe[str]` are written as ordinary named types, so `resolveTypeExpr` has to
recognise them. Don't match on the name: a lookup that finds the builtin is more robust, and
it makes `type Result struct {}` a "builtin can't be redeclared" error like every other
builtin. Add two symbols to `newUniverse` (Step 4):

```go
ResultSym = &TypeSymbol{symbolBase{Name: "Result", State: Resolved}}  // Type stays nil
MaybeSym  = &TypeSymbol{symbolBase{Name: "Maybe",  State: Resolved}}
```

They have no `Type` of their own, because `Result` alone isn't a type: `Result[int]` is.
`resolveTypeExpr` recognises them by pointer (`sym == ResultSym`).

### 7b. `resolveTypeExpr(t ast.Type) Type`

A **method** on `*Analyzer` (it needs the scopes and the error list). `resolution.go`
currently declares it as a plain function with no return statement, which doesn't compile.

It looks names up from `a.CurrentScope`, so the same function works for top-level types in
pass 2 and for local annotations (`var x: []Todo = []`) in pass 3.

| AST | Result |
|---|---|
| `nil` | `nil`. Not an error: "no annotation". The caller decides what that means. |
| `NamedType` | the procedure below |
| `SliceType` | `&SliceType{ElemType: a.resolveTypeExpr(n.ElementType)}` |
| `ResultType` (`T!`) | `&ResultType{ValueType: a.resolveTypeExpr(n.Value)}` |
| `MaybeType` (`T?`) | `&MaybeType{ValueType: a.resolveTypeExpr(n.Value)}` |
| `FunctionType` | `&FunctionType{...}`: each parameter's type resolved (names are ignored), the return type resolved, or `VoidT` if there is none. |
| `StructType`, `EnumType`, `InterfaceType` | only valid as the definition of a `type` declaration, which doesn't go through here. Anywhere else: "a struct type must be declared with `type`", `InvalidT`. |
| anything else | the loud default: "not implemented: %T", `InvalidT` |

**`NamedType`**, in this order:
1. `len(n.Path) > 0` (`fmt.Thing`): "qualified types are not supported yet", `InvalidT`.
   Modules don't export anything yet.
2. `sym, _ := a.CurrentScope.Lookup(n.Name)`. Not found: "undefined type `X`", `InvalidT`.
3. Not a `*TypeSymbol` (a variable, function or module): "`x` is not a type", `InvalidT`.
4. `sym == ResultSym` or `sym == MaybeSym`: return `&ResultType{...}` / `&MaybeType{...}` of
   the resolved argument. More than one argument: "`Result` takes one type argument",
   `InvalidT`. No argument: `Result` means `Result[void]` (spec §8.1), so
   `&ResultType{ValueType: VoidT}`. `Maybe` without one: "`Maybe` needs one type argument:
   `Maybe[int]`", `InvalidT`.
5. `len(n.Args) > 0`: "generic types are not supported yet", `InvalidT`.
6. `sym.Type != nil` (a builtin, or a struct, enum or interface from pass 1): return it.
   **Don't call `resolveSymbol`**: a reference only needs the identity (7e).
7. Otherwise it is an alias whose target hasn't been resolved yet:
   `return a.resolveSymbol(sym)`.

A composite with a bad part stays a composite: `[]Undefined` is a `SliceType` of `InvalidT`.
The `InvalidT` inside still suppresses later errors.

### 7c. `buildFunctionType(sig *ast.FunctionSignature, self Type) *FunctionType`

Interfaces need this here, and functions and methods use it in Step 8.

`self` is the receiver type: the `impl` target for a method, `SelfT` for an interface method,
and nil for an ordinary function or a function literal. `self` is written without a type
(spec §4.3), so this is where it gets one.

- **A parameter named `self`:**
  - first parameter, `self != nil`, no type written: its type is `self`.
  - a type written (`self: Point`): "`self` is written without a type". Use `self` anyway, so
    the body is still checked.
  - not the first parameter, or `self == nil`: "`self` is only allowed as the first parameter
    of a method", type `InvalidT`.
- **Any other parameter:** `resolveTypeExpr` of its type. A missing type is an error,
  "parameter `x` needs a type" (spec §4.2). The parser accepts the omission, so this is the
  only place it's caught. Use `InvalidT` as the type.
- The return type: `resolveTypeExpr`, or `VoidT` if there is none.
- `TypeParams` not empty: "generic functions are not supported yet", once, on the signature.
  Use the same trick as 7d to keep the body's references to `T` quiet.

### 7d. `resolveSymbol(sym Symbol) Type`: on-demand resolution

`resolveSymbol` fills in a symbol's type the first time something needs it, and detects
cycles with the `State` field. In this step it handles type symbols only:

```go
func (a *Analyzer) resolveSymbol(sym Symbol) Type {
	b := sym.base()
	switch b.State {
	case Resolved:
		return b.Type
	case Resolving:
		// We reached this symbol again while resolving it: a cycle.
		a.addError(b.Node, "`%s` refers to itself", b.Name)
		return InvalidT
	}

	b.State = Resolving
	switch s := sym.(type) {
	case *TypeSymbol:
		a.resolveTypeDecl(s)
	default:
		a.addError(b.Node, "not implemented: resolving %T", sym)
	}
	b.State = Resolved

	// Safety net: a symbol never leaves here with a nil type.
	if b.Type == nil {
		b.Type = InvalidT
	}
	return b.Type
}
```

Step 10 adds a `case *VarSymbol:` for top-level variables. Functions never come through here:
Step 8 builds their type directly. Modules are `Resolved` from pass 1.

Builtins have a nil `Node`, which `addError` can't handle. They're `Resolved` from the start,
so they always return at the top and never reach a line that reports an error.

`resolveTypeDecl` reads the declaration from the symbol's `Node`. A type symbol's `Node` is
one of two statements (Step 2), so it switches on which, and binds `node` to the concrete
statement:

```go
func (a *Analyzer) resolveTypeDecl(s *TypeSymbol) {
	switch node := s.Node.(type) {
	case *ast.TypeAliasStmt:
		// node.Value is the target after `=`: in `type Tags = []str;` it is the `[]str`
		// ast.Type. The alias's type is the resolved target.
		s.Type = a.resolveTypeExpr(node.Value)
	case *ast.TypeDeclStmt:
		a.resolveTypeBody(s, node)
	default:
		a.addError(s.Node, "not implemented: type declaration %T", s.Node)
	}
}
```

- **`*ast.TypeAliasStmt`:** an alias is just another name for its target, so its `Type` is
  the resolved target. An alias with type parameters (`type Pair[A] = ...`) gets the same
  "generic types are not supported yet" treatment as below.
- **`*ast.TypeDeclStmt`:** `resolveTypeBody(s, node)` handles the type parameters first,
  then fills in the body according to `node.Definition`.

```go
func (a *Analyzer) resolveTypeBody(s *TypeSymbol, node *ast.TypeDeclStmt) {
	if len(node.TypeParams) > 0 {
		// see "Type parameters" below: report once, open a scope, declare them
		defer a.exitScope()
	}
	switch def := node.Definition.(type) {
	case *ast.StructType:
		t := s.Type.(*StructType) // the empty struct created in pass 1
		// fill t.Fields from def.Fields
	case *ast.EnumType:
		t := s.Type.(*EnumType)
		// fill t.Members from def.Members
	case *ast.InterfaceType:
		t := s.Type.(*InterfaceType)
		// fill t.Methods and t.MethodOrder from def.Methods
	}
}
```

Each case fills the type **that pass 1 created**, through the type assertion on `s.Type`. It
never assigns a new value to `s.Type`. The rules for each part:

- **Type parameters.** If `node.TypeParams` is not empty, report "generic types are not
  supported yet" once, on the declaration. Then `a.enterScope(BlockScope, s.Name)` and declare
  each parameter as a `TypeSymbol` with `Type: InvalidT`, `State: Resolved`, so `first: A` in
  the body resolves to `InvalidT` silently instead of "undefined type `A`". Resolve the body,
  then `a.exitScope()`. `TypeParameter` only has a `Token`: its name is `Token.Literal`.
- **Struct:** for each `*ast.Field` in `def.Fields`, append
  `FieldInfo{Name, a.resolveTypeExpr(f.Type), f.IsPublic}` to `t.Fields`. Track the names in a
  local `map[string]bool`. A repeat: "duplicate field `x` in `Point`" on the field, and skip it.
- **Enum:** for each `*ast.EnumMember` in `def.Members`, append a
  `MemberInfo` with its fields, built like struct fields. Duplicate variant name: "duplicate
  variant `X` in `Dir`". Duplicate field inside one variant: "duplicate field `x` in `Dir.X`".
  (`pub` on a variant field is parsed, but means nothing yet. Ignore it.)
- **Interface:** make the map (`t.Methods = map[string]*FunctionType{}`). For each
  `*ast.FunctionSignature` in `def.Methods`:
  `buildFunctionType(sig, SelfT)`. A signature without `self` as its first parameter:
  "interface method `area` needs `self`". Duplicate name: "duplicate method `area` in
  `Shape`". Add the rest to the map and to `MethodOrder`.

A field, variant or method of a bad type is still added, with `InvalidT` where needed. That
way a later `p.x` finds the field and doesn't report "no field `x`" on top of the first error.

### 7e. Why references to structs skip `resolveSymbol`

Take two structs that refer to each other:

```solr
type List struct { head: Node? }
type Node struct { next: Node?, owner: List }
```

The loop in 7f resolves `List`. It is `Resolving` while its field `head: Node?` is resolved.
`resolveTypeExpr(Node)` finds `Node`'s symbol with a `Type` already set, and returns the
pointer (rule 6). `Node`'s body isn't needed to *refer* to `Node`, so `List` finishes. Then
the loop resolves `Node`, whose field `owner: List` returns `List`'s pointer the same way.

If rule 6 called `resolveSymbol(Node)` instead, resolving `Node` would reach `owner: List`,
and `List` is still `Resolving`: a false "`List` refers to itself".

Only aliases go through `resolveSymbol` from a reference, because an alias has no identity
until its target is known. That is where real cycles live:

```solr
type A = B;
type B = []A;        // A → B → []A → A: reported once, "`A` refers to itself"
```

The error is reported on the symbol that is reached a second time, and both aliases end up as
(slices of) `InvalidT`, so nothing that uses them reports again.

### 7f. `a.resolve()`, part 1

```go
for _, sym := range a.Package.Symbols() {
    if ts, ok := sym.(*TypeSymbol); ok {
        a.resolveSymbol(ts)
    }
}
```

Run it with `a.CurrentScope == a.Package`. Aliases may already be `Resolved` by the time the
loop reaches them (a struct field used them first). `resolveSymbol` returns immediately in
that case, which is why it checks `State` first.

### Check

Two golden files.

`ref/types.solr` (no errors):

```solr
type Point struct { pub x: int, pub y: int }
type Shape enum { Circle(r: float), Rect(w: float, h: float), Empty }
type Named interface { fn name(self) -> str }
type List struct { head: Node? }
type Node struct { next: Node?, owner: List, tags: Tags }
type Tags = []str;
type Handler = fn(Point) -> bool!;
type Box struct { value: Result[int], maybe: Maybe[str] }
```

The dump shows every field, variant and method with its type. `tags` is `[]str` (resolved
through the alias, declared after its use), `value` is `int!`, `maybe` is `str?`, and
`name` is `fn(self) -> str`. No errors.

`ref/type_errors.solr` (the expected errors are in the comments, in this order):

```solr
type A = B;                                  // `A` refers to itself
type B = []A;
type C struct { x: Missing }                 // undefined type `Missing`
type D struct { x: int, x: str }             // duplicate field `x` in `D`
type E enum { V, V }                         // duplicate variant `V` in `E`
type F interface { fn f() -> int }           // interface method `f` needs `self`
type G[T] struct { v: T }                    // generic types are not supported yet (only once)
type H struct { r: Maybe }                   // `Maybe` needs one type argument
type int struct {}                           // `int` is a builtin and can't be redeclared (pass 1)
type J struct { f: fmt.Thing }               // qualified types are not supported yet
```

The dump still lists `C`, `D` and the rest, with `InvalidT` where a type was bad, and `D` with
one `x`.

## Step 8 — Pass 2b: functions and methods

**Goal:** every function and method has a `FunctionType`, and every body is queued for pass 3.

Signatures are built with `buildFunctionType` (Step 7c).

**`a.resolve()`, part 2: functions.** For each `*FuncSymbol` in the package:
`Type = buildFunctionType(sig, nil)`, `State = Resolved`, and append a `pendingBody` with
`scope: a.Package`.

A `FunctionStmt` without a body (`fn f();`, which the parser accepts, see Part 4.3) has a nil
`Body`: report "function `f` has no body" and don't queue it.

**`a.resolve()`, part 3: `impl` blocks.** For each `ImplDeclStmt` in `a.Impls`:
1. `resolveTypeExpr(Target)`. It must be a `*StructType` or `*EnumType`; otherwise report
   "can only implement methods on a struct or enum". (`impl Pair[A, B]` gets the generics error
   from `resolveTypeExpr`.)
2. For each method: `buildFunctionType(sig, target)`. A method whose first parameter isn't
   `self` would be a static function, which is **[provisional]** (spec §4.3): "a method
   needs `self` as its first parameter".
3. Add it to the target's `MethodSet`. A duplicate method name, or a method with the same
   name as a field, is an error.
4. Queue a `pendingBody` with `scope: a.Package`.

**Check:** `ref/functions.solr`: functions with and without return types, an untyped
parameter, a method on a struct, a duplicate method, a method named like a field, an `impl` on
`int`, `self: Point` (error), a method without `self` (error), `self` in a plain function
(error), `self` as a second parameter (error). Check the signatures in the dump (a method's
first parameter shows the target type) and one error per problem.

## Step 9 — Expressions, part 1 (`expressions.go`)

**Goal:** the type of literals, names and operators. This is enough to check top-level `var`s.

```go
func (a *Analyzer) typeOf(e ast.Expression, want Type) Type {
    t := a.typeOfInner(e, want)
    a.ExprTypes[e] = t
    return t
}
```

`typeOfInner` is the big type switch. In this step:

1. **Literals:** `IntegerLiteralExpr` → `IntT`, `FloatLiteralExpr` → `FloatT`,
   `StringLiteralExpr` → `StrT`, `BoolLiteralExpr` → `BoolT`.
2. **`IdentifierExpr`:**
   - `_` → "`_` can't be used as a value", `InvalidT`.
   - `Lookup`. Not found → `` undefined: `x` ``, `InvalidT`.
   - `*VarSymbol`, `*FuncSymbol` → `a.resolveSymbol(sym)` (an unresolved top-level `var` is
     resolved right now, which is how `var a = b;` works when `b` is declared further down).
   - `*TypeSymbol` → "`Point` is a type, not a value", `InvalidT`. (A type *is* allowed as the
     callee of a call or the left side of `.`; those cases look at the identifier **before**
     calling `typeOf` on it, see Step 11.)
   - `*ModuleSymbol` → "`fmt` is a module, not a value", `InvalidT`.
3. **`NegativeExpr`:** operand `int` or `float` → the same type. **`NotExpr`:** operand
   `bool` → `bool`. Otherwise an error. `InvalidT` operand → `InvalidT`, no error.
4. **`InfixExpr`:** look the operator and operand types up in the operator table (Part 3.2).
   Implement the table as data (a slice or map of rules), not as an `if` chain. If either side
   is `InvalidT`, return `InvalidT` with no error. No matching rule → "invalid operation:
   `int` + `float`", `InvalidT`.

Also write, in `types.go`:

```go
func assignable(from, to Type) bool  // Part 3.3
```

and in the analyzer:

```go
// reports "expected `int`, got `str`" unless assignable or either side is InvalidT
func (a *Analyzer) expectType(n ast.Node, got, want Type)
```

Start `assignable` as `identical` plus the `InvalidT` and `NeverT` rules. Step 15 adds
interfaces.

**Check:** none yet. Step 10 exercises this.

## Step 10 — Pass 2c: top-level variables

**Goal:** every top-level `var` has a type. This is the first point where the existing
`ref/simple.solr` gives meaningful output.

**`a.resolve()`, part 4:** for each `*VarSymbol` in the package, call `resolveSymbol`.

Add a case for variables to the type switch in `resolveSymbol` (Step 7d), next to
`*TypeSymbol`:

```go
	case *VarSymbol:
		a.resolveTopLevelVar(s)
```

and write it:

```go
func (a *Analyzer) resolveTopLevelVar(s *VarSymbol) {
	node := s.Node.(*ast.VarStmt)

	// nil when there is no annotation
	declared := a.resolveTypeExpr(node.Type)

	var got Type
	if cp, ok := node.Value.(*ast.CopyExpr); ok {
		// `var t = @x;` is allowed. The full `@` rules come in Step 14.
		got = a.typeOf(cp.Value, declared)
	} else {
		got = a.typeOf(node.Value, declared)
	}

	if declared != nil {
		a.expectType(node.Value, got, declared)
		s.Type = declared
	} else {
		s.Type = got
	}
}
```

It runs while the symbol is `Resolving`. So if the initializer reads the variable itself,
directly or through another variable, `typeOf` reaches `resolveSymbol` on it again and the cycle
is reported.

Order inside `resolve()`: types (part 1), functions (part 2), impls (part 3), vars (part 4). A
`var` initializer may call a function or a method, so their signatures must exist first. A
`var` that reads another `var` resolves it on demand. `var a = b; var b = a;` reports one
"refers to itself" error.

**Function literals at the top level** (`var add = fn(x: int) -> int: x + 1;`): `typeOf`
builds the type with `buildFunctionType(sig, nil)`. Because `a.inPass3` is false, it **queues** the body
as a `pendingBody` with `scope: a.Package`. Inside pass 3 a function literal is checked
immediately instead (Step 12).

**Check:** `ref/simple.solr`. Expect exactly one error per wrong line, a type for every
`var`, and no "invalid" types on lines that are fine. Add `ref/vars.solr` with a forward
reference, a cycle, a mismatched annotation, and `1 + 1.0`.

## Step 11 — Expressions, part 2: calls, members, indexing

**Goal:** everything an expression can do with a value.

### Calls (`CallExpr`)

Look at the callee **before** typing it, because a type name is a valid callee:

1. **The callee is an identifier or member naming a type** (`Point(...)`, `int(...)`,
   `Error(...)`, `Action.Add(...)`). Write a helper `a.typeSymbolOf(e) *TypeSymbol` that
   returns the symbol when `e` is an `IdentifierExpr` resolving to a `*TypeSymbol`.
   - primitive `int`, `float`, `str`: **conversion**. Exactly one positional argument of a
     primitive type → the target type (spec §9.3). `bool(x)` and `void(x)`: "no conversion to
     `bool`".
   - `ErrorT`: `Error("message")`. One positional `str` argument → `ErrorT`.
   - a struct: **construction**, Part 3.4.
   - an interface, an alias of a non-struct, or `Result`/`Maybe` (whose symbols have no
     `Type`; their values are made with `Ok`/`Err`/`Some`/`None`): "cannot construct `X`".
   - `Enum.Variant(...)`: the callee is a `MemberExpr` whose container is an enum type
     symbol. Variant construction, Part 3.4, over the variant's fields. A variant with no
     fields is written without parentheses: `Action.Quit()` → "variant `Quit` has no fields;
     write `Action.Quit`".
2. **The callee is `x.with`** where `x` is a struct or enum value: `.with(...)` (spec §10.3).
   Check it like a construction of `x`'s type with a spread of `x` first (Part 3.4).
3. **Otherwise** `ct := typeOf(callee, nil)`, then switch on `ct`:
   - `*FunctionType`: arguments must be positional (named arguments on functions are
     **[open]**: "named arguments are only for construction"). A spread argument is an error
     here. Arity must match. Each argument: `typeOf(arg, param.Type)`, then `expectType`.
     The result is `ReturnType`, **even if an argument was wrong**, so the error doesn't
     spread to the code around the call.
   - `*BuiltinFuncType`: per-builtin rule (Step 16).
   - `InvalidT`: type the arguments anyway (they may contain their own errors), return
     `InvalidT`, no new error.
   - anything else: "`x` is not callable".

A `CopyExpr` argument (`f(@x)`) is an error: "`@` is only allowed as a variable
initializer". There is no call-site copy marker.

### Member access (`MemberExpr`)

Again look at the container first:
- **container names an enum type** (`Action.Quit`): the member must be a variant →
  the enum type. A variant **with** fields, used without a call, is an error ("construct it
  with `Action.Add(...)`"), except as a `switch` case (Step 13 handles cases before calling
  `typeOf`).
- **container names a struct type** (`Point.origin`): static functions are **[provisional]**:
  "static functions are not supported yet".
- **container names a module** (`fmt.println`): the module's contents are **[open]** (spec
  §10.4). Return `InvalidT` **without** an error, so calls into `std` don't flood the output.
  Remove this when modules are real.
- **otherwise** `ct := typeOf(container, nil)`:
  - `*StructType`: a field → its type. A method → a **bound method**: its `FunctionType`
    without `Params[0]`, so the call rule above works unchanged for `p.add(q)`.
  - `*EnumType`: a method → bound method. Fields of a variant are **not** readable here
    (narrowing is provisional, Part 4.1): "`action` has no field `title`".
  - `*InterfaceType`: a method → bound method (its `FunctionType` without `Params[0]`).
  - `*SliceType`, `StrT`: builtin methods (Step 16). Until then, "no method `push`".
  - `InvalidT`: `InvalidT`, no error.
  - not found: "`Point` has no field or method `z`".

### Indexing (`SubscriptExpr`)

- Receiver `[]T`: an `int` index → `T`. Receiver `str`: an `int` index → `str` (spec §6.7).
- The argument is a `SliceExpr` (`xs[a:b]`): both bounds optional, each `int`. The result has
  the receiver's type.
- Anything else: "cannot index `Point`".
- A `SliceExpr` reached by `typeOf` directly (not as a subscript argument) is an error: "`:` is
  only allowed inside `[ ]`". The parser accepts `a:b` anywhere for now (Part 4.3).

### Sequence literals (`SequenceLiteral`)

- `want` is a `*SliceType`: each element is checked against its `ElemType`. The result is
  `want`.
- No `want`: `[]` → "cannot infer the type of `[]`; add a type annotation", `InvalidT`.
  Otherwise the first element's type decides, and every other element must be identical.
- A `SpreadExpr` element (`[...xs, y]`) must be a slice of the element type.

### `SpreadExpr` and `CopyExpr`

A `SpreadExpr` is only valid as a sequence element or a construction argument. Reached by
`typeOf` anywhere else: "`...` is only allowed in a list or a constructor". A `CopyExpr`
reached by `typeOf`: "`@` is only allowed as a variable initializer" (the `var`/`mut` rules in
Step 14 look at it before calling `typeOf`).

**Check:** `ref/calls.solr` (top-level `var`s are enough): construction with a missing field,
an omitted `int` field (fine), a positional construction argument, a wrong argument type, wrong
arity, a method call, a conversion, `Error("x")`, an enum variant with and without fields,
indexing a slice and a `str`, slicing, `[]` with and without an annotation, a call into `fmt`
(no error).

## Step 12 — Pass 3: function bodies (`validation.go`)

**Goal:** check every statement in every function body.

### `checkBodies()`

Set `a.inPass3 = true`. Loop `for i := 0; i < len(a.Bodies); i++` (index loop, because the
slice may still grow). For each body, call `checkFunctionBody(pb)`.

### Function state

Add these fields to `Analyzer`. They describe the function being checked right now:

```go
    curFunc   *FunctionType  // its signature, for `return` and `try`
    loopDepth int            // loops around the current statement, for `break`/`continue`
```

Step 13 adds a third, `blocks`. Until then, leave it out of the save/restore below.

### `checkFunctionBody(pb *pendingBody)`

1. **Save** `CurrentScope`, `curFunc`, `loopDepth` (and `blocks`, from Step 13). Set `CurrentScope = pb.scope`,
   then `enterScope(FunctionScope, pb.name)`.
2. Declare every parameter as a `*VarSymbol` (`IsMut: false`, `Resolved`, type from
   `funcType.Params`).
3. `curFunc = pb.funcType`, `loopDepth = 0` (and `blocks = nil`, from Step 13).
4. **Block body:** check its statements **directly in the function scope** (don't open
   another scope, so `fn f(x: int) { var x = 1; }` is a redeclaration). If the return type is
   not `VoidT` and the statements don't terminate (Part 3.6): "missing return".
   **Expression body** (`fn f() -> int: e;`): `typeOf(e, ReturnType)`, then `expectType`.
5. `exitScope()`, and **restore** everything saved in step 1.

Saving and restoring is what lets a closure be checked in the middle of its parent (below).

### Statements: `checkStmt(s ast.Statement)`

| Statement | Rule |
|---|---|
| `VarStmt` | Type the initializer **first**, with the annotation as `want`, then `declare` the symbol. That order makes `var x = x + 1;` see the outer `x` (spec §4.4). If the annotation and initializer disagree, report and use the annotation. Always declare it, with `InvalidT` if the initializer was bad, so later uses don't report "undefined". `mut` rules: Step 14. |
| `AssignStmt` | `typeOf` the target and the value (with the target's type as `want`), then `expectType`. The operator is `n.Token.Type` (`ASSIGN`, `PLUS_ASSIGN`, …): `AssignStmt` has no operator field. A compound operator looks up `target op value` in the operator table and requires the result to be assignable to the target. Who may assign: Step 14. |
| `ReturnStmt` | With a value: `typeOf(v, curFunc.ReturnType)` + `expectType`. Without one: the return type must be `VoidT`. |
| `BreakStmt`, `ContinueStmt` | Error if `loopDepth == 0`. |
| `EmitStmt` (`>>`) | Step 13. |
| `ExpressionStatement` | Part 3.5: only certain expressions may be statements. Then type it. `if`, `switch`, `for` and blocks as statements: Step 13. |

### Function literals inside a body

`typeOf(FunctionExpr)` with `a.inPass3 == true`: build its type with `buildFunctionType(sig, nil)`, and
**check its body right away** by calling `checkFunctionBody` with
`scope: a.CurrentScope`. It must happen now, not queued: the closure can see exactly the
locals declared before it, and `CurrentScope` changes as soon as the walk moves on. The
function scope it creates is what makes `Lookup` report `crossedFunc` for its parent's
variables.

**Check:** `ref/bodies.solr`: a return type mismatch, a bare `return` in a function returning
`int`, a missing return, `break` outside a loop, an undefined variable, use before
declaration, `var x = x + 1;` with an outer `x`, a redeclared local, a shadowing local in an
inner block (fine), a parameter redeclared in the body, a builtin name used as a local, a
closure reading a parent variable (fine).

## Step 13 — Blocks, `if`, `switch`, `for`, `>>`

**Goal:** control flow, both as statements and as values.

### Value blocks and `>>`

`>>` yields a value from the **innermost** enclosing block (spec §6.8):

```solr
var label = if n > 0 { >> "pos"; } else { >> "neg"; };
```

For each block being checked, the checker needs two things: the type the block is expected to
produce (so `>> [];` can be typed), and the types of the `>>` operands seen so far (so that,
when the block ends, they can be compared and turned into the block's type). Blocks nest, and a
`>>` belongs to the innermost one, so these records form a **stack**:

```go
type blockCtx struct {
    want   Type     // expected type of the block's value, may be nil
    yields []Type   // types of the `>>` operands seen so far
}
```

Add `blocks []*blockCtx` to `Analyzer`, and add it to the save/restore in
`checkFunctionBody` (Step 12) next to `curFunc` and `loopDepth`, with `blocks = nil` for the
new function. `checkBlock` pushes an entry on the way in and pops it on the way out, and
`EmitStmt` appends to the top entry.

- `checkBlock(b *ast.BlockExpr, want Type, pre ...Symbol) Type` pushes a `blockCtx`, enters a
  `BlockScope`, `declare`s the `pre` symbols in it, checks each statement, exits, pops, and
  computes the block's type. `pre` is how a construct puts its own names **in the block's
  scope**: the `for` bindings and the `try … else |err|` binding. That makes them behave like
  function parameters: `for _, x in xs { var x = 1; }` is a redeclaration, not shadowing
  (spec §4.4, §6.11). Most calls pass no `pre`. The block's type is:
  - the statements **terminate** (Part 3.6) and nothing was yielded → `NeverT`;
  - nothing was yielded → `VoidT`;
  - otherwise every yield must be identical to the first (report each one that isn't), and the
    block's type is the first yield.
- `EmitStmt`: if `a.blocks` is empty, the `>>` is directly in a function body →
  "`>>` outside a block value; use `return`" (spec §6.8). Otherwise
  `typeOf(value, top.want)` and append to `top.yields`.
- A function body is **not** pushed as a `blockCtx`. `checkFunctionBody` sets `blocks = nil`,
  which is exactly what makes a top-level `>>` an error, and what stops a `>>` inside a
  closure from yielding to the parent's block.

`if`, `switch` and `for` are checked by one function each, with a flag:
`checkIf(n *ast.IfExpr, want Type, asValue bool) Type`, and the same for `switch` and `for`.
`typeOfInner` calls them with `asValue = true`. An `ExpressionStatement` calls them with
`false`. Only the branch-matching and comprehension rules depend on the flag.

### `if` (`IfExpr`), spec §6.9
- The condition must be `bool`.
- `Then` is a block: `checkBlock`. `Else` is a block, another `IfExpr`, or any expression.
- **As a value** (reached through `typeOf`): with an `else`, both branch types must match,
  ignoring a branch whose type is `NeverT` (`if c { >> 1; } else { return; }` is `int`). The
  result is the common type. Without an `else`, the type is `VoidT`.
- **As a statement:** branch types don't have to match.

### `switch` (`SwitchExpr`), spec §6.10
1. **Subject:** `typeOf(Subject, nil)`. Without a subject, every case must be `bool`.
2. **Cases**, handled **before** `typeOf`:
   - an identifier `_` is the default arm. It must be the last arm, and appear once.
     Anything after it: "unreachable arm".
   - `Enum.Variant` (a `MemberExpr` on the subject's enum type) is a variant pattern, allowed
     even for variants with fields. Record the variant name as covered.
   - otherwise `typeOf(case, subjectType)` and it must be identical to the subject's type.
3. **Coverage:** an enum subject needs an arm for every variant, or `_`: "switch is missing
   `Action.Quit`". **A non-enum `switch` used as a value needs `_`** (otherwise there is no
   value when nothing matches, see Part 4.2).
4. **Arm values:** a block arm → `checkBlock`, an expression arm → `typeOf`. As a value, all
   arm types must match, ignoring `NeverT` arms. As a statement, they don't have to.

### `for`, spec §6.11
- `ForWhileExpr`: a non-nil `Condition` must be `bool`. A nil `Condition` is `for { }`.
- `ForInExpr`: the container must be a slice (`str` is **[open]**). Build `VarSymbol`s for
  `IndexBinding` (`int`) and `ValueBinding` (the element type), both `IsMut: false`, `State:
  Resolved`, `Node` the `ForInExpr`, skipping `_`. Pass them to `checkBlock` as `pre`, so they
  live in the body's scope. Don't open a scope of your own. `for i, i in xs` is reported by
  `declare` as a redeclaration.
- `loopDepth++` around the body.
- **As a value** (a comprehension): the body's `>>` operands are collected, and the result
  is `[]T` of the yield type. A `for` used as a value with no `>>`: "a `for` used as a value
  must yield with `>>`".

### `try … else { }` blocks
The `else` block of a `try` is a `checkBlock` with `want` = the unwrapped type. Step 15
covers `try` fully.

**Check:** `ref/control.solr`: `if` as a value with matching and mismatched branches, an `if`
whose `else` returns (fine), a non-`bool` condition, a `>>` in a function body, a `switch` over
an enum missing a variant, `_` not last, a value `switch` over `int` without `_`, `for` over a
non-slice, a comprehension, `break` inside a closure inside a loop (error), a missing return
where an `if` without `else` is the last statement.

## Step 14 — Mutability and copying

**Goal:** spec §5, completely. All rules are in Part 3.7. What to build:

- `rootVar(target ast.Expression) *ast.IdentifierExpr`: follow `MemberExpr.Container` and
  `SubscriptExpr.Receiver` down to an identifier. If the chain ends in anything else (a call,
  a literal), report "cannot assign to this expression" and return nil.
- `lookupVar(id) (*VarSymbol, crossed bool)`: `Lookup` plus "`f` is not a variable" when the
  symbol isn't a `*VarSymbol`.
- `AssignStmt`:
  - target is a plain identifier and the operator is `=` → **rebind**: needs `!crossed`.
  - target is a field or element, **or** the operator is compound (`x += 1`) → **change
    contents**: needs `v.IsMut && !crossed`.
  - Report the precise reason (Part 3.7 lists the messages).
- `VarStmt` with `IsMut`: the initializer must be **fresh** (`isFresh`, Part 3.7), otherwise
  "`mut x = y` needs a copy: write `mut x = @y`".
- `VarStmt` whose initializer is a `CopyExpr`: type its operand. This is the only place `@` is
  accepted. Every other `CopyExpr` hits the error in `typeOf` (Step 11).
- In-place builtin methods (`push`, `sort`): their receiver's root variable must pass the
  "change contents" check (Step 16 marks which methods those are).

**Check:** `ref/mutability.solr`: every line of spec §5.6, plus: rebinding a `var` in an
inner block (fine), `x += 1` on a `var` (error), assigning a top-level `var` from a function
(error), a closure rebinding a parent variable (error), a closure changing its own `mut`
(fine), `mut t = xs[1:3]` (error), `mut t = @xs[1:3]` (fine), `f(@x)` (error), assigning a
field of a call result (error).

## Step 15 — `Result`, `Maybe`, `try`, `must`, interfaces

**Goal:** error handling (spec §8) and structural interfaces (spec §3.7).

### Constructors
| Expression | Type |
|---|---|
| `Ok(x)` | `ResultType{typeOf(x, want's ValueType)}` |
| `Ok()` | `ResultType{VoidT}` |
| `Some(x)` | `MaybeType{typeOf(x, ...)}` |
| `Err(e)` | needs `want` to be a `*ResultType` (from the annotation, the return type, …): otherwise "cannot infer the type of `Err(...)`; add a type annotation". `e` must be assignable to `Error`. Result: `want`. |
| `None()` | needs `want` to be a `*MaybeType`, same message. Result: `want`. |
| `Err()` | "`Err` needs an error value". The parser accepts it for now (Part 4.3). |

With a `want`, `Ok(x)` checks `x` against `want.ValueType` too, so `return Ok("a");` in a
function returning `int!` reports on the `"a"`.

### `try` and `must` (spec §8.3, §8.4)
- `must e`: `e` must be a `ResultType` or `MaybeType`. The result is its `ValueType`.
- `try e` (no `else`): `e` must be a `ResultType` (`Maybe` is **[provisional]**: accept it
  only if you decide to). `curFunc.ReturnType` must be a `ResultType`: "`try` needs the
  function to return a `Result`; use `try … else` or `must`". The result is `ValueType`.
- `try e else v`: same operand rule, no requirement on the function. `typeOf(v, ValueType)`,
  then `expectType`.
- `try e else { … }` and `else |err| { … }`: the block is `checkBlock(block, ValueType)`. Its
  type must be `ValueType` or `NeverT`. For `|err|`, pass a `VarSymbol` named `ErrName`, type
  `ErrorT`, `IsMut: false`, `Node` the `TryExpr`, to `checkBlock` as `pre`, so it lives in
  the block's own scope.
- A `try` or `must` **statement** needs a call operand (`try save(x);`), Part 3.5.

### Interfaces
`implements(t Type, i *InterfaceType) bool`: `t` is a struct or enum, and for every method `m`
of `i`, `t`'s `MethodSet` has a method with the same name, the same number of parameters,
`identical` parameter types **after `self`** (`Params[1:]`), and an `identical` return type.
`Params[0]` is skipped: it is `SelfT` on the interface side and `t` on the method side, and
the receiver always matches by construction. There is no `Self` type (spec §3.7), so no
other position can refer to the implementing type, and no substitution is needed.

A missing method should be reported precisely when the check is the reason for an error:
"`Rect` does not implement `Shape`: missing method `name`" (or "method `area` has type ...,
want ..."). Return the reason from `implements` as a string, so `expectType` can use it.

Extend `assignable(from, to)`: also true when `to` is an `*InterfaceType` and
`implements(from, to)`, or `from` and `to` are the same interface. Every rule that compares
types through `expectType` (arguments, `return`, `var` annotations, assignments, constructor
fields, the `Err` payload) gets interfaces for free.

**Check:** `ref/results.solr`: each row of the constructor table, `try` in a function that
doesn't return a `Result`, `try … else` with a wrong-typed fallback, `try … else |e| {
return; }`, `must` on an `int`, `Err(1)` (not an `Error`), a struct implementing `Error` and
passed to `Err`, a struct missing one interface method assigned to an interface variable.

## Step 16 — Builtin functions and methods

**Goal:** spec §10.

- `len(x)`: the callee type is `*BuiltinFuncType{"len"}`. One argument, a slice or `str` →
  `int`.
- Builtin methods, looked up by `MemberExpr` when the receiver is a slice or `str`. Each
  entry builds an ordinary `FunctionType` from the element type `T`, so calls need no special
  handling:

  | Receiver | Method | Type | In place |
  |---|---|---|---|
  | `[]T` | `append` | `fn(T) -> []T` | |
  | `[]T` | `prepend` | `fn(T) -> []T` | |
  | `[]T` | `get` | `fn(int) -> T!` | |
  | `[]T` | `contains` | `fn(T) -> bool` | |
  | `[]T` | `find` | `fn(fn(T) -> bool) -> T!` **[open]** | |
  | `[]T` | `push` | `fn(T) -> void` | yes |
  | `[]T` | `sort` | `fn() -> void`, `T` must be `int`, `float` or `str` | yes |
  | `str` | `get` | `fn(int) -> str!` | |

- For an "in place" method, the call check runs Step 14's "change contents" rule on the
  receiver's root variable.
- `.with` is handled in the call rule (Step 11), not here.

**Check:** `ref/builtins.solr`: each method once, `push` on a `var` slice (error), `push` on a
`mut` slice (fine), `len` of an `int` (error).

## Step 17 — Acceptance

Write (or port) one realistic program for the current language: a todo app or the lexer,
with `impl`, `var` rebinding, `mut` with `@`, `.with`, spread, `Some`/`None`, `try … else`,
`for`, and `switch` over an enum. It should check with zero errors. Then copy it and add one
mistake per line from each earlier golden file, and check that each produces exactly one
error.

The existing programs in `reference_programs/` are written for older models and don't parse,
so they are not usable as they are.

---

# Part 3 — Rule reference

## 3.1 AST coverage

Every AST node, and the step that handles it. A node not handled yet goes to the loud
`default` (§1.8).

| Node | Where |
|---|---|
| `ImportStmt` | Step 6 |
| `TypeDeclStmt`, `TypeAliasStmt` (+ `StructType`, `EnumType`, `EnumMember`, `InterfaceType`, `Field`, `TypeParameter`) | Steps 6, 7 |
| `FunctionStmt`, `FunctionSignature`, `Parameter` | Steps 6, 8, 12 |
| `ImplDeclStmt` | Steps 6, 8 |
| `VarStmt` | top level: Steps 6, 10. Local: Steps 12, 14. |
| `AssignStmt`, `ReturnStmt`, `BreakStmt`, `ContinueStmt`, `ExpressionStatement` | Step 12 |
| `EmitStmt` | Step 13 |
| `BadStmt` | skip (the analyzer only runs on programs without parse errors) |
| `NamedType`, `SliceType`, `ResultType`, `MaybeType`, `FunctionType` | Step 7 |
| `IntegerLiteralExpr`, `FloatLiteralExpr`, `StringLiteralExpr`, `BoolLiteralExpr`, `IdentifierExpr`, `NegativeExpr`, `NotExpr`, `InfixExpr` | Step 9 |
| `CallExpr`, `Argument`, `MemberExpr`, `SubscriptExpr`, `SliceExpr`, `SequenceLiteral`, `SpreadExpr` | Step 11 |
| `CopyExpr` | Steps 11, 14 |
| `FunctionExpr` | Steps 10, 12 |
| `BlockExpr`, `IfExpr`, `SwitchExpr`, `SwitchArm`, `ForInExpr`, `ForWhileExpr` | Step 13 |
| `ResultExpr`, `MaybeExpr`, `TryExpr`, `MustExpr` | Step 15 |

## 3.2 Operators (spec §9)

Both operands must have the same type. There are no implicit conversions.

| Operators | Operands | Result |
|---|---|---|
| `+ - * /` | `int, int` / `float, float` | the same type |
| `%` | `int, int` | `int` |
| `+` | `str, str` | `str` |
| `== !=` | the same primitive type, or the same enum | `bool` |
| `< <= > >=` | `int`, `float` or `str` pairs | `bool` |
| `and or` | `bool, bool` | `bool` |
| unary `-` | `int` or `float` | the same type |
| unary `!` | `bool` | `bool` |

`==` on structs, slices, results and maybes is **[open]** (spec §9.1): report "`==` is not
supported for `Point` yet". Enums are allowed because `switch` compares with `==`.

## 3.3 Assignability

`assignable(from, to)` is true when:
1. either is `InvalidT` (already reported), or
2. `from` is `NeverT` (the code never produces a value), or
3. `identical(from, to)`, or
4. `to` is an interface and `from` implements it (Step 15).

There are no other conversions. `int` is never assignable to `float`.

## 3.4 Construction (structs, enum variants, `.with`)

Spec §3.4, §3.5, §10.3:
- Arguments are **named**. A positional argument: "fields must be named: `Point(x=1)`".
- Each name must be a field of the struct (or variant): "`Point` has no field `z`". A name
  given twice is an error.
- Each value: `typeOf(value, field.Type)`, then `expectType`.
- **Spread** (`Todo(...t, title="y")`): at most one, and it must come first. Its operand must
  have the type being constructed. It supplies every field, and the named arguments after it
  replace fields.
- **Omitted fields** (no spread): a field of type `int`, `float`, `str`, `bool` or a slice
  takes its zero value. A `Maybe` field is `None()`. Any other field must be given: "missing
  field `status`".
- `x.with(f=v)` is construction of `x`'s type with `...x` first.
- The result is the struct type (or the enum type, for a variant).

## 3.5 Expression statements

Spec §7.2. An `ExpressionStatement` may be:
- a call (`f(x);`, `xs.push(1);`);
- `try` or `must` whose operand is a call (`try save(x);`);
- an `if`, `switch`, or `for`, or a block.

Anything else (`1 + 2;`, `x;`, `try x;`): "expression has no effect". A call statement whose
result isn't `void` is allowed: the value is discarded.

## 3.6 Termination

`terminates(stmts []ast.Statement) bool` decides whether the code always leaves with
`return`, `break` or `continue`. It looks at the **last** statement:
- `ReturnStmt`, `BreakStmt`, `ContinueStmt` → true;
- an `ExpressionStatement` holding:
  - a `BlockExpr` → `terminates` of its statements;
  - an `IfExpr` with an `Else` → both branches terminate (an `Else` that is an `IfExpr`
    recursively);
  - a `SwitchExpr` with `_` or full enum coverage → every arm's value terminates (only block
    arms can);
  - a `ForWhileExpr` with no `Condition` and no `break` that belongs to it → true (spec §6.11:
    it never ends);
- anything else → false.

`must` and panics don't count. Used for "missing return" (Step 12) and `NeverT` blocks
(Step 13).

## 3.7 Mutability (spec §5)

| Operation | Allowed when | Error |
|---|---|---|
| rebind `x = e` | `x` is a variable declared in the current function (not `crossed`) | "cannot assign to `x` from inside a closure" / "cannot assign to top-level `x`" |
| change contents `x.f = e`, `x[i] = e`, `x += e`, `x.push(e)` | `x` is `IsMut` **and** not `crossed` | "cannot change `x`: declare it with `mut x = @...`" (for a `var`) / "parameters are immutable" / "`for` bindings are immutable" / "cannot change `x` from inside a closure" |
| `mut x = e` | `e` is fresh | "`mut x = y` needs a copy: write `mut x = @y`" |
| `@e` | only as the whole initializer of `var` or `mut` | "`@` is only allowed as a variable initializer" |

`crossed` comes from `Lookup`. A top-level `var` is always `crossed` when read from a function,
so it can be read but never assigned. A closure's parent variables are `crossed` for the same
reason.

**`isFresh(e)`** (spec §5.3):
- fresh: `CopyExpr`, literals, sequence literals, `ResultExpr`, `MaybeExpr`, calls (including
  construction and `.with`), `NegativeExpr`, `NotExpr`, `InfixExpr`, `FunctionExpr`;
- `IfExpr`, `SwitchExpr`, `BlockExpr`: fresh when every branch value (every `>>` operand, and
  every expression arm) is fresh;
- `TryExpr`, `MustExpr`: fresh when the operand is;
- not fresh: `IdentifierExpr`, `MemberExpr`, `SubscriptExpr` (index **and** slice).

---

# Part 4 — Scope of the first version

## 4.1 Left out on purpose

| Feature | What the analyzer does |
|---|---|
| Generics (type parameters, `Pair[A, B]`, `impl Pair[A, B]`) | "not supported yet" (Steps 7, 8) |
| Enum payload narrowing in `switch` (spec §6.10, **[provisional]**) | "no field" on the subject. Add it after Step 15: in an arm whose case is `Enum.Variant`, field reads on the subject variable use that variant's fields. |
| Static functions `Type.fn()` | "not supported yet" (Step 11) |
| Module contents (`fmt.println`) | `InvalidT` without an error (Step 11) |
| Visibility (`pub`) | recorded, not enforced: there is only one module |
| Tuples, maps, fixed arrays | not parsed yet |

## 4.2 Open decisions, and the default this plan uses

Each of these is open in the spec. The plan picks the default in the right column so you're
not blocked. Change the plan if you decide otherwise.

| Question | Default here |
|---|---|
| May a parameter be rebound (`x = ...`) inside its function? (spec §4.2) | Yes: it falls out of the rebind rule, since a parameter is a variable of the current function. |
| `==` on structs and other composites (spec §9.1) | Error for now. Enums allowed. |
| A struct that contains itself with no `?` or slice in between (`type N struct { next: N }`) can never be constructed. Is it an error? | Not checked in the first version. To add it later: after 7f, walk each struct's fields, following struct-typed fields only (not through `?` or `[]`), and report a struct reached again. |
| `try` on a `Maybe` (spec §8.3, provisional) | Not allowed until decided. |
| A `switch` over a non-enum used as a value, without `_` | Error, because no arm may match. |
| A spread in a construction: position and count (spec §3.4, provisional) | At most one, first. |
| Iterating a `str` with `for` | Not allowed until decided. |

## 4.3 Parser gaps the checker sees

The parser still accepts these (spec §15). Each can be fixed in the parser, which is the
better place, or caught in the checker as noted.

| Input | Checker handling | Parser fix |
|---|---|---|
| `Err()` | Step 15: "`Err` needs an error value" | In `parseResultExpr`, take the early return on `)` only for `Ok`. |
| `a:b`, `xs[a:b:c]` | Step 11: `SliceExpr` outside `[ ]` is an error. `xs[a:b:c]` arrives as a `SliceExpr` whose `End` is another `SliceExpr`: report it. | Remove `COLON` from `PMap` and the prefix/infix tables, delete `parseSliceExprPrefix`/`parseSliceExprInfix`, and parse `[start:end]` inside `parseSubscriptExpr`. |
| `fn f();` (no body) | Step 8: "function `f` has no body" | Add a `default` case that reports "expected function body" to the `switch` in `parseFunctionStmt` and `parseFunctionExpr`. |
| `pub impl`, `pub import`, `pub` before a local `var` | Ignored (visibility isn't enforced) | In `next()`, report when `pub` precedes `import` or `impl`. In `parseStatement`, stop accepting `pub`. |
| `fn f(x) {}` (no parameter type) | Step 8: "parameter `x` needs a type" | Keep in the checker. |
| `1 + 2;` as a statement | Part 3.5 | Keep in the checker. |
