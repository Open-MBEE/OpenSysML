package opensysml

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// capabilityConstants reads the Capability constants a package directory
// declares: each name and the expression it is given.
func capabilityConstants(t *testing.T, dir string) map[string]ast.Expr {
	t.Helper()
	pkgs, err := parser.ParseDir(token.NewFileSet(), dir, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing %s: %v", dir, err)
	}
	consts := map[string]ast.Expr{}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.CONST {
					continue
				}
				for _, spec := range gen.Specs {
					vs := spec.(*ast.ValueSpec)
					for i, ident := range vs.Names {
						if strings.HasPrefix(ident.Name, "Capability") && i < len(vs.Values) {
							consts[ident.Name] = vs.Values[i]
						}
					}
				}
			}
		}
	}
	return consts
}

// TestCapabilityConstantsCoverTheService holds the client's re-stated
// Capability constants to the service's: the same names, none missing or extra.
func TestCapabilityConstantsCoverTheService(t *testing.T) {
	service := capabilityConstants(t, "../../internal/frontend/grpc")
	client := capabilityConstants(t, ".")

	named := map[string]string{}
	for name, expr := range service {
		lit, ok := expr.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			t.Errorf("service constant %s is not a string literal", name)
			continue
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Errorf("service constant %s: %v", name, err)
			continue
		}
		named[value] = name
		if _, ok := client[name]; !ok {
			t.Errorf("the service declares %s (%q) and the client does not re-state it", name, value)
		}
	}
	for _, capability := range sysmlgrpc.Capabilities() {
		if _, ok := named[capability]; !ok {
			t.Errorf("the service reports %q, which no Capability constant names", capability)
		}
	}
	for name, expr := range client {
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != name {
			t.Errorf("client constant %s does not re-state the service constant of its name", name)
			continue
		}
		if _, ok := service[name]; !ok {
			t.Errorf("client constant %s names no service constant", name)
		}
	}
}
