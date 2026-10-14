package types

import (
	"sort"
	"strings"

	"stellar/internal/ast"
)

// call checks a call expression: a built-in, a conversion, a function or
// closure, a method, a static function, or an enum variant constructor.
func (c *checker) call(x *ast.CallExpr, want Type) Type {
	switch fun := x.Fun.(type) {
	case *ast.Ident:
		switch s := c.scope.lookup(fun.Name).(type) {
		case *BuiltinSym:
			c.info.Uses[fun] = s
			return c.builtin(x, s, want)
		case *GenericSym:
			c.info.Uses[fun] = s
			if s.Name == "Ref" {
				return c.refCall(x, want)
			}
			c.errorf(fun.Pos(), "%s is a type, not a function", fun.Name)
			c.skipArgs(x)
			return Invalid
		case *TypeSym:
			c.info.Uses[fun] = s
			if b, ok := s.Type.(*Basic); ok {
				return c.conversion(x, b)
			}
			if _, ok := s.Type.(*Object); ok {
				c.errorf(fun.Pos(), "%s is a type; construct it with %s{...}", fun.Name, fun.Name)
			} else {
				c.errorf(fun.Pos(), "%s is a type, not a function", fun.Name)
			}
			c.skipArgs(x)
			return Invalid
		}
	case *ast.SelectorExpr:
		if ts := c.typeQualifier(fun.X); ts != nil {
			c.unusedMark(x, fun)
			return c.typeMember(fun, ts, want, x)
		}
		return c.methodOrFieldCall(x, fun)
	}

	c.unusedMark(x, x.Fun)
	ft := c.expr(x.Fun, nil)
	f, ok := ft.(*Func)
	if !ok {
		if !bad(ft) {
			c.errorf(x.Pos(), "cannot call %s (type %s)", describe(x.Fun), ft)
		}
		c.skipArgs(x)
		return Invalid
	}
	return c.callArgs(x, f, describe(x.Fun), nil)
}

// methodOrFieldCall checks value.name(args): a method call, or a call of a
// function stored in a field. A method taking mut self changes its
// receiver, so the call must be marked, `mut b.retitle(...)`, and the
// receiver must be a writable place.
func (c *checker) methodOrFieldCall(x *ast.CallExpr, fun *ast.SelectorExpr) Type {
	marked := c.markedCalls[x]
	var recv place
	var recvT Type
	if marked {
		p, ok := c.place(fun.X)
		if r, isRef := p.t.(*Ref); isRef {
			// The receiver is the Ref's target, which is always writable.
			p = place{t: r.Elem}
		}
		if !ok && !(p.t == Invalid && isIdent(fun.X)) {
			c.errorf(fun.X.Pos(), "%s is not a variable, field or element, so its methods cannot change it; store it in a var first", describe(fun.X))
		}
		recv, recvT = p, p.t
		if r, isRef := c.info.Types[fun.X].(*Ref); isRef && ok {
			recvT = r
		}
		if !ok {
			recvT = Invalid
		}
	} else {
		recvT = c.expr(fun.X, nil)
	}

	ft := c.valueMember(fun, recvT, true)
	c.info.Types[fun] = ft
	f, ok := ft.(*Func)
	if !ok {
		if !bad(ft) {
			c.errorf(x.Pos(), "cannot call %s (type %s)", describe(fun), ft)
		}
		c.skipArgs(x)
		return Invalid
	}

	sel := c.info.Selections[fun]
	mutSelf := sel != nil && sel.Kind == SelMethod && sel.Method.Self.Mut
	var recvPlace *place
	switch {
	case mutSelf && !marked:
		c.errorf(fun.Pos(), "%s changes %s; mark the call: mut %s(...)", fun.Sel.Name, describe(fun.X), describe(fun))
	case mutSelf:
		c.requireWritable(recv, fun.X.Pos(), "call "+fun.Sel.Name+" on "+describe(fun.X))
		recvPlace = &recv
	case marked && sel != nil && sel.Kind == SelMethod:
		c.errorf(fun.Pos(), "%s does not change %s; remove mut", fun.Sel.Name, describe(fun.X))
	case marked:
		c.errorf(fun.Pos(), "mut can only mark a call to a mut self method")
	}
	return c.callArgs(x, f, describe(fun), recvPlace)
}

// unusedMark reports `mut f(...)` where the call has no receiver to mark.
func (c *checker) unusedMark(x *ast.CallExpr, fun ast.Expr) {
	if c.markedCalls[x] {
		c.errorf(fun.Pos(), "mut can only mark a call to a mut self method")
	}
}

