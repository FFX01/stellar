# Pratt Parser for Stellar

## What is a Pratt parser?

A Pratt parser (also called "top-down operator precedence") is a recursive-descent
technique where **each token type owns two functions**:

1. **Prefix** — "parse what comes first" (literals, identifiers, `(`, `!`, `-`, `new`, etc.)
2. **Infix** — "handle the binary operator that follows" (`+`, `-`, `==`, `if`, etc.)

The core insight: when you're parsing a binary expression like `a + b * c`, you
don't need to know the whole grammar upfront. You just need each operator to know
**how far it should consume**. That's expressed as a **precedence level**, and
each operator bumps the minimum-acceptable-precedence (the "bind power") when it
hands control to the next level.

```
    +                     (prefix: parse "a")
   / \
  a   *                 (infix + sees *, decides: "is * higher precedence than me?")
     / \                (YES → parse b * c as a unit, return it)
    b   *               (infix *: parse "b")
       / \
      c   -            (infix *: is - higher precedence than *? NO → stop, return b*c)
```

The result is a tree: `a + (b * c)`, which is exactly what you want.

## Why Pratt over hand-written recursive descent?

For a language like Stellar with many expression types, hand-written recursive
descent means writing one function per precedence level — and each level calls
the next. That's fine, but **Pratt unifies it**: one `parseExpr(bp)` function
does precedence climbing, and each token type just registers its prefix/infix.

This maps naturally to your existing `Precedence` constants and makes adding
new operators trivial — just register a new infix handler.

## Core data structures

### Prefix / Infix function signatures

```go
// prefixFn parses what comes first: a literal, identifier, parenthesized expr, etc.
// It returns the node and consumes no infix tokens.
type prefixFn func(p *Parser) ast.Expr

// infixFn handles a binary operator. It receives the left side (already parsed),
// the current token, and the bind power of the *current* operator. It must return
// a fully-reduced right-associative subtree (or just `right` if non-associative).
type infixFn func(p *Parser, left ast.Expr) ast.Expr
```

### Precedence table (extend yours)

Your existing table stops at `PProduct`. Here's what you'll need:

```go
type Precedence int

const (
    _ Precedence = iota
    PLowest
    PAssign        // =
    POr            // or
    PAnd           // and
    PEq            // == !=
    PRelational    // < > <= >=
    PSum           // + -
    PProduct       // * /
    PUnary         // ! - (prefix)
    PCall          // .  ()  []   (left-associative, highest atomic)
    PIndex         // [expr]  (left-associative)
    PHighest       // end sentinel
)
```

### The registry

Each token type maps to optional prefix/infix handlers:

```go
type parser struct {
    tokens   []lexer.Token
    pos      int
    errors   []parser.Error

    prefixes map[lexer.T_Token]prefixFn
    infixes  map[lexer.T_Token]infixFn

    precedences map[lexer.T_Token]Precedence
}
```

## The central function: `parseExpr(bp)`

This is the heart of the Pratt parser. It takes a **minimum bind power** (`bp`)
and parses an expression that must have precedence ≥ `bp`.

```go
func (p *parser) parseExpr(bp Precedence) ast.Expr {
    // 1. Parse the left side (prefix).
    prefix := p.prefixes[p.cur().Type]
    if prefix == nil {
        p.errorf(p.cur(), "Expected expression, got %s", p.cur().Type)
        return nil
    }
    left := prefix(p)  // e.g. parses "42" or "x" or "(..."

    // 2. While the next token is an infix operator with precedence >= bp,
    //    keep consuming it.
    for p.peekPrecedence() >= bp {
        infix := p.infixes[p.cur().Type]
        if infix == nil {
            break  // not an infix operator at this level; stop
        }
        left = infix(p, left)  // e.g. handles "+", "*", etc.
    }

    return left
}
```

That's it. One function, ~15 lines. Everything else is registration.

## Step-by-step: parsing `a + b * c`

Tokens: `IDENT("a") PLUS IDENT("b") AST IDENT("c")`

