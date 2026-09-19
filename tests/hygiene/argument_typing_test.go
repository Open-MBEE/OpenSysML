package hygiene

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	runtimePkg = "github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	passesPkg  = "github.com/Open-MBEE/OpenSysML/internal/check/passes"
)

// Frontends whose runtime models the check must keep seeing; a restructuring
// that hides one of these construction sites from the walk fails here.
var runtimeModelFrontends = []string{
	"internal/repl/session.go",
	"internal/grpc/cache.go",
	"internal/core/model/runtime.go",
}

// Runtime constructors that call NewModel on a semantic model handed to them,
// and the production callers allowed to hand them one: the model each caller
// passes is pinned typed by a test over its product path, since the walk cannot
// see through the field or parameter it arrives in.
var runtimeModelForwarders = map[string][]string{
	"NewDeclaredReader": {"internal/doc/queryexec/derived.go"},
}

// TestRuntimeModelsCarryArgumentTyping pins that every production site that
// hands a semantic model to runtime.NewModel built it with passes.NewTypedModel,
// so the runtime selects overloads with the checker's argument typing and its
// missing-typer error stays unreachable from the shipped frontends. Inside the
// runtime package, every unqualified NewModel call must sit in a constructor
// listed in runtimeModelForwarders, whose callers are in turn confined to the
// files listed there.
func TestRuntimeModelsCarryArgumentTyping(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", "{{.Dir}} {{join .GoFiles \" \"}}", "./...")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	sites := map[string]bool{}
	forwarders := map[string]bool{}
	forwarderCallers := map[string]map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		inRuntime := filepath.ToSlash(fields[0]) == filepath.ToSlash(filepath.Join(root, "internal/exec/runtime"))
		for _, name := range fields[1:] {
			path := filepath.Join(fields[0], name)
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if inRuntime {
				for fn, call := range unqualifiedNewModelCalls(file) {
					if _, ok := runtimeModelForwarders[fn]; !ok {
						t.Errorf("%s: NewModel called inside %s, a runtime constructor not listed in runtimeModelForwarders",
							fset.Position(call.Pos()), fn)
					}
					forwarders[fn] = true
				}
				continue
			}
			for _, site := range runtimeModelSites(file) {
				sites[rel] = true
				if !site.typed {
					t.Errorf("%s: runtime.NewModel receives a semantic model not built by passes.NewTypedModel (%s)",
						fset.Position(site.call.Pos()), site.reason)
				}
			}
			for fn, call := range runtimeForwarderCalls(file) {
				if forwarderCallers[fn] == nil {
					forwarderCallers[fn] = map[string]bool{}
				}
				forwarderCallers[fn][rel] = true
				if !slices.Contains(runtimeModelForwarders[fn], rel) {
					t.Errorf("%s: runtime.%s receives a semantic model from a caller not listed in runtimeModelForwarders",
						fset.Position(call.Pos()), fn)
				}
			}
		}
	}
	for _, want := range runtimeModelFrontends {
		if !sites[want] {
			t.Errorf("%s: expected a runtime.NewModel construction site", want)
		}
	}
	for fn, callers := range runtimeModelForwarders {
		if !forwarders[fn] {
			t.Errorf("runtime.%s no longer calls NewModel; drop it from runtimeModelForwarders", fn)
		}
		for _, want := range callers {
			if !forwarderCallers[fn][want] {
				t.Errorf("%s: expected a runtime.%s call", want, fn)
			}
		}
	}
	var seen []string
	for s := range sites {
		seen = append(seen, s)
	}
	sort.Strings(seen)
	t.Logf("runtime.NewModel sites checked: %s", strings.Join(seen, ", "))
}

// unqualifiedNewModelCalls maps each top-level function of a runtime-package
// file that calls NewModel to one such call.
func unqualifiedNewModelCalls(file *ast.File) map[string]*ast.CallExpr {
	calls := map[string]*ast.CallExpr{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "NewModel" {
				calls[fn.Name.Name] = call
			}
			return true
		})
	}
	return calls
}

// runtimeForwarderCalls maps each forwarder in runtimeModelForwarders that file
// calls through the runtime package to one such call.
func runtimeForwarderCalls(file *ast.File) map[string]*ast.CallExpr {
	runtimeName := ""
	for _, imp := range file.Imports {
		if path, _ := strconv.Unquote(imp.Path.Value); path == runtimePkg {
			runtimeName = importName(imp, "runtime")
		}
	}
	if runtimeName == "" {
		return nil
	}
	calls := map[string]*ast.CallExpr{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for fn := range runtimeModelForwarders {
			if isSelectorCall(call, runtimeName, fn) {
				calls[fn] = call
			}
		}
		return true
	})
	return calls
}

type runtimeModelSite struct {
	call   *ast.CallExpr
	typed  bool
	reason string
}

// runtimeModelSites lists the runtime.NewModel calls in file and whether each
// first argument is a passes.NewTypedModel call, directly or through a local
// assigned from one and never reassigned within the enclosing declaration.
func runtimeModelSites(file *ast.File) []runtimeModelSite {
	runtimeName, passesName := "", ""
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		switch path {
		case runtimePkg:
			runtimeName = importName(imp, "runtime")
		case passesPkg:
			passesName = importName(imp, "passes")
		}
	}
	if runtimeName == "" {
		return nil
	}
	var sites []runtimeModelSite
	for _, decl := range file.Decls {
		ast.Inspect(decl, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isSelectorCall(call, runtimeName, "NewModel") || len(call.Args) == 0 {
				return true
			}
			site := runtimeModelSite{call: call}
			switch arg := call.Args[0].(type) {
			case *ast.CallExpr:
				site.typed = passesName != "" && isSelectorCall(arg, passesName, "NewTypedModel")
				if !site.typed {
					site.reason = "argument is a call other than passes.NewTypedModel"
				}
			case *ast.Ident:
				site.typed, site.reason = assignedFromTypedModel(decl, arg.Name, passesName)
			default:
				site.reason = "argument is neither a call nor a local"
			}
			sites = append(sites, site)
			return true
		})
	}
	return sites
}

func importName(imp *ast.ImportSpec, base string) string {
	if imp.Name != nil {
		return imp.Name.Name
	}
	return base
}

func isSelectorCall(call *ast.CallExpr, pkg, fn string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != fn {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == pkg
}

// assignedFromTypedModel reports whether every assignment to name within decl
// gives it a passes.NewTypedModel call, and that at least one such assignment exists.
func assignedFromTypedModel(decl ast.Decl, name, passesName string) (bool, string) {
	assigned, typed := 0, 0
	ast.Inspect(decl, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range as.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || id.Name != name {
				continue
			}
			assigned++
			if i < len(as.Rhs) && len(as.Rhs) == len(as.Lhs) {
				if call, ok := as.Rhs[i].(*ast.CallExpr); ok && passesName != "" && isSelectorCall(call, passesName, "NewTypedModel") {
					typed++
				}
			}
		}
		return true
	})
	switch {
	case assigned == 0:
		return false, name + " is not assigned in the enclosing declaration"
	case typed != assigned:
		return false, name + " is assigned from something other than passes.NewTypedModel"
	}
	return true, ""
}
