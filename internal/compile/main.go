package compile

import (
	"fmt"
	"log"
	"os"
	"stellar/internal/hir"

	"tinygo.org/x/go-llvm"
)

type Generator struct {
	ctx    llvm.Context
	mod    llvm.Module
	b      llvm.Builder
	fns    map[*hir.Function]llvm.Value
	ftypes map[*hir.Function]llvm.Type
	vars   map[*hir.Variable]llvm.Value
	rt     struct {
		init, print   llvm.Value
		initT, printT llvm.Type
	}
	mainF llvm.Value
	mainT llvm.Type
	t     struct {
		int, bool, float, str, i32, main, void llvm.Type
	}
}

func NewGenerator() *Generator {
	g := &Generator{}
	g.fns = map[*hir.Function]llvm.Value{}
	g.ftypes = map[*hir.Function]llvm.Type{}
	g.vars = map[*hir.Variable]llvm.Value{}
	g.ctx = llvm.NewContext()
	g.mod = g.ctx.NewModule("stellar_module")

	// initializeglobal types
	g.t.int = g.ctx.Int64Type()
	g.t.bool = g.ctx.Int1Type()
	g.t.float = g.ctx.DoubleType()
	g.t.str = llvm.PointerType(g.ctx.Int8Type(), 0)
	g.t.i32 = g.ctx.Int32Type()
	g.t.void = g.ctx.VoidType()

	// initialize builtins
	g.rt.initT = llvm.FunctionType(g.t.void, nil, false)
	g.rt.init = llvm.AddFunction(g.mod, "stellar_init", g.rt.initT)
	g.rt.printT = llvm.FunctionType(g.t.void, []llvm.Type{g.t.str}, false)
	g.rt.print = llvm.AddFunction(g.mod, "stellar_print", g.rt.printT)

	return g
}

func Compile(a *hir.Analyzer) {
	llvm.InitializeAllTargets()
	llvm.InitializeAllTargetMCs()
	llvm.InitializeAllTargetInfos()
	llvm.InitializeAllAsmPrinters()

	triple := llvm.DefaultTargetTriple()
	target, err := llvm.GetTargetFromTriple(triple)
	if err != nil {
		log.Fatal(err.Error())
	}
	tm := target.CreateTargetMachine(triple, "generic", "",
		llvm.CodeGenLevelDefault, llvm.RelocPIC, llvm.CodeModelDefault)
	defer tm.Dispose()
	td := tm.CreateTargetData()
	defer td.Dispose()

	g := NewGenerator()

	g.mod.SetTarget(triple)
	g.mod.SetDataLayout(td.String())

	// allocType := llvm.FunctionType(ptrType, []llvm.Type{td.IntPtrType()}, false)
	// allocFn := llvm.AddFunction(module, "stellar_alloc", allocType)

	g.b = g.ctx.NewBuilder()
	defer g.b.Dispose()

	// build user code
	g.addValues(a)

	// wrap stellar init and stellar main in C main
	cMainT := llvm.FunctionType(g.t.i32, nil, false)
	cMainF := llvm.AddFunction(g.mod, "main", cMainT)
	entry := g.ctx.AddBasicBlock(cMainF, "entry")
	g.b.SetInsertPointAtEnd(entry)

	g.b.CreateCall(g.rt.initT, g.rt.init, nil, "")
	g.b.CreateCall(g.mainT, g.mainF, nil, "")

	g.b.CreateRet(llvm.ConstInt(g.t.i32, 0, false))

	fmt.Print(g.mod.String())

	if err := llvm.VerifyModule(g.mod, llvm.PrintMessageAction); err != nil {
		panic(err.Error())
	}

	file, err := os.Create("output.bc")
	if err != nil {
		log.Fatalf("%s", err.Error())
	}
	defer file.Close()

	if err := llvm.WriteBitcodeToFile(g.mod, file); err != nil {
		panic(err.Error())
	}
}

