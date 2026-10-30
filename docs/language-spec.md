# The Stellar Language Specification

> This is the reference for the Stellar language: its syntax, its type system, and its
> rules for values and mutability. It describes the language as decided on branch
> `immutability` as of 2026-10-28.
>
> **Authority.** Where this document disagrees with `docs/design.md` or
> `docs/value-semantics.md`, this document wins. Those two files are kept as the record of
> how each decision was reached. `docs/analyzer-plan.md` describes how the checker is built.
>
> **How to read the markers.** Every rule that isn't settled is marked.
>
> | Marker | Meaning |
> |---|---|
> | *(no marker)* | Decided. |
> | **[provisional]** | Used in the reference programs, but not formally agreed. |
> | **[open]** | An unresolved question. Don't rely on the current wording. |
> | **[not implemented]** | Decided, but the lexer or parser doesn't handle it yet (§15). |
>
> Source files use the extension `.solr`. Reference programs live in `reference_programs/`.

## Contents
1. Principles
2. Lexical structure
3. Types
4. Declarations
5. Values, mutability, and copying
6. Expressions
7. Statements
8. Errors and absence
9. Operators and conversions
10. Built-in functions and methods
11. Modules and visibility
12. Concurrency
13. Complete grammar
14. Example programs
15. Implementation status
16. Open questions
17. To do: planned features

---

## 1. Principles

- **General purpose and garbage collected.** There is no manual memory management.
- **Values are immutable.** Assigning, passing, or storing a value gives the receiver its
  own value. Nobody can change a value somebody else holds.
- **Mutation is local.** Inside one function you can declare a `mut` variable and change it
  in place. The change is visible only inside that function.
- **No null, no exceptions.** A value that might be missing is a `Maybe`. An operation that
  might fail returns a `Result`. The language panics only on programmer errors (`must`,
  out-of-range indexing).
- **No implicit conversions.** `1 + 1.0` is a compile error.
- **Explicit mutability.** A reader can tell from the declaration whether something can
  change (`var` or `mut`), and where a copy is made (`@`).
- **Structural interfaces.** A type satisfies an interface by having its methods.
- **Phases.** Lexer, parser, checker, compiler. The parser records what was written, and
  the checker enforces the rules in this document.

---

## 2. Lexical structure

### 2.1 Source text and comments
- A source file is text. Identifiers and keywords are ASCII. The lexer currently reads bytes,
  so non-ASCII characters are only safe inside strings and comments (see §17).
- A comment starts with `//` and ends at the end of the line. There are no block comments.
- Whitespace (space, tab, newline) separates tokens and is otherwise insignificant. Line
  breaks do **not** end a statement (see §7.1).

### 2.2 Identifiers
An identifier starts with an ASCII letter or `_`, followed by letters, digits, or `_`.
`_` alone is an identifier, used by convention for "ignore this".

Built-in type names such as `int`, `str`, `bool`, `float`, and `void` are **not keywords**.
They are ordinary identifiers that the checker resolves in type position. **Built-in names
can't be redeclared or shadowed anywhere**: not as a type, function, variable, parameter, or
loop binding. This covers the built-in types, `Error`, and the built-in functions (`len`).
Struct fields and enum variants aren't declarations in a scope, so a field may be named `str`.

Naming convention **[provisional]**: built-in primitive types are lower case (`int`, `str`).
Every other type, built-in or user-defined, is `PascalCase` (`Result`, `Maybe`, `Error`,
`Todo`).

### 2.3 Keywords
```
if  else  switch  for  in  break  continue  return
var  mut  fn  type  impl  struct  enum  interface  pub  import
try  must  and  or  true  false  Ok  Err  Some  None
```
`Ok` and `Err` are keywords because they are constructors of the built-in `Result` type, and
`Some` and `None` because they are constructors of `Maybe`.
**[provisional]** `actor` is planned as a keyword for concurrency (§12). Today it is an ordinary
identifier, so a program can use it as a name.

### 2.4 Literals
| Literal | Examples | Notes |
|---|---|---|
| Integer | `0`, `42` | Decimal digits only. Other bases and `_` separators are planned (§17). |
| Float | `1.5`, `0.25` | Digits, `.`, digits. `1.` and `.5` are errors. |
| String | `"hello"` | On one line. The lexer decodes escape sequences, so the token holds the decoded text: `\n`, `\r`, `\t`, `\"`, `\\`. The full escape set is that of Go string literals **[provisional]**. An invalid escape, or a string that isn't closed on its line, is a lexer error. |
| Bool | `true`, `false` | |
| Sequence | `[1, 2, 3]`, `[]` | Type comes from context or from its elements (§3.3). |
| Map | `["a": 1]`, `[:]` | **[open]** (a `Map` type is not specified yet). |
| Tuple | `(1, "a")` | **[open]** syntax (§3.6, §17). |

String interpolation and format strings are **[open]**.

### 2.5 Operators and punctuation
```
+  -  *  /  %                    arithmetic
==  !=  <  <=  >  >=            comparison
and  or  !                      logical
=  +=  -=  *=  /=  %=           assignment
.  ::  :  ,  ;                  member, module path, slice/annotation, separators
->  =>  >>                      return type, switch arm, yield
@  |  ( )  [ ]  { }             copy, try-binding, grouping
...                              spread
?                                Maybe suffix in types (`T?`)
```
`!` is logical not as a prefix and the result suffix in types (`T!`). `:` is a type
annotation in declarations, and a range inside `[ ]` (§6.7).

**Maximal munch.** The lexer always takes the longest token. So `int!=Ok(1)`, with no space,
lexes as `int`, `!=`, `Ok(1)`, not as `int`, `!`, `=`. The parser resolves this by context:
after a type, no rule accepts `!=`, so it reads the token as `!` followed by `=`.
Writing `int! = Ok(1)` works too. `T?` has no such problem, because
`?` is not part of any longer token.

