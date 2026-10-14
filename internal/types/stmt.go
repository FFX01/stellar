package types

import (
	"stellar/internal/ast"
	"stellar/internal/token"
)

// stmts checks a statement list and reports whether it terminates, that is,
// whether control can never fall off its end.
func (c *checker) stmts(list []ast.Stmt) bool {
	terminates := false
	for _, s := range list {
		if c.stmt(s) {
			terminates = true
		}
	}
	return terminates
}

// block checks a block in statement position.
func (c *checker) block(b *ast.Block) bool {
	c.openScope()
	defer c.closeScope()
	c.info.Types[b] = Void
	return c.stmts(b.Stmts)
}

func (c *checker) stmt(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.LetStmt:
		c.let(s)
	case *ast.AssignStmt:
		c.assignStmt(s)
	case *ast.ReturnStmt:
		c.returnStmt(s)
		return true
	case *ast.BreakStmt:
		c.loopOnly(s.Pos(), "break")
		return true
	case *ast.ContinueStmt:
		c.loopOnly(s.Pos(), "continue")
		return true
	case *ast.WhileStmt:
		c.openScope()
		c.cond(s.Cond)
		c.loopBody(s.Body)
		c.closeScope()
	case *ast.ForStmt:
		c.forStmt(s)
	case *ast.ExprStmt:
		return c.exprStmt(s.X)
	}
	return false
}

func (c *checker) loopOnly(pos token.Pos, what string) {
	if c.fn.loops == 0 {
		c.errorf(pos, "%s is only allowed inside a loop", what)
	}
}

func (c *checker) loopBody(b *ast.Block) {
	c.fn.loops++
	c.block(b)
	c.fn.loops--
}

func (c *checker) let(s *ast.LetStmt) {
	var t Type
	if s.Type != nil {
		want := c.resolveType(s.Type)
		c.assignTo(s.Value, want)
		t = want
	} else {
		t = c.expr(s.Value, nil)
	}
	if t == Void {
		c.errorf(s.Value.Pos(), "%s produces no value", describe(s.Value))
		t = Invalid
	}
	if s.Name.Name == "_" {
		if s.Var {
			c.errorf(s.Name.Pos(), "use let, not var, to discard a value")
		}
		if _, ok := t.(*Result); ok {
			c.errorf(s.Value.Pos(), "a Result cannot be discarded; handle it with try or match")
		}
		return
	}
	kind := VarLet
	if s.Var {
		kind = VarVar
	}
	c.declareVar(s.Name, t, kind)
}

func (c *checker) returnStmt(s *ast.ReturnStmt) {
	want := c.fn.result
	switch {
	case want == Void && s.Value != nil:
		c.errorf(s.Value.Pos(), "function has no result type, so return takes no value")
		c.expr(s.Value, Invalid)
	case want != Void && s.Value == nil:
		c.errorf(s.Pos(), "missing return value of type %s", want)
	case s.Value != nil:
		c.assignTo(s.Value, want)
	}
}

func (c *checker) assignStmt(s *ast.AssignStmt) {
	p, ok := c.place(s.Target)
	if !ok {
		if p.t != Invalid || !isIdent(s.Target) {
			c.errorf(s.Target.Pos(), "cannot assign to %s", describe(s.Target))
		}
		c.expr(s.Value, Invalid)
		return
	}
	c.requireWritable(p, s.Target.Pos(), "assign to "+describe(s.Target))
	c.assignTo(s.Value, p.t)
}

func isIdent(x ast.Expr) bool {
	_, ok := x.(*ast.Ident)
	return ok
}

func (c *checker) forStmt(s *ast.ForStmt) {
	c.openScope()
	defer c.closeScope()
	var elem Type = Invalid
	if r, ok := s.Iter.(*ast.BinaryExpr); ok && r.Op == token.DOTDOT {
		elem = c.rangeExpr(r)
	} else {
		t := c.expr(s.Iter, nil)
		switch u := t.(type) {
		case *Array:
			elem = u.Elem
		case *Map:
			elem = u.Key
		default:
			if !bad(t) {
				c.errorf(s.Iter.Pos(), "cannot iterate over %s (type %s)", describe(s.Iter), t)
			}
		}
	}
	c.declareVar(s.Var, elem, VarLoop)
	c.loopBody(s.Body)
}

func (c *checker) rangeExpr(r *ast.BinaryExpr) Type {
	tx, ty := c.operands(r.X, r.Y, nil)
	c.info.Types[r] = Invalid
	if bad(tx) || bad(ty) {
		return Invalid
	}
	b, ok := tx.(*Basic)
	if !ok || !b.IsInteger() || !Identical(tx, ty) {
		c.errorf(r.OpPos, "range bounds must be integers of the same type, not %s and %s", tx, ty)
		return Invalid
	}
	c.info.Types[r] = b
	return b
}

// exprStmt checks an expression used as a statement and applies the strict
// discard rule: a value may not be dropped implicitly.
func (c *checker) exprStmt(x ast.Expr) bool {
	switch e := x.(type) {
	case *ast.IfExpr:
		return c.ifStmt(e)
	case *ast.MatchExpr:
		_, terminates := c.match(e, nil, false)
		c.info.Types[e] = Void
		return terminates
	}
	t := c.expr(x, nil)
	switch t.(type) {
	case voidType, invalidType:
		return false
	case neverType:
		return true
	case *Result:
		c.errorf(x.Pos(), "unhandled Result from %s: handle it with try or match", describe(x))
	default:
		c.errorf(x.Pos(), "unused value of type %s: use let _ = ... to discard it", t)
	}
	return false
}

func (c *checker) ifStmt(x *ast.IfExpr) bool {
	c.info.Types[x] = Void
	c.openScope()
	c.cond(x.Cond)
	thenTerm := c.block(x.Then)
	c.closeScope()
	switch e := x.Else.(type) {
	case *ast.Block:
		return c.block(e) && thenTerm
	case *ast.IfExpr:
		return c.ifStmt(e) && thenTerm
	}
	return false
}

// cond checks a condition of an if or while. `x is T as name` bindings in
// the condition, or in an `and` chain, are declared in the current scope so
// later conjuncts and the body can use them.
func (c *checker) cond(x ast.Expr) {
	switch e := x.(type) {
	case *ast.IsExpr:
		c.info.Types[e] = c.isExpr(e, true)
		return
	case *ast.BinaryExpr:
		if e.Op == token.AND {
			c.cond(e.X)
			c.cond(e.Y)
			c.info.Types[e] = TBool
			return
		}
	}
	c.assignTo(x, TBool)
}
