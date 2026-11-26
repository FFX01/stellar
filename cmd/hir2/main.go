package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"stellar/internal/hir2"
	"stellar/internal/loader"
	"stellar/lib"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("No file provided")
	}

	preludeFS, err := fs.Sub(lib.PreludeFS, "prelude")
	if err != nil {
		log.Fatal(err)
	}
	preludeLoader := loader.New(preludeFS)
	_, err = preludeLoader.Load(preludeFS, "main.solr", nil)
	if err != nil {
		log.Fatal(err)
	}

	path := os.Args[1]
	userFS := os.DirFS(filepath.Dir(path))
	ld := loader.New(userFS)
	if _, err := ld.Load(userFS, filepath.Base(path), nil); err != nil {
		log.Fatal(err)
	}
	for _, e := range ld.Errors {
		fmt.Println(e)
	}

	ctx := hir2.NewContext(preludeLoader.Order, ld.Order)
	ctx.Run()
	ctx.Dump()
}