---

## 3. Types

### 3.1 Built-in types
| Type | Values | Zero value |
|---|---|---|
| `int` | whole numbers, 64-bit where the platform supports it, otherwise 32-bit | `0` |
| `float` | 64-bit floating point | `0.0` |
| `str` | immutable text | `""` |
| `bool` | `true`, `false` | `false` |
| `void` | the unit type: "nothing". The return type of a function with no `-> T`. Not a null: no other type can hold it. | — |
| `[]T` | slice of `T`, variable length | the empty slice |
| `Result[T]` / `T!` | success with a `T`, or an error | none |
| `Maybe[T]` / `T?` | `Some(x)` or `None()` | `None()` |
| `fn(A, B) -> R` | function | none |
| `Error` | built-in interface for error values | none |

There are no sized integers or floats. A program that needs another width builds it.

### 3.2 Named types
A type is introduced with `type`. After the name, and an optional type-parameter list, one of
four forms is allowed:

```solr
type Point struct { pub x: int, pub y: int }          // a record
type Dir enum { North, South, East, West }            // enum (a tagged union)
type Named interface { fn name(self) -> str }         // interface
type Handler = fn(int) -> bool;                       // alias
```

There is no "new distinct type" form: `type UserID int` is an error. An alias (`=`) names the
same type. Type names are `PascalCase` **[provisional]**.

### 3.3 Slices
`[]T` is a growable sequence. Operations are in §10. A sequence literal `[a, b, c]` has the
type `[]T` where `T` is the type of its elements, or the declared type if there is one.
`[]` needs an expected type: `var xs: []int = [];`.

Fixed-length arrays `[N]T` are **[not implemented]**. Their length is an integer literal
and is part of the type. They are **[provisional]**.

### 3.4 Structs
A struct is a record of named fields.
```solr
type Todo struct {
    pub id: int,
    pub title: str,
    status: Status        // private field
}
```
- Fields are separated by commas. A trailing comma is allowed.
- `pub` makes a field visible outside the module (§11).
- **Construction** uses the type name as a function, with **named** arguments:
  `Todo(id=1, title="x", status=Status.Open)`. Positional arguments are an error.
- **Omitted fields [working]:** a field of primitive or container type may be omitted and takes
  its zero value. A `Maybe` field may be omitted and is `None()`. A field of any other type
  must be given. Per-field defaults may be added later.
- **Updated copies:** `todo.with(title="y")` returns a copy with the listed fields replaced.
  `Todo(...todo, title="y")` means the same **[provisional]** (the spread token is `...`).
- Fields are read with `.`: `todo.title`. Assigning to a field needs a `mut` variable (§5).

### 3.5 Enums
An enum is a tagged union. Each variant may carry named fields.
```solr
type Action enum {
    Add(title: str),
    Complete(id: int),
    Quit
}
```
- Variants are listed with commas, and a trailing comma is allowed.
- **Construction:** `Action.Add(title="x")`, `Action.Quit`. Fields are named, and positional
  arguments are an error.
- A variant is read through `switch` (§6.10).
- `Result` and `Maybe` are built-in enums with special support in the checker (§8).

### 3.6 Tuples
Tuples are fixed-size, immutable, and may hold different types. The type syntax (`(A, B)` or
`tuple[A, B]`), element access, and destructuring are **[open]**. They are **[not implemented]**.

### 3.7 Interfaces
```solr
type Shape interface {
    fn area(self) -> float
    fn name(self) -> str
}
```
- An interface is a list of method signatures, with no bodies and no separators.
- **Structural:** a type satisfies an interface by having methods with these names and
  signatures. There is no `implements` declaration, and the consumer declares the interface.
- Interface methods are always public, so `pub` is not allowed on them.
- Every interface method takes `self`, written without a type (§4.3).
- There is no `Self` type. A method that needs "the same type as the receiver" for another
  parameter or the result (equality, ordering) belongs in a generic interface, such as
  `type Eq[T] interface { fn equals(self, other: T) -> bool }`, which `Rect` satisfies as
  `Eq[Rect]`. Generic interfaces are part of the generics work (§3.8).
- Whether the checker also requires `pub` on the implementing methods is **[open]**.

### 3.8 Generics
Type parameters are listed in square brackets after the name:
```solr
type Pair[A, B] struct { pub first: A, pub second: B }
fn first[T](xs: []T) -> T!: xs.get(0);
```
- They apply to structs, enums, interfaces, aliases, and functions.
- Type arguments at a call are **inferred**. There is no explicit-argument syntax in
  expressions, because `[` after an expression always means indexing.
- Constraints: `[T, E, S: Iterable[E]]` **[provisional]**.
- Checking generic code is **[not implemented]** in the checker's first version.

### 3.9 Function types
`fn(int, str) -> bool` is the type of a function taking an `int` and a `str` and returning a
`bool`. Parameter names are optional in a function type, and ignored when types are compared.
A function with no `-> T` returns `void`. Functions are first-class: they can be passed,
stored, and returned.

### 3.10 Type equality
Two types are the same type when:
- both are the same built-in type;
- both are slices, results, or maybes of the same element type;
- both are function types with the same parameter types and return type;
- both are the **same declaration** (a struct, enum, or interface is identified by its
  declaration, not by its shape).

---

## 4. Declarations

### 4.1 Top level
A file is a sequence of: `import`, `fn`, `type`, `impl`, and `var`. `pub` may prefix `fn`,
`type`, and `var`. Declaration order doesn't matter at the top level: a function can call one
defined below it, and a type can refer to one defined below it.

A top-level `var` is an immutable binding that functions can read, never change or rebind.
Top-level `mut` is an error.

### 4.2 Functions
```solr
fn pick[T](a: T, b: T, first: bool) -> T {
    if first { return a; }
    return b;
}
pub fn inc(x: int) -> int: x + 1;       // expression body
```
- **Parameter types are required**, and so is the return type unless the function returns
  `void`. The parser accepts the omission, and the checker reports it.
