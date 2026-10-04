package passes

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// codesOf counts the diagnostics of one code and returns their severities.
func multiplicityDiags(t *testing.T, src, code string) []diag.Diagnostic {
	t.Helper()
	return only(constraintDiags(t, src), code)
}

func TestW7GSubsettingUpperBoundIsAWarning(t *testing.T) {
	const src = `package M {
		part def P {
			part cap[0..5];
			part few[0..9] subsets cap;
		}
	}`
	diags := multiplicityDiags(t, src, "subsetting-multiplicity")
	if len(diags) != 1 {
		t.Fatalf("expected one upper-bound diagnostic, got %v", diags)
	}
	if diags[0].Severity != diag.SeverityWarning {
		t.Fatalf("expected a warning, got %v", diags[0].Severity)
	}
	if diags[0].Message != msgSubsettingMultiplicityConformance {
		t.Fatalf("message is not the reference's: %q", diags[0].Message)
	}
}

func TestW7GRedefinitionLowerAndUpperBoundsAreSeparateWarnings(t *testing.T) {
	const src = `package M {
		part def P { part cyl[2..4]; }
		part def Q :> P { part mycyl[1..8] redefines cyl; }
	}`
	diags := constraintDiags(t, src)
	if got := len(only(diags, "redefinition-multiplicity")); got != 1 {
		t.Fatalf("expected one lower-bound warning, got %d in %v", got, diags)
	}
	if got := len(only(diags, "subsetting-multiplicity")); got != 1 {
		t.Fatalf("expected one upper-bound warning, got %d in %v", got, diags)
	}
	for _, d := range diags {
		if d.Severity != diag.SeverityWarning {
			t.Fatalf("multiplicity conformance is a warning in the reference, got %v", d)
		}
	}
}

func TestW7GRedefinitionWithinTheBoundsIsSilent(t *testing.T) {
	const src = `package M {
		part def P { part cyl[2..4]; }
		part def Q :> P { part mycyl[3..4] redefines cyl; }
	}`
	if diags := constraintDiags(t, src); len(diags) != 0 {
		t.Fatalf("a conforming redefinition should be silent, got %v", diags)
	}
}

func TestW7GDefaultMultiplicityAppliesToPartAttributeItemAndPort(t *testing.T) {
	const src = `package M {
		port def Pt;
		item def It;
		part def P {
			attribute a;
			item i : It;
			part p;
			port t : Pt;
		}
		part def Q :> P {
			attribute a2[2] subsets a;
			item i2[2] : It subsets i;
			part p2[2] subsets p;
			port t2[2] : Pt subsets t;
		}
	}`
	if got := len(multiplicityDiags(t, src, "subsetting-multiplicity")); got != 4 {
		t.Fatalf("expected the implicit 1..1 of all four kinds to be exceeded, got %d", got)
	}
}

func TestW7GNoDefaultMultiplicityForActionsStatesOrKerMLFeatures(t *testing.T) {
	const src = `package M {
		action def A;
		state def S;
		part def P {
			action a : A;
			state s : S;
			feature f;
			occurrence o;
			connection c;
		}
		part def Q :> P {
			action a2[2] : A subsets a;
			state s2[2] : S subsets s;
			feature f2[2] subsets f;
			occurrence o2[2] subsets o;
			connection c2[2] subsets c;
		}
	}`
	if diags := multiplicityDiags(t, src, "subsetting-multiplicity"); len(diags) != 0 {
		t.Fatalf("the reference gives these kinds no default multiplicity, got %v", diags)
	}
}

type expectedW7GMultiplicityDiagnostic struct {
	code    string
	message string
	needle  string
	target  string
}

func bothW7GMultiplicityDiagnostics(needle, target string) []expectedW7GMultiplicityDiagnostic {
	return []expectedW7GMultiplicityDiagnostic{
		{
			code:    "redefinition-multiplicity",
			message: "Redefining feature should not have smaller multiplicity lower bound",
			needle:  needle,
			target:  target,
		},
		{
			code:    "subsetting-multiplicity",
			message: "Subsetting/redefining feature should not have larger multiplicity upper bound",
			needle:  needle,
			target:  target,
		},
	}
}

