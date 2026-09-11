package passes

import "testing"

// A bare feature reference is typed by the effective type of the feature it names —
// declared, inherited, redefined, subsetted or valued — and judged as a literal would be.
func TestBareFeatureReferenceIsTypedByItsFeature(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"declared", `attribute s : ScalarValues::String; attribute r = -s;`},
		{"subsetting", `attribute s : ScalarValues::String; attribute t :> s; attribute r = -t;`},
		{"valued", `attribute s = "x"; attribute r = -s;`},
		{"chain-inherited", `attribute def A { attribute n : ScalarValues::String; } attribute def B :> A; attribute b : B; attribute r = -b.n;`},
		{"chain-redefined", `attribute def A { attribute n : ScalarValues::String; } attribute def B :> A { attribute :>> n; } attribute b : B; attribute r = -b.n;`},
		{"alias", `attribute s = "x"; alias t for s; attribute r = -t;`},
		{"subsetting-valued", `attribute s = "x"; attribute u :> s; attribute r = -u;`},
		{"redefined-valued", `attribute def A { attribute n = "x"; } attribute def B :> A { attribute :>> n; } attribute b : B; attribute r = -b.n;`},
		{"subsetting-inherited-valued", `attribute def A { attribute n = "x"; } attribute def C :> A { attribute m :> n; } attribute c : C; attribute r = -c.m;`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantOneDiag(t, `package P { `+tc.src+` }`,
				"operator '-' requires a numeric operand, found String")
		})
	}
}

func TestBareFeatureReferenceComparisonIsJudged(t *testing.T) {
	wantOneWarning(t, `package P {
		attribute s : ScalarValues::String;
		attribute b = s == 1;
	}`, "type.expr", "comparing String with Natural is always false")
}

// A scalar-typed condition is judged by inference, naming the type found; a
// feature of no known type is left to the executor.
func TestBareFeatureReferenceConditionIsJudgedStatically(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def C {
			in n : ScalarValues::String;
			while n { return : ScalarValues::Integer = 1; }
			return : ScalarValues::Integer = 0;
		}
	}`, "condition of 'while' must be Boolean, found String")
	wantNoDiags(t, `package P {
		attribute u;
		attribute r = -u;
		attribute b = u == 1;
		calc def C {
			in n;
			while n { return : ScalarValues::Integer = 1; }
			return : ScalarValues::Integer = 0;
		}
	}`)
}

// A computed value is judged against a scalar-typed feature by the runtime's write
// conformance classification: only a statically disjoint type is reported.
func TestComputedValueConformanceToScalarFeature(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def GetReal { return : ScalarValues::Real = 1.0; }
		attribute s : ScalarValues::String = GetReal();
	}`, "cannot bind Real value to a feature typed by String")
	wantOneDiag(t, `package P {
		calc def GetReal { return : ScalarValues::Real = 1.0; }
		calc def Again :> GetReal;
		attribute s : ScalarValues::String = Again();
	}`, "cannot bind Real value to a feature typed by String")
	wantOneDiag(t, `package P {
		attribute a : ScalarValues::Real = 1.0;
		attribute s : ScalarValues::String = a + 1.0;
	}`, "cannot bind Real value to a feature typed by String")
	wantOneDiag(t, `package P {
		part def E;
		part e : E;
		calc def GetE { return : E = e; }
		attribute s : ScalarValues::String = GetE();
	}`, "cannot bind a value of type E to a feature typed by String")
}

// Numeric results conform along the lattice in either direction, as the values
// may; a behavior declaring no result leaves the binding unjudged.
func TestComputedValueConformanceLeavesOverlapAndUnknownAlone(t *testing.T) {
	wantNoDiags(t, `package P {
		calc def GetInt { return : ScalarValues::Integer = 1; }
		calc def GetReal { return : ScalarValues::Real = 1.0; }
		calc def NoResult { in x : ScalarValues::Integer; }
		action def Build;
		attribute a : ScalarValues::Real = 1.0;
		attribute r : ScalarValues::Real = GetInt();
		attribute i : ScalarValues::Integer = GetReal();
		attribute sum : ScalarValues::Real = a + 1.0;
		attribute s : ScalarValues::String = NoResult(1);
		attribute t : ScalarValues::String = Build();
	}`)
}
