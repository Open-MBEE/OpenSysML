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
		// A call names a recorded calculation: its arity and result type are
		// its parameters, owned and inherited.
		"behavior parameters": {
			"calcs.sysml": []byte(`package Calcs {
	calc def Add { in x : ScalarValues::Integer; in y : ScalarValues::Integer; return : ScalarValues::Integer = x + y; }
	calc def Add3 :> Add { in z : ScalarValues::Integer; }
}
`),
			"use.sysml": []byte(`package Use {
	attribute a : ScalarValues::Integer = Calcs::Add(1, 2, 3);
	attribute b : ScalarValues::Integer = Calcs::Add(1, 2);
	attribute c : ScalarValues::String = Calcs::Add(1, 2);
	attribute d : ScalarValues::Integer = Calcs::Add3(1, 2, 3);
	attribute e : ScalarValues::Integer = Calcs::Add3(1, 2);
}
`),
		},
		// An annotation of a recorded metadata definition inherits the
		// defaults its features declare, which a filter reads.
		"metadata defaults": {
			"meta.sysml": []byte(`package Meta {
	metadata def Flagged { attribute flag : ScalarValues::Boolean = true; }
}
`),
			"use.sysml": []byte(`package Use {
	private import Meta::*;
	package Items { part def Plain; #Flagged part def Marked; }
	package Picked { public import Items::*[@Flagged and Flagged::flag]; }
	part m : Picked::Marked;
	part p : Picked::Plain;
}
`),
		},
		// A perform that borrows the name of an action it does not resolve binds
		// none, so a specialization declaring that name inherits no duplicate.
		"borrowed name": {
			"acts.sysml": []byte(`package Acts {
	action def Flow { perform nope; }
	part def Base { part x; }
	part def Derived :> Base { part :>> x; part :>> gone; }
}
`),
			"use.sysml": []byte(`package Use {
	action def D :> Acts::Flow { action nope; }
	part def E :> Acts::Derived { part x; part gone; }
	part d : Acts::Derived { part q :>> x; part r :>> gone; }
}
`),
		},
		// A connector specializing a recorded nonbinary connector inherits its
		// ends by position: their count, order and types.
		"connector ends": {
			"conns.sysml": []byte(`package Conns {
	part def A; part def B; part def C;
	connection def Triple { end a : A; end b : B; end c : C; }
	part def Host {
		part pa : A; part pb : B; part pc : C;
		connection base : Triple connect (pa, pb, pc);
		connection named : Triple connect (a references pa, b references pb, c references pc);
	}
}
`),
			"use.sysml": []byte(`package Use {
	private import Conns::*;
	connection def Derived :> Triple;
	connection def Retyped :> Triple { end :>> a : A; end d : B; }
	part def Host2 :> Host { connection more :> base; connection alike :> named; }
	part h : Host {
		connection t : Triple connect (pa, pb, pc);
		connection u : Triple connect (pa, pb);
		connection v : Derived connect (pa, pb, pc);
		connection w : Triple connect (pc, pb, pa);
	}
}
`),
		},
		// A union naming itself among its operands is the union of the rest;
		// what conforms to a recorded union is what conforms to those.
		"composed operands": {
			"k.kerml": []byte(`package K {
	class Base;
	class A specializes Base;
	class U unions U, A, A;
	class I intersects A, I, A;
}
`),
			"q.kerml": []byte(`package Q {
	class H { feature b : K::Base; feature c : K::Base; feature d : K::Base; }
	class G specializes H { feature :>> b : K::U; feature :>> c : K::I; feature :>> d : K::A; }
}
`),
		},
		// An about annotation stated in a recorded document annotates elements
		// of another, which a third document's filters read with its values;
		// the value a recorded metadata feature fixes with ` = ` stays fixed.
		"about annotation": {
			"meta.sysml": []byte(`package Meta {
	metadata def Flag { attribute on : ScalarValues::Boolean = true; }
}
`),
			"goods.sysml": []byte(`package Goods { part def Plain; part def Marked; part def Off; }
`),
			"tags.sysml": []byte(`package Tags {
	private import Meta::*;
	private import Goods::*;
	metadata f : Flag about Marked;
	metadata g : Flag about Off { on = false; }
}
`),
			"use.sysml": []byte(`package Use {
	private import Meta::*;
	package Picked { public import Goods::*[@Flag]; }
	package On { public import Goods::*[@Flag and Flag::on]; }
	part m : Picked::Marked;
	part p : Picked::Plain;
	part o : Picked::Off;
	part m2 : On::Marked;
	part o2 : On::Off;
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

// A workspace owns the content of a recorded document: the bytes the caller
// passed may be reused, and what the document holds and locates is unchanged.
func TestInterfaceRecordOwnsItsContent(t *testing.T) {
	t.Parallel()
	a := []byte("package A { part def P; part def Q :> Missing; }")
	loaded := model.NewWorkspace()
	loaded.OpenAll([]model.Input{{Name: "a.sysml", Content: a, Version: 1}})
	loaded.DiagnosticsAll([]string{"a.sysml"})
	rec, err := loaded.InterfaceRecord("a.sysml")
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Diagnostics) == 0 {
		t.Fatal("a.sysml stores no diagnostics: the fixture is vacuous")
	}
	ws := model.NewWorkspace()
	buf := bytes.Clone(a)
	if err := ws.OpenRecorded(rec, buf); err != nil {
		t.Fatal(err)
	}
	for i := range buf {
		buf[i] = 'x'
	}
	doc := ws.Document("a.sysml")
	if !bytes.Equal(doc.Content, a) || doc.Digest() != rec.Digest {
		t.Fatalf("recorded a.sysml holds content %q, digest %q after the caller's buffer changed", doc.Content, doc.Digest())
	}
	content, diags, ok := ws.AnalyzedContent("a.sysml")
	if !ok || !bytes.Equal(content, a) || len(diags) != len(rec.Diagnostics) {
		t.Fatalf("AnalyzedContent = %q, %d diagnostics, %v", content, len(diags), ok)
	}
	span := diags[0].Span
	if pos := doc.Lines().PosAt(span.Offset); string(content[span.Offset:span.End()]) != "Missing" || pos.Line != 1 {
		t.Fatalf("the stored diagnostic locates %q at %v", content[span.Offset:span.End()], pos)
	}
	if p := ws.LookupQualified("A::P"); len(p) != 1 || p[0].DocName != "a.sysml" {
		t.Fatalf("A::P: %v", p)
	}
}

// A record answers the conformance question it was written under: it installs
// only into a workspace asking the same, and a workspace holding one keeps its
// mode until the document is hydrated.
func TestInterfaceRecordConformanceMode(t *testing.T) {
	t.Parallel()
	a := []byte("package A { part def P; part def Q :> Missing; }")
	loaded := model.NewWorkspace()
	loaded.OpenAll([]model.Input{{Name: "a.sysml", Content: a, Version: 1}})
	loaded.DiagnosticsAll([]string{"a.sysml"})
	rec, err := loaded.InterfaceRecord("a.sysml")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Mode != diag.ConformanceDefault {
		t.Fatalf("record mode %s, want %s", rec.Mode, diag.ConformanceDefault)
	}
	strict := model.NewWorkspace(model.WithConformanceMode(diag.ConformanceStrict))
	if err := strict.OpenRecorded(rec, a); !errors.Is(err, model.ErrRecordMismatch) {
		t.Fatalf("OpenRecorded into a strict workspace: got %v, want ErrRecordMismatch", err)
	}
	if strict.Document("a.sysml") != nil {
		t.Fatal("a refused record was installed")
	}

	ws := model.NewWorkspace()
	if err := ws.SetConformanceMode(diag.ConformanceStrict); err != nil {
		t.Fatalf("SetConformanceMode over no recorded document: %v", err)
	}
	if err := ws.SetConformanceMode(diag.ConformanceDefault); err != nil {
		t.Fatal(err)
	}
	if err := ws.OpenRecorded(rec, a); err != nil {
		t.Fatal(err)
	}
	var needs *symbols.NeedsHydration
	err = ws.SetConformanceMode(diag.ConformanceStrict)
	if !errors.Is(err, symbols.ErrNeedsHydration) || !errors.As(err, &needs) || needs.Doc != "a.sysml" {
		t.Fatalf("SetConformanceMode over a recorded document: got %v, want a NeedsHydration for a.sysml", err)
	}
	if ws.ConformanceMode() != diag.ConformanceDefault {
		t.Fatalf("the mode moved to %s under a recorded document", ws.ConformanceMode())
	}
	if diags := ws.Diagnostics("a.sysml"); len(diags) != len(rec.Diagnostics) {
		t.Fatalf("a.sysml reports %d diagnostics, its record stores %d", len(diags), len(rec.Diagnostics))
	}
	if err := ws.SetConformanceMode(diag.ConformanceDefault); err != nil {
		t.Fatalf("SetConformanceMode to the mode held: %v", err)
	}
}