```
parseExpr(PLowest+1)
  prefix(IDENT "a") → IdentifierExpr{Name: "a"}
  cur = PLUS, precedence(PLUS) = PSum ≥ PLowest+1 → continue
    infix(PLUS):
      parseExpr(PSum + 1)   ← bumped bp because PLUS is left-assoc
        prefix(IDENT "b") → IdentifierExpr{Name: "b"}
        cur = AST "*", precedence(AST) = PProduct ≥ PSum+1 → continue
          infix(AST):
            parseExpr(PProduct + 1)   ← bumped bp again
              prefix(IDENT "c") → IdentifierExpr{Name: "c"}
              cur = EOF, no infix → return IdentifierExpr{c}
            return BinaryExpr{Left: b, Op: *, Right: c}
        return BinaryExpr{Left: b, Op: *, Right: c}
      return BinaryExpr{Left: a, Op: +, Right: (b*c)}
  return BinaryExpr{a, +, (b*c)}
```

The tree is `a + (b * c)`. Correct.

## Registration examples for Stellar

### Literals (prefix only)

```go
p.prefixes[lexer.NUM] = func(p *parser) ast.Expr {
    n := &ast.IntegerLiteralExpr{Token: p.advance()}
    n.Value, _ = strconv.Atoi(n.Token.Literal)
    return n
}

p.prefixes[lexer.STRING] = func(p *parser) ast.Expr {
    return &ast.StringLiteralExpr{Token: p.advance(), Value: p.tokens[p.pos-1].Literal}
}

p.prefixes[lexer.TRUE] = func(p *parser) ast.Expr {
    return &ast.BooleanLiteralExpr{Token: p.advance(), Value: true}
}

p.prefixes[lexer.FALSE] = func(p *parser) ast.Expr {
    return &ast.BooleanLiteralExpr{Token: p.advance(), Value: false}
}

// IDENT covers all identifiers: variables, function names, type names.
// Note: builtin types like "int", "str", "bool" are lexed as IDENT,
// not as special keyword tokens.
p.prefixes[lexer.IDENT] = func(p *parser) ast.Expr {
    return &ast.IdentifierExpr{Token: p.advance(), Name: p.tokens[p.pos-1].Literal}
}
```

### Parenthesized expressions (prefix)

```go
p.prefixes[lexer.LPAREN] = func(p *parser) ast.Expr {
    p.advance()  // consume (
    expr := p.parseExpr(PLowest + 1)
    p.expect(lexer.RPAREN)
    return expr
}
```

### Prefix unary operators (`-`, `!`)

```go
p.prefixes[lexer.MINUS] = func(p *parser) ast.Expr {
    expr := p.parseExpr(PUnary)  // right-assoc: parse rhs at same precedence
    return &ast.UnaryExpr{Token: p.advance(), Op: "-", Operand: expr}
}

p.prefixes[lexer.NOT] = func(p *parser) ast.Expr {
    expr := p.parseExpr(PUnary)
    return &ast.UnaryExpr{Token: p.advance(), Op: "!", Operand: expr}
}
```

### Binary operators

**Left-associative** (most operators): bump `bp` by 1, parse rhs at bumped level.

```go
p.infixes[lexer.PLUS] = func(p *parser, left ast.Expr) ast.Expr {
    expr := &ast.BinaryExpr{Token: p.advance(), Op: "+"}
    expr.Right = p.parseExpr(PSum + 1)
    expr.Left = left
    return expr
}

p.infixes[lexer.AST] = func(p *parser, left ast.Expr) ast.Expr {
    expr := &ast.BinaryExpr{Token: p.advance(), Op: "*"}
    expr.Right = p.parseExpr(PProduct + 1)
    expr.Left = left
    return expr
}

p.infixes[lexer.EQ] = func(p *parser, left ast.Expr) ast.Expr {
    expr := &ast.BinaryExpr{Token: p.advance(), Op: "=="}
    expr.Right = p.parseExpr(PEq + 1)
    expr.Left = left
    return expr
}
```