// skipArgs checks arguments whose parameter types are unknown, without
// reporting missing-context errors for literals.
func (c *checker) skipArgs(x *ast.CallExpr) {
	for _, a := range x.Args {
		c.expr(a.Value, Invalid)
	}
}

// noNamedArgs reports named arguments, which only variant constructors take.
func (c *checker) noNamedArgs(x *ast.CallExpr) {
	for _, a := range x.Args {
		if a.Name != nil {
			c.errorf(a.Name.Pos(), "named arguments are only allowed when constructing enum variants")
		}
	}
}

// callArgs checks a call's arguments against f's parameters. A mut
// parameter needs an argument marked `mut` that is a writable place, and no
// two mut places in one call (including a mut receiver, recv) may overlap.
func (c *checker) callArgs(x *ast.CallExpr, f *Func, name string, recv *place) Type {
	c.noNamedArgs(x)
	if len(x.Args) != len(f.Params) {
		c.errorf(x.Pos(), "wrong number of arguments in call to %s: want %d, got %d", name, len(f.Params), len(x.Args))
	}
	var places []place
	if recv != nil {
		places = append(places, *recv)
	}
	for i, a := range x.Args {
		if i >= len(f.Params) {
			c.expr(a.Value, Invalid)
			continue
		}
		m, isMut := f.Params[i].(*Mut)
		if !isMut {
			c.assignTo(a.Value, f.Params[i])
			continue
		}
		p, ok := c.mutParamArg(a.Value, m.Elem, name, i)
		if !ok {
			continue
		}
		for _, q := range places {
			if overlaps(p, q) {
				c.errorf(a.Value.Pos(), "%s overlaps another mut argument of this call", describe(a.Value))
				break
			}
		}
		places = append(places, p)
	}
	return f.Result
}

// mutParamArg checks the argument for a mut parameter of type want.
func (c *checker) mutParamArg(arg ast.Expr, want Type, name string, i int) (place, bool) {
	me, marked := arg.(*ast.MutExpr)
	if !marked {
		c.errorf(arg.Pos(), "%s may change argument %d; mark it: mut %s", name, i+1, describe(arg))
		c.assignTo(arg, want)
		return place{}, false
	}
	p, ok := c.mutArg(me)
	if ok && !Identical(p.t, want) {
		c.errorf(me.X.Pos(), "cannot pass %s (type %s) as mut %s: a mut argument must have exactly the parameter's type", describe(me.X), p.t, want)
		return p, false
	}
	return p, ok
}

// arity checks a built-in's argument count.
func (c *checker) arity(x *ast.CallExpr, name string, n int) bool {
	c.noNamedArgs(x)
	if len(x.Args) == n {
		return true
	}
	c.errorf(x.Pos(), "%s takes %d %s, got %d", name, n, plural(n, "argument"), len(x.Args))
	c.skipArgs(x)
	return false
}

// ---------------------------------------------------------------------------
// Members

// typeQualifier returns the type named by x when x is a type name used as
// the left side of a selector, as in Book.new or Shape.Circle.
func (c *checker) typeQualifier(x ast.Expr) *TypeSym {
	id, ok := x.(*ast.Ident)
	if !ok {
		return nil
	}
	ts, ok := c.scope.lookup(id.Name).(*TypeSym)
	if ok {
		c.info.Uses[id] = ts
	}
	return ts
}

// typeMember checks Type.name, called (call != nil) or used as a value.
func (c *checker) typeMember(sel *ast.SelectorExpr, ts *TypeSym, want Type, call *ast.CallExpr) Type {
	name := sel.Sel.Name
	switch t := ts.Type.(type) {
	case *Enum:
		if v := t.Variant(name); v != nil {
			c.info.Selections[sel] = &Selection{Kind: SelVariant, Variant: v}
			var res Type = t
			c.info.Types[sel] = res
			switch {
			case call == nil && v.Fields != nil:
				c.errorf(sel.Pos(), "%s.%s has fields; construct it with %s.%s(%s: ...)", t, name, t, name, v.Fields[0].Name)
				return Invalid
			case call != nil && v.Fields == nil:
				c.errorf(call.Pos(), "%s.%s has no fields; remove the ()", t, name)
				c.skipArgs(call)
			case call != nil:
				c.variantArgs(call, v, res)
			}
			return res
		}
	case *Interface:
		im := t.Method(name)
		if im == nil {
			c.errorf(sel.Sel.Pos(), "%s has no method %s", t, name)
			c.skipCall(call)
			return Invalid
		}
		sig := Subst(im.Sig, t).(*Func)
		m := &Method{Name: name, Owner: t, Interface: t, Self: &SelfParam{Mut: im.MutSelf}, Sig: sig}
		c.info.Selections[sel] = &Selection{Kind: SelMethodExpr, Method: m}
		return c.finishMember(sel, methodExpr(m), call)
	}

	m, ambiguous := c.lookupMethod(ts.Type, name)
	if len(ambiguous) > 0 {
		c.ambiguous(sel, ambiguous, describe(sel.X))
		c.skipCall(call)
		return Invalid
	}
	if m == nil {
		what := "function or method"
		if isEnum(ts.Type) {
			what = "variant, function or method"
		}
		c.errorf(sel.Sel.Pos(), "%s has no %s named %s", ts.Type, what, name)
		c.skipCall(call)
		return Invalid
	}
	if m.Self == nil {
		c.info.Selections[sel] = &Selection{Kind: SelStatic, Method: m}
		return c.finishMember(sel, m.Sig, call)
	}
	c.info.Selections[sel] = &Selection{Kind: SelMethodExpr, Method: m}
	return c.finishMember(sel, methodExpr(m), call)
}

