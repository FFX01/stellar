package interp

import (
	"fmt"
	"math"

	"stellar/internal/ast"
	"stellar/internal/token"
	"stellar/internal/types"
)

func (in *Interp) call(fr frame, x *ast.CallExpr) Value {
	switch fun := x.Fun.(type) {
	case *ast.Ident:
		switch s := in.info.Uses[fun].(type) {
		case *types.BuiltinSym:
			return in.builtin(fr, x, s)
		case *types.TypeSym:
			return in.convert(in.eval(fr, x.Args[0].Value), s.Type.(*types.Basic), x.Pos())
		case *types.GenericSym: // Ref(x)
			v := in.eval(fr, x.Args[0].Value)
			markShared(v)
			return &RefVal{V: v}
		}
	case *ast.SelectorExpr:
		if sel := in.info.Selections[fun]; sel != nil {
			switch sel.Kind {
			case types.SelVariant:
				return in.variant(fr, x, sel.Variant)
			case types.SelMethod:
				return in.methodCall(fr, x, fun, sel)
			}
		}
	}
	f := in.eval(fr, x.Fun).(*Func)
	ft := in.info.Types[x.Fun].(*types.Func)
	return in.invoke(f, nil, in.args(fr, x.Args, ft.Params), x.Pos())
}

// args evaluates call arguments. An argument for a mut parameter is a
// place; its current value is copied in, and invoke writes back to it.
func (in *Interp) args(fr frame, args []*ast.Arg, params []types.Type) []arg {
	out := make([]arg, len(args))
	for i, a := range args {
		if _, ok := params[i].(*types.Mut); ok {
			lv := in.lvalue(fr, a.Value.(*ast.MutExpr).X)
			out[i] = arg{v: lv.unique(), lv: lv}
			continue
		}
		v := in.eval(fr, a.Value)
		markShared(v)
		out[i] = arg{v: v}
	}
	return out
}

func (in *Interp) methodCall(fr frame, x *ast.CallExpr, fun *ast.SelectorExpr, sel *types.Selection) Value {
	m := sel.Method
	var self arg
	switch {
	case m.Self.Mut && sel.Deref:
		lv := refLV{r: in.eval(fr, fun.X).(*RefVal)}
		self = arg{v: lv.unique(), lv: lv}
	case m.Self.Mut:
		lv := in.lvalue(fr, fun.X)
		self = arg{v: lv.unique(), lv: lv}
	default:
		self = arg{v: in.target(fr, fun.X, sel)}
		markShared(self.v)
	}
	f := in.methodFor(m, self.v, x.Pos())
	args := in.args(fr, x.Args, in.info.Types[fun].(*types.Func).Params)
	if f == nil {
		return self.v.(*ErrorVal).Msg
	}
	return in.invoke(f, &self, args, x.Pos())
}

// dispatchExpr calls an interface method used as a function value, such as
// Sortable.sortKey(b): the first argument is the receiver.
func (in *Interp) dispatchExpr(m *types.Method, args []arg, call token.Pos) Value {
	self := args[0]
	f := in.methodFor(m, self.v, call)
	if f == nil {
		return self.v.(*ErrorVal).Msg
	}
	return in.invoke(f, &self, args[1:], call)
}

func (in *Interp) variant(fr frame, x *ast.CallExpr, v *types.Variant) Value {
	e := &EnumVal{Type: v.Enum, Variant: v, Fields: make([]Value, len(v.Fields))}
	for _, a := range x.Args {
		for i, f := range v.Fields {
			if f.Name == a.Name.Name {
				val := in.eval(fr, a.Value)
				markShared(val)
				e.Fields[i] = val
			}
		}
	}
	return e
}

// errorMessage returns the message of an Error value.
func (in *Interp) errorMessage(e Value, pos token.Pos) string {
	if ev, ok := e.(*ErrorVal); ok {
		return ev.Msg
	}
	impl := in.info.Error.Impls[runtimeType(e)]["message"]
	return in.invoke(in.funcs[impl.Decl], &arg{v: e}, nil, pos).(string)
}

// ---------------------------------------------------------------------------
// Built-ins

func (in *Interp) builtin(fr frame, x *ast.CallExpr, b *types.BuiltinSym) Value {
	switch b.ID {
	case types.BuiltinPrint:
		fmt.Fprintln(in.out, in.eval(fr, x.Args[0].Value).(string))
		return nil
	case types.BuiltinPanic:
		in.fail(x.Pos(), "%s", in.eval(fr, x.Args[0].Value).(string))
	case types.BuiltinLen:
		switch v := in.eval(fr, x.Args[0].Value).(type) {
		case *Array:
			return int64(len(v.Elems))
		case *MapVal:
			return int64(len(v.Vals))
		case string:
			return int64(len(v))
		}
	case types.BuiltinPush:
		lv := in.lvalue(fr, x.Args[0].Value.(*ast.MutExpr).X)
		v := in.eval(fr, x.Args[1].Value)
		markShared(v)
		arr := lv.unique().(*Array)
		arr.Elems = append(arr.Elems, v)
		return nil
	case types.BuiltinWrap:
		e := in.eval(fr, x.Args[0].Value)
		context := in.eval(fr, x.Args[1].Value).(string)
		markShared(e)
		return &ErrorVal{Msg: context + ": " + in.errorMessage(e, x.Pos()), Cause: e}
	case types.BuiltinCause:
		if ev, ok := in.eval(fr, x.Args[0].Value).(*ErrorVal); ok && ev.Cause != nil {
			return &Some{V: ev.Cause}
		}
		return None
	case types.BuiltinOk:
		if len(x.Args) == 0 {
			return &OkVal{}
		}
		return &OkVal{V: in.eval(fr, x.Args[0].Value)}
	case types.BuiltinSome:
		return &Some{V: in.eval(fr, x.Args[0].Value)}
	case types.BuiltinErr:
		v := in.eval(fr, x.Args[0].Value)
		if s, ok := v.(string); ok {
			return &ErrVal{E: &ErrorVal{Msg: s}}
		}
		return &ErrVal{E: v}
	}
	panic("interp: unexpected built-in " + b.Name)
}

