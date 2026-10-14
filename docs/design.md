# Stellar Language Design

Status: draft, 2026-10-14. Source files use the `.st` extension.

Sections marked **(provisional)** were not explicitly discussed; they are
proposed defaults, open to change. Everything else is a recorded decision.

## 1. Goals

- Strong, strict static typing. No implicit conversions, no null.
- No user-defined generics. Polymorphism comes from interfaces plus a small
  set of compiler-provided parametric types.
- Nominal interfaces (`impl X for T`).
- First-class functions.
- Data is immutable by default; mutability is explicit everywhere.
- Value semantics: values behave as copies, so mutable state is never
  shared by accident. `Ref[T]` opts into sharing explicitly (§5).
- Errors are values. Every error must be handled. There are no exceptions.

## 2. Syntax rules

These rules were settled while building the parser and reviewed
afterwards. All are decided.

### 2.1 Lexical structure

- Comments are `// line` and `/* block */`.
- Identifiers use camelCase for values and functions.
- **Declared type names (objects, interfaces, enums) must be PascalCase**:
  they start with an upper-case letter and contain no underscores. The
  parser rejects `type book object` and `type Snake_Case enum`.
- Primitive types (`int32`, `str`, `bool`, ...) are lower case. Every other
  type, built-in or user-defined, is PascalCase: `Result`, `Maybe`, `Map`,
  `Error`.
- Type names such as `int32`, `str`, `Result`, `Map`, `self` and `Self` are
  identifiers, not keywords.
- Keywords: `pub fn type object interface enum impl for let var mut return
  if else match try is as in while break continue and or not true false
  import`.
- Integer literals: `42`, `1_000`, `0xFF`, `0b1010`, `0o17`. Float literals
  need digits on both sides of the point, or an exponent: `1.5`, `1e10`.
  So `0..n` is a range, not a float.
- String escapes: `\n \t \r \0 \\ \"` and `\u{1F600}`.
- `!` is not an operator; the lexer suggests `not`.

### 2.2 Line breaks

There are no semicolons. As in Go, a line break ends a statement only if the
line's last token could end one: an identifier, a literal, `)`, `]`, `}`,
`return`, `break`, `continue`, `true` or `false`. Consequences:

- To continue an expression, end the line with an operator or `,`.
- `else` must be on the same line as the preceding `}`.
- Inside `( )`, `[ ]` and object literals, line breaks between elements are
  ignored, and a trailing comma is allowed.

### 2.3 Expression grammar

Precedence, lowest to highest:

| Level | Operators |
|---|---|
| 1 | `or` |
| 2 | `and` |
| 3 | `not` |
| 4 | `== != < <= > >=`, `is` (cannot be chained: `a < b < c` is an error) |
| 5 | `..` (cannot be chained) |
| 6 | `+ -` |
| 7 | `* / %` |
| 8 | unary `-`, `try` |
| 9 | call, `.field`, `[index]`, object literal |

- `try f() + 1` means `(try f()) + 1`.
- A `try ... else` fallback extends as far right as possible:
  `try f() else a + b` means `try f() else (a + b)`.
- **Object literals are decided by whitespace.** `Name{`, with no space, is
  always an object literal; `Name {`, with a space, never is. So literals
  work anywhere, including in headers:

  ```
  for x in Range{ lo: 0, hi: n } { ... }
  if p == Point{ x: 0, y: 0 } { ... }
  ```

  The common mistakes get targeted errors and are then parsed as intended:
  `Book { title: t }` (space in a literal), `if done{` (lower-case names are
  never types), and `Color.Red{` followed by something other than
  `field: value`.
- `let` and `var` always require an initial value.

## 3. Types

### 3.1 Primitive types

| Type | Notes |
|---|---|
| `int8 int16 int32 int64` | Overflow panics (provisional) |
| `uint8 uint16 uint32 uint64` | Overflow panics (provisional) |
| `float32 float64` | IEEE 754 |
| `bool` | `true` / `false` |
| `str` | Immutable UTF-8 |

There is no unit or `void` type in the surface language (see §6.4).

### 3.2 Compiler-provided parametric types

Users cannot declare parametric types. Only the following exist, and all of
them use square brackets:

