package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// notationFromTheGraphAlone converts src to Turtle, strips the source text and
// converts back, then converts once more and requires the structural triples to
// match. The sourced path is the negative control: it replays src byte for byte.
func notationFromTheGraphAlone(t *testing.T, name, src string) string {
	t.Helper()
	turtle, err := export.Convert(name, []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	sourced, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back with source text: %v", err)
	}
	if string(sourced) != src {
		t.Errorf("the sourced path must replay the source\n got: %s\nwant: %s", sourced, src)
	}
	stripped := withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:sourceTail")
	back, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back from the graph alone: %v", err)
	}
	again, err := export.Convert(name, back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v\n%s", err, back)
	}
	first := structuralTriples(t, stripped)
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
	if t.Failed() {
		t.Logf("notation from the graph alone:\n%s", back)
	}
	return string(back)
}

func wantFragments(t *testing.T, text string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			t.Errorf("missing %q in\n%s", fragment, text)
		}
	}
}

// An anonymous connector's own multiplicity is its declaration and is written
// ahead of the ends, where the grammar reads it as the connector's; the `[1]`s
// of a `bind` shorthand are both end multiplicities and stay on their ends.
func TestConnectorOwnMultiplicityPrecedesTheEnds(t *testing.T) {
	src := `package S {
    part def D {
        part a;
        part b;
        attribute n : Natural = 1;
        succession [n] first [0..1] a then [0..1] b;
        bind [1] a = [1] b;
    }
}
`
	back := notationFromTheGraphAlone(t, "s.sysml", src)
	wantFragments(t, back,
		"succession [n] first [0..1] a then [0..1] b;",
		"bind [1] a = [1] b;")
}

// In SysML the `bind` shorthand declares nothing, so a binding whose own
// bounds the graph states is written `binding [1] bind a = b`, never `bind [1]`,
// which the parser reads as the first end's multiplicity.
func TestSysMLBindingOwnMultiplicityTakesTheDeclaredForm(t *testing.T) {
	src := `package S {
    part def D {
        part a;
        part b;
        binding [1] bind a = b;
        binding bb [2] bind a = b;
    }
}
`
	back := notationFromTheGraphAlone(t, "s.sysml", src)
	wantFragments(t, back, "binding [1] bind a = b;", "binding bb[2] bind a = b;")
	// A graph from another tool may state the bounds on a `bind` and no verb.
	turtle, err := export.Convert("s.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	for _, property := range []string{"sysx:sourceText", "sysx:sourceTail", "sysx:endVerb"} {
		turtle = withoutTriples(t, turtle, property)
	}
	// The anonymous binding is the first end-binding head; state it as a `bind`.
	turtle = []byte(strings.Replace(string(turtle), `sysx:endForm "equals" ;`, "sysx:endForm \"equals\" ;\n    sysx:declaredKeyword \"bind\" ;", 1))
	back2, err := export.Convert("s.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back from a `bind` with its own bounds: %v", err)
	}
	if strings.Contains(string(back2), "bind [") {
		t.Errorf("`bind [n]` hands the bounds to the first end:\n%s", back2)
	}
	wantFragments(t, string(back2), "binding [1] bind a = b;", "binding bb[2] bind a = b;")
	again, err := export.Convert("s.sysml", back2, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(again), "sysml:upperBound expr:"); n != 2 {
		t.Errorf("want the two bindings to keep their own bounds, got %d\n%s", n, again)
	}
	if strings.Contains(string(again), "_pend0_pupperBound") {
		t.Errorf("the bounds moved onto the first end\n%s", again)
	}
}

// In KerML a leading `[1]` without `of`/`first` is the first end's cross
// multiplicity, so a connector multiplicity is always followed by the verb.
func TestKerMLConnectorOwnMultiplicityWritesTheVerb(t *testing.T) {
	src := `package K {
    class C {
        feature a;
        feature b;
        binding [1] of a = [1] b;
        succession [1] first a then [0..1] b;
    }
}
`
	back := notationFromTheGraphAlone(t, "k.kerml", src)
	wantFragments(t, back,
		"binding [1] of a = [1] b;",
		"succession [1] first a then [0..1] b;")
}