**Right-associative** (assignment `=`): do **not** bump. Parse rhs at the *same*
precedence so that `a = b = c` groups as `a = (b = c)`.

```go
p.infixes[lexer.ASSIGN] = func(p *parser, left ast.Expr) ast.Expr {
    expr := &ast.AssignExpr{Token: p.advance()}
    expr.Right = p.parseExpr(PAssign)  // same bp → right-assoc
    expr.Left = left
    return expr
}
```

### Call / index / dot (highest precedence, left-assoc)

```go
p.infixes[lexer.LPAREN] = func(p *parser, left ast.Expr) ast.Expr {
    expr := &ast.CallExpr{Token: p.advance(), Function: left}
    for !p.check(lexer.RPAREN) {
        expr.Arguments = append(expr.Arguments, p.parseExpr(PLowest+1))
        if !p.match(lexer.COMMA) { break }
    }
    p.expect(lexer.RPAREN)
    return expr
}

p.infixes[lexer.LBRACK] = func(p *parser, left ast.Expr) ast.Expr {
    expr := &ast.IndexExpr{Token: p.advance(), Target: left}
    expr.Index = p.parseExpr(PLowest + 1)
    p.expect(lexer.RBRACK)
    return expr
}

p.infixes[lexer.DOT] = func(p *parser, left ast.Expr) ast.Expr {
    name := p.expect(lexer.IDENT).Literal
    return &ast.MemberExpr{Token: p.advance(), Object: left, Field: name}
}
```

**Note on `new`:** Since `new` is a keyword token (`lexer.NEW`), it gets its own
prefix handler, not merged with IDENT. Register it separately:

```go
p.prefixes[lexer.NEW] = func(p *parser) ast.Expr {
    p.advance()  // consume "new"
    typeName := p.expect(lexer.IDENT).Literal
    return &ast.NewExpr{Token: p.advance(), Type: typeName}
}
```

### The `if` expression (special infix)

`if` is tricky: it's an **infix** operator because it follows an expression, and
it consumes a block. The bp doesn't matter much since `if` is a statement-level
construct, but use `PLowest + 1` so it parses after any prefix expression.

```go
p.infixes[lexer.IF] = func(p *parser, left ast.Expr) ast.Expr {
    // left is the condition expression (already parsed by prefix)
    expr := &ast.IfExpr{Token: p.advance(), Condition: left}
    expr.Body = p.parseBlock()
    if p.check(lexer.ELSE) {
        p.advance()
        expr.Else = p.parseExpr(PAssign)  // or parse another if/else chain
    }
    return expr
}
```

## Token-to-precedence lookup

```go
var precedenceMap = map[lexer.T_Token]Precedence{
    lexer.ASSIGN:  PAssign,
    lexer.OR:      POr,
    lexer.AND:     PAnd,
    lexer.EQ:      PEq,
    lexer.NEQ:     PEq,
    lexer.LT:      PRelational,
    lexer.LTE:     PRelational,
    lexer.GT:      PRelational,
    lexer.GTE:     PRelational,
    lexer.PLUS:    PSum,
    lexer.MINUS:   PSum,
    lexer.AST:     PProduct,
    lexer.DIV:     PProduct,
    lexer.NOT:     PUnary,
    lexer.MINUS:   PUnary,  // also prefix
}

func (p *parser) peekPrecedence() Precedence {
    if prec, ok := precedenceMap[p.cur().Type]; ok {
        return prec
    }
    return PLowest
}
```

## Putting it together: the full parser skeleton

