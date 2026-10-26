# Stellar — Value-Semantics Variant (branch `immutability`)

> Exploration spec for branch `immutability`. Since 2026-10-24 the direction is
> `reference_programs/lexer2/main.solr`: **values are immutable by default, and
> mutation is explicit and exclusive**. The earlier fully immutable style
> (`reference_programs/lexer/main.solr`, recursion + spread) is still valid code,
> but no longer the idiom.
> Where this file conflicts with `docs/design.md`, this file wins **on this branch
> only**. `docs/design.md` §2–§4 (bare `try` panics, `Maybe`, `new`/pointers) are
> stale with respect to it.

Markers:
- **[decided]** agreed.
- **[working]** the current working assumption; the reference code depends on it.
- **[provisional]** used in reference code but not agreed.
- **[open]** an unresolved question.

## 1. Core model

In one paragraph:

> Everything is a value, never a reference: assigning, passing, storing, returning
> or capturing gives the receiver its own value. Local variables belong to the
> function that declares them and can be changed freely. Parameters are read-only,
> unless the function declares them `mut T` and the caller writes `mut x` at the call.
> That gives the function exclusive access to the caller's variable for the
> duration of the call. Concurrency happens through `actor fn`s, which only
> receive read-only values.

### Values and locals
- **[decided]** Values are never aliased. `var l2 = l` gives an independent value
  (copy-on-write in the implementation).
- **[decided]** A `var` declared in a scope belongs to that scope and is mutable
  there: you can rebind it, assign fields, `push`, `xs[i] = …`, `+=`. There is
  no separate `mut var`.
- **[decided]** Assigning to a variable from an inner block (loop body, `if`
  branch) updates the outer variable. Blocks and loops are not boundaries;
  functions and closures are.
- **[working]** Closures capture **values**, i.e. snapshots. A closure cannot change
  or rebind a captured variable.
- **[working]** There are no pointers: `*T`, `&x` and `new()` do not exist in this
  variant.

### `mut` parameters
- **[decided]** Parameters are read-only by default. `fn next(lexer: mut Lexer)`
  declares a parameter the function may change. The changes are visible to the caller
  after the call returns (in/out semantics: Swift `inout`, Hylo, Rust `&mut`).
- **[decided]** The call site must mark it: `next(mut lexer)`. Without `mut`, the
  call is a compile error.
- **[decided]** Only a value the current scope owns can be passed as `mut`:
  - a local `var`
  - a field or element of one (`mut state.items`)
  - the function's own `mut` parameter

  A read-only parameter can't be passed as `mut`; copy it first (`var l2 = l;`).
- **[working] Exclusivity:** while a `mut` access is active, nothing else may access
  the same value. `f(mut a, mut a)`, `f(mut a, a)` and
  `merge(mut shelf, mut shelf.books[0])` are compile errors.
- **[working] Non-escaping:** a `mut` parameter can't be stored, can't be captured by
  a closure that outlives the call, and can't be sent to an actor. The access
  ends when the call returns.
- **[working]** Terminology: this is exclusive access for the duration of a call
  (a borrow), not an ownership transfer. The caller keeps the value afterwards.

### Methods
- **[decided]** `fn m(self)` is read-only. `fn m(mut self)` may change the receiver.
- **[working]** A `mut self` method can only be called on a value that could be
  passed as `mut` (see above).
- **[open]** The call-site marker for `mut self` calls: `mut lexer.advance()` (as in
  v0, explicit but noisy) or unmarked `lexer.advance()` (as in lexer2).

### Concurrency
- **[provisional]** `actor fn` declares a function that may run concurrently.
  Its parameters can't be `mut`.
- **[working]** Arguments to an actor are logical copies (copy-on-write), not
  references, so the caller can keep changing its own variable without a data race.
- **[open]** How actors are called and how results come back: future/task +
  `await`, or mailbox replies.

### Shared state
- **[open]** `Ref[T]` (a shared slot that is replaced atomically) vs. a state-owning
  actor. With `mut` parameters, single-threaded code rarely needs either. The Elm
  architecture (`Msg` + `update`) remains a valid pattern, but it's no longer required.

## 2. Runtime guarantees

- **[working] Copy-on-write values.** Assigning, passing and capturing share storage
  until someone writes, so `var l2 = l` and actor arguments are cheap until changed.
- **[working] In-place updates when unique.** `[..xs, x]`, `append`, `.with(…)`,
  `Type(..v, f=x)` and string `+=` reuse storage when the old value isn't used
  again (its last use).
- **[working] Slices and string slices are views** sharing storage with the original.
- **[provisional] Self tail calls are compiled to loops.** The recursive style
  depends on this. Mutual recursion is not guaranteed (Go target).
  Tail position includes:
  - the expression of a `return`
  - the value of a `switch`/`if` in tail position
  - a trailing `>>` in such an arm or block

## 3. Types and objects

- **[working]** Objects hold data, and `impl` blocks hold methods: read-only
  (`self`) or changing (`mut self`).
