package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestRuntimeRobustnessActionResultParameter(t *testing.T) {
	t.Run("inherited_result_never_written", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
			calc def Answer { return r : Integer; }
			action def Silent :> Answer {
				first start;
				then done;
			}
		}`))
		outputs, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Silent"))
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if _, ok := outputs["r"]; ok {
			t.Errorf("outputs = %v, want no value for unwritten inherited result r", outputs)
		}
	})

	t.Run("performer_reads_unwritten_inherited_result", func(t *testing.T) {
		run := func(t *testing.T, direction string) map[string]Value {
			t.Helper()
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
				calc def Answer { `+direction+` r : Integer; }
				action def Silent :> Answer {
					first start;
					then done;
				}
				action def Host {
					out got : Integer;
					first start;
					then action s : Silent;
					then action read { assign got := s.r; }
					then done;
				}
			}`))
			outputs, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Host"))
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			return outputs
		}
		inherited := run(t, "return")
		ordinaryOutput := run(t, "out")
		if len(inherited) != len(ordinaryOutput) {
			t.Fatalf("inherited result outputs = %v, ordinary out outputs = %v", inherited, ordinaryOutput)
		}
		for name, got := range inherited {
			want, ok := ordinaryOutput[name]
			if !ok || !valueEqual(got, want) {
				t.Errorf("inherited result output %s = %v, ordinary out output = %v", name, got, want)
			}
		}
		got, ok := inherited["got"]
		if !ok || got.Kind != ValSequence || got.Sequence().Size() != 0 {
			t.Errorf("got = %v, want the empty sequence after reading the unwritten result", got)
		}
	})

	t.Run("declared_return_beside_inherited_result_is_refused", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
			calc def Answer { return r : Integer; }
			action def Bad :> Answer {
				return r2 : Integer;
				first start;
				then done;
			}
		}`))
		_, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Bad"))
		if !errors.Is(err, ErrActionResultParameter) {
			t.Fatalf("ExecuteAction error = %v, want ErrActionResultParameter", err)
		}
		if !strings.Contains(err.Error(), "declares `return r2`") {
			t.Errorf("error = %v, want it to name the declared return parameter", err)
		}
	})

}
