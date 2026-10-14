package types

import (
	"stellar/internal/ast"
	"stellar/internal/token"
)

// A place is a location that can be written to: a variable, or a field or
// index path starting from one. Assignment targets, mut arguments and the
// receivers of mut self calls must be places.
type place struct {
	t    Type
	root *Var
	// path lists the fields and indexes from the root; "[]" stands for any
	// index, since indexes are not known until run time.
	path []string
}

// place checks x as a place. It reports false, without a diagnostic, when x
// is not of a place's shape (for example a call); callers explain why that
// matters in their context. Other problems are reported here.
func (c *checker) place(x ast.Expr) (place, bool) {
	switch e := x.(type) {
	case *ast.ParenExpr:
		p, ok := c.place(e.X)
		c.info.Types[e] = p.t
		return p, ok

	case *ast.Ident:
		sym := c.scope.lookup(e.Name)
		v, ok := sym.(*Var)
		if !ok {
			if sym == nil {
				c.errorf(e.Pos(), "undefined: %s", e.Name)
				return place{t: Invalid}, false
			}
			return place{}, false // a function or type name: not a place
		}
		c.info.Uses[e] = v
		c.info.Types[e] = v.Type
		c.capture(e, v)
		return place{t: v.Type, root: v}, true

	case *ast.SelectorExpr:
		if c.typeQualifier(e.X) != nil {
			return place{t: Invalid}, false
		}
		p, ok := c.place(e.X)
		if !ok || p.t == Invalid {
			return place{t: Invalid}, ok
		}
		if r, isRef := p.t.(*Ref); isRef {
			// A Ref's target is shared and always writable, whatever the
			// variable holding the Ref. Overlap through Refs cannot be
			// known statically, so the place starts a new root.
			if e.Sel.Name == "value" {
				c.info.Selections[e] = &Selection{Kind: SelRefValue}
				c.info.Types[e] = r.Elem
				return place{t: r.Elem}, true
			}
			p = place{t: r.Elem}
			defer func() {
				if s := c.info.Selections[e]; s != nil {
					s.Deref = true
				}
			}()
		}
		obj, _ := p.t.(*Object)
		var f *Field
		if obj != nil {
			f = obj.Field(e.Sel.Name)
		}
		if f == nil {
			c.errorf(e.Sel.Pos(), "%s has no field %s", p.t, e.Sel.Name)
			return place{t: Invalid}, true
		}
		c.info.Selections[e] = &Selection{Kind: SelField, Field: f}
		c.info.Types[e] = f.Type
		return place{t: f.Type, root: p.root, path: extend(p.path, f.Name)}, true

	case *ast.IndexExpr:
		p, ok := c.place(e.X)
		if !ok || p.t == Invalid {
			c.expr(e.Index, Invalid)
			return place{t: Invalid}, ok
		}
		var elem Type
		switch u := p.t.(type) {
		case *Array:
			c.assignTo(e.Index, TInt64)
			elem = u.Elem
		case *Map:
			c.assignTo(e.Index, u.Key)
			elem = u.Value
		default:
			c.errorf(e.Pos(), "cannot index %s (type %s)", describe(e.X), p.t)
			c.expr(e.Index, Invalid)
			return place{t: Invalid}, true
		}
		c.info.Types[e] = elem
		return place{t: elem, root: p.root, path: extend(p.path, "[]")}, true
	}
	return place{t: Invalid}, false
}

func extend(path []string, part string) []string {
	out := make([]string, len(path), len(path)+1)
	copy(out, path)
	return append(out, part)
}

// requireWritable reports an error if the place's root variable cannot
// change. action describes the attempted change, such as "assign to b.title".
func (c *checker) requireWritable(p place, at token.Pos, action string) {
	v := p.root
	if v == nil || v.Writable() || p.t == Invalid {
		return
	}
	switch v.Kind {
	case VarLet:
		c.errorf(at, "cannot %s: %s is declared with let; use var", action, v.Name)
	case VarParam:
		c.errorf(at, "cannot %s: parameter %s is not mut; declare it as %s: mut %s", action, v.Name, v.Name, v.Type)
	case VarSelf:
		c.errorf(at, "cannot %s: the method takes self; declare it with mut self", action)
	case VarLoop:
		c.errorf(at, "cannot %s: loop variable %s is a copy; index the collection instead", action, v.Name)
	default:
		c.errorf(at, "cannot %s: %s is a copy bound by a pattern", action, v.Name)
	}
}

// overlaps reports whether two places may refer to the same data: the same
// root, with one path a prefix of the other. Any two indexes may be equal.
func overlaps(a, b place) bool {
	if a.root == nil || a.root != b.root {
		return false
	}
	n := min(len(a.path), len(b.path))
	for i := 0; i < n; i++ {
		if a.path[i] != b.path[i] && a.path[i] != "[]" && b.path[i] != "[]" {
			return false
		}
	}
	return true
}

// mutArg checks an argument passed to a mut parameter or a mut receiver:
// it must be marked with mut and be a writable place.
func (c *checker) mutArg(me *ast.MutExpr) (place, bool) {
	p, ok := c.place(me.X)
	c.info.Types[me] = p.t
	if !ok {
		if p.t == Invalid && isIdent(me.X) {
			return p, false // already reported as undefined
		}
		c.errorf(me.X.Pos(), "%s is not a variable, field or element, so it cannot be passed as mut; store it in a var first", describe(me.X))
		return p, false
	}
	c.requireWritable(p, me.X.Pos(), "pass "+describe(me.X)+" as mut")
	return p, p.t != Invalid
}
