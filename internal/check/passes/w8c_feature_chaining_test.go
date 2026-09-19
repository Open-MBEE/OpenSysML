package passes

import (
	"strings"
	"testing"
)

// chainConformanceLines returns the 1-based source lines on which src draws a
// feature-chain-conformance diagnostic, in order.
func chainConformanceLines(t *testing.T, name, src string) []int {
	t.Helper()
	var lines []int
	for _, d := range only(w8cLibraryDiagnostics(t, name, src), "feature-chain-conformance") {
		if d.Message != msgReferentIsFeature {
			t.Errorf("message = %q, want %q", d.Message, msgReferentIsFeature)
		}
		lines = append(lines, strings.Count(src[:d.Span.Offset], "\n")+1)
	}
	return lines
}

// A chain segment reached through an alias of the previous segment's type is
// not featured within it; the pilot reports every declared chain so written
// (validateFeatureChainingFeatureConformance): subsettings, references, chains,
// connector and binding ends — but not a chain's conforming segments.
func TestW8CFeatureChainingConformanceKerML(t *testing.T) {
	const src = `package P {
		class B { feature y : B; }
		class A { alias z for B::y; feature w : B; }
		class K {
			feature k : A;
			feature q : B;
			feature r1 references k.z;
			feature r2 chains k.z;
			feature r3 :> k.z;
			feature r4 references k.w.y;
			feature r5 references k.z.y;
			connector c from k.z to q;
			binding b of k.z = q;
			succession s first k.w then q;
		}
	}`
	want := []int{7, 8, 9, 11, 12, 13}
	if got := chainConformanceLines(t, "<t>.kerml", src); !sameLines(got, want) {
		t.Fatalf("feature-chain-conformance lines = %v, want %v", got, want)
	}
}

func TestW8CFeatureChainingConformanceSysML(t *testing.T) {
	const src = `package P {
		part def B { part y : B; }
		part def A { alias z for B::y; }
		part def K {
			part k : A;
			part q : B;
			ref r1 :> k.z;
			ref r2 references k.z;
			connect k.z to q;
			bind k.z = q;
			flow from k.z to q;
		}
	}`
	want := []int{7, 8, 9, 10}
	if got := chainConformanceLines(t, "<t>.sysml", src); !sameLines(got, want) {
		t.Fatalf("feature-chain-conformance lines = %v, want %v", got, want)
	}
}

// A segment inherited by, or reached through a subsetting of, the previous one
// is featured within it; a flow end's last segment names the end's nested
// feature and is not a chain segment.
func TestW8CFeatureChainingConformanceValid(t *testing.T) {
	cases := map[string]string{
		"<t>.kerml": `package P {
			class B { feature y : B; }
			class A { feature z : B; }
			class A2 :> A;
			class K {
				feature k : A2;
				feature k2 :> k;
				feature r1 references k.z;
				feature r2 references k2.z;
				feature r3 references k.z.y;
				feature r4 chains k2.z.y;
				connector c from k.z to k2.z;
				binding b of k.z = k2.z;
				succession s first k.z then k2.z;
				flow from k.z.y to k2.z;
			}
		}`,
		"<t>.sysml": `package P {
			part def B { part y : B; }
			part def A { part z : B; }
			part def A2 :> A;
			part def K {
				part k : A2;
				part k2 :> k;
				ref r1 :> k.z;
				ref r2 references k2.z.y;
				connect k.z to k2.z;
				bind k.z = k2.z;
				flow from k.z.y to k2.z;
				action a { perform action p; }
				action a2 :> a { perform a.p; }
			}
		}`,
	}
	for name, src := range cases {
		if got := chainConformanceLines(t, name, src); len(got) != 0 {
			t.Errorf("%s: conforming chains reported on lines %v", name, got)
		}
	}
}
