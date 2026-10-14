package types

import (
	"fmt"
	"sort"
	"strings"

	"stellar/internal/ast"
	"stellar/internal/diag"
	"stellar/internal/printer"
	"stellar/internal/token"
)

// funcCtx describes the function (declared or literal) being checked.
type funcCtx struct {
	parent   *funcCtx
	result   Type
	loops    int // depth of enclosing loops within this function
	reported map[*Var]bool
}

type checker struct {
	info         *Info
	errs         diag.List
	seen         map[string]bool
	universe     *scope
	pkg          *scope
	scope        *scope
	fn           *funcCtx
	selfT        Type // the type Self stands for, or nil
	errorIface   *Interface
	basicMethods map[*Basic]map[*Interface]map[string]*Method
	declTypes    map[ast.Decl]Type
	implTargets  map[*ast.ImplDecl]Type
	markedCalls  map[*ast.CallExpr]bool // calls written as mut recv.method(...)
	named        []Type                 // declared objects and enums, in source order
}

// Check type-checks a parsed file. The returned Info is complete only when
// there are no diagnostics.
func Check(file *ast.File) (*Info, diag.List) {
	universe, errIface := newUniverse()
	c := &checker{
		info: &Info{
			Types:      map[ast.Expr]Type{},
			TypeExprs:  map[ast.TypeExpr]Type{},
			Defs:       map[*ast.Ident]Symbol{},
			Uses:       map[*ast.Ident]Symbol{},
			Selections: map[*ast.SelectorExpr]*Selection{},
			Variants:   map[*ast.VariantPattern]*Variant{},
			Funcs:      map[*ast.FuncDecl]*FuncSym{},
			Methods:    map[*ast.FuncDecl]*Method{},
			Error:      errIface,
		},
		seen:         map[string]bool{},
		universe:     universe,
		pkg:          newScope(universe),
		errorIface:   errIface,
		basicMethods: map[*Basic]map[*Interface]map[string]*Method{},
		declTypes:    map[ast.Decl]Type{},
		implTargets:  map[*ast.ImplDecl]Type{},
		markedCalls:  map[*ast.CallExpr]bool{},
	}
	c.scope = c.pkg
	c.checkFile(file)
	c.errs.Sort()
	return c.info, c.errs
}

func (c *checker) errorf(pos token.Pos, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	key := pos.String() + "\x00" + msg
	if c.seen[key] {
		return
	}
	c.seen[key] = true
	c.errs = append(c.errs, diag.Error{Pos: pos, Msg: msg})
}

// describe renders an expression for use in a diagnostic.
func describe(x ast.Expr) string {
	s := printer.Expr(x)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " ..."
	}
	if len(s) > 40 {
		s = s[:37] + "..."
	}
	return s
}

// bad reports whether t should silence further diagnostics.
func bad(t Type) bool { return t == Invalid || t == Never }

// ---------------------------------------------------------------------------
// Declarations

