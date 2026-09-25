package semantics

import (
	"reflect"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const sharedAnnotationsSrc = `
	metadata def Layout { attribute x : Integer = 0; attribute y : Integer = 0; attribute z : Integer; }
	part a { @Layout { z = 1; } }
	part b { @Layout { x = 5; } }
	part c;
	part d;
	metadata la : Layout about c, d { y = 2; }
`

// Every annotation of one metadata type reads the type's declared values from one
// map memoized per type, not a copy merged into each annotation.
func TestAnnotationsOfOneTypeShareItsDefaults(t *testing.T) {
	m, root := buildModel(t, sharedAnnotationsSrc)
	var defaults []map[string]symbols.FilterValue
	for _, name := range []string{"a", "b", "c", "d"} {
		annots := m.annotationsOf(sym(t, root, name))
		if len(annots) != 1 {
			t.Fatalf("%s bears %d annotations, want 1", name, len(annots))
		}
		defaults = append(defaults, annots[0].defaults)
	}
	for i := 1; i < len(defaults); i++ {
		if reflect.ValueOf(defaults[i]).Pointer() != reflect.ValueOf(defaults[0]).Pointer() {
			t.Errorf("annotation %d holds defaults of its own rather than Layout's", i)
		}
	}
	// What the body binds shadows the default; what it leaves unbound reads it.
	want := map[string]map[string]int64{
		"a": {"x": 0, "y": 0, "z": 1},
		"b": {"x": 5, "y": 0},
		"c": {"x": 0, "y": 2},
		"d": {"x": 0, "y": 2},
	}
	for name, values := range want {
		facts := m.AnnotationFactsOf(sym(t, root, name))
		if len(facts) != 1 {
			t.Fatalf("%s reports %d annotations, want 1", name, len(facts))
		}
		got := map[string]int64{}
		for _, v := range facts[0].Values {
			got[v.Feature] = v.Value.Int
		}
		if !reflect.DeepEqual(got, values) {
			t.Errorf("%s values = %v, want %v", name, got, values)
		}
		if _, unbound := m.annotationsOf(sym(t, root, name))[0].value("nothing"); unbound {
			t.Errorf("%s values a feature Layout does not declare", name)
		}
	}
}

// Models sharing an AboutIndex build it once, however many ask at once, and read
// one and the same index.
func TestModelsOverOneIndexShareTheAboutIndex(t *testing.T) {
	_, root, r, _ := buildUnresolvedModel(t, "t.sysml", source.KindOf("t.sysml"), sharedAnnotationsSrc)
	idx := r.Index()
	shared := NewAboutIndex()
	const workers = 8
	models := make([]*Model, workers)
	var wg sync.WaitGroup
	for i := range models {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m := NewModel(resolve.New(idx))
			m.ShareAbout(shared)
			m.AboutAnnotatedSymbols()
			models[i] = m
		}(i)
	}
	wg.Wait()
	first := reflect.ValueOf(models[0].aboutAnnots).Pointer()
	for i, m := range models {
		if reflect.ValueOf(m.aboutAnnots).Pointer() != first {
			t.Errorf("model %d indexed the about annotations on its own", i)
		}
		if got := annotationTypes(m, sym(t, root, "c")); len(got) != 1 || got[0] != "Layout" {
			t.Errorf("model %d: annotations of c = %v, want [Layout]", i, got)
		}
	}
	if got := len(shared.order); got != 2 {
		t.Errorf("the shared index lists %d annotated elements, want 2 (c, d)", got)
	}
}
