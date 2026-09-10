package export_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

var update = flag.Bool("update", false, "rewrite the .golden.ttl and .golden.sysml files")

// TestGoldenConversions locks each model's Turtle and the notation it converts
// back to: as written with its source text, `.canonical.golden.sysml` without.
func TestGoldenConversions(t *testing.T) {
	for _, path := range modelFiles(t) {
		name, ext := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			turtle, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert(name+".ttl", turtle, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v\n%s", err, turtle)
			}
			if want := string(src); string(back) != want {
				t.Errorf("the model did not come back as written:\n--- want ---\n%s--- got ---\n%s", want, back)
			}
			canonical, err := export.Convert(name+".ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				canonical = []byte("sysml: " + err.Error() + "\n")
			}
			stem := strings.TrimSuffix(path, ext)
			checkGolden(t, stem+".golden.ttl", turtle)
			checkGolden(t, stem+".golden"+ext, back)
			checkGolden(t, stem+".canonical.golden"+ext, canonical)
		})
	}
}

// TestConvertedNotationParses checks that the notation written from a graph is
// valid SysML: it must parse without a single syntax error.
func TestConvertedNotationParses(t *testing.T) {
	for _, path := range modelFiles(t) {
		name, ext := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			turtle, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert(name+".ttl", turtle, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			p := parser.New(source.New(name+".converted"+ext, back))
			p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Errorf("converted notation does not parse: %v\n%s", p.Diagnostics, back)
			}
		})
	}
}

// textOnlyFixtures are the models whose graph the mapping cannot write back
// without its source text, by the refusal it must keep reporting for them.
var textOnlyFixtures = map[string]string{
	"action_nodes":               "this expression states no notation and no structure",
	"expression_body_members":    "a declaration inside an expression body is carried as its notation",
	"expression_body_order":      "a declaration inside an expression body is carried as its notation",
	"payload_declaration_bodies": "it has no sysx:endForm",
}

// TestRoundTripIsLossless is the fidelity contract: converting the notation a
// graph produced back to a graph gives the same graph. Notation and RDF say the
// same thing in different words, so this is what "no data lost" means — the
// notation itself may legitimately be spelled differently (a name written
// relative to its scope, a keyword written in place of its symbol).
func TestRoundTripIsLossless(t *testing.T) {
	for _, path := range modelFiles(t) {
		name, ext := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			first, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert(name+".ttl", first, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			second, err := export.Convert(name+ext, back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if string(first) != string(second) {
				t.Errorf("round trip changed the graph\n--- first ---\n%s\n--- second ---\n%s", first, second)
			}
			if textOnly, ok := textOnlyFixtures[name]; ok {
				_, err := export.Convert(name+".ttl", withoutTriples(t, first, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
				var unsupported *export.UnsupportedError
				if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), textOnly) {
					t.Fatalf("the mapping alone should still refuse %s (%s), got: %v", name, textOnly, err)
				}
				return
			}
			structuralRoundTrip(t, name+ext, first)
		})
	}
}

