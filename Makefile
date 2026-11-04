export CGO_CFLAGS := -I/opt/homebrew/opt/llvm/include -I/opt/homebrew/include
export CGO_LDFLAGS := -L/opt/homebrew/opt/llvm/lib -L/opt/homebrew/lib -lgc

build:
	go build -tags llvm23 ./cmd/compile

comp:
	clang output.bc internal/compile/runtime.c -I/opt/homebrew/include -L/opt/homebrew/lib -lgc -o stellar
