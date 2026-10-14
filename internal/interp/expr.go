package interp

import (
	"math/big"
	"strconv"
	"strings"

	"stellar/internal/ast"
	"stellar/internal/token"
	"stellar/internal/types"
)

func (in *Interp) eval(fr frame, x ast.Expr) Value {
	switch x := x.(type) {
	case *ast.IntLit:
		return in.constant(x)
	case *ast.FloatLit:
		return in.constant(x)
	case *ast.StringLit:
		return x.Value
	case *ast.BoolLit:
		return x.Value
	case *ast.ParenExpr:
		return in.eval(fr, x.X)
	case *ast.Ident:
		return in.ident(fr, x)
	case *ast.UnaryExpr:
		if x.Op == token.NOT {
			return !in.eval(fr, x.X).(bool)
		}
		if _, ok := in.info.Types[x].(*types.Basic); ok {
			if c, ok := in.intConst(x); ok {
				return c
			}
		}
		return in.negate(in.eval(fr, x.X), x.Pos())
	case *ast.BinaryExpr:
		return in.binary(fr, x)
	case *ast.IsExpr:
		return runtimeType(in.eval(fr, x.X)) == in.info.TypeExprs[x.Type]
	case *ast.IndexExpr:
		return in.index(fr, x)
	case *ast.ArrayLit:
		arr := &Array{Elems: make([]Value, len(x.Elems))}
		for i, e := range x.Elems {
			v := in.eval(fr, e)
			markShared(v)
			arr.Elems[i] = v
		}
		return arr
	case *ast.MapLit:
		m := &MapVal{Vals: make(map[Value]Value, len(x.Entries))}
		for _, e := range x.Entries {
			k := in.eval(fr, e.Key)
			v := in.eval(fr, e.Value)
			markShared(v)
			if _, exists := m.Vals[k]; !exists {
				m.Keys = append(m.Keys, k)
			}
			m.Vals[k] = v
		}
		return m
	case *ast.ObjectLit:
		obj := in.info.Types[x].(*types.Object)
		o := &Object{Type: obj, Fields: make([]Value, len(obj.Fields))}
		for _, fi := range x.Fields {
			v := in.eval(fr, fi.Value)
			markShared(v)
			o.Fields[fieldIndex(obj, fi.Name.Name)] = v
		}
		return o
	case *ast.FuncLit:
		// Closures capture only let values and parameters, which never
		// change, so sharing the current cells is equivalent to copying
		// them. Copying the map keeps each loop iteration's cells distinct.
		captured := make(frame, len(fr))
		for k, c := range fr {
			captured[k] = c
		}
		return &Func{name: "function literal", params: x.Params, body: x.Body, captured: captured}
	case *ast.IfExpr:
		if in.cond(fr, x.Cond) {
			return in.valueBlock(fr, x.Then)
		}
		return in.branch(fr, x.Else)
	case *ast.MatchExpr:
		return in.matchValue(fr, x)
	case *ast.TryExpr:
		return in.try(fr, x)
	case *ast.CallExpr:
		return in.call(fr, x)
	case *ast.SelectorExpr:
		return in.selector(fr, x)
	case *ast.MutExpr:
		return in.eval(fr, x.X) // a marked method call
	case *ast.Block:
		return in.valueBlock(fr, x)
	}
	panic("interp: unexpected expression")
}

func (in *Interp) ident(fr frame, x *ast.Ident) Value {
	switch s := in.info.Uses[x].(type) {
	case *types.Var:
		return fr[s].v
	case *types.FuncSym:
		return in.funcs[s.Decl]
	case *types.BuiltinSym:
		if s.ID == types.BuiltinNone {
			return None
		}
	}
	panic("interp: unresolved identifier " + x.Name)
}

// ---------------------------------------------------------------------------
// Constants

// constant returns the value of a numeric literal, parsed once.
func (in *Interp) constant(x ast.Expr) Value {
	if v, ok := in.consts[x]; ok {
		return v
	}
	var v Value
	switch lit := x.(type) {
	case *ast.IntLit:
		v, _ = in.intConst(lit)
	case *ast.FloatLit:
		f, _ := strconv.ParseFloat(strings.ReplaceAll(lit.Raw, "_", ""), 64)
		if in.info.Types[x] == types.TFloat32 {
			v = float32(f)
		} else {
			v = f
		}
	}
	in.consts[x] = v
	return v
}

// intConst evaluates an integer literal, possibly negated, as its checked
// type. Negation is folded so that -128 fits in int8.
func (in *Interp) intConst(x ast.Expr) (Value, bool) {
	if v, ok := in.consts[x]; ok {
		return v, true
	}
	n, ok := intConstValue(x)
	if !ok {
		return nil, false
	}
	b := in.info.Types[x].(*types.Basic)
	var v Value
	if b.IsSigned() {
		v = fromInt64(n.Int64(), b)
	} else {
		v = fromUint64(n.Uint64(), b)
	}
	in.consts[x] = v
	return v, true
}

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

