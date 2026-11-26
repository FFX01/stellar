/* Package hir2 */
package hir2

import (
	"fmt"
	"io"
	"math"
	"stellar/internal/ast"
	"stellar/internal/loader"
	"strings"
)

var stringInterner *StringInterner

type BinaryOp int

const (
	OpAdd BinaryOp = iota // +
	OpSub                 // -
	OpMul                 // *
	OpDiv                 // /
	OpEq                  // ==
	OpLt                  // <
	OpLte
	OpGt
	OpGtE
	OpAnd // and
	OpOr  // or
)

var stringToBinaryOp = map[string]BinaryOp{
	"+":   OpAdd,
	"-":   OpSub,
	"*":   OpMul,
	"/":   OpDiv,
	"==":  OpEq,
	"<":   OpLt,
	"<=":  OpLte,
	">":   OpGt,
	">=":  OpGtE,
	"and": OpAnd,
	"or":  OpOr,
}

type UnaryOp int

const (
	OpNot UnaryOp = iota
	OpNeg
)

type StringID int

func (sid StringID) LitterDump(w io.Writer) {
	w.Write([]byte(": " + stringInterner.Get(sid)))
}

type StringInterner struct {
	Strings []string
	ByKey   map[string]StringID
}

func (si *StringInterner) Get(id StringID) string {
	return si.Strings[id]
}
func (si *StringInterner) intern(s string) StringID {
	if id, ok := si.ByKey[s]; ok {
		return id
	}

	sid := StringID(len(si.Strings))
	si.Strings = append(si.Strings, s)

	si.ByKey[s] = sid

	return sid
}

func newStringInterner() *StringInterner {
	si := &StringInterner{}
	si.ByKey = map[string]StringID{}
	return si
}

type TypeInterner struct {
	Types []*Type
	ByKey map[string]TypeID
}

func (ti *TypeInterner) Get(id TypeID) *Type {
	return ti.Types[id]
}
func (ti *TypeInterner) intern(key string, t *Type) TypeID {
	if id, ok := ti.ByKey[key]; ok {
		return id
	}

	tid := TypeID(len(ti.Types))
	t.ID = tid

	ti.Types = append(ti.Types, t)
	ti.ByKey[key] = tid

	return tid
}

func newTypeInterner() *TypeInterner {
	ti := &TypeInterner{}
	ti.ByKey = map[string]TypeID{}
	return ti
}

type TypeID int
type TypeKind uint8

const (
	TKInvalid TypeKind = iota
	TKPrimitive
	TKStruct
	TKInterface
)

type Type struct {
	ID   TypeID
	Kind TypeKind
}

type SymbolID int

const NoSymbol SymbolID = -1

type SymbolKind uint8

const (
	SKInvalid SymbolKind = iota
	SKStruct
	SKEnum
	SKVariant
	SKAlias
	SKInterface
	SKFunc
	SKFuncSig
	SKVar
	SKParam
	SKModule
	SKTypeParam
	SKConst
	SKField
	SKBuiltinType
)

type Symbol struct {
	ID           SymbolID
	Name         StringID
	Kind         SymbolKind
	IsPublic     bool
	Scope        ScopeID
	Module       ModID
	TargetModule ModID
	DataA        int
	DataB        int
	DataC        int
}

type ScopeID int

const NoScope ScopeID = -1

type Scope struct {
	ID      ScopeID
	Parent  ScopeID
	Symbols map[StringID]SymbolID
}

func (s *Scope) declare(name StringID, sid SymbolID) error {
	if _, ok := s.Symbols[name]; ok {
		return fmt.Errorf("already declared")
	}
	s.Symbols[name] = sid
	return nil
}

func newScope(
	parent ScopeID,
) *Scope {
	s := &Scope{Parent: parent}
	s.Symbols = map[StringID]SymbolID{}
	return s
}

type ModID int

const NoModule ModID = -1

type Module struct {
	ID    ModID
	Name  StringID
	Scope ScopeID
}

func newModule(name StringID, scope ScopeID) *Module {
	mod := &Module{
		Name:  name,
		Scope: scope,
	}
	return mod
}