// structuralRoundTrip strips the source text from a graph and requires the
// structure alone to carry it to notation and back; it returns that notation.
// name may carry the notation's extension; a bare name reads as SysML.
func structuralRoundTrip(t *testing.T, name string, first []byte) []byte {
	t.Helper()
	ext := filepath.Ext(name)
	if ext == "" {
		ext = ".sysml"
	}
	name = strings.TrimSuffix(name, ext)
	fromGraph, err := export.Convert(name+".ttl", withoutTriples(t, first, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v", err)
	}
	again, err := export.Convert(name+ext, fromGraph, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again from the mapping alone: %v", err)
	}
	requireSameGraphBytes(t, fromGraph, first, again)
	return fromGraph
}

// requireSameGraphBytes requires two hops' Turtle, source text stripped, to be
// the same triple set and then the same bytes; notation is the second hop's input.
func requireSameGraphBytes(t *testing.T, notation, first, second []byte) {
	t.Helper()
	first, second = withoutSourceText(t, first), withoutSourceText(t, second)
	if lost, gained := tripleSetDiff(t, first, second); len(lost)+len(gained) > 0 {
		t.Errorf("the mapping alone changed the graph\n--- notation ---\n%s\n--- lost ---\n%s\n--- gained ---\n%s",
			notation, strings.Join(lost, "\n"), strings.Join(gained, "\n"))
		return
	}
	if !bytes.Equal(first, second) {
		t.Errorf("the same graph was written in a different order\n--- notation ---\n%s\n--- first difference ---\n%s",
			notation, firstLineDifference(first, second))
	}
}

// firstLineDifference reports the first line two documents disagree on.
func firstLineDifference(first, second []byte) string {
	a, b := strings.Split(string(first), "\n"), strings.Split(string(second), "\n")
	for i := 0; i < len(a) || i < len(b); i++ {
		var left, right string
		if i < len(a) {
			left = a[i]
		}
		if i < len(b) {
			right = b[i]
		}
		if left != right {
			return fmt.Sprintf("line %d:\n- %s\n+ %s", i+1, left, right)
		}
	}
	return ""
}

// withoutSourceText strips the triples that carry notation rather than structure.
func withoutSourceText(t *testing.T, turtle []byte) []byte {
	t.Helper()
	for _, property := range []string{"sysx:sourceText", "sysx:sourceTail", "sysx:sourceLanguage"} {
		turtle = withoutTriples(t, turtle, property)
	}
	return turtle
}

// TestWrittenReferencesResolveWhereWritten pins the spelling rule: short when
// the short name resolves to the graph's element there, else qualified enough.
func TestWrittenReferencesResolveWhereWritten(t *testing.T) {
	path := filepath.Join("testdata", "convert", "shadowed_references.sysml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{
		// The graph names the package-level targets, not the shadowing members.
		"sysml:redefines elmt:Shadowing__packet_20data_20field",
		"sysml:redefines elmt:Shadowing__Packet__data_20field",
		"sysml:subsets elmt:Shadowing__payload",
		"sysml:references elmt:Shadowing__Bus__payload",
		"sysml:type elmt:Shadowing__Packet",
		"sysml:type elmt:Shadowing__Bus__Packet",
		"sysml:type elmt:Shadowing__Frame",
		"sysml:type elmt:Shadowing__Frame__Frame",
		"sysml:type elmt:Shadowing__Field",
		"sysml:targetFeature elmt:Shadowing__Packet__payload",
		"sysml:referent elmt:Shadowing__payload",
		"sysml:importedNamespace elmt:Shadowing__Lib__Cell",
		"sysml:importedNamespace elmt:Shadowing__Lib",
		"sysml:type elmt:Shadowing__Lib__Cell",
		"sysml:type elmt:Shadowing__Consumer__Lib__Cell",
	} {
		if !strings.Contains(string(graph), want) {
			t.Errorf("graph does not record %q\n%s", want, graph)
		}
	}
	notation := structuralRoundTrip(t, "shadowed_references", graph)
	for _, want := range []string{
		// A redefinition target is looked up in the owner's generals, past the
		// redefining feature and its members, so the short name reaches it.
		"attribute 'packet data field' redefines 'packet data field';",
		"attribute 'data field' redefines 'data field' {",
		// The subsetting feature itself, or a sibling, captures the short name;
		// a reference to that sibling is short because it is the target.
		"part payload subsets Shadowing::payload;",
		"part cargo subsets Shadowing::payload;",
		"ref part carried references payload;",
		// A nested definition captures the outer one's name; the nested one
		// is what the short name reaches.
		"part wrapped : Shadowing::Packet;",
		"part raw : Packet;",
		"ref part outer : Shadowing::Frame;",
		"ref part inner : Frame;",
		"attribute 'packet data field' : Shadowing::Field;",
		// Nothing shadows Field inside Packet, so it stays short.
		"attribute 'data field' : Field;",
		"connection link connect wrapped.payload to Shadowing::payload;",
		// A nested package captures the imported one's name; the import itself
		// does not surface its target as a spelling of its target.
		"private import Shadowing::Lib::Cell;",
		"private import Shadowing::Lib::*;",
		"part cell : Cell;",
		"part inner : Lib::Cell;",
	} {
		if !strings.Contains(string(notation), want) {
			t.Errorf("notation does not write %q\n%s", want, notation)
		}
	}
}

// TestGlobalSpellingIsReachedPastAShadowedRoot covers a target whose short
// name and whose qualified name are both captured where it is written: only the
// global form reaches it, so that is what the graph alone must write.
func TestGlobalSpellingIsReachedPastAShadowedRoot(t *testing.T) {
	src := "package Root {\n    part def Target;\n    part def Holder {\n        part def Root {\n            part def Target;\n        }\n" +
		"        part def Target;\n        part t : $::Root::Target;\n    }\n}\n"
	graph, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if want := "sysml:type elmt:Root__Target ;"; !strings.Contains(string(graph), want) {
		t.Fatalf("graph does not record %q\n%s", want, graph)
	}
	notation := structuralRoundTrip(t, "shadowed_root", graph)
	if want := "part t : $::Root::Target;"; !strings.Contains(string(notation), want) {
		t.Errorf("notation does not write %q\n%s", want, notation)
	}
}

// TestCastTypeIsSpelledForItsScope covers the type of a cast, a reference the
// expression tree carries: written where a nearer definition bears its name, the
// graph alone must keep the qualified spelling, and relinked to that nearer
// definition it must write the short one.
func TestCastTypeIsSpelledForItsScope(t *testing.T) {
	src := "package P {\n    part def T;\n    part def H {\n        part def T;\n        attribute v = (as P::T);\n    }\n}\n"
	graph, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if want := "sysx:typeArgument elmt:P__T ."; !strings.Contains(string(graph), want) {
		t.Fatalf("graph does not record %q\n%s", want, graph)
	}
	notation := structuralRoundTrip(t, "cast_type", graph)
	if want := "attribute v = (as P::T);"; !strings.Contains(string(notation), want) {
		t.Errorf("notation does not write %q\n%s", want, notation)
	}
	structural := withoutTriples(t, graph, "sysx:sourceText")
	relinkedGraph := relinked(t, structural, "sysx:typeArgument elmt:P__T .", "sysx:typeArgument elmt:P__H__T .")
	back, err := export.Convert("cast_type.ttl", relinkedGraph, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v\n%s", err, relinkedGraph)
	}
	if want := "attribute v = (as T);"; !strings.Contains(string(back), want) {
		t.Errorf("notation does not write %q\n%s", want, back)
	}
}

// A named multiplicity's body is a namespace like any other: the references
// its members make are links, spelled from that body when written back.
func TestMultiplicityBodyMembersLinkTheirReferences(t *testing.T) {
	src := "package P {\n    datatype T;\n    feature base : T;\n    multiplicity m [1..2] {\n        feature f : T;\n        feature g subsets base;\n    }\n" +
		"    package Q {\n        datatype T;\n        multiplicity n [0..1] {\n            feature h : P::T;\n        }\n    }\n}\n"
	graph, err := export.Convert("m.kerml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{"sysml:type elmt:P__T", "sysml:subsets elmt:P__base"} {
		if !strings.Contains(string(graph), want) {
			t.Errorf("graph does not record %q\n%s", want, graph)
		}
	}
	for _, text := range []string{`sysml:type "`, `sysml:subsets "`} {
		if strings.Contains(string(graph), text) {
			t.Errorf("a body member's reference is carried as text %s\n%s", text, graph)
		}
	}
	notation := structuralRoundTrip(t, "multiplicity_body", graph)
	for _, want := range []string{"feature f : T;", "feature g subsets base;", "feature h : P::T;"} {
		if !strings.Contains(string(notation), want) {
			t.Errorf("notation does not write %q\n%s", want, notation)
		}
	}
}

// TestPacketsRoundTripsStructurally is the corpus case the spelling rule was
// found on: a redefining attribute that bears its target's own name.
func TestPacketsRoundTripsStructurally(t *testing.T) {
	path := filepath.Join(corpusRoundTripExamples, "pilot-corpora", "sysml-examples", "Packet Example", "Packets.sysml")
	src, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		for _, root := range corpusRoundTripRoots {
			if root.name == "pilot-corpora/sysml-examples" {
				root.skip(t, path+" is missing")
			}
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	graph, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	// Two definitions redefine the package's 'packet data field', one nested
	// attribute its 'user data field'; none may be its own target.
	for want, n := range map[string]int{
		"sysml:redefines elmt:Packets__packet_20data_20field ;":                      2,
		"sysml:redefines elmt:Packets__packet_20data_20field__user_20data_20field ;": 1,
	} {
		if got := strings.Count(string(graph), want); got != n {
			t.Errorf("graph records %q %d times, want %d\n%s", want, got, n, graph)
		}
	}
	notation := structuralRoundTrip(t, "Packets", graph)
	// Written short, 'packet data field' would reach the redefinition
	// inherited from 'Data Packet' instead of the package's attribute.
	if want := "attribute 'packet data field' redefines Packets::'packet data field' {"; !strings.Contains(string(notation), want) {
		t.Errorf("notation does not write %q\n%s", want, notation)
	}
}

// TestTransitionEffectSuccessionLinksItsEnds covers a succession inside the
// action a transition performs: its ends are members of that action, so the
// graph links them and the notation written back names the same members.
func TestTransitionEffectSuccessionLinksItsEnds(t *testing.T) {
	src := `package P {
    item def Order;
    port def Inbox { in item o : Order; }
    part sys {
        port inbox : Inbox;
        state B {
            state Waiting;
            accept o : Order via inbox do action {
                first start;
                then action fulfil { in what = o; }
                then send o via inbox;
            } then Waiting;
        }
    }
}`
	graph, err := export.Convert("effect.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if want := "sysml:targetFeature elmt:P__sys__B___401___400__fulfil"; !strings.Contains(string(graph), want) {
		t.Errorf("graph does not link the succession's target %q\n%s", want, graph)
	}
	if want := "sysml:sourceFeature elmt:P__sys__B___401___400__fulfil"; !strings.Contains(string(graph), want) {
		t.Errorf("graph does not link the next succession's source %q\n%s", want, graph)
	}
	notation := structuralRoundTrip(t, "effect", graph)
	if want := "then action fulfil {"; !strings.Contains(string(notation), want) {
		t.Errorf("notation does not write %q\n%s", want, notation)
	}
}

// relinked is a graph with one link pointed at another element, the shape of a
// graph another tool wrote or an edit made; the link must occur exactly once.
func relinked(t *testing.T, graph []byte, link, to string) []byte {
	t.Helper()
	if n := strings.Count(string(graph), link); n != 1 {
		t.Fatalf("graph records %q %d times, want once\n%s", link, n, graph)
	}
	return []byte(strings.Replace(string(graph), link, to, 1))
}

// refusedAsUnsupported requires a graph to be refused rather than written as
// notation that would read back as a different graph.
func refusedAsUnsupported(t *testing.T, name string, graph []byte, why string) {
	t.Helper()
	out, err := export.Convert(name+".ttl", graph, export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError, got %v; notation:\n%s", err, out)
	}
	if !strings.Contains(err.Error(), why) {
		t.Errorf("error %q should say %q", err, why)
	}
}

// TestChainReachingAUsageNamedByAChainWritesItsEffectiveName covers a chain
// segment the graph links to an unnamed usage whose name comes from the chain
// it performs: the segment is written as that chain's last member, which is the
// name the usage answers to (KerML 7.3.4.5), not the chain text.
func TestChainReachingAUsageNamedByAChainWritesItsEffectiveName(t *testing.T) {
	src := `package P {
    action provide { action generate; }
    part generator { perform provide.generate; }
    part train { part engine { perform provide.generate; } }
    allocate generator.generate to train.engine.generate;
}`
	graph, err := export.Convert("performed.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{"sysml:targetFeature elmt:P__generator___400", "sysml:targetFeature elmt:P__train__engine___400"} {
		if !strings.Contains(string(graph), want) {
			t.Errorf("graph does not link %q\n%s", want, graph)
		}
	}
	notation := structuralRoundTrip(t, "performed", graph)
	if want := "allocate generator.generate to train.engine.generate;"; !strings.Contains(string(notation), want) {
		t.Errorf("notation does not write %q\n%s", want, notation)
	}
}

// TestSpellingsAreCheckedBesideEachOther covers an import target whose short
// name reaches its element only while the sibling imports it reads through are
// fully qualified: `P211` reaches through `Pkg211::*::**` beside
// `Pkg2::Pkg21::*` only when the resolver can bind their prefixes, so the
// spelling chosen must reach its element in the notation actually written.
func TestSpellingsAreCheckedBesideEachOther(t *testing.T) {
	src := `package ImportTest {
    package Pkg1 {
        private import Pkg2::Pkg21::Pkg211::P211;
        private import Pkg2::Pkg21::*;
        private import Pkg211::*::**;
        part p11 : Pkg211::P211;
        part def P12;
    }
    package Pkg2 {
        private import Pkg1::*;
        package Pkg21 { package Pkg211 { part def P211 :> P12; } }
    }
}`
	graph, err := export.Convert("imports.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if want := "sysml:importedNamespace elmt:ImportTest__Pkg2__Pkg21__Pkg211__P211"; !strings.Contains(string(graph), want) {
		t.Fatalf("graph does not link %q\n%s", want, graph)
	}
	notation := structuralRoundTrip(t, "imports", graph)
	if want := "private import Pkg211::P211;"; !strings.Contains(string(notation), want) {
		t.Errorf("notation does not write %q\n%s", want, notation)
	}
}

// TestChainSegmentIsSpelledToReachTheGraphsTarget covers a chain whose target
// the graph links to a same-named feature of another type than its operand's:
// the segment is written qualified, since its name alone would read as the
// operand's own feature, and a target no segment spelling reaches is refused.
func TestChainSegmentIsSpelledToReachTheGraphsTarget(t *testing.T) {
	src := `package P {
    part def A { attribute x; }
    part def B { attribute x; }
    part a : A;
    part b : B;
    attribute v = a.x;
    attribute w = a.x + a.x;
    connect a.x to b.x;
}`
	graph, err := export.Convert("chain.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{"sysml:targetFeature elmt:P__A__x", "sysml:targetFeature elmt:P__B__x"} {
		if !strings.Contains(string(graph), want) {
			t.Errorf("graph does not link %q\n%s", want, graph)
		}
	}
	structuralRoundTrip(t, "chain", graph)
	structural := withoutTriples(t, graph, "sysx:sourceText")
	// The value's chain `a.x` relinked to B::x is written `a.B::x`, as is the
	// connector's first end, whether or not the notation is kept with the graph.
	value := "    sysml:argument expr:P__v_pvalue_pa0 ;\n    sysml:targetFeature elmt:P__A__x ."
	relinkedValue := relinked(t, structural, value, strings.Replace(value, "A__x", "B__x", 1))
	if back := backFromTheGraphAlone(t, string(relinkedValue)); !strings.Contains(back, "attribute v = a.B::x;") {
		t.Errorf("the relinked value should be written qualified\n%s", back)
	}
	end := "    sysml:argument expr:P___406_pend0_pa0 ;\n    sysml:targetFeature elmt:P__A__x ;"
	for name, g := range map[string][]byte{"structure": structural, "notation": graph} {
		back, err := export.Convert("chain-"+name+".ttl", relinked(t, g, end, strings.Replace(end, "A__x", "B__x", 1)), export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("back to notation (%s): %v", name, err)
		}
		if !strings.Contains(string(back), "connect a.B::x to b.x;") {
			t.Errorf("the relinked end should be written qualified (%s)\n%s", name, back)
		}
	}
	// A target no chain from a can name — the package itself — is refused.
	refusedAsUnsupported(t, "chain-package", relinked(t, structural, value, strings.Replace(value, "P__A__x", "P", 1)),
		"no spelling of the segment reads as the element the graph names from the operand")
	// The sum repeats `a.x`: the occurrence still reaching A::x must not vouch
	// for the one relinked to B::x, which is spelled to reach its own element.
	second := "    sysml:argument expr:P__w_pvalue_pa1_pa0 ;\n    sysml:targetFeature elmt:P__A__x ;"
	repeated := relinked(t, structural, second, strings.Replace(second, "A__x", "B__x", 1))
	if back := backFromTheGraphAlone(t, string(repeated)); !strings.Contains(back, "attribute w = a.x + a.B::x;") {
		t.Errorf("each repeated chain should be spelled for its own element\n%s", back)
	}
	// Both relinked, neither occurrence reads as A::x any more.
	first := "    sysml:argument expr:P__w_pvalue_pa0_pa0 ;\n    sysml:targetFeature elmt:P__A__x ;"
	both := relinked(t, repeated, first, strings.Replace(first, "A__x", "B__x", 1))
	if back := backFromTheGraphAlone(t, string(both)); !strings.Contains(back, "attribute w = a.B::x + a.B::x;") {
		t.Errorf("both relinked chains should be spelled qualified\n%s", back)
	}
}

// TestSharedChainIsCheckedInEveryDeclaration covers one FeatureChainExpression
// two declarations share: written in each, its segment must reach the graph's
// element from each, not only from the owner met last.
func TestSharedChainIsCheckedInEveryDeclaration(t *testing.T) {
	src := `package P {
    part def A { attribute x; }
    part def B { attribute x; }
    part a : A;
    attribute v = a.x;
    part def H {
        part a : B;
        attribute w = a.x;
    }
}`
	graph, err := export.Convert("shared.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural := withoutTriples(t, graph, "sysx:sourceText")
	// w's value is v's chain: its root, linked to P::a, is spelled to reach it
	// from both declarations, so both state A::x.
	shared := relinked(t, structural, "sysml:value expr:P__H__w_pvalue ;", "sysml:value expr:P__v_pvalue ;")
	back, err := export.Convert("shared.ttl", shared, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v\n%s", err, shared)
	}
	for _, want := range []string{"attribute v = a.x;", "attribute w = P::a.x;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("notation does not write %q\n%s", want, back)
		}
	}
	// With the root a name as written, `a.x` reads A::x from v but B::x from w:
	// the declaration that reads it otherwise qualifies the segment, whichever
	// owner is met last.
	byName := relinked(t, shared, "sysml:referent elmt:P__a ;", `sysml:referent "a" ;`)
	if back := toNotation(t, byName); !strings.Contains(back, "attribute w = a.A::x;") {
		t.Errorf("w should reach A::x through a qualified segment\n%s", back)
	}
	// The same with w's chain shared into v, so the qualifying owner is the other one.
	shared = relinked(t, structural, "sysml:value expr:P__v_pvalue ;", "sysml:value expr:P__H__w_pvalue ;")
	byName = relinked(t, shared, "sysml:referent elmt:P__H__a ;", `sysml:referent "a" ;`)
	if back := toNotation(t, byName); !strings.Contains(back, "attribute v = a.B::x;") {
		t.Errorf("v should reach B::x through a qualified segment\n%s", back)
	}
}

// TestInitialStartMustBeAMemberOfItsBody covers a `first` whose start the graph
// links to an action outside the body: written by name, it would read as a
// label or as a member of the same name.
func TestInitialStartMustBeAMemberOfItsBody(t *testing.T) {
	src := `package P {
    action def Outer {
        action s1;
        action inner {
            action s2;
            first s2;
        }
    }
}`
	graph, err := export.Convert("initial.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structuralRoundTrip(t, "initial", graph)
	structural := withoutTriples(t, graph, "sysx:sourceText")
	const link = "sysml:sourceFeature elmt:P__Outer__inner__s2"
	refusedAsUnsupported(t, "initial", relinked(t, structural, link, "sysml:sourceFeature elmt:P__Outer__s1"),
		"`first s1` does not name P::Outer::s1 in the body it is written in")
	// A same-named sibling of the body would read as the start instead.
	shadowed := strings.Replace(src, "action s1;", "action s2;", 1)
	graph, err = export.Convert("initial.sysml", []byte(shadowed), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural = withoutTriples(t, graph, "sysx:sourceText")
	refusedAsUnsupported(t, "initial", relinked(t, structural, link, "sysml:sourceFeature elmt:P__Outer__s2"),
		"`first s2` does not name P::Outer::s2 in the body it is written in")
}

// TestFixturesComeBackFromTheGraphAlone strips sysx:sourceText before writing back,
// so the structural triples alone must carry each head and survive a second hop.
func TestFixturesComeBackFromTheGraphAlone(t *testing.T) {
	fixtures := []string{
		"ref_subsets.sysml",
		"composite_multiplicity.kerml",
		"end_prefix_metadata.sysml",
		"nested_namespace_import.sysml",
		"quoted_succession_ends.sysml",
		"individual_definitions.sysml",
	}
	for _, fixture := range fixtures {
		path := filepath.Join("testdata", "convert", fixture)
		name, ext := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			first, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert(name+".ttl", withoutTriples(t, first, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			second, err := export.Convert(name+ext, back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			requireSameGraphBytes(t, back, first, second)
		})
	}
}

// TestIndividualDefinitionWithoutItsFlagReadsAsIndividual covers a graph typed
// sysml:IndividualDefinition that carries no sysml:isIndividual, the shape
// earlier releases wrote: the metaclass states the fact, so it reads back as
// `individual def` and its next hop is the graph an `individual def` writes today.
func TestIndividualDefinitionWithoutItsFlagReadsAsIndividual(t *testing.T) {
	src := `package P {
    individual def Eagle;
}`
	first, err := export.Convert("legacy.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if !strings.Contains(string(first), "sysml:isIndividual") {
		t.Fatalf("an individual def should carry sysml:isIndividual\n%s", first)
	}
	legacy := withoutTriples(t, withoutTriples(t, first, "sysx:sourceText"), "sysml:isIndividual")
	back, err := export.Convert("legacy.ttl", legacy, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "individual def Eagle;") {
		t.Errorf("an IndividualDefinition without its flag should still read as individual\n%s", back)
	}
	second, err := export.Convert("legacy.sysml", back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	requireSameGraphBytes(t, back, first, second)
}

// tripleSetDiff parses two Turtle documents and returns the triples only the
// first holds, then the triples only the second holds, each in document order.
func tripleSetDiff(t *testing.T, first, second []byte) (lost, gained []string) {
	t.Helper()
	parse := func(data []byte) []rdf.Triple {
		graph, err := rdf.ParseTurtle(data)
		if err != nil {
			t.Fatalf("parse turtle: %v", err)
		}
		return graph.Triples()
	}
	only := func(triples, others []rdf.Triple) []string {
		seen := make(map[rdf.Triple]bool, len(others))
		for _, triple := range others {
			seen[triple] = true
		}
		var out []string
		for _, triple := range triples {
			if !seen[triple] {
				out = append(out, triple.Subject.String()+" "+triple.Predicate.String()+" "+triple.Object.String())
			}
		}
		return out
	}
	a, b := parse(first), parse(second)
	return only(a, b), only(b, a)
}

// TestSaveKeepsComments covers the notation-to-notation path a save uses: every
// lexeme survives, including the comments an AST printer would drop.
func TestSaveKeepsComments(t *testing.T) {
	src := `package P {
// a line note
part def Q; // trailing note
/* a comment */
part def R;
}`
	out, err := export.Convert("save.sysml", []byte(src), export.FormatSysML, export.FormatSysML)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	for _, want := range []string{"// a line note", "// trailing note", "/* a comment */"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("save dropped %q:\n%s", want, out)
		}
	}
}

// One element of a document is written through the same notation path a whole
// save goes through: the source at the span, comments included, re-indented.
func TestSysMLElementWritesOneElement(t *testing.T) {
	src := `package P {
	// which wheel
	part def Q {
attribute d = 16.0;
}
	part def R;
}`
	file := source.New("session.sysml", []byte(src))
	span := source.Span{Offset: strings.Index(src, "// which wheel")}
	span.Len = strings.Index(src, "\tpart def R;") - span.Offset

	out, syntax, err := export.SysMLElement(file, span)
	if err != nil || syntax != nil {
		t.Fatalf("element: err=%v syntax=%v", err, syntax)
	}
	got := string(out)
	for _, want := range []string{"// which wheel", "part def Q {", "    attribute d = 16.0;"} {
		if !strings.Contains(got, want) {
			t.Errorf("element output is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "part def R") || strings.Contains(got, "package P") {
		t.Errorf("element output covers more than the element:\n%s", got)
	}
}

// A span running past the element into the comments written for what follows
// writes the element alone: trailing trivia is not part of it.
func TestSysMLElementDropsTrailingComments(t *testing.T) {
	src := "part def Q { attribute d = 16.0; }\n\n// which wheel\n/* and another */\npart def R;"
	file := source.New("session.sysml", []byte(src))
	span := source.Span{Offset: 0, Len: strings.Index(src, "part def R;")}

	out, _, err := export.SysMLElement(file, span)
	if err != nil {
		t.Fatalf("element: %v", err)
	}
	if got := strings.TrimRight(string(out), "\n"); got != "part def Q { attribute d = 16.0; }" {
		t.Errorf("element output carries trailing trivia:\n%q", got)
	}
	if _, _, err := export.SysMLElement(file, source.Span{
		Offset: strings.Index(src, "// which wheel"),
		Len:    len("// which wheel\n"),
	}); !errors.Is(err, export.ErrNoNotation) {
		t.Errorf("a span holding only a comment: err=%v, want ErrNoNotation", err)
	}
}

// A span naming no source is reported as such rather than written as an empty
// document, so a caller can explain it instead of printing nothing.
func TestSysMLElementWithoutSource(t *testing.T) {
	file := source.New("session.sysml", []byte("part def Q;"))
	for _, span := range []source.Span{{}, {Offset: 0, Len: 99}, {Offset: -1, Len: 2}} {
		if _, _, err := export.SysMLElement(file, span); !errors.Is(err, export.ErrNoNotation) {
			t.Errorf("span %+v: err=%v, want ErrNoNotation", span, err)
		}
	}
	if _, _, err := export.SysMLElement(nil, source.Span{Len: 1}); !errors.Is(err, export.ErrNoNotation) {
		t.Errorf("no file: err=%v, want ErrNoNotation", err)
	}
}

// Several kind keywords are synonyms that the AST records as one kind, so the
// keyword as written is carried through the graph rather than normalized.
func TestKindKeywordSynonymsSurviveRDF(t *testing.T) {
	for _, decl := range []string{
		"datatype D;",
		"feature f;",
		"function def F;",
		"message m;",
		"allocate a to b;",
		"timeslice ts;",
		"snapshot sn;",
	} {
		src := "package P {\n\t" + decl + "\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", decl, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", decl, err)
		}
		if !strings.Contains(string(back), decl) {
			t.Errorf("the keyword of %q was rewritten:\n%s", decl, back)
		}
	}
}

// A condition member states a condition rather than declaring a feature, so
// each form it is written in has to come back as written: the constraint it
// asserts, its negation, a bare condition, and a nested condition body.
func TestConditionMembersSurviveRDF(t *testing.T) {
	for _, member := range []string{
		"assert Light;",
		"assert not Light;",
		"mass < 1000;",
		"assume constraint {\n            mass != 0;\n        }",
		"assert constraint inner {\n            mass < 1000;\n        }",
	} {
		src := "package P {\n\tattribute mass;\n\tconstraint def Light;\n\tconstraint c {\n\t\t" + member + "\n\t}\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", member, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", member, err)
		}
		if !strings.Contains(string(back), member) {
			t.Errorf("the condition %q was rewritten:\n%s", member, back)
		}
	}
}

// A requirement's assumptions and required conditions are members of the same
// kind, written as the constraint they state or as a nested condition body.
func TestRequirementConditionsSurviveRDF(t *testing.T) {
	for _, member := range []string{
		"assume constraint {\n            mass > 0;\n        }",
		"require Light;",
		"require constraint {\n            mass < 100;\n        }",
	} {
		src := "package P {\n\tattribute mass;\n\tconstraint def Light;\n\trequirement r {\n\t\t" + member + "\n\t}\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", member, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", member, err)
		}
		if !strings.Contains(string(back), member) {
			t.Errorf("the requirement member %q was rewritten:\n%s", member, back)
		}
	}
}

// An `assume`/`require` member declares a constraint usage of its own — name,
// typing, multiplicity, value, specializations — which the graph must carry
// without the source text, prefixed or not.
func TestRequirementConditionDeclarationsSurviveRDF(t *testing.T) {
	// back is the spelling the mapping alone writes where it differs from the
	// one written: a body's trailing condition is its result expression, bare.
	for _, member := range []struct{ written, back string }{
		{written: "assume constraint c : Light;"},
		{written: "require #Goal constraint d[1] = true;"},
		{
			written: "assume #Goal constraint f : Light subsets Light[0..1] {\n            true;\n        }",
			back:    "assume #Goal constraint f : Light subsets Light[0..1] {\n            true\n        }",
		},
		{written: "assume constraint c references Light;"},
		{written: "assume #Goal constraint c references Light;"},
		{written: "require #Goal constraint c references Light;"},
		{written: "require constraint references Light;"},
		{written: "require Light subsets Light[1];"},
		{written: "require Light {\n        }"},
	} {
		src := "package P {\n\tattribute mass;\n\tconstraint def Light;\n\tmetadata def Goal;\n\trequirement r {\n\t\t" + member.written + "\n\t}\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", member.written, err)
		}
		structural := member.back
		if structural == "" {
			structural = member.written
		}
		for _, hop := range []struct {
			graph []byte
			want  string
		}{
			{turtle, member.written},
			{withoutTriples(t, turtle, "sysx:sourceText"), structural},
		} {
			back, err := export.Convert("m.ttl", hop.graph, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("%s: back to notation: %v", member.written, err)
			}
			if !strings.Contains(string(back), hop.want) {
				t.Errorf("the requirement member %q was rewritten:\n%s", hop.want, back)
			}
		}
	}
}

// `assert` before a kind keyword says what the declaration it qualifies is for,
// so dropping it would come back as a plain constraint — a different model.
func TestAssertedUsagePrefixSurvivesRDF(t *testing.T) {
	for _, decl := range []string{
		"assert constraint ok : Light;",
		"assert not constraint bad : Light;",
	} {
		src := "package P {\n\tconstraint def Light;\n\tpart def Q {\n\t\t" + decl + "\n\t}\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", decl, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", decl, err)
		}
		if !strings.Contains(string(back), decl) {
			t.Errorf("the prefix of %q was dropped:\n%s", decl, back)
		}
	}
}

// The graph carries a name rather than the notation it was written in, so a
// name that is not a basic name (KerML §8.2.2) has to be written back with the
// quotes of an unrestricted name — without them it is two names, or none.
func TestQuotedNamesSurviveRDF(t *testing.T) {
	for _, decl := range []string{
		"package 'Package Example';",
		"part def <'1'> 'Lander Model';",
		"part 'my rover' : 'Rover Model';",
		"alias 'the rover' for 'Rover Model';",
		"part 'off model' : 'Not Declared Here';",
		"import 'Other Package'::*;",
		// A reserved word and a quote are names a graph can carry, and both
		// need quoting to lex as the name again.
		"part def 'state';",
		"part def 'it\\'s';",
	} {
		src := "package P {\n\tpart def 'Rover Model';\n\t" + decl + "\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", decl, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", decl, err)
		}
		if !strings.Contains(string(back), decl) {
			t.Errorf("the quoted names of %q were rewritten:\n%s", decl, back)
		}
		p := parser.New(source.New("m.converted.sysml", back))
		p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Errorf("%s: converted notation does not parse: %v\n%s", decl, p.Diagnostics, back)
		}
	}
}

