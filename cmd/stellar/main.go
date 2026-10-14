// Command stellar is the Stellar toolchain.
package main

import (
	"fmt"
	"io"
	"os"

	"stellar/internal/ast"
	"stellar/internal/interp"
	"stellar/internal/parser"
	"stellar/internal/printer"
	"stellar/internal/types"
)

const usage = `usage: stellar <command> <file.st>

commands:
  run     type-check and run a file
  check   type-check a file
  parse   parse a file and print it in canonical form (comments are dropped)
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, path := args[0], args[1]
	switch cmd {
	case "parse":
		return parseCmd(path, stdout, stderr)
	case "check":
		return checkCmd(path, stderr)
	case "run":
		return runCmd(path, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "stellar: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
}

func parseCmd(path string, stdout, stderr io.Writer) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "stellar: %v\n", err)
		return 1
	}
	file, errs := parser.ParseFile(src)
	if len(errs) > 0 {
		fmt.Fprint(stderr, errs.WithFile(path))
		return 1
	}
	fmt.Fprint(stdout, printer.File(file))
	return 0
}

func checkCmd(path string, stderr io.Writer) int {
	if _, _, ok := load(path, stderr); !ok {
		return 1
	}
	return 0
}

// runCmd exits with 1 for compile errors or main returning Err, and with 2
// for a panic.
func runCmd(path string, stdout, stderr io.Writer) int {
	file, info, ok := load(path, stderr)
	if !ok {
		return 1
	}
	err := interp.Run(file, info, stdout)
	switch err.(type) {
	case nil:
		return 0
	case *interp.RuntimeError:
		fmt.Fprintln(stderr, err)
		return 2
	default:
		fmt.Fprintln(stderr, err)
		return 1
	}
}

// load parses and type-checks a file, reporting any diagnostics.
func load(path string, stderr io.Writer) (*ast.File, *types.Info, bool) {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "stellar: %v\n", err)
		return nil, nil, false
	}
	file, errs := parser.ParseFile(src)
	var info *types.Info
	if len(errs) == 0 {
		info, errs = types.Check(file)
	}
	if len(errs) > 0 {
		fmt.Fprint(stderr, errs.WithFile(path))
		return nil, nil, false
	}
	return file, info, true
}