// fromInt64 converts n to the Go representation of b. n must fit.
func fromInt64(n int64, b *types.Basic) Value {
	switch b.Kind {
	case types.Int8:
		return int8(n)
	case types.Int16:
		return int16(n)
	case types.Int32:
		return int32(n)
	}
	return n
}

// fromUint64 converts n to the Go representation of b. n must fit.
func fromUint64(n uint64, b *types.Basic) Value {
	switch b.Kind {
	case types.Uint8:
		return uint8(n)
	case types.Uint16:
		return uint16(n)
	case types.Uint32:
		return uint32(n)
	}
	return n
}

// ---------------------------------------------------------------------------
// Operators

func (in *Interp) binary(fr frame, x *ast.BinaryExpr) Value {
	switch x.Op {
	case token.AND:
		return in.eval(fr, x.X).(bool) && in.eval(fr, x.Y).(bool)
	case token.OR:
		return in.eval(fr, x.X).(bool) || in.eval(fr, x.Y).(bool)
	}
	a, b := in.eval(fr, x.X), in.eval(fr, x.Y)
	switch x.Op {
	case token.EQ:
		return equal(a, b)
	case token.NEQ:
		return !equal(a, b)
	case token.LT:
		return compare(a, b) < 0
	case token.LTE:
		return compare(a, b) <= 0
	case token.GT:
		return compare(a, b) > 0
	case token.GTE:
		return compare(a, b) >= 0
	}
	return in.arith(x.Op, a, b, x.OpPos)
}

func (in *Interp) index(fr frame, x *ast.IndexExpr) Value {
	coll := in.eval(fr, x.X)
	key := in.eval(fr, x.Index)
	switch c := coll.(type) {
	case *Array:
		i := key.(int64)
		if i < 0 || i >= int64(len(c.Elems)) {
			return None
		}
		return &Some{V: c.Elems[i]}
	case *MapVal:
		if v, ok := c.Vals[key]; ok {
			return &Some{V: v}
		}
		return None
	}
	panic("interp: indexing a non-collection")
}

func (in *Interp) try(fr frame, x *ast.TryExpr) Value {
	v := in.eval(fr, x.X)
	var failure Value
	switch r := v.(type) {
	case *OkVal:
		return r.V
	case *Some:
		return r.V
	case *ErrVal:
		failure = r.E
	}
	if x.Else == nil {
		if failure != nil {
			in.fail(x.Pos(), "try failed: %s", in.errorMessage(failure, x.Pos()))
		}
		in.fail(x.Pos(), "try failed: no value")
	}
	if x.ErrName != nil {
		if variable, ok := in.info.Defs[x.ErrName].(*types.Var); ok {
			markShared(failure)
			fr[variable] = &cell{v: failure}
		}
	}
	if in.info.Types[x] == types.Void {
		// A bare Result: the fallback runs as a statement.
		if b, ok := x.Else.(*ast.Block); ok {
			if c := in.execStmts(fr, b.Stmts); c != ctrlNone {
				panic(ctrlSignal{kind: c, val: in.ret})
			}
			return nil
		}
		in.eval(fr, x.Else)
		return nil
	}
	return in.branch(fr, x.Else)
}

// ---------------------------------------------------------------------------
// Selectors

func fieldIndex(obj *types.Object, name string) int {
	for i, f := range obj.Fields {
		if f.Name == name {
			return i
		}
	}
	panic("interp: unknown field " + name)
}

// selector evaluates X.Sel outside a call.
func (in *Interp) selector(fr frame, x *ast.SelectorExpr) Value {
	sel := in.info.Selections[x]
	switch sel.Kind {
	case types.SelRefValue:
		return in.eval(fr, x.X).(*RefVal).V
	case types.SelField:
		o := in.target(fr, x.X, sel).(*Object)
		return o.Fields[fieldIndex(o.Type, sel.Field.Name)]
	case types.SelVariant:
		return &EnumVal{Type: sel.Variant.Enum, Variant: sel.Variant}
	case types.SelStatic:
		return in.funcs[sel.Method.Decl]
	case types.SelMethodExpr:
		if _, ok := sel.Method.Owner.(*types.Interface); ok {
			return &Func{name: methodName(sel.Method), dispatch: sel.Method}
		}
		return in.funcs[sel.Method.Decl]
	case types.SelMethod:
		recv := in.target(fr, x.X, sel)
		markShared(recv)
		f := in.methodFor(sel.Method, recv, x.Pos())
		if f == nil { // the built-in error type's message method
			return &Func{name: "message", recv: recv, bound: true, native: builtinMessage}
		}
		return &Func{name: f.name, params: f.params, body: f.body, recv: recv, bound: true}
	}
	panic("interp: unexpected selection")
}

// target evaluates the left side of a selector, reaching through a Ref
// when the selection does.
func (in *Interp) target(fr frame, x ast.Expr, sel *types.Selection) Value {
	v := in.eval(fr, x)
	if sel.Deref {
		return v.(*RefVal).V
	}
	return v
}

