package model_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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
	connection def Link { end left : A; end right : B; }
	part def Host {
		part pa : A; part pb : B; part pc : C;
		connection base : Triple connect (pa, pb, pc);
		connection named : Triple connect (a references pa, b references pb, c references pc);
		connection link : Link connect (pa, pb);
	}
}
`),
			"use.sysml": []byte(`package Use {
	private import Conns::*;
	connection def Derived :> Triple;
	connection def Retyped :> Triple { end :>> a : A; end d : B; }
	connection def SubLink :> Link { end :>> left : A; }
	connection def Relinked :> Link { end :>> right : B; end :>> left : A; }
	part def Host2 :> Host { connection more :> base; connection alike :> named; connection relink :> link; }
	part h : Host {
		connection t : Triple connect (pa, pb, pc);
		connection u : Triple connect (pa, pb);
		connection v : Derived connect (pa, pb, pc);
		connection w : Triple connect (pc, pb, pa);
		connection x : Link connect (pa, pb);
		connection y : SubLink connect (left references pa, right references pb);
		connection z : Link connect (pb, pa);
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
		// A KerML end takes its multiplicity from a recorded general's end at
		// the same position: a range written on the end, a bound naming a
		// feature the declaring scope values, a `multiplicity` member, the
		// member it subsets and the end a `references` end attaches to; a
		// redefinition reads the named bound as unknown.
		"end multiplicity": {
			"a.sysml": []byte(`package A {
	part def T;
	attribute one : ScalarValues::Integer = 1;
	connection def L { end x : T[1]; end y : T[one]; end z : T[0..2]; }
	part def P { part p : T[one]; part q : T[1]; }
}
`),
			"k.kerml": []byte(`package K {
	class T;
	feature one : ScalarValues::Integer = 1;
	assoc L {
		end feature w : T[one];
		end feature x : T { multiplicity mx [1]; }
		end feature y : T { multiplicity my subsets K::L::x::mx; }
		end feature z : T { multiplicity mz [0..2]; }
		end feature v references w;
	}
}
`),
			"b.kerml": []byte(`package B {
	assoc M specializes A::L { end feature x : A::T; end feature y : A::T; end feature z : A::T; }
	assoc N specializes K::L { end feature w : K::T; end feature x : K::T; end feature y : K::T; end feature z : K::T; end feature v : K::T; }
}
`),
			"c.sysml": []byte(`package C {
	part def Q :> A::P { part :>> p : A::T[5]; part :>> q : A::T[5]; }
}
`),
		},
		// A classifier's own multiplicity gives an end typed by it its one.
		"classifier multiplicity": {
			"one.kerml": []byte(`package One {
	classifier One [1];
	classifier Many [0..*];
}
`),
			"use.kerml": []byte(`package Use {
	assoc R { end feature p : One::One; end feature q : One::Many; end feature r : One::Many; }
	struct S { end feature g : One::Many; end feature h : One::One; }
}
`),
		},
		// An alias in a cycle names an alias, an alias to nothing names nothing.
		"alias cycle": {
			"p.sysml": []byte(`package P {
	alias A for B;
	alias B for A;
	alias C for Missing;
	part def D;
	alias E for D;
}
`),
			"q.sysml": []byte(`package Q {
	private import P::*;
	part a : A;
	part c : C;
	part e : E;
}
`),
		},
		// A usage the classification has no kind for is still a feature, and
		// is named by its notation where a reference to it is refused.
		"unclassified usage": {
			"acts.sysml": []byte(`package Acts {
	action def A { transition y; }
}
`),
			"use.sysml": []byte(`package Use {
	part w { satisfy Acts::A::y; }
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
	a := []byte("package A { part def P; part def Q :> Missing; part def R :> P; }")
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

	// The record itself is the caller's to change: nothing installed reads it,
	// and the workspace that wrote it keeps its own diagnostics.
	message := diags[0].Message
	for i := range rec.Diagnostics {
		rec.Diagnostics[i].Message = "changed"
	}
	for i := range rec.Scope.Symbols {
		for j := range rec.Scope.Symbols[i].Facts.Supers {
			rec.Scope.Symbols[i].Facts.Supers[j] = symbols.ElementRef{}
		}
	}
	if _, diags, _ := ws.AnalyzedContent("a.sysml"); diags[0].Message != message {
		t.Fatalf("the stored diagnostic reads %q after the caller's record changed", diags[0].Message)
	}
	if diags := loaded.Diagnostics("a.sysml"); diags[0].Message != message {
		t.Fatalf("the writing workspace's diagnostic reads %q after the caller's record changed", diags[0].Message)
	}
	if r := ws.LookupQualified("A::R"); len(r) != 1 || len(r[0].Facts.Supers) != 1 || r[0].Facts.Supers[0].FQN != "A::P" {
		t.Fatalf("A::R's recorded supertypes after the caller's record changed: %+v", r)
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

// A `@@` filter classifies an imported element by its reflective metaclass,
// which its symbol kind alone does not always tell: a KerML `struct` from a
// `datatype`, a transition from an action, an interface's end from a
// connection's, a specialization from a conjugation, a multiplicity with a
// range from one without, a metadata body's feature from an attribute.
// Recorded, each element must pass or fail the filter exactly as it does loaded.
func TestInterfaceRecordMetaclassFilters(t *testing.T) {
	t.Parallel()
	docs := map[string][]byte{
		"k.kerml": []byte(`package K {
			struct S;
			datatype D;
			class C;
			classifier B;
			metaclass Meta;
			specialization Sp subtype S :> B;
			conjugation Cj conjugate C ~ B;
			@ m : Meta;
			feature f : S;
			multiplicity mr [1..3];
			multiplicity mn :> mr;
			metaclass Body { feature level : ScalarValues::Integer; }
			metadata body : Body { level = 3; }
		}`),
		"u.kerml": []byte(`package U {
			private import K::*[@@KerML::Kernel::Structure];
			feature s : S;
			feature d : D;
			feature c : C;
		}
		package U2 {
			private import K::*[@@KerML::Core::Specialization];
			alias asp for Sp;
			alias acj for Cj;
		}
		package U3 {
			private import K::*[@@KerML::Core::Conjugation];
			alias asp for Sp;
			alias acj for Cj;
		}
		package U4 {
			private import K::*[@@KerML::Kernel::MetadataFeature];
			alias am for m;
			alias af for f;
		}
		package U5 {
			private import K::*[@@KerML::Kernel::MultiplicityRange];
			alias amr for mr;
			alias amn for mn;
			alias am for m;
		}
		package U6 {
			private import K::*[@@KerML::Core::Multiplicity];
			alias amr for mr;
			alias amn for mn;
			alias am for m;
		}
		package U7 {
			private import K::body::*[@@KerML::Core::Feature];
			alias al for level;
		}
		package U8 {
			private import K::body::*[@@SysML::Systems::AttributeUsage];
			alias al for level;
		}`),
		"s.sysml": []byte(`package S {
			part def W;
			port def P;
			part def Car {
				part w : W;
				port p : P;
				binding b bind w = w;
				connection c connect w to w;
				interface i connect a references p to z references p;
				state def SD { state a; state z; transition t first a then z; }
			}
			metadata def Meta { attribute level : ScalarValues::Integer; }
			metadata m : Meta { level = 3; }
		}`),
		"t.sysml": []byte(`package T {
			private import S::Car::*[@@SysML::Systems::BindingConnectorAsUsage];
			alias ab for b;
			alias ac for c;
		}
		package T2 {
			private import S::Car::SD::*[@@SysML::Systems::TransitionUsage];
			alias atr for t;
			alias aa for a;
		}
		package T3 {
			private import S::Car::i::*[@@SysML::Systems::PortUsage];
			alias aa for a;
		}
		package T4 {
			private import S::Car::c::*[@@SysML::Systems::PortUsage];
			alias aa for a;
		}
		package T5 {
			private import S::m::*[@@SysML::Systems::ReferenceUsage];
			alias al for level;
		}
		package T6 {
			private import S::m::*[@@SysML::Systems::AttributeUsage];
			alias al for level;
		}`),
	}
	// Loaded, the filters admit exactly the elements of the metaclass, which
	// the references to the others report; the fixture is void otherwise.
	loaded := model.NewWorkspace()
	names := []string{"k.kerml", "s.sysml", "t.sysml", "u.kerml"}
	for _, name := range names {
		loaded.OpenAll([]model.Input{{Name: name, Content: docs[name], Version: 1}})
	}
	want := map[string][]string{
		"k.kerml": nil,
		"s.sysml": nil,
		"u.kerml": {"D", "C", "Cj", "Sp", "f", "mn", "m", "m", "level"},
		"t.sysml": {"c", "a", "a", "level"},
	}
	for _, name := range names {
		var got []string
		for _, d := range loaded.Diagnostics(name) {
			ref, ok := strings.CutPrefix(d.Message, "unresolved reference: ")
			if !ok {
				t.Fatalf("%s: unexpected diagnostic %s", name, d.Message)
			}
			got = append(got, strings.Fields(ref)[0])
		}
		if !slices.Equal(got, want[name]) {
			t.Fatalf("%s loaded: unresolved %v, want %v", name, got, want[name])
		}
	}
	if recorded := recordDifferential(t, docs); recorded != len(docs) {
		t.Fatalf("%d of %d documents recorded", recorded, len(docs))
	}
}

// A recorded document is a closed file: it has no open buffer, and a change to
// the file on disk reindexes it from the changed text.
func TestInterfaceRecordIsAClosedFile(t *testing.T) {
	t.Parallel()
	content := []byte("package A { part def X; }\n")
	loaded := model.NewWorkspace()
	loaded.OpenAll([]model.Input{{Name: "a.sysml", Content: content, Version: 1}})
	rec, err := loaded.InterfaceRecord("a.sysml")
	if err != nil {
		t.Fatal(err)
	}
	ws := model.NewWorkspace()
	ws.Open("a.sysml", content, 1)
	if !ws.IsOpen("a.sysml") {
		t.Fatal("precondition: a.sysml is not open")
	}
	if err := ws.OpenRecorded(rec, content); err != nil {
		t.Fatal(err)
	}
	if ws.IsOpen("a.sysml") {
		t.Fatal("a recorded document has an open buffer")
	}
	if !ws.Document("a.sysml").Recorded() {
		t.Fatal("a.sysml is not recorded")
	}
	ws.SetOnDisk("a.sysml", []byte("package A { part def X :> Missing; }\n"))
	if ws.Document("a.sysml").Recorded() {
		t.Fatal("a.sysml stays recorded after its file changed")
	}
	if diags := ws.Diagnostics("a.sysml"); len(diags) == 0 {
		t.Fatal("the changed file reports nothing for its unresolved general")
	}
}

// A document query reads a recorded document through facts: the individuals
// a tree descends, and the sequence a metadata feature defaults to.
func TestInterfaceRecordDocumentQueriesMatchLoaded(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		recorded, name string
		content        []byte
		report         []byte
		wants          []string
	}{
		"individual tree": {
			recorded: "site.sysml",
			content: []byte(`package Site {
	part def Station { part pumps : Pump[*]; }
	part def Pump;
	part plant {
		individual part def Run1 :> Station { individual part :>> pumps : Run1Pump; }
		individual part def Run1Pump :> Pump { individual part seal : Run1Seal; }
		individual part def Run1Seal;
		individual part def Run2Pump :> Pump;
	}
}
`),
			report: []byte(`package Report {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	calc def Individuals :> Query {
		in root : Element;
		Project(
			source = Tree(source = WhereFeature(
				source = WhereType(source = Descendants(source = root), type = "PartDefinition"),
				'feature' = "isIndividual", operator = "=", value = "true")),
			properties = ("name"))
	}
	part def Runs :> Document {
		attribute redefines title = "Runs";
		part runs : Table {
			attribute redefines caption = "Runs";
			calc rows : Individuals { in root = Site::plant; }
		}
	}
}
`),
			name:  "Report::Runs",
			wants: []string{"Run1Pump", "Run1Seal", "Run2Pump"},
		},
		"metadata sequence default": {
			recorded: "meta.sysml",
			content: []byte(`package Meta {
	private import ScalarValues::*;
	metadata def Tagged { attribute tags : String[0..*] ordered = ("alpha", "beta"); }
}
`),
			report: []byte(`package Report {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import Meta::*;
	part items { #Tagged part def Marked; }
	calc def Tags :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root), type = "PartDefinition"),
			properties = ("name"),
			columns = (Column(name = "Tags", expression = Meta::Tagged::tags ?? "")))
	}
	part def Tagging :> Document {
		attribute redefines title = "Tags";
		part tags : Table {
			attribute redefines caption = "Tags";
			calc rows : Tags { in root = items; }
		}
	}
}
`),
			name:  "Report::Tagging",
			wants: []string{"alpha", "beta"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			inputs := []model.Input{
				{Name: tc.recorded, Content: tc.content, Version: 1},
				{Name: "report.sysml", Content: tc.report, Version: 1},
			}
			loaded := model.NewWorkspace()
			loaded.OpenAll(inputs)
			for _, in := range inputs {
				if diags := loaded.Diagnostics(in.Name); len(diags) != 0 {
					t.Fatalf("%s loaded: %v", in.Name, diags)
				}
			}
			want, err := loaded.RenderDocumentMarkdown(tc.name, docrender.MarkdownOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.wants {
				if !strings.Contains(want, w) {
					t.Fatalf("loaded render lacks %q:\n%s", w, want)
				}
			}
			rec, err := loaded.InterfaceRecord(tc.recorded)
			if err != nil {
				t.Fatal(err)
			}
			ws := model.NewWorkspace()
			ws.OpenAll(inputs[1:])
			if err := ws.OpenRecorded(rec, tc.content); err != nil {
				t.Fatal(err)
			}
			if diags := ws.Diagnostics("report.sysml"); len(diags) != 0 {
				t.Fatalf("report.sysml over the recorded %s: %v", tc.recorded, diags)
			}
			got, err := ws.RenderDocumentMarkdown(tc.name, docrender.MarkdownOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("render over the recorded %s differs from loaded:\n%s\n--- loaded ---\n%s", tc.recorded, got, want)
			}
		})
	}
}

// TestInterfaceRecordAnnotationSpans checks that an identity annotation a
// recorded document states about a loaded element keeps the span of the node
// stating it, as the identity table of the loaded workspace reports.
func TestInterfaceRecordAnnotationSpans(t *testing.T) {
	t.Parallel()
	goods := []byte(`package Goods {
    @IdentityMetadata::ProjectRef { projectId = "proj"; }
    part def A;
}
`)
	tags := []byte(`package Tags {
    metadata aid : IdentityMetadata::ElementId about Goods::A { id = "a-id"; }
}
`)
	inputs := []model.Input{
		{Name: "goods.sysml", Content: goods, Version: 1},
		{Name: "tags.sysml", Content: tags, Version: 1},
	}
	type declaration struct {
		about bool
		span  source.Span
	}
	declarationsOf := func(ws *model.Workspace) []declaration {
		syms := ws.LookupQualified("Goods::A")
		if len(syms) != 1 {
			t.Fatalf("%d symbols named Goods::A", len(syms))
		}
		info, ok := ws.IdentityOf("goods.sysml", syms[0])
		if !ok {
			t.Fatal("no identity for Goods::A")
		}
		var out []declaration
		for _, d := range info.Declarations {
			out = append(out, declaration{about: d.About, span: d.Span})
		}
		return out
	}

	loaded := model.NewWorkspace()
	loaded.OpenAll(inputs)
	want := declarationsOf(loaded)
	if len(want) != 1 || !want[0].about || want[0].span.Len == 0 {
		t.Fatalf("loaded declarations of Goods::A: %+v; want one about-declaration with a span", want)
	}

	rec, err := loaded.InterfaceRecord("tags.sysml")
	if err != nil {
		t.Fatal(err)
	}
	ws := model.NewWorkspace()
	ws.OpenAll(inputs)
	if err := ws.OpenRecorded(rec, tags); err != nil {
		t.Fatal(err)
	}
	if got := declarationsOf(ws); !slices.Equal(got, want) {
		t.Fatalf("declarations of Goods::A with tags.sysml recorded: %+v; loaded: %+v", got, want)
	}
}