type TypeExprID int

const NoTypeExpr TypeExprID = -1

type TypeExprKind int

const (
	TEKInvalid TypeExprKind = iota
	TEKNamed
	TEKGeneric
	TEKSlice
	TEKTuple
	TEKFunc
	TEKMaybe
	TEKResult
	TEKSelf
)

type TypeExpr struct {
	ID   TypeExprID
	Kind TypeExprKind
	// Named/Generic: SymbolID
	// slice/maybe/result: element TypeExprID
	// tuple/func: start index in extra
	DataA int
	// generic/tuple/func: count of items in extra
	DataB int
}

type FuncMetaID int

const InvalidFuncMetaID FuncMetaID = -1

type FuncMeta struct {
	ID         FuncMetaID
	BodyScope  ScopeID
	TypeParams []SymbolID
	Params     []SymbolID
	ReturnType TypeExprID
}

type StructMetaID int

const InvalidStructMeta StructMetaID = -1

type StructMeta struct {
	ID        StructMetaID
	BodyScope ScopeID
	Fields    []SymbolID
}

type InterfaceMetaID int

const InvalidInterfaceMetaID InterfaceMetaID = -1

type InterfaceMeta struct {
	ID        InterfaceMetaID
	BodyScope ScopeID
	Methods   []SymbolID
}

type EnumMetaID int

const InvalidEnumMetaID EnumMetaID = -1

type EnumMeta struct {
	ID        EnumMetaID
	BodyScope ScopeID
	Variants  []SymbolID
}

type ImplMetaID int

const InvalidImplMetaID ImplMetaID = -1

type ImplMeta struct {
	ID         ImplMetaID
	BodyScope  ScopeID
	TargetType TypeExprID
	Methods    []SymbolID
}

type ExprKind int

const (
	EKInvalid ExprKind = iota
	EKStrLit
	EKIntLit
	EKFloatLit
	EKBoolLit
	EKSliceLit
	EKTupleLit
	EKBlock
	EKVarRef
	EKUnresolved
	EKInfix
	EKUnary
)

type ExprID int

const InvalidExprID ExprID = -1

type Expr struct {
	ID                  ExprID
	Kind                ExprKind
	Op                  int
	DataA, DataB, DataC int
}

type Context struct {
	Strings *StringInterner
	Types   *TypeInterner

	Symbols        []Symbol
	Exprs          []Expr
	TypeExprs      []TypeExpr
	Extra          []int
	FuncMetas      []FuncMeta
	StructMetas    []StructMeta
	InterfaceMetas []InterfaceMeta
	EnumMetas      []EnumMeta
	ImplMetas      []ImplMeta

	Modules []*Module
	ModMap  map[*loader.Module]ModID

	Scopes   []*Scope
	Universe ScopeID
	Prelude  ScopeID

	PreludeModules []*loader.Module
	UserModules    []*loader.Module
}

func (c *Context) lookupIn(
	scope ScopeID, name StringID,
) (SymbolID, bool) {
	sc := c.GetScope(scope)
	if id, ok := sc.Symbols[name]; ok {
		return id, true
	}

	if sc.Parent != NoScope {
		return c.lookupIn(sc.Parent, name)
	}

	return NoSymbol, false
}

func (c *Context) addExpr(
	expr Expr,
) ExprID {
	id := ExprID(len(c.Exprs))
	expr.ID = id
	c.Exprs = append(c.Exprs, expr)
	return id
}

func (c *Context) addImplMeta(
	im ImplMeta,
) ImplMetaID {
	id := ImplMetaID(len(c.ImplMetas))
	im.ID = id
	c.ImplMetas = append(c.ImplMetas, im)
	return id
}

func (c *Context) addEnumMeta(
	em EnumMeta,
) EnumMetaID {
	id := EnumMetaID(len(c.EnumMetas))
	em.ID = id
	c.EnumMetas = append(c.EnumMetas, em)
	return id
}

