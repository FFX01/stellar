package interp

import (
	"fmt"
	"io"
	"strings"

	"stellar/internal/ast"
	"stellar/internal/token"
	"stellar/internal/types"
)

// maxDepth bounds recursion so runaway programs fail with a Stellar panic
// instead of exhausting the Go stack.
const maxDepth = 10000

// Interp runs one type-checked file.
type Interp struct {
	info   *types.Info
	out    io.Writer
	funcs  map[*ast.FuncDecl]*Func
	consts map[ast.Expr]Value
	stack  []stackEntry
	ret    Value // value of the return statement being executed
}

type stackEntry struct {
	name string
	call token.Pos // where this function was called from
}

// RuntimeError is a Stellar panic: a failed try, an explicit panic(),
// overflow, division by zero or an out-of-range write.
type RuntimeError struct {
	Msg   string
	Pos   token.Pos
	Trace []string // innermost first: "function (line:col)"
}

func (e *RuntimeError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "panic: %s", e.Msg)
	for _, t := range e.Trace {
		fmt.Fprintf(&b, "\n    at %s", t)
	}
	return b.String()
}

// MainError reports that main returned Err.
type MainError struct{ Msg string }

func (e *MainError) Error() string { return "error: " + e.Msg }

// Func is a function value: a declared function or method, a closure, or a
// method bound to its receiver.
type Func struct {
	name     string
	params   []*ast.Param
	body     *ast.Block
	captured frame
	recv     Value // the bound receiver, if bound
	bound    bool
	dispatch *types.Method     // an interface method used as a value
	native   func(Value) Value // a built-in method bound to recv
}

type cell struct{ v Value }

// frame holds the variables of one function call. Variables are keyed by
// the checker's *types.Var, which is unique per declaration, so nested
// blocks need no scopes of their own.
type frame map[*types.Var]*cell

type ctrl int

const (
	ctrlNone ctrl = iota
	ctrlBreak
	ctrlContinue
	ctrlReturn
)

// ctrlSignal carries break, continue or return out of a block that was
// evaluated as a value, such as `if c { 1 } else { return 0 }`.
type ctrlSignal struct {
	kind ctrl
	val  Value
}

// Run executes the program's main function, writing its output to out.
// It returns a *RuntimeError if the program panics and a *MainError if
// main returns Err.
func Run(file *ast.File, info *types.Info, out io.Writer) (err error) {
	in := &Interp{info: info, out: out, funcs: map[*ast.FuncDecl]*Func{}, consts: map[ast.Expr]Value{}}
	var main *ast.FuncDecl
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			in.funcs[d] = &Func{name: d.Name.Name, params: d.Params, body: d.Body}
			if d.Name.Name == "main" {
				main = d
			}
		case *ast.ImplDecl:
			for _, m := range d.Methods {
				in.funcs[m] = &Func{name: methodName(info.Methods[m]), params: m.Params, body: m.Body}
			}
		}
	}
	if main == nil {
		return fmt.Errorf("no main function")
	}

	defer func() {
		if r := recover(); r != nil {
			rerr, ok := r.(*RuntimeError)
			if !ok {
				panic(r)
			}
			err = rerr
		}
	}()
	result := in.invoke(in.funcs[main], nil, nil, token.Pos{})
	if e, ok := result.(*ErrVal); ok {
		return &MainError{Msg: in.errorMessage(e.E, token.Pos{})}
	}
	return nil
}

func methodName(m *types.Method) string {
	if m == nil {
		return "method"
	}
	return m.Owner.String() + "." + m.Name
}

// fail raises a Stellar panic at pos.
func (in *Interp) fail(pos token.Pos, format string, args ...any) {
	e := &RuntimeError{Msg: fmt.Sprintf(format, args...), Pos: pos}
	at := pos
	for i := len(in.stack) - 1; i >= 0; i-- {
		e.Trace = append(e.Trace, fmt.Sprintf("%s (%s)", in.stack[i].name, at))
		at = in.stack[i].call
	}
	panic(e)
}

func (in *Interp) overflow(pos token.Pos) {
	in.fail(pos, "integer overflow")
}