| Type | Meaning |
|---|---|
| `Result[T]` | `Ok(T)` or `Err(Error)` |
| `Result` | `Ok()` or `Err(Error)`: a fallible operation that has no success value |
| `Maybe[T]` / `?T` | `Some(T)` or `None` |
| `[]T` | Array |
| `Map[K, V]` | Map. Keys are limited to primitives and `str` in the MVP (provisional). |
| `Ref[T]` | Shared, writable reference (§5.5). `Ref(x)` creates one. |

`None` is the only "nothing" in the language. It is a variant of `Maybe[T]`
and is never assignable to `T`:

```
let b: Book = None          // error
let m: ?Book = None         // ok
```

### 3.2.1 Array and map literals

Like every literal, array and map literals take their type from context:

```
let xs: []int32 = [1, 2, 3]
let none: []int32 = []
let m: Map[str, int32] = ["a": 1, "b": 2]
let empty: Map[str, int32] = [:]
let bad = ["a": 1]          // error: literal has no type
```

Reading `m["a"]` returns `?int32`; `m["a"] = 1` requires `m` to be a `var`.

### 3.3 Objects

```
pub type Book object {
    pub title: str
    pub author: Author
    isbn: str               // private to this module
}
```

Construction uses named fields, and every field is required:

```
let b = Book{ title: "Dune", author: a, isbn: "978-0441013593" }
```

Literals with private fields can only be constructed inside the defining
module. Other modules use constructor functions (§7.3).

### 3.4 Enums (sum types)

```
pub type Shape enum {
    Circle(r: float64)
    Rect(w: float64, h: float64)
    Empty
}

let s = Shape.Circle(r: 1.5)    // variant fields are always named
```

Matching on an enum must be exhaustive (§8.3).

### 3.5 Function types

```
fn(int32, int32) -> Result[int32]
fn(str)                 // produces no value
```

### 3.6 Interfaces

```
pub type Sortable interface {
    fn sortKey(self) -> str
}
```

`self` is the receiver and has type `Self`, which is the implementing type.
An interface can be used as a type: `let s: Sortable = book`.

## 4. Literals and type inference

Literals of every kind, whether integer, float, or string, **have no
default type**. A literal takes its type from its context: a type
annotation, a parameter type, a field type, or the other operand of an
operator. If there is no context, the compiler reports an error.

```
let a: int32 = 5            // ok
let b = add(5, 6)           // ok: the parameters are int32
let c = a + 1               // ok: 1 takes a's type
let d = 5                   // error: cannot infer type of literal
let e: int8 = 300           // error: 300 overflows int8
let f: int32 = 1.5          // error: not representable
let g: float64 = 5          // error: integer literal cannot be a float
let h: float64 = 5.0        // ok
let s = "hi"                // error
let s: str = "hi"           // ok
let xs: []int32 = [1, 2, 3] // ok
let n = 1 + 2               // error
```

`true` and `false` are named constants of type `bool`. Object and enum
literals name their type explicitly.

Local inference applies only to expressions whose type is already fully
determined: `let r = add(a, b)` infers `Result[int32]`. Function
signatures are always fully annotated.

There are no implicit numeric conversions. Casts are explicit:
`int64(x)`, `float64(x)`, `str(x)`.

## 5. Values and mutability

Stellar uses **mutable value semantics** (decided 2026-10-15, replacing the
earlier reference model): values behave as if they are copied whenever they
are assigned, passed, stored or returned, so mutable state is never shared
by accident. The implementation shares memory internally until something is
written, so passing values stays cheap.

### 5.1 `let` and `var`

| Declaration | Reassign? | Change in place? |
|---|---|---|
| `let b = ...` | no | no |
| `var b = ...` | yes | yes |

Changing a field (`b.title = "x"`) updates the value held in the variable,
so it needs a `var`, exactly like reassignment. There is no `mut` in variable
or field types.

```
var b = Book{ title: "Dune", authorName: "Herbert" }
let shelf: []Book = [b]        // the shelf holds its own Book
b.title = "Dune Messiah"       // shelf[0].title is still "Dune"
```

An immutable container therefore always holds immutable contents: the
contents are part of its value.

### 5.2 `mut` parameters

A function changes its caller's data only through a `mut` parameter. The
definition declares it and **the call site marks it**:

```
fn rename(b: mut Book, t: str) {
    b.title = t
}

var b = Book{ ... }
rename(mut b, "New Title")     // b changes; the marker makes that visible
```

A `mut` argument must be a place that can be written to: a `var`, a `mut`
parameter, or a field or index path starting from one (`shelf.books[0]`).
Passing a `let` or a temporary (a value not held in any variable, such as the
result of a call) is an error, because the change would be lost. Writing to
an index that does not exist panics.

