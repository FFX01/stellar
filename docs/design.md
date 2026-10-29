# Stellar — Language Design

> The design is **derived from the reference program** `reference_programs/todo-cli/main.st`.
> Every decision here is a rule we've agreed on; the reference code is edited to
> match each decision as we go. Anything marked "(provisional)" is not yet agreed.

Source of truth for this iteration: `reference_programs/todo-cli/main.st`.

## Status

- **In progress.** Design is being derived reference-by-reference.

- Resolved questions marked **[resolved]**. Open ones live in `docs/status.md`.

- **Branch `immutability`:** the language reference is `docs/language-spec.md`. See also
  `docs/value-semantics.md`. It overrides §2–§4
  of this file on that branch (`try` / `must`, no pointers or `new`; immutable values
  with local `mut` variables).

## Lexing [resolved]

- Primitive/builtin types (`int`, `str`, `bool`, `float`, …) are **not keywords** and get **no special token type** — they are lexed as plain `IDENT`, exactly like user-defined types (`Todo`, `APIError`). This keeps builtins and user types uniform at the lexer level.
- Builtin *types* are resolved by the **checker** in type position, via a pre-seeded **type namespace** that is separate from the **value namespace** (so `int` as a type and `int` as a variable do not collide).
- Keywords reserved by the lexer: `fn`, `type`, `pub`, `return`, `if`, `else`, `match`, `for`, `loop`, `in`, `new`, `try`, `as`, `impl`, `interface`, `enum`, `object`, `implements`, and operators/punctuation (`->`, `=>`, `|`, `[`, `]`, `(`, `)`, `+`, `==`, `!=`, `<=`, `>=`, `!`, …).
- Reserved-name polish (e.g. forbidding `var int = 5`) is deferred to the checker if desired.
- Casing convention (#100, lowercase = primitive/value, PascalCase = type) is a *checker enforcement* rule, not a lexer token distinction.

## 1. Functions & return types [resolved]

- Return type uses `->` only. `=>` is **match-arm** syntax, never a return annotation.
- A bare `Result` (no type args) is sugar for `Result[void]` — i.e. the function
  returns either `Ok(void)` or `Err(Error)`.
- `Result[T]` is the success-type-parameterized form.

## 2. Error handling [resolved]

- `Ok()` is the canonical success constructor (lowercase `ok` was a typo).
- `Error` is a **builtin interface**. A user type `type X implements Error` is
  syntactic sugar that auto-implements the `Error` interface (exact mechanism TBD,
  see status #error).
- `Error("msg")` constructs a generic builtin error.
- `wrap(e, NewErr)` returns `Err(Error)`: the original error `e` is wrapped by the
  new error, so the wrapped error becomes the function's returned error.
- `try expr` unwraps `Ok()/Some()`; bare `try expr` **panics** on failure.
- `try expr else |e| { … }` handles the error by binding it to `e`.
- `try expr else { … }` (or `else value`) provides a default for a non-error channel.

## 3. `new` and value/pointer semantics [resolved]

- `new T` **always** creates a pointer to a **zero value** of `T`; it cannot
  initialize fields.
- Passing a **concrete value** passes a **copy**: mutations affect only the copy.
- Passing a **pointer** (`*T`) shares the original: mutations persist to the caller.
- Receiver mode (`self: T` vs `self: *T`) is how a method declares whether it may
  mutate its receiver.

## 4. Result & Maybe as built-in generic enums [resolved]

- `Result[T]` and `Maybe[T]` are **built-in generic enums** (sum types / tagged unions),
  structurally:
  ```
  type Result[T] enum { Ok(value: T); Err(error: Error) }
  type Maybe[T] enum { Some(value: T); None }
  ```
- They are **not user-writable stdlib source** — they are pre-seeded in the checker's
  type namespace with this exact shape. Reason: `try` unwrapping (`try expr` early-returns
  on `Err`/`None`, `try … else |e|` / `else value` handle it) must be tied to these
  specific names. The checker grants the `Ok`/`Err`/`Some`/`None` constructor sugar and
  the `try` protocol because it knows those names — a user-defined enum is *not* try-able.
- The **enum mechanism itself is fully general**: user types can define enums, and enums
  support **generic type parameters** (needed for `Result[T]`, `Maybe[T]`, `Map[K,V]`).
  Result/Maybe are just the built-in instances.
- `try` is currently tied to Result/Maybe only (no `Tryable` protocol yet). Generalize
  later if user types need to be try-able.
- `Result[void]` = bare `Result` (section 1): uses `Ok()` with no payload; `Maybe[void]`
  uses `Some()`. Payload and void constructors are uniform across enums.
- **Still open (§7 in status):** Result vs Maybe as *channels* — whether indexing /
  lookup-miss returns `Maybe` (not found = expected) or `Result` (out of range = error).
  Deciding the type shape doesn't resolve which channel each operation uses.

## 5. Boolean negation & inequality [resolved]

- `!` is the **`NOT`** operator — prefix boolean negation (`!someCondition`).
- `!=` is the **`NEQ`** (not-equal) operator — a comparison operator, sibling of `==`/`<`/`>`.
  It shares the `!` glyph with `NOT` but is a distinct token (different arity/precedence);
  do not treat `!=` as a form of `NOT`.

## 6. Type-constructor keywords [resolved]

- `object`, `enum`, `interface`, `implements` are keywords. They are **type constructors**
  (metatypes — they build types), not types, so the lexer must recognize them to parse
  what *kind* of type is being declared. Same rationale as `fn`/`type`.

## 7. Numeric types and operators [resolved]

- There are two numeric types, `int` and `float`. `float` is 64-bit. `int` is 64-bit where
  the platform supports it, and 32-bit otherwise. There are no sized variants.
- **No implicit numeric coercion.** An operator whose operands are one `int` and one
  `float` is a type error, for every operator including `/`. Both operands must have the
  same type. Example: `1 + 1.2` and `7.0 / 2` are errors; write `7.0 / 2.0`.
  How to convert between `int` and `float` is not yet decided.
- `int / int` is **floor division** (rounds toward negative infinity): `-7 / 2 == -4`.
  The result type is `int`.
- `float / float` is true division. The result type is `float`.
- For `int`s, `%` takes the sign of the divisor, so `a / b * b + a % b == a` holds.
  This follows from floor division.
- Conversions are builtin functions: `int(x)`, `float(x)`, `str(x)`. They are the only way
  to mix numeric types, since there is no implicit coercion.
- The checker resolves the argument's type, so each conversion is selected at check time.
  No runtime type dispatch is needed.
- **TBD:** what each conversion accepts and how it fails. For example: does `int(3.7)`
  truncate or floor? Does `int("42")` parse, and if so does it return `int!`? Is `str(true)`
  allowed?

## 8. Operator typing rules [resolved]

- `+` on two `str` operands is **concatenation**, result `str`.
- `==` and `!=` require both operands to have the same type. Different types are a
  **compile error**, not `false`.
- Ordering comparisons (`<`, `<=`, `>`, `>=`) are allowed on `str`, as well as on
  `int` and `float`. Result `bool`. The ordering of `str` values (e.g. byte-wise or
  Unicode code point) is **TBD**.
- Operand types must match exactly for every operator (see section 7).

## Other inferred constructs (provisional, not yet agreed)

- Imports: `import std::x` (double-colon module path).
- `Error` stores its message (storage model TBD).
- Interfaces: `type Name interface { fn ... }`; `impl Intf for T { ... }`.
- Enums: `pub type S enum { Variant | DataVar(payload) }`.
- Objects: `pub type T object { field: type }`.
- `pub` = exported type.
- String interpolation: `f"{expr}"`.
- Builtins: `len`, `append`, `print`, `os.exit`.
- Maybe: `Maybe[T]`, `Some`, `None`.
- Control: `loop`, `continue`, `for x in coll`, `match` with `as a` binding + `_` default.

## Contents (to be filled)

- Treated as we iterate. See `docs/status.md` for active questions.