// An operand that binds more loosely than the operator around it is written in
// parentheses; one that binds as tightly or tighter is not.
func TestLoosePrecedenceOperandsAreParenthesized(t *testing.T) {
	src := `package P {
    part def D {
        attribute ae : Integer[0..*];
        attribute af : Integer[0..*];
        attribute edges : Integer[0..*];
        attribute p : Boolean;
        attribute q : Boolean;
        assert constraint { size(ae) == (if isEmpty(af) ? 0 else 2) and size(edges) == (if isEmpty(af) ? 2 else 4) }
        attribute i = if isEmpty(af) ? size(ae) else size(edges) + 1;
        attribute l = if (if isEmpty(af) ? p else q) ? 1 else 2;
        attribute c = (p ?? q) implies (p xor q);
        attribute d = (p implies q) == (q or p) and (p ?? q) != q;
        attribute e = (edges as Integer)->size() + (p istype Boolean) as Integer;
        attribute f = (p hastype Boolean) or (edges @ Integer) == edges;
        attribute g = - (1 + 2) ** 2 - (- 3) ** (- 1);
        attribute h = (ae + edges)[1] + ae#(edges[2] - 1);
        attribute k = not (p and q) and not p;
        attribute m = (1 .. 2) == (3 .. 4) and 1 .. 2 + 3 == null;
        attribute mt = (D meta SysML::Usage) == null and D meta SysML::Usage == null;
    }
}
`
	back := notationFromTheGraphAlone(t, "p.sysml", src)
	wantFragments(t, back,
		"size(ae) == (if isEmpty(af) ? 0 else 2) and size(edges) == (if isEmpty(af) ? 2 else 4)",
		"attribute i = if isEmpty(af) ? size(ae) else size(edges) + 1;",
		"attribute l = if (if isEmpty(af) ? p else q) ? 1 else 2;",
		"attribute c = (p ?? q) implies p xor q;",
		"attribute d = (p implies q) == (q or p) and (p ?? q) != q;",
		"attribute e = (edges as Integer)->size() + (p istype Boolean) as Integer;",
		"attribute f = p hastype Boolean or edges @ Integer == edges;",
		"attribute g = - (1 + 2) ** 2 - - 3 ** - 1;",
		"attribute h = (ae + edges)[1] + ae#(edges[2] - 1);",
		"attribute k = not (p and q) and not p;",
		"attribute m = 1 .. 2 == 3 .. 4 and 1 .. 2 + 3 == null;",
		"attribute mt = D meta SysML::Usage == null and D meta SysML::Usage == null;")
}

// A guarded succession keeps `succession` and `first` however many words
// precede the source, and the decoder writes the named form the grammar accepts.
func TestGuardedSuccessionKeepsItsSyntax(t *testing.T) {
	src := `package DecisionTest {
    attribute x : Integer;
    action def A;
    action A1 : A;
    action A2 : A;
    action A3 : A;
    public succession S first A1 if x == 0 then A2;
    succession first A1 if x == 1 then A3;
    state def Machine {
        state off;
        state on;
        private transition T first off if x == 2 then on;
        transition first on if x == 3 then off;
        transition on if x == 4 then off;
        public succession U first off if x == 5 then on;
        /* transition */ succession /* first */ V first /* first */ on if x == 6 then off;
        transition // succession first
            first off if x == 7 then on;
    }
}
`
	back := notationFromTheGraphAlone(t, "d.sysml", src)
	wantFragments(t, back,
		"public succession S first A1 if x == 0 then A2;",
		"succession first A1 if x == 1 then A3;",
		"private transition T first off if x == 2 then on;",
		"transition first on if x == 3 then off;",
		"transition on if x == 4 then off;",
		"public succession U first off if x == 5 then on;",
		"succession V first on if x == 6 then off;",
		"transition first off if x == 7 then on;")
	turtle, err := export.Convert("d.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(turtle), `sysx:transitionSyntax "first"`); n != 7 {
		t.Errorf("want 7 transitions recorded with `first`, got %d\n%s", n, turtle)
	}
	if n := strings.Count(string(turtle), `sysx:declaredKeyword "succession"`); n != 4 {
		t.Errorf("want 4 successions recorded, got %d\n%s", n, turtle)
	}
}