- Parameters are **never mutable**. `fn f(x: mut T)` is an error.
- A function whose return type isn't `void` must return a value on every path. A block body
  that can reach its closing `}` is a "missing return" error. A `for` with no condition and no
  `break` counts as never finishing (§6.11).
- A function body is a block with `return`, or `: expression;`.
- A function expression (a lambda) is written the same way, with no name:
  `fn(x: int) -> int: x + 1`. Its type comes from its signature.
- **Whether a parameter can be rebound** inside the function (`x = ...`) is **[open]**.

### 4.3 Methods
Methods are defined in `impl` blocks:
```solr
impl Point {
    pub fn add(self, other: Point) -> Point {
        return Point(x=self.x + other.x, y=self.y + other.y);
    }
}
```
- A method's first parameter is `self`, **written without a type**: the `impl` block already
  names the receiver's type, and inside an interface it is the implementing type.
  `self: Point` is an error, and so is `self` anywhere but the first parameter of a method.
  Otherwise `self` behaves like an ordinary parameter. A method **can't change its receiver**. A method that
  "changes" a value returns a new one.
- `impl` can appear anywhere at the top level. Several `impl` blocks for one type are allowed.
- Methods follow the same terminator rule as top-level functions (§7.1). A method with a block
  body needs nothing after its `}`. A method with an expression body ends with `;`:
  ```solr
  impl Point {
      pub fn sum(self) -> int: self.x + self.y;
      pub fn origin(self) -> bool { return self.x == 0 and self.y == 0; }
  }
  ```
- The target may carry type arguments (`impl Pair[A, B] { ... }`). It parses, but what it means
  is **[open]**.
- A method is called with `value.method(args)`.
- Static functions (no `self`) called as `Type.fn(...)` are needed **[provisional]**.
- A method and a field with the same name on one type are an error **[provisional]**.

### 4.4 Variables
```solr
var x = 1;                  // immutable binding, type inferred
var y: float = 2.0;         // with a type annotation
mut total = 0;              // mutable local (§5)
mut xs: []int = [];
```
- An initializer is **required**. `var x: int;` is an error.
- The annotation is optional. When present, the initializer must match it.
- A name can be declared once per scope. An inner scope may shadow an outer name, except a
  built-in name (§2.2). A function's parameters and the top level of its body are one scope,
  so `fn f(x: int) { var x = 1; }` is a redeclaration.
- Names are introduced **after** their initializer, so `var x = x + 1;` refers to an outer `x`.

### 4.5 Imports
`import std::fmt;` imports a module by path. Path elements are separated by `::`. The module's
name is its last element. What a module is (a file or a directory) is **[open]**.

---

## 5. Values, mutability, and copying

This section is the core of the language.

> Every value is immutable. Assigning, passing, or storing a value gives the receiver its own
> value, so nobody else can change what you hold. Inside a function you may declare a variable
> with `mut` and change its contents in place. That change is visible only inside the function.
> A value that leaves the function (returned, passed, or captured) is immutable again. To change
> a value it was given, a function copies it first with `@`, changes the copy, and returns it.

### 5.1 Bindings

| | `var x = e;` | `mut x = e;` |
|---|---|---|
| Rebind: `x = e2;` | allowed | allowed |
| Change contents: `x.f = v`, `x[i] = v`, `x += 1`, `x.push(v)` | **error** | allowed |
| Where it applies | its function | its function only |

- **Rebinding** points the name at a different value. It's always allowed for a variable in
  the current function, and doesn't change any value.
- **Changing contents** modifies a value in place. It needs `mut`.
- A `mut` variable is mutable only inside the function that declares it. Blocks (`if`, `for`,
  `switch` arms) are **not** boundaries. Functions and closures are.
- Rebinding inside an inner block updates the outer variable:
  ```solr
  var s = State(todos=[]);
  for _, cmd in commands {
      s = update(s, cmd);        // updates the outer `s`
  }
  ```
- Parameters, `for` bindings, and top-level `var`s are never mutable. Top-level `mut` is an
  error.

### 5.2 Copies
- Assigning (`var b = a;`), passing (`f(a)`), storing (`h.item = a;`), and binding a loop
  variable (`for _, t in xs`) each give the receiver an independent value.
- This is a **semantic** rule. In memory, copies share storage and only the parts that change
  are copied, when they change.
- A value returned from a function is a concrete value, never a reference. The caller
  receives it as an immutable value, even when it binds the result to a `mut` variable:
  ```solr
  mut s = State(todos=[]);
  s = update(s, msg);               // rebinding. Not a mutation.
  s.todos.push(todo);               // a mutation, in place
  ```
- Since there is no shared mutable state, a value passed to another function (or another
  concurrent actor) is a snapshot. Later changes by the sender don't reach it.

### 5.3 The copy operator `@`
`@x` makes a copy of `x` that the current function can change.
```solr
fn rename(todo: Todo, title: str) -> Todo {
    mut mine = @todo;             // a mutable copy, local to this function
    mine.title = title;
    return mine;                  // the caller receives an immutable value
}
```
- `@x` is valid **only as the initializer of a binding**: `mut t = @x;` or `var t = @x;`.
- `mut t = x;` (no `@`) is an error. A mutable binding must show where the copy is made.
  `var t = @x;` is allowed, though it has no use.
- A `mut` initializer must be a **fresh value**: `@x`, a literal, a sequence literal, a
  call or construction, `Ok`/`Err`/`Some`/`None`, an operator expression, or a function
  literal. `if`, `switch`, and blocks are fresh when every branch is. `try e` and `must e`
  are fresh when `e` is. A variable, a field (`a.b`), an element (`a[i]`), and a **slice**
  (`xs[a:b]`) are not fresh, so `mut t = xs[a:b];` is an error. Write `mut t = @xs[a:b];`.
  `var t = xs[a:b];` is fine.