func assertW7GMultiplicityDiagnostics(t *testing.T, src string, want []expectedW7GMultiplicityDiagnostic) {
	t.Helper()
	all := constraintDiags(t, src)
	var got []diag.Diagnostic
	for _, d := range all {
		if d.Code == "redefinition-multiplicity" || d.Code == "subsetting-multiplicity" {
			got = append(got, d)
		} else {
			t.Errorf("unexpected constraint diagnostic: %v", d)
		}
	}
	offsetOf := func(needle, target string) int {
		start := strings.Index(src, needle)
		if start < 0 {
			t.Fatalf("diagnostic locator %q not found", needle)
		}
		targetAt := strings.LastIndex(needle, target)
		if targetAt < 0 {
			t.Fatalf("target %q not found in diagnostic locator %q", target, needle)
		}
		return start + targetAt
	}
	sort.Slice(got, func(i, j int) bool {
		if got[i].Span.Offset == got[j].Span.Offset {
			return got[i].Code < got[j].Code
		}
		return got[i].Span.Offset < got[j].Span.Offset
	})
	sort.Slice(want, func(i, j int) bool {
		left, right := offsetOf(want[i].needle, want[i].target), offsetOf(want[j].needle, want[j].target)
		if left == right {
			return want[i].code < want[j].code
		}
		return left < right
	})
	if len(got) != len(want) {
		t.Fatalf("got %d multiplicity diagnostics, want %d: %v", len(got), len(want), got)
	}
	for i, expected := range want {
		d := got[i]
		wantOffset := offsetOf(expected.needle, expected.target)
		if d.Code != expected.code || d.Message != expected.message {
			t.Errorf("diagnostic %d = (%s, %q), want (%s, %q)", i, d.Code, d.Message, expected.code, expected.message)
		}
		if d.Severity != diag.SeverityWarning {
			t.Errorf("diagnostic %d severity = %v, want warning", i, d.Severity)
		}
		if d.Span.Offset != wantOffset || d.Span.Len != len(expected.target) {
			t.Errorf("diagnostic %d span = %+v, want %d:%d", i, d.Span, wantOffset, len(expected.target))
		} else if gotText := src[d.Span.Offset:d.Span.End()]; gotText != expected.target {
			t.Errorf("diagnostic %d targets %q, want %q", i, gotText, expected.target)
		}
	}
}

func TestW7GDefaultMultiplicityAppliesToAdditionalUsageMetaclasses(t *testing.T) {
	both := bothW7GMultiplicityDiagnostics
	cases := []struct {
		name string
		src  string
		want []expectedW7GMultiplicityDiagnostic
	}{
		{
			name: "enumeration",
			src: `package P {
				enum def E { enum a; enum b; }
				part def D { enum e : E; }
				part def F :> D { enum :>> e [0..*]; }
			}`,
			want: both("enum :>> e [0..*]", "e"),
		},
		{
			name: "view",
			src: `package P {
				view def V;
				part def D { view v : V; }
				part def F :> D { view :>> v [0..*]; }
			}`,
			want: both("view :>> v [0..*]", "v"),
		},
		{
			name: "rendering",
			src: `package P {
				rendering def R;
				part def D { rendering r : R; }
				part def F :> D { rendering :>> r [0..*]; }
			}`,
			want: both("rendering :>> r [0..*]", "r"),
		},
		{
			name: "requirement actors",
			src: `package P {
				part def U;
				requirement def R {
					subject s : U;
					actor a : U;
					stakeholder k : U;
				}
				requirement def R2 :> R {
					subject :>> s;
					actor :>> a [0..*];
					stakeholder :>> k [0..*];
				}
			}`,
			want: append(
				both("actor :>> a [0..*]", "a"),
				both("stakeholder :>> k [0..*]", "k")...,
			),
		},
		{
			name: "reference subsetting",
			src: `package P {
				part def D { part a [0..*]; part b ::> a; }
				part def E :> D { part :>> b [0..*]; }
			}`,
		},
		{
			name: "references keyword",
			src: `package P {
				part def D { part a [0..*]; part b references a; }
				part def E :> D { part :>> b [0..*]; }
			}`,
		},
		{
			name: "reference chain",
			src: `package P {
				part def W { part w [0..*]; }
				part def D { part a : W; part b ::> a.w; }
				part def E :> D { part :>> b [0..*]; }
			}`,
		},
		{
			name: "package-level target",
			src: `package P {
				part a [0..*];
				part def D { part b ::> a; }
				part def E :> D { part :>> b [0..*]; }
			}`,
			want: both("part :>> b [0..*]", "b"),
		},
		{
			name: "metadata target",
			src: `package P {
				metadata def M;
				part def D { metadata m : M; item x [0..*] :> m; }
			}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertW7GMultiplicityDiagnostics(t, tc.src, tc.want)
		})
	}
}

func TestW7GEndAndNonEndFeaturesAreNotCompared(t *testing.T) {
	const src = `package M {
		part def P {
			part parts[0..4];
			connection c {
				end p1[0..*] subsets parts;
				end p2[0..*] subsets parts;
			}
		}
	}`
	if diags := multiplicityDiags(t, src, "subsetting-multiplicity"); len(diags) != 0 {
		t.Fatalf("an end subsetting a non-end is exempt in the reference, got %v", diags)
	}
}

func TestW7GUnboundedSubsettedUpperBoundIsSilent(t *testing.T) {
	const src = `package M {
		part def P {
			part all[0..*];
			part some[0..7] subsets all;
		}
	}`
	if diags := constraintDiags(t, src); len(diags) != 0 {
		t.Fatalf("an unbounded upper bound admits any upper bound, got %v", diags)
	}
}

func TestW7GSubsettingLowerBoundIsNotDiagnosed(t *testing.T) {
	const src = `package M {
		part def P {
			part cap[2..4];
			part few[0..4] subsets cap;
		}
	}`
	if diags := multiplicityDiags(t, src, "redefinition-multiplicity"); len(diags) != 0 {
		t.Fatalf("the lower-bound rule is a redefinition rule only, got %v", diags)
	}
}