// Expression bodies as arguments, chained bodies and casts under a feature
// chain are written in the form the grammar reads back to the same graph.
func TestExpressionBodiesAndChainsFromTheGraphAlone(t *testing.T) {
	sysml := `package B {
    part def D {
        attribute xs : Integer[0..*];
        part sub : D[0..1];
        attribute ok : Boolean = xs->forAll { in e : Integer; e > 0 };
        attribute picked = xs->select { in e; e > 1 and e < 9 };
        attribute deep = xs.{ in e; e * 2 };
        attribute chained = sub.xs->size();
        attribute casted = (sub as D).xs;
    }
}
`
	back := notationFromTheGraphAlone(t, "b.sysml", sysml)
	wantFragments(t, back,
		"attribute ok : Boolean = xs->forAll({ in e : Integer; e > 0 });",
		"attribute picked = xs->select({ in e; e > 1 and e < 9 });",
		"attribute deep = xs.{ in e; e * 2 };",
		"attribute chained = sub.xs->size();",
		"attribute casted = (sub as D).xs;")
	kerml := `package K {
    classifier C {
        feature xs : Base::Anything[0..*];
        feature sub : C[0..1];
        inv { xs->forAll { in e; e != null } }
        feature slice redefines xs = (sub as C).xs;
        inv { notEmpty(xs) implies (sub.xs->size() > 0 xor false) }
    }
}
`
	back = notationFromTheGraphAlone(t, "k.kerml", kerml)
	wantFragments(t, back,
		"xs->forAll({ in e; e != null })",
		"feature slice redefines xs = (sub as C).xs;",
		"notEmpty(xs) implies sub.xs->size() > 0 xor false")
}

// A usage redefining a namesake keeps that target through the graph alone even
// though the writer puts `redefines causes` after `subsets participant`.
func TestSelfNamedRedefinitionTargetSurvivesTheGraphAlone(t *testing.T) {
	src := `package P {
	abstract occurrence causes[*];
	occurrence def Link {
		ref occurrence participant[*];
	}
	abstract occurrence def Multicausation :> Link {
		abstract constant ref occurrence causes[1..*] :>> causes :> participant;
	}
}
`
	back := notationFromTheGraphAlone(t, "p.sysml", src)
	wantFragments(t, back, "occurrence causes[1..*] subsets participant redefines causes;")
}

// An unnamed usage takes its name from the first feature it redefines, so a
// chain through it reads that name back however many features it redefines.
func TestNameFromTheFirstOfSeveralRedefinitionsFromTheGraphAlone(t *testing.T) {
	src := `package NP {
    item def Disc {
        attribute innerSpaceDimension;
    }
    item def Shell {
        item faces {
            attribute innerSpaceDimension;
        }
    }
    item def ConeOrCylinder :> Shell {
        item :>> faces;
        item base : Disc :> faces {
            attribute :>> Disc::innerSpaceDimension, faces::innerSpaceDimension;
        }
        attribute dim = base.innerSpaceDimension;
    }
}
`
	back := notationFromTheGraphAlone(t, "np.sysml", src)
	wantFragments(t, back,
		"attribute redefines innerSpaceDimension, faces::innerSpaceDimension;",
		"attribute dim = base.innerSpaceDimension;")
}