func (c *checker) skipCall(call *ast.CallExpr) {
	if call != nil {
		c.skipArgs(call)
	}
}

// methodExpr is the function type of Type.method: the receiver becomes the
// first parameter.
func methodExpr(m *Method) *Func {
	recv := m.Owner
	if m.Self.Mut {
		recv = &Mut{Elem: recv}
	}
	return &Func{Params: append([]Type{recv}, m.Sig.Params...), Result: m.Sig.Result}
}

func (c *checker) finishMember(sel *ast.SelectorExpr, f *Func, call *ast.CallExpr) Type {
	c.info.Types[sel] = f
	if call == nil {
		return f
	}
	return c.callArgs(call, f, describe(sel), nil)
}

// valueMember checks value.name: a field, or a method bound to the value.
// forCall is set when the member is called immediately.
func (c *checker) valueMember(sel *ast.SelectorExpr, recv Type, forCall bool) Type {
	if bad(recv) {
		return Invalid
	}
	if r, ok := recv.(*Ref); ok {
		if sel.Sel.Name == "value" {
			c.info.Selections[sel] = &Selection{Kind: SelRefValue}
			return r.Elem
		}
		t := c.valueMember(sel, r.Elem, forCall)
		if s := c.info.Selections[sel]; s != nil {
			s.Deref = true
		}
		return t
	}
	u := recv
	name := sel.Sel.Name
	if obj, ok := u.(*Object); ok {
		if f := obj.Field(name); f != nil {
			c.info.Selections[sel] = &Selection{Kind: SelField, Field: f}
			return f.Type
		}
	}
	m, ambiguous := c.lookupMethod(u, name)
	if len(ambiguous) > 0 {
		c.ambiguous(sel, ambiguous, describe(sel.X))
		return Invalid
	}
	if m == nil {
		switch u.(type) {
		case *Object:
			c.errorf(sel.Sel.Pos(), "%s has no field or method %s", u, name)
		case *Enum:
			c.errorf(sel.Sel.Pos(), "%s has no method %s; read variant fields with match", u, name)
		default:
			c.errorf(sel.Sel.Pos(), "%s has no method %s", u, name)
		}
		return Invalid
	}
	if m.Self == nil {
		c.errorf(sel.Sel.Pos(), "%s is a static function; call it as %s.%s(...)", name, m.Owner, name)
		return Invalid
	}
	if m.Self.Mut && !forCall {
		c.errorf(sel.Pos(), "%s changes its receiver, so it cannot be used as a value; call it with mut %s(...) or use %s.%s", name, describe(sel), m.Owner, name)
		return Invalid
	}
	c.info.Selections[sel] = &Selection{Kind: SelMethod, Method: m}
	return &Func{Params: m.Sig.Params, Result: m.Sig.Result}
}

// lookupMethod finds a method of t by name: an inherent method first, then
// methods of implemented interfaces. If several interfaces provide the
// name, it returns them instead.
func (c *checker) lookupMethod(t Type, name string) (*Method, []*Interface) {
	var inherent map[string]*Method
	var impls map[*Interface]map[string]*Method
	switch t := t.(type) {
	case *Interface:
		im := t.Method(name)
		if im == nil {
			return nil, nil
		}
		return &Method{Name: name, Owner: t, Interface: t, Self: &SelfParam{Mut: im.MutSelf}, Sig: Subst(im.Sig, t).(*Func)}, nil
	case *Object:
		inherent, impls = t.Methods, t.Impls
	case *Enum:
		inherent, impls = t.Methods, t.Impls
	case *Basic:
		impls = c.basicMethods[t]
	}
	if m := inherent[name]; m != nil {
		return m, nil
	}
	var found []*Method
	for _, ms := range impls {
		if m := ms[name]; m != nil {
			found = append(found, m)
		}
	}
	switch len(found) {
	case 0:
		return nil, nil
	case 1:
		return found[0], nil
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Interface.Name < found[j].Interface.Name })
	ifaces := make([]*Interface, len(found))
	for i, m := range found {
		ifaces[i] = m.Interface
	}
	return nil, ifaces
}

