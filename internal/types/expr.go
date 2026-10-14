package types

import (
	"math/big"
	"strings"

	"stellar/internal/ast"
	"stellar/internal/token"
)

// expr checks x and records its type. want is the type the context expects,
// or nil if the context imposes none. Untyped literals take their type from
// want; other expressions may ignore it, so callers that need a specific
// type use assignTo.
func (c *checker) expr(x ast.Expr, want Type) Type {
	t := c.exprInner(x, want)
	if t == nil {
		t = Invalid
	}
	c.info.Types[x] = t
	return t
}

// assignTo checks x where a value of type want is required.
func (c *checker) assignTo(x ast.Expr, want Type) Type {
	t := c.expr(x, want)
	if t == Invalid || want == Invalid || Assignable(t, want) {
		return t
	}
	if t == Void {
		c.errorf(x.Pos(), "%s produces no value", describe(x))
		return t
	}
	c.errorf(x.Pos(), "cannot use %s (type %s) as %s%s", describe(x), t, want, assignHint(t, want))
	return t
}

// assignHint suggests a fix for a failed assignment.
func assignHint(from, to Type) string {
	switch f := from.(type) {
	case *Result:
		if f.Ok != Void && Assignable(f.Ok, to) {
			return "; use try to unwrap it"
		}
	case *Maybe:
		if Assignable(f.Elem, to) {
			return "; use try to unwrap it"
		}
	}
	switch t := to.(type) {
	case *Maybe:
		if Assignable(from, t.Elem) {
			return "; wrap it with Some(...)"
		}
	case *Result:
		if t.Ok != Void && Assignable(from, t.Ok) {
			return "; wrap it with Ok(...)"
		}
	}
	return ""
}

// needsContext reports whether x's type comes only from its context.
func needsContext(x ast.Expr) bool {
	switch x := x.(type) {
	case *ast.IntLit, *ast.FloatLit, *ast.StringLit, *ast.ArrayLit, *ast.MapLit:
		return true
	case *ast.ParenExpr:
		return needsContext(x.X)
	case *ast.UnaryExpr:
		return x.Op == token.MINUS && needsContext(x.X)
	case *ast.BinaryExpr:
		return isArithmetic(x.Op) && needsContext(x.X) && needsContext(x.Y)
	case *ast.Ident:
		return x.Name == "None"
	case *ast.CallExpr:
		fun, ok := x.Fun.(*ast.Ident)
		if !ok {
			return false
		}
		switch fun.Name {
		case "Err":
			return true
		case "Ok", "Some":
			for _, a := range x.Args {
				if !needsContext(a.Value) {
					return false
				}
			}
			return len(x.Args) > 0
		}
	}
	return false
}

func isArithmetic(op token.Kind) bool {
	switch op {
	case token.PLUS, token.MINUS, token.STAR, token.SLASH, token.PERCENT:
		return true
	}
	return false
}

// contextFrom turns an operand's type into a context for the other
// operand, ignoring types that cannot serve as one.
func contextFrom(t Type) Type {
	if t == Void || t == Never {
		return Invalid
	}
	return t
}

// operands checks the two operands of a binary operator. An operand that
// needs context takes the other operand's type, so `n + 1` and `0 < n` work
// in either order; if both need context, the first takes want.
func (c *checker) operands(x, y ast.Expr, want Type) (Type, Type) {
	switch {
	case needsContext(x) && !needsContext(y):
		ty := c.expr(y, nil)
		return c.expr(x, contextFrom(ty)), ty
	case needsContext(x):
		tx := c.expr(x, want)
		return tx, c.expr(y, contextFrom(tx))
	default:
		tx := c.expr(x, nil)
		return tx, c.expr(y, contextFrom(tx))
	}
}