// ---------------------------------------------------------------------------
// Calls

// arg is an evaluated argument. For a mut parameter, lv is the caller's
// place: the value is copied in before the call and written back after it.
type arg struct {
	v  Value
	lv lvalue
}

// invoke calls f. self is the receiver of a method, if any.
func (in *Interp) invoke(f *Func, self *arg, args []arg, call token.Pos) Value {
	if f.dispatch != nil {
		return in.dispatchExpr(f.dispatch, args, call)
	}
	if f.native != nil {
		return f.native(f.recv)
	}
	if f.bound {
		self = &arg{v: f.recv}
	}
	if len(in.stack) >= maxDepth {
		in.fail(call, "stack overflow")
	}

	fr := make(frame, len(f.params)+len(f.captured))
	for k, c := range f.captured {
		fr[k] = c
	}
	var selfCell *cell
	cells := make([]*cell, len(args))
	i := 0
	for _, p := range f.params {
		c := &cell{}
		if p.IsSelf {
			if self != nil {
				c.v = self.v
			}
			selfCell = c
		} else if i < len(args) {
			c.v = args[i].v
			cells[i] = c
			i++
		}
		if v, ok := in.info.Defs[p.Name].(*types.Var); ok {
			fr[v] = c
		}
	}

	in.stack = append(in.stack, stackEntry{name: f.name, call: call})
	result := in.runBody(fr, f.body)
	in.stack = in.stack[:len(in.stack)-1]

	// Copy-out: mut arguments and a mut receiver write back to the caller.
	if self != nil && self.lv != nil && selfCell != nil {
		self.lv.set(selfCell.v)
	}
	for j, a := range args {
		if a.lv != nil && cells[j] != nil {
			a.lv.set(cells[j].v)
		}
	}
	return result
}

func (in *Interp) runBody(fr frame, body *ast.Block) (ret Value) {
	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(ctrlSignal); ok && s.kind == ctrlReturn {
				ret = s.val
				return
			}
			panic(r)
		}
	}()
	if in.execStmts(fr, body.Stmts) == ctrlReturn {
		return in.ret
	}
	return nil
}

// ---------------------------------------------------------------------------
// Statements

func (in *Interp) execStmts(fr frame, stmts []ast.Stmt) ctrl {
	for _, s := range stmts {
		if c := in.exec(fr, s); c != ctrlNone {
			return c
		}
	}
	return ctrlNone
}

func (in *Interp) exec(fr frame, s ast.Stmt) ctrl {
	switch s := s.(type) {
	case *ast.LetStmt:
		v := in.eval(fr, s.Value)
		if variable, ok := in.info.Defs[s.Name].(*types.Var); ok {
			markShared(v)
			fr[variable] = &cell{v: v}
		}
	case *ast.AssignStmt:
		lv := in.lvalue(fr, s.Target)
		v := in.eval(fr, s.Value)
		markShared(v)
		lv.set(v)
	case *ast.ReturnStmt:
		var v Value
		if s.Value != nil {
			v = in.eval(fr, s.Value)
			markShared(v)
		}
		in.ret = v
		return ctrlReturn
	case *ast.BreakStmt:
		return ctrlBreak
	case *ast.ContinueStmt:
		return ctrlContinue
	case *ast.WhileStmt:
		for in.cond(fr, s.Cond) {
			switch in.iterate(fr, s.Body) {
			case ctrlBreak:
				return ctrlNone
			case ctrlReturn:
				return ctrlReturn
			}
		}
	case *ast.ForStmt:
		return in.forStmt(fr, s)
	case *ast.ExprStmt:
		switch x := s.X.(type) {
		case *ast.IfExpr:
			return in.ifStmt(fr, x)
		case *ast.MatchExpr:
			return in.matchStmt(fr, x)
		}
		in.eval(fr, s.X)
	}
	return ctrlNone
}

// iterate runs one loop iteration, catching break and continue raised from
// blocks evaluated as values.
func (in *Interp) iterate(fr frame, body *ast.Block) (c ctrl) {
	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(ctrlSignal); ok && s.kind != ctrlReturn {
				c = s.kind
				return
			}
			panic(r)
		}
	}()
	return in.execStmts(fr, body.Stmts)
}