func (c *checker) checkFile(f *ast.File) {
	for _, imp := range f.Imports {
		c.errorf(imp.Pos(), "imports are not supported yet")
	}

	// Pass 1: declare type names, so declarations can refer to each other
	// in any order.
	for _, d := range f.Decls {
		var name *ast.Ident
		var t Type
		switch d := d.(type) {
		case *ast.ObjectDecl:
			name = d.Name
			t = &Object{Name: d.Name.Name, Methods: map[string]*Method{}, Impls: map[*Interface]map[string]*Method{}}
			c.named = append(c.named, t)
		case *ast.EnumDecl:
			name = d.Name
			t = &Enum{Name: d.Name.Name, Methods: map[string]*Method{}, Impls: map[*Interface]map[string]*Method{}}
			c.named = append(c.named, t)
		case *ast.InterfaceDecl:
			name = d.Name
			t = &Interface{Name: d.Name.Name, Impls: map[Type]map[string]*Method{}}
		default:
			continue
		}
		c.declTypes[d] = t
		c.declareTop(name, &TypeSym{Name: name.Name, Type: t})
	}

	// Pass 2: resolve fields, variants and interface methods.
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.ObjectDecl:
			c.objectBody(d, c.declTypes[d].(*Object))
		case *ast.EnumDecl:
			c.enumBody(d, c.declTypes[d].(*Enum))
		case *ast.InterfaceDecl:
			c.interfaceBody(d, c.declTypes[d].(*Interface))
		}
	}

	// Pass 3: function signatures and impls.
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			sig, _ := c.signature(d.Params, d.Result, false)
			sym := &FuncSym{Name: d.Name.Name, Sig: sig, Decl: d}
			c.declareTop(d.Name, sym)
			c.info.Funcs[d] = sym
		case *ast.ImplDecl:
			c.implDecl(d)
		}
	}
	c.checkMethodClashes()

	// Pass 4: bodies.
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			sym := c.info.Funcs[d]
			c.funcBody(d.Params, sym.Sig, nil, nil, d.Body, "function "+d.Name.Name, d.Name.Pos())
		case *ast.ImplDecl:
			target := c.implTargets[d]
			if target == nil {
				continue
			}
			c.selfT = target
			for _, md := range d.Methods {
				if m := c.info.Methods[md]; m != nil {
					c.funcBody(md.Params, m.Sig, m.Self, target, md.Body, "method "+md.Name.Name, md.Name.Pos())
				}
			}
			c.selfT = nil
		}
	}

	c.checkMain()
}

func (c *checker) declareTop(id *ast.Ident, sym Symbol) {
	switch {
	case c.universe.syms[id.Name] != nil:
		c.errorf(id.Pos(), "%s is a built-in name and cannot be redeclared", id.Name)
	case c.pkg.syms[id.Name] != nil:
		c.errorf(id.Pos(), "%s is already declared", id.Name)
	default:
		c.pkg.syms[id.Name] = sym
		c.info.Defs[id] = sym
	}
}

func (c *checker) checkMain() {
	sym, ok := c.pkg.syms["main"].(*FuncSym)
	if !ok {
		return
	}
	r, isResult := sym.Sig.Result.(*Result)
	if len(sym.Sig.Params) > 0 || (sym.Sig.Result != Void && !(isResult && r.Ok == Void)) {
		c.errorf(sym.Decl.Name.Pos(), "main must be declared as fn main() or fn main() -> Result")
	}
}

func (c *checker) objectBody(d *ast.ObjectDecl, obj *Object) {
	seen := map[string]bool{}
	for _, f := range d.Fields {
		if seen[f.Name.Name] {
			c.errorf(f.Name.Pos(), "duplicate field %s in %s", f.Name.Name, obj.Name)
		}
		seen[f.Name.Name] = true
		t := c.resolveType(f.Type)
		obj.Fields = append(obj.Fields, &Field{Name: f.Name.Name, Type: t, Pub: f.Pub})
	}
}

func (c *checker) enumBody(d *ast.EnumDecl, e *Enum) {
	seen := map[string]bool{}
	for i, v := range d.Variants {
		if seen[v.Name.Name] {
			c.errorf(v.Name.Pos(), "duplicate variant %s in %s", v.Name.Name, e.Name)
		}
		seen[v.Name.Name] = true
		variant := &Variant{Name: v.Name.Name, Enum: e, Index: i}
		if v.Fields != nil && len(v.Fields) == 0 {
			c.errorf(v.Name.Pos(), "variant %s has no fields; remove the ()", v.Name.Name)
		}
		fseen := map[string]bool{}
		for _, f := range v.Fields {
			if fseen[f.Name.Name] {
				c.errorf(f.Name.Pos(), "duplicate field %s in %s.%s", f.Name.Name, e.Name, v.Name.Name)
			}
			fseen[f.Name.Name] = true
			t := c.resolveType(f.Type)
			variant.Fields = append(variant.Fields, &Field{Name: f.Name.Name, Type: t, Pub: true})
		}
		e.Variants = append(e.Variants, variant)
	}
}

