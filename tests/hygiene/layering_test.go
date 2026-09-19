package hygiene

import (
	"os/exec"
	"sort"
	"strings"
	"testing"
)

const modulePath = "github.com/Open-MBEE/OpenSysML/"

// layers is the module's layer directories under internal/ from the bottom up.
var layers = []string{
	"syntax",
	"semantic",
	"ir",
	"check",
	"exec",
	"translate",
	"doc",
	"workspace",
	"frontend",
	"tooling",
}

// permitted names the layers each layer may import, its own included; exec runs
// lowered IR and takes its argument typer from the caller, so it never imports check.
var permitted = map[string][]string{
	"syntax":    {"syntax"},
	"semantic":  {"syntax", "semantic"},
	"ir":        {"syntax", "semantic", "ir"},
	"check":     {"syntax", "semantic", "ir", "check"},
	"exec":      {"syntax", "semantic", "ir", "exec"},
	"translate": {"syntax", "semantic", "ir", "exec", "translate"},
	"doc":       {"syntax", "semantic", "ir", "exec", "translate", "doc"},
	"workspace": {"syntax", "semantic", "ir", "check", "exec", "translate", "doc", "workspace"},
	"frontend":  {"syntax", "semantic", "ir", "check", "exec", "translate", "doc", "workspace", "frontend"},
	"tooling":   layers,
}

// packageLayer assigns every package under internal/, cmd/, api/ and client/ to a
// layer; an unnamed package fails, as does an entry the root module no longer has.
var packageLayer = map[string]string{
	"internal/syntax/source":       "syntax",
	"internal/syntax/ast":          "syntax",
	"internal/syntax/diag":         "syntax",
	"internal/syntax/ast/astcodec": "syntax",
	"internal/syntax/pack":         "syntax",

	"internal/syntax/lexer":  "syntax",
	"internal/syntax/parser": "syntax",
	"internal/syntax/format": "syntax",

	"internal/core/symbols":   "semantic",
	"internal/core/suggest":   "semantic",
	"internal/core/resolve":   "semantic",
	"internal/core/semantics": "semantic",
	"internal/core/identity":  "semantic",

	"internal/core/lower":     "ir",
	"internal/core/queryplan": "ir",
	"internal/core/docplan":   "ir",
	"internal/core/view":      "ir",

	"internal/core/passes":          "check",
	"internal/core/passes/kit":      "check",
	"internal/core/passes/document": "check",
	"internal/core/passes/diagram":  "check",
	"internal/core/passes/identity": "check",
	"internal/core/passes/behavior": "check",
	"internal/core/edit":            "check",

	"internal/core/runtime":             "exec",
	"internal/core/solve":               "exec",
	"internal/core/smt":                 "exec",
	"internal/core/analysis":            "exec",
	"internal/core/analysis/enginewire": "exec",
	"internal/core/analysis/modelform":  "exec",
	"internal/core/engines":             "exec",
	"internal/core/objref":              "exec",

	"internal/core/rdf":          "translate",
	"internal/core/rdf/ontology": "translate",
	"internal/core/convert":      "translate",
	"internal/core/export":       "translate",
	"internal/core/migrate":      "translate",
	"internal/core/xmi":          "translate",
	"internal/core/xmi/sysmlv1":  "translate",
	"internal/core/codegen":      "translate",
	"internal/interop/flexo":     "translate",
	"internal/interop/reposync":  "translate",

	"internal/core/query":     "doc",
	"internal/core/queryexec": "doc",
	"internal/core/docir":     "doc",
	"internal/core/docrender": "doc",
	"internal/core/docpdf":    "doc",

	"internal/core/model":       "workspace",
	"internal/core/highlight":   "workspace",
	"internal/core/libs":        "workspace",
	"internal/core/libs/errata": "workspace",
	"internal/core/project":     "workspace",
	"internal/core/envvar":      "workspace",

	"api/proto":              "frontend",
	"api/proto/protoconnect": "frontend",
	"client/opensysml":       "frontend",
	"internal/protoconv":     "frontend",
	"internal/repl":          "frontend",
	"internal/lsp":           "frontend",
	"internal/grpc":          "frontend",
	"internal/stdiorpc":      "frontend",
	"internal/usage":         "frontend",
	"cmd/sysml":              "frontend",
	"cmd/sysml-grpc":         "frontend",
	"cmd/sysml-lsp":          "frontend",
}

