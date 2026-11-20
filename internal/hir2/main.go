/* Package hir2 */
package hir2

import (
	"fmt"
	"stellar/internal/ast"
	"stellar/internal/loader"
)

type SymbolID uint32
type TypeID uint32
type ModID uint32
type StringID uint32
type ScopeID uint32
type NodeID uint32

const (
	SymbolError SymbolID = 0
	SymbolVoid  SymbolID = 1
)

const (
	TypeInvalid TypeID = 0
	TypeError   TypeID = 1
	TypeVoid    TypeID = 2
	TypeNever   TypeID = 3
)

type SymbolKind int

const (
	SKInvalid = iota
	SKType
	SKModule
	SKFunc
	SKVar
	SKConst
	SKParam
)

type TypeKind int

const (
	TKStruct = iota + 1
	TKEnum
	TKInterface
	TKAlias
	TKPrimitive
)

type Field struct {
	Name       StringID
	Type       TypeID
	ByteOffset uint32
}

type FieldRange struct {
	Start  uint32
	Length uint32
}

type Symbol struct {
	ID    SymbolID
	Scope ScopeID
	Name  StringID
	Type  TypeID
	Mod   ModID
	Kind  SymbolKind
	Node  NodeID
}

type Type struct {
	ID       TypeID
	Name     StringID
	Kind     TypeKind
	AliasFor TypeID
	Fields   FieldRange
}

type TypeInterner struct {
	Types []*Type
	ByKey map[StringID]TypeID
}

func (ti *TypeInterner) Get(name StringID) *Type {
	id := ti.ByKey[name]
	return ti.Types[id]
}

func (ti *TypeInterner) internNominalType(
	name StringID, t *Type,
) TypeID {
	return ti.intern(name, t)
}

func (ti *TypeInterner) intern(
	key StringID, t *Type,
) TypeID {
	if id, ok := ti.ByKey[key]; ok {
		return id
	}

	tid := TypeID(len(ti.Types))

	t.ID = tid
	ti.Types = append(ti.Types, t)
	ti.ByKey[key] = tid
	return tid
}

func NewTypeInterner() *TypeInterner {
	ti := &TypeInterner{}
	ti.ByKey = map[StringID]TypeID{}
	return ti
}

type StringInterner struct {
	Strings []string
	ByValue map[string]StringID
}

func (si *StringInterner) intern(s string) StringID {
	if id, ok := si.ByValue[s]; ok {
		return id
	}
	sid := StringID(len(si.Strings))
	si.Strings = append(si.Strings, s)
	si.ByValue[s] = sid
	return sid
}

func (si *StringInterner) Get(id StringID) string {
	return si.Strings[id]
}

func NewStringInterner() *StringInterner {
	si := &StringInterner{}
	si.Strings = append(si.Strings, "")
	si.ByValue = map[string]StringID{"": 0}
	return si
}

type Module struct {
	ID       ModID
	Name     StringID
	Scope    ScopeID
	Original *loader.Module
}

type Scope struct {
	ID       ScopeID
	Name     string
	Parent   *Scope
	Children []*Scope
	Symbols  map[StringID]SymbolID
}

func newScope(name string, parent *Scope) *Scope {
	sc := &Scope{
		Name: name,
	}
	sc.Symbols = map[StringID]SymbolID{}
	if parent != nil {
		sc.Parent = parent
		parent.Children = append(parent.Children, sc)
	}
	return sc
}

func newUniverse() *Scope {
	u := newScope("universe", nil)
	return u
}

func (s *Scope) declareSymbol(name StringID, id SymbolID) error {
	if _, ok := s.Symbols[name]; ok {
		return fmt.Errorf("`%s` already declared in this scope", name)
	}
	s.Symbols[name] = id
	return nil
}

type Diag struct {
	Node ast.Node
	File string
	Msg  string
}

func (d *Diag) String() string {
	return fmt.Sprintf(
		"%s %s: %s",
		d.File,
		d.Node.Pos().String(),
		d.Msg,
	)
}

type Context struct {
	Universe     *Scope
	CurrentScope *Scope
	Scopes       []*Scope
	Modules      []*Module
	Diags        []*Diag
	Types        *TypeInterner
	Strings      *StringInterner
	Nodes        []ast.Node
	Symbols      []Symbol
	Fields       []Field
}

func (c *Context) addField(f Field) {
	c.Fields = append(c.Fields, f)
}

func (c *Context) diag(
	mod *Module, node ast.Node, fmtStr string, args ...any,
) {
	msg := fmt.Sprintf(fmtStr, args...)
	d := &Diag{File: mod.Original.File, Node: node, Msg: msg}
	c.Diags = append(c.Diags, d)
}

func (c *Context) GetScope(id ScopeID) *Scope {
	return c.Scopes[id]
}

func (c *Context) addScope(s *Scope) ScopeID {
	sid := ScopeID(len(c.Scopes))
	s.ID = sid
	c.Scopes = append(c.Scopes, s)
	return sid
}