// Empty braces are part of what a declaration says, so a subject written with
// them must not come back terminated by a semicolon.
func TestSubjectBodySurvivesRDF(t *testing.T) {
	for member, want := range map[string]string{
		"subject r : Rover;":    "subject r : Rover;",
		"subject r : Rover { }": "subject r : Rover {",
		"subject s : Rover { }": "subject s : Rover {",
	} {
		src := "package P {\n\tpart def Rover;\n\trequirement req {\n\t\t" + member + "\n\t}\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", member, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", member, err)
		}
		if !strings.Contains(string(back), want) {
			t.Errorf("the subject %q was rewritten:\n%s", member, back)
		}
	}
}

// A keyword sitting in a comment inside a declaration head is trivia, not the
// declaration's kind, so it must not become the keyword written back.
func TestCommentInHeadDoesNotChangeKeyword(t *testing.T) {
	for _, src := range []string{
		"package P {\n\tattribute // the flow rate\n\t\trate : Real;\n}",
		"package P {\n\tpart /* a state */ def X;\n}",
	} {
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("to turtle: %v", err)
		}
		if strings.Contains(string(turtle), "declaredKeyword") {
			t.Errorf("a comment word was recorded as the kind keyword:\n%s", turtle)
		}
		// The comment itself comes back with the source text; the keyword the
		// printer chooses shows without it.
		back := toNotation(t, withoutTriples(t, turtle, "sysx:sourceText"))
		for _, keyword := range []string{"flow ", "state "} {
			if strings.Contains(back, keyword) {
				t.Errorf("the declaration came back as a %sdeclaration:\n%s", keyword, back)
			}
		}
		if back, want := toNotation(t, turtle), src; back != want {
			t.Errorf("the comment did not come back as written:\n--- want ---\n%s--- got ---\n%s", want, back)
		}
	}
}

// A directed usage records no keyword, so whether it wrote its kind out is read
// from the source — where a comment naming a kind must not count as one written.
// The printer's choice shows on the graph without source text.
func TestCommentedKindKeywordIsNotWrittenBack(t *testing.T) {
	// One case per comment shape the lexer distinguishes, plus a keyword the
	// declaration really does write.
	for name, tt := range map[string]struct{ src, want string }{
		"regular comment":     {"package P {\n\tpart p {\n\t\tin /* attribute */ x : Real;\n\t}\n}", "in x : Real;"},
		"single-line note":    {"package P {\n\tpart p {\n\t\tout // attribute\n\t\t\ty : Real;\n\t}\n}", "out y : Real;"},
		"multi-line note":     {"package P {\n\tpart p {\n\t\tin //* a note\n\t\t\tattribute */ w : Real;\n\t}\n}", "in w : Real;"},
		"note over two lines": {"package P {\n\tpart p {\n\t\tin /* a note\n\t\t\tattribute */ v : Real;\n\t}\n}", "in v : Real;"},
		"keyword written":     {"package P {\n\tpart p {\n\t\tin attribute z : Real;\n\t}\n}", "in attribute z : Real;"},
	} {
		t.Run(name, func(t *testing.T) {
			turtle, err := export.Convert("m.sysml", []byte(tt.src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back := toNotation(t, withoutTriples(t, turtle, "sysx:sourceText"))
			if !strings.Contains(back, tt.want) {
				t.Errorf("wanted %q written back from %q:\n%s", tt.want, tt.src, back)
			}
		})
	}
}

// A usage whose head is kept verbatim comes back as written, so a synonym
// keyword on it needs no rebuilding and is not refused.
func TestVerbatimSynonymConverts(t *testing.T) {
	src := "package P {\n\trequirement def R;\n\tverify R;\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "verify R;") {
		t.Errorf("`verify R;` did not survive the round trip:\n%s", back)
	}
}

// A `perform` declares an action of its own, so the graph carries the keyword
// it was written with: the canonical `action` would be a different declaration.
func TestPerformedActionKeepsItsKeyword(t *testing.T) {
	src := "package P {\n\taction def A;\n\tpart def Q {\n\t\tperform a : A;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "perform a : A;") {
		t.Errorf("the `perform` did not survive the round trip:\n%s", back)
	}
}

// A kind keyword written as one word of a two-word kind (`verification def`
// for a verification case) comes back as written: the canonical spelling
// reparses as a plain `case`, a different kind.
func TestShortKindKeywordSurvivesTheRoundTrip(t *testing.T) {
	src := "package P {\n\tverification def V;\n\tanalysis def A;\n\tanalysis a : A;\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, want := range []string{"verification def V;", "analysis def A;", "analysis a : A;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("`%s` did not survive the round trip:\n%s", want, back)
		}
	}
}

// A `#M` prefix is an owned metadata usage linked to its definition; the head
// comes back from the graph alone, a `$::`/short-name type as its declared name.
func TestPrefixMetadataComesBackFromTheGraphAlone(t *testing.T) {
	heads := []struct{ written, back string }{
		{written: "#Safety part def Car;"},
		{written: "abstract #Safety #Reviewed part def Truck;"},
		{written: "#Safety part car : Car;"},
		{written: "#$::P::Safety part car : Car;", back: "#Safety part car : Car;"},
		{written: "#safe part named : Car;", back: "#Safety part named : Car;"},
		{written: "private ref #Safety part spare : Car;"},
		{written: "variant #Reviewed part option : Car;"},
		{written: "end #Safety part wheel : Car;"},
		{written: "#Reviewed package Notes;"},
		{written: "#Safety dependency from Car to Vehicle;"},
		{written: "requirement def R {\n        subject #Safety s : Car;\n    }"},
		{written: "use case def U {\n        objective #Safety o : Goal;\n    }"},
		{written: "use case def U {\n        #Safety include Ride;\n    }"},
		{written: "use case def U {\n        #Safety include use case ride : Ride;\n    }"},
		{written: "requirement def R {\n        assume #Reviewed constraint {\n            true\n        }\n    }"},
		{written: "requirement def R {\n        require #Safety constraint {\n            true\n        }\n    }"},
		{written: "part def Q {\n        #Safety assert constraint ok : Stopped;\n    }"},
		{written: "part def Q {\n        #Safety assert not constraint bad : Stopped;\n    }"},
		{written: "part def Q {\n        ref #Safety assert not constraint bad : Stopped;\n    }"},
		{written: "part def Q {\n        #Safety perform action go : Move;\n    }"},
	}
	for _, head := range heads {
		t.Run(head.written, func(t *testing.T) {
			src := "package P {\n    metadata def <safe> Safety;\n    metadata def Reviewed;\n    part def Vehicle;\n    requirement def Goal;\n    use case def Ride;\n    constraint def Stopped;\n    action def Move;\n    " + head.written + "\n}\n"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			if !strings.Contains(string(turtle), `sysx:declaredKeyword "#"`) {
				t.Fatalf("the graph does not state the prefix form:\n%s", turtle)
			}
			for _, name := range []string{"Safety", "safe", "$::P::Safety", "Reviewed"} {
				if strings.Contains(string(turtle), `sysml:type "`+name+`"`) {
					t.Fatalf("the metadata type %s is written as a name, not linked to its definition:\n%s", name, turtle)
				}
			}
			back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			want := head.back
			if want == "" {
				want = head.written
			}
			if !strings.Contains(string(back), want) {
				t.Fatalf("the head should come back as `%s`:\n%s", want, back)
			}
			again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			first, second := turtle, again
			if head.back != "" {
				first = withoutTriples(t, turtle, "sysx:sourceText")
				second = withoutTriples(t, again, "sysx:sourceText")
			}
			if string(second) != string(first) {
				t.Errorf("the second hop changed the graph\n--- first ---\n%s\n--- second ---\n%s", first, second)
			}
		})
	}
}

// KerML's FeaturePrefix puts prefix metadata after `var`, so a variable
// feature's `#M` comes back there from the graph alone.
func TestVarPrefixMetadataComesBackFromTheGraphAlone(t *testing.T) {
	heads := []string{
		"var #Safety feature x;",
		"derived var #Safety feature y : Safety;",
	}
	for _, head := range heads {
		t.Run(head, func(t *testing.T) {
			src := "package P {\n    metadata def Safety;\n    class C {\n        " + head + "\n    }\n}\n"
			turtle, err := export.Convert("m.kerml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if !strings.Contains(string(back), head) {
				t.Fatalf("the head should come back as written:\n%s", back)
			}
			again, err := export.Convert("m.kerml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if string(again) != string(turtle) {
				t.Errorf("the second hop changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
			}
		})
	}
}

// A prefix annotation is identified by its position after the body members,
// so a body member named as that position is refused rather than merged with
// it; a member named as another position is no collision.
func TestPrefixCollidingWithAPositionNamedMemberIsReported(t *testing.T) {
	src := "package P {\n\tmetadata def Safety;\n\t#Safety part def Car {\n\t\tpart '@1';\n\t}\n}"
	_, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected an unsupported error, got %v", err)
	}
	for _, want := range []string{"the prefix annotation at m.sysml:3:2", "identified by its position as P::Car::@1, which a body member is named"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in error:\n%s", want, err.Error())
		}
	}

	src = "package P {\n\tmetadata def Safety;\n\t#Safety part def Car {\n\t\tpart '@0';\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "#Safety part def Car {\n        part '@0';\n    }") {
		t.Errorf("the prefix and the member should both come back:\n%s", back)
	}
}

// A metadata usage is owned through an OwningMembership even when a
// relationship owns it, so a client reaches it the way it reaches any member.
func TestMetadataOnARelationshipIsOwnedThroughAMembership(t *testing.T) {
	src := "package P {\n\tmetadata def Safety;\n\tpart def Car;\n\t#Safety dependency from P to Car;\n\trequirement def R {\n\t\tsubject #Safety s : Car;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, owner := range []string{"P___402", "P__R__s"} {
		member, membership := "elmt:"+owner+"___400", "elmt:"+owner+"___400_om"
		for _, want := range []string{
			membership + "\n    a sysml:OwningMembership ;",
			"sysml:memberElement " + member,
			"sysml:ownedMemberElement " + member,
			"sysml:owningRelatedElement elmt:" + owner,
			"sysml:owningMembership " + membership,
			"sysml:ownedRelationship " + membership,
		} {
			if !strings.Contains(string(turtle), want) {
				t.Errorf("the graph does not state %q:\n%s", want, turtle)
			}
		}
		for _, reject := range []string{"sysml:memberElement " + member + " ;\n    sysml:ownedMemberFeature", "sysml:membershipOwningNamespace elmt:" + owner} {
			if strings.Contains(string(turtle), reject) {
				t.Errorf("a relationship is no namespace, yet the graph states %q:\n%s", reject, turtle)
			}
		}
	}
	back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, head := range []string{"#Safety dependency from P to Car;", "subject #Safety s : Car;"} {
		if !strings.Contains(string(back), head) {
			t.Errorf("the head should come back as written:\n%s", back)
		}
	}
}

// A metadata usage member carries its body's feature values, its name, the
// elements it is about and the `@` it was written with, so every member form
// comes back from the mapping alone and converts to the same graph again. A
// linked type or body reference is spelled relative to the member's scope, so a
// globally qualified type comes back by its name and an inherited feature
// unqualified.
func TestMetadataMembersComeBackFromTheGraphAlone(t *testing.T) {
	members := []struct{ written, back string }{
		{written: "@Safety;"},
		{written: "private @Safety;"},
		{written: "protected @ checked : Safety about Car;"},
		{written: "@Safety {\n            level = 2;\n        }"},
		{written: "@Safety {\n            redefines level = 3;\n            reviewer = \"ops\";\n            audit {\n                year = 2026;\n            }\n        }"},
		{written: "@ checked : Safety;"},
		{written: "@Safety about Car, Vehicle;"},
		{
			written: "@Safety about Car {\n            level = mass;\n            reviewer = Car::name;\n        }",
			back:    "@Safety about Car {\n            level = mass;\n            reviewer = name;\n        }",
		},
		{written: "metadata tagged : Safety about Car;"},
		{written: "metadata Safety about Car;"},
		{
			written: "metadata $::P::Safety about Car {\n            level = 4;\n        }",
			back:    "metadata Safety about Car {\n            level = 4;\n        }",
		},
	}
	for _, member := range members {
		t.Run(member.written, func(t *testing.T) {
			want := member.back
			if want == "" {
				want = member.written
			}
			src := "package P {\n    metadata def Safety {\n        attribute level : Integer;\n        attribute reviewer : String;\n        item audit {\n            attribute year : Integer;\n        }\n    }\n    part def Vehicle;\n    part def Car {\n        attribute name : String;\n    }\n    part car : Car {\n        attribute mass : Real;\n        " + member.written + "\n    }\n}\n"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if !strings.Contains(string(back), want) {
				t.Fatalf("the member should come back as %q:\n%s", want, back)
			}
			again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			first, second := turtle, again
			if member.back != "" {
				first = withoutTriples(t, turtle, "sysx:sourceText")
				second = withoutTriples(t, again, "sysx:sourceText")
			}
			if string(second) != string(first) {
				t.Errorf("the second hop changed the graph\n--- first ---\n%s\n--- second ---\n%s", first, second)
			}
		})
	}
}

// A metadata usage that annotates several elements states each of them, and
// the writer puts every one back in its about clause.
func TestEveryAnnotatedElementIsStated(t *testing.T) {
	src := "package P {\n\tmetadata def Safety;\n\tpart def Car;\n\tpart def Truck;\n\tpart def Van;\n\t@Safety about Car, Truck, Van;\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if !strings.Contains(string(turtle), "sysml:annotatedElement elmt:P__Car, elmt:P__Truck, elmt:P__Van") {
		t.Errorf("the graph does not annotate every element:\n%s", turtle)
	}
	back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "@Safety about Car, Truck, Van;") {
		t.Errorf("the about clause did not come back whole:\n%s", back)
	}
}

// A prefix annotation is spelled as its type alone, so a graph that gives one
// a body, a name or an about clause is refused rather than written as a form
// the grammar has no place for.
func TestPrefixAnnotationWithABodyIsReported(t *testing.T) {
	src := "package P {\n\tmetadata def Safety {\n\t\tattribute level : Integer;\n\t}\n\tpart car {\n\t\t@Safety {\n\t\t\tlevel = 2;\n\t\t}\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	asPrefix := strings.Replace(string(withoutTriples(t, turtle, "sysx:sourceText")), `sysx:declaredKeyword "@"`, `sysx:declaredKeyword "#"`, 1)
	_, err = export.Convert("m.ttl", []byte(asPrefix), export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected an unsupported error, got %v", err)
	}
	for _, want := range []string{"the prefix annotation <urn:sysmlv2:element:P__car__", "a body is written by the `@` member form"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in error:\n%s", want, err.Error())
		}
	}
}

// A `metadata` usage typed by no definition, or by several, is refused in every
// written form rather than written as a declaration that does not parse.
func TestMetadataUsageWithoutOneDefinitionIsReported(t *testing.T) {
	src := "package P {\n\tmetadata def M;\n\tpart def Car;\n\tmetadata m : M about Car;\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural := string(withoutTriples(t, turtle, "sysx:sourceText"))
	if !strings.Contains(structural, "    sysml:type elmt:P__M ;\n") {
		t.Fatalf("the metadata usage's typing was not found in the graph:\n%s", structural)
	}
	for _, form := range []struct{ keyword, replacement string }{
		{"metadata", ""},
		{"metadata", "    sysml:type elmt:P__M, elmt:P__Car ;\n"},
		{"@", ""},
		{"@", "    sysml:type elmt:P__M, elmt:P__Car ;\n"},
	} {
		graph := strings.Replace(structural, "    sysml:type elmt:P__M ;\n", form.replacement, 1)
		if form.keyword == "@" {
			graph = strings.Replace(graph, `sysml:declaredName "m" ;`, `sysml:declaredName "m" ;`+"\n"+`    sysx:declaredKeyword "@" ;`, 1)
		}
		_, err := export.Convert("m.ttl", []byte(graph), export.FormatTurtle, export.FormatSysML)
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Fatalf("%s form, types %q: expected an unsupported error, got %v", form.keyword, form.replacement, err)
		}
		for _, want := range []string{"the element <urn:sysmlv2:element:P__m>", "sysml:type", "names the one metadata definition it applies"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s form, types %q: expected %q in error:\n%s", form.keyword, form.replacement, want, err.Error())
			}
		}
	}
}