- There is no implicit form such as `@todo.name = name`.
- The same function can be written without `mut`:
  `return todo.with(title=title);`

### 5.4 Closures
A function expression is its own scope. It reads its parent's variables through a **copy taken
when the closure is created**. It can't change or rebind a parent's variable, whether the
parent declared it `var` or `mut`. This keeps a closure that outlives its parent safe.
```solr
fn counterAdder(base: int) -> fn(int) -> int {
    return fn(n: int) -> int: base + n;     // reads a copy of `base`
}
```
Inside a closure, its own `mut` variables work as usual.

### 5.5 Methods and mutation
A method receives `self` as an immutable value. `todo.complete()` can only return a new
`Todo`. Built-in operations come in two kinds (§10.2): ones that return a new value
(`append`, `with`), and in-place ones that need a `mut` receiver (`push`).

### 5.6 Compile errors, in summary
```solr
fn main() {
    var first = Todo(id=1, title="a", status=Status.Open);
    var second = first;               // a copy
    second.title = "b";               // error: `second` is not mutable; declare it with `mut`

    mut third = first;                // error: use `@` to make a mutable copy
    mut part = xs[1:3];               // error: a slice isn't a fresh value either
    mut fourth = @first;              // ok
    fourth.title = "c";               // ok, changes only `fourth`

    var h = fn() { fourth.title = "y"; };   // error: a closure can't change a parent variable
}

fn f(t: Todo) { t.title = "x"; }      // error: parameters are immutable
fn g(t: mut Todo) { }                 // error: there are no mutable parameters
```

### 5.7 Costs
- Mutation can't cross a function call. A helper that "changes" something returns the new value,
  and the caller rebinds or destructures: `s = update(s, msg);`.
- Shared mutable resources (a database connection, a cache) can't be values. They need a
  context parameter, package-level read-only handles, or a dedicated actor (§12).
- Graph-shaped data (cycles, back-references) uses IDs into a collection.
- A shared mutable reference type (`ref`) may be added later **[open]**. It would be explicit
  and discouraged.

---

## 6. Expressions

Almost everything is an expression: `if`, `switch`, `for`, blocks, function literals, and `try`.

### 6.1 Precedence
From loosest to tightest binding. Binary operators associate to the left.

| Level | Operators |
|---|---|
| 1 | `or` |
| 2 | `and` |
| 3 | `==` `!=` |
| 4 | `<` `<=` `>` `>=` |
| 5 | `+` `-` |
| 6 | `*` `/` `%` |
| 7 | prefix `-` `!` `@` `try` `must` |
| 8 | call `f(x)`, index `a[i]`, slice `a[i:j]`, member `a.b` |

`try` and `must` are prefix operators at the same level as unary `-`. So `try f() + 1` is
`(try f()) + 1`, and `must f().x` is `must (f().x)`, because call and member access bind
tighter than a prefix operator.

The `else` part of `try` is the exception: its value extends as far right as possible, so
`try f() else 0 + 1` is `try f() else (0 + 1)`.

### 6.2 Literals and names
Literals are in §2.4. An identifier names a variable, function, or type. `(e)` groups.

### 6.3 Calls
`f(a, b)`, or with names `f(x=a, y=b)`. Named arguments are required when constructing structs
and enum variants. For ordinary functions, whether names are allowed is **[open]**.
A trailing comma is allowed. A call whose callee is a type is a construction (§3.4) or a
conversion (§9.3).

### 6.4 Member access
`a.b` reads a field, calls a method (`a.b(x)`), or picks an enum variant (`Action.Quit`).
Calling a function stored in a field is written the same way as calling a method **[open]**.

### 6.5 Unary operators
`-x` negates an `int` or `float`. `!b` negates a `bool`. `@x` is the copy operator (§5.3).

### 6.6 Binary operators
Typing rules are in §9.

### 6.7 Indexing and slicing
- `xs[i]` reads an element. An out-of-range index **panics**. `xs.get(i)` returns `T!`.
- `xs[a:b]`, `xs[:b]`, `xs[a:]` take a sub-sequence, and also work on `str`.
  The result is a value, as always. An implementation may share storage with the original.
  A slice can initialize a `var`, but not a `mut` (§5.3).
- Indexing a `str` gives a one-character `str`.

### 6.8 Blocks and yield
A block `{ ... }` is an expression. It has its own scope. Its value is what a `>>` statement
inside it yields:
```solr
var label = {
    var n = len(items);
    >> "count: " + str(n);
};
```
- `>>` yields the value of the **innermost** enclosing block. In `if c { >> x; }` used as a
  statement, `x` is discarded, and does not leave the outer block. This is easy to get wrong.
- A block with no `>>` has the type `void`.
- `return` leaves the enclosing **function**, not the block.
- `>>` is only valid inside a block that is used as a value (an `if`, `switch`, or `try`
  branch, or a bare block). A `>>` directly in a function body is an error: use `return`.

### 6.9 `if`
```solr
if n > 0 { log.print("positive"); } else if n < 0 { log.print("negative"); } else { log.print("zero"); }
var sign = if n < 0 { >> -1; } else { >> 1; };
```
- The condition must be a `bool`. It isn't parenthesized.
- The `then` part is always a braced block. The `else` part may be a block, another `if`, or
  any expression.
- As a value, both branches must have the same type. Without `else`, the type is `void`
  **[working]**.

### 6.10 `switch`
`switch` selects by comparing values. It doesn't destructure.
```solr
var name = switch dir {
    Dir.North => "n",
    Dir.South => "s",
    _ => "other"
};

switch {                               // no subject: each case is a bool
    n < 0 => log.print("negative"),
    n == 0 => log.print("zero"),
    _ => log.print("positive")
}
```
- **Arms** are `case => value`, separated by commas. A trailing comma is allowed. After an arm
  whose value is a block, the comma is optional.