func (c *checker) ambiguous(sel *ast.SelectorExpr, ifaces []*Interface, recv string) {
	names := make([]string, len(ifaces))
	for i, iface := range ifaces {
		names[i] = iface.Name
	}
	c.errorf(sel.Sel.Pos(), "%s is ambiguous: it is declared by %s; call %s.%s(%s, ...) instead",
		sel.Sel.Name, strings.Join(names, " and "), names[0], sel.Sel.Name, recv)
}

// variantArgs checks the named arguments of an enum variant constructor.
func (c *checker) variantArgs(x *ast.CallExpr, v *Variant, enumT Type) {
	seen := map[string]bool{}
	for _, a := range x.Args {
		if a.Name == nil {
			c.errorf(a.Value.Pos(), "variant fields must be named: %s.%s(%s: ...)", v.Enum, v.Name, v.Fields[0].Name)
			c.expr(a.Value, Invalid)
			continue
		}
		f := v.Field(a.Name.Name)
		switch {
		case f == nil:
			c.errorf(a.Name.Pos(), "%s.%s has no field %s", v.Enum, v.Name, a.Name.Name)
			c.expr(a.Value, Invalid)
			continue
		case seen[f.Name]:
			c.errorf(a.Name.Pos(), "duplicate field %s", f.Name)
		}
		seen[f.Name] = true
		c.assignTo(a.Value, f.Type)
	}
	var missing []string
	for _, f := range v.Fields {
		if !seen[f.Name] {
			missing = append(missing, f.Name)
		}
	}
	if len(missing) > 0 {
		c.errorf(x.Pos(), "missing %s %s in %s.%s", plural(len(missing), "field"), strings.Join(missing, ", "), v.Enum, v.Name)
	}
}

// ---------------------------------------------------------------------------
// Conversions and built-ins

// conversion checks an explicit conversion such as int64(x) or str(x).
func (c *checker) conversion(x *ast.CallExpr, to *Basic) Type {
	if !c.arity(x, to.Name, 1) {
		return to
	}
	arg := x.Args[0].Value
	if needsContext(arg) {
		c.expr(arg, to)
		return to
	}
	t := c.expr(arg, nil)
	if bad(t) {
		return to
	}
	from, ok := t.(*Basic)
	switch {
	case !ok:
		c.errorf(arg.Pos(), "cannot convert %s (type %s) to %s", describe(arg), t, to)
	case from == to || to == TStr:
	case from == TStr:
		c.errorf(arg.Pos(), "cannot convert str to %s; parsing strings is not supported yet", to)
	case from.IsNumeric() && to.IsNumeric():
	default:
		c.errorf(arg.Pos(), "cannot convert %s (type %s) to %s", describe(arg), t, to)
	}
	return to
}

func (c *checker) builtin(x *ast.CallExpr, b *BuiltinSym, want Type) Type {
	switch b.ID {
	case BuiltinPrint, BuiltinPanic:
		if c.arity(x, b.Name, 1) {
			c.assignTo(x.Args[0].Value, TStr)
		}
		if b.ID == BuiltinPanic {
			return Never
		}
		return Void

	case BuiltinLen:
		if !c.arity(x, b.Name, 1) {
			return TInt64
		}
		t := c.expr(x.Args[0].Value, nil)
		switch t.(type) {
		case *Array, *Map, invalidType, neverType:
		default:
			if t != TStr {
				c.errorf(x.Args[0].Value.Pos(), "len needs an array, map or str, not %s", t)
			}
		}
		return TInt64

	case BuiltinPush:
		if !c.arity(x, b.Name, 2) {
			return Void
		}
		arg := x.Args[0].Value
		me, marked := arg.(*ast.MutExpr)
		if !marked {
			c.errorf(arg.Pos(), "push changes its first argument; mark it: mut %s", describe(arg))
			c.expr(arg, nil)
			c.expr(x.Args[1].Value, Invalid)
			return Void
		}
		p, ok := c.mutArg(me)
		arr, isArr := p.t.(*Array)
		switch {
		case !ok:
			c.expr(x.Args[1].Value, Invalid)
		case !isArr:
			c.errorf(me.X.Pos(), "push needs an array, not %s", p.t)
			c.expr(x.Args[1].Value, Invalid)
		default:
			c.assignTo(x.Args[1].Value, arr.Elem)
		}
		return Void

	case BuiltinWrap:
		if c.arity(x, b.Name, 2) {
			c.assignTo(x.Args[0].Value, c.errorIface)
			c.assignTo(x.Args[1].Value, TStr)
		}
		return c.errorIface

	case BuiltinCause:
		if c.arity(x, b.Name, 1) {
			c.assignTo(x.Args[0].Value, c.errorIface)
		}
		return &Maybe{Elem: c.errorIface}

	case BuiltinOk:
		return c.okOrSome(x, b.Name, want)
	case BuiltinSome:
		return c.okOrSome(x, b.Name, want)
	case BuiltinErr:
		return c.errCall(x, want)
	case BuiltinNone:
		c.errorf(x.Pos(), "None is a value, not a function; remove the ()")
		c.skipArgs(x)
		return Invalid
	}
	return Invalid
}