func (c *checker) interfaceBody(d *ast.InterfaceDecl, iface *Interface) {
	c.selfT = SelfT
	defer func() { c.selfT = nil }()
	seen := map[string]bool{}
	for _, m := range d.Methods {
		if seen[m.Name.Name] {
			c.errorf(m.Name.Pos(), "duplicate method %s in %s", m.Name.Name, iface.Name)
		}
		seen[m.Name.Name] = true
		sig, self := c.signature(m.Params, m.Result, true)
		if self == nil {
			c.errorf(m.Name.Pos(), "interface method %s must take self as its first parameter", m.Name.Name)
			self = &SelfParam{}
		}
		iface.Methods = append(iface.Methods, &IfaceMethod{Name: m.Name.Name, MutSelf: self.Mut, Sig: sig})
	}
}

// signature resolves a parameter list and result. A leading self parameter
// is returned separately when allowSelf is set.
func (c *checker) signature(params []*ast.Param, result ast.TypeExpr, allowSelf bool) (*Func, *SelfParam) {
	f := &Func{Result: Void}
	var self *SelfParam
	for i, p := range params {
		if p.IsSelf {
			if !allowSelf || i != 0 {
				c.errorf(p.Pos(), "self is only allowed as the first parameter of a method")
				continue
			}
			self = &SelfParam{Mut: p.MutSelf}
			continue
		}
		f.Params = append(f.Params, c.resolveType(p.Type))
	}
	if result != nil {
		f.Result = c.resolveType(result)
	}
	return f, self
}

func (c *checker) implDecl(d *ast.ImplDecl) {
	target := c.resolveType(d.Target)
	switch t := target.(type) {
	case *Object, *Enum:
	case *Basic:
		if d.Interface == nil {
			c.errorf(d.Target.Pos(), "cannot define methods on built-in type %s", t)
			return
		}
	case invalidType:
		return
	default:
		c.errorf(d.Target.Pos(), "cannot implement methods for %s; impl needs an object, enum or primitive type", t)
		return
	}
	c.implTargets[d] = target
	c.selfT = target
	defer func() { c.selfT = nil }()

	if d.Interface == nil {
		c.inherentImpl(d, target)
		return
	}
	it := c.resolveType(d.Interface)
	iface, ok := it.(*Interface)
	if !ok {
		if it != Invalid {
			c.errorf(d.Interface.Pos(), "%s is not an interface", it)
		}
		delete(c.implTargets, d)
		return
	}
	if _, dup := iface.Impls[target]; dup {
		c.errorf(d.Pos(), "%s already implements %s", target, iface)
		delete(c.implTargets, d)
		return
	}
	if _, isBasic := target.(*Basic); isBasic && iface == c.errorIface {
		c.errorf(d.Pos(), "cannot implement built-in interface Error for built-in type %s", target)
		delete(c.implTargets, d)
		return
	}

	methods := map[string]*Method{}
	for _, md := range d.Methods {
		sig, self := c.signature(md.Params, md.Result, true)
		name := md.Name.Name
		if md.Pub {
			c.errorf(md.Pos(), "methods in an interface impl take the interface's visibility; remove pub")
		}
		im := iface.Method(name)
		switch {
		case im == nil:
			c.errorf(md.Name.Pos(), "%s is not a method of %s", name, iface)
			continue
		case methods[name] != nil:
			c.errorf(md.Name.Pos(), "%s.%s is already declared", target, name)
			continue
		}
		want := Subst(im.Sig, target).(*Func)
		if self == nil || self.Mut != im.MutSelf || !Identical(want, sig) {
			have := "a function without self"
			if self != nil {
				have = methodString(self.Mut, sig)
			}
			c.errorf(md.Name.Pos(), "%s.%s does not match %s: want %s, have %s",
				target, name, iface, methodString(im.MutSelf, want), have)
			if self == nil {
				self = &SelfParam{Mut: im.MutSelf}
			}
		}
		m := &Method{Name: name, Owner: target, Interface: iface, Self: self, Sig: sig, Decl: md}
		methods[name] = m
		c.info.Methods[md] = m
	}
	for _, im := range iface.Methods {
		if methods[im.Name] == nil {
			c.errorf(d.Target.Pos(), "%s does not implement %s: missing method %s", target, iface, im.Name)
		}
	}

	iface.Impls[target] = methods
	switch t := target.(type) {
	case *Object:
		t.Impls[iface] = methods
	case *Enum:
		t.Impls[iface] = methods
	case *Basic:
		if c.basicMethods[t] == nil {
			c.basicMethods[t] = map[*Interface]map[string]*Method{}
		}
		c.basicMethods[t][iface] = methods
	}
}

