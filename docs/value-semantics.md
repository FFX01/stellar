# Stellar — Immutable Values with Local Mutability (branch `immutability`)

> Exploration spec for branch `immutability`. Direction as of 2026-10-27: **values are
> immutable and behave as copies; mutation exists only inside a single function, through
> variables declared `mut`.** This replaces the earlier `mut`-parameter model
> (`reference_programs/lexer2`) and the earlier fully functional style.
> **The authoritative language reference is now `docs/language-spec.md`.** This file
> records how the mutability decisions were reached, and lists their open questions.
> Where this file conflicts with `docs/design.md`, this file wins **on this branch
> only**. `docs/design.md` §2–§4 (bare `try` panics, `new`/pointers) are stale with
> respect to it.

Markers:
- **[decided]** agreed.
- **[working]** the current working assumption; the reference code depends on it.
- **[provisional]** used in reference code but not agreed.
- **[open]** an unresolved question.

## 1. Core model

> Every value is immutable. Assigning, passing, or storing a value gives the receiver
> its own value, so no one else can change what you hold. Inside a function you may
> declare a variable with `mut`, and change its contents in place. That mutation is
> visible only inside that function. A value that leaves a function (returned, passed,
> or captured) is immutable again. A function that wants to "change" something it was
> given copies it first with `@`, changes the copy, and returns it.

### Bindings
- **[decided]** `var x = e;` declares an immutable binding. Its contents can't be
  changed, but the variable can be **rebound**: `x = e2;`. Example:
  `var s = State(todos=[]); s = update(s, msg);`
- **[decided]** `mut x = e;` declares a variable whose contents can be **changed in
  place**: field assignment (`x.done = true`), `x.items.push(t)`, `x[i] = v`, `+=`.
  Rebinding a `mut` variable is also allowed.
- **[decided]** A `mut` variable is mutable only inside the function (including its
  inner blocks and loops) that declares it. Blocks are not boundaries; functions and
  closures are.
- **[decided]** Rebinding from an inner block (loop body, `if` branch) updates the
  outer variable.
- **[decided]** Parameters are never mutable. `fn f(x: mut T)` is illegal. There is no
  `mut self`.
- **[decided]** Mutating through an immutable binding (`second.done = true` where
  `second` was declared with `var`) is a compile error.

### Copies
- **[decided]** Assigning, passing, and storing a value are value copies:
  `var second = first;`. The copy is semantic. Memory is shared until someone writes,
  and then only the changed part is copied (structural sharing).
- **[decided]** A returned value is a concrete value, not a reference. It lands in the
  caller as an immutable value, even when the caller binds it to a `mut` variable.
  `mut s = State(...); s = update(s, msg);` rebinds `s`. It doesn't mutate.
- **[decided]** `@x` is the **copy operator**: `mut myTodo = @todo;` gives a copy that
  can be changed in this scope. It is sugar for `copy(todo)`. `@` is required when the
  result is bound to a `mut` variable: `mut x = y;` is a compile error. Implicit forms
  such as `@todo.name = name` (a hidden mutable copy) are not part of the language.
- **[decided]** A loop binding (`for _, t in xs`) is a copy of the element, immutable.
- **[decided]** `h.saved = l` stores a copy of `l`. Nothing is shared.

### Updates without `mut`
- **[decided]** `first.with(done=true)` and `Todo(...first, done=true)` build a new value
  with some fields replaced. They mean the same thing.
