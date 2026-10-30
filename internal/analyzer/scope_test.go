package analyzer

import "testing"

func newVar(name string) *VarSymbol {
	return &VarSymbol{symbolBase: symbolBase{Name: name}}
}

func TestNewScopeLinksParent(t *testing.T) {
	parent := NewScope(nil, PackageScope, "package")
	child := NewScope(parent, FunctionScope, "main")

	if child.Parent != parent {
		t.Errorf("child.Parent = %p, want %p", child.Parent, parent)
	}
	if len(parent.Children) != 1 || parent.Children[0] != child {
		t.Errorf("parent.Children = %v, want [child]", parent.Children)
	}
}

func TestDeclare(t *testing.T) {
	s := NewScope(nil, PackageScope, "package")
	first := newVar("x")
	second := newVar("x")

	if existing := s.Declare(first); existing != nil {
		t.Fatalf("first Declare returned %v, want nil", existing)
	}
	existing := s.Declare(second)
	if existing != Symbol(first) {
		t.Errorf("second Declare returned %v, want the first symbol", existing)
	}
	if got := s.LookupLocal("x"); got != Symbol(first) {
		t.Errorf("LookupLocal after redeclaration = %v, want the first symbol", got)
	}
	if n := len(s.Symbols()); n != 1 {
		t.Errorf("len(Symbols()) = %d, want 1", n)
	}
}

func TestSymbolsOrder(t *testing.T) {
	s := NewScope(nil, PackageScope, "package")
	names := []string{"c", "a", "b"}
	for _, n := range names {
		s.Declare(newVar(n))
	}

	got := s.Symbols()
	if len(got) != len(names) {
		t.Fatalf("len(Symbols()) = %d, want %d", len(got), len(names))
	}
	for i, sym := range got {
		if sym.base().Name != names[i] {
			t.Errorf("Symbols()[%d] = %s, want %s", i, sym.base().Name, names[i])
		}
	}
}

func TestLookupLocalIgnoresParents(t *testing.T) {
	parent := NewScope(nil, PackageScope, "package")
	child := NewScope(parent, BlockScope, "if")
	parent.Declare(newVar("x"))

	if got := child.LookupLocal("x"); got != nil {
		t.Errorf("child.LookupLocal(x) = %v, want nil", got)
	}
}

// The scope chain used by TestLookup:
//
//	universe      int
//	└ package     g
//	  └ fn f      x, shadowed
//	    └ if      (block)
//	      └ fn closure   shadowed
//	        └ if  (block)
func TestLookup(t *testing.T) {
	universe := NewScope(nil, UniverseScope, "universe")
	pkg := NewScope(universe, PackageScope, "package")
	fn := NewScope(pkg, FunctionScope, "f")
	fnBlock := NewScope(fn, BlockScope, "if")
	closure := NewScope(fnBlock, FunctionScope, "closure")
	closureBlock := NewScope(closure, BlockScope, "if")

	intSym := &TypeSymbol{symbolBase: symbolBase{Name: "int", Type: IntT}}
	g := newVar("g")
	x := newVar("x")
	outerShadowed := newVar("shadowed")
	innerShadowed := newVar("shadowed")

	universe.Declare(intSym)
	pkg.Declare(g)
	fn.Declare(x)
	fn.Declare(outerShadowed)
	closure.Declare(innerShadowed)

	tests := []struct {
		name        string
		from        *Scope
		lookup      string
		want        Symbol
		wantCrossed bool
	}{
		{"local in own function scope", fn, "x", x, false},
		{"local from a block in the same function", fnBlock, "x", x, false},
		{"parent local from the closure scope", closure, "x", x, true},
		{"parent local from a block inside the closure", closureBlock, "x", x, true},
		{"package symbol from the package scope", pkg, "g", g, false},
		{"package symbol from a function", fn, "g", g, true},
		{"package symbol from a block inside a closure", closureBlock, "g", g, true},
		{"builtin from the package scope", pkg, "int", intSym, false},
		{"builtin from a function", fn, "int", intSym, true},
		{"shadowing name inside the closure", closureBlock, "shadowed", innerShadowed, false},
		{"shadowed name outside the closure", fnBlock, "shadowed", outerShadowed, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, crossed := tc.from.Lookup(tc.lookup)
			if got != tc.want {
				t.Errorf("Lookup(%q) from %s = %v, want %v",
					tc.lookup, tc.from.Name, got, tc.want)
			}
			if crossed != tc.wantCrossed {
				t.Errorf("Lookup(%q) from %s: crossedFunc = %v, want %v",
					tc.lookup, tc.from.Name, crossed, tc.wantCrossed)
			}
		})
	}
}

// crossedFunc is only meaningful when a symbol is found, so it isn't checked here.
func TestLookupNotFound(t *testing.T) {
	pkg := NewScope(nil, PackageScope, "package")
	fn := NewScope(pkg, FunctionScope, "f")

	if got, _ := fn.Lookup("missing"); got != nil {
		t.Errorf("Lookup(missing) = %v, want nil", got)
	}
}