// A metadata usage is typed by a metadata definition (or a KerML metaclass);
// a graph typing one by a part or attribute definition is refused rather than
// written as an `@Car` annotation, in each of the three forms.
func TestMetadataUsageTypedByANonMetadataDefinitionIsReported(t *testing.T) {
	src := "package P {\n\tmetadata def M;\n\tpart def Car;\n\tattribute def Mass;\n\tmetadata m : M about Car;\n\t@M;\n\t#M part def Truck;\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural := string(withoutTriples(t, turtle, "sysx:sourceText"))
	const typing = "    sysml:type elmt:P__M ;\n"
	typings := strings.Split(structural, typing)
	if len(typings) != 4 {
		t.Fatalf("expected three metadata usages typed by M in the graph:\n%s", structural)
	}
	if _, err := export.Convert("m.ttl", []byte(structural), export.FormatTurtle, export.FormatSysML); err != nil {
		t.Fatalf("control: the graph typed by M does not convert: %v", err)
	}
	for _, other := range []struct{ id, metaclass string }{{"P__Car", "PartDefinition"}, {"P__Mass", "AttributeDefinition"}} {
		// Retype the metadata usages one at a time: the named member, the
		// `@` member and the `#` prefix.
		for i := 0; i < 3; i++ {
			graph := typings[0]
			for n, rest := range typings[1:] {
				if n == i {
					graph += "    sysml:type elmt:" + other.id + " ;\n" + rest
				} else {
					graph += typing + rest
				}
			}
			_, err := export.Convert("m.ttl", []byte(graph), export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("usage %d typed by %s: expected an unsupported error, got %v", i, other.id, err)
			}
			for _, want := range []string{"sysml:type <urn:sysmlv2:element:" + other.id + ">", "is a sysml:" + other.metaclass, "names the one metadata definition it applies"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("usage %d typed by %s: expected %q in error:\n%s", i, other.id, want, err.Error())
				}
			}
		}
	}
}

// A metadata usage typed by a literal is typed by a name the graph does not
// define; a literal that is no name — a number, a boolean, a tagged or an
// expression-typed string, an empty or broken qualified name — is refused in
// each of the three forms rather than written as `@42` or `@1 + 2`.
func TestMetadataUsageTypedByANonNameLiteralIsReported(t *testing.T) {
	src := "package P {\n\tmetadata def M;\n\tpart def Car;\n\tmetadata m : M about Car;\n\t@M;\n\t#M part def Truck;\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural := string(withoutTriples(t, turtle, "sysx:sourceText"))
	const typing = "    sysml:type elmt:P__M ;\n"
	typings := strings.Split(structural, typing)
	if len(typings) != 4 {
		t.Fatalf("expected three metadata usages typed by M in the graph:\n%s", structural)
	}
	retyped := func(i int, object string) string {
		graph := typings[0]
		for n, rest := range typings[1:] {
			if n == i {
				graph += "    sysml:type " + object + " ;\n" + rest
			} else {
				graph += typing + rest
			}
		}
		return graph
	}
	// Control: a name the graph does not define is written as that name.
	for i, want := range []string{"metadata m : Ext::'Safety Level' about Car;", "@Ext::'Safety Level';", "#Ext::'Safety Level' part def Truck;"} {
		back, err := export.Convert("m.ttl", []byte(retyped(i, `"Ext::Safety Level"`)), export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("usage %d typed by a plain name: %v", i, err)
		}
		if !strings.Contains(string(back), want) {
			t.Errorf("usage %d typed by a plain name: expected %q in:\n%s", i, want, back)
		}
	}
	// A literal of a datatype no name has is refused by the graph-wide literal
	// gate; a string that spells no name by the metadata usage's own check.
	for _, literal := range []struct{ object, why string }{
		{`"42"^^xsd:integer`, "sysml:type takes a string or sysx:Expression"},
		{`"true"^^xsd:boolean`, "sysml:type takes a string or sysx:Expression"},
		{`"M"@en`, "a language-tagged literal is an rdf:langString"},
	} {
		for i := 0; i < 3; i++ {
			_, err := export.Convert("m.ttl", []byte(retyped(i, literal.object)), export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("usage %d typed by %s: expected an unsupported error, got %v", i, literal.object, err)
			}
			for _, want := range []string{"the literal " + literal.object + " stated by <urn:sysmlv2:element:P__", "sysml:type", literal.why} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("usage %d typed by %s: expected %q in error:\n%s", i, literal.object, want, err.Error())
				}
			}
		}
	}
	for _, literal := range []struct{ object, why string }{
		{`"1 + 2"^^sysx:Expression`, "an expression, not a name"},
		{`""`, "is empty"},
		{`"P::"`, "an empty name segment"},
		{`"$::"`, "an empty name segment"},
		{"\"Safety\\nLevel\"", "a line break"},
		{`"M\\"`, "ending in a backslash"},
	} {
		for i := 0; i < 3; i++ {
			_, err := export.Convert("m.ttl", []byte(retyped(i, literal.object)), export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("usage %d typed by %s: expected an unsupported error, got %v", i, literal.object, err)
			}
			for _, want := range []string{"the element <urn:sysmlv2:element:P__", "sysml:type", literal.why, "names the one metadata definition it applies"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("usage %d typed by %s: expected %q in error:\n%s", i, literal.object, want, err.Error())
				}
			}
		}
	}
}

// A metadata usage's keyword is `metadata`, `@` or `#`; a graph stating any
// other is refused rather than written as a declaration of that other kind,
// and a `#` prefix owned by no declaration has nothing to prefix.
func TestMetadataUsageWithAnUnsupportedKeywordIsReported(t *testing.T) {
	refused := func(name, graph string, wants ...string) {
		t.Helper()
		_, err := export.Convert("m.ttl", []byte(graph), export.FormatTurtle, export.FormatSysML)
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Fatalf("%s: expected an unsupported error, got %v", name, err)
		}
		for _, want := range wants {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: expected %q in error:\n%s", name, want, err.Error())
			}
		}
	}

	src := "package P {\n\tmetadata def M;\n\tpart def Car;\n\tmetadata m : M about Car;\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural := string(withoutTriples(t, turtle, "sysx:sourceText"))
	if !strings.Contains(structural, `sysml:declaredName "m" ;`) {
		t.Fatalf("the metadata usage's name was not found in the graph:\n%s", structural)
	}
	for _, keyword := range []string{"part", "metadata", "@@", ""} {
		graph := strings.Replace(structural, `sysml:declaredName "m" ;`, `sysml:declaredName "m" ;`+"\n"+`    sysx:declaredKeyword "`+keyword+`" ;`, 1)
		refused(keyword, graph, "the element <urn:sysmlv2:element:P__m>", "sysx:declaredKeyword", `"`+keyword+`"`, "is not a metadata form")
	}
	// A repeated keyword is refused by the graph-wide cardinality gate whichever
	// value comes first, rather than read as the first one and written as a
	// prefix or a member.
	for _, keywords := range []string{`"@", "#"`, `"#", "@"`, `"@", "metadata"`} {
		graph := strings.Replace(structural, `sysml:declaredName "m" ;`, `sysml:declaredName "m" ;`+"\n"+`    sysx:declaredKeyword `+keywords+` ;`, 1)
		refused(keywords, graph, "the subject <urn:sysmlv2:element:P__m>", "states sysx:declaredKeyword twice", "one of them would be dropped")
	}

	root := "metadata def M;\n@M;\n"
	turtle, err = export.Convert("m.sysml", []byte(root), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural = string(withoutTriples(t, turtle, "sysx:sourceText"))
	if !strings.Contains(structural, `sysx:declaredKeyword "@"`) {
		t.Fatalf("the root annotation's sigil was not found in the graph:\n%s", structural)
	}
	asPrefix := strings.Replace(structural, `sysx:declaredKeyword "@"`, `sysx:declaredKeyword "#"`, 1)
	refused("root prefix", asPrefix, "the prefix annotation <urn:sysmlv2:element:", "owned by no declaration")
}

// A prefix annotation on an element whose notation has no place for one is
// refused rather than dropped from the body it was read from.
func TestPrefixOnAnUnprefixableHeadIsReported(t *testing.T) {
	src := "package P {\n\tmetadata def Safety;\n\taction def A {\n\t\t#Safety action a;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	asFork := strings.Replace(string(withoutTriples(t, turtle, "sysx:sourceText")), "a sysml:ActionUsage ;", "a sysml:ForkNode ;", 1)
	if asFork == string(turtle) {
		t.Fatalf("the action usage was not found in the graph:\n%s", turtle)
	}
	_, err = export.Convert("m.ttl", []byte(asFork), export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected an unsupported error, got %v", err)
	}
	if !strings.Contains(err.Error(), "whose notation takes no prefix annotation") {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

// A prefix annotation on a condition member that states an inline condition or
// a constraint reference has no position in the notation (`assume #M x > 0`
// does not parse), so such a graph is refused rather than written unparseable.
func TestPrefixOnAConditionWithoutADeclarationIsReported(t *testing.T) {
	src := "package P {\n\tmetadata def Safety;\n\tconstraint def C;\n\trequirement def R {\n\t\tassume #Safety constraint c;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural := string(withoutTriples(t, turtle, "sysx:sourceText"))
	// The reference form declares nothing: neither a name nor the `constraint`
	// keyword, which is what tells a declaration's `references C` from it.
	declaration := `sysml:declaredName "c" ;` + "\n    " + `sysx:declaredKeyword "constraint" ;`
	if !strings.Contains(structural, declaration) {
		t.Fatalf("the assume member's declaration was not found in the graph:\n%s", structural)
	}
	for _, tc := range []struct{ triples, form string }{
		{declaration + "\n    " + `sysx:condition "true" ;`, "an inline condition"},
		{`sysml:references elmt:P__C ;`, "a constraint reference"},
	} {
		t.Run(tc.form, func(t *testing.T) {
			edited := strings.Replace(structural, declaration, tc.triples, 1)
			_, err := export.Convert("m.ttl", []byte(edited), export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("expected an unsupported error, got %v", err)
			}
			for _, want := range []string{"the condition member <", "this member states " + tc.form} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected %q in error:\n%s", want, err.Error())
				}
			}
		})
	}
}

// A condition member stating no condition, reference, keyword or body
// (`sysx:hasBody false`) is refused, not written as an invented constraint.
func TestConditionWithoutAConditionIsReported(t *testing.T) {
	src := "package P {\n\tconstraint def C;\n\trequirement def R {\n\t\tassume constraint c;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural := string(withoutTriples(t, turtle, "sysx:sourceText"))
	for _, want := range []string{`sysx:declaredKeyword "constraint" ;`, `sysx:hasBody "false"^^xsd:boolean .`} {
		if !strings.Contains(structural, want) {
			t.Fatalf("%s was not found in the graph:\n%s", want, structural)
		}
	}
	edited := strings.Replace(structural, `sysx:declaredKeyword "constraint" ;`, "", 1)
	_, err = export.Convert("m.ttl", []byte(edited), export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected an unsupported error, got %v", err)
	}
	if !strings.Contains(err.Error(), "a condition member states a condition") {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

// An `assume`/`require` member's sysx:declaredKeyword names the `constraint`
// declaration form and nothing else; another value is refused rather than the
// member written in a form the keyword did not state.
func TestConditionWithAnUnsupportedKeywordIsReported(t *testing.T) {
	src := "package P {\n\tconstraint def C;\n\trequirement def R {\n\t\tassume constraint c { true }\n\t\trequire constraint d;\n\t\trequire C;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural := string(withoutTriples(t, turtle, "sysx:sourceText"))
	if got := strings.Count(structural, `sysx:declaredKeyword "constraint" ;`); got != 2 {
		t.Fatalf("expected two declared constraints in the graph, found %d:\n%s", got, structural)
	}
	// `require C;` reads as an inline condition naming C; the reference form is
	// what a graph states through sysml:references.
	const inline = `sysx:condition expr:P__R___402_pcondition .`
	if !strings.Contains(structural, inline) {
		t.Fatalf("the inline require member was not found in the graph:\n%s", structural)
	}
	const declared, assert = `sysx:declaredKeyword "constraint" ;`, `sysx:declaredKeyword "assert" ;`
	// The two declared constraints are written in source order: c, then d.
	secondDeclared := strings.Replace(structural, declared, "\x00", 1)
	secondDeclared = strings.Replace(strings.Replace(secondDeclared, declared, assert, 1), "\x00", declared, 1)
	for _, tc := range []struct{ name, edited, member string }{
		{"bodied assume", strings.Replace(structural, declared, assert, 1), "assume"},
		{"bodyless require", secondDeclared, "require"},
		{"inline require", strings.Replace(structural, inline, `sysx:declaredKeyword "verify" ;`+"\n    "+inline, 1), "require"},
		{"reference-form require", strings.Replace(structural, inline, `sysx:declaredKeyword "verify" ;`+"\n    "+`sysml:references elmt:P__C .`, 1), "require"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.edited == structural {
				t.Fatal("the graph was not edited")
			}
			_, err := export.Convert("m.ttl", []byte(tc.edited), export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("expected an unsupported error, got %v", err)
			}
			for _, want := range []string{"the condition member <", "is not a form of a " + tc.member + " member"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected %q in error:\n%s", want, err.Error())
				}
			}
		})
	}
	// The unedited graph still round-trips: `constraint` is the one supported keyword.
	back, err := export.Convert("m.ttl", []byte(structural), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to sysml: %v", err)
	}
	for _, want := range []string{"assume constraint c {", "require constraint d;", "require C;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("expected %q in:\n%s", want, back)
		}
	}
}

// A member stating an inline condition and also facts of the declaration or
// reference form is refused: the notation writes one form, and writing the
// condition alone would drop the rest.
func TestInlineConditionWithDeclarationFactsIsReported(t *testing.T) {
	src := "package P {\n\tconstraint def C;\n\trequirement def R {\n\t\trequire C;\n\t}\n\tconstraint q {\n\t\tassert C;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural := string(withoutTriples(t, turtle, "sysx:sourceText"))
	const require, assert = `sysx:condition expr:P__R___400_pcondition .`, `sysx:condition expr:P__q___400_pcondition .`
	for _, inline := range []string{require, assert} {
		if !strings.Contains(structural, inline) {
			t.Fatalf("%s was not found in the graph:\n%s", inline, structural)
		}
	}
	for _, tc := range []struct{ name, inline, added, drops string }{
		{"declared constraint keyword", require, `sysx:declaredKeyword "constraint" ;`, "declares a `constraint`"},
		{"stated constraint", require, `sysml:references elmt:P__C ;`, "states a constraint through sysml:references"},
		{"body", require, `sysx:hasBody "true"^^xsd:boolean ;`, "has a body"},
		{"name", require, `sysml:declaredName "c" ;`, "declares a name"},
		{"specialization", require, `sysml:subsets elmt:P__C ;`, "declares specializations"},
		{"multiplicity", require, `sysml:upperBound "1" ;`, "declares a multiplicity"},
		{"value", require, `sysml:value "1" ;`, "has a value"},
		{"asserted body", assert, `sysx:hasBody "true"^^xsd:boolean ;`, "has a body"},
		{"asserted name", assert, `sysml:declaredName "c" ;`, "declares a name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edited := strings.Replace(structural, tc.inline, tc.added+"\n    "+tc.inline, 1)
			if edited == structural {
				t.Fatal("the graph was not edited")
			}
			_, err := export.Convert("m.ttl", []byte(edited), export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("expected an unsupported error, got %v", err)
			}
			for _, want := range []string{"the condition member <", "states an inline condition", tc.drops} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected %q in error:\n%s", want, err.Error())
				}
			}
		})
	}
	// The unedited graph still round-trips: the inline form alone is the form.
	back, err := export.Convert("m.ttl", []byte(structural), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to sysml: %v", err)
	}
	for _, want := range []string{"require C;", "assert C;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("expected %q in:\n%s", want, back)
		}
	}
}

