package passes

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// sharedWorkspace is the shared semantic state a workspace keeps over its
// index: one tracked resolver, one model and one set of gathers.
type sharedWorkspace struct {
	idx      *symbols.Index
	resolver *resolve.Resolver
	model    *semantics.Model
	gathers  *Gathers
	docs     map[string]*ast.RootNamespace
}

func newSharedWorkspace() *sharedWorkspace {
	idx := newTestIndex()
	resolver := resolve.New(idx)
	sem := semantics.NewModel(resolver)
	resolver.SetModel(sem)
	sem.SetArgumentTyper(NewArgumentTyper(resolver, sem))
	resolver.Track()
	return &sharedWorkspace{idx: idx, resolver: resolver, model: sem, gathers: NewGathers(), docs: map[string]*ast.RootNamespace{}}
}

// put indexes src under name and invalidates what the change reached, as the
// workspace does on a replacement.
func (w *sharedWorkspace) put(name, src string) {
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	w.docs[name] = root
	w.idx.AddDocument(name, root)
	w.idx.ExpandWildcardImports()
	w.invalidate(name)
}

func (w *sharedWorkspace) remove(name string) {
	delete(w.docs, name)
	w.idx.RemoveDocument(name)
	w.invalidate(name)
}

func (w *sharedWorkspace) context() *Context {
	ctx := NewContextWithOptions("", source.KindSysML, w.idx, nil, Options{})
	ctx.Share(w.resolver, w.model, w.gathers)
	return ctx
}

func (w *sharedWorkspace) invalidate(name string) {
	ch := w.idx.TakeChanges()
	if ch.Docs == nil {
		ch.Docs = map[string]bool{}
	}
	ch.Docs[name] = true
	dropped := w.resolver.Invalidate(ch)
	regather := ch.Docs
	for len(regather) > 0 {
		for _, doc := range dropped {
			if gathered, ok := resolve.GatheredDoc(doc); ok {
				regather[gathered] = true
			}
		}
		changed := w.gathers.Regather(w.context(), regather)
		if len(changed) == 0 {
			break
		}
		names := map[string]bool{}
		for _, n := range changed {
			names[n] = true
		}
		dropped = w.resolver.Invalidate(symbols.Changes{Names: names})
		regather = map[string]bool{}
	}
}

func (w *sharedWorkspace) analyze(name string) []diag.Diagnostic {
	return AnalyzeShared(name, source.KindSysML, w.docs[name], nil, Options{}, w.resolver, w.model, w.gathers)
}

func (w *sharedWorkspace) analyzeAll() {
	for _, name := range w.names() {
		w.analyze(name)
	}
}

