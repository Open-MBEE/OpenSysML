package passes

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Reference subsetting is a subsetting (KerML 8.3.3.3.9), so a referencing feature
// features what the referenced one does and its body's ends are accessible (8.3.4.5.3).
func TestFeaturesHeldByReferenceSubsettingAreAccessible(t *testing.T) {
	cases := map[string]string{
		"succession over referenced events": `package P {
	part source { event occurrence sent; event occurrence got; }
	ref part observer ::> source { succession first sent then got; }
	part client :> observer, source;
}`,
		"succession through a chain of references": `package P {
	part source { event occurrence sent; event occurrence got; }
	ref part mid ::> source;
	ref part observer ::> mid { succession first sent then got; }
}`,
		"succession over events the referenced part inherits": `package P {
	part def S { event occurrence sent; event occurrence got; }
	part source : S;
	ref part observer ::> source { succession first sent then got; }
}`,
		"succession between referenced actions": `package P {
	action source { action a; action b; }
	action observer ::> source { first a then b; }
}`,
		"connection": `package P {
	part source { part a; part b; }
	ref part observer ::> source { connect a to b; }
}`,
		"binding": `package P {
	part source { attribute a; attribute b; }
	ref part observer ::> source { bind a = b; }
}`,
		"flow": `package P {
	part source { part a { out item x; } part b { in item y; } }
	ref part observer ::> source { flow a.x to b.y; }
}`,
		"feature value": `package P {
	part source { attribute a = 1; }
	ref part observer ::> source { attribute c = a; }
}`,
		"a nested referenced feature reached by dot notation": `package P {
	part host { part source { part a; part b; } }
	part elsewhere { ref part observer ::> host.source { connect a to b; } }
}`,
		"a referenced feature the owner inherits": `package P {
	part def A { part source { part a; part b; } }
	part def B :> A { ref part observer ::> A::source { connect a to b; } }
}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if diags := analyzeAll(t, "clean.sysml", src); len(diags) != 0 {
				t.Errorf("diagnostics: %v", diags)
			}
		})
	}
}

// Without the reference, or from outside the referencing feature, the same ends stay
// inaccessible, each at the location the pinned pilot reports it.
func TestFeaturesNotHeldByReferenceSubsettingStayInaccessible(t *testing.T) {
	cases := map[string]struct {
		src  string
		want []string
	}{
		"observer does not reference source": {`package P {
	part source { event occurrence sent; event occurrence got; }
	ref part observer { succession first source::sent then source::got; }
}`, []string{"3:39", "3:57"}},
		"connection without the reference": {`package P {
	part source { part a; part b; }
	ref part observer { connect source::a to source::b; }
}`, []string{"3:30", "3:43"}},
		"feature value without the reference": {`package P {
	part source { attribute a = 1; }
	ref part observer { attribute c = source::a; }
}`, []string{"3:36"}},
		"the referenced part does not feature the observer's events": {`package P {
	part source { succession first observer::sent then observer::got; }
	ref part observer ::> source { event occurrence sent; event occurrence got; }
}`, []string{"2:33", "2:53"}},
		"an end of an unreferenced sibling": {`package P {
	part source { event occurrence sent; event occurrence got; }
	part other { event occurrence x; }
	ref part observer ::> source { succession first sent then other::x; }
}`, []string{"4:60"}},
		"a part nested in the observer": {`package P {
	part source { event occurrence sent; event occurrence got; }
	ref part observer ::> source { part inner { succession first sent then got; } }
}`, []string{"3:63", "3:73"}},
		"a referenced feature of another definition, connection": {`package P {
	part def A { part source { part a; part b; } }
	part def B { ref part observer ::> A::source { connect a to b; } }
}`, []string{"3:37"}},
		"a referenced feature of another definition, feature value": {`package P {
	part def A { part source { attribute a = 1; } }
	part def B { ref part observer ::> A::source { attribute c = a; } }
}`, []string{"3:37"}},
		"a referenced feature of another definition, succession": {`package P {
	part def A { part source { event occurrence sent; event occurrence got; } }
	part def B { ref part observer ::> A::source { succession first sent then got; } }
}`, []string{"3:37"}},
		"a referenced action of another definition": {`package P {
	action def A { action source { action a; action b; } }
	action def B { action observer ::> A::source { first a then b; } }
}`, []string{"3:37"}},
		"a referenced feature of another usage": {`package P {
	part host { part source { part a; part b; } }
	part elsewhere { ref part observer ::> host::source { connect a to b; } }
}`, []string{"3:41"}},
		"a referenced feature of another definition, no body": {`package P {
	part def A { part source; }
	part def B { ref part observer ::> A::source; }
}`, []string{"3:37"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var got []string
			for _, d := range analyzeAll(t, "inaccessible.sysml", tc.src) {
				if d.Severity == diag.SeverityError && d.Message == msgConnectorTypeFeaturing &&
					(d.Code == "feature-reference-featuring-types" || d.Code == "connector-type-featuring" ||
						d.Code == "subsetting-featuring-types") {
					got = append(got, locationOf(tc.src, d.Span.Offset))
				}
			}
			for _, want := range tc.want {
				if !slices.Contains(got, want) {
					t.Errorf("no accessibility error at %s, got %v", want, got)
				}
			}
		})
	}
}
