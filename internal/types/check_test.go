package types_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"stellar/internal/parser"
	"stellar/internal/types"
)

// errorComment matches an expectation such as // ERROR "cannot use"
// The quoted text must be a substring of a diagnostic reported on that
// line. A line may carry several expectations.
var errorComment = regexp.MustCompile(`ERROR ("(?:[^"\\]|\\.)*")`)

// TestCheck type-checks every testdata/*.st file and compares the
// diagnostics with the file's // ERROR comments. A file without such
// comments must check cleanly.
func TestCheck(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "*.st"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no test files in testdata")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			file, perrs := parser.ParseFile(src)
			if len(perrs) > 0 {
				t.Fatalf("parse errors:\n%s", perrs.WithFile(path))
			}
			_, errs := types.Check(file)

			want := expectations(t, string(src))
			for _, e := range errs {
				if !consume(want, e.Pos.Line, e.Msg) {
					t.Errorf("%s:%s: unexpected error: %s", path, e.Pos, e.Msg)
				}
			}
			for line, subs := range want {
				for _, s := range subs {
					t.Errorf("%s:%d: missing error containing %q", path, line, s)
				}
			}
		})
	}
}

func expectations(t *testing.T, src string) map[int][]string {
	want := map[int][]string{}
	for i, line := range strings.Split(src, "\n") {
		for _, m := range errorComment.FindAllStringSubmatch(line, -1) {
			s, err := strconv.Unquote(m[1])
			if err != nil {
				t.Fatalf("line %d: bad ERROR comment: %v", i+1, err)
			}
			want[i+1] = append(want[i+1], s)
		}
	}
	return want
}

// consume removes the first expectation on line that msg satisfies.
func consume(want map[int][]string, line int, msg string) bool {
	for i, s := range want[line] {
		if strings.Contains(msg, s) {
			want[line] = append(want[line][:i], want[line][i+1:]...)
			if len(want[line]) == 0 {
				delete(want, line)
			}
			return true
		}
	}
	return false
}

// TestExamples requires every program in examples/ to type-check cleanly.
func TestExamples(t *testing.T) {
	paths, _ := filepath.Glob(filepath.Join("..", "..", "examples", "*.st"))
	if len(paths) == 0 {
		t.Fatal("no examples found")
	}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, errs := parser.ParseFile(src)
		if len(errs) == 0 {
			_, errs = types.Check(file)
		}
		if len(errs) > 0 {
			t.Errorf("%s", errs.WithFile(path))
		}
	}
}