- `_` matches anything and is the default arm. It must be last.
- Without a subject, each case is a `bool` expression and the first true one wins.
- With a subject, each case is compared to it with `==`.
- **Narrowing [provisional]:** inside an arm whose case is an enum variant, the subject
  variable is known to be that variant, so its fields are readable:
  ```solr
  switch action {
      Action.Add => "add " + action.title,
      Action.Complete => "complete " + str(action.id),
      Action.Quit => "quit"
  }
  ```
  Binding the fields in the case (`Action.Add(title) =>`) and an `is` test are **[open]**.
- The checker requires the arms to cover every variant when the subject is an enum, or to
  include `_` **[provisional]**.
- All arms must have the same type.

### 6.11 `for`
Three forms, all expressions:
```solr
for i, x in xs { log.print(str(i) + ": " + str(x)); }   // index and element. `_` ignores the index.
for n > 0 { n -= 1; }                                    // while
for { if done { break; } }                               // loop until `break`
```
- The index binding is required in the first form: `for _, x in xs`.
- Loop bindings are copies of the element. They are immutable.
- Like a function's parameters, the loop bindings and the top level of the loop body are one
  scope, so `for _, x in xs { var x = 1; }` is a redeclaration. The same holds for the error
  binding of `try … else |err| { }` (§8.3).
- Iterating over a `mut` variable iterates over a copy taken at the start of the loop, so
  changing the variable in the body doesn't affect the iteration.
- A `for` used as a value collects each `>>` in its body into a slice (a comprehension):
  ```solr
  var ids = for _, t in todos { >> t.id; };
  ```
- A `for` with no condition and no `break` never terminates, which the checker accounts for
  when it checks that a function returns.

### 6.12 Function literals
`fn(a: int) -> int { return a + 1; }` or `fn(a: int) -> int: a + 1`. See §4.2 and §5.4.

### 6.13 `try` and `must`
See §8.

### 6.14 Constructor expressions
`Ok(x)`, `Err(e)`, `Some(x)`, `None()`. See §8.

---

## 7. Statements

### 7.1 Terminators
A `;` ends every statement and every top-level `import`, `var`, or alias. It is required even
before a closing `}`.

**Exception:** an item whose last token is `}` doesn't need one. That covers `fn`, `type`, and
`impl` declarations, `if`, `for`, `switch`, and block statements, and also a `var` or assignment
whose value ends in a block (`if ... { } else { }`, `try ... else { }`, or a function literal
with a block body). A `;` is allowed after any of these, and is ignored.
```solr
var x = 1;
if x > 0 { log.print("positive"); }                 // no `;` needed after the closing brace
var y = if x > 0 { >> 1; } else { >> 2; }           // also fine: the last token is `}`
var z = try parse(s) else { return; }               // same
var w = x + 1;                                      // needs `;`: the last token is `1`
```
Line breaks have no meaning.

### 7.2 Statement forms
| Statement | Form |
|---|---|
| Immutable binding | `var name [: Type] = expr;` |
| Mutable binding | `mut name [: Type] = expr;` |
| Assignment | `target = expr;` and `target op= expr;` for `+ - * / %` |
| Return | `return;` or `return expr;` |
| Yield | `>> expr;` |
| Break, continue | `break;` `continue;` |
| Expression | `expr;` (a call, `if`, `for`, `switch`, a block, or `try`/`must` of a call) |

An assignment target is a variable, a field, or an element. Assigning to the variable itself
rebinds it. Assigning to a field or element changes contents and needs `mut` (§5.1).

---

## 8. Errors and absence

### 8.1 `Result`
`Result[T]`, written `T!`, is either `Ok(value)` or `Err(error)`, where `error` implements
`Error`. It is used when an operation can **fail with a reason**. `void!` is a result with no
payload: `Ok()`, `Err(e)`. A bare `Result`, with no type argument, means `Result[void]`, so
`fn save() -> Result` and `fn save() -> void!` are the same. `Maybe` always needs its type
argument.
```solr
fn parseAge(s: str) -> int! {
    if s == "" { return Err(Error("empty input")); }
    return Ok(42);
}
```
`Error` is a built-in interface with one method:
```solr
type Error interface { fn error(self) -> str }
```
`Error("message")` creates a generic error **[provisional]**. A user type implements `Error`
by having a method `fn error(self) -> str` in its `impl`.

### 8.2 `Maybe`
`Maybe[T]`, written `T?`, is either `Some(value)` or `None()`. It is used when a value may
**legitimately be absent**:
```solr
type Profile struct {
    pub name: str,
    pub email: str?              // may be absent. Defaults to None().
}
```
Which absences are `Maybe` and which are `Result` is **[open]**. `xs.get(i)` returns a
`Result`.

### 8.3 `try`
`try e` unwraps a `Result` (or `Maybe` **[provisional]**).
| Form | Meaning |
|---|---|
| `try e` | If `e` is `Ok(v)`, the value is `v`. If it is `Err`, the **enclosing function returns that `Err`** immediately. The function must return a `Result`. |
| `try e else v` | On failure, the value is `v`. |
| `try e else { ... }` | On failure, run the block. It either yields a value (`>>`) or leaves the function (`return`, `break`, `continue`). |
| `try e else \|err\| { ... }` | Same, with the error bound to `err`. |

The `else` value extends as far right as possible (§6.1).
```solr
fn loadAge(s: str) -> int! {
    var n = try parseAge(s);                       // returns the Err to the caller
    return Ok(n);
}
fn ageOrZero(s: str) -> int {
    return try parseAge(s) else 0;
}
```

### 8.4 `must`
`must e` unwraps a `Result` or `Maybe`, and **panics** if it failed. Use it where a failure
means a bug. `must` on a value that isn't a `Result` or `Maybe` is a compile error.

### 8.5 Panics
A panic stops the program with a message. They come from `must`, from indexing out of range,
and from integer division by zero. There is no recovery mechanism.