```go
type parser struct {
    tokens      []lexer.Token
    pos         int
    errors      []parser.Error
    prefixes    map[lexer.T_Token]prefixFn
    infixes     map[lexer.T_Token]infixFn
}

func NewParser(tokens []lexer.Token) *parser {
    p := &parser{
        tokens:    tokens,
        prefixes:  make(map[lexer.T_Token]prefixFn),
        infixes:   make(map[lexer.T_Token]infixFn),
    }
    p.registerPrefixes()
    p.registerInfixes()
    return p
}

func (p *parser) registerPrefixes() {
    p.prefixes[lexer.NUM] = p.parseIntegerLiteral
    p.prefixes[lexer.STRING] = p.parseStringLiteral
    p.prefixes[lexer.TRUE] = p.parseBoolLiteral
    p.prefixes[lexer.FALSE] = p.parseBoolLiteral
    p.prefixes[lexer.IDENT] = p.parseIdentifier
    p.prefixes[lexer.LPAREN] = p.parseGroupingExpr
    p.prefixes[lexer.MINUS] = p.parsePrefixExpr   // unary -
    p.prefixes[lexer.NOT] = p.parsePrefixExpr     // unary !
    p.prefixes[lexer.NEW] = p.parseNewExpr
}

func (p *parser) registerInfixes() {
    p.infixes[lexer.PLUS] = p.parseBinaryExpr
    p.infixes[lexer.MINUS] = p.parseBinaryExpr
    p.infixes[lexer.AST] = p.parseBinaryExpr
    p.infixes[lexer.DIV] = p.parseBinaryExpr
    p.infixes[lexer.EQ] = p.parseBinaryExpr
    p.infixes[lexer.NEQ] = p.parseBinaryExpr
    p.infixes[lexer.LT] = p.parseBinaryExpr
    p.infixes[lexer.LTE] = p.parseBinaryExpr
    p.infixes[lexer.GT] = p.parseBinaryExpr
    p.infixes[lexer.GTE] = p.parseBinaryExpr
    p.infixes[lexer.LPAREN] = p.parseCallExpr
    p.infixes[lexer.LBRACK] = p.parseIndexExpr
    p.infixes[lexer.DOT] = p.parseMemberExpr
    p.infixes[lexer.ASSIGN] = p.parseAssignExpr
    p.infixes[lexer.IF] = p.parseIfExpr
}

func (p *parser) parseExpr(bp Precedence) ast.Expr {
    prefix := p.prefixes[p.cur().Type]
    if prefix == nil {
        p.error(p.cur(), "Expected expression, got %s", p.cur().Type)
        return nil
    }
    left := prefix(p)

    for p.peekPrecedence() >= bp {
        infix := p.infixes[p.cur().Type]
        if infix == nil {
            break
        }
        left = infix(p, left)
    }
    return left
}

func (p *parser) Parse() *ast.Program {
    prog := &ast.Program{}
    for !p.check(lexer.EOF) {
        prog.Nodes = append(prog.Nodes, p.parseStatement())
    }
    return prog
}

func (p *parser) parseStatement() ast.Node {
    switch p.cur().Type {
    case lexer.VAR:
        return p.parseVarStmt()
    case lexer.FUNC:
        return p.parseFuncDecl()
    default:
        // Expression statement (or assignment via infix =)
        expr := p.parseExpr(PLowest + 1)
        if expr == nil {
            p.advance()
            return nil
        }
        // If it's an assignment, it will be consumed by infix(ASSIGN) above.
        // Otherwise wrap as an expression statement.
        return &ast.ExprStmt{Expression: expr}
    }
}
```

## Common pitfalls

### 1. Forgetting to bump precedence for left-associative operators

If you parse `a - b - c` without bumping, you get `(a - b) - c` which is correct
for left-assoc. But if you **don't bump at all**, then `a - b * c` would parse as
`(a - b) * c` because `-` would consume `b * c` greedily.

The rule: **bump by 1** for left-associative (`+`, `-`, `*`, `/`, calls, index).
**Don't bump** for right-associative (`=`, `->` return type).

### 2. Confusing prefix and infix roles

A token can be **both** prefix and infix (e.g. `-` is unary prefix and binary
subtraction). The registry handles this naturally — `-` has entries in both maps.

### 3. The `bp` (bind power) is the *minimum*, not "current"

When `parseExpr(bp)` is called, `bp` says "I already have a left side at this
precedence; only let the next operator take over if it's ≥ bp." Think of it as
"how tight must the next binding be?" Not "what's my precedence."

### 4. Forgetting to handle `EOF` / end of input

