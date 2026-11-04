package compile

import (
	"fmt"
	"log"
	"os"
	"stellar/internal/hir"

	"tinygo.org/x/go-llvm"
)

type sgen struct {
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

	gen := sgen{}
	gen.fns = map[*hir.Function]llvm.Value{}
	gen.ftypes = map[*hir.Function]llvm.Type{}
	gen.vars = map[*hir.Variable]llvm.Value{}

	gen.ctx = llvm.NewContext()
	gen.mod = gen.ctx.NewModule("stellar_module")
	gen.mod.SetTarget(triple)
	gen.mod.SetDataLayout(td.String())

	gen.t.int = gen.ctx.Int64Type()
	gen.t.bool = gen.ctx.Int1Type()
	gen.t.float = gen.ctx.DoubleType()
	gen.t.str = llvm.PointerType(gen.ctx.Int8Type(), 0)
	gen.t.i32 = gen.ctx.Int32Type()
	gen.t.void = gen.ctx.VoidType()

	gen.rt.initT = llvm.FunctionType(gen.t.void, nil, false)
	gen.rt.init = llvm.AddFunction(gen.mod, "stellar_init", gen.rt.initT)
	gen.rt.printT = llvm.FunctionType(gen.t.void, []llvm.Type{gen.t.str}, false)
	gen.rt.print = llvm.AddFunction(gen.mod, "stellar_print", gen.rt.printT)
	// allocType := llvm.FunctionType(ptrType, []llvm.Type{td.IntPtrType()}, false)
	// allocFn := llvm.AddFunction(module, "stellar_alloc", allocType)

	gen.b = gen.ctx.NewBuilder()
	defer gen.b.Dispose()

	// build user code
	for name, value := range a.Package.Values {
		switch t := value.(type) {
		case *hir.Function:
			if t.Builtin {
				continue
			}
			var params []llvm.Type
			for _, p := range t.Params {
				pt, err := gen.llvmType(p.Type)
				if err != nil {
					log.Fatal(err)
				}
				params = append(params, pt)
			}
			ret, err := gen.llvmType(t.ReturnType)
			if err != nil {
				log.Fatal(err)
			}
			ft := llvm.FunctionType(ret, params, false)
			llname := name
			if llname == "main" {
				llname = "stellar.main"
			}
			fn := llvm.AddFunction(gen.mod, llname, ft)
			fn.SetLinkage(llvm.InternalLinkage)
			for i, p := range t.Params {
				fn.Param(i).SetName(p.Name)
			}
			gen.fns[t] = fn
			gen.ftypes[t] = ft
			if llname == "stellar.main" {
				gen.mainT = ft
				gen.mainF = fn
			}
		}
	}
	for f, fn := range gen.fns {
		gen.genFunction(f, fn)
	}

	// wrap stellar init and stellar main in C main
	cMainT := llvm.FunctionType(gen.t.i32, nil, false)
	cMainF := llvm.AddFunction(gen.mod, "main", cMainT)
	entry := gen.ctx.AddBasicBlock(cMainF, "entry")
	gen.b.SetInsertPointAtEnd(entry)

	gen.b.CreateCall(gen.rt.initT, gen.rt.init, nil, "")
	gen.b.CreateCall(gen.mainT, gen.mainF, nil, "")

	gen.b.CreateRet(llvm.ConstInt(gen.t.i32, 0, false))

	fmt.Print(gen.mod.String())

	if err := llvm.VerifyModule(gen.mod, llvm.PrintMessageAction); err != nil {
		panic(err.Error())
	}

	file, err := os.Create("output.bc")
	if err != nil {
		log.Fatalf("%s", err.Error())
	}
	defer file.Close()

	if err := llvm.WriteBitcodeToFile(gen.mod, file); err != nil {
		panic(err.Error())
	}
}

func (gen *sgen) genStmt(s hir.Value) {
	switch t := s.(type) {
	case *hir.Variable:
		v := gen.genExpr(t.Value)
		gen.vars[t] = v
	default:
		gen.genExpr(t)
	}
}

func (gen *sgen) genFunctionCall(v *hir.FunctionCall) llvm.Value {
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
		"",
	)
}

func (gen *sgen) genExpr(v hir.Value) llvm.Value {
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

func (gen *sgen) llvmType(t hir.Type) (llvm.Type, error) {
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

func (gen *sgen) genFunction(f *hir.Function, fn llvm.Value) {
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