---

## 9. Operators and conversions

### 9.1 Typing rules
Operand types must be the same. There is no implicit conversion.

| Operators | Operands | Result |
|---|---|---|
| `+ - *` | `int, int` or `float, float` | the same type |
| `/` | `int, int` | `int`, **floor division** |
| `/` | `float, float` | `float` |
| `%` | `int, int` | `int` |
| `+` | `str, str` | `str` (concatenation) |
| `== !=` | two values of the **same type** | `bool` |
| `< <= > >=` | `int`, `float`, or `str` pairs | `bool` |
| `and or` | `bool, bool` | `bool` |
| unary `-` | `int` or `float` | the same type |
| unary `!` | `bool` | `bool` |

- `1 + 1.0` and `7.0 / 2` are errors. Write `7.0 / 2.0`.
- `==` between different types is a **compile error**, not `false`.
- Comparing structs and enums with `==` is **[open]**.
- The order of `str` values (bytes or code points) is **[open]**.

### 9.2 Division and remainder
`int / int` rounds toward negative infinity: `7 / 2 == 3` and `-7 / 2 == -4`.
`%` on `int`s takes the sign of the divisor, so `a / b * b + a % b == a`. Dividing by zero is a
panic.

### 9.3 Conversions
`int(x)`, `float(x)`, and `str(x)` are the built-in conversion functions, and the only way to
combine different numeric types. The checker knows the argument's type, so each conversion is
chosen at compile time.
```solr
var ratio = float(3) / 2.0;
var label = "n=" + str(3);
```
What each conversion accepts and how it fails (for example `int(3.7)`, `int("42")`, `str(true)`)
is **[open]**.

---

## 10. Built-in functions and methods

The standard library isn't specified yet. This section lists what the reference programs use.
All of it is **[provisional]**.

### 10.1 Functions
| Name | Meaning |
|---|---|
| `len(x)` | length of a slice or `str`, as `int` |
| `int(x)`, `float(x)`, `str(x)` | conversions (§9.3) |
| `Error(msg)` | creates an error (§8.1) |

### 10.2 Slice and string methods
**Return a new value** (work on any binding):

| Method | Meaning |
|---|---|
| `xs.append(x)` | new slice with `x` added at the end |
| `xs.prepend(x)` | new slice with `x` added at the front |
| `xs.get(i)` | `T!`, the element, or an error if out of range |
| `xs.contains(x)` | `bool` |
| `xs.find(f)` | `T!` the first element for which `f` is true **[open]** |
| `s.get(i)` | `str!` for a `str` |

**Change in place** (need a `mut` receiver):

| Method | Meaning |
|---|---|
| `xs.push(x)` | add `x` to the end |
| `xs[i] = x` | replace an element |
| `xs.sort()` | sort |

`pop` on a `mut` slice (in place, or returning a pair) is **[open]**.

### 10.3 Structs
`x.with(field=value, ...)` returns a copy with the fields replaced. It is built in for every
struct and enum variant. Its arguments are checked against the field names and types.

### 10.4 Modules
`std::fmt`, `std::log`, `std::errors`, and `std::io` appear in the reference programs. Their
contents are **[open]**.

---

## 11. Modules and visibility

- Everything is private to its module unless it is marked `pub`.
- `pub` applies to: top-level `fn`, `type`, and `var`; methods in an `impl`; and struct fields.
- Interface methods are always public.
- Whether enum variants share their type's visibility, and whether a module is a file or a
  directory, are **[open]**.

---

## 12. Concurrency

**[open]**. The direction:
- Concurrency is done with **actors**, declared with `actor fn` **[provisional]**.
- Everything an actor receives is an immutable value, so there are no data races. An actor that
  wants to change a value copies it, then returns its copy or sends a message.
- Shared mutable state does not exist. State that has to be shared is owned by one actor, and
  others send it messages.
- How an actor is started, how it is called, and how results come back are not decided.

---

## 13. Complete grammar

Notation: `a b` sequence; `a | b` choice; `[a]` optional; `{a}` zero or more; `'x'` a token.
Where the grammar says `list(x)`, items are separated by commas, with an optional trailing
comma. The rule for `;` is in §7.1.

```
program      = { topItem } ;
topItem      = importDecl | [ 'pub' ] ( fnDecl | typeDecl | varDecl ) | implDecl ;

importDecl   = 'import' ident { '::' ident } ';' ;
varDecl      = 'var' ident [ ':' type ] '=' expr ';' ;
fnDecl       = 'fn' ident [ typeParams ] '(' list(param) ')' [ '->' type ] ( block | ':' expr ';' ) ;
param        = ident ':' type | 'self' ;      (* `self` only first, in a method or interface *)
typeParams   = '[' list(ident [ ':' type ]) ']' ;
implDecl     = 'impl' type '{' { [ 'pub' ] fnDecl } '}' ;

typeDecl     = 'type' ident [ typeParams ] ( '=' type ';'
                                           | 'struct' '{' list(field) '}'
                                           | 'enum' '{' list(variant) '}'
                                           | 'interface' '{' { 'fn' sig } '}' ) ;
field        = [ 'pub' ] ident ':' type ;
variant      = ident [ '(' list(field) ')' ] ;
sig          = ident [ typeParams ] '(' list(param) ')' [ '->' type ] ;

type         = typeAtom [ '!' | '?' ] ;
typeAtom     = ident { '.' ident } [ '[' list(type) ']' ]
             | '[' ']' typeAtom
             | 'fn' '(' list([ ident ':' ] type) ')' [ '->' type ] ;

block        = '{' { stmt } '}' ;
stmt         = 'var' ident [ ':' type ] '=' expr ';'
             | 'mut' ident [ ':' type ] '=' expr ';'
             | expr assignOp expr ';'
             | 'return' [ expr ] ';'
             | '>>' [ expr ] ';'
             | 'break' ';' | 'continue' ';'
             | expr ';' ;
assignOp     = '=' | '+=' | '-=' | '*=' | '/=' | '%=' ;

expr         = prefix { infix } ;
prefix       = literal | ident | '(' expr ')' | '[' list(expr | '...' expr) ']'
             | '-' expr | '!' expr | '@' expr
             | 'try' expr [ 'else' [ '|' ident '|' ] ( block | expr ) ]
             | 'must' expr
             | 'Ok' '(' [ expr ] ')' | ( 'Err' | 'Some' ) '(' expr ')' | 'None' '(' ')'
             | 'if' expr block [ 'else' expr ]
             | 'switch' [ expr ] '{' list(arm) '}'
             | 'for' [ ident ',' ident 'in' ] [ expr ] block
             | 'fn' '(' list(param) ')' [ '->' type ] ( block | ':' expr )
             | block ;
arm          = expr '=>' ( block | expr ) ;
infix        = binop expr
             | '(' list(arg) ')'
             | '[' [ expr ] [ ':' [ expr ] ] ']'
             | '.' ident ;
arg          = [ ident '=' ] expr | '...' expr ;
binop        = 'or' | 'and' | '==' | '!=' | '<' | '<=' | '>' | '>='
             | '+' | '-' | '*' | '/' | '%' ;
```