// tolerated is the imports the layer table does not permit and that still
// exist, importer → imported; an entry whose edge is gone fails, so it only shrinks.
var tolerated = map[string][]string{
	"internal/core/analysis/modelform": {"internal/core/libs"},
	"internal/core/codegen":            {"internal/core/passes"},
	"internal/core/export":             {"internal/core/libs"},
	"internal/core/identity":           {"internal/core/rdf"},
	"internal/core/passes/identity":    {"internal/core/rdf"},
	"internal/core/migrate":            {"internal/core/libs"},
	"internal/core/runtime":            {"internal/core/envvar"},
}

// removed is the imports the layering took out, importer → imported; reintroducing
// one fails even where the layer table would permit it.
var removed = map[string][]string{
	"internal/core/analysis":            {"internal/core/export"},
	"internal/core/analysis/enginewire": {"internal/core/export"},
	"internal/core/export":              {"internal/core/migrate", "internal/core/runtime", "internal/core/lower"},
	"internal/core/passes/kit":          {"internal/core/passes"},
	"internal/core/passes/document":     {"internal/core/passes"},
	"internal/core/passes/diagram":      {"internal/core/passes"},
	"internal/core/passes/identity":     {"internal/core/passes"},
	"internal/core/passes/behavior":     {"internal/core/passes"},
	"internal/core/runtime":             {"internal/syntax/parser", "internal/core/passes"},
	"internal/repl":                     {"internal/grpc"},
}

// TestPackageLayering checks the import graph against the layer tables: every
// package has a layer, and the imports outside permitted are exactly the tolerated ones.
func TestPackageLayering(t *testing.T) {
	known := map[string]bool{}
	for _, l := range layers {
		known[l] = true
	}
	for pkg, layer := range packageLayer {
		if !known[layer] {
			t.Fatalf("%s is assigned to unknown layer %q", pkg, layer)
		}
	}
	allowed := map[string]map[string]bool{}
	for layer, targets := range permitted {
		if !known[layer] {
			t.Fatalf("permitted names unknown layer %q", layer)
		}
		allowed[layer] = map[string]bool{}
		for _, target := range targets {
			if !known[target] {
				t.Fatalf("permitted[%q] names unknown layer %q", layer, target)
			}
			allowed[layer][target] = true
		}
	}
	for _, l := range layers {
		if allowed[l] == nil {
			t.Fatalf("permitted has no entry for layer %q", l)
		}
	}

	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", "./internal/...", "./cmd/...")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	seen := map[string]bool{}
	listed := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		from := strings.TrimPrefix(fields[0], modulePath)
		listed[from] = true
		fromLayer, ok := packageLayer[from]
		if !ok {
			t.Errorf("%s is not assigned to a layer", from)
			continue
		}
		for _, imp := range fields[1:] {
			if !strings.HasPrefix(imp, modulePath) {
				continue
			}
			to := strings.TrimPrefix(imp, modulePath)
			if strings.HasPrefix(to, "tests/") {
				t.Errorf("%s imports %s; test support under tests/ is not reached from internal/ or cmd/", from, to)
				continue
			}
			toLayer, ok := packageLayer[to]
			if !ok {
				t.Errorf("%s imports %s, which is not assigned to a layer", from, to)
				continue
			}
			if contains(removed[from], to) {
				t.Errorf("%s imports %s again; the layering removed that edge", from, to)
				continue
			}
			if allowed[fromLayer][toLayer] {
				continue
			}
			if !contains(tolerated[from], to) {
				t.Errorf("%s (%s) imports %s (%s); %s may import only %s", from, fromLayer, to, toLayer, fromLayer, strings.Join(permitted[fromLayer], ", "))
				continue
			}
			seen[from+" -> "+to] = true
		}
	}

	var stale []string
	for from, tos := range tolerated {
		for _, to := range tos {
			if edge := from + " -> " + to; !seen[edge] {
				stale = append(stale, edge)
			}
		}
	}
	sort.Strings(stale)
	for _, edge := range stale {
		t.Errorf("%s no longer exists; move it from tolerated to removed so it cannot return", edge)
	}

	var gone []string
	for pkg := range packageLayer {
		if (strings.HasPrefix(pkg, "internal/") || strings.HasPrefix(pkg, "cmd/")) && !listed[pkg] {
			gone = append(gone, pkg)
		}
	}
	sort.Strings(gone)
	for _, pkg := range gone {
		t.Errorf("%s is not in the module; remove it from the layer table", pkg)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