func (c *Context) addStructMeta(
	sm StructMeta,
) StructMetaID {
	id := StructMetaID(len(c.StructMetas))
	sm.ID = id
	c.StructMetas = append(c.StructMetas, sm)
	return id
}

func (c *Context) addFuncMeta(fm FuncMeta) FuncMetaID {
	id := FuncMetaID(len(c.FuncMetas))
	fm.ID = id
	c.FuncMetas = append(c.FuncMetas, fm)
	return id
}
func (c *Context) addInterfaceMeta(
	im InterfaceMeta,
) InterfaceMetaID {
	id := InterfaceMetaID(len(c.InterfaceMetas))
	im.ID = id
	c.InterfaceMetas = append(c.InterfaceMetas, im)
	return id
}

func (c *Context) addExtra(items []int) (int, int) {
	start := len(c.Extra)
	length := 0
	for _, it := range items {
		c.Extra = append(c.Extra, it)
		length++
	}
	return start, length
}

func (c *Context) addTypeExpr(te TypeExpr) TypeExprID {
	id := TypeExprID(len(c.TypeExprs))
	te.ID = id
	c.TypeExprs = append(c.TypeExprs, te)
	return id
}

func (c *Context) addScope(sc *Scope) ScopeID {
	scid := ScopeID(len(c.Scopes))
	sc.ID = scid
	c.Scopes = append(c.Scopes, sc)
	return scid
}
func (c *Context) addSymbol(sym Symbol) SymbolID {
	sid := SymbolID(len(c.Symbols))
	sym.ID = sid
	c.Symbols = append(c.Symbols, sym)
	return sid
}
func (c *Context) addModule(mod *Module) ModID {
	mid := ModID(len(c.Modules))
	mod.ID = mid
	c.Modules = append(c.Modules, mod)
	return mid
}
func (c *Context) mapModule(lmod *loader.Module, mod ModID) {
	c.ModMap[lmod] = mod
}
func (c *Context) declareIn(
	scid ScopeID, name StringID, sid SymbolID,
) {
	scope := c.GetScope(scid)
	if err := scope.declare(name, sid); err != nil {
		msg := fmt.Sprintf(
			"error declaring `%s`: %v",
			c.Strings.Get(name),
			err,
		)
		panic(msg)
	}
}

func (c *Context) GetScope(scid ScopeID) *Scope {
	return c.Scopes[scid]
}

func NewContext(
	prelude, userMods []*loader.Module,
) *Context {
	c := &Context{
		PreludeModules: prelude,
		UserModules:    userMods,
		ModMap:         map[*loader.Module]ModID{},
	}

	c.Types = newTypeInterner()
	c.Strings = newStringInterner()
	stringInterner = c.Strings

	universeScope := newScope(NoScope)
	c.Universe = c.addScope(universeScope)
	preludeScope := newScope(c.Universe)
	c.Prelude = c.addScope(preludeScope)

	builtinMod := c.addModule(&Module{
		Name:  c.Strings.intern("<builtin>"),
		Scope: c.Universe,
	})

	buitlins := []string{
		"int", "str", "float", "bool", "Result",
		"Maybe", "void", "Self",
	}
	for _, bi := range buitlins {
		n := c.Strings.intern(bi)
		c.declareIn(
			c.Universe,
			n,
			c.addSymbol(Symbol{
				Name:         n,
				Kind:         SKBuiltinType,
				IsPublic:     true,
				Scope:        c.Universe,
				Module:       builtinMod,
				TargetModule: NoModule,
			}),
		)
	}

	return c
}

