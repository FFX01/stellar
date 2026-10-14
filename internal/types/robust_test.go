package types_test

import (
	"os"
	"path/filepath"
	"testing"

	"stellar/internal/parser"
	"stellar/internal/types"
)

// FuzzCheck checks that the type checker never panics on any input the
// parser accepts.
func FuzzCheck(f *testing.F) {
	for _, dir := range []string{"testdata", "../parser/testdata/ok"} {
		paths, _ := filepath.Glob(filepath.Join(dir, "*.st"))
		for _, path := range paths {
			src, err := os.ReadFile(path)
			if err != nil {
				f.Fatal(err)
			}
			f.Add(src)
		}
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		file, errs := parser.ParseFile(src)
		if len(errs) > 0 {
			return
		}
		types.Check(file)
	})
}