- **[working]** Interfaces are **structural** (Go-style): a type satisfies an
  interface by having the methods. There is no `impl Interface for Type`, and the
  consumer declares the interface.
- **[open]** Whether an interface method can require `mut self`, and what
  satisfying such an interface means for a read-only value.
- **[working]** `Self` in an interface means the implementing type.
- **[working]** Type parameters can be constrained: `fn f[T, E, S: Iterable[E]](…)`.
  `TypeParameter` needs a `Constraint` field in the AST.
- **[decided]** `.with(field=…)` is a builtin on objects and enum members. Spread
  construction is `Type(..v, field=x)`.
- **[open]** Should setting fields through `.with`/spread require the same
  visibility as the field, so that invariants can't be bypassed?
- **[decided]** Tuples are required: fixed size, immutable, written `(a, b)`. The
  type syntax, element access and destructuring forms are open (todo #8).
- **[decided]** Indexing a `str` yields a one-character `str`. Bytes are a separate,
  not yet designed type.

## 4. Errors

- **[decided]** `Maybe` / `Some` / `None` are removed. A lookup that may find
  nothing returns `Result[T]`.
- **[decided]** `try expr` unwraps an `Ok`; on `Err` the enclosing function
  returns the `Err`. It is a checker error if that function doesn't return a
  `Result`.
- **[decided]** `try expr else v` and `try expr else |e| { …; >> v; }` handle the
  error locally.
- **[decided]** `must expr` unwraps or panics. Using `must` on a non-`Result` is a
  compile error.
- **[decided]** `xs[i]` returns `T` and panics when out of range. `xs.get(i)`
  returns `Result[T]`.
- **[provisional]** `T!` is shorthand for `Result[T]`.

## 5. Blocks and `>>`

- **[decided]** `>>` yields the value of the **innermost** enclosing block.
  `if c { >> x; }` used as a statement throws `x` away and doesn't yield from the
  outer block, which is easy to get wrong (lexer2 line 58).
- **[open]** Should the checker warn when a `>>` value is discarded?

## 6. Collections and iteration

- **[decided]** Both changing and non-changing operations exist:
  - **New value (works anywhere):** `append`, `prepend`, `with`, `set`, `delete`,
    `sorted`, spread.
  - **In place (owned variables only):** `push`, `xs[i] = …`, `m[k] = …`, `sort`.
    Allowed on a local `var` or a `mut` parameter.
- **[provisional]** Builtin sequences satisfy `Iterable[T]`, and sequence syntax
  is defined by its methods:

  ```
  type Iterable[T] interface {
      fn len(self) -> int
      fn take(self) -> (T, Self)!      // var [head, ..tail] = seq
      fn pop(self) -> (Self, T)!       // var [..init, last] = seq
      fn prepend(self, x: T) -> Self   // [x, ..seq]
      fn append(self, x: T) -> Self    // [..seq, x]
      fn contains(self, x: T) -> bool
  }
  ```
- **[provisional]** The builtin `len(x)` calls `x.len()`.
- **[open]** Iteration over user types: a pure step function, push iterators
  (`fn each(self, yield: fn(T) -> bool)`), a `mut self` `next()` (now possible), or
  generators via `>>`.
- **[decided]** A `for` used as a value collects each `>>` into a slice (a
  comprehension).

## 7. Syntax used by the reference programs

| syntax | example | status |
|---|---|---|
| `mut` parameter | `fn next(lexer: mut Lexer)` | [decided] |
| `mut` argument | `next(mut lexer)` | [decided] |
| `mut self` | `fn advance(mut self)` | [decided] |
| `actor fn` | `actor fn asyncLex(lexer: Lexer) -> []Token` | [provisional] |
| subject-less switch | `switch { isDigit(ch) => …, _ => … }` | [provisional] (parser: `switch {` means no subject) |
| spread in sequences | `[..tokens, tok]` | [provisional] |
| slice destructuring | `var [head, ..tail] = seq` | [provisional] |
| tuple destructuring | `var (a, b) = f()` / `a, var b = f()` | [open] (todo #8) |
| expression-bodied lambda | `fn(ch: str) -> bool: isDigit(ch)` | [provisional] |
| top-level `const` | `const wsChars: []str = […];` | [provisional]; how it differs from a top-level `var` is open |
| `mut var` | — | removed: plain `var` locals are mutable |
| `T?` | — | removed together with `Maybe` |

Parser impact: a `mut` prefix in parameter types (`Param.Mut`) and receivers, a
prefix `MUT` in argument position (`MutArg`/flag on `Argument`), the `MUST` and
`ACTOR` keywords, and subject-less `switch`.

## 8. Known costs of this variant

- **Cyclic or back-referencing graphs** need IDs/indexes into a collection (arenas).
- **Exclusivity errors** (`f(mut a, a)`) are a new class of compile error to learn.
- **`mut` markers** at call sites add noise. The friction v0 had with this needs
  re-checking against real code.
- **Shared state across actors** needs a decision (§1, Shared state).
- **Performance depends on uniqueness:** an extra reference to an old value
  turns an in-place update into a copy (§2).
