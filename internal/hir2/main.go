/* Package hir2 */
package hir2

import (
	"fmt"
	"io"
	"stellar/internal/ast"
	"stellar/internal/loader"
	"strconv"
	"strings"
)

var stringInterner *StringInterner
var typeInterner *TypeInterner

type SymbolID uint32

type TypeID uint32

func (tid TypeID) LitterDump(w io.Writer) {
	typ := typeInterner.Types[tid]
	name := stringInterner.Get(typ.Name)
	out := fmt.Sprintf(" {%d: %s}", tid, name)
	w.Write([]byte(out))
}

type ModID uint32
type StringID uint32

func (sid StringID) LitterDump(w io.Writer) {
	name := stringInterner.Get(sid)
	out := fmt.Sprintf(
		" {%d: %s}",
		sid,
		name,
	)
	w.Write([]byte(out))
}

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

func (sk SymbolKind) LitterDump(w io.Writer) {
	out := fmt.Sprintf(
		" {%d: %s}",
		sk,
		SKString[sk],
	)
	w.Write([]byte(out))
}

const (
	SKInvalid = iota
	SKType
	SKModule
	SKFunc
	SKVar
	SKConst
	SKParam
)

var SKString = map[SymbolKind]string{
	SKInvalid: "invalid",
	SKType:    "type",
	SKModule:  "module",
	SKFunc:    "function",
	SKVar:     "variable",
	SKConst:   "constant",
	SKParam:   "Param",
}

type TypeKind int

func (tk TypeKind) LitterDump(w io.Writer) {
	name := TKString[tk]
	w.Write([]byte(" " + name))
}

const (
	TKInvalid = iota
	TKStruct
	TKEnum
	TKInterface
	TKAlias
	TKPrimitive
	TKSlice
	TKFunc
)

var TKString = map[TypeKind]string{
	TKInvalid:   "invalid",
	TKStruct:    "struct",
	TKEnum:      "enum",
	TKInterface: "interface",
	TKAlias:     "alias",
	TKPrimitive: "primitive",
	TKSlice:     "slice",
}

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

type Variant struct {
	Name   StringID
	Tag    uint32
	Fields FieldRange
}

type Param struct {
	Name StringID
	Type TypeID
}

type ParamRange struct {
	Start  uint32
	Length uint32
}

type Type struct {
	ID         TypeID
	Name       StringID
	Kind       TypeKind
	AliasFor   TypeID
	Fields     FieldRange
	Params     ParamRange
	ElemType   TypeID
	Variants   []Variant
	ReturnType TypeID
}

type TypeInterner struct {
	Types []*Type
	ByKey map[StringID]TypeID
}

