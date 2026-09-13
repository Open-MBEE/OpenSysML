package solve

// The corpus gates run the differential property over the models the repository
// already gates. Every untranslatable element is counted as a refusal rather
// than passed over silently, so coverage drift is reviewable in the summary.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const (
	conformanceDir = "../runtime/testdata/conformance"
	trainingDir    = "../../../examples/sysml-v2-training"

	// Set in CI, where the corpora are provided, so that an absent one fails the
	// gate instead of passing by covering nothing.
	corpusRequiredEnv = "OPENSYSML_REQUIRE_TRAINING_CORPUS"
)

// conditionElements collects the elements of a document the gate checks: every named
// constraint and requirement, the anonymous ones no such element owns, and every
// satisfaction assertion, in declaration order.
func conditionElements(ctx *runtime.Context, idx *symbols.Index, doc string) []diffElement {
	root := idx.DocumentRoot(doc)
	if root == nil {
		return nil
	}
	var out []diffElement
	walkScopes(root, func(scope *symbols.Scope) {
		for _, sym := range scopeMembers(scope) {
			kind := elementKind(sym)
			if kind == "" || coveredByOwner(sym) {
				continue
			}
			// A definition states no values, so it is checked through the usages
			// that do, which this walk reaches in their own right.
			if definitionKind(sym.Kind) && used(ctx, root, sym) {
				continue
			}
			for _, host := range hostsOf(ctx, root, hostOf(sym)) {
				out = append(out, diffElement{file: doc, kind: kind, name: elementName(sym) + onHost(host, sym),
					sym: sym, scope: sym.OwnerScope, host: host})
			}
		}
	})
	for _, a := range ctx.SatisfyAssertionsIn(root) {
		out = append(out, diffElement{file: doc, kind: "satisfaction", name: a.Text(), sym: a.Symbol, assertion: a})
	}
	return out
}

// hostOf is the object whose values an element's conditions read: the usage it
// is stated inside, or the element itself when a package states it, as a usage
// carries values of its own. It is nil for a definition, which carries none.
func hostOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner != nil && owner.Kind != symbols.SymbolPackage {
		if definitionKind(owner.Kind) {
			return nil
		}
		return owner
	}
	if definitionKind(sym.Kind) {
		return nil
	}
	return sym
}

// hostsOf names the objects an element is checked on: an abstract host declares
// no values, so it is checked through the usages specializing it that do.
func hostsOf(ctx *runtime.Context, root *symbols.Scope, host *symbols.Symbol) []*symbols.Symbol {
	if host == nil || !abstract(host) {
		return []*symbols.Symbol{host}
	}
	var out []*symbols.Symbol
	walkScopes(root, func(scope *symbols.Scope) {
		for _, sym := range scopeMembers(scope) {
			if sym == host || sym.Name == "" || definitionKind(sym.Kind) || abstract(sym) {
				continue
			}
			if ctx.Semantics().Conforms(sym, host) {
				out = append(out, sym)
			}
		}
	})
	if len(out) == 0 {
		return []*symbols.Symbol{host}
	}
	return out
}

// abstract reports whether a declaration is abstract, so no object is it.
func abstract(sym *symbols.Symbol) bool {
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		return decl.IsAbstract
	case *ast.Definition:
		return decl.IsAbstract
	}
	return false
}

// onHost names the object an element is checked on, when that is not the
// declaration stating it.
func onHost(host, sym *symbols.Symbol) string {
	if host == nil || host == hostOf(sym) {
		return ""
	}
	return " on " + elementName(host)
}

// used reports whether the document states a usage of a definition, which is
// where the values its conditions read are declared.
func used(ctx *runtime.Context, root *symbols.Scope, def *symbols.Symbol) bool {
	found := false
	walkScopes(root, func(scope *symbols.Scope) {
		for _, sym := range scopeMembers(scope) {
			if sym == def || definitionKind(sym.Kind) || elementKind(sym) == "" {
				continue
			}
			if ctx.Semantics().Conforms(sym, def) {
				found = true
			}
		}
	})
	return found
}