The same place, or overlapping parts of it, cannot be passed to two `mut`
parameters of one call: `swap(mut a, mut a)` and
`merge(mut shelf, mut shelf.books[0])` are errors. Passing a variable both as
`mut` and as an ordinary argument is fine; the ordinary argument is a
snapshot taken before the call.

Methods declared with `mut self` follow the same rules, and the call marks
the receiver:

```
mut b.retitle("x")             // b must be a writable place
b.retitle("x")                 // error: retitle changes b; mark the call
mut b.label()                  // error: label does not change b
```

A `mut self` method cannot be used as a function value bound to its
receiver (`b.retitle`); use `Book.retitle`, whose first parameter is
`mut Book`. The guiding rule: **data never changes unless the code that
changes it is marked.**

### 5.3 Closures

A closure may capture `let` values and parameters, and nothing that can
change: not a `var`, not a `mut` parameter, not `self` in a `mut self`
method. Closures therefore never change captured variables, and there is a
single `fn` type.

```
let minRating: int32 = 4
let good = filter(books, fn(b: Book) -> bool { return b.rating >= minRating })

var count: int32 = 0
let inc = fn() { count = count + 1 }      // error: cannot capture var count
let snapshot = count
let show = fn() { print(str(snapshot)) }  // ok: copy the var into a let
```

State that must change across calls is either passed in as a `mut`
parameter or shared through a `Ref` (§5.5). A local closure cannot call
itself; recursion uses top-level functions.

### 5.4 Loops

The loop variable is a copy: changing it does not change the collection. To
change elements in place, index them:

```
for i in 0..len(books) {
    books[i].rating = 0        // books must be a var
}
```

### 5.5 Shared mutable data: `Ref[T]`

When data really must be shared and changed in one place, `Ref[T]` is an
explicit reference. Copying a `Ref` copies the reference, not its target.

```
let herbert: Ref[Author] = Ref(Author{ firstName: "Frank", lastName: "Herbert" })
let dune = Book{ title: "Dune", author: herbert }
let messiah = Book{ title: "Dune Messiah", author: herbert }
herbert.firstName = "F."       // both books see the change
```

- A `Ref` is always writable. Read-only references are unnecessary, because
  immutable values can already be shared safely.
- `let` versus `var` on a `Ref` decides whether the variable can point at a
  different reference, not whether the target can change. Writing through a
  `let` Ref is allowed.
- Fields and methods are reached through a `Ref` automatically.
- `==` on two Refs means "same target".
- A `Ref` is the mark for shared mutation everywhere, consistently: in
  parameters, fields and closures. Passing a value that contains a `Ref`
  needs no `mut` marker; the `Ref` in the type is the mark.
- The whole target of a `Ref` is read or replaced with `r.value`, which
  matters for primitives such as `Ref[int32]`. If the target has its own
  field named `value`, reach it with `r.value.value`.
- Indexing does not reach through a `Ref`; write `r.value[0]`.
- A `Ref` changes *writability*, not *marking*. Its target is writable even
  when the variable holding the `Ref` is a `let`, but calls are marked as
  everywhere else: `mut herbert.rename("x")`, `rename(mut herbert.value, "x")`.
  Plain assignments are never marked: `herbert.name = "x"`.
- Two `mut` arguments that reach the same target through different `Ref`s
  cannot be detected by the checker. The behaviour is well defined (both see
  the same data); a run-time check may come later.

```
let counter: Ref[int32] = Ref(0)
let inc = fn() { counter.value = counter.value + 1 }   // closures share state through a Ref
```

### 5.6 Covariance

Arrays are covariant: `[]Book` is accepted as `[]Sortable` when `Book`
implements `Sortable`. This is sound because the array is a value; nothing
can write a different `Sortable` into the original `[]Book`.

## 6. Errors and absence

### 6.1 `Error`

`Error` is a built-in interface:

```
pub type Error interface {
    fn message(self) -> str
}
```

Users define their own error types by implementing it.

`Err("msg")` wraps the message in a built-in concrete type that implements
`Error`. That type is hidden: it has no name and cannot be created any other
way.

Wrapping and inspecting errors use built-in functions, as in Go, so user
error types only ever implement `message()`:

