package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// A short name on a requirement's subject, assume or require member is carried by
// sysml:declaredShortName and comes back from the graph without the source text.
func TestRequirementMemberShortNamesRoundTrip(t *testing.T) {
	src := `package P {
	part def T;
	constraint def C;
	requirement def R {
		subject <s> x : T;
		assume constraint <a> ac : C;
		require constraint <r> rc : C;
	}
	requirement def R2 {
		subject <s> : T;
		assume constraint <a> : C;
		require constraint <r> : C;
	}
	requirement def R3 :> R {
		subject #Meta <s2> y :>> R::s;
		assume constraint <a2> :>> R::a;
		require constraint <r2> :>> R::r;
	}
	metadata def Meta;
}
`
	turtle := roundTripsExactly(t, src)
	text := string(turtle)
	for _, want := range []string{
		`sysml:declaredShortName "s"`,
		`sysml:declaredShortName "a"`,
		`sysml:declaredShortName "r"`,
		`sysml:declaredShortName "s2"`,
		`sysml:declaredShortName "a2"`,
		`sysml:declaredShortName "r2"`,
	} {
		if strings.Count(text, want) < 1 {
			t.Errorf("graph lacks %s:\n%s", want, text)
		}
	}
	if got, want := strings.Count(text, `sysml:declaredShortName "s"`), 2; got != want {
		t.Errorf("declaredShortName \"s\" appears %d times, want %d (R and R2):\n%s", got, want, text)
	}

	// Without the source text the printer must regenerate every short name from
	// the graph, and that regenerated notation must convert to the same graph.
	structural := withoutSourceText(t, turtle)
	back := toNotation(t, structural)
	for _, want := range []string{
		"subject <s> x : T;",
		"assume constraint <a> ac : C;",
		"require constraint <r> rc : C;",
		"subject <s> : T;",
		"assume constraint <a> : C;",
		"require constraint <r> : C;",
		"subject #Meta <s2> y redefines x;",
		"assume constraint <a2> redefines ac;",
		"require constraint <r2> redefines rc;",
	} {
		if !strings.Contains(back, want) {
			t.Errorf("structural round trip lost %q:\n%s", want, back)
		}
	}
	again, err := export.Convert("m.sysml", []byte(back), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("regenerated notation does not convert: %v\n%s", err, back)
	}
	if got, want := string(withoutSourceText(t, again)), string(structural); got != want {
		t.Errorf("regenerated notation yields a different graph:\n--- want ---\n%s--- got ---\n%s", want, got)
	}
}

// The short name is what distinguishes the member: a graph that drops it comes
// back without one, so the assertion above is load-bearing.
func TestRequirementMemberShortNamesAreNotImplied(t *testing.T) {
	src := "package P {\n\tpart def T;\n\trequirement def R {\n\t\tsubject <s> x : T;\n\t}\n}\n"
	turtle := withoutSourceText(t, idTurtle(t, src))
	back := toNotation(t, withoutTriples(t, turtle, "sysml:declaredShortName"))
	if strings.Contains(back, "<s>") {
		t.Errorf("short name survived without its triple:\n%s", back)
	}
	if !strings.Contains(back, "subject x : T;") {
		t.Errorf("subject lost more than its short name:\n%s", back)
	}
}
