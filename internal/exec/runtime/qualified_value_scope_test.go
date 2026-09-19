package runtime

import "testing"

// TestQualifiedValueReadsItsOwnScope evaluates qualified names from outside the
// package that declares them: a value expression is written in its own scope, so
// the units and enumerations imported there answer it however it is read.
func TestQualifiedValueReadsItsOwnScope(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package vehicles {
			public import ISQ::*;
			public import SI::*;

			enum def Color {
				enum red;
				enum blue;
			}

			part def Car {
				attribute mass : MassValue;
				attribute color : Color;
			}

			part sedan : Car {
				attribute redefines mass = 1600.0 [kg];
				attribute redefines color = Color::blue;
			}
		}
	`))
	root := idx.DocumentRoot("<test>")

	cases := []struct {
		src  string
		want string
	}{
		{"vehicles::sedan::mass", "1600.0 [kg]"},
		{"vehicles::sedan::color", "Color::blue"},
	}
	for _, tc := range cases {
		val, err := evalIn(t, ctx, root, tc.src)
		if err != nil {
			t.Fatalf("eval %s: %v", tc.src, err)
		}
		if got := FormatValue(val); got != tc.want {
			t.Errorf("eval %s = %s, want %s", tc.src, got, tc.want)
		}
	}
}
