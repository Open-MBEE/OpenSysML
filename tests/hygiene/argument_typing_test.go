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

// Runtime model builders whose construction sites the check must keep seeing;
// a restructuring that hides one of these sites from the walk fails here.
var runtimeModelBuilders = []string{
	"internal/frontend/repl/session.go",
	"internal/frontend/grpc/cache.go",
	"internal/workspace/modelrt/modelrt.go",
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
// directly or through a detached model's typed Semantics accessor, so the
// runtime selects overloads with the checker's argument typing and its
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
	detachedSemanticsTyped, err := detachedSemanticsIsTyped(root)
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
			for _, site := range runtimeModelSites(file, detachedSemanticsTyped) {
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
	for _, want := range runtimeModelBuilders {
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
// first argument is a passes.NewTypedModel call, a local assigned from one, or
// the typed Semantics accessor on a value returned from Detach.
func runtimeModelSites(file *ast.File, detachedSemanticsTyped bool) []runtimeModelSite {
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
				site.typed = site.typed || detachedSemanticsTyped && detachedSemanticsCall(arg, decl)
				if !site.typed {
					site.reason = "argument is neither passes.NewTypedModel nor a typed Detached.Semantics"
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

func detachedSemanticsCall(call *ast.CallExpr, decl ast.Decl) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Semantics" {
		return false
	}
	receiver, ok := sel.X.(*ast.Ident)
	return ok && assignedFromDetach(decl, receiver.Name)
}

func assignedFromDetach(decl ast.Decl, name string) bool {
	assigned, detached := 0, 0
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
			if i == 0 && len(as.Rhs) == 1 {
				call, ok := as.Rhs[i].(*ast.CallExpr)
				if !ok {
					continue
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if ok && sel.Sel.Name == "Detach" {
					detached++
				}
			}
		}
		return true
	})
	return assigned == 1 && detached == 1
}

func detachedSemanticsIsTyped(root string) (bool, error) {
	path := filepath.Join(root, "internal/workspace/model/detached.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return false, err
	}
	typedAssignment, stored, accessor := false, false, false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		switch fn.Name.Name {
		case "detachLocked":
			typedAssignment, _ = assignedFromTypedModel(fn, "sem", "passes")
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.KeyValueExpr:
					key, keyOK := n.Key.(*ast.Ident)
					value, valueOK := n.Value.(*ast.Ident)
					stored = stored || keyOK && valueOK && key.Name == "semantics" && value.Name == "sem"
				}
				return true
			})
		case "Semantics":
			if fn.Recv != nil && len(fn.Recv.List) == 1 &&
				isPointerIdentType(fn.Recv.List[0].Type, "Detached") &&
				len(fn.Type.Results.List) == 1 && isPointerSelectorType(fn.Type.Results.List[0].Type, "semantics", "Model") {
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					ret, ok := n.(*ast.ReturnStmt)
					if !ok || len(ret.Results) != 1 {
						return true
					}
					field, ok := ret.Results[0].(*ast.SelectorExpr)
					if !ok {
						return true
					}
					receiver, receiverOK := field.X.(*ast.Ident)
					accessor = accessor || receiverOK && receiver.Name == "d" && field.Sel.Name == "semantics"
					return true
				})
			}
		}
	}
	return typedAssignment && stored && accessor, nil
}

func isPointerIdentType(expr ast.Expr, name string) bool {
	ptr, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	id, ok := ptr.X.(*ast.Ident)
	return ok && id.Name == name
}

func isPointerSelectorType(expr ast.Expr, pkg, name string) bool {
	ptr, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := ptr.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	pkgName, ok := sel.X.(*ast.Ident)
	return ok && pkgName.Name == pkg
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