func (c *Context) collectStructDecl(
	mid ModID, scid ScopeID, node *ast.TypeDeclStmt, def *ast.StructType,
) {
	name := c.Strings.intern(node.Name)
	sym := Symbol{
		Name:         name,
		Kind:         SKStruct,
		IsPublic:     node.IsPublic,
		Scope:        scid,
		Module:       mid,
		TargetModule: NoModule,
	}

	structScope := newScope(scid)
	structScopeID := c.addScope(structScope)

	meta := StructMeta{
		BodyScope: structScopeID,
	}

	for _, f := range def.Fields {
		fname := c.Strings.intern(f.Name)
		sym := Symbol{
			Name:         fname,
			Kind:         SKField,
			Scope:        structScopeID,
			Module:       mid,
			TargetModule: NoModule,
			IsPublic:     f.IsPublic,
		}
		sym.DataA = int(c.resolveTypeExpr(structScopeID, f.Type))
		sid := c.addSymbol(sym)
		c.declareIn(structScopeID, fname, sid)
		meta.Fields = append(meta.Fields, sid)
	}

	sym.DataA = int(c.addStructMeta(meta))

	sid := c.addSymbol(sym)
	c.declareIn(scid, name, sid)
}

func (c *Context) collectInterfaceDecl(
	mid ModID, scid ScopeID, node *ast.TypeDeclStmt, def *ast.InterfaceType,
) {
	name := c.Strings.intern(node.Name)
	sym := Symbol{
		Name:         name,
		Kind:         SKInterface,
		IsPublic:     node.IsPublic,
		Scope:        scid,
		Module:       mid,
		TargetModule: NoModule,
	}

	iScope := newScope(scid)
	iScopeID := c.addScope(iScope)

	meta := InterfaceMeta{
		BodyScope: iScopeID,
	}

	for _, m := range def.Methods {
		sid := c.collectFuncSig(mid, iScopeID, nil, m)
		meta.Methods = append(meta.Methods, sid)
	}

	sym.DataA = int(c.addInterfaceMeta(meta))

	sid := c.addSymbol(sym)
	c.declareIn(scid, name, sid)
}

func (c *Context) collectEnumDecl(
	mod ModID, scope ScopeID, node *ast.TypeDeclStmt, def *ast.EnumType,
) {
	name := c.Strings.intern(node.Name)
	sym := Symbol{
		Name:         name,
		Kind:         SKEnum,
		IsPublic:     node.IsPublic,
		Scope:        scope,
		Module:       mod,
		TargetModule: NoModule,
	}

	eScope := newScope(scope)
	eScopeID := c.addScope(eScope)
	sym.DataC = int(eScopeID)
	meta := EnumMeta{
		BodyScope: eScopeID,
	}

	for _, v := range def.Members {
		sym := Symbol{
			Name:         c.Strings.intern(v.Name),
			Kind:         SKVariant,
			Module:       mod,
			Scope:        eScopeID,
			TargetModule: NoModule,
		}
		vScope := newScope(eScopeID)
		vScopeID := c.addScope(vScope)
		sym.DataC = int(vScopeID)

		var extra []int
		for _, f := range v.Fields {
			sym := Symbol{
				Name:         c.Strings.intern(f.Name),
				Kind:         SKField,
				Module:       mod,
				Scope:        vScopeID,
				TargetModule: NoModule,
			}
			sym.DataA = int(c.resolveTypeExpr(vScopeID, f.Type))
			sid := c.addSymbol(sym)
			c.declareIn(vScopeID, sym.Name, sid)
			extra = append(extra, int(sid))
		}

		sym.DataA, sym.DataB = c.addExtra(extra)
		sid := c.addSymbol(sym)
		c.declareIn(eScopeID, sym.Name, sid)
		meta.Variants = append(meta.Variants, sid)
	}

	metaID := c.addEnumMeta(meta)
	sym.DataA = int(metaID)

	sid := c.addSymbol(sym)
	c.declareIn(scope, name, sid)
}

func (c *Context) collectTypeDeclStmt(
	mid ModID, scid ScopeID, node *ast.TypeDeclStmt,
) {
	switch t := node.Definition.(type) {
	case *ast.StructType:
		c.collectStructDecl(mid, scid, node, t)
	case *ast.EnumType:
		c.collectEnumDecl(mid, scid, node, t)
	case *ast.InterfaceType:
		c.collectInterfaceDecl(mid, scid, node, t)
	}
}