The loop `for p.peekPrecedence() >= bp` naturally stops at EOF because
`precedenceMap[EOF]` returns `PLowest`. But make sure your `cur()` and
`peekPrecedence()` handle EOF gracefully.

### 5. Statement-level constructs (`if`, `match`, `try`) as infix

These are **not** prefix — they always follow an expression (the condition or
value being matched). So register them as **infix** handlers, not prefix. The
condition is already parsed as `left` when the infix handler runs.

## AST nodes you'll need (beyond what you have)

```go
// Binary: a + b, a * b, a == b, etc.
type BinaryExpr struct {
    Token lexer.Token
    Left  ast.Expr
    Op    string
    Right ast.Expr
}

// Unary: !a, -a
type UnaryExpr struct {
    Token   lexer.Token
    Op      string
    Operand ast.Expr
}

// Call: foo(a, b)
type CallExpr struct {
    Token    lexer.Token
    Function ast.Expr
    Args     []ast.Expr
}

// Index: a[0], arr[i]
type IndexExpr struct {
    Token  lexer.Token
    Target ast.Expr
    Index  ast.Expr
}

// Member: obj.field, Type.Method
type MemberExpr struct {
    Token  lexer.Token
    Object ast.Expr
    Field  string
}

// Assign: x = expr
type AssignExpr struct {
    Token lexer.Token
    Left  ast.Expr
    Right ast.Expr
}

// If: if cond { body } else { ... }
type IfExpr struct {
    Token     lexer.Token
    Condition ast.Expr
    Body      []ast.Node
    Else      ast.Expr  // another IfExpr or block
}

// New: new Type
type NewExpr struct {
    Token lexer.Token
    Type  string
}

// StringLiteral, BooleanLiteral (you already have IntegerLiteralExpr)
type StringLiteralExpr struct {
    Token lexer.Token
    Value string
}

type BooleanLiteralExpr struct {
    Token lexer.Token
    Value bool
}

// Identifier: x, myVar, SomeType
type IdentifierExpr struct {
    Token lexer.Token
    Name  string
}

// ExprStmt: standalone expression (e.g. a function call used as statement)
type ExprStmt struct {
    Expression ast.Expr
}
```

## Example: parsing `t.rename(title)`

Tokens: `IDENT("t") DOT IDENT("rename") LPAREN IDENT("title") RPAREN`

```
parseExpr(PLowest+1)
  prefix(IDENT "t") → IdentifierExpr{"t"}
  cur = DOT, precedence(DOT) = PCall ≥ PLowest+1 → continue
    infix(DOT):
      name = "rename"
      return MemberExpr{Object: t, Field: "rename"}
  cur = LPAREN, precedence(LPAREN) = PCall ≥ PCall → continue
    infix(LPAREN):
      expr = CallExpr{Function: MemberExpr{t.rename}, ...}
      parse args: IDENT("title") → IdentifierExpr{"title"}
      expect RPAREN
      return CallExpr{Function: t.rename, Args: [title]}
  cur = EOF → stop
```

Result: `t.rename(title)` as a CallExpr wrapping a MemberExpr. Correct.

## Example: parsing `new Todo`

Tokens: `NEW IDENT("Todo")`

```
parseExpr(PLowest+1)
  prefix(NEW):
    advance to consume "new"
    typeName = IDENT("Todo")
    return NewExpr{Type: "Todo"}
  cur = EOF → stop
```

Result: `new Todo` as a NewExpr. Correct.

## Summary

| Concept | What it does |
|---------|-------------|
| `prefixes[T]` | Parse what comes first (literals, `(`, `!`, `-`) |
| `infixes[T]` | Handle binary operators after a left side |
| `parseExpr(bp)` | Core loop: parse prefix, then keep applying infix while precedence ≥ bp |
| Bump `bp + 1` | Left-associative operators (most of them) |
| Keep `bp` | Right-associative operators (`=`, `->`) |
| `peekPrecedence()` | Look up a token's precedence from the registry |

One function, ~15 lines, handles all operator precedence. Everything else is
just registering handlers.