func (w *sharedWorkspace) names() []string {
	out := make([]string, 0, len(w.docs))
	for name := range w.docs {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// fresh analyzes name over a resolver and model of the run alone, the answer
// the shared analysis has to match.
func (w *sharedWorkspace) fresh(name string) []diag.Diagnostic {
	return AnalyzeWithOptions(name, source.KindSysML, w.docs[name], nil, w.idx, Options{})
}

// oosemSplit is a mission requirement in one document and a system requirement
// in each of n others, derived from nothing: whether that is a finding depends
// on the union of them all, though no document reads another.
func oosemSplit(n int) map[string]string {
	docs := map[string]string{
		"hub.sysml": oosemModel(`
		#missionRequirement requirement mission;`),
	}
	for i := 0; i < n; i++ {
		docs[fmt.Sprintf("sat%02d.sysml", i)] = fmt.Sprintf(`package S%d {
		private import OOSEM::*;
		#systemRequirement requirement sys%d;
	}`, i, i)
	}
	return docs
}

// gatherPointers are the per-document gathers held, by identity, so a test can
// tell a gather kept from one done again.
func gatherPointers(g *Gathers) map[string][3]uintptr {
	out := map[string][3]uintptr{}
	for doc, f := range g.Union("oosem").(*oosemUnion).perDoc {
		p := out[doc]
		p[0] = reflect.ValueOf(f).Pointer()
		out[doc] = p
	}
	for doc, f := range g.Union("mosa").(*mosaUnion).perDoc {
		p := out[doc]
		p[1] = reflect.ValueOf(f).Pointer()
		out[doc] = p
	}
	for doc, f := range g.Union("identity").(*identity.Union).Contributions() {
		p := out[doc]
		p[2] = reflect.ValueOf(f).Pointer()
		out[doc] = p
	}
	return out
}

// Analyzing every document of a workspace gathers each once: a second pass
// over them all finds every gather where the first left it.
func TestGathersGatherEachDocumentOnce(t *testing.T) {
	w := newSharedWorkspace()
	docs := oosemSplit(6)
	for name, src := range docs {
		w.put(name, src)
	}
	w.analyzeAll()
	first := gatherPointers(w.gathers)
	if len(first) != len(docs) {
		t.Fatalf("gathered %d documents, want %d", len(first), len(docs))
	}
	for doc, p := range first {
		if p[0] == 0 || p[1] == 0 || p[2] == 0 {
			t.Errorf("%s: gathers missing an audit: %v", doc, p)
		}
	}
	w.analyzeAll()
	if again := gatherPointers(w.gathers); !reflect.DeepEqual(first, again) {
		t.Fatalf("a second analysis of every document gathered again:\n%v\n%v", first, again)
	}
}

// A change gathers the changed document again and leaves the others' gathers
// in place; a removal drops the gather, an addition makes one.
func TestGathersRegatherOnlyTheChangedDocument(t *testing.T) {
	w := newSharedWorkspace()
	for name, src := range oosemSplit(4) {
		w.put(name, src)
	}
	w.analyzeAll()
	before := gatherPointers(w.gathers)

	w.put("sat01.sysml", "package S1 { private import OOSEM::*; #systemRequirement requirement other; }")
	after := gatherPointers(w.gathers)
	for doc, p := range before {
		if doc == "sat01.sysml" {
			if after[doc] == p {
				t.Errorf("%s: changed but its gathers were kept", doc)
			}
			continue
		}
		if after[doc] != p {
			t.Errorf("%s: unchanged but gathered again", doc)
		}
	}

	w.remove("sat02.sysml")
	if _, ok := gatherPointers(w.gathers)["sat02.sysml"]; ok {
		t.Error("a removed document keeps its gathers")
	}
	if w.gathers.Gathered("sat02.sysml") {
		t.Error("a removed document is still counted a workspace document")
	}
	w.put("sat09.sysml", "package S9 { private import OOSEM::*; #systemRequirement requirement added; }")
	if _, ok := gatherPointers(w.gathers)["sat09.sysml"]; !ok {
		t.Error("an added document has no gathers")
	}
	for _, name := range w.names() {
		if got, want := w.analyze(name), w.fresh(name); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: shared analysis\n%v\nfresh analysis\n%v", name, got, want)
		}
	}
}

// A change that moves the union — the mission requirement leaves, so no system
// requirement has a level above it to be derived from — reaches every document
// whose verdict read it, though none of them changed.
func TestGathersUnionChangeReachesItsReaders(t *testing.T) {
	w := newSharedWorkspace()
	for name, src := range oosemSplit(3) {
		w.put(name, src)
	}
	w.analyzeAll()
	if w.resolver.Owned("sat00.sysml") == 0 {
		t.Fatal("the analysis of sat00.sysml memoized nothing to drop")
	}
	if len(only(w.analyze("sat00.sysml"), CodeOOSEMRequirementNotDerived)) == 0 {
		t.Fatal("sat00.sysml: no underived system requirement while the hub declares a mission requirement")
	}
	before := gatherPointers(w.gathers)
	// With no mission requirement above them, the system requirements are no
	// longer expected to derive from one.
	w.put("hub.sysml", oosemModel(`#stakeholderNeed requirement need;`))
	if w.resolver.Owned("sat00.sysml") != 0 {
		t.Fatal("sat00.sysml kept what it memoized over a union that moved")
	}
	after := gatherPointers(w.gathers)
	for doc, p := range before {
		if doc == "hub.sysml" {
			continue
		}
		if after[doc] != p {
			t.Errorf("%s: read a union that moved, but was gathered again", doc)
		}
	}
	if got := only(w.analyze("sat00.sysml"), CodeOOSEMRequirementNotDerived); len(got) != 0 {
		t.Fatalf("sat00.sysml: underived system requirement reported with no mission requirement anywhere:\n%v", got)
	}
	for _, name := range w.names() {
		if got, want := w.analyze(name), w.fresh(name); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: shared analysis\n%v\nfresh analysis\n%v", name, got, want)
		}
	}
}

// Gathers populated once serve analyses on other goroutines, each over a
// resolver and model of its own, as a batch pool over a read-only index runs.
func TestGathersServeConcurrentAnalyses(t *testing.T) {
	w := newSharedWorkspace()
	docs := oosemSplit(8)
	for name, src := range docs {
		w.put(name, src)
	}
	w.analyze("hub.sysml")
	want := map[string][]diag.Diagnostic{}
	for _, name := range w.names() {
		want[name] = w.fresh(name)
	}
	var wg sync.WaitGroup
	errs := make(chan string, len(docs))
	for _, name := range w.names() {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			resolver := resolve.New(w.idx)
			sem := semantics.NewModel(resolver)
			resolver.SetModel(sem)
			sem.SetArgumentTyper(NewArgumentTyper(resolver, sem))
			got := AnalyzeShared(name, source.KindSysML, w.docs[name], nil, Options{}, resolver, sem, w.gathers)
			if !reflect.DeepEqual(got, want[name]) {
				errs <- fmt.Sprintf("%s: concurrent analysis\n%v\nfresh analysis\n%v", name, got, want[name])
			}
		}(name)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