// definitionKind reports whether a kind is a definition rather than a usage:
// every definition kind's name ends in "Def".
func definitionKind(kind symbols.SymbolKind) bool {
	return strings.HasSuffix(kind.String(), "Def")
}

// walkScopes visits a scope and, recursively, the scopes nested in it.
func walkScopes(scope *symbols.Scope, visit func(*symbols.Scope)) {
	if scope == nil {
		return
	}
	visit(scope)
	for _, child := range scope.Children() {
		walkScopes(child, visit)
	}
}

// scopeMembers returns the symbols declared directly in a scope, named ones
// first, then the anonymous ones an `assert constraint { ... }` declares.
func scopeMembers(scope *symbols.Scope) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, name := range scope.MemberNames() {
		out = append(out, scope.LookupLocalAll(name)...)
	}
	return append(out, scope.AnonymousMembers()...)
}

// elementKind names the kind of element the gate checks a symbol as, or "" for
// a symbol that states no conditions.
func elementKind(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	switch sym.Kind {
	case symbols.SymbolConstraintDef, symbols.SymbolConstraintUsage:
		return "constraint"
	case symbols.SymbolRequirementDef, symbols.SymbolRequirementUsage:
		return "requirement"
	}
	return ""
}

// coveredByOwner reports whether an element is a condition of an element the gate
// already checks, which its owner's translation covers.
func coveredByOwner(sym *symbols.Symbol) bool {
	if sym.Name != "" {
		return false
	}
	if sym.OwnerScope == nil {
		return false
	}
	return elementKind(sym.OwnerScope.Owner()) != ""
}

// elementName names an element as a verdict about it would.
func elementName(sym *symbols.Symbol) string {
	if fqn := symbols.FQNOf(sym); fqn != "" {
		return fqn
	}
	if sym.Name != "" {
		return sym.Name
	}
	return "<anonymous>"
}

// mentionsConditions reports whether a source could declare an element the gate
// checks: one cannot exist without the keyword that declares it, so this filter
// keeps a large corpus from being parsed for nothing without hiding a candidate.
func mentionsConditions(src []byte) bool {
	text := string(src)
	return strings.Contains(text, "constraint") ||
		strings.Contains(text, "requirement") ||
		strings.Contains(text, "satisfy")
}

// modelOf parses one file into a runtime context, loading the standard library
// when asked for, as the conformance runner does.
func modelOf(t *testing.T, path string, src []byte, libraries bool) (*runtime.Context, *symbols.Index) {
	t.Helper()
	idx := symbols.NewIndex()
	if libraries {
		idx = libraryIndex()
	}
	sf := source.New(path, src)
	idx.AddDocument(path, parser.New(sf).ParseFile())
	if libraries {
		idx.ExpandWildcardImports()
	}
	resolver := resolve.New(idx)
	ctx := runtime.NewContext(runtime.NewModel(semantics.NewModel(resolver), resolver), 10000)
	ctx.Model().RegisterSource(sf)
	return ctx, idx
}

// needsLibraries reads the conformance case's declaration of whether its model
// names standard library elements.
func needsLibraries(path string) bool {
	data, err := os.ReadFile(strings.TrimSuffix(path, ".sysml") + ".expected.json")
	if err != nil {
		return true
	}
	var expected struct {
		Libraries bool `json:"libraries"`
	}
	if json.Unmarshal(data, &expected) != nil {
		return true
	}
	return expected.Libraries
}

