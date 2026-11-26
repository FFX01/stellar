package hir2

import (
	"regexp"

	"github.com/sanity-io/litter"
)

func (c *Context) Dump() {
	litter.Config.FieldExclusions = regexp.MustCompile(`^(Node)$`)
	litter.Dump(c.Strings.Strings)
	litter.Dump(c.Scopes)
	litter.Dump(c.Types)
	litter.Dump(c.Symbols)
	litter.Dump(c.TypeExprs)
	litter.Dump(c.Extra)
	litter.Dump(c.FuncMetas)
	litter.Dump(c.InterfaceMetas)
	litter.Dump(c.EnumMetas)
	litter.Dump(c.ImplMetas)
	litter.Dump(c.Exprs)
	// if len(c.Diags) > 0 {
	// 	sorted := slices.Clone(c.Diags)
	// 	slices.SortFunc(sorted, func(a, b *Diag) int {
	// 		aLine, bLine := a.Node.Pos().Line, b.Node.Pos().Line
	// 		if aLine < bLine {
	// 			return -1
	// 		} else if bLine < aLine {
	// 			return 1
	// 		} else {
	// 			return 0
	// 		}
	// 	})
	// 	fmt.Println("--- Errors ---")
	// 	for _, e := range sorted {
	// 		fmt.Println(e.String())
	// 	}
	// }
}
