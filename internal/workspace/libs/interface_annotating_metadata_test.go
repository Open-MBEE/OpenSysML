package libs

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// An element restored from a recorded interface names the metadata usages
// annotating it as its parsed self does: the body one, then the `about` ones.
func TestRecordedElementListsItsAnnotatingMetadata(t *testing.T) {
	const name = "annotated.sysml"
	const text = `package P {
		metadata def Safety;
		part def Belt { @Safety; part buckle; }
		metadata onBelt : Safety about Belt;
		metadata onBoth : Safety about Belt, Belt::buckle;
	}`
	idx := NewModelIndex()
	root := parser.New(source.New(name, []byte(text))).ParseFile()
	idx.AddDocument(name, root)
	r := resolve.New(idx)
	model := semantics.NewModel(r)
	r.SetModel(model)
	r.ResolveDocument(name, root)

	rec, err := WriteInterface(name, source.KindSysML, idx, r, model, nil, nil)
	if err != nil {
		t.Fatalf("WriteInterface: %v", err)
	}
	recorded, err := symbols.BuildRecorded(rec.Scope, name)
	if err != nil {
		t.Fatalf("BuildRecorded: %v", err)
	}
	ridx := NewModelIndex()
	ridx.AddRecordedDocument(name, source.KindSysML, recorded, nil)
	rr := resolve.New(ridx)
	rmodel := semantics.NewModel(rr)
	rr.SetModel(rmodel)

	want := map[string][]string{"Belt": {"", "onBelt", "onBoth"}, "buckle": {"onBoth"}, "onBelt": nil}
	for label, fixture := range map[string]struct {
		model *semantics.Model
		root  *symbols.Scope
	}{"parsed": {model, idx.DocumentRoot(name)}, "recorded": {rmodel, ridx.DocumentRoot(name)}} {
		belt := lookupPath(t, fixture.root, "P", "Belt")
		if label == "recorded" && !belt.Recorded() {
			t.Fatal("Belt was not restored from the record")
		}
		targets := map[string]*symbols.Symbol{
			"Belt":   belt,
			"buckle": lookupPath(t, fixture.root, "P", "Belt", "buckle"),
			"onBelt": lookupPath(t, fixture.root, "P", "onBelt"),
		}
		for target, names := range want {
			var got []string
			for _, usage := range fixture.model.AnnotatingMetadataOf(targets[target]) {
				got = append(got, usage.Name)
			}
			if !slices.Equal(got, names) {
				t.Errorf("%s: annotating metadata of %s = %q, want %q", label, target, got, names)
			}
		}
	}
}