```
let sum = try add(1, 2) else |e| {
    return Err(wrap(e, "adding"))    // message: "adding: <e's message>"
}

match cause(e) {                     // the wrapped error, or None
    Some(c) => match c {
        is NotFound => ...
        _ => ...
    }
    None => ...
}
```

- `wrap(e: Error, context: str) -> Error` returns a hidden wrapper type that
  keeps the original error, rather than flattening it into a string.
- `cause(e: Error) -> ?Error` returns the wrapped error, or `None` if `e` was
  not produced by `wrap`. Combined with `is` type tests, this replaces Go's
  `errors.Unwrap`, `errors.Is` and `errors.As`.

### 6.2 Constructing results

Construction is always explicit. A bare `return 1` in a `Result[int32]`
function is a type error.

```
pub fn add(a: int32, b: int32) -> Result[int32] {
    if a > 100 or b > 100 {
        return Err("Both values must be < 100")   // Err accepts str or Error
    }
    return Ok(a + b)
}

pub fn save(b: Book) -> Result {
    ...
    return Ok()
}

pub fn findByIsbn(books: []Book, isbn: str) -> ?Book {
    for b in books {
        if b.isbn == isbn { return Some(b) }
    }
    return None
}
```

### 6.3 `try`

`try` unwraps a `Result[T]` or `Maybe[T]`. It **never returns from the
enclosing function implicitly.**

| Form | `Result[T]` | `Maybe[T]` |
|---|---|---|
| `try x` | `Err` → panic | `None` → panic |
| `try x else expr` | `Err` → `expr` | `None` → `expr` |
| `try x else \|e\| expr` | `Err` → `expr`, with `e: Error` | compile error |
| `try x else { ... }` | `Err` → block value | `None` → block value |
| `try x else \|e\| { ... }` | `Err` → block value, with `e: Error` | compile error |

The fallback `expr`, or the block's last expression (§8.4), must have type
`T`. Alternatively the block may diverge: end in `return`, `panic`, `break`,
or `continue` on every path.

```
let sum = try add(1, 2)
let sum = try add(1, 2) else 0
let sum = try add(1, 2) else |e| {
    return Err(wrap(e, "adding"))
}
let first = try books[0] else { return Err("no books") }
try save(b)
```

### 6.4 Values with no result

A function without a return type produces no value. Using its call as a
value is a compile error:

```
fn log(msg: str) { ... }
log("hi")                   // ok
let x = log("hi")           // error: log produces no value
```

### 6.5 Discarding

No value may be dropped implicitly. A call that produces a value cannot be a
bare statement.

```
compute()                   // error: unused int32
let _ = compute()           // ok
let _ = findByIsbn(bs, i)   // ok: Maybe may be discarded explicitly
let _ = add(1, 2)           // error: a Result can never be discarded
let _ = try add(1, 2)       // ok: the error is handled
try save(b)                 // ok: produces no value
```

### 6.6 Indexing

Array indexing and map lookup return `?T`:

```
let b = try books[0]                  // panic if out of bounds
let b = try books[i] else fallback
```

### 6.7 Panics

A panic prints its message and stack trace, then exits with a non-zero code.
Nothing can catch it. Sources: `try` without `else`, `panic("msg")`,
integer overflow, and division by zero (provisional).

## 7. Interfaces and methods

### 7.1 Implementations

Interface implementations are nominal only. A type satisfies an interface
only through an explicit `impl`:

```
impl Sortable for Book {
    fn sortKey(self) -> str {
        return self.isbn
    }
}
```

Methods in an interface impl have no `pub`; their visibility follows the
interface.

### 7.2 Orphan rule

An `impl X for T` must live in the module that defines `X` or the module
that defines `T`.

### 7.3 Inherent methods

Inherent methods are methods that belong to a type rather than an
interface:

```
impl Book {
    pub fn new(title: str, author: Author, isbn: str) -> Book {
        return Book{ title: title, author: author, isbn: isbn }
    }

    pub fn label(self) -> str {
        return self.title + " by " + self.author.lastName
    }

    pub fn retitle(mut self, t: str) {         // changes the caller's Book
        self.title = t
    }
}
```

- They may only be written in the module that defines the type.
- Each one needs its own `pub`.
- A function without `self` is static: `Book.new(...)`.
- An inherent method may not share a name with an interface method the type
  implements (compile error).

### 7.4 Method calls

- `b.label()` is call syntax for `Book.label(b)`. Methods are first-class:
  `let f = Book.label`.
