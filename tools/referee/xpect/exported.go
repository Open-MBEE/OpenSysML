package xpect

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// exportedObjectsRow adjudicates an XPECT exportedObjects assertion, which
// declares every object the file exports to the index as
// `sysml::<Metaclass>: <qualified name>` — the pilot's Xtext resource
// description, whose counterpart here is the document's contribution to the
// symbol index.
func exportedObjectsRow(ws *model.Workspace, main string, a assertion) row {
	r := row{Kind: a.Kind, Block: a.Block, Line: a.Line, Declared: fmt.Sprintf("%d exported object(s)", len(a.Exported))}

	doc := ws.Document(main)
	if doc == nil || doc.Scope == nil {
		r.Verdict = verdictUnlocated
		r.Actual = "the file did not parse into a document"
		return r
	}
	ours := exportedObjects(doc.Scope, "")
	sort.Strings(ours)
	want := append([]string(nil), a.Exported...)
	sort.Strings(want)

	missing, extra := diffSets(want, ours)
	if len(missing) == 0 && len(extra) == 0 {
		r.Verdict = verdictAgree
		r.Actual = fmt.Sprintf("%d exported object(s)", len(ours))
		return r
	}
	r.Verdict = verdictDisagree
	r.Actual = fmt.Sprintf("%d exported object(s): missing %d (%s), extra %d (%s)",
		len(ours), len(missing), sample(missing), len(extra), sample(extra))
	r.Names = &scopeCounts{Declared: len(want), Ours: len(ours), Missing: len(missing), Extra: len(extra)}
	return r
}

// exportedObjects renders the objects a scope contributes to the index, each as
// the pilot writes it. Only a named element is exported; an alias binds a name
// to an element already exported under its own.
func exportedObjects(scope *symbols.Scope, prefix string) []string {
	var out []string
	for _, sym := range scope.Members() {
		if sym == nil || sym.Kind == symbols.SymbolAlias {
			continue
		}
		name := leafOf(sym.Name)
		if name == "" {
			continue
		}
		qn := name
		if prefix != "" {
			qn = prefix + "::" + name
		}
		out = append(out, fmt.Sprintf("sysml::%s: %s", metaclassOf(sym), qn))
		if sym.Scope != nil {
			out = append(out, exportedObjects(sym.Scope, qn)...)
		}
	}
	return out
}

// metaclassOf names the abstract-syntax metaclass of a declaration, which the
// KerML keyword decides where a symbol kind alone does not (KerML §8.2.4). An
// unmapped declaration is reported as such rather than guessed.
func metaclassOf(sym *symbols.Symbol) string {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		if name := kermlMetaclasses[d.Keyword]; name != "" {
			return name
		}
	case *ast.Usage:
		if name := kermlMetaclasses[d.Keyword]; name != "" {
			return name
		}
		return "Feature"
	}
	if name := kindMetaclasses[sym.Kind]; name != "" {
		return name
	}
	return "unclassified(" + sym.Kind.String() + ")"
}

// kermlMetaclasses maps a KerML declaration keyword to its metaclass.
var kermlMetaclasses = map[string]string{
	"package":      "Package",
	"library":      "Package",
	"namespace":    "Namespace",
	"type":         "Type",
	"classifier":   "Classifier",
	"class":        "Class",
	"struct":       "Structure",
	"datatype":     "DataType",
	"assoc":        "Association",
	"association":  "Association",
	"assoc struct": "AssociationStructure",
	"behavior":     "Behavior",
	"function":     "Function",
	"predicate":    "Predicate",
	"interaction":  "Interaction",
	"metaclass":    "Metaclass",
	"feature":      "Feature",
	"step":         "Step",
	"expr":         "Expression",
	"bool":         "BooleanExpression",
	"inv":          "Invariant",
	"connector":    "Connector",
	"binding":      "BindingConnector",
	"succession":   "Succession",
	"flow":         "FlowConnectionUsage",
	"multiplicity": "MultiplicityRange",
}

// kindMetaclasses maps the symbol kinds whose declaration carries no KerML
// keyword to their metaclass.
var kindMetaclasses = map[symbols.SymbolKind]string{
	symbols.SymbolPackage:   "Package",
	symbols.SymbolNamespace: "Namespace",
}

// leafOf is the last segment of a symbol's name, which the index may hold
// qualified.
func leafOf(name string) string {
	if at := strings.LastIndex(name, "::"); at >= 0 {
		return name[at+len("::"):]
	}
	return name
}

// diffSets returns what want holds and got does not, and the reverse. Both
// inputs are sorted.
func diffSets(want, got []string) (missing, extra []string) {
	have := map[string]bool{}
	for _, s := range got {
		have[s] = true
	}
	seek := map[string]bool{}
	for _, s := range want {
		seek[s] = true
		if !have[s] {
			missing = append(missing, s)
		}
	}
	for _, s := range got {
		if !seek[s] {
			extra = append(extra, s)
		}
	}
	return missing, extra
}
