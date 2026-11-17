package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"stellar/internal/hir"
	"stellar/internal/loader"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("No file provided")
	}

	path := os.Args[1]
	root := filepath.Dir(path)
	ld := loader.New(root)
	if _, err := ld.Load(path, nil); err != nil {
		log.Fatal(err)
	}
	for _, e := range ld.Errors {
		fmt.Println(e)
	}

	universe := hir.NewUniverse()
	done := map[*loader.Module]*hir.Module{}
	for _, lm := range ld.Order {
		deps := map[string]*hir.Module{}
		for key, dep := range lm.Imports {
			deps[key] = done[dep]
		}
		a := hir.NewAnalyzer(universe, strings.Join(lm.Path, "::"))
		a.Run(lm.Program, deps)
		done[lm] = a.Mod
		fmt.Println("==", lm.File)
		a.Dump()
	}
}
