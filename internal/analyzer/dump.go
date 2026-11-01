package analyzer

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"stellar/internal/ast"
)

// Dump writes every scope and symbol in a as an indented tree, starting at the
// universe scope, followed by the analyzer's errors, one per line.
//
// Extending the dumper when the analyzer grows:
//   - new Symbol kind: add a case to dumper.symbol.
//   - new Type kind: add a case to dumper.typeLit (one-line form) and, if it has
//     members worth listing, to dumper.typeBody.
//
// Kinds without a case print a "<not implemented: T>" marker instead of being
// silently skipped, so gaps show up in the output.
func Dump(w io.Writer, a *Analyzer) {
	if a == nil {
		fmt.Fprintln(w, "<nil analyzer>")
		return
	}
	d := &dumper{w: w, current: a.CurrentScope, owners: map[Type]*TypeSymbol{}}

	root := a.Universe
	if root == nil {
		root = a.Package
	}
	if root != nil {
		d.collectOwners(root)
		d.scope(root)
	}

	if len(a.Errors) > 0 {
		fmt.Fprintln(w, "--- errors ---")
		for i := range a.Errors {
			fmt.Fprintln(w, errorString(&a.Errors[i]))
		}
	}
}

const dumpIndent = "  "

type dumper struct {
	w       io.Writer
	depth   int
	current *Scope
	// owners maps each declared type to the symbol that declares it, so type
	// references print by name instead of expanding (and recursing into)
	// the referenced type.
	owners map[Type]*TypeSymbol
}

// line writes one line at the current indentation.
func (d *dumper) line(format string, args ...any) {
	fmt.Fprintf(d.w, "%s%s\n", strings.Repeat(dumpIndent, d.depth), fmt.Sprintf(format, args...))
}

// indented runs f one indentation level deeper.
func (d *dumper) indented(f func()) {
	d.depth++
	defer func() { d.depth-- }()
	f()
}

func (d *dumper) collectOwners(s *Scope) {
	for _, sym := range s.Symbols() {
		if ts, ok := sym.(*TypeSymbol); ok && ts.Type != nil {
			if _, seen := d.owners[ts.Type]; !seen {
				d.owners[ts.Type] = ts
			}
		}
	}
	for _, c := range s.Children {
		d.collectOwners(c)
	}
}

// Scopes

func (d *dumper) scope(s *Scope) {
	header := "scope " + s.Id
	if s == d.current {
		header += " (current)"
	}
	d.line("%s", header)
	d.indented(func() {
		for _, sym := range s.Symbols() {
			d.symbol(sym)
		}
		for _, c := range s.Children {
			d.scope(c)
		}
	})
}

// Symbols

func (d *dumper) symbol(sym Symbol) {
	switch s := sym.(type) {
	case nil:
		d.line("<nil symbol>")
	case *TypeSymbol:
		d.typeSymbol(s)
	case *VarSymbol:
		d.varSymbol(s)
	case *FunctionSymbol:
		d.functionSymbol(s)
	case *ImportSymbol:
		d.importSymbol(s)
	default:
		d.line("<not implemented: %T> %s", sym, sym.Name())
	}
}

func (d *dumper) typeSymbol(s *TypeSymbol) {
	// A type that is owned by another symbol (e.g. an alias) is shown by
	// name; its members are listed under the owning symbol.
	if owner := d.owners[s.Type]; owner != nil && owner != s {
		d.line("type %s: %s%s", s.Id, owner.Id, at(s.Node))
		return
	}
	d.line("type %s: %s%s", s.Id, d.typeLit(s.Type), at(s.Node))
	d.indented(func() { d.typeBody(s.Type) })
}

func (d *dumper) varSymbol(s *VarSymbol) {
	kw := "var"
	if s.IsMut {
		kw = "mut"
	}
	d.line("%s%s %s: %s%s", pub(s.IsPublic), kw, s.Id, d.typeRef(s.Type), at(s.Node))
}

func (d *dumper) functionSymbol(s *FunctionSymbol) {
	d.line("%sfn %s: %s%s", pub(s.IsPublic), s.Id, d.typeRef(s.Type), at(s.Node))
}

func (d *dumper) importSymbol(s *ImportSymbol) {
	path := UNRESOLVED
	if s.Node != nil {
		parts := make([]string, len(s.Node.Path))
		for i, tok := range s.Node.Path {
			parts[i] = tok.Literal
		}
		path = strings.Join(parts, "::")
	}
	d.line("import %s (%s)%s", s.Id, path, at(s.Node))
}

// Types