- **[open]** Spread token: `...` (in the design notes) or `..` (in `reference_programs`).
- **[open]** Named constructor arguments: `Todo(id=1)` (decided in #129) or `Todo(id: 1)`
  (used in some examples).

### Closures
- **[decided]** A closure reads its parent's variables through a **copy** taken when the
  closure is created. It can't change or rebind a parent variable, `mut` or not. This
  avoids dangling references when a closure outlives its scope.

### Concurrency
- **[provisional]** Values that cross to another actor are immutable, so the actor gets
  a snapshot. An actor that wants to change one copies it, and returns its copy or sends a
  message. Shared mutable state does not exist.
- **[open]** `actor fn` syntax, and how actors are called and how results come back.
- **[open]** A `ref` escape hatch for shared mutable state, possibly later. It would be
  explicit and discouraged.

## 2. Runtime assumptions

- **[working] Structural sharing.** Copies share memory. Changing a part copies only
  what changed.
- **[working] In-place update of `mut` variables when unshared.** `push` and field
  assignment on a `mut` local don't copy if nothing else holds the same storage.
- **[open] Persistent trees vs. copy-on-write arrays.** Persistent trees give O(log n)
  updates and keep old versions valid, but indexing is slower. Copy-on-write arrays
  update in O(1) when unshared and copy in O(n) otherwise, and need a way to know
  whether storage is shared. A `mut` local behaves like a Clojure transient.
- **[open] Garbage collector.** A generational collector that rarely scans old data is
  expected to work well for this model. It assumes Stellar controls the runtime, which
  conflicts with a Go transpilation target. The compile target is still undecided (#141).
- **[provisional] Self tail calls are compiled to loops.** Recursive style depends on
  it. Mutual recursion is not guaranteed (Go target).
  Tail position includes:
  - the expression of a `return`
  - the value of a `switch`/`if` in tail position
  - a trailing `>>` in such an arm or block
- **[working] Slices and string slices are views** sharing storage with the original.

## 3. Types and structs

- **[working]** Structs hold data, and `impl` blocks hold methods. A method takes
  `self` and can't change it. A method that "changes" a value returns a new one.
- **[working]** Interfaces are **structural** (Go-style): a type satisfies an
  interface by having the methods. There is no `impl Interface for Type`, and the
  consumer declares the interface.
- **[working]** `Self` in an interface means the implementing type.
- **[working]** Type parameters can be constrained: `fn f[T, E, S: Iterable[E]](…)`.
  `TypeParameter` needs a `Constraint` field in the AST.
- **[open]** Should `.with`/spread require the same visibility as the field, so that
  invariants can't be bypassed?
- **[decided]** Tuples are required: fixed size, immutable, written `(a, b)`. The
  type syntax, element access and destructuring forms are open (todo #8).
- **[decided]** Indexing a `str` yields a one-character `str`.

### `void`, absence, and zero values
- **[decided]** `void` is a **unit type**: it means "nothing", and is the return type of a
  function with no `-> T`. It is not a null that other types can hold.
- **[decided]** `Maybe[T]` exists, with shorthand `T?`. Its variants are `Some(x)` and
  `None()`. A field that can truly be absent is declared as a `Maybe`:
  `item: Item?`, and `State()` leaves it as `None()`. This reverses the earlier removal
  of `Maybe`.
- **[decided]** Primitives and built-in containers have real zero values: `0`, `0.0`,
  `""`, `false`, and the empty slice or map.
- **[working]** Omitting an argument when constructing a struct: a field of primitive
  or container type takes its zero value. A field of a struct type that isn't a
  `Maybe` must be given, so `State(item=Item())` is valid and `State()` is not when
  `item: Item`. Per-field defaults may be added later.
- **[open]** Which kinds of absence use `Maybe` and which use `Result`. `xs.get(i)` is
  `Result[T]` under §4. A lookup with no failure reason, such as a map key, may be
  better as `Maybe`.

## 4. Errors

- **[decided]** `try expr` unwraps an `Ok`; on `Err` the enclosing function
  returns the `Err`. It is a checker error if that function doesn't return a
  `Result`.
- **[decided]** `try expr else v` and `try expr else |e| { …; >> v; }` handle the
  error locally.
- **[decided]** `must expr` unwraps or panics. Using `must` on a non-`Result` is a
  compile error.
- **[decided]** `xs[i]` returns `T` and panics when out of range. `xs.get(i)`
  returns `Result[T]`.
- **[provisional]** `T!` is shorthand for `Result[T]`, and `void!` is a result with no
  value.

## 5. Blocks and `>>`

- **[decided]** `>>` yields the value of the **innermost** enclosing block.
  `if c { >> x; }` used as a statement throws `x` away and doesn't yield from the
  outer block, which is easy to get wrong.
- **[open]** Should the checker warn when a `>>` value is discarded?

## 6. Collections and iteration

- **[decided]** Operations that return a new value work on any binding: `append`,
  `prepend`, `with`, `set`, `delete`, `sorted`, spread. Operations that change in place
  need a `mut` variable: `push`, `xs[i] = …`, `m[k] = …`, `sort`. `append` returns a new
  list. `push` changes the list.
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
- **[open]** `pop` on a `mut` list: the in-place form (changes the list, returns the
  element) or the `Iterable` form above (returns both).
- **[provisional]** The builtin `len(x)` calls `x.len()`.
- **[open]** Iteration over user types: a pure step function, push iterators, or
  generators via `>>`.
- **[decided]** A `for` used as a value collects each `>>` into a slice (a
  comprehension).

## 7. Syntax used by the reference programs

| syntax | example | status |
|---|---|---|
| `var` binding, rebindable | `var s = State(todos=[]); s = update(s, m);` | [decided] |
| `mut` binding | `mut result: []Todo = [];` | [decided] |
| copy operator | `mut t = @todo;` | [decided] |
| `.with` / spread | `first.with(done=true)`, `Todo(...first, done=true)` | [decided] (spread token open) |
| `T?` | `item: Item?` | [decided] |
| `T!` | `-> State!` | [provisional] |
| `actor fn` | `actor fn asyncLex(lexer: Lexer) -> []Token` | [provisional] |
| subject-less switch | `switch { isDigit(ch) => …, _ => … }` | [provisional] |
| slice destructuring | `var [head, ..tail] = seq` | [provisional] |
| tuple destructuring | `var (a, b) = f()` | [open] (todo #8) |
| expression-bodied lambda | `fn(ch: str) -> bool: isDigit(ch)` | [provisional] |
| top-level `const` | `const wsChars: []str = […];` | [provisional] |
| enum payload extraction in `switch` | `is Add => action.title` | [open] |

Removed from earlier drafts: `mut` parameters (`x: mut T`), `mut self`, `@x` as a call
argument marker, exclusivity and non-escaping rules, `var mine = @x` as a plain copy
(it is now `mut mine = @x`).

**Parser impact:**
- **Add:** a `mut` declaration keyword (`VarStmt` needs a `Mutable` flag), the `?` type
  suffix, the spread token, the `MUST` handling already present.
- **Remove:** `MutableType` and `parseMutableType` (parameters can't be `mut`).
- **Reuse:** `@` stays as a prefix operator. `GrantExpr` now means "copy".

## 8. Known costs of this model

- **Mutation can't cross a function call.** A helper that "changes" something returns the
  new value, so state-like code uses rebinding (`s = update(s, msg)`) or tuple returns.
  This is the pattern described in the exploration notes (#158, #162).
- **Cyclic or back-referencing graphs** need IDs/indexes into a collection (arenas).
- **I/O handles** (a database, an HTTP client) can't be mutable shared objects. They need
  a context parameter, package-level read-only handles, or an actor that owns them.
- **Performance depends on sharing:** keeping an extra reference to an old value turns
  an in-place update into a copy (§2).