func (c *checker) exprInner(x ast.Expr, want Type) Type {
	switch x := x.(type) {
	case *ast.IntLit:
		return c.intConst(x, want)
	case *ast.FloatLit:
		return c.literal(x, want, "float literal", func(b *Basic) bool { return b.IsFloat() })
	case *ast.StringLit:
		return c.literal(x, want, "string literal", func(b *Basic) bool { return b == TStr })
	case *ast.BoolLit:
		return TBool
	case *ast.Ident:
		return c.ident(x, want)
	case *ast.ParenExpr:
		return c.expr(x.X, want)
	case *ast.UnaryExpr:
		return c.unary(x, want)
	case *ast.BinaryExpr:
		return c.binary(x, want)
	case *ast.IsExpr:
		return c.isExpr(x, false)
	case *ast.IndexExpr:
		return c.index(x)
	case *ast.ArrayLit:
		return c.arrayLit(x, want)
	case *ast.MapLit:
		return c.mapLit(x, want)
	case *ast.ObjectLit:
		return c.objectLit(x, want)
	case *ast.FuncLit:
		return c.funcLit(x)
	case *ast.IfExpr:
		return c.ifValue(x, want)
	case *ast.MatchExpr:
		t, _ := c.match(x, want, true)
		return t
	case *ast.TryExpr:
		return c.try(x)
	case *ast.MutExpr:
		return c.mutExpr(x, want)
	case *ast.CallExpr:
		return c.call(x, want)
	case *ast.SelectorExpr:
		if ts := c.typeQualifier(x.X); ts != nil {
			return c.typeMember(x, ts, want, nil)
		}
		return c.valueMember(x, c.expr(x.X, nil), false)
	case *ast.Block:
		return c.valueBlock(x, want)
	}
	c.errorf(x.Pos(), "unexpected expression")
	return Invalid
}

// ---------------------------------------------------------------------------
// Literals

func (c *checker) literal(x ast.Expr, want Type, kind string, fits func(*Basic) bool) Type {
	switch want {
	case Invalid:
		return Invalid
	case nil:
		c.errorf(x.Pos(), "cannot infer the type of %s %s: literals take their type from context; add a type annotation", kind, describe(x))
		return Invalid
	}
	if b, ok := want.(*Basic); ok && fits(b) {
		return b
	}
	c.errorf(x.Pos(), "cannot use %s %s as %s%s", kind, describe(x), want, literalHint(x, want))
	return Invalid
}

func literalHint(x ast.Expr, want Type) string {
	if _, ok := want.(*Maybe); ok {
		return "; wrap it with Some(...)"
	}
	if r, ok := want.(*Result); ok && r.Ok != Void {
		return "; wrap it with Ok(...)"
	}
	return ""
}

// intConst checks an integer literal, possibly negated, against want.
func (c *checker) intConst(x ast.Expr, want Type) Type {
	v, _ := intConstValue(x)
	b, ok := want.(*Basic)
	switch {
	case want == Invalid:
		return Invalid
	case want == nil:
		c.errorf(x.Pos(), "cannot infer the type of integer literal %s: literals take their type from context; add a type annotation", describe(x))
		return Invalid
	case ok && b.IsInteger():
		if v != nil && !representable(v, b) {
			c.errorf(x.Pos(), "%s overflows %s", v, b)
		}
		markConst(c.info, x, b)
		return b
	case ok && b.IsFloat():
		c.errorf(x.Pos(), "integer literal %s cannot be %s; write %s.0", describe(x), b, describe(x))
		return Invalid
	}
	c.errorf(x.Pos(), "cannot use integer literal %s as %s%s", describe(x), want, literalHint(x, want))
	return Invalid
}

// intConstValue evaluates an integer literal wrapped in any number of
// parentheses and negations.
func intConstValue(x ast.Expr) (*big.Int, bool) {
	switch x := x.(type) {
	case *ast.IntLit:
		return new(big.Int).SetString(x.Raw, 0)
	case *ast.ParenExpr:
		return intConstValue(x.X)
	case *ast.UnaryExpr:
		if x.Op == token.MINUS {
			if v, ok := intConstValue(x.X); ok {
				return v.Neg(v), true
			}
		}
	}
	return nil, false
}

