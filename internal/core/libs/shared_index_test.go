package libs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// sharedLibraryBase loads the bundled library into one index and freezes it, so
// every model can resolve against that one copy.
func sharedLibraryBase(t *testing.T) *symbols.Index {
	t.Helper()
	idx := symbols.NewIndex()
	if err := NewLoader(DefaultSource(), &Cache{dir: t.TempDir()}).LoadAll(idx); err != nil {
		t.Fatalf("load the standard library: %v", err)
	}
	idx.Freeze()
	return idx
}

// ownLibraryIndex loads the bundled library into an index of its own, which is
// what a model held before the base was shared.
func ownLibraryIndex(t *testing.T) *symbols.Index {
	t.Helper()
	idx := symbols.NewIndex()
	if err := NewLoader(DefaultSource(), &Cache{dir: t.TempDir()}).LoadAll(idx); err != nil {
		t.Fatalf("load the standard library: %v", err)
	}
	return idx
}

// sharedIndexCorpus names the real models these cases resolve, chosen for what
// they ask of the library: units and quantities, behaviour, and requirements.
func sharedIndexCorpus(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, pattern := range []string{
		"../../../examples/*.sysml",
		"../../../tests/testdata/*.sysml",
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		out = append(out, matches...)
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatal("no corpus files to resolve against the library")
	}
	return out
}

// analyse adds path to idx as a model's document and returns the diagnostics the
// passes report, rendered so two indexes can be compared.
func analyse(t *testing.T, idx *symbols.Index, path string) []string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	kind := source.KindOf(path)
	p := parser.New(source.NewWithKind(path, content, kind))
	root := p.ParseFile()
	idx.AddDocumentWithKind(path, root, kind)
	idx.ExpandWildcardImports()
	if len(p.Diagnostics) > 0 {
		return nil // a parse error gates the passes, as it does in every consumer
	}
	var out []string
	for _, d := range passes.AnalyzeWithOptions(path, kind, root, nil, idx, passes.Options{}) {
		out = append(out, fmt.Sprintf("%s %s %d %s", d.Code, d.Severity, d.Span.Offset, d.Message))
	}
	sort.Strings(out)
	return out
}

// A model resolving against the shared library base must reach every conclusion
// it reaches against a library index of its own — the diagnostics it reports and
// every qualified name in it — over real models rather than a fixture.
func TestModelsOverASharedLibraryAgreeWithModelsOverTheirOwn(t *testing.T) {
	base := sharedLibraryBase(t)
	for _, path := range sharedIndexCorpus(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			over := symbols.NewOverlay(base)
			own := ownLibraryIndex(t)
			shared, alone := analyse(t, over, path), analyse(t, own, path)
			if len(shared) != len(alone) {
				t.Fatalf("%d diagnostics over the shared library, %d over its own:\n shared %v\n own    %v",
					len(shared), len(alone), shared, alone)
			}
			for i := range shared {
				if shared[i] != alone[i] {
					t.Fatalf("diagnostic %d differs:\n shared %s\n own    %s", i, shared[i], alone[i])
				}
			}

			fqns, wantFQNs := over.FQNs(), own.FQNs()
			if len(fqns) != len(wantFQNs) {
				t.Fatalf("%d FQNs over the shared library, %d over its own", len(fqns), len(wantFQNs))
			}
			for i := range fqns {
				if fqns[i] != wantFQNs[i] {
					t.Fatalf("FQN %d is %q over the shared library, %q over its own", i, fqns[i], wantFQNs[i])
				}
				if got, want := lookupNames(over, fqns[i]), lookupNames(own, fqns[i]); got != want {
					t.Fatalf("LookupQualified(%q) = %s over the shared library, %s over its own",
						fqns[i], got, want)
				}
				if got, want := childNames(over, fqns[i]), childNames(own, fqns[i]); got != want {
					t.Fatalf("LookupDirectChildren(%q) = %s over the shared library, %s over its own",
						fqns[i], got, want)
				}
			}
			if got, want := bindings(over, path), bindings(own, path); got != want {
				t.Fatalf("TopLevelBindings(%q) = %s over the shared library, %s over its own", path, got, want)
			}
		})
	}
}

// Library marking is what tells a library name from the user's, and it has to
// survive being read through the base: a consumer withholding library content
// depends on it.
func TestLibraryMarkingReadsThroughTheSharedBase(t *testing.T) {
	base := sharedLibraryBase(t)
	over := symbols.NewOverlay(base)
	analyse(t, over, "../../../examples/combined-behavioral-demo.sysml")

	marked, unmarked := 0, 0
	for _, fqn := range over.FQNs() {
		for _, sym := range over.LookupQualified(fqn) {
			if over.Library(sym) {
				marked++
			} else {
				unmarked++
			}
		}
	}
	if marked == 0 {
		t.Error("no symbol is marked as library content through the shared base")
	}
	if unmarked == 0 {
		t.Error("the model's own symbols are marked as library content")
	}
	for _, sym := range over.LookupQualified("ISQ") {
		if !over.Library(sym) {
			t.Error("ISQ is not marked as library content through the shared base")
		}
	}
}

// The point of the shared base: a model costs its own document rather than
// another copy of the library. One index per model measured ~15.9 MiB retained
// at the gRPC cache's default of 100 models; an overlay must stay far under it.
func TestAModelOverTheSharedBaseCostsFarLessThanItsOwnIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("measuring retained memory loads the library")
	}
	const models = 100
	base := sharedLibraryBase(t)
	path := "../../../examples/combined-behavioral-demo.sysml"

	before := heapInUse()
	overlays := make([]*symbols.Index, 0, models)
	for i := 0; i < models; i++ {
		over := symbols.NewOverlay(base)
		analyse(t, over, path)
		overlays = append(overlays, over)
	}
	perModel := float64(heapInUse()-before) / models / (1 << 20)
	runtime.KeepAlive(overlays)

	t.Logf("%d models over one shared library base: %.2f MiB retained per model", models, perModel)
	if perModel > 4 {
		t.Errorf("a model over the shared base retains %.2f MiB, want well under the 15.9 MiB "+
			"an index of its own cost", perModel)
	}
}

// heapInUse returns the live heap after a collection, so what a measurement
// holds is what it retains.
func heapInUse() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func lookupNames(idx *symbols.Index, fqn string) string {
	return names(idx.LookupQualified(fqn))
}

func childNames(idx *symbols.Index, prefix string) string {
	return names(idx.LookupDirectChildren(prefix))
}

// names renders symbols by name and kind, since their identity differs between
// two indexes over the same sources.
func names(syms []*symbols.Symbol) string {
	out := make([]string, 0, len(syms))
	for _, sym := range syms {
		out = append(out, fmt.Sprintf("%s/%v", sym.Name, sym.Kind))
	}
	sort.Strings(out)
	return fmt.Sprint(out)
}

func bindings(idx *symbols.Index, doc string) string {
	out := make([]string, 0)
	for _, b := range idx.TopLevelBindings(doc) {
		out = append(out, b.Name+"="+b.Sym.Name)
	}
	sort.Strings(out)
	return fmt.Sprint(out)
}
