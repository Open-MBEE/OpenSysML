package passes

import (
	"strings"
	"testing"
)

func TestTransitionGuardIsElementScoped(t *testing.T) {
	src := `package P {
	part broken : Missing;
	state def S {
		state a;
		state b;
		transition first a if "test" then b;
	}
}`
	diags := diagsIn(t, "a.sysml", src, "type")
	if len(diags) != 1 ||
		diags[0].Message != "transition guard must be Boolean, found String" ||
		diags[0].Span.Offset != strings.Index(src, `"test"`) {
		t.Fatalf("got %v, want only the independent transition guard diagnostic", diags)
	}
}

func TestTransitionGuardUnresolvedDoesNotCascade(t *testing.T) {
	src := `package P {
	state def S {
		state a;
		state b;
		transition first a if missing then b;
	}
}`
	if diags := diagsIn(t, "a.sysml", src, "type"); len(diags) != 0 {
		t.Fatalf("got %v, want no type cascade for the unresolved guard", diags)
	}
}

// A fault on another element does not hide an invalid trigger, wherever the
// trigger is written; a trigger whose own argument is the fault draws nothing.
func TestTriggerArgumentIsElementScoped(t *testing.T) {
	for _, tc := range []struct{ body, code string }{
		{"transition first a accept after 5 then b;", "trigger-after-duration"},
		{"transition first a accept at 5 then b;", "trigger-at-time-instant"},
		{"transition first a accept when 5 then b;", "trigger-when-boolean"},
		{"transition first a when 5 then b;", "trigger-when-boolean"},
		{"entry action { accept after 5; }", "trigger-after-duration"},
		{"do action { first start; then accept at 5; }", "trigger-at-time-instant"},
		{"transition first a then b { accept when 5; }", "trigger-when-boolean"},
	} {
		src := `package P {
	part broken : Missing;
	state def S {
		state a;
		state b;
		` + tc.body + `
	}
}`
		diags := diagsIn(t, "a.sysml", src, "type")
		if len(diags) != 1 || diags[0].Code != tc.code || diags[0].Span.Offset != strings.LastIndex(src, "5") {
			t.Errorf("%q: got %v, want only the independent %s diagnostic", tc.body, diags, tc.code)
		}
	}
	for _, body := range []string{
		"transition first a accept after missing then b;",
		"transition first a accept at missing + 1 then b;",
		"entry action { accept when missing; }",
	} {
		src := `package P {
	state def S {
		state a;
		state b;
		` + body + `
	}
}`
		if diags := diagsIn(t, "a.sysml", src, "type"); len(diags) != 0 {
			t.Errorf("%q: got %v, want no type cascade for the unresolved trigger argument", body, diags)
		}
	}
}

func TestKerMLSubsettingMetaclassIsElementScoped(t *testing.T) {
	src := `package P {
	classifier non;
	feature aa subsets non;
	feature broken subsets missing;
}`
	diags := diagsIn(t, "a.kerml", src, "type")
	if len(diags) != 1 ||
		diags[0].Message != "subsets target must be a feature, found kermlType" ||
		diags[0].Span.Offset != strings.Index(src, "non;\n\tfeature broken") {
		t.Fatalf("got %v, want only the independent subsetting diagnostic", diags)
	}
}

func TestKerMLSubsettingMetaclassUnresolvedDoesNotCascade(t *testing.T) {
	src := `package P {
	feature aa subsets missing;
}`
	if diags := diagsIn(t, "a.kerml", src, "type"); len(diags) != 0 {
		t.Fatalf("got %v, want no type cascade for the unresolved subset target", diags)
	}
}