// typeRef is the one-line form of a type used where it is referenced
// (variable, field, and return types): declared types print by name.
func (d *dumper) typeRef(t Type) string {
	if owner := d.owners[t]; owner != nil {
		return owner.Id
	}
	return d.typeLit(t)
}

// typeLit is the one-line form of a type itself, ignoring any name it was
// declared under.
func (d *dumper) typeLit(t Type) string {
	switch t := t.(type) {
	case nil:
		return UNRESOLVED
	case *PrimitiveType:
		return t.Id
	case *FunctionType:
		params := make([]string, len(t.Params))
		for i, p := range t.Params {
			params[i] = p.Id + ": " + d.typeRef(p.Type)
		}
		return fmt.Sprintf("fn(%s) -> %s", strings.Join(params, ", "), d.typeRef(t.ReturnType))
	case *StructType:
		return "struct"
	case *EnumType:
		return "enum"
	case *InterfaceType:
		return "interface"
	case *SliceType:
		return "[]" + d.typeRef(t.ElementType)
	default:
		return fmt.Sprintf("<not implemented: %T>", t)
	}
}

// typeBody writes the members of t, if it has any, at the current indentation.
func (d *dumper) typeBody(t Type) {
	switch t := t.(type) {
	case *StructType:
		for _, name := range sourceOrder(t.Fields, func(f Field) ast.Node {
			if f.Node == nil {
				return nil
			}
			return f.Node
		}) {
			d.field(t.Fields[name])
		}
		d.methods(t.Methods)
	case *EnumType:
		for _, m := range t.Members {
			d.line("member %s", m.Id)
			d.indented(func() { d.fields(m.Fields) })
		}
		d.methods(t.Methods)
	case *InterfaceType:
		for _, name := range sourceOrder(t.Methods, func(ft *FunctionType) ast.Node {
			if ft == nil || ft.Node == nil {
				return nil
			}
			return ft.Node
		}) {
			ft := t.Methods[name]
			suffix := ""
			if ft != nil {
				suffix = at(ft.Node)
			}
			d.line("method %s: %s%s", name, d.typeLit(nilType(ft)), suffix)
		}
	}
}

// methods writes a struct or enum method set.
func (d *dumper) methods(methods map[string]*FunctionSymbol) {
	for _, name := range sourceOrder(methods, func(fs *FunctionSymbol) ast.Node {
		if fs == nil || fs.Node == nil {
			return nil
		}
		return fs.Node
	}) {
		fs := methods[name]
		if fs == nil {
			d.line("method %s: <nil symbol>", name)
			continue
		}
		d.line("%smethod %s: %s%s", pub(fs.IsPublic), name, d.typeRef(fs.Type), at(fs.Node))
	}
}

// sourceOrder returns the keys of a field or method map in declaration order. node
// must return nil (not a typed nil) for entries without a node; those sort
// first. Names break ties, so the order is always deterministic.
func sourceOrder[V any](m map[string]V, node func(V) ast.Node) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	pos := func(k string) (int, int) {
		if n := node(m[k]); n != nil {
			p := n.Pos()
			return p.Line, p.Column
		}
		return 0, 0
	}
	slices.SortFunc(keys, func(a, b string) int {
		aLine, aCol := pos(a)
		bLine, bCol := pos(b)
		return cmp.Or(cmp.Compare(aLine, bLine), cmp.Compare(aCol, bCol), cmp.Compare(a, b))
	})
	return keys
}

// nilType converts a possibly nil *FunctionType to a Type without producing
// a typed nil, so typeLit's nil case still matches.
func nilType(ft *FunctionType) Type {
	if ft == nil {
		return nil
	}
	return ft
}

func (d *dumper) fields(fields []Field) {
	for _, f := range fields {
		d.field(f)
	}
}

func (d *dumper) field(f Field) {
	d.line("%sfield %s: %s", pub(f.IsPublic), f.Id, d.typeRef(f.Type))
}

// Helpers

func pub(isPublic bool) string {
	if isPublic {
		return "pub "
	}
	return ""
}

// at formats a symbol's declaring node as a position suffix. Symbols without
// a node (the builtins in the universe scope) are marked as builtin. Taking
// the concrete node pointer avoids the typed-nil-in-interface trap.
func at[T any, P interface {
	*T
	ast.Node
}](n P) string {
	if n == nil {
		return "  (builtin)"
	}
	return "  @" + n.Pos().String()
}

func errorString(e *AnalyzerError) string {
	if e.Node == nil {
		return "Error: " + e.Msg
	}
	return e.String()
}