The grammar above is the **target** language. In `type`, the suffix `!` or `?` applies to the
whole type before it: `[]int!` is a `Result` of `[]int`, not a slice of results. The lexer and
parser differ from the grammar in the ways listed in §15.

---

## 14. Example programs

The examples are checked: each one parses as written with the current parser. The one exception
is the block in §5.6, which shows programs that are rejected. All of it parses except
`fn g(t: mut Todo)`, which the parser rejects (there are no `mut` parameter types). The checker
reports the other errors in that block.

### 14.1 Values and mutability
```solr
type Status enum { Open, Done }

type Todo struct {
    pub id: int,
    pub title: str,
    pub status: Status
}

type State struct {
    pub todos: []Todo
}

// A new State: nothing is changed.
fn add(s: State, title: str) -> State {
    var todo = Todo(id=len(s.todos) + 1, title=title, status=Status.Open);
    return s.with(todos=s.todos.append(todo));
}

// `done` is changed in place. The change is local to this function.
fn completeAll(s: State) -> State {
    mut done: []Todo = [];
    for _, t in s.todos {
        done.push(t.with(status=Status.Done));
    }
    return s.with(todos=done);
}

// Copy, change, return.
fn rename(t: Todo, title: str) -> Todo {
    mut mine = @t;
    mine.title = title;
    return mine;
}

fn main() {
    var s = State(todos=[]);
    s = add(s, "write the spec");       // rebinding
    s = add(s, "review the spec");
    var before = s;                     // a copy: it never changes
    s = completeAll(s);
    // before.todos are still Open. s.todos are Done.
}
```

### 14.2 Structs, methods, and interfaces
```solr
type Shape interface {
    fn area(self) -> float
}

type Rect struct { pub w: float, pub h: float }
type Circle struct { pub r: float }

impl Rect {
    pub fn area(self) -> float {
        return self.w * self.h;
    }
}

impl Circle {
    pub fn area(self) -> float {
        return 3.14159 * self.r * self.r;
    }
}

// Accepts anything that has an `area` method.
fn total(shapes: []Shape) -> float {
    mut sum = 0.0;
    for _, s in shapes {
        sum += s.area();
    }
    return sum;
}
```

### 14.3 Enums and `switch`
```solr
type Action enum {
    Add(title: str),
    Complete(id: int),
    Quit
}

fn describe(a: Action) -> str {
    return switch a {
        Action.Add => "add " + a.title,
        Action.Complete => "complete " + str(a.id),
        Action.Quit => "quit"
    };
}

fn classify(n: int) -> str {
    return switch {
        n < 0 => "negative",
        n == 0 => "zero",
        _ => "positive"
    };
}
```

### 14.4 Errors
```solr
fn find(todos: []Todo, id: int) -> Todo! {
    for _, t in todos {
        if t.id == id {
            return Ok(t);
        }
    }
    return Err(Error("todo " + str(id) + " not found"));
}

// `try` returns the error to the caller.
fn titleOf(todos: []Todo, id: int) -> str! {
    var t = try find(todos, id);
    return Ok(t.title);
}

// `try ... else` handles the error here.
fn titleOrDefault(todos: []Todo, id: int) -> str {
    var t = try find(todos, id) else {
        return "(missing)";
    };
    return t.title;
}

// `must` panics. Use it only where a failure is a bug.
fn firstTitle(todos: []Todo) -> str {
    var t = must find(todos, 1);
    return t.title;
}
```

### 14.5 Closures and generics
```solr
fn makeAdder(base: int) -> fn(int) -> int {
    return fn(n: int) -> int: base + n;
}

fn map[A, B](xs: []A, f: fn(A) -> B) -> []B {
    mut out: []B = [];
    for _, x in xs {
        out.push(f(x));
    }
    return out;
}

fn main() {
    var addTen = makeAdder(10);
    var doubled = map([1, 2, 3], fn(n: int) -> int: n * 2);
    log.print(str(addTen(5)));
}
```