func (c *Context) collectTypeAliasStmt(
	mod ModID, scope ScopeID, node *ast.TypeAliasStmt,
) {
	name := c.Strings.intern(node.Name)
	sym := Symbol{
		Name:         name,
		Kind:         SKAlias,
		Scope:        scope,
		Module:       mod,
		TargetModule: NoModule,
		IsPublic:     node.IsPublic,
	}
	sym.DataA = int(c.resolveTypeExpr(scope, node.Value))

	sid := c.addSymbol(sym)
	c.declareIn(scope, name, sid)
}

func (c *Context) collectFuncStmt(
	mod ModID, scope ScopeID, node *ast.FunctionStmt,
) SymbolID {
	sid := c.collectFuncSig(mod, scope, node, node.Signature)
	sym := c.Symbols[sid]
	meta := c.FuncMetas[FuncMetaID(sym.DataA)]
	sym.DataB = int(c.collectExpr(meta.BodyScope, node.Body))
	c.Symbols[sid] = sym
	return sid
}

func (c *Context) collectFuncSig(
	mod ModID, scope ScopeID, stmt *ast.FunctionStmt, sig *ast.FunctionSignature,
) SymbolID {
	name := c.Strings.intern(sig.Name)
	kind := SKFunc
	if stmt == nil {
		kind = SKFuncSig
	}
	fSym := Symbol{
		Name:         name,
		Kind:         kind,
		Scope:        scope,
		Module:       mod,
		TargetModule: NoModule,
	}
	if stmt != nil {
		fSym.IsPublic = stmt.IsPublic
	}

	fScope := newScope(scope)
	fScopeID := c.addScope(fScope)

	meta := FuncMeta{
		BodyScope: fScopeID,
	}
	if sig.ReturnType != nil {
		meta.ReturnType = c.resolveTypeExpr(scope, sig.ReturnType)
	} else {
		void := TypeExpr{
			Kind:  TEKNamed,
			DataA: int(c.Strings.intern("void")),
			DataB: int(scope),
		}
		meta.ReturnType = c.addTypeExpr(void)
	}

	if stmt != nil {
		for _, tp := range sig.TypeParams {
			tpName := c.Strings.intern(tp.Token.Literal)
			sym := Symbol{
				Name:         tpName,
				Kind:         SKTypeParam,
				Scope:        fScopeID,
				Module:       mod,
				TargetModule: NoModule,
			}
			if tp.Bound != nil {
				sym.DataA = int(c.resolveTypeExpr(fScopeID, tp.Bound))
			}
			sid := c.addSymbol(sym)
			c.declareIn(fScopeID, tpName, sid)
			meta.TypeParams = append(meta.TypeParams, sid)
		}
	}

	for _, p := range sig.Parameters {
		pName := c.Strings.intern(p.Name)
		sym := Symbol{
			Name:         pName,
			Kind:         SKParam,
			Scope:        fScopeID,
			Module:       mod,
			TargetModule: NoModule,
		}
		if p.Name == "self" && p.Type == nil {
			texpr := TypeExpr{
				Kind:  TEKSelf,
				DataA: int(c.Strings.intern("Self")),
				DataB: int(fScopeID),
			}
			teid := c.addTypeExpr(texpr)
			sym.DataA = int(teid)
		} else {
			sym.DataA = int(c.resolveTypeExpr(scope, p.Type))
		}
		sid := c.addSymbol(sym)
		c.declareIn(fScopeID, pName, sid)
		meta.Params = append(meta.Params, sid)
	}

	fSym.DataA = int(c.addFuncMeta(meta))

	fsid := c.addSymbol(fSym)
	c.declareIn(scope, name, fsid)
	return fsid
}

func (c *Context) collectConst(
	mod ModID, scope ScopeID, node *ast.VarStmt,
) {
	name := c.Strings.intern(node.Name.Literal)
	sym := Symbol{
		Name:         name,
		Kind:         SKConst,
		Scope:        scope,
		Module:       mod,
		TargetModule: NoModule,
		IsPublic:     node.IsPublic,
	}

	if node.Type != nil {
		sym.DataA = int(c.resolveTypeExpr(scope, node.Type))
	}

	sid := c.addSymbol(sym)

	c.declareIn(scope, name, sid)
}