// A head kept as source text writes its prefix annotations in that text; when
// the text and the graph disagree, the text is stale and the head is rebuilt
// from the graph rather than the annotation lost.
func TestPrefixOnAVerbatimHeadIsWrittenOrReported(t *testing.T) {
	// The text may space or qualify a prefix in ways the graph's rendering does
	// not; a `#` in the body or in a sequence index is not a prefix.
	heads := []string{
		"#Safety connect x to y;",
		"# Safety connect x to y;",
		"#P::Safety #Audit connect x to y;",
		"#$::P::Safety connect x to y;",
		"#Safety connect x to y {\n\t\t\t#Audit part p;\n\t\t}",
		"connect x to y {\n\t\t\t#Safety part p;\n\t\t}",
		"transition t first x if xs#(1) > 0 then y;",
	}
	for _, head := range heads {
		src := "package P {\n\tmetadata def Safety;\n\tmetadata def Audit;\n\tattribute xs : Integer[*];\n\tpart def A {\n\t\tport x;\n\t\tport y;\n\t}\n\tpart a : A {\n\t\t" + head + "\n\t}\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", head, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", head, err)
		}
		spaced := func(s string) string { return strings.Join(strings.Fields(s), " ") }
		if !strings.Contains(spaced(string(back)), spaced(head)) {
			t.Errorf("%s: the prefixed head should come back as written:\n%s", head, back)
		}
	}
	src := "package P {\n    metadata def Safety;\n    metadata def Audit;\n    part a {\n        port x;\n        port y;\n        #Safety connect x to y;\n    }\n}\n"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation without source text: %v", err)
	}
	if !strings.Contains(string(back), "#Safety connect x to y;") {
		t.Errorf("the prefixed head should come back from the graph alone:\n%s", back)
	}
	written := `sysx:sourceText "        #Safety connect x to y;\n"`
	for _, stale := range []string{
		`"        connect x to y;\n"`,
		`"        #Audit connect x to y;\n"`,
		`"        #Q::Safety connect x to y;\n"`,
		`"        #Safety #Safety connect x to y;\n"`,
	} {
		edited := strings.Replace(string(turtle), written, `sysx:sourceText `+stale, 1)
		if edited == string(turtle) {
			t.Fatalf("the verbatim head was not found in the graph:\n%s", turtle)
		}
		back, err := export.Convert("m.ttl", []byte(edited), export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", stale, err)
		}
		if strings.Count(string(back), "#Safety connect x to y;") != 1 || strings.Contains(string(back), "#Audit") || strings.Contains(string(back), "#Q::") {
			t.Errorf("%s: the stale head should be rebuilt with the graph's annotation:\n%s", stale, back)
		}
	}
	// The verbatim head prints no members, so an annotation whose keyword is
	// not a metadata form is refused there rather than dropped with the body;
	// a repeated keyword is refused by the graph-wide cardinality gate.
	for _, tc := range []struct{ keywords, subject, note string }{
		{`"#", "@"`, "the subject <urn:sysmlv2:element:P__a__", "states sysx:declaredKeyword twice"},
		{`"part"`, "the element <urn:sysmlv2:element:P__a__", `sysx:declaredKeyword "part" is not a metadata form`},
	} {
		edited := strings.Replace(string(turtle), `sysx:declaredKeyword "#"`, `sysx:declaredKeyword `+tc.keywords, 1)
		if edited == string(turtle) {
			t.Fatalf("the prefix's keyword was not found in the graph:\n%s", turtle)
		}
		_, err = export.Convert("m.ttl", []byte(edited), export.FormatTurtle, export.FormatSysML)
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Fatalf("%s: expected an unsupported error, got %v", tc.keywords, err)
		}
		for _, want := range []string{tc.subject, tc.note} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: expected %q in error:\n%s", tc.keywords, want, err.Error())
			}
		}
	}
}

// A feature that wrote no kind keyword takes its kind from its owner, so the
// graph must not put a keyword back that the author never wrote.
func TestImplicitKindStaysImplicitThroughTheRoundTrip(t *testing.T) {
	src := "package P {\n\taction def Drive {\n\t\tin x : Real;\n\t\tin attribute y : Real;\n\t\tout result : Real;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, want := range []string{"in x : Real;", "in attribute y : Real;", "out result : Real;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("`%s` did not survive the round trip:\n%s", want, back)
		}
	}
}

// Every direction rejects notation the parser cannot read, including the
// notation-to-notation save: formatting broken input would suggest it is valid.
func TestSysMLToSysMLChecksSyntax(t *testing.T) {
	_, err := export.Convert("bad.sysml", []byte("package P {\n\tpart ((( ;\n}"), export.FormatSysML, export.FormatSysML)
	var syntax *export.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("want a SyntaxError, got %v", err)
	}
}

// A member-attached `then` is a succession edge whose ends the graph states,
// and whose form says the source end is the member before it, so the notation
// it was written in comes back as written. Member order alone would not carry
// the sequencing: the declaration order here is the reverse.
func TestSuccessionRoundTrips(t *testing.T) {
	src := `package P {
	action def A;
	action def B;
	action def Move {
		action b : B;
		action a : A;
		then action c : A;
	}
}`
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{"sysml:sourceFeature", "sysml:targetFeature", "SuccessionAsUsage"} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph should carry the succession as %s:\n%s", want, turtle)
		}
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "then action c : A;") {
		t.Fatalf("the succession should come back as the form it was written in:\n%s", back)
	}
	// The notation that came back declares the same succession: converting it
	// again yields the same graph, which member order could not have done.
	again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if string(again) != string(turtle) {
		t.Errorf("round trip changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
	}
}

// A succession is written back as the edge form, which every body that can carry
// a succession has to read for the notation to survive the round trip.
func TestSuccessionRoundTripsInEveryBody(t *testing.T) {
	bodies := map[string]string{
		"definition":  "part def Q {\n\t\tpart a;\n\t\tthen part b;\n\t}",
		"action":      "action def Q {\n\t\taction a;\n\t\tthen action b;\n\t}",
		"state":       "state def Q {\n\t\tstate a : S;\n\t\tthen state b : S;\n\t}",
		"calculation": "calc def Q {\n\t\tpart a;\n\t\tthen part b;\n\t}",
		"requirement": "requirement def Q {\n\t\tpart a;\n\t\tthen part b;\n\t}",
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			src := "package P {\n\tstate def S;\n\t" + body + "\n}"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			if !strings.Contains(string(turtle), "sysml:sourceFeature") {
				t.Fatalf("the graph should carry the succession's ends:\n%s", turtle)
			}
			back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			// The notation that came back has to parse, and to declare the same
			// succession: a body that cannot read the edge form loses the order.
			again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again (%s):\n%s\n%v", name, back, err)
			}
			if string(again) != string(turtle) {
				t.Errorf("round trip changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
			}
		})
	}
}

// A succession is its two ends, so a graph from elsewhere that names only one of
// them declares no order: that is reported rather than written back as notation
// (`succession;`) that says nothing.
func TestHalfNamedSuccessionInAGraphIsReported(t *testing.T) {
	const graph = `@prefix elmt: <urn:sysmlv2:element:> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

elmt:P
    a sysml:Package ;
    sysml:qualifiedName "P" ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysml:declaredName "P" ;
    sysx:hasBody "true"^^xsd:boolean .

elmt:P::a
    a sysml:ActionUsage ;
    sysml:qualifiedName "P::a" ;
    sysml:owningNamespace elmt:P ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysml:declaredName "a" ;
    sysx:hasBody "false"^^xsd:boolean .

<urn:sysmlv2:element:P::@1>
    a sysml:SuccessionAsUsage ;
    sysml:qualifiedName "P::@1" ;
    sysml:owningNamespace elmt:P ;
    sysx:memberIndex "1"^^xsd:integer ;
    sysml:sourceFeature elmt:P::a .
`
	out, err := export.Convert("m.ttl", []byte(graph), export.FormatTurtle, export.FormatSysML)
	if err == nil {
		t.Fatalf("a succession naming one end converted to:\n%s", out)
	}
	if !strings.Contains(err.Error(), "does not name both of the members it sequences") {
		t.Errorf("error %q should say why the order cannot be written back", err)
	}
}

// A `then` before a member the notation does not allow a succession in front of
// is a syntax error, so no graph is built from a model whose order is unclear.
func TestSuccessionOnNonUsageIsASyntaxError(t *testing.T) {
	for _, src := range []string{
		"package P {\n\tpart def Q {\n\t\tpart a;\n\t\tthen part def B;\n\t}\n}",
		"package P {\n\tpart def Q {\n\t\tpart a;\n\t\tthen package Inner { }\n\t}\n}",
		"package P {\n\tpart def Q {\n\t\tpart a;\n\t\tthen attribute x;\n\t}\n}",
	} {
		_, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		var syntax *export.SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("want a SyntaxError for %q, got %v", src, err)
		}
	}
}

// A qualified name identifies an element, so two members of one namespace
// sharing a name would merge into a single subject.
func TestDuplicateNameIsUnsupported(t *testing.T) {
	src := "package P {\n\tpart def A;\n\tpart def A;\n}"
	_, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError for a duplicate name, got %v", err)
	}
}

// Ownership that forms a cycle leaves no root to print from, which would
// otherwise write an empty document and report success.
func TestOwnershipCycleIsUnsupported(t *testing.T) {
	const turtle = `@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix elmt: <urn:sysmlv2:element:> .
elmt:A a sysml:Package ; sysml:declaredName "A" ; sysml:qualifiedName "A" ;
  sysml:owningNamespace elmt:B .
elmt:B a sysml:Package ; sysml:declaredName "B" ; sysml:qualifiedName "B" ;
  sysml:owningNamespace elmt:A .
`
	out, err := export.Convert("cycle.ttl", []byte(turtle), export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError for a containment cycle, got %v (output %q)", err, out)
	}
}

// A round trip through RDF keeps `doc` and `comment` because those are
// declarations, and lexical trivia because the source text carries it.
func TestCommentsThroughRDF(t *testing.T) {
	src := `package Demo {
	// a lexical line comment
	doc /* what this package is for */
	comment about Wheel /* a note on wheels */
	part def Wheel;
}`
	ttl, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", ttl, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("to sysml: %v", err)
	}
	got := string(back)
	for _, want := range []string{
		"doc /* what this package is for */",
		"comment about Wheel /* a note on wheels */",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("round trip dropped the declaration %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "// a lexical line comment") {
		t.Errorf("the source text did not carry the note through:\n%s", got)
	}
	// Only the source text carries trivia: the structural triples alone
	// convert to canonical notation, in which it has no place.
	stripped := toNotation(t, withoutTriples(t, ttl, "sysx:sourceText"))
	if strings.Contains(stripped, "a lexical line comment") {
		t.Errorf("trivia survived without source text:\n%s", stripped)
	}
	if !strings.Contains(stripped, "comment about Wheel /* a note on wheels */") {
		t.Errorf("the stripped graph dropped a comment declaration:\n%s", stripped)
	}
}

func TestVerbatimHeadsRoundTrip(t *testing.T) {
	src := `package Connections {
    part def Engine;
    part def Vehicle {
        part engine : Engine;
        part spare : Engine;
        connect engine to spare;
    }
}`
	turtle, err := export.Convert("conn.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if !strings.Contains(string(turtle), "sourceText") {
		t.Fatalf("expected the connect declaration to be carried as source text:\n%s", turtle)
	}
	back, err := export.Convert("conn.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "connect engine to spare") {
		t.Errorf("connect declaration lost:\n%s", back)
	}
}

// withoutTriples writes the graph again without the named property, given with
// its prefix: the head is then rebuilt from the mapping rather than read back
// from the text it was written as, the shape a graph from another tool has.
func withoutTriples(t *testing.T, turtle []byte, property string) []byte {
	t.Helper()
	var blocks []string
	for _, block := range strings.Split(string(turtle), "\n\n") {
		var kept []string
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), property+" ") {
				continue
			}
			kept = append(kept, line)
		}
		// Dropping the last triple of a block leaves the one before it to end it.
		for i := len(kept) - 1; i >= 0; i-- {
			if trimmed := strings.TrimRight(kept[i], "\n"); strings.HasSuffix(trimmed, " ;") {
				kept[i] = strings.TrimSuffix(trimmed, " ;") + " ."
				break
			} else if trimmed != "" {
				break
			}
		}
		blocks = append(blocks, strings.Join(kept, "\n"))
	}
	return []byte(strings.Join(blocks, "\n\n"))
}

// Every end-binding head states the form its ends are written in, so the
// declaration comes back as written from the mapping alone — without the source
// text that our own graphs also carry. Converting that notation again gives the
// original graph back, which is what proves the second hop loses nothing.
func TestEndBindingHeadsComeBackFromTheGraphAlone(t *testing.T) {
	heads := []string{
		"connect left to right;",
		"connect (left, right);",
		"connection c connect left to right;",
		"bind a = b;",
		"bind [0..1] a = [0..1] b;",
		"bind a = [1] b;",
		"binding ab bind [2] a = b;",
		"binding ab bind [0..1] a = [1..*] b;",
		"bind e1 ::> a = e2 references b;",
		"bind [1] e1 ::> a = b;",
		"binding ab bind e1 ::> a = e2 ::> b;",
		"connect [1] left to [0..1] right;",
		"connect ([1] left, [2] right);",
		"allocate a to b;",
		"flow left to right;",
		"flow of Bus from left to right;",
		"succession first left then right;",
		"satisfy R by v;",
		"verify R;",
	}
	for _, head := range heads {
		t.Run(head, func(t *testing.T) {
			src := "package P {\n    port def Bus;\n    requirement def R;\n    part v;\n    part def Car {\n        port left : Bus;\n        port right : Bus;\n        attribute a : Integer;\n        attribute b : Integer;\n        " + head + "\n    }\n}\n"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if !strings.Contains(string(back), head) {
				t.Fatalf("the head should come back as written:\n%s", back)
			}
			again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if string(again) != string(turtle) {
				t.Errorf("the second hop changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
			}
		})
	}
}

// A binding end's multiplicity is graph structure — bounds on the end node —
// so `bind [0..1] a = [0..1] b` keeps both ends' bounds without its source text.
func TestBindingEndMultiplicitiesAreStatedAsStructure(t *testing.T) {
	src := "package P {\n    part def Car {\n        attribute a : Integer;\n        attribute b : Integer;\n        bind [0..1] a = [0..1] b;\n    }\n}\n"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	for _, triple := range []string{
		"sysx:relatedFeature expr:P__Car___402_pend0, expr:P__Car___402_pend1 ;",
		"expr:P__Car___402_pend0\n    a sysml:FeatureReferenceExpression ;\n    sysx:sourceText \"a\" ;",
		"sysx:endIndex \"0\"^^xsd:integer ;\n    sysml:lowerBound expr:P__Car___402_pend0_plowerBound ;\n    sysml:upperBound expr:P__Car___402_pend0_pupperBound .",
		"sysx:endIndex \"1\"^^xsd:integer ;\n    sysml:lowerBound expr:P__Car___402_pend1_plowerBound ;\n    sysml:upperBound expr:P__Car___402_pend1_pupperBound .",
		"expr:P__Car___402_pend0_plowerBound\n    a sysml:LiteralInteger ;\n    sysx:sourceText \"0\" ;",
		"expr:P__Car___402_pend1_pupperBound\n    a sysml:LiteralInteger ;\n    sysx:sourceText \"1\" ;",
	} {
		if !strings.Contains(graph, triple) {
			t.Errorf("the graph should state %q:\n%s", triple, graph)
		}
	}
	for _, legacy := range []string{"sysml:value expr:", "sysml:references", "_pvalue"} {
		if strings.Contains(graph, legacy) {
			t.Errorf("a binding's ends are connector ends, not a reference and a value (%s):\n%s", legacy, graph)
		}
	}
	// Without the bounds the ends come back bare: the notation reads the graph.
	stripped := withoutTriples(t, turtle, "sysx:sourceText")
	for _, property := range []string{"sysml:lowerBound", "sysml:upperBound"} {
		stripped = withoutTriples(t, stripped, property)
	}
	back, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "bind a = b;") {
		t.Fatalf("ends without bounds should be written bare:\n%s", back)
	}
}

