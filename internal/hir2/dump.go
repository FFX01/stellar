package hir2

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/sanity-io/litter"
)

func (c *Context) Dump() {
	litter.Config.FieldExclusions = regexp.MustCompile(`^(Node)$`)
	litter.Dump(c.Symbols)
	litter.Dump(c.Types)
	litter.Dump(c.Universe)
	if len(c.Diags) > 0 {
		sorted := slices.Clone(c.Diags)
		slices.SortFunc(sorted, func(a, b *Diag) int {
			aLine, bLine := a.Node.Pos().Line, b.Node.Pos().Line
			if aLine < bLine {
				return -1
			} else if bLine < aLine {
				return 1
			} else {
				return 0
			}
		})
		fmt.Println("--- Errors ---")
		for _, e := range sorted {
			fmt.Println(e.String())
		}
	}
}