func (c *Context) collectExpr(
	scope ScopeID, node ast.Node,
) ExprID {
	switch t := node.(type) {
	case *ast.IntegerLiteralExpr:
		expr := Expr{
			Kind:  EKIntLit,
			DataA: int(t.Value),
		}
		return c.addExpr(expr)
	case *ast.StringLiteralExpr:
		expr := Expr{
			Kind:  EKStrLit,
			DataA: int(c.Strings.intern(t.Value)),
		}
		return c.addExpr(expr)
	case *ast.FloatLiteralExpr:
		expr := Expr{
			Kind:  EKFloatLit,
			DataA: int(math.Float64frombits(uint64(t.Value))),
		}
		return c.addExpr(expr)
	case *ast.BoolLiteralExpr:
		expr := Expr{
			Kind: EKBoolLit,
		}
		if t.Value {
			expr.DataA = 1
		}
		return c.addExpr(expr)
	case *ast.IdentifierExpr:
		sid, ok := c.lookupIn(scope, c.Strings.intern(t.Value))
		if !ok {
			return c.addExpr(Expr{
				Kind:  EKUnresolved,
				DataA: int(c.Strings.intern(t.Value)),
				DataB: int(scope),
			})
		} else {
			return c.addExpr(Expr{
				Kind:  EKVarRef,
				DataA: int(sid),
			})
		}
	case *ast.InfixExpr:
		expr := Expr{
			Kind: EKInfix,
			Op:   int(stringToBinaryOp[t.Operator]),
		}
		expr.DataA = int(c.collectExpr(scope, t.Left))
		expr.DataB = int(c.collectExpr(scope, t.Right))
		return c.addExpr(expr)
	case *ast.NotExpr:
		expr := Expr{
			Kind: EKUnary,
			Op:   int(OpNot),
		}
		expr.DataA = int(c.collectExpr(scope, t.Value))
		return c.addExpr(expr)
	case *ast.NegativeExpr:
		expr := Expr{
			Kind: EKUnary,
			Op:   int(OpNeg),
		}
		expr.DataA = int(c.collectExpr(scope, t.Value))
		return c.addExpr(expr)
	case *ast.BlockExpr:
		expr := Expr{
			Kind: EKBlock,
		}
		exprScopeID := c.addScope(newScope(scope))
		expr.DataC = int(exprScopeID)
		var extra []int
		for _, n := range t.Statements {
			extra = append(extra, int(c.collectExpr(exprScopeID, n)))
		}
		expr.DataA, expr.DataB = c.addExtra(extra)
		return c.addExpr(expr)
	}
	return InvalidExprID
}

func (c *Context) resolveTypeExpr(scope ScopeID, t ast.Type) TypeExprID {
	switch t := t.(type) {
	case *ast.NamedType:
		te := TypeExpr{
			Kind:  TEKNamed,
			DataA: int(c.Strings.intern(t.Name)),
			DataB: int(scope),
		}
		return c.addTypeExpr(te)
	case *ast.SliceType:
		te := TypeExpr{
			Kind:  TEKSlice,
			DataA: int(c.resolveTypeExpr(scope, t.ElementType)),
		}
		return c.addTypeExpr(te)
	case *ast.TupleType:
		te := TypeExpr{
			Kind: TEKTuple,
		}
		var extra []int
		for _, et := range t.ElementTypes {
			eteid := c.resolveTypeExpr(scope, et)
			extra = append(extra, int(eteid))
		}
		te.DataA, te.DataB = c.addExtra(extra)
		return c.addTypeExpr(te)
	case *ast.ResultType:
		te := TypeExpr{
			Kind: TEKResult,
		}
		te.DataA = int(c.resolveTypeExpr(scope, t.Value))
		return c.addTypeExpr(te)
	case *ast.MaybeType:
		te := TypeExpr{
			Kind: TEKMaybe,
		}
		te.DataA = int(c.resolveTypeExpr(scope, t.Value))
		return c.addTypeExpr(te)
	case *ast.FunctionType:
		te := TypeExpr{
			Kind: TEKFunc,
		}
		var extra []int
		extra = append(extra, int(c.resolveTypeExpr(scope, t.ReturnType)))
		for _, p := range t.Parameters {
			extra = append(extra, int(c.resolveTypeExpr(scope, p.Type)))
		}
		te.DataA, te.DataB = c.addExtra(extra)
		return c.addTypeExpr(te)
	default:
		return NoTypeExpr
	}
}