// convert applies an explicit conversion such as int64(x) or str(x).
// Numeric conversions that do not fit panic rather than wrap.
func (in *Interp) convert(v Value, to *types.Basic, pos token.Pos) Value {
	if to == types.TStr {
		return formatPrimitive(v)
	}
	if to == types.TBool {
		return v
	}
	if f, ok := asFloat(v); ok {
		switch {
		case to.IsFloat() && to.Bits() == 32:
			return float32(f)
		case to.IsFloat():
			return f
		case to.IsSigned():
			lo, hi := signedRange(to)
			if !fitsFloat(f, float64(lo), float64(hi)) {
				in.fail(pos, "%s is out of range for %s", formatPrimitive(v), to)
			}
			return fromInt64(int64(f), to)
		default:
			if !fitsFloat(f, 0, float64(unsignedMax(to))) {
				in.fail(pos, "%s is out of range for %s", formatPrimitive(v), to)
			}
			return fromUint64(uint64(f), to)
		}
	}
	i, u, signed := asInt(v)
	switch {
	case to.IsFloat() && to.Bits() == 32 && signed:
		return float32(i)
	case to.IsFloat() && to.Bits() == 32:
		return float32(u)
	case to.IsFloat() && signed:
		return float64(i)
	case to.IsFloat():
		return float64(u)
	case to.IsSigned():
		lo, hi := signedRange(to)
		if signed && (i < lo || i > hi) || !signed && u > uint64(hi) {
			in.fail(pos, "%s is out of range for %s", formatPrimitive(v), to)
		}
		if !signed {
			i = int64(u)
		}
		return fromInt64(i, to)
	default:
		max := unsignedMax(to)
		if signed && (i < 0 || uint64(i) > max) || !signed && u > max {
			in.fail(pos, "%s is out of range for %s", formatPrimitive(v), to)
		}
		if signed {
			u = uint64(i)
		}
		return fromUint64(u, to)
	}
}

func signedRange(b *types.Basic) (int64, int64) {
	bits := b.Bits()
	if bits == 64 {
		return math.MinInt64, math.MaxInt64
	}
	return -(1 << (bits - 1)), 1<<(bits-1) - 1
}

func unsignedMax(b *types.Basic) uint64 {
	if b.Bits() == 64 {
		return math.MaxUint64
	}
	return 1<<b.Bits() - 1
}

// ---------------------------------------------------------------------------
// Match

// matchArm finds the arm matching v and binds its variables.
func (in *Interp) matchArm(fr frame, x *ast.MatchExpr, v Value) *ast.MatchArm {
	for _, arm := range x.Arms {
		if in.matches(fr, arm.Pattern, v) {
			return arm
		}
	}
	panic("interp: no match arm matched") // the checker guarantees exhaustiveness
}

func (in *Interp) bind(fr frame, id *ast.Ident, v Value) {
	if variable, ok := in.info.Defs[id].(*types.Var); ok {
		markShared(v)
		fr[variable] = &cell{v: v}
	}
}

func (in *Interp) matches(fr frame, p ast.Pattern, v Value) bool {
	switch p := p.(type) {
	case *ast.WildcardPattern:
		return true
	case *ast.TypePattern:
		if runtimeType(v) != in.info.TypeExprs[p.Type] {
			return false
		}
		if p.As != nil {
			in.bind(fr, p.As, v)
		}
		return true
	case *ast.VariantPattern:
		if variant := in.info.Variants[p]; variant != nil {
			e := v.(*EnumVal)
			if e.Variant != variant {
				return false
			}
			for _, b := range p.Bindings {
				name := b.Field
				if b.Name != nil {
					name = b.Name
				}
				for i, f := range variant.Fields {
					if f.Name == b.Field.Name {
						in.bind(fr, name, e.Fields[i])
					}
				}
			}
			return true
		}
		var payload Value
		matched := false
		switch r := v.(type) {
		case *OkVal:
			payload, matched = r.V, p.Name.(*ast.Ident).Name == "Ok"
		case *ErrVal:
			payload, matched = r.E, p.Name.(*ast.Ident).Name == "Err"
		case *Some:
			payload, matched = r.V, p.Name.(*ast.Ident).Name == "Some"
		case noneVal:
			matched = p.Name.(*ast.Ident).Name == "None"
		}
		if matched && len(p.Bindings) == 1 {
			in.bind(fr, p.Bindings[0].Field, payload)
		}
		return matched
	}
	return false
}

func (in *Interp) matchValue(fr frame, x *ast.MatchExpr) Value {
	arm := in.matchArm(fr, x, in.eval(fr, x.Subject))
	return in.branch(fr, arm.Body)
}

func (in *Interp) matchStmt(fr frame, x *ast.MatchExpr) ctrl {
	arm := in.matchArm(fr, x, in.eval(fr, x.Subject))
	if b, ok := arm.Body.(*ast.Block); ok {
		return in.execStmts(fr, b.Stmts)
	}
	return in.exec(fr, &ast.ExprStmt{X: arm.Body})
}