// A KerML connector written without `of`/`first` has no declaration, so a leading
// multiplicity is the first end's: bounds on the end node, not the connector.
func TestKerMLConnectorEndMultiplicitiesAreStatedAsStructure(t *testing.T) {
	const endNodes = "sysx:relatedFeature expr:P__C___402_pend0, expr:P__C___402_pend1 ;"
	cases := []struct {
		// head is the connector as written; bare is how it reads without bounds.
		head, bare string
		// bounds are the bound triples the graph must state; onConnector says
		// they hang on the connector node rather than on its ends.
		bounds      []string
		onConnector bool
	}{
		{
			head: "binding [1] a = [0..1] b;",
			bare: "binding a = b;",
			bounds: []string{
				"sysx:endIndex \"0\"^^xsd:integer ;\n    sysml:upperBound expr:P__C___402_pend0_pupperBound .",
				"sysx:endIndex \"1\"^^xsd:integer ;\n    sysml:lowerBound expr:P__C___402_pend1_plowerBound ;\n    sysml:upperBound expr:P__C___402_pend1_pupperBound .",
				"expr:P__C___402_pend0_pupperBound\n    a sysml:LiteralInteger ;\n    sysx:sourceText \"1\" ;",
				"expr:P__C___402_pend1_plowerBound\n    a sysml:LiteralInteger ;\n    sysx:sourceText \"0\" ;",
			},
		},
		{
			head: "binding [1] a = b;",
			bare: "binding a = b;",
			bounds: []string{
				"sysx:endIndex \"0\"^^xsd:integer ;\n    sysml:upperBound expr:P__C___402_pend0_pupperBound .",
			},
		},
		{
			head: "succession [1] a then [*] b;",
			bare: "succession a then b;",
			bounds: []string{
				"sysx:endIndex \"0\"^^xsd:integer ;\n    sysml:upperBound expr:P__C___402_pend0_pupperBound .",
				"sysx:endIndex \"1\"^^xsd:integer ;\n    sysml:upperBound expr:P__C___402_pend1_pupperBound .",
				"expr:P__C___402_pend1_pupperBound\n    a sysml:LiteralInfinity ;\n    sysx:sourceText \"*\" ;",
			},
		},
		{
			head:        "binding [1] of a = b;",
			bounds:      []string{"sysml:upperBound expr:P__C___402_pupperBound ;\n    sysx:relatedFeature"},
			onConnector: true,
		},
		{
			head:        "succession [1] first a then b;",
			bounds:      []string{"sysml:upperBound expr:P__C___402_pupperBound ;\n    sysx:relatedFeature"},
			onConnector: true,
		},
	}
	for _, c := range cases {
		t.Run(c.head, func(t *testing.T) {
			src := "package P {\n    class C {\n        step a;\n        step b;\n        " + c.head + "\n    }\n}\n"
			turtle, err := export.Convert("m.kerml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			graph := string(turtle)
			for _, triple := range append([]string{endNodes}, c.bounds...) {
				if !strings.Contains(graph, triple) {
					t.Errorf("the graph should state %q:\n%s", triple, graph)
				}
			}
			for _, bound := range []string{"pend0_plowerBound", "pend0_pupperBound", "pend1_plowerBound", "pend1_pupperBound"} {
				if c.onConnector && strings.Contains(graph, bound) {
					t.Errorf("a declared connector's multiplicity is not an end's:\n%s", graph)
				}
			}
			for _, bound := range []string{"expr:P__C___402_plowerBound", "expr:P__C___402_pupperBound"} {
				if !c.onConnector && strings.Contains(graph, bound) {
					t.Errorf("an undeclared connector has no multiplicity of its own:\n%s", graph)
				}
			}
			// A declared connector's own multiplicity is written after its ends,
			// which does not read back yet; only the end forms round-trip bare.
			if c.onConnector {
				return
			}
			// The bounds alone carry the multiplicities back into notation.
			back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if !strings.Contains(string(back), c.head) {
				t.Fatalf("the head should come back from the bounds:\n%s", back)
			}
			// Without the bounds the connector comes back bare.
			stripped := withoutTriples(t, turtle, "sysx:sourceText")
			for _, property := range []string{"sysml:lowerBound", "sysml:upperBound"} {
				stripped = withoutTriples(t, stripped, property)
			}
			back, err = export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation without bounds: %v", err)
			}
			if !strings.Contains(string(back), c.bare) {
				t.Fatalf("a connector without bounds should be written bare:\n%s", back)
			}
		})
	}
}

// An end-binding usage's body members are elements of their own and come back
// from the structure alone (Open-MBEE/OpenSysML#89).
func TestEndBindingBodiesComeBackFromTheGraphAlone(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "convert", "end_binding_bodies.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	turtle, err := export.Convert("m.sysml", src, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	for _, triple := range []string{
		"elmt:R89__Ctx__seam__coupling\n    a sysml:AttributeUsage ;",
		"sysml:declaredName \"coupling\" ;",
		"sysx:memberIndex \"0\"^^xsd:integer ;\n    sysml:owningNamespace elmt:R89__Ctx__seam ;",
		"sysml:ownedMember elmt:R89__Ctx__seam__coupling ;",
		"sysml:ownedFeature elmt:R89__Ctx__seam__coupling ;",
		"sysml:ownedMembership elmt:R89__Ctx__seam__coupling_om ;",
		"elmt:R89__Ctx__seam__coupling_om\n    a sysml:FeatureMembership ;",
		"sysx:sourceText \"        interface seam connect w.outp to r.inp {\\n\" ;",
	} {
		if !strings.Contains(graph, triple) {
			t.Errorf("the graph should carry the body of the interface usage:\nmissing %q in\n%s", triple, graph)
		}
	}
	if !strings.Contains(graph, "elmt:R89__Ctx__seam__coupling\n") || !strings.Contains(graph, "sysml:type elmt:R89__SeamCoupling") || !strings.Contains(graph, "SeamCoupling::learnFromData") {
		t.Errorf("the attribute should keep its type and value:\n%s", graph)
	}
	// The text of a usage stops at its body; the members are text of their own.
	if strings.Count(graph, "attribute w : Prio;") != 11 || strings.Contains(graph, "to r.inp {\\n            attribute coupling") {
		t.Errorf("the body should not be carried as its owner's text:\n%s", graph)
	}
	// Without the text, the notation is rebuilt from the mapping alone; it may
	// differ in layout from the notation the text writes, never in what it says.
	structural := withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:sourceTail")
	back, err := export.Convert("m.ttl", structural, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation without source text: %v", err)
	}
	withText, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if string(withText) != string(src) {
		t.Errorf("the text should write the model as written\n--- written ---\n%s\n--- got ---\n%s", src, withText)
	}
	if strings.Count(string(back), "attribute w : Prio;") != 11 || !strings.Contains(string(back), "interface seam connect w.outp to r.inp {\n            attribute coupling : SeamCoupling = SeamCoupling::learnFromData;\n        }") {
		t.Errorf("every body should come back with its members:\n%s", back)
	}
	for _, transition := range []string{
		"transition t1 first off then on {\n                attribute w : Prio;\n            }",
		"transition first on do assign x.a := 1 then off {\n            }",
		"transition first off do {\n                assign x.a := 2;\n            } then on {\n                attribute w : Prio;\n            }",
	} {
		if !strings.Contains(string(back), transition) {
			t.Errorf("a transition should keep its effect apart from its body:\nmissing %q in\n%s", transition, back)
		}
	}
	again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if got := withoutTriples(t, withoutTriples(t, again, "sysx:sourceText"), "sysx:sourceTail"); string(got) != string(structural) {
		t.Errorf("the second hop changed the graph\n--- first ---\n%s\n--- second ---\n%s", structural, got)
	}
}

// An empty `do { }` and an empty trailing body are transition blocks with no
// members to link, so the graph states each one's presence outright.
func TestEmptyTransitionBlocksComeBackFromTheGraphAlone(t *testing.T) {
	// Each transition with the canonical notation it is written back as.
	transitions := map[string]string{
		"transition first s1 do { } then s2;":    "transition first s1 do {\n        } then s2;",
		"transition first s1 then s2 { }":        "transition first s1 then s2 {\n        }",
		"transition first s1 do { } then s2 { }": "transition first s1 do {\n        } then s2 {\n        }",
	}
	for transition, want := range transitions {
		t.Run(transition, func(t *testing.T) {
			src := "package P {\n\tstate def M {\n\t\tstate s1;\n\t\tstate s2;\n\t\t" + transition + "\n\t}\n}"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			stripped := withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:sourceTail")
			back, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if !strings.Contains(string(back), want) {
				t.Errorf("expected %q in:\n%s", want, back)
			}
		})
	}
}

// A transition graph written before members were linked as effect or body
// owns the effect alone, with sysx:hasBody recording its braces. Such a graph
// still reads as the effect it was, braced or not.
func TestLegacyTransitionEffectsStayEffects(t *testing.T) {
	// Each effect with the canonical notation it is written back as.
	effects := map[string][2]string{
		"unbraced": {"do action stop : Warm", "transition first s1 do action stop : Warm then s2;"},
		"braced":   {"do { action stop : Warm; }", "transition first s1 do {\n            action stop : Warm;\n        } then s2;"},
	}
	for name, effect := range effects {
		effect, want := effect[0], effect[1]
		t.Run(name, func(t *testing.T) {
			src := "package P {\n\taction def Warm;\n\tstate def M {\n\t\tstate s1;\n\t\tstate s2;\n\t\ttransition first s1 " + effect + " then s2;\n\t}\n}"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			for _, property := range []string{"sysx:sourceText", "sysx:sourceTail", "sysx:effectMember", "sysx:bodyMember"} {
				turtle = withoutTriples(t, turtle, property)
			}
			legacy := strings.ReplaceAll(string(turtle), "sysx:bracedEffect ", "sysx:hasBody ")
			if !strings.Contains(legacy, "sysx:hasBody") {
				t.Fatalf("the legacy shape needs sysx:hasBody for the effect's braces:\n%s", legacy)
			}
			back, err := export.Convert("m.ttl", []byte(legacy), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if !strings.Contains(string(back), want) {
				t.Errorf("the effect did not come back as one:\n%s", back)
			}
			if strings.Contains(string(back), "then s2 {") {
				t.Errorf("the effect became a body:\n%s", back)
			}
		})
	}
}

// The effect and body links of a transition must partition its members: a graph
// whose links are missing, doubled or dangling is refused rather than have an
// action silently moved after the target.
func TestInconsistentTransitionLinksAreRefused(t *testing.T) {
	src := "package P {\n\taction def Warm;\n\tstate def M {\n\t\tstate s1;\n\t\tstate s2;\n\t\ttransition first s1 do { action stop : Warm; } then s2 { action tidy : Warm; }\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	const effect, body = "sysx:effectMember elmt:P__M___402__stop", "sysx:bodyMember elmt:P__M___402__tidy"
	if !strings.Contains(string(turtle), effect) || !strings.Contains(string(turtle), body) {
		t.Fatalf("the links are not the ones the test rewrites:\n%s", turtle)
	}
	faults := map[string]func(string) string{
		"missing effect link": func(g string) string { return string(withoutTriples(t, []byte(g), "sysx:effectMember")) },
		"missing body link":   func(g string) string { return string(withoutTriples(t, []byte(g), "sysx:bodyMember")) },
		"overlapping links": func(g string) string {
			return strings.Replace(g, body, "sysx:bodyMember elmt:P__M___402__stop", 1)
		},
		"dangling effect link": func(g string) string {
			return strings.Replace(g, effect, "sysx:effectMember elmt:P__M___402__gone", 1)
		},
		"dangling body link": func(g string) string {
			return strings.Replace(g, body, "sysx:bodyMember elmt:P__M___402__gone", 1)
		},
	}
	for name, fault := range faults {
		t.Run(name, func(t *testing.T) {
			var unsupported *export.UnsupportedError
			_, err := export.Convert("m.ttl", []byte(fault(string(turtle))), export.FormatTurtle, export.FormatSysML)
			if !errors.As(err, &unsupported) {
				t.Fatalf("got %v, want an UnsupportedError", err)
			}
			if !strings.Contains(err.Error(), "P__M___402") {
				t.Errorf("the error does not name the transition: %v", err)
			}
		})
	}
}

// A graph whose canonical notation does not parse gives the spelling of its
// references nothing to be checked against, so the conversion refuses rather
// than write them unchecked.
func TestUnreadableNotationRefusesToSpellReferences(t *testing.T) {
	src := "package P {\n\tpart def A;\n\tpart def B :> A;\n\tstate def M {\n\t\tstate s1;\n\t\tstate s2;\n\t\ttransition first s1 if true then s2;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	const guard = "sysx:guard expr:P__M___402_pguard ;"
	if !strings.Contains(string(turtle), guard) {
		t.Fatalf("the guard is not the one the test rewrites:\n%s", turtle)
	}
	// A guard kept as text is written as it is, and this one cannot be read.
	graph := strings.Replace(string(turtle), guard, `sysx:guard "(" ;`, 1)
	graph = string(withoutTriples(t, []byte(graph), "sysx:sourceText"))
	graph = string(withoutTriples(t, []byte(graph), "sysx:sourceTail"))
	var unsupported *export.UnsupportedError
	_, err = export.Convert("m.ttl", []byte(graph), export.FormatTurtle, export.FormatSysML)
	if !errors.As(err, &unsupported) {
		t.Fatalf("got %v, want an UnsupportedError", err)
	}
	if !strings.Contains(err.Error(), "does not parse") {
		t.Errorf("the error does not say the notation is unreadable: %v", err)
	}
}

// A transition and an accept state their trigger and payload in the head too,
// inside the bodies that allow them.
func TestBehavioralHeadsComeBackFromTheGraphAlone(t *testing.T) {
	bodies := map[string]string{
		"transition": "state def M {\n        state s1;\n        state s2;\n        transition first s1 accept e then s2;\n    }",
		"accept":     "action def A {\n        accept x : Bus;\n    }",
		"send":       "action def A {\n        send x to y;\n    }",
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			src := "package P {\n    port def Bus;\n    part x;\n    part y;\n    " + body + "\n}\n"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if string(again) != string(turtle) {
				t.Errorf("the head did not come back as written\n--- notation ---\n%s\n--- first ---\n%s\n--- second ---\n%s", back, turtle, again)
			}
		})
	}
}

// A `then` beside a member the notation leaves unnamed states its source end as
// that member, so it comes back as written where a name could not have said it.
func TestUnnamedSuccessionEndComesBackFromTheGraph(t *testing.T) {
	src := `package P {
	state def S;
	state def M {
		entry;
		then s1;
		state s1 : S;
	}
}`
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "then s1;") {
		t.Fatalf("the succession beside the unnamed entry should come back:\n%s", back)
	}
	again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if string(again) != string(turtle) {
		t.Errorf("the second hop changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
	}
	// The form and the member it names carry the succession without the text.
	fromGraph, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v", err)
	}
	if !strings.Contains(string(fromGraph), "then s1;") {
		t.Errorf("the mapping alone should say which member the `then` follows:\n%s", fromGraph)
	}
}

// A head's form is what its tokens say, not how they are laid out: line breaks,
// tabs and comments inside the head do not stop the graph from stating it.
func TestEndFormsSurviveIrregularLayout(t *testing.T) {
	heads := map[string]struct{ written, rebuilt string }{
		"connect": {"connect left\n\t\t\t/* to the */ to\n\t\t\tright;", "connect left to right;"},
		"bind":    {"bind\tleft\t=\tright;", "bind left = right;"},
		"satisfy": {"satisfy R // by the car\n\t\t\tby left;", "satisfy R by left;"},
		// The notes after a head are not part of it.
		"connect then note":   {"connect left to right; // and done", "connect left to right;"},
		"bind then note line": {"bind left = right;\n\t\t/* on to the next */", "bind left = right;"},
		"satisfy then note":   {"satisfy R by left; /* checked */", "satisfy R by left;"},
	}
	for name, head := range heads {
		t.Run(name, func(t *testing.T) {
			src := "package P {\n\trequirement def R;\n\tpart def Car {\n\t\tpart left;\n\t\tpart right;\n\t\t" + head.written + "\n\t}\n}\n"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			if !strings.Contains(string(turtle), "sysx:endForm") {
				t.Fatalf("the head's form should be stated:\n%s", turtle)
			}
			back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation from the mapping alone: %v", err)
			}
			if !strings.Contains(string(back), head.rebuilt) {
				t.Errorf("the head should be rebuilt as %q:\n%s", head.rebuilt, back)
			}
		})
	}
}

// A graph that relates ends but states no form for them is refused: the ends
// alone do not say which keyword and notation the head was written in.
func TestEndsWithoutTheirFormAreReported(t *testing.T) {
	src := "package P {\n\tpart def Car {\n\t\tpart left;\n\t\tpart right;\n\t\tconnect left to right;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	stripped := withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:endForm")
	_, err = export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err == nil {
		t.Fatal("a head whose form the graph does not state should be reported")
	}
	if !strings.Contains(err.Error(), "sysx:endForm") {
		t.Errorf("the report should name the property it needs: %v", err)
	}
}

func TestSyntaxErrorIsReported(t *testing.T) {
	_, err := export.Convert("bad.sysml", []byte("part def {"), export.FormatSysML, export.FormatTurtle)
	if err == nil {
		t.Fatal("expected a syntax error")
	}
	syntax, ok := err.(*export.SyntaxError)
	if !ok {
		t.Fatalf("expected a *export.SyntaxError, got %T: %v", err, err)
	}
	if len(syntax.Messages) == 0 {
		t.Error("expected at least one message")
	}
	if !strings.Contains(syntax.Error(), "bad.sysml") {
		t.Errorf("error should name the input: %v", syntax)
	}
}

