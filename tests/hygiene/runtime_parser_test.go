package hygiene

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRuntimeDoesNotDependOnTheParser keeps the runtime a consumer of parsed trees:
// the notation text a run reads (witness files, tool units) is parsed by the
// runtime.ExpressionParser its frontend installs, never by the runtime itself.
func TestRuntimeDoesNotDependOnTheParser(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./internal/core/runtime")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if dep == "github.com/Open-MBEE/OpenSysML/internal/core/parser" {
			t.Errorf("internal/core/runtime depends on %s", dep)
		}
	}
}

// TestRuntimeModelsInstallTheExpressionParser checks that every function of shipped
// code that builds a runtime.Model installs the notation's parser on it, so no product
// path reaches a witness file or a tool's unit with runtime.ErrNoExpressionParser.
func TestRuntimeModelsInstallTheExpressionParser(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}\t{{join .GoFiles \" \"}}", "./...")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	sites := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		dir, files, _ := strings.Cut(line, "\t")
		for _, name := range strings.Fields(files) {
			path := filepath.Join(dir, name)
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				builds, installs := false, false
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					switch {
					case !ok:
					case isPackageSelector(sel, "runtime", "NewModel"):
						builds = true
					case sel.Sel.Name == "SetExpressionParser" && len(call.Args) == 1:
						if arg, ok := call.Args[0].(*ast.SelectorExpr); ok && isPackageSelector(arg, "parser", "ParseOneExpression") {
							installs = true
						}
					}
					return true
				})
				if builds && !installs {
					t.Errorf("%s: %s builds a runtime.Model without SetExpressionParser(parser.ParseOneExpression)", path, fn.Name.Name)
				}
				if builds {
					sites++
				}
			}
		}
	}
	if sites == 0 {
		t.Fatal("no shipped code builds a runtime.Model; the check is vacuous")
	}
}

// isPackageSelector reports whether sel spells pkg.name.
func isPackageSelector(sel *ast.SelectorExpr, pkg, name string) bool {
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == pkg && sel.Sel.Name == name
}