func (c *Context) addNode(n ast.Node) NodeID {
	nid := NodeID(len(c.Nodes))
	c.Nodes = append(c.Nodes, n)
	return nid
}

func (c *Context) addSymbol(s Symbol) SymbolID {
	sid := SymbolID(len(c.Symbols))

	s.ID = sid
	c.Symbols = append(c.Symbols, s)
	return sid
}

func NewContext(modules []*loader.Module) *Context {
	c := &Context{Universe: newUniverse()}
	c.CurrentScope = c.Universe

	c.Types = NewTypeInterner()
	c.Strings = NewStringInterner()

	errorT := &Type{
		Name: c.Strings.intern("Error"),
		Kind: TKStruct,
	}
	voidT := &Type{
		Name: c.Strings.intern("void"),
		Kind: TKPrimitive,
	}
	neverT := &Type{
		Name: c.Strings.intern("Never"),
		Kind: TKPrimitive,
	}

	errorID := c.Types.intern(errorT.Name, errorT)
	c.Types.intern(voidT.Name, voidT)
	c.Types.intern(neverT.Name, neverT)

	errorSym := Symbol{
		ID:   SymbolError,
		Name: errorT.Name,
		Type: errorID,
		Kind: SKType,
	}
	c.addSymbol(errorSym)
	if err := c.Universe.declareSymbol(errorSym.Name, SymbolError); err != nil {
		panic(err)
	}

	for idx, mod := range modules {
		sc := newScope(mod.File, c.Universe)
		scid := c.addScope(sc)
		mid := ModID(idx)
		m := &Module{
			ID:       mid,
			Name:     c.Strings.intern(mod.Name),
			Scope:    scid,
			Original: mod,
		}
		c.Modules = append(c.Modules, m)
	}
	return c
}

func (c *Context) storeTypeDecl(
	mod *Module, node ast.Node, name string, tk TypeKind,
) {
	nid := c.addNode(node)
	nameID := c.Strings.intern(name)

	t := &Type{Name: nameID, Kind: tk}
	tid := c.Types.intern(t.Name, t)

	sym := Symbol{
		Name:  t.Name,
		Type:  tid,
		Mod:   mod.ID,
		Node:  nid,
		Kind:  SKType,
		Scope: mod.Scope,
	}
	sid := c.addSymbol(sym)

	if err := c.CurrentScope.declareSymbol(sym.Name, sid); err != nil {
		c.diag(mod, node, "%s", err.Error())
	}
}
func (c *Context) collectTypeDecl(mod *Module, node *ast.TypeDeclStmt) {
	switch node.Definition.(type) {
	case *ast.StructType:
		c.storeTypeDecl(mod, node, node.Name, TKStruct)
	case *ast.EnumType:
		c.storeTypeDecl(mod, node, node.Name, TKEnum)
	case *ast.InterfaceType:
		c.storeTypeDecl(mod, node, node.Name, TKInterface)
	}
}

func (c *Context) pass2() {
	for _, sym := range c.Symbols {
		t := c.Types.Types[sym.Type]
		switch t.Kind {
		case TKStruct:
			node := c.Nodes[sym.Node]
			def := node.(*ast.TypeDeclStmt).Definition.(*ast.StructType)
			fieldsLen := 0
			for _, f := range def.Fields {
				fname := c.Strings.intern()
			}
		}
	}
}

func (c *Context) pass1() {
	for _, mod := range c.Modules {
		c.CurrentScope = c.GetScope(mod.Scope)
		for _, stmt := range mod.Original.Program.Statements {
			switch t := stmt.(type) {
			case *ast.TypeDeclStmt:
				c.collectTypeDecl(mod, t)
			case *ast.TypeAliasStmt:
				c.storeTypeDecl(mod, t, t.Name, TKAlias)
			case *ast.FunctionStmt:
				nid := c.addNode(t)
				sym := Symbol{
					Name:  c.Strings.intern(t.Signature.Name),
					Mod:   mod.ID,
					Node:  nid,
					Kind:  SKFunc,
					Scope: mod.Scope,
				}
				sid := c.addSymbol(sym)
				if err := c.CurrentScope.declareSymbol(sym.Name, sid); err != nil {
					c.diag(mod, t, "%s", err.Error())
				}
			}
		}
	}
}

func (c *Context) pass0() {
	for _, mod := range c.Modules {
		c.CurrentScope = c.GetScope(mod.Scope)
		for _, imp := range mod.Original.Imports {
			nid := c.Strings.intern(imp.Name)
			sym := Symbol{Name: nid, Kind: SKModule, Mod: mod.ID, Scope: mod.Scope}
			sid := c.addSymbol(sym)
			if err := c.CurrentScope.declareSymbol(sym.Name, sid); err != nil {
				c.diag(
					mod,
					mod.Original.Program.Statements[0],
					"%s",
					err.Error(),
				)
			}
		}
	}
}

func (c *Context) Run() {
	c.pass0()
	c.pass1()
}
