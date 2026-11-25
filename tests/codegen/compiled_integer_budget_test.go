package codegen_test

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"
)

// A compiled program refuses an Integer past OPENSYSML_MAX_INTEGER_BITS where
// the interpreter does, at the budget's least value and at one a few limbs wide.
func TestCompiledIntegerSizeBudgetMatchesInterpreter(t *testing.T) {
	cases := []compiledCase{
		{"Wide::Pow", []string{"2"}}, {"Wide::Pow", []string{"-2"}}, {"Wide::Pow", []string{"3"}}, {"Wide::Pow", []string{"1"}},
		{"Wide::Fact", []string{"20"}}, {"Wide::Fact", []string{"21"}}, {"Wide::Fact", []string{"34"}}, {"Wide::Fact", []string{"35"}},
		{"Wide::Lit", []string{"0"}}, {"Wide::Lit", []string{"1"}}, {"Wide::Lit", []string{"-4294967296"}},
		{"Wide::Sum", []string{"(9223372036854775807,1)"}}, {"Wide::Sum", []string{"(9223372036854775807,1,-1)"}},
		{"Wide::Product", []string{"(4294967296,4294967296,4294967296,4294967296)"}}, {"Wide::Product", []string{"(4294967296,4294967296,4294967296,4294967295)"}},
		{"Wide::Compare", []string{"340282366920938463463374607431768211456", "1"}}, {"Wide::Compare", []string{"340282366920938463463374607431768211455", "1"}},
		{"Neg", []string{"-9223372036854775808"}}, {"Lib::IntAbs", []string{"-9223372036854775808"}},
		// A range sizes each successor and neither its bounds nor its count.
		{"Wide::Span", []string{"18446744073709551616", "18446744073709551616"}}, {"Wide::Span", []string{"18446744073709551614", "18446744073709551616"}},
		{"Wide::Span", []string{"-18446744073709551616", "-18446744073709551614"}}, {"Wide::Span", []string{"-18446744073709551617", "-18446744073709551616"}},
		{"Wide::SpanSize", []string{"-9223372036854775808", "9223372036854775807"}}, {"Wide::SpanSize", []string{"0", "18446744073709551616"}},
	}
	for _, target := range codegen.Targets() {
		t.Run(string(target), func(t *testing.T) {
			if target == codegen.TargetC {
				if _, err := exec.LookPath("cc"); err != nil {
					t.Skip("no C compiler on PATH")
				}
			}
			s := loadCompileFixture(t)
			refused := 0
			for _, bits := range []int64{runtime.MinMaxIntegerBits, 128} {
				budgets := runtime.DefaultBudgets()
				budgets.MaxIntegerBits = bits
				if err := s.SetBudgets(budgets); err != nil {
					t.Fatal(err)
				}
				t.Setenv(runtime.MaxIntegerBitsEnvVar, strconv.FormatInt(bits, 10))
				for _, c := range cases {
					wantValue, wantFailure := interpreted(t, s, c)
					exe, _ := fixtureCalc(t, s, c.calc, target)
					gotValue, gotFailure := compiledRun(t, exe, c)
					if gotValue != wantValue || gotFailure != wantFailure {
						t.Errorf("%d bits: %s(%s): compiled = (%q, %q), interpreted = (%q, %q)",
							bits, c.calc, strings.Join(c.args, ", "), gotValue, gotFailure, wantValue, wantFailure)
					}
					if strings.Contains(wantFailure, "integer size limit exceeded") {
						refused++
					}
				}
			}
			if refused < 8 {
				t.Errorf("%d runs met the Integer size budget, want the cases to exercise it", refused)
			}
		})
	}
}
