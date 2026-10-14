// Package diag defines the diagnostics reported by every compiler phase.
package diag

import (
	"fmt"
	"sort"
	"strings"

	"stellar/internal/token"
)

// Error is a single diagnostic tied to a source position.
type Error struct {
	Pos token.Pos
	Msg string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

// List is an ordered collection of diagnostics. It implements error so a
// phase can return all of its diagnostics at once.
type List []Error

// Add appends a diagnostic.
func (l *List) Add(pos token.Pos, format string, args ...any) {
	*l = append(*l, Error{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// Sort orders diagnostics by position, keeping insertion order for ties.
func (l List) Sort() {
	sort.SliceStable(l, func(i, j int) bool {
		a, b := l[i].Pos, l[j].Pos
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
}

// Err returns l as an error, or nil if it is empty.
func (l List) Err() error {
	if len(l) == 0 {
		return nil
	}
	return l
}

func (l List) Error() string {
	var b strings.Builder
	for i, e := range l {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(e.Error())
	}
	return b.String()
}

// WithFile renders every diagnostic as "file:line:col: msg", one per line.
func (l List) WithFile(name string) string {
	var b strings.Builder
	for _, e := range l {
		fmt.Fprintf(&b, "%s:%s\n", name, e.Error())
	}
	return b.String()
}