// methodFor finds the implementation of m for a receiver value: m itself
// for a concrete method, or the receiver's impl for an interface method.
// It returns nil for the built-in error type's message method.
func (in *Interp) methodFor(m *types.Method, recv Value, pos token.Pos) *Func {
	if m.Decl != nil {
		return in.funcs[m.Decl]
	}
	if _, ok := recv.(*ErrorVal); ok {
		return nil
	}
	impl := m.Interface.Impls[runtimeType(recv)][m.Name]
	return in.funcs[impl.Decl]
}

// ---------------------------------------------------------------------------
// Places

// lvalue is a place that can be written: a variable, or a field or element
// path starting from one.
type lvalue interface {
	get() Value
	set(Value)
	// unique returns the value at the place, first replacing it with an
	// unshared copy if necessary, so it can be changed in place.
	unique() Value
}

type varLV struct{ c *cell }

func (l varLV) get() Value  { return l.c.v }
func (l varLV) set(v Value) { l.c.v = v }
func (l varLV) unique() Value {
	l.c.v = unshare(l.c.v)
	return l.c.v
}

// refLV is the target of a Ref. It is reached through the Ref, not
// through the variable holding it, so it is writable even from a let.
type refLV struct{ r *RefVal }

func (l refLV) get() Value  { return l.r.V }
func (l refLV) set(v Value) { l.r.V = v }
func (l refLV) unique() Value {
	l.r.V = unshare(l.r.V)
	return l.r.V
}

type fieldLV struct {
	parent lvalue
	idx    int
}

func (l fieldLV) get() Value  { return l.parent.get().(*Object).Fields[l.idx] }
func (l fieldLV) set(v Value) { l.parent.unique().(*Object).Fields[l.idx] = v }
func (l fieldLV) unique() Value {
	o := l.parent.unique().(*Object)
	o.Fields[l.idx] = unshare(o.Fields[l.idx])
	return o.Fields[l.idx]
}

type indexLV struct {
	in     *Interp
	parent lvalue
	key    Value
	pos    token.Pos
}

func (l indexLV) get() Value {
	switch c := l.parent.get().(type) {
	case *Array:
		return c.Elems[l.arrayIndex(c)]
	case *MapVal:
		return l.mapValue(c)
	}
	panic("interp: indexing a non-collection")
}

func (l indexLV) set(v Value) {
	switch c := l.parent.unique().(type) {
	case *Array:
		c.Elems[l.arrayIndex(c)] = v
	case *MapVal:
		if _, ok := c.Vals[l.key]; !ok {
			c.Keys = append(c.Keys, l.key)
		}
		c.Vals[l.key] = v
	}
}

func (l indexLV) unique() Value {
	switch c := l.parent.unique().(type) {
	case *Array:
		i := l.arrayIndex(c)
		c.Elems[i] = unshare(c.Elems[i])
		return c.Elems[i]
	case *MapVal:
		v := unshare(l.mapValue(c))
		c.Vals[l.key] = v
		return v
	}
	panic("interp: indexing a non-collection")
}

func (l indexLV) arrayIndex(a *Array) int64 {
	i := l.key.(int64)
	if i < 0 || i >= int64(len(a.Elems)) {
		l.in.fail(l.pos, "index %d out of range for array of length %d", i, len(a.Elems))
	}
	return i
}

func (l indexLV) mapValue(m *MapVal) Value {
	v, ok := m.Vals[l.key]
	if !ok {
		l.in.fail(l.pos, "key %s not found in map", formatPrimitive(l.key))
	}
	return v
}

// lvalue evaluates a place expression. Index expressions are evaluated
// once, here.
func (in *Interp) lvalue(fr frame, x ast.Expr) lvalue {
	switch x := x.(type) {
	case *ast.ParenExpr:
		return in.lvalue(fr, x.X)
	case *ast.Ident:
		return varLV{c: fr[in.info.Uses[x].(*types.Var)]}
	case *ast.SelectorExpr:
		sel := in.info.Selections[x]
		if sel.Kind == types.SelRefValue {
			return refLV{r: in.eval(fr, x.X).(*RefVal)}
		}
		var parent lvalue
		if sel.Deref {
			parent = refLV{r: in.eval(fr, x.X).(*RefVal)}
		} else {
			parent = in.lvalue(fr, x.X)
		}
		return fieldLV{parent: parent, idx: fieldIndexOf(parent, sel.Field.Name)}
	case *ast.IndexExpr:
		parent := in.lvalue(fr, x.X)
		return indexLV{in: in, parent: parent, key: in.eval(fr, x.Index), pos: x.Pos()}
	}
	panic("interp: not a place")
}

func builtinMessage(recv Value) Value { return recv.(*ErrorVal).Msg }

// fieldIndexOf finds a field's index in the object currently at a place.
func fieldIndexOf(parent lvalue, name string) int {
	return fieldIndex(parent.get().(*Object).Type, name)
}