- If two implemented interfaces declare the same method name, the call must
  be qualified: `Sortable.sortKey(b)`.

## 8. Expressions and statements

### 8.1 Operators (provisional)

| Kind | Operators |
|---|---|
| Arithmetic | `+ - * / %` (`+` also concatenates `str`) |
| Comparison | `== != < <= > >=` |
| Logical | `and or not` |

- `==` compares structure. Function values are not comparable.
- Values have no identity. `==` on two `Ref`s compares targets (§5.5).

### 8.2 Type checks

`x is T` tests the concrete type held by an interface value. `as name`
optionally binds the value, typed as `T`, for use inside the branch:

```
if s is Book as b {
    print(b.title)
}
if s is Book {              // test only; no binding
    print("a book")
}
```

- `T` must be a type that implements the interface being tested. Testing
  for a type that cannot be held (e.g. `is int32` on a `Speaker` that
  `int32` does not implement) is a compile error.
- There is no `any` type. Type tests work only on interface values.

### 8.3 Match (provisional syntax)

```
match shape {
    Shape.Circle(r) => print(str(r))
    Shape.Rect(w, h) => { ... }
    Shape.Empty => { }
}

match add(1, 2) {
    Ok(v) => ...
    Err(e) => ...
}

fn describe(s: Speaker) -> str {
    return match s {
        is Author as a => "Hi " + a.firstName
        is Robot as r  => "Beep " + r.serial
        is Book        => "a book"
        _              => s.hello()
    }
}
```

- Every `match` must cover all cases; the compiler rejects one that doesn't.
- `_` is a wildcard arm covering everything not matched above it.
- Enum and `Result`/`Maybe` matches can be covered by listing every variant.
- Interface matches always need `_`. The set of implementing types is open,
  since any module may implement the interface for its own types.
- `if` and `match` are expressions. See §8.4 for how branches produce
  values.

### 8.4 Block values

When a block is used as a value, its value is its **last expression**:

```
let price: float64 = match item {
    is Book as b => b.basePrice
    is Magazine as m => {
        let base: float64 = m.issuePrice
        let discount: float64 = base * 0.1
        base - discount
    }
    _ => 0.0
}
```

Whether a block is used as a value depends only on where it appears:

- **Value position:** the right side of `let`, `var` or an assignment; a
  function argument; a `return` operand; an operand of an operator; a field
  initializer; a `try ... else` fallback; or a branch of an `if` or `match`
  that is itself in value position.
- **Statement position:** everywhere else.

Rules:

1. In value position, the block's last statement must be an expression of
   the expected type, or the block must diverge (`return`, `panic`, `break`,
   `continue`). A block ending in `let`, an assignment, or a call that
   produces no value is an error: "block produces no value".
2. In statement position, the last expression is an ordinary statement, so
   the strict discard rule (§6.5) applies to it.
3. Every branch of an `if` or `match` in value position must produce the
   same type. Diverging branches are compatible with any type.
4. An `if` in value position must have an `else`.
5. Assignment is a statement, not an expression; `{ x = 5 }` produces no
   value.
6. Untyped literals in branches take their type from the context, as
   anywhere else (§4).

```
let x = if c { 1 } else { 2 }               // error: literal has no type
let x: int32 = if c { 1 } else { 2 }        // ok

if c { compute() } else { compute() }       // error: unused int32 (rule 2)
let _ = if c { compute() } else { 0 }       // ok

let y: int32 = if c { 1 }                   // error: missing else (rule 4)
let z: int32 = if c { 1 } else { log("x") } // error: no value (rule 1)
let w: int32 = if c { 1 } else { return Err("no") }  // ok: diverges
```

Function bodies are the exception: they always use explicit `return`, and
the last-expression rule does not apply to them.

### 8.5 Control flow (provisional)

- `if` / `else if` / `else`
- `while cond { }`
- `for x in xs { }`
- `break` and `continue`
- `for i in 0..n { }`: the range takes its type from `n`.

### 8.6 Built-in container operations

There are no user generics, so the compiler provides typed operations on
built-in containers: `len`, `push` (whose array argument is `mut`), `map`, `filter`,
and `reduce`. Their exact signatures are provisional.

## 9. Modules (provisional)

- Each file is one module. `pub` marks exports.
- `import "path/to/module"` makes the module available; its members are
  reached as `module.name`.

## 10. Built-ins (MVP)

