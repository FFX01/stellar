package parser_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stellar/internal/parser"
	"stellar/internal/printer"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestGoldenOK parses every testdata/ok/*.st file, compares the printed tree
// with the .golden file, and checks that printing is a fixed point: parsing
// the printed output and printing it again yields the same text.
func TestGoldenOK(t *testing.T) {
	for _, path := range sources(t, "ok") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			file, errs := parser.ParseFile(readFile(t, path))
			if len(errs) > 0 {
				t.Fatalf("unexpected parse errors:\n%s", errs.WithFile(path))
			}
			got := printer.File(file)
			compareGolden(t, path, got)

			again, errs := parser.ParseFile([]byte(got))
			if len(errs) > 0 {
				t.Fatalf("printed output does not parse:\n%s\n%s", errs, got)
			}
			if second := printer.File(again); second != got {
				t.Errorf("printing is not idempotent.\nfirst:\n%s\nsecond:\n%s", got, second)
			}
		})
	}
}

// TestGoldenErrors parses every testdata/errors/*.st file and compares the
// diagnostics with the .golden file.
func TestGoldenErrors(t *testing.T) {
	for _, path := range sources(t, "errors") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, errs := parser.ParseFile(readFile(t, path))
			if len(errs) == 0 {
				t.Fatal("expected parse errors, got none")
			}
			var b strings.Builder
			for _, e := range errs {
				b.WriteString(e.Error())
				b.WriteByte('\n')
			}
			compareGolden(t, path, b.String())
		})
	}
}

func sources(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", dir, "*.st"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no test files in testdata/%s", dir)
	}
	return paths
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func compareGolden(t *testing.T, srcPath, got string) {
	t.Helper()
	goldenPath := strings.TrimSuffix(srcPath, ".st") + ".golden"
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("%v (run `go test ./internal/parser -update` to create it)", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, want)
	}
}