### 14.6 A small lexer
```solr
type Kind enum { Plus, Minus, Int, Eof, Illegal }

type Token struct {
    pub kind: Kind,
    pub lit: str
}

fn isDigit(ch: str) -> bool: ch >= "0" and ch <= "9";

fn lex(input: str) -> []Token {
    mut tokens: []Token = [];
    mut pos = 0;
    for pos < len(input) {
        var ch = input[pos];
        if ch == " " {
            pos += 1;
            continue;
        }
        if isDigit(ch) {
            var start = pos;
            for pos < len(input) and isDigit(input[pos]) {
                pos += 1;
            }
            tokens.push(Token(kind=Kind.Int, lit=input[start:pos]));
            continue;
        }
        var tok = switch ch {
            "+" => Token(kind=Kind.Plus, lit=ch),
            "-" => Token(kind=Kind.Minus, lit=ch),
            _ => Token(kind=Kind.Illegal, lit=ch)
        };
        tokens.push(tok);
        pos += 1;
    }
    tokens.push(Token(kind=Kind.Eof, lit=""));
    return tokens;
}
```
Every helper that would need to change the lexer's position would instead have to return it.
This version keeps the whole loop in one function, so `tokens` and `pos` are `mut` locals.

---

## 15. Implementation status

The lexer and parser are ahead of the checker, which is being built (`docs/analyzer-plan.md`).
As of 2026-10-28 they implement everything in this document except the items below.

**Not implemented:**

| Feature | Status |
|---|---|
| Tuples, fixed arrays `[N]T`, map literals | See §3.6, §3.3, §2.4, and §17. |
| Other integer bases, `_` in numbers, `'` characters, non-ASCII source | See §17. |
| `actor fn` | `actor` is an ordinary identifier. There is no keyword and no parse rule. |
| `is` in `switch` arms | **[open]** as a feature. |
| Type-parameter constraints (`S: Iterable[T]`) | `TypeParameter` has no constraint field, so `[T, S: X]` is a parse error. |

**The parser accepts more than the grammar, or parses it differently.** The checker, or a later
parser change, must reject or fix these:

| Input | Behavior now | Spec |
|---|---|---|
| `Err()` | accepted | `Err` needs a value (§8.1) |
| `a:b`, `xs[a:b:c]` | `:` is a general infix operator, accepted anywhere an expression is | only `xs[a:b]`, `xs[:b]`, `xs[a:]` inside `[ ]` (§6.7) |
| `fn f();` | a function with no body is accepted | a body is required (§4.2) |
| `fn f(x) {}` | a parameter with no type is accepted | types are required (§4.2) |
| `pub impl A {}`, `pub import a;`, `pub var x = 1;` inside a function | accepted | `pub` applies to `fn`, `type`, and top-level `var` (§11) |
| `1 + 2;` as a statement | accepted | expression statements are calls, `if`, `for`, `switch`, blocks, and `try`/`must` of a call (§7.2) |

**Enforced by the checker, not the parser.** The rules in this document that are written as an
error: types and conversions, mutability, `@x` only as a binding initializer, `mut x = y` without
`@`, `_` last in a `switch`, enum coverage, the `PascalCase` convention, and so on. A top-level
`mut` is rejected by the parser, with a generic "unexpected token" message.

**Changes in the current lexer and parser, for reference:** `mut` declarations; `T?`; `Some` and
`None`; `...` in sequence literals and call arguments; `!=` after a type read as `!` then `=`;
`@x` as the copy operator (`CopyExpr`); `mut` as a parameter type removed; `try` at the unary
level; expression-bodied `impl` methods ending in `;`; string escapes decoded in the lexer, with
lexer errors reported separately from parse errors.

---

## 16. Open questions

Consolidated from the markers above.

**Values and mutability**
- Can a parameter be rebound inside its function?
- Do omitted struct fields follow §3.4? Will per-field defaults be added?
- A `ref` type for shared mutable state, if ever.

**Types**
- Tuple syntax, element access, destructuring.
- `Maybe` vs `Result`: which absences are which?
- Constraint syntax for generics.
- Equality of structs and enums (`==`).
- Is a struct that contains itself directly (`type N struct { next: N }`, with no `?` or slice
  in between) an error? It can never be constructed.
- Map type and literal syntax. Fixed-length arrays.

**Syntax**
- Named arguments for ordinary functions. Calling a function stored in a field.
- `switch`: field binding in the case, `is`, and several values per arm (`1 | 2 =>`).
- Format strings and interpolation. The exact escape set.
- Static functions (`Type.fn`). Whether a module is a file or a directory.

**Library and runtime**
- Conversion semantics: what `int()`, `float()`, and `str()` accept and how they fail.
- Ordering of `str`. The standard library surface. In-place `pop`.
- Persistent trees or copy-on-write arrays. A generational collector assumes Stellar owns the
  runtime, which conflicts with a Go transpilation target (#141).
- Actor syntax, creation, and messaging.

---

## 17. To do: planned features

Features that are not supported now but are likely to be added. Nothing here is decided. The
current behavior is noted for each one, so a program doesn't depend on it by accident.

**Number literals**
- Hexadecimal (`0xFF`), binary (`0b101`), and octal (`0o7`) integers, and `_` as a digit
  separator (`1_000`).
- Now: `0xFF` lexes as the integer `0` followed by the identifier `xFF`, and `1_000` as `1`
  followed by `_000`.

**Tuples**
- Type syntax (`(A, B)` or `tuple[A, B]`), element access (`t.0` or destructuring only),
  one-element tuples, destructuring in `var`, assignment, and `for`.
- **Lexer caveat for `t.0`:** if element access is written `t.0`, then `t.0.1` lexes as `t`,
  `.`, and the float `0.1`. The lexer would need to read digits as an integer after a `.`.
- Tuples are required by the language (`Result[(A, B)]`, iterator steps, map entries).

**Characters**
- A character type and single-quoted literals (`'a'`).
- Now: `'` is a lexer error, and indexing a `str` gives a one-character `str` (§6.7).

**Non-ASCII source**
- The lexer is being considered for conversion from bytes to runes, so that columns count
  characters and non-ASCII text outside of strings and comments is handled.
- Now: a byte above 127 outside a string or comment gives one error token per byte, and
  columns count bytes. Inside strings and comments it passes through unchanged.

**Others already listed in the open questions (§16)**
- Fixed-length arrays and the map type.
- String interpolation and format strings.
- A `ref` type for shared mutable state.
