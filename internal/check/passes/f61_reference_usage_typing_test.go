package passes

import (
	"strings"
	"testing"
)

// F61: a usage written with no kind keyword is a ReferenceUsage (SysML.xtext
// DefaultReferenceUsage), and a metadata-prefixed one an ExtendedUsage — neither
// is an attribute usage, so the attribute typing rule does not constrain it.
// Corpus: "Arrowhead Framework Example/AHFCoreLib.sysml" (`#service serviceDiscovery
// : ServiceDiscovery;`), matching the pilot, which reports nothing on any of these.
func TestF61ReferenceUsageTypedByAnyDefinition(t *testing.T) {
	for _, src := range []string{
		"port def PD; part def P { sd : PD; }",
		"port def PD; part def P { ref sd : PD; }",
		"port def PD; part def P { in sd : PD; }",
		"metadata def service; port def PD; part def P { #service sd : PD; }",
		"port def PD; part def P { sd : ~PD; }",
		"port def PD; part def P { ref sd : ~PD; }",
		"metadata def service; port def PD; part def P { #service sd : ~PD; }",
		"action def A; part def P { sd : A; }",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// F61 negative: the attribute typing rule still fires where the declaration says
// `attribute`, as the pilot's "An attribute must be typed by attribute
// definitions." does, and the conjugated-port restriction still fires with it.
func TestF61ExplicitAttributeTypingStillRejected(t *testing.T) {
	diags := typeDiags(t, "port def PD; part def P { attribute sd : PD; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if diags[0].Message != "An attribute must be typed by attribute definitions." {
		t.Errorf("got %q", diags[0].Message)
	}

	diags = typeDiags(t, "port def PD; part def P { attribute sd : ~PD; }")
	if len(diags) != 2 {
		t.Fatalf("expected two type diagnostics, got %v", diags)
	}
	var conjugated bool
	for _, d := range diags {
		if strings.Contains(d.Message, "typed by a conjugated port definition") {
			conjugated = true
		}
	}
	if !conjugated {
		t.Errorf("expected the conjugated-port restriction, got %v", diags)
	}
}

// F61 negative: a keyword-less usage is a reference, not a licence to type a
// usage of a stated kind by anything — a part is still typed by occurrences.
func TestF61KeywordedUsagesStillConstrained(t *testing.T) {
	for _, src := range []string{
		"attribute def M; part def P { part p : M; }",
		"port def PD; part def P { attribute def A; attribute a : PD; }",
	} {
		if diags := typeDiags(t, src); len(diags) == 0 {
			t.Errorf("%s: expected a type diagnostic, got none", src)
		}
	}
}
