package view

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestRepeatedExpansionFold(t *testing.T) {
	src := strings.Replace(sharedQueryModel(5, 3), "metadata Layout about root { x = 0; y = 0; }", "", 1)
	r, idx := loadSources(t, []string{"shared.sysml"}, [][]byte{[]byte(src)})
	r.expansionFoldThreshold = 5
	v := lookup(t, idx, "P::v")
	query, err := r.DrawnIn(v)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(v)
	if err != nil {
		t.Fatal(err)
	}
	if got.foldedOccurrences == 0 {
		t.Fatal("no occurrence folded")
	}
	if !reflect.DeepEqual(got.Drawn(), query) {
		t.Fatal("fold changed drawn set")
	}
	if !strings.Contains(got.ExpansionFoldNotice, "Repeated definition expansions folded:") {
		t.Fatal("notice missing")
	}
	again, err := r.Render(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range []Form{FormDot, FormMermaid, FormPlantUML, FormD2} {
		first, err := got.Write(form)
		if err != nil {
			t.Fatal(err)
		}
		second, err := again.Write(form)
		if err != nil {
			t.Fatal(err)
		}
		if first != second {
			t.Fatalf("%s folding is not deterministic", form)
		}
		if !strings.Contains(first, "Repeated definition expansions folded:") || !strings.Contains(first, "repeated expansion folded") {
			t.Fatalf("%s lost the fold notice or occurrence note:\n%s", form, first)
		}
	}
	t.Logf("nodes=%d folds=%d definitions=%d", len(everyNode(got.Roots)), got.foldedOccurrences, len(got.foldedDefinitions))
}

func TestRepeatedExpansionFoldFixtureDrawnParity(t *testing.T) {
	files, err := filepath.Glob("testdata/*.sysml")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		file := filepath.Base(path)
		r, idx := loadFixture(t, file)
		r.expansionFoldThreshold = 3
		visitQuerySymbols(idx.DocumentRoot(file), func(sym *symbols.Symbol) {
			if !semantics.IsView(sym) {
				return
			}
			kind, _, err := r.KindOf(sym)
			if err != nil || kind != KindTree && kind != KindInterconnection {
				return
			}
			t.Run(file+"/"+idx.GetFQN(sym), func(t *testing.T) {
				query, err := r.DrawnIn(sym)
				if err != nil {
					t.Fatal(err)
				}
				full, err := r.Render(sym)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(full.Drawn(), query) {
					t.Fatalf("fold changed drawn set:\nfull=%#v\nquery=%#v\nnotes=%#v", full.Drawn(), query, full.Notes)
				}
			})
		})
	}
}

func TestRepeatedExpansionFoldBelowThreshold(t *testing.T) {
	for _, kind := range []string{"asTreeDiagram", "asInterconnectionDiagram"} {
		src := strings.Replace(sharedQueryModel(3, 2), "asTreeDiagram", kind, 1)
		r, idx := loadSources(t, []string{"small.sysml"}, [][]byte{[]byte(src)})
		v := lookup(t, idx, "P::v")
		baseline, err := r.Render(v)
		if err != nil {
			t.Fatal(err)
		}
		if baseline.expansionPlans[v].total != 23 || baseline.expansionFolding {
			t.Fatal("wrong unfolded count or fold below threshold")
		}
		r.expansionFoldThreshold = 23
		atThreshold, err := r.Render(v)
		if err != nil {
			t.Fatal(err)
		}
		if atThreshold.expansionFolding {
			t.Fatal("fold must trigger strictly above threshold")
		}
		r.expansionFoldThreshold = -1
		disabled, err := r.Render(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, form := range baseline.Kind.SupportedForms() {
			got, err := baseline.Write(form)
			if err != nil {
				t.Fatal(err)
			}
			want, err := disabled.Write(form)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("below-threshold %s output changed", form)
			}
		}
		r.expansionFoldThreshold = 22
		above, err := r.Render(v)
		if err != nil {
			t.Fatal(err)
		}
		if !above.expansionFolding {
			t.Fatal("fold must trigger above threshold")
		}
	}
}

func TestRepeatedExpansionCountSaturates(t *testing.T) {
	r, idx := loadSources(t, []string{"huge.sysml"}, [][]byte{[]byte(sharedQueryModel(12, 8))})
	v := lookup(t, idx, "P::v")
	exposed, err := r.model.ExposedElements(v)
	if err != nil {
		t.Fatal(err)
	}
	out := &Rendering{Kind: KindTree}
	fold := r.prepareExpansionFold(v, exposed, out, KindTree)
	if fold == nil || fold.total != 10001 {
		t.Fatal("count must saturate at 10001")
	}
	if len(fold.sizes) > 2000 {
		t.Fatalf("count unfolded %d states instead of sharing subtrees", len(fold.sizes))
	}
}