// markConst records t as the type of every node of a constant expression.
func markConst(info *Info, x ast.Expr, t Type) {
	info.Types[x] = t
	switch x := x.(type) {
	case *ast.ParenExpr:
		markConst(info, x.X, t)
	case *ast.UnaryExpr:
		markConst(info, x.X, t)
	}
}

func representable(v *big.Int, b *Basic) bool {
	bits := uint(b.Bits())
	if b.IsSigned() {
		limit := new(big.Int).Lsh(big.NewInt(1), bits-1)
		return v.Cmp(new(big.Int).Neg(limit)) >= 0 && v.Cmp(limit) < 0
	}
	limit := new(big.Int).Lsh(big.NewInt(1), bits)
	return v.Sign() >= 0 && v.Cmp(limit) < 0
}

func (c *checker) arrayLit(x *ast.ArrayLit, want Type) Type {
	arr, ok := want.(*Array)
	if !ok {
		c.compositeError(x, want, "array literal")
		for _, e := range x.Elems {
			c.expr(e, Invalid)
		}
		return Invalid
	}
	for _, e := range x.Elems {
		c.assignTo(e, arr.Elem)
	}
	return want
}

func (c *checker) mapLit(x *ast.MapLit, want Type) Type {
	m, ok := want.(*Map)
	if !ok {
		c.compositeError(x, want, "map literal")
		for _, e := range x.Entries {
			c.expr(e.Key, Invalid)
			c.expr(e.Value, Invalid)
		}
		return Invalid
	}
	for _, e := range x.Entries {
		c.assignTo(e.Key, m.Key)
		c.assignTo(e.Value, m.Value)
	}
	return want
}

func (c *checker) compositeError(x ast.Expr, want Type, kind string) {
	switch want {
	case Invalid:
	case nil:
		c.errorf(x.Pos(), "cannot infer the type of %s: literals take their type from context; add a type annotation", kind)
	default:
		c.errorf(x.Pos(), "cannot use %s as %s%s", kind, want, literalHint(x, want))
	}
}

func (c *checker) objectLit(x *ast.ObjectLit, want Type) Type {
	name, ok := x.Type.(*ast.Ident)
	if !ok {
		c.errorf(x.Pos(), "module-qualified types are not supported yet")
		return Invalid
	}
	sym, _ := c.pkg.lookup(name.Name).(*TypeSym)
	var obj *Object
	if sym != nil {
		c.info.Uses[name] = sym
		obj, _ = sym.Type.(*Object)
	}
	if obj == nil {
		switch {
		case sym == nil:
			c.errorf(name.Pos(), "undefined type %s", name.Name)
		case isEnum(sym.Type):
			c.errorf(name.Pos(), "%s is an enum; construct a variant with %s.Variant(...)", name.Name, name.Name)
		default:
			c.errorf(name.Pos(), "%s is not an object type", name.Name)
		}
		for _, f := range x.Fields {
			c.expr(f.Value, Invalid)
		}
		return Invalid
	}

	seen := map[string]bool{}
	for _, fi := range x.Fields {
		f := obj.Field(fi.Name.Name)
		switch {
		case f == nil:
			c.errorf(fi.Name.Pos(), "%s has no field %s", obj, fi.Name.Name)
			c.expr(fi.Value, Invalid)
			continue
		case seen[f.Name]:
			c.errorf(fi.Name.Pos(), "duplicate field %s in %s literal", f.Name, obj)
		}
		seen[f.Name] = true
		c.assignTo(fi.Value, f.Type)
	}
	var missing []string
	for _, f := range obj.Fields {
		if !seen[f.Name] {
			missing = append(missing, f.Name)
		}
	}
	if len(missing) > 0 {
		c.errorf(x.Pos(), "missing %s in %s literal", plural(len(missing), "field")+" "+strings.Join(missing, ", "), obj)
	}
	return obj
}

