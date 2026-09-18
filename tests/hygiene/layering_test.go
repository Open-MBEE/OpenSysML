package hygiene

import (
	"os/exec"
	"sort"
	"strings"
	"testing"
)

const modulePath = "github.com/Open-MBEE/OpenSysML/"

// layers is the module's layers from the bottom up, as the roadmap's package
// layering track states them.
var layers = []string{
	"foundation",
	"syntax",
	"semantics",
	"semantic IR",
	"validation",
	"execution",
	"translation",
	"documents",
	"workspace",
	"frontends",
	"tooling",
}

// permitted names the layers each layer may import, its own included; execution
// runs lowered IR, so it reaches neither syntax nor validation.
var permitted = map[string][]string{
	"foundation":  {"foundation"},
	"syntax":      {"foundation", "syntax"},
	"semantics":   {"foundation", "semantics"},
	"semantic IR": {"foundation", "semantics", "semantic IR"},
	"validation":  {"foundation", "syntax", "semantics", "semantic IR", "validation"},
	"execution":   {"foundation", "semantics", "semantic IR", "execution"},
	"translation": {"foundation", "syntax", "semantics", "semantic IR", "execution", "translation"},
	"documents":   {"foundation", "semantics", "semantic IR", "execution", "translation", "documents"},
	"workspace":   {"foundation", "syntax", "semantics", "semantic IR", "validation", "execution", "translation", "documents", "workspace"},
	"frontends":   {"foundation", "syntax", "semantics", "semantic IR", "validation", "execution", "translation", "documents", "workspace", "frontends"},
	"tooling":     layers,
}

// packageLayer assigns every package under internal/, cmd/, api/ and client/ to a
// layer; an unnamed package fails, as does an entry the root module no longer has.
var packageLayer = map[string]string{
	"internal/core/source":       "foundation",
	"internal/core/ast":          "foundation",
	"internal/core/diag":         "foundation",
	"internal/core/ast/astcodec": "foundation",
	"internal/core/pack":         "foundation",

	"internal/core/lexer":  "syntax",
	"internal/core/parser": "syntax",
	"internal/core/format": "syntax",

	"internal/core/symbols":   "semantics",
	"internal/core/suggest":   "semantics",
	"internal/core/resolve":   "semantics",
	"internal/core/semantics": "semantics",
	"internal/core/identity":  "semantics",

	"internal/core/lower":     "semantic IR",
	"internal/core/queryplan": "semantic IR",
	"internal/core/docplan":   "semantic IR",
	"internal/core/view":      "semantic IR",

	"internal/core/passes": "validation",
	"internal/core/rename": "validation",
	"internal/core/edit":   "validation",

	"internal/core/runtime":             "execution",
	"internal/core/solve":               "execution",
	"internal/core/smt":                 "execution",
	"internal/core/analysis":            "execution",
	"internal/core/analysis/enginewire": "execution",
	"internal/core/engines":             "execution",
	"internal/core/objref":              "execution",

	"internal/core/rdf":          "translation",
	"internal/core/rdf/ontology": "translation",
	"internal/core/convert":      "translation",
	"internal/core/export":       "translation",
	"internal/core/migrate":      "translation",
	"internal/core/xmi":          "translation",
	"internal/core/codegen":      "translation",
	"internal/interop/flexo":     "translation",
	"internal/interop/reposync":  "translation",

	"internal/core/query":     "documents",
	"internal/core/queryexec": "documents",
	"internal/core/docir":     "documents",
	"internal/core/docrender": "documents",
	"internal/core/docpdf":    "documents",

	"internal/core/model":       "workspace",
	"internal/core/highlight":   "workspace",
	"internal/core/libs":        "workspace",
	"internal/core/libs/errata": "workspace",
	"internal/core/project":     "workspace",
	"internal/core/envvar":      "workspace",

	"api/proto":              "frontends",
	"api/proto/protoconnect": "frontends",
	"client/opensysml":       "frontends",
	"internal/protoconv":     "frontends",
	"internal/repl":          "frontends",
	"internal/lsp":           "frontends",
	"internal/grpc":          "frontends",
	"internal/stdiorpc":      "frontends",
	"internal/usage":         "frontends",
	"cmd/sysml":              "frontends",
	"cmd/sysml-grpc":         "frontends",
	"cmd/sysml-lsp":          "frontends",

	"internal/fixtures":    "tooling",
	"internal/stressmodel": "tooling",
}

// tolerated is the imports the layer table does not permit and that still
// exist, importer → imported; an entry whose edge is gone fails, so it only shrinks.
var tolerated = map[string][]string{
	"internal/core/analysis":            {"internal/core/export"},
	"internal/core/analysis/enginewire": {"internal/core/export"},
	"internal/core/codegen":             {"internal/core/passes"},
	"internal/core/export":              {"internal/core/libs"},
	"internal/core/identity":            {"internal/core/rdf"},
	"internal/core/migrate":             {"internal/core/libs"},
	"internal/core/passes":              {"internal/core/rdf"},
	"internal/core/runtime":             {"internal/core/envvar"},
}

// removed is the imports the layering took out, importer → imported; reintroducing
// one fails even where the layer table would permit it.
var removed = map[string][]string{
	"internal/core/export":  {"internal/core/migrate"},
	"internal/core/runtime": {"internal/core/parser", "internal/core/passes"},
	"internal/repl":         {"internal/grpc"},
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
