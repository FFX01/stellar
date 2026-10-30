package analyzer

import "stellar/internal/ast"

func (a *Analyzer) resolve() {}

func (a *Analyzer) resolveResultNamedType(t *ast.NamedType) Type {
	symT := &ResultType{}
	if len(t.Args) < 1 {
		symT.ValueType = VoidT
	} else if len(t.Args) > 1 {
		a.addError(t, "Result takes exactly 0 or 1 type arguments")
		symT.ValueType = InvalidT
	} else {
		symT.ValueType = a.resolveTypeExpr(t.Args[0])
	}
	return symT
}

func (a *Analyzer) resolveMaybeNamedType(t *ast.NamedType) Type {
	symT := &MaybeType{}
	if len(t.Args) < 1 {
		a.addError(t, "Maybe takes exactly 1 type argument")
		symT.ValueType = InvalidT
	} else if len(t.Args) > 1 {
		a.addError(t, "Maybe takes exactly 1 type argument")
		symT.ValueType = InvalidT
	} else {
		symT.ValueType = a.resolveTypeExpr(t.Args[0])
	}
	return symT
}

func (a *Analyzer) resolveNamedType(t *ast.NamedType) Type {
	if len(t.Path) > 0 {
		a.addError(t, "qualified types are not yet implemented")
		return InvalidT
	}
	sym, _ := a.CurrentScope.Lookup(t.Name)
	if sym == nil {
		a.addError(t, "undefined type: `%s`", t.Name)
		return InvalidT
	}
	if _, ok := sym.(*TypeSymbol); !ok {
		a.addError(t, "%s is not a type", t.Name)
		return InvalidT
	}
	if sym == ResultSym {
		return a.resolveResultNamedType(t)
	}
	if sym == MaybeSym {
		return a.resolveMaybeNamedType(t)
	}
	if len(t.Args) > 0 {
		a.addError(t, "generic types are not yet supported")
		return InvalidT
	}
	if sym.base().Type != nil {
		return sym.base().Type
	}
	return a.resolveSymbol(sym)
}

func (a *Analyzer) resolveFunctionType(t *ast.FunctionType) Type {
	symT := &FunctionType{}
	for _, p := range t.Parameters {
		resolvedT := a.resolveTypeExpr(p.Type)
		symT.Params = append(symT.Params, Param{Type: resolvedT, Name: p.Name})
	}
	if t.ReturnType != nil {
		symT.ReturnType = a.resolveTypeExpr(t.ReturnType)
	} else {
		symT.ReturnType = VoidT
	}
	return symT

}

func (a *Analyzer) resolveTypeExpr(t ast.Type) Type {
	switch tt := t.(type) {
	case nil:
		return nil
	case *ast.NamedType:
		return a.resolveNamedType(tt)
	case *ast.SliceType:
		return &SliceType{ElemType: a.resolveTypeExpr(tt.ElementType)}
	case *ast.ResultType:
		return &ResultType{ValueType: a.resolveTypeExpr(tt.Value)}
	case *ast.MaybeType:
		return &MaybeType{ValueType: a.resolveTypeExpr(tt.Value)}
	case *ast.FunctionType:
		return a.resolveFunctionType(tt)
	case *ast.StructType, *ast.EnumType, *ast.InterfaceType:
		a.addError(tt, "a struct type must be declared with `type`")
		return InvalidT
	}
	a.addError(t, "not implemented: %T", t)
	return InvalidT
}

func (a *Analyzer) declareTypeParams(
	params []*ast.TypeParameter, name string,
) {
	if len(params) > 0 {
		a.enterScope(BlockScope, name)
		for _, typeParam := range params {
			sym := &TypeSymbol{symbolBase{
				Node:  typeParam,
				Name:  typeParam.Token.Literal,
				Type:  InvalidT,
				State: Resolved,
			}}
			a.declare(sym)
		}
	}
}

func (a *Analyzer) buildFunctionType(
	sig *ast.FunctionSignature, self Type,
) *FunctionType {
	ft := &FunctionType{}
	if len(sig.TypeParams) > 0 {
		defer a.exitScope()
		a.addError(
			sig.TypeParams[0],
			"Generic types are not yet implemented for functions",
		)
		a.declareTypeParams(sig.TypeParams, sig.Name)
	}
	for idx, p := range sig.Parameters {
		symParam := Param{Name: p.Name}
		if p.Name == "self" {
			switch {
			case idx == 0 && self != nil && p.Type == nil:
				symParam.Type = self
			case idx != 0 || self == nil:
				a.addError(
					p,
					"`self` must be the first parameter of a method",
				)
				symParam.Type = InvalidT
			case p.Type != nil:
				a.addError(
					p,
					"you cannot specify a type for `self`",
				)
				symParam.Type = self
			}
		} else {
			tt := a.resolveTypeExpr(p.Type)
			if tt == nil {
				a.addError(
					p,
					"Parameter `%s` missing a type",
					p.Name,
				)
				symParam.Type = InvalidT
			} else {
				symParam.Type = tt
			}
		}
		ft.Params = append(ft.Params, symParam)
	}
	ft.ReturnType = a.resolveTypeExpr(sig.ReturnType)
	if ft.ReturnType == nil {
		ft.ReturnType = VoidT
	}
	return ft
}