func (gen *Generator) genStmt(s hir.Value) {
	switch t := s.(type) {
	case *hir.Variable:
		v := gen.genExpr(t.Value)
		gen.vars[t] = v
	case *hir.ReturnStmt:
		if t.Value == nil {
			gen.b.CreateRetVoid()
		} else {
			gen.b.CreateRet(gen.genExpr(t.Value))
		}
	default:
		gen.genExpr(t)
	}
}

func (gen *Generator) genFunctionCall(v *hir.FunctionCall) llvm.Value {
	var args []llvm.Value
	for _, arg := range v.Args {
		args = append(args, gen.genExpr(arg))
	}
	if v.Callee.Builtin {
		switch v.Callee.Name {
		case "print":
			return gen.b.CreateCall(gen.rt.printT, gen.rt.print, args, "")
		}
	}
	return gen.b.CreateCall(
		gen.ftypes[v.Callee],
		gen.fns[v.Callee],
		args,
		"call",
	)
}

func (gen *Generator) genExpr(v hir.Value) llvm.Value {
	switch t := v.(type) {
	case *hir.StrLiteral:
		return gen.b.CreateGlobalString(t.Value, ".str")
	case *hir.IntLiteral:
		return llvm.ConstInt(gen.t.int, uint64(t.Value), true)
	case *hir.BoolLiteral:
		v := 1
		if !t.Value {
			v = 0
		}
		return llvm.ConstInt(gen.t.bool, uint64(v), false)
	case *hir.FloatLiteral:
		return llvm.ConstFloat(gen.t.float, t.Value)
	case *hir.Variable:
		return gen.vars[t]
	case *hir.FunctionCall:
		return gen.genFunctionCall(t)
	case *hir.ExpressionStmt:
		return gen.genExpr(t.Value)
	}
	log.Fatalf("unhandled value: %T", v)
	return llvm.Value{}
}

func (gen *Generator) llvmType(t hir.Type) (llvm.Type, error) {
	switch t {
	case hir.IntT:
		return gen.t.int, nil
	case hir.BoolT:
		return gen.t.bool, nil
	case hir.FloatT:
		return gen.t.float, nil
	case hir.StrT:
		return gen.t.str, nil
	case hir.VoidT:
		return gen.t.void, nil
	}
	return llvm.Type{}, fmt.Errorf("unsupported type: %T", t)
}

func (g *Generator) addFunctionValue(name string, v *hir.Function) {
	if v.Builtin {
		return
	}
	var params []llvm.Type
	for _, p := range v.Params {
		pt, err := g.llvmType(p.Type)
		if err != nil {
			log.Fatal(err)
		}
		params = append(params, pt)
	}
	ret, err := g.llvmType(v.ReturnType)
	if err != nil {
		log.Fatal(err)
	}
	ft := llvm.FunctionType(ret, params, false)
	llname := name
	if llname == "main" {
		llname = "stellar.main"
	}
	fn := llvm.AddFunction(g.mod, llname, ft)
	fn.SetLinkage(llvm.InternalLinkage)
	for i, p := range v.Params {
		fn.Param(i).SetName(p.Name)
	}
	g.fns[v] = fn
	g.ftypes[v] = ft
	if llname == "stellar.main" {
		g.mainT = ft
		g.mainF = fn
	}
}

func (g *Generator) addValues(a *hir.Analyzer) {
	for name, value := range a.Package.Values {
		switch t := value.(type) {
		case *hir.Function:
			g.addFunctionValue(name, t)
		}
	}

}

func (gen *Generator) genFunction(f *hir.Function, fn llvm.Value) {
	entry := gen.ctx.AddBasicBlock(fn, "entry")
	gen.b.SetInsertPointAtEnd(entry)

	for idx, _ := range f.Params {
		gen.vars[f.Params[idx].Variable] = fn.Param(idx)
	}

	for _, stmt := range f.Body {
		gen.genStmt(stmt)
	}

	if f.ReturnType == hir.VoidT {
		gen.b.CreateRetVoid()
	}
}

func (g *Generator) genFunctions(a *hir.Analyzer) {
	for f, fn := range g.fns {
		g.genFunction(f, fn)
	}
}
