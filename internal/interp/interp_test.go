package interp_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stellar/internal/interp"
	"stellar/internal/parser"
	"stellar/internal/types"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestRun runs every testdata/*.st program and every example and compares its output, plus
// any panic or error from main, with the matching .out file.
func TestRun(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "*.st"))
	examples, _ := filepath.Glob(filepath.Join("..", "..", "examples", "*.st"))
	paths = append(paths, examples...)
	if err != nil || len(paths) == 0 {
		t.Fatal("no test programs in testdata")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			got := run(t, path)
			golden := strings.TrimSuffix(path, ".st") + ".out"
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run with -update to create it)", err)
			}
			if got != string(want) {
				t.Errorf("output differs from %s\n--- got ---\n%s--- want ---\n%s", golden, got, want)
			}
		})
	}
}

func run(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, errs := parser.ParseFile(src)
	var info *types.Info
	if len(errs) == 0 {
		info, errs = types.Check(file)
	}
	if len(errs) > 0 {
		t.Fatalf("compile errors:\n%s", errs.WithFile(path))
	}
	var out bytes.Buffer
	if err := interp.Run(file, info, &out); err != nil {
		out.WriteString(err.Error() + "\n")
	}
	return out.String()
}
