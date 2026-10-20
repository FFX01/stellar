package ast

import (
	"fmt"
	"io"
	"reflect"

	"stellar/internal/lexer"
)

var tokenType = reflect.TypeFor[lexer.Token]

func Dump(w io.Writer, node Node) {
	d := &dumper{w: w}
	d.value(reflect.ValueOf(node), "", "", true, true)
}

type dumper struct{ w io.Writer }

func (d *dumper) value(v reflect.Value, label, prefix string, last, root bool) {
	// Line prefix for this node, and the prefix its children inherit.
	head, childPrefix := prefix, prefix
	if !root {
		if last {
			head, childPrefix = prefix+"└─ ", prefix+"   "
		} else {
			head, childPrefix = prefix+"├─ ", prefix+"│  "
		}
	}
	if label != "" {
		head += label + ": "
	}

	// Unwrap interfaces and pointers. IsNil catches typed nils too.
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			fmt.Fprintln(d.w, head+"nil")
			return
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Invalid: // Dump(w, nil)
		fmt.Fprintln(d.w, head+"nil")

	case reflect.Struct:
		// A token in any field other than Token (e.g. VarStmt.Name) is a leaf.
		if v.Type() == tokenType() {
			tok := v.Interface().(lexer.Token)
			fmt.Fprintf(d.w, "%s%q %s\n", head, tok.Literal, tok.Position)
			return
		}

		type child struct {
			label string
			v     reflect.Value
		}
		header := v.Type().Name()
		var children []child
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if !f.IsExported() {
				continue
			}
			// A node's own Token goes on its header line, not as a child.
			if f.Name == "Token" && f.Type == tokenType() {
				tok := v.Field(i).Interface().(lexer.Token)
				header += fmt.Sprintf(" %q %s", tok.Literal, tok.Position)
				continue
			}
			children = append(children, child{f.Name, v.Field(i)})
		}

		fmt.Fprintln(d.w, head+header)
		for i, c := range children {
			d.value(c.v, c.label, childPrefix, i == len(children)-1, false)
		}

	case reflect.Slice:
		if v.Len() == 0 {
			fmt.Fprintln(d.w, head+"[]")
			return
		}
		fmt.Fprintf(d.w, "%s[%d]\n", head, v.Len())
		for i := 0; i < v.Len(); i++ {
			d.value(v.Index(i), "", childPrefix, i == v.Len()-1, false)
		}

	case reflect.String:
		fmt.Fprintf(d.w, "%s%q\n", head, v.String())

	default: // bool, int, float, ...
		fmt.Fprintf(d.w, "%s%v\n", head, v.Interface())
	}
}