| Built-in | Purpose |
|---|---|
| `print(s: str)` | Write a line to output |
| `panic(msg: str)` | Abort the program |
| `wrap(e: Error, context: str) -> Error` | Add context to an error, keeping it as the cause (§6.1) |
| `cause(e: Error) -> ?Error` | The error wrapped by `wrap`, if any (§6.1) |
| `len(x)` | Length of an array, map, or string |
| `str(x)`, `int32(x)`, ... | Explicit conversions |

## 11. Implementation plan (Go)

```
cmd/stellar/        CLI: `stellar parse`, `stellar check`, `stellar run`
internal/token/     token kinds and positions
internal/diag/      diagnostics shared by all phases
internal/lexer/
internal/ast/
internal/parser/    parser; testdata/ok and testdata/errors golden files
internal/printer/   AST printer (canonical form; drops comments and blank lines)
internal/types/     type checker
internal/interp/    tree-walking interpreter
examples/
```

| Phase | Deliverable |
|---|---|
| 1 | Lexer, parser, AST, and pretty-printer, with golden tests (done) |
| 2 | Type checker: primitives, functions, objects, mutability, literal inference, `Result`/`Maybe`/`try`, discard rules (done, including the phase 4 checks below) |
| 3 | Tree-walking interpreter (done: value semantics by copy-on-write, `mut` arguments by copy-in/copy-out) |
| 4 | Interfaces and `impl`, enums with `match`, arrays and maps, `is` narrowing |
| 5 | Modules, CLI, minimal standard library, example programs |

Provisional choices made while building the checker:

- Indexes are `int64`, and `len()` returns `int64`.
- `fn main()` or `fn main() -> Result` is the entry point.
- Converting `str` to a number is not supported until parsing functions
  returning `Result` exist.
- Iterating over a `Map` yields its keys.

Later work: transpile to Go.

## 12. Open questions

1. ~~Can an integer literal initialize a float?~~ No.
2. ~~Call-site marker for mutation?~~ Superseded: `mut` arguments are marked
   at the call, `rename(mut b, "x")` (§5.2).
3. ~~Split `let mut`?~~ Superseded by value semantics: `var` covers both
   reassignment and in-place change (§5.1).
4. ~~Enum construction syntax~~: named fields, `Shape.Circle(r: 1.5)`.
5. ~~Should `if` and `match` be expressions?~~ Yes. A block's value is its
   last expression (§8.4).
6. ~~Type-test arms inside `match`~~: `is T as name`, name optional (§8.2).
7. ~~Conversion syntax~~: keep `str(x)`, `int64(x)`; no `cast()`.
8. ~~Map literal syntax~~: `["a": 1]`, empty `[:]`, typed by context (§3.2).
9. ~~Review the parser-level syntax rules in §2~~: reviewed.
10. ~~Built-in type names~~: primitives (`int32`, `str`, `bool`, ...) stay
    lower case; user-defined types must be PascalCase. Built-in
    non-primitives are PascalCase too: `Error`, `Map[K, V]`.
11. ~~Constructing an `Error` from a string~~: only via `Err("msg")`; the
    concrete type is hidden (§6.1).
12. ~~Error wrapping~~: built-in functions `wrap(e, context)` and `cause(e)`,
    following Go (§6.1).
13. ~~Shorter syntax for wrap-and-fail~~: none. Failing with context is
    `return Err(wrap(e, "context"))`, as in Rust without `?` or `.context()`.
    Considered and rejected: `Err(e, "context")` and a context-dependent
    `wrap` that produces `Err(...)` where a Result is expected.
14. ~~Mutability model~~: mutable value semantics (§5), chosen over shared
    references with deep immutability and over references with a read-only
    view type. Removes `mut` types, `copy()`, `same()` and `mut fn`.
15. ~~Mark `mut self` receivers?~~ Yes: `mut b.retitle("x")` (§5.2).
16. ~~Closures changing captured state~~: closures capture only `let`
    values and parameters (§5.3), as in Java.
17. ~~Refs and markers~~: the `Ref` type is the mark that its target is
    shared and writable; passing a `Ref` needs no `mut`. Calls that change a
    receiver or `mut` argument are still marked, including through a `Ref`
    (§5.5). This last point was decided during implementation; confirm.
18. Future: a built-in `Iterable` interface (`fn items(self) -> []T`) so
    user containers work with `for`, `map`, `filter` and `reduce` without
    generics; the element type comes from each impl's signature.
