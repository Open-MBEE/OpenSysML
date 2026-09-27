package model

import "testing"

// A recorded annotation names its metadata type by document as well as name:
// a document declaring the same name does not take the annotation over.
func TestInterfaceRecordAnnotationKeepsItsMetadataType(t *testing.T) {
	meta := []byte("package Meta { metadata def Flag { attribute on : ScalarValues::Boolean = true; } }")
	goods := []byte("package Goods { private import Meta::*; part def Plain; #Flag part def Marked; }")
	loaded := NewWorkspace()
	loaded.OpenAll([]Input{{Name: "meta.sysml", Content: meta, Version: 1}, {Name: "goods.sysml", Content: goods, Version: 1}})
	loaded.DiagnosticsAll([]string{"meta.sysml", "goods.sysml"})
	rec, err := loaded.InterfaceRecord("goods.sysml")
	if err != nil {
		t.Fatal(err)
	}
	var annotated bool
	for _, sym := range rec.Scope.Symbols {
		for _, a := range sym.Facts.Annotations {
			annotated = true
			if a.TypeFQN != "Meta::Flag" || a.Type.Doc != "meta.sysml" {
				t.Fatalf("Marked's annotation reads %+v, want Meta::Flag of meta.sysml", a)
			}
		}
	}
	if !annotated {
		t.Fatal("the record states no annotation: the fixture is vacuous")
	}

	other := []byte("package Meta { metadata def Flag { attribute on : ScalarValues::Boolean = false; } }")
	ws := NewWorkspace()
	ws.OpenAll([]Input{
		{Name: "a-meta.sysml", Content: other, Version: 1},
		{Name: "meta.sysml", Content: meta, Version: 1},
	})
	if err := ws.OpenRecorded(rec, goods); err != nil {
		t.Fatal(err)
	}
	ws.Diagnostics("meta.sysml")
	marked := ws.index.LookupQualified("Goods::Marked")
	if len(marked) != 1 {
		t.Fatalf("Goods::Marked resolves to %d symbols", len(marked))
	}
	facts := ws.model.AnnotationFactsOf(marked[0])
	if len(facts) != 1 || facts[0].Type.Doc != "meta.sysml" {
		t.Fatalf("recorded Marked's annotation restored as %+v, want Meta::Flag of meta.sysml", facts)
	}
}
