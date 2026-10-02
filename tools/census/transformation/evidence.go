package transformation

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
)

// implementationRef is a `<file>.go:<func>` or `<file>.go:<Type>.<method>`
// citation; group 3 captures any continuation past the symbol other than the
// punctuation that ends a cell, list item or sentence, which is malformed.
var implementationRef = regexp.MustCompile(`^([\w/.-]+\.go):([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)$`)

// testRef is a `<file>_test.go:<TestFunc>` citation into a Go test file.
var testRef = regexp.MustCompile(`^([\w/.-]+_test\.go):(Test[A-Za-z_]\w*)$`)

// declarations caches the function and method names each cited Go file declares.
type declarations struct {
	root  string
	files map[string]map[string]bool
}

func newDeclarations(root string) *declarations {
	return &declarations{root: root, files: make(map[string]map[string]bool)}
}

func (d *declarations) parse(file string) (*ast.File, error) {
	path := filepath.Join(d.root, filepath.FromSlash(file))
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("%s does not exist", file)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return parsed, nil
}

// declared reports whether file declares the named function or Type.method;
// a bare name matches a function or a method with that name.
func (d *declarations) declared(file, name string) (bool, error) {
	names, ok := d.files[file]
	if !ok {
		parsed, err := d.parse(file)
		if err != nil {
			return false, err
		}
		names = make(map[string]bool)
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			names[fn.Name.Name] = true
			if recv := receiverType(fn); recv != "" {
				names[recv+"."+fn.Name.Name] = true
			}
		}
		d.files[file] = names
	}
	return names[name], nil
}

// testDeclared reports whether file declares `func TestX(t *testing.T)`.
func (d *declarations) testDeclared(file, name string) (bool, error) {
	parsed, err := d.parse(file)
	if err != nil {
		return false, err
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name || fn.Recv != nil {
			continue
		}
		if isTestFunc(fn) {
			return true, nil
		}
		return false, fmt.Errorf("%s: %s exists but is not func %s(t *testing.T)", file, name, name)
	}
	return false, nil
}

// isTestFunc reports whether fn has exactly the `t *testing.T` parameter list.
func isTestFunc(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	param := fn.Type.Params.List[0]
	star, ok := param.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "T" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "testing"
}

func receiverType(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.IndexExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.IndexListExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// checkCites verifies the rows' and beyond entries' implementation and test
// citations resolve: every implementation cite is a repo-relative .go file
// declaring the named function or Type.method, and every test cite is a
// _test.go file declaring `func TestX(t *testing.T)`.
func checkCites(decls *declarations, base *Baseline) []string {
	var problems []string
	check := func(subject string, implementation, tests []string) {
		for _, cite := range implementation {
			match := implementationRef.FindStringSubmatch(cite)
			if match == nil {
				problems = append(problems, fmt.Sprintf("%s implementation %q is not a <file>.go:<function> or <file>.go:<Type>.<method> location", subject, cite))
				continue
			}
			found, err := decls.declared(match[1], match[2])
			switch {
			case err != nil:
				problems = append(problems, fmt.Sprintf("%s implementation: %v", subject, err))
			case !found:
				problems = append(problems, fmt.Sprintf("%s implementation %s declares no %s", subject, cite, match[2]))
			}
		}
		for _, cite := range tests {
			match := testRef.FindStringSubmatch(cite)
			if match == nil {
				problems = append(problems, fmt.Sprintf("%s test %q is not a <file>_test.go:<TestFunc> location", subject, cite))
				continue
			}
			found, err := decls.testDeclared(match[1], match[2])
			switch {
			case err != nil:
				problems = append(problems, fmt.Sprintf("%s test: %v", subject, err))
			case !found:
				problems = append(problems, fmt.Sprintf("%s test %s declares no func %s(t *testing.T)", subject, cite, match[2]))
			}
		}
	}
	for _, m := range base.Mappings {
		check(m.Name, m.Implementation, m.Tests)
	}
	for _, e := range base.Beyond {
		check(e.Behaviour, e.Implementation, e.Tests)
	}
	return problems
}
