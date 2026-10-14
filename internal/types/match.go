package types

import (
	"strings"

	"stellar/internal/ast"
)

// subjectKind classifies what a match inspects.
type subjectKind int

const (
	subjectInvalid subjectKind = iota
	subjectEnum
	subjectResult
	subjectMaybe
	subjectInterface
)

// coverage tracks which cases earlier arms of a match have handled.
type coverage struct {
	all   bool            // a _ arm was seen
	cases map[string]bool // variant names, Ok/Err/Some/None
	types map[Type]bool   // concrete types tested with `is`
}

// match checks a match expression. In value position every arm produces a
// value and the match has their common type; in statement position the arms
// are statements, and the result reports whether every arm terminates.
func (c *checker) match(x *ast.MatchExpr, want Type, value bool) (Type, bool) {
	subj := c.expr(x.Subject, nil)
	kind := c.subjectKind(x.Subject, subj)
	cov := &coverage{cases: map[string]bool{}, types: map[Type]bool{}}
	u := &branches{want: want}
	allTerminate := len(x.Arms) > 0

	for _, arm := range x.Arms {
		c.openScope()
		c.pattern(arm.Pattern, subj, kind, cov)
		if value {
			c.branch(u, arm.Body)
		} else if !c.armStmt(arm.Body) {
			allTerminate = false
		}
		c.closeScope()
	}
	c.checkExhaustive(x, subj, kind, cov)

	if len(x.Arms) == 0 {
		allTerminate = false
	}
	if !value {
		return Void, allTerminate
	}
	if len(x.Arms) == 0 {
		return Invalid, false
	}
	return u.final(), false
}

func (c *checker) armStmt(body ast.Expr) bool {
	if b, ok := body.(*ast.Block); ok {
		return c.block(b)
	}
	return c.exprStmt(body)
}

func (c *checker) subjectKind(x ast.Expr, t Type) subjectKind {
	switch t.(type) {
	case *Enum:
		return subjectEnum
	case *Result:
		return subjectResult
	case *Maybe:
		return subjectMaybe
	case *Interface:
		return subjectInterface
	case invalidType, neverType:
		return subjectInvalid
	}
	c.errorf(x.Pos(), "cannot match on %s (type %s): match needs an enum, Result, Maybe or interface value", describe(x), t)
	return subjectInvalid
}

// pattern checks one arm's pattern against the subject, declares its
// bindings in the current scope, and records what it covers.
func (c *checker) pattern(p ast.Pattern, subj Type, kind subjectKind, cov *coverage) {
	if cov.all {
		c.errorf(p.Pos(), "unreachable match arm: an earlier _ arm matches everything")
	}
	switch p := p.(type) {
	case *ast.WildcardPattern:
		cov.all = true

	case *ast.TypePattern:
		target := c.resolveType(p.Type)
		if kind != subjectInterface {
			if kind != subjectInvalid {
				c.errorf(p.Pos(), "is patterns need an interface value, but the subject has type %s", subj)
			}
			c.declareVar(asOrBlank(p.As), Invalid, VarBinding)
			return
		}
		ok := c.checkTypeTestPattern(p, subj, target)
		if ok && cov.types[target] {
			c.errorf(p.Pos(), "duplicate arm for %s", target)
		}
		cov.types[target] = true
		bound := Invalid
		if ok {
			bound = target
		}
		if p.As != nil {
			c.declareVar(p.As, bound, VarBinding)
		}

	case *ast.VariantPattern:
		c.variantPattern(p, subj, kind, cov)
	}
}

func asOrBlank(id *ast.Ident) *ast.Ident {
	if id == nil {
		return &ast.Ident{Name: "_"}
	}
	return id
}

func (c *checker) checkTypeTestPattern(p *ast.TypePattern, subj, target Type) bool {
	if target == Invalid {
		return false
	}
	iface := subj.(*Interface)
	switch target.(type) {
	case *Object, *Enum, *Basic:
	default:
		c.errorf(p.Type.Pos(), "type tests need a concrete type, not %s", target)
		return false
	}
	if !Implements(target, iface) {
		c.errorf(p.Type.Pos(), "%s does not implement %s, so this arm can never match", target, iface)
		return false
	}
	return true
}

