package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// Prefix metadata after a subject, actor, stakeholder, objective, variant,
// assume or require keyword — and ahead of `assert` — round-trips through the
// graph, and comes back in the same place without the source text.
func TestMemberPrefixMetadataPlacementRoundTrips(t *testing.T) {
	src := `package P {
	metadata def M;
	metadata def B;
	part def T;
	part def A;
	constraint def C;
	requirement def R;
	requirement def Req {
		subject #M s : T;
		actor #M who : A;
		stakeholder #M k;
		assume #M constraint a : C;
		require #M constraint r : C;
		require #M <r2> rc : C;
	}
	requirement def ReqShort {
		subject #M <s> x : T;
		assume #M constraint <a> ac : C;
	}
	use case def U {
		objective #M o : R;
	}
	variation part def V {
		variant #M part v;
	}
	part def D {
		#B assert not constraint c;
	}
}
`
	turtle := roundTripsExactly(t, src)

	// Without the source text the keyword-less `require #M <r2> rc` comes back
	// in its canonical spelling with `constraint`; everything else is verbatim.
	structural := withoutSourceText(t, turtle)
	back := toNotation(t, structural)
	for _, want := range []string{
		"subject #M s : T;",
		"actor #M who : A;",
		"stakeholder #M k;",
		"assume #M constraint a : C;",
		"require #M constraint r : C;",
		"require #M constraint <r2> rc : C;",
		"subject #M <s> x : T;",
		"assume #M constraint <a> ac : C;",
		"objective #M o : R;",
		"variant #M part v;",
		"#B assert not constraint c;",
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

// Prefix metadata ahead of one of those keywords is a syntax error, so the
// conversion refuses it rather than writing the member in the accepted spelling.
func TestMemberPrefixMetadataBeforeKeywordDoesNotConvert(t *testing.T) {
	for _, member := range []string{
		"#M subject s : T;",
		"#M assume constraint a : C;",
		"#M require constraint r : C;",
	} {
		src := "package P {\n\tmetadata def M;\n\tpart def T;\n\tconstraint def C;\n\trequirement def R {\n\t\t" + member + "\n\t}\n}\n"
		_, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		var syntax *export.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("%s: err = %v, want a syntax error", member, err)
		}
		if !strings.Contains(err.Error(), "prefix metadata follows") {
			t.Errorf("%s: err = %v, want it to name the placement", member, err)
		}
	}
}