func (c *Context) collectImplDecl(
	mod ModID, scope ScopeID, node *ast.ImplDeclStmt,
) {
	target := c.resolveTypeExpr(scope, node.Target)
	implScope := newScope(scope)
	implScopeID := c.addScope(implScope)
	meta := ImplMeta{
		TargetType: target,
		BodyScope:  implScopeID,
	}

	for _, m := range node.Methods {
		sid := c.collectFuncStmt(mod, meta.BodyScope, m)
		meta.Methods = append(meta.Methods, sid)
	}

	c.addImplMeta(meta)
}

func (c *Context) collectDeclarations(
	mid ModID, scid ScopeID, lmod *loader.Module,
) {
	for _, stmt := range lmod.Program.Statements {
		switch t := stmt.(type) {
		case *ast.TypeDeclStmt:
			c.collectTypeDeclStmt(mid, scid, t)
		case *ast.TypeAliasStmt:
			c.collectTypeAliasStmt(mid, scid, t)
		case *ast.VarStmt:
			c.collectConst(mid, scid, t)
		case *ast.FunctionStmt:
			c.collectFuncStmt(mid, scid, t)
		case *ast.ImplDeclStmt:
			c.collectImplDecl(mid, scid, t)
		}
	}
}

func (c *Context) declareImports(
	mod ModID, scope ScopeID, lmod *loader.Module,
) {
	for _, stmt := range lmod.Program.Statements {
		imp, ok := stmt.(*ast.ImportStmt)
		if !ok {
			continue
		}

		key := importKey(imp)
		dep, ok := lmod.Imports[key]
		if !ok {
			continue
		}
		target, ok := c.ModMap[dep]
		if !ok {
			panic(fmt.Sprintf(
				"module `%s` not collected before `%s`",
				key,
				lmod.File,
			))
		}

		name := c.Strings.intern(imp.Path[len(imp.Path)-1].Literal)
		sid := c.addSymbol(Symbol{
			Name:         name,
			Kind:         SKModule,
			Module:       mod,
			TargetModule: target,
			Scope:        scope,
		})
		c.declareIn(scope, name, sid)
	}
}

func importKey(imp *ast.ImportStmt) string {
	parts := make([]string, len(imp.Path))
	for idx, t := range imp.Path {
		parts[idx] = t.Literal
	}
	return strings.Join(parts, "::")
}

func (c *Context) collectGroup(
	mods []*loader.Module, parent ScopeID,
) {
	for _, mod := range mods {
		modScope := newScope(parent)
		scid := c.addScope(modScope)
		name := c.Strings.intern(mod.Name)
		hirMod := newModule(name, scid)
		mid := c.addModule(hirMod)
		c.mapModule(mod, mid)

		c.declareImports(mid, scid, mod)
		c.collectDeclarations(mid, scid, mod)
	}
}

func (c *Context) fillPreludeScope() {
	if len(c.PreludeModules) < 1 {
		return
	}

	root := c.PreludeModules[len(c.PreludeModules)-1]
	mid, ok := c.ModMap[root]
	if !ok {
		panic("prelude root module not collected")
	}

	src := c.GetScope(c.Modules[mid].Scope)
	dst := c.GetScope(c.Prelude)
	for name, sid := range src.Symbols {
		if c.Symbols[sid].IsPublic {
			dst.Symbols[name] = sid
		}
	}
}

func (c *Context) pass1A() {
	c.collectGroup(c.PreludeModules, c.Universe)
	c.fillPreludeScope()
	c.collectGroup(c.UserModules, c.Prelude)
}

func (c *Context) Run() {
	c.pass1A()
}