// TestDifferentialConformanceCorpus runs the gate over the runtime conformance
// corpus: every constraint, requirement and satisfaction assertion those
// fixtures declare, on the values they declare, has to make the evaluator and
// the solver agree.
func TestDifferentialConformanceCorpus(t *testing.T) {
	gate := newGate(t, "runtime conformance corpus")

	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatalf("read the conformance corpus: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sysml" {
			continue
		}
		path := filepath.Join(conformanceDir, entry.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !mentionsConditions(src) {
			continue
		}
		gate.summary.files++
		t.Run(entry.Name(), func(t *testing.T) {
			ctx, idx := modelOf(t, path, src, needsLibraries(path))
			for _, el := range conditionElements(ctx, idx, path) {
				gate.check(t, ctx, el)
			}
		})
	}
	if gate.summary.files == 0 {
		t.Fatalf("no conformance fixture declares conditions: %s", conformanceDir)
	}
	gate.summary.report(t)
	if gate.summary.counts[diffAgreed] == 0 {
		t.Errorf("the gate agreed on nothing, so it checked nothing: %+v", gate.summary.counts)
	}
}

// TestDifferentialStandardLibrary runs the gate over the bundled standard
// library: its constraints are mostly outside the translatable subset and
// declare no concrete values, so this gate measures how much of a library model
// the translation reaches rather than proving verdicts about it.
func TestDifferentialStandardLibrary(t *testing.T) {
	gate := newGate(t, "standard library")

	idx := symbols.NewIndex()
	parseLibraries(t, idx)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := runtime.NewContext(runtime.NewModel(semantics.NewModel(resolver), resolver), 10000)

	for _, doc := range libraryDocuments(idx) {
		gate.summary.files++
		for _, el := range conditionElements(ctx, idx, doc) {
			gate.check(t, ctx, el)
		}
	}
	if gate.summary.elements == 0 {
		t.Fatal("the standard library declares no constraint or requirement, which cannot be right")
	}
	gate.summary.report(t)
}

// parseLibraries indexes the standard library as ordinary documents, bypassing
// the loader: a library it registers is index-only and holds no condition to
// read, so this gate has to parse the files itself.
func parseLibraries(t *testing.T, idx *symbols.Index) {
	t.Helper()
	src := libs.DefaultSource()
	for _, name := range src.List() {
		content, err := src.Read(name)
		if err != nil {
			t.Fatalf("read library %s: %v", name, err)
		}
		idx.AddDocument(name, parser.New(source.New(name, content)).ParseFile())
	}
}

// libraryDocuments names the indexed standard library documents, in a stable order.
func libraryDocuments(idx *symbols.Index) []string {
	var out []string
	for _, name := range libs.DefaultSource().List() {
		if idx.DocumentRoot(name) != nil {
			out = append(out, name)
		}
	}
	return out
}

// TestDifferentialTrainingCorpus runs the gate over the OMG training corpus,
// which is not vendored: it skips while the corpus is absent unless CI declares
// it mandatory.
func TestDifferentialTrainingCorpus(t *testing.T) {
	files := trainingModels(t)
	gate := newGate(t, "OMG training corpus")

	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !mentionsConditions(src) {
			continue
		}
		gate.summary.files++
		t.Run(filepath.Base(path), func(t *testing.T) {
			ctx, idx := modelOf(t, path, src, true)
			for _, el := range conditionElements(ctx, idx, path) {
				gate.check(t, ctx, el)
			}
		})
	}
	if gate.summary.files == 0 {
		t.Fatalf("no training model declares conditions: %s", trainingDir)
	}
	gate.summary.report(t)
}

// trainingModels lists the corpus files, skipping loudly when the corpus has not
// been downloaded — and failing when CI declares it mandatory, so the gate
// cannot pass green without running.
func trainingModels(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.Walk(trainingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".sysml" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil || len(files) == 0 {
		if required := os.Getenv(corpusRequiredEnv); required != "" {
			t.Fatalf("%s=%s but the OMG training corpus is not at %s (%v): "+
				"run ./scripts/download-training-examples.sh", corpusRequiredEnv, required, trainingDir, err)
		}
		t.Skipf("the OMG training corpus is not at %s (run ./scripts/download-training-examples.sh)", trainingDir)
	}
	return files
}