func (a *Analyzer) resolveSymbol(sym Symbol) Type {
	curScope := a.CurrentScope
	b := sym.base()
	switch b.State {
	case Resolved:
		return b.Type
	case Resolving:
		a.addError(b.Node, "`%s` refers to itself", b.Name)
		return InvalidT
	}
	b.State = Resolving
	a.CurrentScope = a.Package
	switch s := sym.(type) {
	case *TypeSymbol:
		a.resolveTypeDecl(s)
	default:
		a.addError(b.Node, "Not implemented: resolving %T", sym)
	}
	a.CurrentScope = curScope
	b.State = Resolved

	if b.Type == nil {
		b.Type = InvalidT
	}
	return b.Type
}

func (a *Analyzer) resolveTypeDecl(s *TypeSymbol) {
	switch t := s.Node.(type) {
	case *ast.TypeAliasStmt:
		if len(t.TypeParams) > 0 {
			defer a.exitScope()
			a.addError(
				t.TypeParams[0],
				"generic types are not yet supported",
			)
			a.declareTypeParams(t.TypeParams, s.Name)
		}
		s.Type = a.resolveTypeExpr(t.Value)
	case *ast.TypeDeclStmt:
		a.resolveTypeBody(s, t)
	default:
		a.addError(
			s.Node,
			"not implemented: type declaration: %T", s.Node,
		)
	}
}

func (a *Analyzer) resolveStructTypeBody(
	s *StructType, def *ast.StructType, node *ast.TypeDeclStmt,
) {
	fieldNames := map[string]bool{}
	for _, f := range def.Fields {
		if _, ok := fieldNames[f.Name]; ok {
			a.addError(
				f,
				"Duplicate field %s in %s",
				f.Name,
				node.Name,
			)
			continue
		}
		symF := FieldInfo{Name: f.Name, IsPublic: f.IsPublic}
		symF.Type = a.resolveTypeExpr(f.Type)
		s.Fields = append(s.Fields, symF)
		fieldNames[f.Name] = true
	}
}

func (a *Analyzer) resolveEnumTypeBody(
	s *EnumType, def *ast.EnumType, node *ast.TypeDeclStmt,
) {
	memberNames := map[string]bool{}
	for _, m := range def.Members {
		if _, ok := memberNames[m.Name]; ok {
			a.addError(
				m,
				"Duplicate member %s in %s",
				m.Name,
				node.Name,
			)
			continue
		}
		symM := MemberInfo{Name: m.Name}
		fieldNames := map[string]bool{}
		for _, f := range m.Fields {
			if _, ok := fieldNames[f.Name]; ok {
				a.addError(
					f,
					"duplicate field %s in %s.%s",
					f.Name,
					node.Name,
					m.Name,
				)
				continue
			}
			symF := FieldInfo{Name: f.Name}
			symF.Type = a.resolveTypeExpr(f.Type)
			symM.Fields = append(symM.Fields, symF)
			fieldNames[f.Name] = true
		}
		s.Members = append(s.Members, symM)
		memberNames[m.Name] = true
	}
}

func (a *Analyzer) resolveInterfaceTypeBody(
	s *InterfaceType, def *ast.InterfaceType, node *ast.TypeDeclStmt,
) {
	s.Methods = map[string]*FunctionType{}
	methodNames := map[string]bool{}
	for _, method := range def.Methods {
		if _, ok := methodNames[method.Name]; ok {
			a.addError(
				method,
				"duplicate method `%s` in `%s`",
				method.Name,
				node.Name,
			)
			continue
		}
		if len(method.Parameters) < 1 || method.Parameters[0].Name != "self" {
			a.addError(
				method,
				"interface method `%s` needs `self`",
				method.Name,
			)
		}
		s.Methods[method.Name] = a.buildFunctionType(method, SelfT)
		s.MethodOrder = append(s.MethodOrder, method.Name)
		methodNames[method.Name] = true
	}
}

func (a *Analyzer) resolveTypeBody(s *TypeSymbol, node *ast.TypeDeclStmt) {
	if len(node.TypeParams) > 0 {
		defer a.exitScope()
		a.addError(node, "generic types are not supported yet")
		a.declareTypeParams(node.TypeParams, s.Name)
	}
	switch def := node.Definition.(type) {
	case *ast.StructType:
		t := s.Type.(*StructType)
		a.resolveStructTypeBody(t, def, node)
	case *ast.EnumType:
		t := s.Type.(*EnumType)
		a.resolveEnumTypeBody(t, def, node)
	case *ast.InterfaceType:
		t := s.Type.(*InterfaceType)
		a.resolveInterfaceTypeBody(t, def, node)
	}
}