func isEnum(t Type) bool {
	_, ok := t.(*Enum)
	return ok
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// ---------------------------------------------------------------------------
// Names

func (c *checker) ident(x *ast.Ident, want Type) Type {
	sym := c.scope.lookup(x.Name)
	if sym != nil {
		c.info.Uses[x] = sym
	}
	switch s := sym.(type) {
	case nil:
		c.errorf(x.Pos(), "undefined: %s", x.Name)
	case *Var:
		c.capture(x, s)
		return s.Type
	case *FuncSym:
		return s.Sig
	case *TypeSym:
		if _, ok := s.Type.(*Basic); ok {
			c.errorf(x.Pos(), "%s is a type, not a value; to convert, call %s(...)", x.Name, x.Name)
		} else {
			c.errorf(x.Pos(), "%s is a type, not a value", x.Name)
		}
	case *GenericSym:
		c.errorf(x.Pos(), "%s is a type, not a value", x.Name)
	case *BuiltinSym:
		if s.ID == BuiltinNone {
			return c.none(x, want)
		}
		c.errorf(x.Pos(), "%s is a built-in function and must be called", x.Name)
	}
	return Invalid
}

// capture enforces the closure rule: a closure may capture let values and
// parameters, and nothing that can change. Shared mutable state goes
// through a Ref.
func (c *checker) capture(x *ast.Ident, v *Var) {
	if v.fn == c.fn || v.fn == nil {
		return
	}
	switch v.Kind {
	case VarVar:
		c.reportCapture(x, v, "a closure cannot capture var %s; copy it into a let first, or share it with a Ref", v.Name)
	case VarMutParam:
		c.reportCapture(x, v, "a closure cannot capture mut parameter %s", v.Name)
	case VarMutSelf:
		c.reportCapture(x, v, "a closure cannot capture self in a mut self method")
	}
}

func (c *checker) reportCapture(x *ast.Ident, v *Var, format string, args ...any) {
	for ctx := c.fn; ctx != nil && ctx != v.fn; ctx = ctx.parent {
		if !ctx.reported[v] {
			ctx.reported[v] = true
			c.errorf(x.Pos(), format, args...)
			return
		}
	}
}

// mutExpr checks `mut X` outside an argument list, where the only valid use
// is marking the receiver of a mut self method call.
func (c *checker) mutExpr(x *ast.MutExpr, want Type) Type {
	if call, ok := x.X.(*ast.CallExpr); ok {
		if _, ok := call.Fun.(*ast.SelectorExpr); ok {
			c.markedCalls[call] = true
			t := c.expr(call, want)
			c.info.Types[x] = t
			return t
		}
	}
	c.errorf(x.Pos(), "mut can only mark an argument to a mut parameter or a call to a mut self method")
	return c.expr(x.X, want)
}

// ---------------------------------------------------------------------------
// Operators

func (c *checker) unary(x *ast.UnaryExpr, want Type) Type {
	if x.Op == token.NOT {
		c.assignTo(x.X, TBool)
		return TBool
	}
	if _, ok := intConstValue(x); ok {
		return c.intConst(x, want)
	}
	var t Type
	if needsContext(x.X) {
		t = c.expr(x.X, want)
	} else {
		t = c.expr(x.X, nil)
	}
	if bad(t) {
		return Invalid
	}
	b, ok := t.(*Basic)
	if !ok || !(b.IsFloat() || b.IsSigned()) {
		c.errorf(x.Pos(), "operator - is not defined for %s", t)
		return Invalid
	}
	return t
}

func (c *checker) binary(x *ast.BinaryExpr, want Type) Type {
	switch x.Op {
	case token.AND, token.OR:
		c.assignTo(x.X, TBool)
		c.assignTo(x.Y, TBool)
		return TBool
	case token.DOTDOT:
		c.errorf(x.OpPos, "ranges can only be used in for loops")
		c.expr(x.X, Invalid)
		c.expr(x.Y, Invalid)
		return Invalid
	case token.EQ, token.NEQ:
		tx, ty := c.operands(x.X, x.Y, nil)
		if bad(tx) || bad(ty) {
			return TBool
		}
		if !Identical(tx, ty) {
			c.errorf(x.OpPos, "mismatched types %s and %s in comparison", tx, ty)
		} else if !Comparable(tx) {
			c.errorf(x.OpPos, "%s values cannot be compared with %s", tx, x.Op)
		}
		return TBool
	case token.LT, token.LTE, token.GT, token.GTE:
		tx, ty := c.operands(x.X, x.Y, nil)
		if bad(tx) || bad(ty) {
			return TBool
		}
		if !Identical(tx, ty) {
			c.errorf(x.OpPos, "mismatched types %s and %s in comparison", tx, ty)
		} else if !Ordered(tx) {
			c.errorf(x.OpPos, "%s values cannot be compared with %s", tx, x.Op)
		}
		return TBool
	}

	arithWant := want
	if b, ok := want.(*Basic); !ok || !(b.IsNumeric() || b == TStr) {
		arithWant = nil
		if want == Invalid {
			arithWant = Invalid
		}
	}
	tx, ty := c.operands(x.X, x.Y, arithWant)
	if bad(tx) || bad(ty) {
		return Invalid
	}
	if !Identical(tx, ty) {
		c.errorf(x.OpPos, "mismatched types %s and %s in %s", tx, ty, x.Op)
		return Invalid
	}
	b, ok := tx.(*Basic)
	valid := ok && b.IsNumeric()
	switch x.Op {
	case token.PLUS:
		valid = valid || tx == TStr
	case token.PERCENT:
		valid = ok && b.IsInteger()
	}
	if !valid {
		c.errorf(x.OpPos, "operator %s is not defined for %s", x.Op, tx)
		return Invalid
	}
	return tx
}

// isExpr checks `x is T` and, when allowed, declares the `as` binding in the
// current scope.
func (c *checker) isExpr(x *ast.IsExpr, allowBind bool) Type {
	t := c.expr(x.X, nil)
	target := c.resolveType(x.Type)
	if x.As != nil && !allowBind {
		c.errorf(x.As.Pos(), "as bindings are only allowed in if and while conditions")
	}
	ok := c.checkTypeTest(x.X, t, target, x.Type.Pos())
	if x.As != nil && allowBind {
		bound := Invalid
		if ok {
			bound = target
		}
		c.declareVar(x.As, bound, VarBinding)
	}
	return TBool
}

// checkTypeTest validates testing whether a value of type t holds target.
func (c *checker) checkTypeTest(x ast.Expr, t, target Type, pos token.Pos) bool {
	if bad(t) || target == Invalid {
		return false
	}
	iface, ok := t.(*Interface)
	if !ok {
		c.errorf(x.Pos(), "type tests need an interface value, but %s has type %s", describe(x), t)
		return false
	}
	switch target.(type) {
	case *Object, *Enum, *Basic:
	default:
		c.errorf(pos, "type tests need a concrete type, not %s", target)
		return false
	}
	if !Implements(target, iface) {
		c.errorf(pos, "%s does not implement %s, so %s can never hold one", target, iface, describe(x))
		return false
	}
	return true
}

func (c *checker) index(x *ast.IndexExpr) Type {
	t := c.expr(x.X, nil)
	if bad(t) {
		c.expr(x.Index, Invalid)
		return Invalid
	}
	switch u := t.(type) {
	case *Array:
		c.assignTo(x.Index, TInt64)
		return &Maybe{Elem: u.Elem}
	case *Map:
		c.assignTo(x.Index, u.Key)
		return &Maybe{Elem: u.Value}
	}
	c.errorf(x.Pos(), "cannot index %s (type %s)", describe(x.X), t)
	c.expr(x.Index, Invalid)
	return Invalid
}

// ---------------------------------------------------------------------------
// Functions, blocks and branches

func (c *checker) funcLit(x *ast.FuncLit) Type {
	f, _ := c.signature(x.Params, x.Result, false)
	c.funcBody(x.Params, f, nil, nil, x.Body, "function literal", x.Pos())
	return f
}

// valueBlock checks a block in value position. Its value is its last
// expression; a block that ends by leaving (return, panic, break,
// continue) has type Never.
func (c *checker) valueBlock(b *ast.Block, want Type) Type {
	c.openScope()
	defer c.closeScope()
	n := len(b.Stmts)
	if n == 0 {
		c.errorf(b.Pos(), "empty block produces no value")
		return Invalid
	}
	terminates := c.stmts(b.Stmts[:n-1])
	last := b.Stmts[n-1]
	if es, ok := last.(*ast.ExprStmt); ok && !terminates {
		var t Type
		if want != nil {
			t = c.assignTo(es.X, want)
		} else {
			t = c.expr(es.X, nil)
		}
		c.info.Types[b] = t
		return t
	}
	if c.stmt(last) || terminates {
		c.info.Types[b] = Never
		return Never
	}
	c.errorf(last.Pos(), "block produces no value: it must end with an expression")
	return Invalid
}

// valueOf checks a branch body (a block or an expression) in value
// position.
func (c *checker) valueOf(body ast.Expr, want Type) Type {
	var t Type
	switch {
	case isBlock(body):
		t = c.valueBlock(body.(*ast.Block), want)
	case want != nil:
		t = c.assignTo(body, want)
	default:
		t = c.expr(body, nil)
	}
	if t == Void {
		if want == nil {
			// With a wanted type, assignTo has already reported this.
			c.errorf(body.Pos(), "this branch produces no value")
		}
		return Invalid
	}
	return t
}

func isBlock(x ast.Expr) bool {
	_, ok := x.(*ast.Block)
	return ok
}

// branches unifies the values of the branches of an if or match. With a
// wanted type every branch must be assignable to it; without one, the first
// branch that produces a value sets the type for the rest.
type branches struct {
	want   Type
	result Type
	failed bool // a branch failed before any set the type
}

func (c *checker) branch(u *branches, body ast.Expr) {
	expect := u.want
	if expect == nil {
		expect = u.result
	}
	if expect == nil && u.failed {
		expect = Invalid
	}
	t := c.valueOf(body, expect)
	switch {
	case u.result != nil:
	case t == Invalid:
		u.failed = true
	case t != Never:
		u.result = t
	}
}

func (u *branches) final() Type {
	switch {
	case u.result == nil && u.failed:
		return Invalid
	case u.result == nil:
		return Never
	case u.want != nil:
		return u.want
	}
	return u.result
}

func (c *checker) ifValue(x *ast.IfExpr, want Type) Type {
	u := &branches{want: want}
	c.openScope()
	c.cond(x.Cond)
	c.branch(u, x.Then)
	c.closeScope()
	if x.Else == nil {
		c.errorf(x.Pos(), "if used as a value needs an else branch")
		return Invalid
	}
	c.branch(u, x.Else)
	return u.final()
}

func (c *checker) try(x *ast.TryExpr) Type {
	t := c.expr(x.X, nil)
	var inner Type = Invalid
	isResult := false
	switch u := t.(type) {
	case *Result:
		inner, isResult = u.Ok, true
	case *Maybe:
		inner = u.Elem
	case invalidType, neverType:
	default:
		c.errorf(x.X.Pos(), "try needs a Result or Maybe, but %s has type %s", describe(x.X), t)
	}
	if x.Else == nil {
		return inner
	}

	c.openScope()
	defer c.closeScope()
	if x.ErrName != nil {
		if !isResult && inner != Invalid {
			c.errorf(x.ErrName.Pos(), "None carries no error; remove |%s|", x.ErrName.Name)
		}
		c.declareVar(x.ErrName, c.errorIface, VarBinding)
	}
	if inner == Void {
		// A bare Result has no success value, so the fallback produces none.
		if b, ok := x.Else.(*ast.Block); ok {
			c.block(b)
		} else {
			c.exprStmt(x.Else)
		}
		return Void
	}
	c.valueOf(x.Else, inner)
	return inner
}
