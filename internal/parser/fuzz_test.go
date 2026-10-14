package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"stellar/internal/parser"
	"stellar/internal/printer"
)

// FuzzParseFile checks that the parser never panics or hangs, and that any
// input it accepts prints to source that parses back to the same output.
func FuzzParseFile(f *testing.F) {
	paths, _ := filepath.Glob(filepath.Join("testdata", "*", "*.st"))
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(src)
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		file, errs := parser.ParseFile(src)
		if len(errs) > 0 {
			return
		}
		out := printer.File(file)
		again, errs := parser.ParseFile([]byte(out))
		if len(errs) > 0 {
			t.Fatalf("printed output does not parse: %v\ninput:\n%s\noutput:\n%s", errs, src, out)
		}
		if second := printer.File(again); second != out {
			t.Fatalf("printing is not idempotent\nfirst:\n%s\nsecond:\n%s", out, second)
		}
	})
}