func (ti *TypeInterner) Get(key StringID) *Type {
	id := ti.ByKey[key]
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

func (s *Scope) lookup(key StringID) (SymbolID, error) {
	if id, ok := s.Symbols[key]; ok {
		return id, nil
	}
	if s.Parent != nil {
		return s.Parent.lookup(key)
	}
	return 0, fmt.Errorf("not found")
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
		return fmt.Errorf("`%d` already declared in this scope", name)
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
	Params       []Param
}

func (c *Context) GetParams(pr ParamRange) []Param {
	start := pr.Start
	end := pr.Start + pr.Length
	return c.Params[start:end]
}

func (c *Context) lookupType(key StringID) (TypeID, error) {
	name := c.Strings.Get(key)
	sid, err := c.CurrentScope.lookup(key)
	if err != nil {
		return 0, fmt.Errorf("type `%s` not found", name)
	}
	sym := c.Symbols[sid]
	if sym.Kind != SKType {
		return 0, fmt.Errorf(
			"`%s` is not a type",
			name,
		)
	}
	return sym.Type, nil
}

func (c *Context) addField(f Field) {
	c.Fields = append(c.Fields, f)
}
func (c *Context) addParam(p Param) {
	c.Params = append(c.Params, p)
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

func (c *Context) seedBuiltins() {
	primitives := []string{
		"str", "int", "float", "bool",
	}
	for _, name := range primitives {
		t := &Type{
			Name: c.Strings.intern(name),
			Kind: TKPrimitive,
		}
		tid := c.Types.intern(t.Name, t)
		sym := Symbol{
			Scope: 0,
			Kind:  SKType,
			Name:  t.Name,
			Type:  tid,
		}
		sid := c.addSymbol(sym)
		if err := c.Universe.declareSymbol(sym.Name, sid); err != nil {
			panic(err)
		}
	}
}

func NewContext(modules []*loader.Module) *Context {
	c := &Context{Universe: newUniverse()}
	c.CurrentScope = c.Universe

	c.Types = NewTypeInterner()
	c.Strings = NewStringInterner()
	stringInterner = c.Strings
	typeInterner = c.Types

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

	invalidName := c.Strings.intern("invalid")
	c.Types.intern(
		invalidName,
		&Type{Name: invalidName, Kind: TKInvalid},
	)
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

	c.seedBuiltins()
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

func (c *Context) resolveNamedType(mod *Module, nt *ast.NamedType) TypeID {
	nid := c.Strings.intern(nt.Name)
	tid, err := c.lookupType(nid)
	if err != nil {
		c.diag(
			mod,
			nt,
			"%s",
			err.Error(),
		)
		return TypeInvalid
	}
	return tid
}

func (c *Context) resolveSliceType(
	mod *Module, node *ast.SliceType,
) TypeID {
	elemTID := c.resolveTypeExpr(mod, node.ElementType)
	key := fmt.Sprintf("[]%d", elemTID)
	kid := c.Strings.intern(key)
	tid := c.Types.intern(kid, &Type{
		Name:     kid,
		Kind:     TKSlice,
		ElemType: elemTID,
	})
	return tid
}

func (c *Context) resolveFunctionSignature(
	mod *Module, node *ast.FunctionSignature,
) TypeID {
	var paramNames []string
	for _, p := range node.Parameters {
		tid := c.resolveTypeExpr(mod, p.Type)
		typ := c.Types.Types[tid]
		n := strconv.Itoa(int(typ.Name))
		paramNames = append(paramNames, n)
	}
	paramsString := strings.Join(paramNames, ",")
	rt := c.resolveTypeExpr(mod, node.ReturnType)
	rtString := strconv.Itoa(int(rt))
	key := fmt.Sprintf(
		"fn(%s)->%s",
		paramsString,
		rtString,
	)
	kid := c.Strings.intern(key)
	tid := c.Types.intern(
		kid,
		&Type{Name: kid, Kind: TKFunc, ReturnType: rt},
	)
	return tid
}

func (c *Context) resolveTypeExpr(
	mod *Module, node ast.Type,
) TypeID {
	switch t := node.(type) {
	case *ast.SliceType:
		return c.resolveSliceType(mod, t)
	case *ast.NamedType:
		return c.resolveNamedType(mod, t)
	case *ast.FunctionSignature:
		return c.resolveFunctionSignature(mod, t)
	}
	return TypeInvalid
}

func (c *Context) resolveFields(
	fields []*ast.Field, mod *Module,
) FieldRange {
	fieldsStart := len(c.Fields)
	fieldsLen := 0
	for _, f := range fields {
		fname := c.Strings.intern(f.Name)
		tid := c.resolveTypeExpr(mod, f.Type)
		c.addField(Field{Name: fname, Type: tid})
		fieldsLen++
	}
	return FieldRange{Start: uint32(fieldsStart), Length: uint32(fieldsLen)}
}

func (c *Context) resolveParams(
	params []*ast.Parameter, mod *Module,
) ParamRange {
	start := len(c.Params)
	length := 0
	for _, f := range params {
		fname := c.Strings.intern(f.Name)
		tid := c.resolveTypeExpr(mod, f.Type)
		c.addParam(Param{Name: fname, Type: tid})
		length++
	}
	return ParamRange{Start: uint32(start), Length: uint32(length)}
}

func (c *Context) pass2Types(sym Symbol) {
	c.CurrentScope = c.GetScope(sym.Scope)
	t := c.Types.Types[sym.Type]
	switch t.Kind {
	case TKStruct:
		node := c.Nodes[sym.Node]
		def := node.(*ast.TypeDeclStmt).Definition.(*ast.StructType)
		t.Fields = c.resolveFields(def.Fields, c.Modules[sym.Mod])
	case TKEnum:
		node := c.Nodes[sym.Node]
		def := node.(*ast.TypeDeclStmt).Definition.(*ast.EnumType)
		for tag, v := range def.Members {
			variant := Variant{
				Name: c.Strings.intern(v.Name),
				Tag:  uint32(tag),
			}
			variant.Fields = c.resolveFields(v.Fields, c.Modules[sym.Mod])
			t.Variants = append(t.Variants, variant)
		}
	}
}

func (c *Context) pass2Funcs(sym Symbol) {
	mod := c.Modules[sym.Mod]
	node := c.Nodes[sym.Node]
	def := node.(*ast.FunctionStmt)
	tid := c.resolveTypeExpr(mod, def.Signature)
	c.Symbols[sym.ID].Type = tid
}

func (c *Context) pass2() {
	for _, sym := range c.Symbols {
		switch sym.Kind {
		case SKType:
			c.pass2Types(sym)
		case SKFunc:
			c.pass2Funcs(sym)
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
	c.pass2()
}
