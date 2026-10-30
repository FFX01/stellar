package analyzer

import (
	"io"
	"strings"
)

var indent = "  "
var unresolved = "<unresolved>"

func (a *Analyzer) Dump(w io.Writer) {
	dumpScope(w, a.Package, 0)
	if len(a.Errors) > 0 {
		io.WriteString(w, "--- Errors ---\n")
		for _, e := range a.Errors {
			io.WriteString(w, e.String()+"\n")
		}
	}
}

func dumpScope(w io.Writer, scope *Scope, depth int) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr+scope.Name+"\n")
	for _, name := range scope.order {
		sym := scope.LookupLocal(name)
		if sym != nil {
			writeSymbol(w, sym, depth)
			io.WriteString(w, "\n")
		}
	}
	if len(scope.Children) > 0 {
		for _, s := range scope.Children {
			dumpScope(w, s, depth+1)
		}
	}
}

func writeVarSymbol(w io.Writer, sym *VarSymbol, depth int) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr+"var "+sym.Name+": ")
	if sym.base().Type != nil {
		io.WriteString(w, sym.base().Type.String())
	} else {
		io.WriteString(w, unresolved)
	}
}

func writeParam(w io.Writer, param Param, depth int) {}

func writeFuncSymbol(w io.Writer, sym *FuncSymbol, depth int) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr+"fn "+sym.Name+": ")
	if sym.Type != nil {
		io.WriteString(w, sym.Type.String())
	} else {
		io.WriteString(w, unresolved)
	}
}

func writeFieldInfo(w io.Writer, info FieldInfo, depth int) {
	depthStr := strings.Repeat(indent, depth)
	pubStr := ""
	if info.IsPublic {
		pubStr = "pub "
	}
	io.WriteString(w, depthStr+pubStr)
	io.WriteString(w, "field "+info.Name)
	if info.Type != nil {
		io.WriteString(w, ": "+info.Type.String())
	}
}

func writeMethodInfo(w io.Writer, info *MethodInfo, depth int) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr)
	if info.IsPublic {
		io.WriteString(w, "pub ")
	}
	io.WriteString(w, "method "+info.Name+": ")
	io.WriteString(w, info.Type.String())
}

func writeStructType(w io.Writer, name string, typ *StructType, depth int, pub bool) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr)
	if pub {
		io.WriteString(w, "pub ")
	}
	io.WriteString(w, "type "+name+": struct")
	if len(typ.Fields) > 0 {
		io.WriteString(w, "\n")
		for idx, f := range typ.Fields {
			writeFieldInfo(w, f, depth+1)
			if idx < len(typ.Fields)-1 {
				io.WriteString(w, "\n")
			}
		}
	}
	if len(typ.Methods.order) > 0 {
		io.WriteString(w, "\n")
		for idx, mi := range typ.Methods.All() {
			writeMethodInfo(w, mi, depth+1)
			if idx < len(typ.Methods.order)-1 {
				io.WriteString(w, "\n")
			}
		}
	}
}

func writeMemberInfo(w io.Writer, info MemberInfo, depth int) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr+"member "+info.Name)
	if len(info.Fields) > 0 {
		io.WriteString(w, "(")
		for idx, f := range info.Fields {
			io.WriteString(w, f.Name+": "+f.Type.String())
			if idx < len(info.Fields)-1 {
				io.WriteString(w, ", ")
			}
		}
		io.WriteString(w, ")")
	}
}

func writeEnumType(w io.Writer, name string, typ *EnumType, depth int, pub bool) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr)
	if pub {
		io.WriteString(w, "pub ")
	}
	io.WriteString(w, "type "+name+": enum")
	if len(typ.Members) > 0 {
		io.WriteString(w, "\n")
		for idx, m := range typ.Members {
			writeMemberInfo(w, m, depth+1)
			if idx < len(typ.Members)-1 {
				io.WriteString(w, "\n")
			}
		}
	}
	if len(typ.Methods.order) > 0 {
		io.WriteString(w, "\n")
		for idx, mi := range typ.Methods.All() {
			writeMethodInfo(w, mi, depth+1)
			if idx < len(typ.Methods.order)-1 {
				io.WriteString(w, "\n")
			}
		}
	}
}

func writeInterfaceType(w io.Writer, name string, typ *InterfaceType, depth int) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr+"type "+name+": interface")
	if len(typ.MethodOrder) > 0 {
		io.WriteString(w, "\n")
		for idx, name := range typ.MethodOrder {
			ft, ok := typ.Methods[name]
			if ok {
				io.WriteString(w, depthStr+"  method "+name+": "+ft.String())
				if idx < len(typ.MethodOrder)-1 {
					io.WriteString(w, "\n")
				}
			}
		}
	}
}

func writeSliceType(w io.Writer, name string, typ *SliceType, depth int) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr+"type "+name+": []")
	if typ.ElemType != nil {
		io.WriteString(w, typ.ElemType.String())
	} else {
		io.WriteString(w, unresolved)
	}
}

func writeResultType(w io.Writer, name string, typ *ResultType, depth int) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr+"type "+name+": Result[")
	if typ.ValueType != nil {
		io.WriteString(w, typ.ValueType.String())
	} else {
		io.WriteString(w, unresolved)
	}
	io.WriteString(w, "]")
}

func writeMaybeType(w io.Writer, name string, typ *MaybeType, depth int) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr+"type "+name+": Maybe[")
	if typ.ValueType != nil {
		io.WriteString(w, typ.ValueType.String())
	} else {
		io.WriteString(w, unresolved)
	}
	io.WriteString(w, "]")
}

func writeTypeSymbol(w io.Writer, sym *TypeSymbol, depth int) {
	switch tt := sym.base().Type.(type) {
	case *StructType:
		writeStructType(w, sym.Name, tt, depth, sym.IsPublic)
	case *EnumType:
		writeEnumType(w, sym.Name, tt, depth, sym.IsPublic)
	case *InterfaceType:
		writeInterfaceType(w, sym.Name, tt, depth)
	case *SliceType:
		writeSliceType(w, sym.Name, tt, depth)
	case *ResultType:
		writeResultType(w, sym.Name, tt, depth)
	case *MaybeType:
		writeMaybeType(w, sym.Name, tt, depth)
	}
}

func writeModuleSymbol(w io.Writer, sym *ModuleSymbol, depth int) {
	depthStr := strings.Repeat(indent, depth)
	io.WriteString(w, depthStr+"module "+sym.Name)
}

func writeSymbol(w io.Writer, sym Symbol, depth int) {
	switch t := sym.(type) {
	case *VarSymbol:
		writeVarSymbol(w, t, depth)
	case *FuncSymbol:
		writeFuncSymbol(w, t, depth)
	case *TypeSymbol:
		writeTypeSymbol(w, t, depth)
	case *ModuleSymbol:
		writeModuleSymbol(w, t, depth)
	}
}
