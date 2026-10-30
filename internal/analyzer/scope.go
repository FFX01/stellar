package analyzer

type ScopeKind int

const (
	UniverseScope ScopeKind = iota
	PackageScope
	FunctionScope
	BlockScope
)

type Scope struct {
	Parent   *Scope
	Children []*Scope
	Kind     ScopeKind
	Name     string
	symbols  map[string]Symbol
	order    []string
}

func NewScope(parent *Scope, kind ScopeKind, name string) *Scope {
	s := &Scope{Name: name, Kind: kind,
		symbols: map[string]Symbol{}}
	if parent != nil {
		s.Parent = parent
		parent.Children = append(parent.Children, s)
	}
	return s
}

func (s *Scope) Declare(sym Symbol) Symbol {
	if symbol, ok := s.symbols[sym.base().Name]; ok {
		return symbol
	}
	s.symbols[sym.base().Name] = sym
	s.order = append(s.order, sym.base().Name)
	return nil
}

func (s *Scope) LookupLocal(name string) Symbol {
	if sym, ok := s.symbols[name]; ok {
		return sym
	}
	return nil
}

func (s *Scope) Lookup(name string) (sym Symbol, crossedFunc bool) {
	if sym, ok := s.symbols[name]; ok {
		return sym, false
	}
	if s.Parent != nil {
		if s.Kind == FunctionScope {
			crossedFunc = true
		}
		sym, parentCrossed := s.Parent.Lookup(name)
		return sym, crossedFunc || parentCrossed
	}
	return nil, false
}

func (s *Scope) Symbols() []Symbol {
	var out []Symbol
	for _, name := range s.order {
		sym, ok := s.symbols[name]
		if ok {
			out = append(out, sym)
		}
	}
	return out
}
