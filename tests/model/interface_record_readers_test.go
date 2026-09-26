package model_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Readers that switch on a declaration's kind or modifiers must read the
// recorded fact when the declaring document is recorded: each case here is a
// question another document asks whose answer used to need the tree.
func TestInterfaceRecordDeclarationReaders(t *testing.T) {
	t.Parallel()
	cases := map[string]map[string][]byte{
		// The redefining end's import names a feature nested under the end it
		// redefines (KerML 8.3.4.4); resolving it starts in the recorded scope.
		"nested in redefined": {
			"a.kerml": []byte(`package P {
	class Account;
	assoc A {
		end cart : Account[1] {
			member feature inCart : Account[0..1] {
				member feature nested : Account { member feature deep : Account; }
			}
		}
		end feature account : Account[1];
	}
	assoc B specializes A {
		end cart : Account[1] redefines cart { public import nested::*; }
		end feature account : Account[1];
	}
}
`),
			"b.kerml": []byte(`package Q {
	private import P::B::cart::*;
	feature f : deep;
}
`),
		},
		// A satisfy naming a viewpoint states no requirement satisfied, so the
		// system requirement in a third document is not expected to be.
		"viewpoint satisfaction": {
			"reqs.sysml": []byte(`package Reqs {
	private import OOSEM::*;
	#systemRequirement requirement sys;
}
`),
			"vantage.sysml": []byte(`package Vantage {
	viewpoint def VP;
	viewpoint vp : VP;
}
`),
			"links.sysml": []byte(`package Links {
	private import OOSEM::*;
	part sat { satisfy Vantage::vp; }
}
`),
		},
		// A variation specializing a variation is refused, which needs the general's modifier.
		"variation": {
			"v.sysml": []byte(`package V {
	variation part def Color { variant part red : Color; variant part blue : Color; }
}
`),
			"u.sysml": []byte(`package U {
	variation part def Paint :> V::Color { variant part p : Paint; }
}
`),
		},
	}
	for name, docs := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ws := model.NewWorkspace()
			inputs := make([]model.Input, 0, len(docs))
			for n, c := range docs {
				inputs = append(inputs, model.Input{Name: n, Content: c, Version: 1})
			}
			ws.OpenAll(inputs)
			if recorded := recordDifferential(t, docs); recorded != len(docs) {
				t.Fatalf("%d of %d documents recorded", recorded, len(docs))
			}
		})
	}
}

// Two documents of equal content are two documents: each has its own record
// under its own key, and installing one never displaces the other.
func TestInterfaceRecordKeyNamesTheDocument(t *testing.T) {
	t.Parallel()
	content := []byte("package Common { part def C; }")
	docs := map[string][]byte{"a.sysml": content, "b.sysml": content}
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	digest := libs.SourceDigest(libs.DefaultSource())
	keyA := cache.InterfaceKey("a.sysml", content, digest, diag.ConformanceDefault)
	keyB := cache.InterfaceKey("b.sysml", content, digest, diag.ConformanceDefault)
	if keyA == keyB {
		t.Fatalf("one key %s for two documents of equal content", keyA)
	}
	if recorded := recordDifferential(t, docs); recorded != 2 {
		t.Fatalf("%d of 2 documents recorded", recorded)
	}
	loaded := model.NewWorkspace()
	loaded.OpenAll([]model.Input{{Name: "a.sysml", Content: content, Version: 1}, {Name: "b.sysml", Content: content, Version: 1}})
	ws := model.NewWorkspace()
	for name, key := range map[string]string{"a.sysml": keyA, "b.sysml": keyB} {
		rec, err := loaded.InterfaceRecord(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := cache.StoreInterface(key, rec); err != nil {
			t.Fatal(err)
		}
		read, ok := cache.LoadInterface(key)
		if !ok || read.Name != name {
			t.Fatalf("%s: read back %v (%v)", name, read, ok)
		}
		if err := ws.OpenRecorded(read, content); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"a.sysml", "b.sysml"} {
		if doc := ws.Document(name); doc == nil || !doc.Recorded() {
			t.Fatalf("%s is not held as its record", name)
		}
	}
}

// A recorded document keeps the text its record was written from, so its
// stored diagnostics locate in it and its digest is the text's; other bytes
// are refused rather than reported against. What the record does not carry —
// the references its body writes — is answered as a hydration request.
func TestInterfaceRecordKeepsContentAndRefusesBodyQuestions(t *testing.T) {
	t.Parallel()
	a := []byte("package A { part def P; part def Q :> Missing; }")
	b := []byte("package B { private import A::*; part p : P; }")
	loaded := model.NewWorkspace()
	loaded.OpenAll([]model.Input{{Name: "a.sysml", Content: a, Version: 1}, {Name: "b.sysml", Content: b, Version: 1}})
	loaded.DiagnosticsAll([]string{"a.sysml", "b.sysml"})
	rec, err := loaded.InterfaceRecord("a.sysml")
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Diagnostics) == 0 {
		t.Fatal("a.sysml stores no diagnostics: the fixture is vacuous")
	}
	ws := model.NewWorkspace()
	ws.OpenAll([]model.Input{{Name: "b.sysml", Content: b, Version: 1}})
	if err := ws.OpenRecorded(rec, []byte("package A { part def P; }")); !errors.Is(err, model.ErrRecordMismatch) {
		t.Fatalf("OpenRecorded with other content: got %v, want ErrRecordMismatch", err)
	}
	if err := ws.OpenRecorded(rec, a); err != nil {
		t.Fatal(err)
	}
	doc := ws.Document("a.sysml")
	if !bytes.Equal(doc.Content, a) || doc.Digest() != loaded.Document("a.sysml").Digest() {
		t.Fatalf("recorded a.sysml holds content %q, digest %q", doc.Content, doc.Digest())
	}
	content, diags, ok := ws.AnalyzedContent("a.sysml")
	if !ok || !bytes.Equal(content, a) || len(diags) != len(rec.Diagnostics) {
		t.Fatalf("AnalyzedContent = %q, %d diagnostics, %v", content, len(diags), ok)
	}
	span := diags[0].Span
	if pos := doc.Lines().PosAt(span.Offset); string(a[span.Offset:span.End()]) != "Missing" || pos.Line != 1 {
		t.Fatalf("the stored diagnostic locates %q at %v", a[span.Offset:span.End()], pos)
	}
	p := ws.LookupQualified("A::P")
	if len(p) != 1 {
		t.Fatalf("A::P: %d symbols", len(p))
	}
	var needs *symbols.NeedsHydration
	if _, err := ws.ReferencesTo(p[0]); !errors.Is(err, symbols.ErrNeedsHydration) || !errors.As(err, &needs) || needs.Doc != "a.sysml" {
		t.Fatalf("ReferencesTo over a recorded document: got %v, want a NeedsHydration for a.sysml", err)
	}
	if _, err := ws.NameReferencesTo(p[0], "P"); !errors.Is(err, symbols.ErrNeedsHydration) {
		t.Fatalf("NameReferencesTo over a recorded document: got %v", err)
	}
	if _, err := ws.RenameConflict(p[0], "P", "R"); !errors.Is(err, symbols.ErrNeedsHydration) {
		t.Fatalf("RenameConflict over a recorded document: got %v", err)
	}
}