// okOrSome checks Ok(...) or Some(...), taking the wrapped type from the
// context when there is one and from the argument otherwise.
func (c *checker) okOrSome(x *ast.CallExpr, name string, want Type) Type {
	c.noNamedArgs(x)
	if name == "Ok" {
		if r, ok := want.(*Result); ok {
			if r.Ok == Void {
				if len(x.Args) != 0 {
					c.errorf(x.Pos(), "Ok takes no value here: the function returns a bare Result")
					c.skipArgs(x)
				}
				return r
			}
			if c.arity(x, name, 1) {
				c.assignTo(x.Args[0].Value, r.Ok)
			}
			return r
		}
		if len(x.Args) == 0 {
			return &Result{Ok: Void}
		}
	} else if m, ok := want.(*Maybe); ok {
		if c.arity(x, name, 1) {
			c.assignTo(x.Args[0].Value, m.Elem)
		}
		return m
	}

	if !c.arity(x, name, 1) {
		return Invalid
	}
	t := c.expr(x.Args[0].Value, contextOrNil(want))
	if bad(t) {
		return Invalid
	}
	if t == Void {
		c.errorf(x.Args[0].Value.Pos(), "%s produces no value", describe(x.Args[0].Value))
		return Invalid
	}
	if name == "Ok" {
		return &Result{Ok: t}
	}
	return &Maybe{Elem: t}
}

// contextOrNil passes Invalid through so literals stay silent after an
// earlier error, and drops any other context.
func contextOrNil(want Type) Type {
	if want == Invalid {
		return Invalid
	}
	return nil
}

func (c *checker) errCall(x *ast.CallExpr, want Type) Type {
	if c.arity(x, "Err", 1) {
		arg := x.Args[0].Value
		if needsContext(arg) {
			c.expr(arg, TStr)
		} else if t := c.expr(arg, nil); !bad(t) && t != TStr && !Assignable(t, c.errorIface) {
			c.errorf(arg.Pos(), "Err needs a str message or an Error, not %s", t)
		}
	}
	switch w := want.(type) {
	case *Result:
		return w
	case nil:
		c.errorf(x.Pos(), "cannot infer the Result type of Err(...); add a type annotation")
	case invalidType:
	default:
		c.errorf(x.Pos(), "cannot use Err(...) as %s", want)
	}
	return Invalid
}

func (c *checker) none(x *ast.Ident, want Type) Type {
	switch w := want.(type) {
	case *Maybe:
		return w
	case nil:
		c.errorf(x.Pos(), "cannot infer the type of None; add a type annotation")
	case invalidType:
	default:
		c.errorf(x.Pos(), "cannot use None as %s", want)
	}
	return Invalid
}

// refCall checks the constructor Ref(x).
func (c *checker) refCall(x *ast.CallExpr, want Type) Type {
	if r, ok := want.(*Ref); ok {
		if c.arity(x, "Ref", 1) {
			c.assignTo(x.Args[0].Value, r.Elem)
		}
		return r
	}
	if !c.arity(x, "Ref", 1) {
		return Invalid
	}
	t := c.expr(x.Args[0].Value, contextOrNil(want))
	if bad(t) {
		return Invalid
	}
	if t == Void {
		c.errorf(x.Args[0].Value.Pos(), "%s produces no value", describe(x.Args[0].Value))
		return Invalid
	}
	return &Ref{Elem: t}
}