func (c *checker) inherentImpl(d *ast.ImplDecl, target Type) {
	methods := inherentMethods(target)
	for _, md := range d.Methods {
		sig, self := c.signature(md.Params, md.Result, true)
		name := md.Name.Name
		if methods[name] != nil {
			c.errorf(md.Name.Pos(), "%s.%s is already declared", target, name)
			continue
		}
		m := &Method{Name: name, Owner: target, Self: self, Sig: sig, Pub: md.Pub, Decl: md}
		methods[name] = m
		c.info.Methods[md] = m
	}
}

func inherentMethods(t Type) map[string]*Method {
	switch t := t.(type) {
	case *Object:
		return t.Methods
	case *Enum:
		return t.Methods
	}
	return nil
}

func implsOf(t Type) map[*Interface]map[string]*Method {
	switch t := t.(type) {
	case *Object:
		return t.Impls
	case *Enum:
		return t.Impls
	}
	return nil
}

// checkMethodClashes enforces that an inherent method shares its name with
// neither a field nor a method of an implemented interface.
func (c *checker) checkMethodClashes() {
	for _, t := range c.named {
		methods := inherentMethods(t)
		ifaces := sortedIfaces(implsOf(t))
		for _, name := range sortedKeys(methods) {
			m := methods[name]
			if obj, ok := t.(*Object); ok && obj.Field(name) != nil {
				c.errorf(m.Decl.Name.Pos(), "%s has both a field and a method named %s", t, name)
			}
			for _, iface := range ifaces {
				if iface.Method(name) != nil {
					c.errorf(m.Decl.Name.Pos(), "%s.%s conflicts with %s.%s, which %s implements", t, name, iface, name, t)
				}
			}
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedIfaces(m map[*Interface]map[string]*Method) []*Interface {
	out := make([]*Interface, 0, len(m))
	for iface := range m {
		out = append(out, iface)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// funcBody checks the body of a declared function, method or literal.
func (c *checker) funcBody(params []*ast.Param, sig *Func, self *SelfParam, owner Type, body *ast.Block, what string, pos token.Pos) {
	ctx := &funcCtx{parent: c.fn, result: sig.Result, reported: map[*Var]bool{}}
	savedFn, savedScope := c.fn, c.scope
	c.fn = ctx
	c.scope = newScope(c.scope)
	i := 0
	for _, p := range params {
		if p.IsSelf {
			if self != nil && owner != nil {
				kind := VarSelf
				if self.Mut {
					kind = VarMutSelf
				}
				c.declareVar(p.Name, owner, kind)
			}
			continue
		}
		if i < len(sig.Params) {
			t, kind := sig.Params[i], VarParam
			if m, ok := t.(*Mut); ok {
				t, kind = m.Elem, VarMutParam
			}
			c.declareVar(p.Name, t, kind)
		}
		i++
	}
	terminates := c.stmts(body.Stmts)
	c.info.Types[body] = Void
	if sig.Result != Void && !terminates {
		c.errorf(pos, "missing return at end of %s", what)
	}
	c.fn, c.scope = savedFn, savedScope
}

func (c *checker) declareVar(id *ast.Ident, t Type, kind VarKind) *Var {
	if id.Name == "_" {
		return nil
	}
	if c.universe.syms[id.Name] != nil {
		c.errorf(id.Pos(), "cannot use built-in name %s as a variable name", id.Name)
		return nil
	}
	if c.scope.syms[id.Name] != nil {
		c.errorf(id.Pos(), "%s is already declared in this scope", id.Name)
		return nil
	}
	v := &Var{Name: id.Name, Type: t, Kind: kind, Pos: id.Pos(), fn: c.fn}
	c.scope.syms[id.Name] = v
	c.info.Defs[id] = v
	return v
}

func (c *checker) openScope()  { c.scope = newScope(c.scope) }
func (c *checker) closeScope() { c.scope = c.scope.parent }

// ---------------------------------------------------------------------------
// Type expressions

func (c *checker) resolveType(te ast.TypeExpr) Type {
	t := c.resolveType0(te)
	c.info.TypeExprs[te] = t
	return t
}

func (c *checker) resolveType0(te ast.TypeExpr) Type {
	switch te := te.(type) {
	case *ast.NamedType:
		return c.namedType(te)
	case *ast.MutType:
		// The parser only allows mut on parameter types.
		inner := c.resolveType(te.Elem)
		if inner == Invalid {
			return Invalid
		}
		return &Mut{Elem: inner}
	case *ast.OptionalType:
		return &Maybe{Elem: c.resolveType(te.Elem)}
	case *ast.ArrayType:
		return &Array{Elem: c.resolveType(te.Elem)}
	case *ast.FuncType:
		f := &Func{Result: Void}
		for _, p := range te.Params {
			f.Params = append(f.Params, c.resolveType(p))
		}
		if te.Result != nil {
			f.Result = c.resolveType(te.Result)
		}
		return f
	}
	panic(fmt.Sprintf("types: unexpected type expression %T", te))
}

func (c *checker) namedType(te *ast.NamedType) Type {
	name := te.Name.Name
	if te.Module != nil {
		c.errorf(te.Pos(), "module-qualified types are not supported yet")
		return Invalid
	}
	if name == "Self" {
		if c.selfT == nil {
			c.errorf(te.Pos(), "Self is only allowed inside impl and interface declarations")
			return Invalid
		}
		if len(te.Args) > 0 {
			c.errorf(te.Pos(), "Self does not take type arguments")
		}
		return c.selfT
	}
	args := make([]Type, len(te.Args))
	for i, a := range te.Args {
		args[i] = c.resolveType(a)
	}
	switch sym := c.pkg.lookup(name).(type) {
	case nil:
		c.errorf(te.Pos(), "undefined type %s", name)
	case *TypeSym:
		if len(args) > 0 {
			c.errorf(te.Pos(), "%s does not take type arguments", name)
		}
		c.info.Uses[te.Name] = sym
		return sym.Type
	case *GenericSym:
		switch {
		case name == "Result" && len(args) == 0:
			return &Result{Ok: Void}
		case name == "Result" && len(args) == 1:
			return &Result{Ok: args[0]}
		case name == "Maybe" && len(args) == 1:
			return &Maybe{Elem: args[0]}
		case name == "Ref" && len(args) == 1:
			return &Ref{Elem: args[0]}
		case name == "Map" && len(args) == 2:
			if _, ok := args[0].(*Basic); !ok && args[0] != Invalid {
				c.errorf(te.Args[0].Pos(), "map keys must be primitive types, not %s", args[0])
			}
			return &Map{Key: args[0], Value: args[1]}
		}
		want := map[string]string{"Result": "Result or Result[T]", "Maybe": "Maybe[T]", "Map": "Map[K, V]", "Ref": "Ref[T]"}[name]
		c.errorf(te.Pos(), "wrong number of type arguments for %s; write %s", name, want)
	default:
		c.errorf(te.Pos(), "%s is not a type", name)
	}
	return Invalid
}
