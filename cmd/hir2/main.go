package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"stellar/internal/hir2"
	"stellar/internal/loader"
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

	ctx := hir2.NewContext(ld.Order)
	ctx.Run()
	ctx.Dump()
}