func TestUnsupportedTurtleConstructs(t *testing.T) {
	const prefix = "@prefix sysml: <https://www.omg.org/spec/SysML#> .\n"
	cases := map[string]string{
		"blank node":       prefix + "_:x a sysml:Package .",
		"collection":       prefix + "<urn:x> sysml:client ( <urn:y> ) .",
		"unknown prefix":   "nope:x a nope:Thing .",
		"unterminated":     prefix + "<urn:x> a sysml:Package",
		"no rdf type":      prefix + "<urn:x> sysml:declaredName \"x\" .",
		"unterminated iri": prefix + "<urn:x a sysml:Package .",
		"missing owner":    prefix + "<urn:sysmlv2:element:A::B> a sysml:PartDefinition ; sysml:owningNamespace <urn:sysmlv2:element:A> .",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := export.Convert(name+".ttl", []byte(src), export.FormatTurtle, export.FormatSysML); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestUnknownMetaclassIsUnsupported(t *testing.T) {
	src := "@prefix sysml: <https://www.omg.org/spec/SysML#> .\n" +
		"<urn:sysmlv2:element:X> a sysml:NoSuchMetaclass ; sysml:declaredName \"X\" ."
	_, err := export.Convert("x.ttl", []byte(src), export.FormatTurtle, export.FormatSysML)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "NoSuchMetaclass") {
		t.Errorf("error should name the metaclass, got: %v", err)
	}
}

// TestForeignGraph covers a graph written by another tool: UUID element IRIs,
// no memberIndex, no hasBody, no sourceText, and links between elements only.
// Every name comes from a property; nothing is recovered from an IRI.
func TestForeignGraph(t *testing.T) {
	src := `@prefix sysml: <https://www.omg.org/spec/SysML#> .

<urn:uuid:aaaa-1> a sysml:Package ;
    sysml:declaredName "Demo" ;
    sysml:qualifiedName "Demo" .
<urn:uuid:aaaa-2> a sysml:PartDefinition ;
    sysml:declaredName "Engine" ;
    sysml:qualifiedName "Demo::Engine" ;
    sysml:owningNamespace <urn:uuid:aaaa-1> .
<urn:uuid:aaaa-3> a sysml:PartUsage ;
    sysml:declaredName "engine" ;
    sysml:qualifiedName "Demo::engine" ;
    sysml:owningNamespace <urn:uuid:aaaa-1> ;
    sysml:type <urn:uuid:aaaa-2> ;
    sysml:upperBound "1" .`
	out, err := export.Convert("foreign.ttl", []byte(src), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	got := strings.Join(strings.Fields(string(out)), " ")
	want := "package Demo { part def Engine; part engine : Engine[1]; }"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A graph whose referenced elements carry no sysml:qualifiedName cannot be
// written back: the name is never recovered from the IRI, so it is reported.
func TestForeignGraphWithoutQualifiedNamesIsReported(t *testing.T) {
	src := `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix elmt: <urn:sysmlv2:element:> .

elmt:Demo a sysml:Package ; sysml:declaredName "Demo" .
elmt:Demo__Engine a sysml:PartDefinition ;
    sysml:declaredName "Engine" ;
    sysml:owningNamespace elmt:Demo .`
	out, err := export.Convert("foreign.ttl", []byte(src), export.FormatTurtle, export.FormatSysML)
	if err == nil {
		t.Fatalf("a graph without qualified names converted to:\n%s", out)
	}
	if !strings.Contains(err.Error(), "sysml:qualifiedName") {
		t.Errorf("error %q should name the missing property", err)
	}
}

// A reference whose target cannot be named from the graph — an IRI that is not
// a subject, or a subject without sysml:qualifiedName — is reported, not named.
func TestUnnameableReferencesAreReported(t *testing.T) {
	const head = `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix elmt: <urn:sysmlv2:element:> .

elmt:P a sysml:Package ; sysml:declaredName "P" ; sysml:qualifiedName "P" .
elmt:P__u a sysml:PartUsage ; sysml:declaredName "u" ; sysml:qualifiedName "P::u" ;
    sysml:owningNamespace elmt:P ;
`
	for name, tail := range map[string]string{
		"reference to an IRI that is not a subject": `    sysml:type <urn:uuid:absent> .`,
		"reference to a subject without qualifiedName": `    sysml:type elmt:P__T .
elmt:P__T a sysml:PartDefinition ; sysml:declaredName "T" ; sysml:owningNamespace elmt:P .`,
	} {
		t.Run(name, func(t *testing.T) {
			out, err := export.Convert("m.ttl", []byte(head+tail), export.FormatTurtle, export.FormatSysML)
			if err == nil {
				t.Fatalf("an unnameable reference converted to:\n%s", out)
			}
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError, got %v", err)
			}
			if !strings.Contains(err.Error(), "sysml:qualifiedName") {
				t.Errorf("error %q should name the missing property", err)
			}
		})
	}
}

// A declaration built from a property the graph does not carry cannot be
// written: reporting it beats emitting notation that will not parse.
func TestMissingRequiredPropertyIsUnsupported(t *testing.T) {
	const head = `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix elmt: <urn:sysmlv2:element:> .

<urn:sysmlv2:element:P> a sysml:Package ; sysml:declaredName "P" ; sysml:qualifiedName "P" .
`
	for name, subject := range map[string]string{
		"alias without aliasedElement": `<urn:sysmlv2:element:P::X> a sysx:Alias ;
    sysml:declaredName "X" ; sysml:owningNamespace elmt:P .`,
		"dependency without supplier": `<urn:sysmlv2:element:P::D> a sysml:Dependency ;
    sysml:declaredName "D" ; sysml:owningNamespace elmt:P ; sysml:client "A" .`,
		"representation without language": `<urn:sysmlv2:element:P::R> a sysml:TextualRepresentation ;
    sysml:declaredName "R" ; sysml:owningNamespace elmt:P ; sysml:body "x" .`,
		"import without importedNamespace": `<urn:sysmlv2:element:P::I> a sysml:Import ;
    sysml:owningNamespace elmt:P .`,
		// `not` negates the declaration a prefix introduces (`assert not
		// constraint c`), so negation without one has no notation.
		"negation without declaredPrefix": `@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
<urn:sysmlv2:element:P::c> a sysml:ConstraintUsage ; sysml:declaredName "c" ;
    sysml:owningNamespace elmt:P ; sysml:isNegated "true"^^xsd:boolean .`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := export.Convert("m.ttl", []byte(head+subject), export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError, got %v", err)
			}
		})
	}
}

func TestElementIRIsEncodeQualifiedNames(t *testing.T) {
	graph, err := export.SysMLToRDF("iri.sysml", []byte("package P { part def Q; }"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	want := rdf.Element + rdf.EncodeElementID("P::Q")
	for _, subject := range graph.Subjects() {
		if subject.Value == want {
			return
		}
	}
	t.Errorf("expected subject %s in:\n%s", want, rdf.WriteTurtle(graph))
}

// Every element IRI in the convert fixtures is the encoding of the qualified
// name the element carries, and the encoding decodes back to that name.
func TestFixtureElementIDsRoundTrip(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "convert", "*.golden.ttl"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		graph, err := rdf.ParseTurtle(data)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, subject := range graph.Subjects() {
			if strings.HasPrefix(subject.Value, rdf.Expression) {
				// An expression node is named for the element and slot it
				// belongs to, not by a qualified name of its own.
				id := strings.TrimPrefix(subject.Value, rdf.Expression)
				owner, positions, ok := rdf.DecodeExpressionNodeID(id)
				if !ok || owner == "" || len(positions) == 0 {
					t.Errorf("%s: expression %s is not named for an element", path, subject.Value)
				}
				continue
			}
			if member, isMembership := rdf.DecodeOwningMembershipID(strings.TrimPrefix(subject.Value, rdf.Element)); isMembership {
				// A membership is named for the member it owns, which is the
				// only element it can belong to, and has no name of its own.
				owned, ok := graph.Object(subject, rdf.SysML+"memberElement")
				if !ok || owned.Value != rdf.Element+rdf.EncodeElementID(member) {
					t.Errorf("%s: membership %s does not own %q", path, subject.Value, member)
				}
				continue
			}
			qname, ok := graph.Lexical(subject, rdf.SysML+"qualifiedName")
			if !ok {
				t.Errorf("%s: subject %s has no qualified name", path, subject.Value)
				continue
			}
			if want := rdf.Element + rdf.EncodeElementID(qname); subject.Value != want {
				t.Errorf("%s: subject %s should be %s for %q", path, subject.Value, want, qname)
			}
			id := strings.TrimPrefix(subject.Value, rdf.Element)
			if got, ok := rdf.DecodeElementID(id); !ok || got != qname {
				t.Errorf("%s: id %q decoded to %q, %v, want %q", path, id, got, ok, qname)
			}
		}
	}
}

func TestFormatDetection(t *testing.T) {
	cases := map[string]export.Format{
		"model.sysml":      export.FormatSysML,
		"model.kerml":      export.FormatSysML,
		"model.ttl":        export.FormatTurtle,
		"dir/model.turtle": export.FormatTurtle,
		"Model.xmi":        export.FormatXMI,
		"Model.mdzip":      export.FormatXMI,
	}
	for path, want := range cases {
		got, err := export.FormatOfPath(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if got != want {
			t.Errorf("%s: got %v, want %v", path, got, want)
		}
	}
	if _, err := export.FormatOfPath("model.json"); err == nil {
		t.Error("expected an error for an unknown extension")
	}
	if _, err := export.FormatOfPath("model"); err == nil {
		t.Error("expected an error for a missing extension")
	}
	for _, name := range []string{"sysml", "SysML", "kerml", "ttl", " turtle ", "rdf", "xmi", "mdzip"} {
		if _, err := export.ParseFormat(name); err != nil {
			t.Errorf("ParseFormat(%q): %v", name, err)
		}
	}
	if _, err := export.ParseFormat("xml"); err == nil {
		t.Error("expected an error for an unknown format name")
	}
	if export.FormatXMI.Writable() || !export.FormatSysML.Writable() || !export.FormatTurtle.Writable() {
		t.Error("XMI is the one format that is read and never written")
	}
}

// TestConvertFromXMI checks the XMI entry into the conversion paths: v1 XMI
// migrates to notation and to Turtle, the report comes back from Migrate, and
// nothing writes XMI.
func TestConvertFromXMI(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "migrate", "testdata", "cameo", "vehicle.xmi"))
	if err != nil {
		t.Fatal(err)
	}
	notation, report, err := export.Migrate("vehicle.xmi", data, export.FormatSysML)
	if err != nil {
		t.Fatalf("Migrate to notation: %v", err)
	}
	if report == nil || len(report.Entries) == 0 {
		t.Fatal("Migrate returned no report")
	}
	if !strings.Contains(string(notation), "part def Vehicle") {
		t.Errorf("migrated notation lacks the Vehicle block:\n%s", notation)
	}
	if again, err := export.Convert("vehicle.xmi", data, export.FormatXMI, export.FormatSysML); err != nil || string(again) != string(notation) {
		t.Errorf("Convert from XMI differs from Migrate: %v", err)
	}

	turtle, _, err := export.Migrate("vehicle.xmi", data, export.FormatTurtle)
	if err != nil {
		t.Fatalf("Migrate to Turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("migrated Turtle does not parse: %v", err)
	}
	if len(graph.Triples()) == 0 {
		t.Error("migrated Turtle is empty")
	}

	var notWritable *export.NotWritableError
	if _, err := export.Convert("model.sysml", []byte("package P;"), export.FormatSysML, export.FormatXMI); !errors.As(err, &notWritable) {
		t.Errorf("writing XMI: got %v, want a NotWritableError", err)
	}
	if _, _, err := export.Migrate("vehicle.xmi", data, export.FormatXMI); !errors.As(err, &notWritable) {
		t.Errorf("migrating to XMI: got %v, want a NotWritableError", err)
	}
	if _, err := export.Convert("model.sysml", []byte("package P;"), export.FormatXMI, export.FormatSysML); err == nil {
		t.Error("notation read as XMI was accepted")
	}
}

func TestEmptyInputs(t *testing.T) {
	if _, err := export.ToSysML(nil); err == nil {
		t.Error("expected an error for a nil graph")
	}
	if _, err := export.ToRDF(nil, nil); err == nil {
		t.Error("expected an error for a nil document")
	}
}

// A graph written before the rename carries the old extension namespace, whose
// properties this version would read as absent; it must be refused instead.
func TestLegacyExtensionNamespaceIsRefused(t *testing.T) {
	for name, src := range map[string]string{
		"property": "@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .\n" +
			"@prefix sysml: <https://www.omg.org/spec/SysML#> .\n" +
			"@prefix elmt: <urn:sysmlv2:element:> .\n" +
			"@prefix sysx: <urn:systemica:sysml:> .\n" +
			"@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .\n" +
			"elmt:Demo rdf:type sysml:Package ; sysml:declaredName \"Demo\" ;\n" +
			"    sysx:memberIndex \"0\"^^xsd:integer .\n",
		"metaclass": "@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .\n" +
			"@prefix sysml: <https://www.omg.org/spec/SysML#> .\n" +
			"@prefix elmt: <urn:sysmlv2:element:> .\n" +
			"@prefix sysx: <urn:systemica:sysml:> .\n" +
			"elmt:Demo rdf:type sysx:InitialNode ; sysml:declaredName \"Demo\" .\n",
	} {
		graph, err := rdf.ParseTurtle([]byte(src))
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		_, err = export.ToSysML(graph)
		if err == nil {
			t.Fatalf("%s: expected the legacy namespace to be refused", name)
		}
		if !strings.Contains(err.Error(), rdf.LegacyExtension) {
			t.Errorf("%s: error does not name the legacy namespace: %v", name, err)
		}
	}
}

// The fixture is 0.4.3's own output: `sysx:prefixMetadata` for a `#` prefix and
// `sysml:annotates` for an `about` target, neither of which this version reads.
func TestSupersededMetadataPredicatesAreRefused(t *testing.T) {
	turtle, err := os.ReadFile(filepath.Join("testdata", "superseded", "metadata_0_4_3.ttl"))
	if err != nil {
		t.Fatal(err)
	}
	refused := func(turtle []byte, property string) {
		t.Helper()
		_, err := export.Convert("old.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Fatalf("expected the graph to be refused for %s, got %v", property, err)
		}
		for _, want := range []string{"the property <" + property + ">", "an earlier version wrote this fact this way", "convert the model from source again"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("expected %q in error:\n%s", want, err.Error())
			}
		}
	}
	refused(turtle, rdf.OpenSysML+"prefixMetadata")
	withoutPrefix := withoutTriples(t, turtle, "sysx:prefixMetadata")
	refused(withoutPrefix, rdf.SysML+"annotates")

	// Without the superseded properties the graph converts, so the refusal is
	// the only thing standing between the graph and a silently dropped annotation.
	back, err := export.Convert("old.ttl", withoutTriples(t, withoutPrefix, "sysml:annotates"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, unwanted := range []string{"#Safety", "about"} {
		if strings.Contains(string(back), unwanted) {
			t.Errorf("%q should not come from a graph that no longer states it:\n%s", unwanted, back)
		}
	}
}

// The fixture is 0.5.1's own output: `sysml:isSnapshot` and `sysml:isTimeslice`
// for a portion, which `sysml:portionKind` now states.
func TestSupersededPortionFlagsAreRefused(t *testing.T) {
	turtle, err := os.ReadFile(filepath.Join("testdata", "superseded", "portions_0_5_1.ttl"))
	if err != nil {
		t.Fatal(err)
	}
	refused := func(turtle []byte, property, now string) {
		t.Helper()
		_, err := export.Convert("old.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Fatalf("expected the graph to be refused for %s, got %v", property, err)
		}
		for _, want := range []string{"the property <" + property + ">", "it is now " + now, "convert the model from source again"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("expected %q in error:\n%s", want, err.Error())
			}
		}
	}
	refused(turtle, rdf.SysML+"isSnapshot", `sysml:portionKind "snapshot"`)
	withoutSnapshot := withoutTriples(t, turtle, "sysml:isSnapshot")
	refused(withoutSnapshot, rdf.SysML+"isTimeslice", `sysml:portionKind "timeslice"`)

	// With both flags gone the keyword contradicts the typing: still refused.
	_, err = export.Convert("old.ttl", withoutTriples(t, withoutSnapshot, "sysml:isTimeslice"), export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected the untyped portion to be refused, got %v", err)
	}
	for _, want := range []string{"the `snapshot` declaration <urn:sysmlv2:element:Portions__atStart>", "no sysml:portionKind"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in error:\n%s", want, err.Error())
		}
	}
}

// modelFiles lists the notation fixtures in testdata/convert, `.sysml` and
// `.kerml` alike; the goldens beside them are not models.
func modelFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, pattern := range []string{"*.sysml", "*.kerml"} {
		paths, err := filepath.Glob(filepath.Join("testdata", "convert", pattern))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			if strings.Contains(path, ".golden.") {
				continue
			}
			out = append(out, path)
		}
	}
	if len(out) == 0 {
		t.Fatal("no models in testdata/convert")
	}
	return out
}

// fixtureName is a fixture's stem, and its extension: the notation it is written in.
func fixtureName(path string) (name, ext string) {
	ext = filepath.Ext(path)
	return strings.TrimSuffix(filepath.Base(path), ext), ext
}

func checkGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if string(got) != string(want) {
		t.Errorf("%s differs\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}

// A save replaces the previous file only once the new bytes are safely written,
// and the result is an ordinary readable document.
func TestWriteFileIsAtomicAndReadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.sysml")

	replaced, err := export.WriteFile(path, []byte("package P;\n"))
	if err != nil || replaced {
		t.Fatalf("first write: replaced=%v err=%v", replaced, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("mode = %o, want 644", got)
	}
	replaced, err = export.WriteFile(path, []byte("package Q;\n"))
	if err != nil || !replaced {
		t.Fatalf("second write: replaced=%v err=%v", replaced, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package Q;\n" {
		t.Errorf("content = %q", data)
	}
	// No temporary file is left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("leftover files in %s: %v", dir, entries)
	}
}

// A missing parent directory is named rather than surfacing as a bare open(2)
// failure.
func TestWriteFileNamesTheMissingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "absent")
	_, err := export.WriteFile(filepath.Join(dir, "model.sysml"), []byte("package P;\n"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), dir) || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("unhelpful error: %v", err)
	}
}

// The REPL's tolerant save writes notation it could not fully parse and reports
// the syntax errors; every other direction still refuses.
func TestConvertTolerant(t *testing.T) {
	broken := []byte("package P { part x; }\npart 3x;\n")
	out, syntax, err := export.ConvertTolerant("<session>", broken, export.FormatSysML, export.FormatSysML)
	if err != nil {
		t.Fatalf("sysml to sysml: %v", err)
	}
	if syntax == nil {
		t.Error("expected the syntax errors to be reported")
	}
	if !strings.Contains(string(out), "part 3x;") {
		t.Errorf("the unreadable text was dropped:\n%s", out)
	}
	if _, _, err := export.ConvertTolerant("<session>", broken, export.FormatSysML, export.FormatTurtle); err == nil {
		t.Error("Turtle should still refuse a broken model")
	}
}

// Saving is an edit of the user's file, so a model they had kept private does
// not become world-readable because they saved it again.
func TestWriteFileKeepsExistingPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private.sysml")
	if err := os.WriteFile(path, []byte("package P;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := export.WriteFile(path, []byte("package Q;\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}

// A pipe or a device is a stream, not a file with contents to protect, so it is
// written as it stands rather than replaced by a rename.
func TestWriteFileWritesThroughAPipe(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "pipe.sysml")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	read := make(chan string, 1)
	go func() {
		data, err := os.ReadFile(fifo)
		if err != nil {
			t.Error(err)
		}
		read <- string(data)
	}()
	replaced, err := export.WriteFile(fifo, []byte("package Q;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if replaced {
		t.Error("a pipe is not an existing file that was replaced")
	}
	if got := <-read; got != "package Q;\n" {
		t.Errorf("read %q from the pipe", got)
	}
	if info, err := os.Stat(fifo); err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("the pipe was replaced by a regular file (%v)", err)
	}
}

// An existing file inside a directory the user cannot add entries to is still
// written: the temporary file is impossible there, but the save is not.
func TestWriteFileFallsBackWhenTheDirectoryIsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "closed")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(path, []byte("package Longer;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0o700) }()
	replaced, err := export.WriteFile(path, []byte("package Q;\n"))
	if err != nil || !replaced {
		t.Fatalf("replaced=%v err=%v", replaced, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package Q;\n" {
		t.Errorf("file = %q, want the new model with nothing of the old one left", data)
	}
}

// A symlink is a pointer to the model, so saving over it updates the model
// rather than replacing the link with a regular file.
func TestWriteFileWritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.sysml")
	link := filepath.Join(dir, "link.sysml")
	if err := os.WriteFile(real, []byte("package P;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	replaced, err := export.WriteFile(link, []byte("package Q;\n"))
	if err != nil || !replaced {
		t.Fatalf("replaced=%v err=%v", replaced, err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the symlink was replaced by a regular file (%v)", err)
	}
	data, err := os.ReadFile(real)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package Q;\n" {
		t.Errorf("the linked model was not updated: %q", data)
	}
}

// A failed save names the file the user asked for, never the temporary file
// this package made up.
func TestWriteFileErrorNamesTheRequestedPath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "model.sysml")
	_, err := export.WriteFile(path, []byte("package P;\n"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error does not name %s: %v", path, err)
	}
	if strings.Contains(err.Error(), ".model.sysml.") {
		t.Errorf("error leaks the temporary file: %v", err)
	}
}

// A reference reached through an import, an alias, a nested package path, a
// feature chain or a redefinition across two generals links to the element it
// names, and that link alone — without the text it was written as — brings the
// notation back in the shortest spelling that reaches the element again, and
// the same graph after it. The fixture pairs each linked reference with a
// same-named declaration nearer the writer, so a link that went to the wrong
// element would surface here. A filter condition names the metadata its own
// import filters by.
func TestLinkedReferencesCarryTheRoundTripWithoutSourceText(t *testing.T) {
	path := filepath.Join("testdata", "convert", "imported_references.sysml")
	turtle := toTurtle(t, path)
	for _, want := range []string{
		"sysml:type elmt:OtherPkg__BudgetLedger ;",
		"sysml:type elmt:OtherPkg__Tempo ;",
		"sysml:referent elmt:OtherPkg__Tempo__operative .",
		"sysml:references elmt:OtherPkg__spare ;",
		"sysml:targetFeature elmt:OtherPkg__Inner__Wheel__size .",
		"sysml:redefines elmt:R90__G2__x ;",
		"sysml:targetFeature elmt:R90__Done__done .",
		`sysml:type "Elsewhere::Missing" ;`,
		"sysx:typeArgument elmt:Meta__Safety .",
		"sysml:type elmt:Meta__Tagged ;",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph should carry %q\n%s", want, turtle)
		}
	}
	back := backFromTheGraphAlone(t, turtle)
	for _, want := range []string{
		"item b2 : BudgetLedger;",
		"item b3 : BudgetLedger;",
		"attribute t : Tempo = Tempo::operative;",
		"part w : Inner::Wheel subsets Inner::wheels;",
		"ref spare references spare;",
		"attribute s = w.size;",
		"part redefines G2::x;",
		"part : Inner::Wheel redefines w;",
		"transition idle then Done::done;",
		"part unresolved : Elsewhere::Missing;",
		"public import Meta::* [(@ Safety)];",
		"attribute other : Tagged;",
	} {
		if !strings.Contains(back, want) {
			t.Errorf("the notation should read %q\n%s", want, back)
		}
	}
}

// The pilot corpora's three `connector <end> to <end>;` shapes (KerML.xtext:836)
// declare no name: the graph relates the ends' features and writes them back alone.
func TestKerMLBinaryConnectorEndsCarryTheRoundTripWithoutSourceText(t *testing.T) {
	src := `package Corpus {
	class V6Engine;
	class FuelTank;
	class A { feature x; }
	class B;
	class Vehicle {
		feature eng : V6Engine;
		feature tanks : FuelTank { feature main1 : FuelTank; }
		feature a : A;
		feature b : B;
		feature transitionLink[0..1];
		feature trigger[1..*];
		connector eng to tanks.main1;
		connector a ::> a.x to b;
		private connector [0..1] transitionLink to [1..*] trigger;
	}
}
`
	turtle, err := export.Convert("corpus.kerml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	for _, want := range []string{
		"sysx:relatedFeature expr:Corpus__Vehicle___406_pend0, expr:Corpus__Vehicle___406_pend1 ;",
		"expr:Corpus__Vehicle___406_pend0\n    a sysml:FeatureReferenceExpression ;\n    sysx:sourceText \"eng\" ;\n    sysml:elementId \"Corpus__Vehicle___406_pend0\" ;\n    sysml:referent elmt:Corpus__Vehicle__eng ;",
		"expr:Corpus__Vehicle___407_pend0\n    a sysml:FeatureChainExpression ;\n    sysx:sourceText \"a.x\" ;",
		"sysml:targetFeature elmt:Corpus__A__x ;\n    sysx:endIndex \"0\"^^xsd:integer ;\n    sysx:endName \"a\" .",
		"sysml:referent elmt:Corpus__Vehicle__transitionLink ;\n    sysx:endIndex \"0\"^^xsd:integer ;\n    sysml:lowerBound expr:Corpus__Vehicle___408_pend0_plowerBound ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should carry %q\n%s", want, graph)
		}
	}
	for _, name := range []string{"eng", "a", "transitionLink"} {
		if strings.Contains(graph, "elmt:Corpus__Vehicle__"+name+"\n    a sysml:ConnectorAsUsage ;") {
			t.Errorf("the connector took the end %s as its name\n%s", name, graph)
		}
	}
	back := string(structuralRoundTrip(t, "corpus.kerml", turtle))
	for _, want := range []string{
		"connector eng to tanks.main1;",
		"connector a ::> a.x to b;",
		"private connector [0..1] transitionLink to [1..*] trigger;",
	} {
		if !strings.Contains(back, want) {
			t.Errorf("the notation should read %q\n%s", want, back)
		}
	}
}

// A comment in a head spelling a verb (`connect /* from */ a to b`) is not the
// verb: the head's form is read from its tokens, so the graph still states it.
func TestEndVerbsInCommentsAreNotVerbs(t *testing.T) {
	src := `package Comments {
	class T { feature eng; feature t; feature u; }
	class V :> T {
		connector /* from */ eng to t;
		connector /* ( */ [1] eng to /* allocate */ [0..1] u;
		connector link /* to */ from eng to u;
	}
	part def P { part a; part b; part c; }
	part p : P {
		connect /* from */ a to b;
		connect /* ( */ (a, b, c);
		binding /* of */ bind a = b;
	}
}
`
	turtle, err := export.Convert("comments.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if n := strings.Count(string(turtle), "sysx:endForm "); n != 6 {
		t.Errorf("the graph should state all 6 heads' forms, got %d\n%s", n, turtle)
	}
	back := backFromTheGraphAlone(t, string(turtle))
	for _, want := range []string{
		"connector eng to t;",
		"connector [1] eng to [0..1] u;",
		"connector link from eng to u;",
		"connect a to b;",
		"connect (a, b, c);",
		"binding bind a = b;",
	} {
		if !strings.Contains(back, want) {
			t.Errorf("the notation should read %q\n%s", want, back)
		}
	}
}

// backFromTheGraphAlone writes turtle back to notation without its source text,
// and checks that notation yields the same structural graph again. The notation
// is canonical spelling, so the graphs are compared as sets of triples.
func backFromTheGraphAlone(t *testing.T, turtle string) string {
	t.Helper()
	stripped := withoutTriples(t, withoutTriples(t, []byte(turtle), "sysx:sourceText"), "sysx:sourceTail")
	back, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v", err)
	}
	again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	first := structuralTriples(t, []byte(turtle))
	second := structuralTriples(t, again)
	for triple := range first {
		if !second[triple] {
			t.Errorf("the second hop lost %s %s %s", triple.Subject.Value, triple.Predicate.Value, triple.Object.Value)
		}
	}
	for triple := range second {
		if !first[triple] {
			t.Errorf("the second hop added %s %s %s", triple.Subject.Value, triple.Predicate.Value, triple.Object.Value)
		}
	}
	return string(back)
}

// A loop's condition is read in the loop's own scope, so it links the actions
// its body declares; a `while` condition reaches the enclosing body's members too.
func TestLoopConditionsLinkTheLoopBodysActions(t *testing.T) {
	turtle := toTurtle(t, filepath.Join("testdata", "convert", "loop_scopes.sysml"))
	for _, want := range []string{
		"sysml:referent elmt:Loops__Charge___401__charging ;",
		"sysml:referent elmt:Loops__Charge__prep ;",
		"sysml:referent elmt:Loops__Charge___402__pace ;",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph should carry %q\n%s", want, turtle)
		}
	}
	back := backFromTheGraphAlone(t, turtle)
	for _, want := range []string{"} until charging.done;", "while prep.done {", "} until pace.done;"} {
		if !strings.Contains(back, want) {
			t.Errorf("the notation should read %q\n%s", want, back)
		}
	}
}

// A transition or initial `then` names a vertex of the whole machine: one in a
// nested state, or in a sibling region, links to that vertex and comes back in
// a spelling that reaches it again.
func TestMachineEndpointsLinkAcrossRegionsAndNesting(t *testing.T) {
	turtle := toTurtle(t, filepath.Join("testdata", "convert", "endpoint_scopes.sysml"))
	for _, want := range []string{
		"sysml:targetFeature elmt:Machines__Lamp__on__heat__warm .",
		"sysml:targetFeature elmt:Machines__Lamp__on__light__bright .",
		"sysml:targetFeature elmt:Machines__Lamp__on__heat__hot .",
		"sysml:sourceFeature elmt:Machines__Lamp__on__heat__hot ;",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph should carry %q\n%s", want, turtle)
		}
	}
	if strings.Contains(turtle, `sysml:sourceFeature "`) || strings.Contains(turtle, `sysml:targetFeature "`) {
		t.Errorf("every endpoint names a vertex of the machine, so none should stay a literal\n%s", turtle)
	}
	backFromTheGraphAlone(t, turtle)
}

// A body expression's parameters are names of its body alone: a result naming
// one is not linked to a same-named import, and a reference outside the body
// still reaches that import — whichever body the element holds.
func TestBodyParametersShadowOnlyInsideTheirBody(t *testing.T) {
	turtle := toTurtle(t, filepath.Join("testdata", "convert", "body_scopes.sysml"))
	for _, want := range []string{
		`sysml:referent "limit" .`,
		`sysml:referent "Gauge" .`,
		"sysml:referent elmt:Lib__limit .",
		"sysml:type elmt:Lib__Gauge ;",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph should carry %q\n%s", want, turtle)
		}
	}
	if strings.Contains(turtle, "elmt:Lib__Gauge .") {
		t.Errorf("the body parameter Gauge must not link to Lib::Gauge\n%s", turtle)
	}
}

// structuralTriples is the set of a graph's triples without the source text.
func structuralTriples(t *testing.T, turtle []byte) map[rdf.Triple]bool {
	t.Helper()
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	out := map[rdf.Triple]bool{}
	for _, triple := range g.Triples() {
		if triple.Predicate == rdf.OpenSysMLTerm("sourceText") || triple.Predicate == rdf.OpenSysMLTerm("sourceTail") {
			continue
		}
		out[triple] = true
	}
	return out
}

// A body parameter, a loop variable or a trigger parameter that shadows an
// outer feature of the same name is no element of the graph, so the reference
// stays a name rather than linking the feature it hides.
func TestShadowingParametersStayNames(t *testing.T) {
	// A body declaring its parameter is written from its notation alone, so
	// that fixture is checked in the graph only.
	body := toTurtle(t, filepath.Join("testdata", "convert", "shadowing_body.sysml"))
	if want := "sysml:referent elmt:ShadowBody__Sensor__readings ."; !strings.Contains(body, want) {
		t.Errorf("the graph should carry %q\n%s", want, body)
	}
	if wrong := "sysml:referent elmt:ShadowBody__Sensor__value"; strings.Contains(body, wrong) {
		t.Errorf("the graph should not link %q\n%s", wrong, body)
	}
	if want := `sysml:referent "value" ;`; !strings.Contains(body, want) {
		t.Errorf("the graph should carry %q\n%s", want, body)
	}

	turtle := toTurtle(t, filepath.Join("testdata", "convert", "shadowing_parameters.sysml"))
	for _, want := range []string{
		`sysml:referent "value" ;`,
		`sysml:referent "value" .`,
		`sysml:referent "w" ;`,
		"sysml:referent elmt:Shadows__Sweep__items .",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph should carry %q\n%s", want, turtle)
		}
	}
	for _, wrong := range []string{
		"sysml:referent elmt:Shadows__Sweep__value",
		"sysml:referent elmt:Shadows__Governor__value",
		"sysml:referent elmt:Shadows__Governor__w",
	} {
		if strings.Contains(turtle, wrong) {
			t.Errorf("the graph should not link %q\n%s", wrong, turtle)
		}
	}
	back := backFromTheGraphAlone(t, turtle)
	for _, want := range []string{
		"accept setSpeed(value) if value > 0 then fast;",
		"accept w : Speed if w > 0 then idle;",
		"attribute cur = value;",
	} {
		if !strings.Contains(back, want) {
			t.Errorf("the notation should read %q\n%s", want, back)
		}
	}
}

// A chain member the operand's type inherits from two generals under one name
// comes back as that name where the name reads as the linked one, and qualified
// where it reads as the other — behind a nested chain too; one reached through
// a subsetting comes back as its name.
func TestChainMembersAreQualifiedOnlyWhereTheirNameReadsAsAnother(t *testing.T) {
	turtle := toTurtle(t, filepath.Join("testdata", "convert", "chain_scopes.sysml"))
	for _, want := range []string{
		"sysml:targetFeature elmt:Gen__G1__x .",
		"sysml:targetFeature elmt:Gen__G2__x .",
		"sysml:targetFeature elmt:Gen__G1__y .",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph should carry %q\n%s", want, turtle)
		}
	}
	back := backFromTheGraphAlone(t, turtle)
	for _, want := range []string{"attribute a = w.x;", "attribute b = w.Gen::G2::x;", "attribute c = w.y;", "attribute d = axle.hub.Gen::G2::x;", "attribute e = axle.hub.y;", "attribute f = spare.y;"} {
		if !strings.Contains(back, want) {
			t.Errorf("the notation should read %q\n%s", want, back)
		}
	}
}
