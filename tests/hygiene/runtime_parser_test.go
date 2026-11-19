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

// TestRuntimeDoesNotDependOnTheParser keeps the runtime from parsing the notation text a run
// reads (witness files, tool units): the runtime.ExpressionParser its frontend installs does.
func TestRuntimeDoesNotDependOnTheParser(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./internal/exec/runtime")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if dep == "github.com/Open-MBEE/OpenSysML/internal/syntax/parser" {
			t.Errorf("internal/exec/runtime depends on %s", dep)
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
						installs = installs || readsWithTheNotationsParser(call.Args[0])
					}
					return true
				})
				if builds && !installs {
					t.Errorf("%s: %s builds a runtime.Model without SetExpressionParser(lower.ExpressionReader(parser.ParseOneExpression, …))", path, fn.Name.Name)
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

// readsWithTheNotationsParser reports whether arg spells lower.ExpressionReader over
// parser.ParseOneExpression: the notation's parser, lowering what it reads.
func readsWithTheNotationsParser(arg ast.Expr) bool {
	reader, ok := arg.(*ast.CallExpr)
	if !ok || len(reader.Args) != 2 {
		return false
	}
	fun, ok := reader.Fun.(*ast.SelectorExpr)
	parse, parses := reader.Args[0].(*ast.SelectorExpr)
	return ok && parses && isPackageSelector(fun, "lower", "ExpressionReader") &&
		isPackageSelector(parse, "parser", "ParseOneExpression")
}

// isPackageSelector reports whether sel spells pkg.name.
func isPackageSelector(sel *ast.SelectorExpr, pkg, name string) bool {
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == pkg && sel.Sel.Name == name
}
