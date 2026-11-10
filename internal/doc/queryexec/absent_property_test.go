package queryexec

import (
	"errors"
	"slices"
	"testing"
)

// A property the metamodel declares that no row carries is absent, not unknown: tolerated an
// empty column, no row kept, the order left; untolerated the missing-feature error.
func TestExecuteDeclaredPropertyNoRowCarriesIsAbsent(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Lander;
part a : Lander;
part b : Lander;
calc def Bodies :> Query {
	Project(source = `+landerParts+`, properties = ("name", "body"), tolerate = true)
}
calc def BodiesStrict :> Query {
	Project(source = `+landerParts+`, properties = ("body"))
}
calc def WithBody :> Query {
	WhereFeature(source = `+landerParts+`, 'feature' = "body", operator = "=", value = "x", tolerate = true)
}
calc def ByBody :> Query {
	OrderBy(source = `+landerParts+`, property = "body", direction = "descending", missing = "last", multiple = "first")
}
calc def Undeclared :> Query {
	Project(source = `+landerParts+`, properties = ("nmae"), tolerate = true)
}
`)
	result, err := fixture.execute(t, "Bodies", Bindings{}, Options{})
	if err != nil {
		t.Fatalf("Bodies: %v", err)
	}
	if got := cellStrings(t, result, "body"); !slices.EqualFunc(got, [][]string{{}, {}}, slices.Equal) {
		t.Fatalf("Bodies: body cells = %q, want two empty cells", got)
	}
	if got := cellStrings(t, result, "name"); !slices.EqualFunc(got, [][]string{{"a"}, {"b"}}, slices.Equal) {
		t.Fatalf("Bodies: name cells = %q", got)
	}
	var executionError *Error
	_, err = fixture.execute(t, "BodiesStrict", Bindings{}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorMissingFeature || executionError.Target != "Observatory::a" {
		t.Fatalf("BodiesStrict: err = %v, want the missing-feature error at Observatory::a", err)
	}
	kept, err := fixture.execute(t, "WithBody", Bindings{}, Options{})
	if err != nil || len(kept.Rows()) != 0 {
		t.Fatalf("WithBody: rows = %v, err = %v, want none and no error", kept.Rows(), err)
	}
	ordered, err := fixture.execute(t, "ByBody", Bindings{}, Options{})
	if err != nil {
		t.Fatalf("ByBody: %v", err)
	}
	if got := rowNames(ordered); !slices.Equal(got, []string{"Observatory::a", "Observatory::b"}) {
		t.Fatalf("ByBody: rows = %v, want the declaration order kept", got)
	}
	_, err = fixture.execute(t, "Undeclared", Bindings{}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorUnknownProperty {
		t.Fatalf("Undeclared: err = %v, want unknown-property", err)
	}
}

const landerParts = `WhereType(source = Descendants(source = Named(qualifiedName = "Observatory"), maxDepth = 1), type = "PartUsage")`

// A metamodel property OpenSysML does not derive yet (an accept's payloadArgument) is the
// unserved-property error untolerated; tolerated, an empty cell, a dropped row, a missing key.
func TestExecuteUnservedPropertyIsAbsentWhenTolerated(t *testing.T) {
	fixture := loadExecutionFixture(t, `
action def Watch {
	action wait accept after 5 [SI::s];
}
calc def Payloads :> Query {
	Project(source = `+watchActions+`, properties = ("name", "payloadArgument"), tolerate = true)
}
calc def PayloadsStrict :> Query {
	Project(source = `+watchActions+`, properties = ("payloadArgument"))
}
calc def WherePayload :> Query {
	WhereFeature(source = `+watchActions+`, 'feature' = "payloadArgument", operator = "=", value = "x", tolerate = true)
}
calc def WherePayloadStrict :> Query {
	WhereFeature(source = `+watchActions+`, 'feature' = "payloadArgument", operator = "=", value = "x")
}
calc def ByPayload :> Query {
	OrderBy(source = `+watchActions+`, property = "payloadArgument", direction = "ascending", missing = "first", multiple = "first")
}
calc def ByPayloadStrict :> Query {
	OrderBy(source = `+watchActions+`, property = "payloadArgument", direction = "ascending", missing = "error", multiple = "first")
}
`)
	result, err := fixture.execute(t, "Payloads", Bindings{}, Options{})
	if err != nil {
		t.Fatalf("Payloads: %v", err)
	}
	if got := cellStrings(t, result, "payloadArgument"); !slices.EqualFunc(got, [][]string{{}}, slices.Equal) {
		t.Fatalf("Payloads: cells = %q, want one empty cell", got)
	}
	if got := cellStrings(t, result, "name"); !slices.EqualFunc(got, [][]string{{"wait"}}, slices.Equal) {
		t.Fatalf("Payloads: name cells = %q", got)
	}
	kept, err := fixture.execute(t, "WherePayload", Bindings{}, Options{})
	if err != nil || len(kept.Rows()) != 0 {
		t.Fatalf("WherePayload: rows = %v, err = %v, want none and no error", kept.Rows(), err)
	}
	ordered, err := fixture.execute(t, "ByPayload", Bindings{}, Options{})
	if err != nil || len(ordered.Rows()) != 1 {
		t.Fatalf("ByPayload: rows = %v, err = %v, want the one action", ordered.Rows(), err)
	}
	var executionError *Error
	for _, query := range []string{"PayloadsStrict", "WherePayloadStrict", "ByPayloadStrict"} {
		_, err = fixture.execute(t, query, Bindings{}, Options{})
		if !errors.As(err, &executionError) || executionError.Kind != ErrorUnservedProperty || executionError.Target != "Observatory::Watch::wait" {
			t.Fatalf("%s: err = %v, want the unserved-property error at Observatory::Watch::wait", query, err)
		}
	}
}

const watchActions = `WhereType(source = Descendants(source = Named(qualifiedName = "Observatory::Watch"), maxDepth = 1), type = "ActionUsage")`