func (c *checker) variantPattern(p *ast.VariantPattern, subj Type, kind subjectKind, cov *coverage) {
	switch kind {
	case subjectInvalid:
		c.declareBindings(p)
		return
	case subjectInterface:
		c.errorf(p.Pos(), "match on interface %s needs is patterns, such as is Type as name", subj)
		c.declareBindings(p)
		return
	case subjectEnum:
		c.enumPattern(p, subj, cov)
		return
	}

	// Result and Maybe: Ok, Err, Some or None, each with at most one binding.
	name, ok := p.Name.(*ast.Ident)
	valid := map[subjectKind][]string{subjectResult: {"Ok", "Err"}, subjectMaybe: {"Some", "None"}}[kind]
	if !ok || (name.Name != valid[0] && name.Name != valid[1]) {
		c.errorf(p.Pos(), "expected %s or %s for a %s subject", valid[0], valid[1], subj)
		c.declareBindings(p)
		return
	}
	if cov.cases[name.Name] {
		c.errorf(p.Pos(), "duplicate arm for %s", name.Name)
	}
	cov.cases[name.Name] = true

	var payload Type
	switch name.Name {
	case "Ok":
		if ok := subj.(*Result).Ok; ok != Void {
			payload = ok
		}
	case "Err":
		payload = c.errorIface
	case "Some":
		payload = subj.(*Maybe).Elem
	}
	switch {
	case len(p.Bindings) == 0:
	case payload == nil:
		c.errorf(p.Pos(), "%s carries no value; remove the ()", name.Name)
		c.declareBindings(p)
	case len(p.Bindings) > 1:
		c.errorf(p.Pos(), "%s binds one value, not %d", name.Name, len(p.Bindings))
		c.declareBindings(p)
	case p.Bindings[0].Name != nil:
		c.errorf(p.Bindings[0].Pos(), "write %s(name) to bind the value", name.Name)
		c.declareBindings(p)
	default:
		c.declareVar(p.Bindings[0].Field, payload, VarBinding)
	}
}

func (c *checker) enumPattern(p *ast.VariantPattern, subj Type, cov *coverage) {
	enum := subj.(*Enum)
	sel, ok := p.Name.(*ast.SelectorExpr)
	var qual *ast.Ident
	if ok {
		qual, _ = sel.X.(*ast.Ident)
	}
	if qual == nil || qual.Name != enum.Name {
		c.errorf(p.Pos(), "expected a variant of %s, written %s.Variant", enum, enum)
		c.declareBindings(p)
		return
	}
	c.info.Uses[qual] = c.pkg.lookup(enum.Name)
	v := enum.Variant(sel.Sel.Name)
	if v == nil {
		c.errorf(sel.Sel.Pos(), "%s has no variant %s", enum, sel.Sel.Name)
		c.declareBindings(p)
		return
	}
	c.info.Variants[p] = v
	if cov.cases[v.Name] {
		c.errorf(p.Pos(), "duplicate arm for %s.%s", enum, v.Name)
	}
	cov.cases[v.Name] = true

	if p.Bindings != nil && v.Fields == nil {
		c.errorf(p.Pos(), "%s.%s has no fields; remove the ()", enum, v.Name)
		c.declareBindings(p)
		return
	}
	seen := map[string]bool{}
	for _, b := range p.Bindings {
		f := v.Field(b.Field.Name)
		if f == nil {
			c.errorf(b.Field.Pos(), "%s.%s has no field %s", enum, v.Name, b.Field.Name)
			c.declareBinding(b, Invalid)
			continue
		}
		if seen[f.Name] {
			c.errorf(b.Field.Pos(), "field %s is bound twice", f.Name)
		}
		seen[f.Name] = true
		c.declareBinding(b, f.Type)
	}
}

// declareBindings declares every binding of a pattern that failed to
// check, as Invalid, so the arm body does not report undefined names.
func (c *checker) declareBindings(p *ast.VariantPattern) {
	for _, b := range p.Bindings {
		c.declareBinding(b, Invalid)
	}
}

func (c *checker) declareBinding(b *ast.Binding, t Type) {
	name := b.Field
	if b.Name != nil {
		name = b.Name
	}
	c.declareVar(name, t, VarBinding)
}

func (c *checker) checkExhaustive(x *ast.MatchExpr, subj Type, kind subjectKind, cov *coverage) {
	if cov.all {
		return
	}
	var missing []string
	switch kind {
	case subjectEnum:
		enum := subj.(*Enum)
		for _, v := range enum.Variants {
			if !cov.cases[v.Name] {
				missing = append(missing, enum.Name+"."+v.Name)
			}
		}
	case subjectResult:
		missing = uncovered(cov, "Ok", "Err")
	case subjectMaybe:
		missing = uncovered(cov, "Some", "None")
	case subjectInterface:
		c.errorf(x.Pos(), "match on interface %s needs a _ arm: any type may implement it", subj)
		return
	default:
		return
	}
	if len(missing) > 0 {
		c.errorf(x.Pos(), "match is not exhaustive: missing %s", strings.Join(missing, ", "))
	}
}

func uncovered(cov *coverage, names ...string) []string {
	var missing []string
	for _, n := range names {
		if !cov.cases[n] {
			missing = append(missing, n)
		}
	}
	return missing
}
