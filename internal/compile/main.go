package compile

import (
	"fmt"
	"log"
	"os"

	"tinygo.org/x/go-llvm"
)

func Compile() {
	llvm.InitializeAllTargets()
	llvm.InitializeAllTargetMCs()
	llvm.InitializeAllTargetInfos()
	llvm.InitializeAllAsmPrinters()

	context := llvm.NewContext()
	module := context.NewModule("stellar_module")
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

	module.SetTarget(triple)
	module.SetDataLayout(td.String())

	ptrType := llvm.PointerType(context.Int8Type(), 0)
	i32 := context.Int32Type()
	// i64 := context.Int64Type()
	void := context.VoidType()

	initType := llvm.FunctionType(void, nil, false)
	// allocType := llvm.FunctionType(ptrType, []llvm.Type{td.IntPtrType()}, false)
	putsType := llvm.FunctionType(i32, []llvm.Type{ptrType}, false)
	// printfType := llvm.FunctionType(i32, []llvm.Type{ptrType}, true)

	initFn := llvm.AddFunction(module, "stellar_init", initType)
	// allocFn := llvm.AddFunction(module, "stellar_alloc", allocType)
	putsFn := llvm.AddFunction(module, "puts", putsType)
	// printFn := llvm.AddFunction(module, "printf", printfType)

	builder := context.NewBuilder()
	defer builder.Dispose()

	mainT := llvm.FunctionType(i32, nil, false)
	mainF := llvm.AddFunction(module, "main", mainT)

	entry := context.AddBasicBlock(mainF, "entry")
	builder.SetInsertPointAtEnd(entry)

	builder.CreateCall(initType, initFn, nil, "")

	msg := builder.CreateGlobalString("hello from stellar", "msg")
	builder.CreateCall(putsType, putsFn, []llvm.Value{msg}, "")
	builder.CreateRet(llvm.ConstInt(i32, 0, false))

	fmt.Print(module.String())

	if err := llvm.VerifyModule(module, llvm.PrintMessageAction); err != nil {
		panic(err.Error())
	}

	file, err := os.Create("output.bc")
	if err != nil {
		log.Fatalf("%s", err.Error())
	}
	defer file.Close()

	if err := llvm.WriteBitcodeToFile(module, file); err != nil {
		panic(err.Error())
	}
}
