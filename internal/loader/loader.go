/* Package loader */
package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"stellar/internal/ast"
	"stellar/internal/lexer"
	"stellar/internal/parser"
	"strings"
)

type Module struct {
	Path    []string
	File    string
	Name    string
	Program *ast.Program
	Imports map[string]*Module
}

type Loader struct {
	Roots  []string
	byFile map[string]*Module
	open   map[string]bool
	Order  []*Module
	Errors []string
}

func (l *Loader) find(path []string) (string, error) {
	rel := filepath.Join(path...) + ".solr"
	for _, root := range l.Roots {
		f := filepath.Join(root, rel)
		if _, err := os.Stat(f); err == nil {
			return f, nil
		}
	}
	return "", fmt.Errorf(
		"module `%s` not found",
		strings.Join(path, "::"),
	)
}

func (l *Loader) Load(file string, path []string) (*Module, error) {
	if m, ok := l.byFile[file]; ok {
		return m, nil
	}

	if l.open[file] {
		return nil, fmt.Errorf("import cycle through `%s`", strings.Join(path, "::"))
	}

	l.open[file] = true
	defer delete(l.open, file)

	prog, err := parseFile(file)
	if err != nil {
		return nil, err
	}

	m := &Module{Path: path, File: file, Program: prog, Imports: map[string]*Module{}}
	if len(path) > 0 {
		m.Name = path[len(path)-1]
	}
	for _, s := range prog.Statements {
		imp, ok := s.(*ast.ImportStmt)
		if !ok {
			continue
		}
		ip := names(imp.Path)
		f, err := l.find(ip)
		if err == nil {
			if dep, err := l.Load(f, ip); err == nil {
				m.Imports[strings.Join(ip, "::")] = dep
				continue
			}
		}
		l.Errors = append(l.Errors, fmt.Sprintf("%s:%d:%d: %s",
			file, imp.Pos().Line, imp.Pos().Column, err))
	}

	l.byFile[file] = m
	l.Order = append(l.Order, m)
	return m, nil
}

func names(tokens []lexer.Token) []string {
	var out []string
	for _, t := range tokens {
		out = append(out, t.Literal)
	}
	return out
}

func parseFile(path string) (*ast.Program, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lexer := lexer.NewLexer(string(content))
	tokens := lexer.Run()
	lexerErrors := lexer.Errors()
	if len(lexerErrors) > 0 {
		for _, e := range lexerErrors {
			fmt.Println(e.Message)
		}
		return nil, fmt.Errorf("lexer errors in `%s`", path)
	}

	p := parser.NewParser(tokens)
	program := p.Parse()
	if p.HasErrors() {
		p.PrintErrors()
		return nil, fmt.Errorf("parser errors in `%s`", path)
	}
	return program, nil
}

func New(roots ...string) *Loader {
	return &Loader{
		Roots:  roots,
		byFile: map[string]*Module{},
		open:   map[string]bool{},
	}
}