func (in *Interp) forStmt(fr frame, s *ast.ForStmt) ctrl {
	loopVar, _ := in.info.Defs[s.Var].(*types.Var)
	bind := func(v Value) {
		if loopVar != nil {
			markShared(v)
			fr[loopVar] = &cell{v: v}
		}
	}
	run := func() (stop bool, c ctrl) {
		switch c := in.iterate(fr, s.Body); c {
		case ctrlBreak:
			return true, ctrlNone
		case ctrlReturn:
			return true, ctrlReturn
		}
		return false, ctrlNone
	}

	if r, ok := s.Iter.(*ast.BinaryExpr); ok && r.Op == token.DOTDOT {
		lo, hi := in.eval(fr, r.X), in.eval(fr, r.Y)
		b := in.info.Types[r].(*types.Basic)
		if b.IsSigned() {
			l, _, _ := asInt(lo)
			h, _, _ := asInt(hi)
			for i := l; i < h; i++ {
				bind(fromInt64(i, b))
				if stop, c := run(); stop {
					return c
				}
			}
		} else {
			_, l, _ := asInt(lo)
			_, h, _ := asInt(hi)
			for i := l; i < h; i++ {
				bind(fromUint64(i, b))
				if stop, c := run(); stop {
					return c
				}
			}
		}
		return ctrlNone
	}

	// The collection is evaluated once; as a value, later writes to the
	// variable it came from do not affect the iteration.
	coll := in.eval(fr, s.Iter)
	markShared(coll)
	var items []Value
	switch c := coll.(type) {
	case *Array:
		items = c.Elems
	case *MapVal:
		items = c.Keys
	}
	for _, item := range items {
		bind(item)
		if stop, c := run(); stop {
			return c
		}
	}
	return ctrlNone
}

func (in *Interp) ifStmt(fr frame, x *ast.IfExpr) ctrl {
	if in.cond(fr, x.Cond) {
		return in.execStmts(fr, x.Then.Stmts)
	}
	switch e := x.Else.(type) {
	case *ast.Block:
		return in.execStmts(fr, e.Stmts)
	case *ast.IfExpr:
		return in.ifStmt(fr, e)
	}
	return ctrlNone
}

// cond evaluates a condition, binding `is T as name` variables when it
// succeeds.
func (in *Interp) cond(fr frame, x ast.Expr) bool {
	switch e := x.(type) {
	case *ast.IsExpr:
		v := in.eval(fr, e.X)
		target := in.info.TypeExprs[e.Type]
		if runtimeType(v) != target {
			return false
		}
		if e.As != nil {
			if variable, ok := in.info.Defs[e.As].(*types.Var); ok {
				markShared(v)
				fr[variable] = &cell{v: v}
			}
		}
		return true
	case *ast.BinaryExpr:
		if e.Op == token.AND {
			return in.cond(fr, e.X) && in.cond(fr, e.Y)
		}
	}
	return in.eval(fr, x).(bool)
}

// valueBlock evaluates a block in value position: its value is its last
// expression. A statement that leaves the block raises a ctrlSignal.
func (in *Interp) valueBlock(fr frame, b *ast.Block) Value {
	n := len(b.Stmts)
	for _, s := range b.Stmts[:n-1] {
		if c := in.exec(fr, s); c != ctrlNone {
			panic(ctrlSignal{kind: c, val: in.ret})
		}
	}
	last := b.Stmts[n-1]
	if es, ok := last.(*ast.ExprStmt); ok {
		return in.eval(fr, es.X)
	}
	if c := in.exec(fr, last); c != ctrlNone {
		panic(ctrlSignal{kind: c, val: in.ret})
	}
	return nil
}

// branch evaluates an if or match branch in value position.
func (in *Interp) branch(fr frame, body ast.Expr) Value {
	if b, ok := body.(*ast.Block); ok {
		return in.valueBlock(fr, b)
	}
	return in.eval(fr, body)
}
