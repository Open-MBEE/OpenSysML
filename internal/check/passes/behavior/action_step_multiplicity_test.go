package behavior_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/behavior"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func actionStepMultiplicityDiags(t *testing.T, text string) []diag.Diagnostic {
	t.Helper()
	sf := source.New("t.sysml", []byte(text))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	index := newTestIndexFromDoc("t.sysml", root)
	ctx := passes.NewContext("t.sysml", index, nil)
	return behavior.ActionStepMultiplicityPass{}.Run(ctx, "t.sysml", root)
}

func TestActionStepMultiplicityPassReportsRuntimeRefusals(t *testing.T) {
	tests := []struct {
		name, code, model, step, multiplicity string
	}{
		{
			name: "unbounded count",
			code: "action-step-multiplicity-not-fixed",
			model: `action def A {
				action a[0..*];
				first start then a;
				then done;
			}`,
			step: "a", multiplicity: "[0..*]",
		},
		{
			name: "open ordering",
			code: "action-step-order-open",
			model: `action def A {
				action p;
				action a[3];
				succession first [0..*] p then [0..*] a;
			}`,
			step: "a", multiplicity: "[3]",
		},
		{
			name: "excluded count",
			code: "action-step-order-unsatisfiable",
			model: `action def A {
				action p;
				action a[3];
				succession first [1] p then [1] a;
			}`,
			step: "a", multiplicity: "[3]",
		},
		{
			name: "guarded edge",
			code: "action-step-multiplicity-unsupported",
			model: `action def A {
				action a[3];
				action q;
				succession first a if true then q;
			}`,
			step: "a", multiplicity: "[3]",
		},
		{
			name: "guarded start edge",
			code: "action-step-multiplicity-unsupported",
			model: `action def A {
				action a[3];
				succession first start if true then a;
			}`,
			step: "a", multiplicity: "[3]",
		},
		{
			name: "guarded done edge",
			code: "action-step-multiplicity-unsupported",
			model: `action def A {
				action a[3];
				succession first start then a;
				succession first a if true then done;
			}`,
			step: "a", multiplicity: "[3]",
		},
		{
			name: "unevaluable succession-end count",
			code: "action-step-multiplicity-not-fixed",
			model: `action def A {
				action p;
				action a[3];
				succession first p then [n] a;
			}`,
			step: "a", multiplicity: "[3]",
		},
		{
			name: "nested external feature read",
			code: "action-step-multiplicity-unsupported",
			model: `package P {
				private import ScalarValues::*;
				action def A {
					attribute total : Integer = 0;
					first start then outer;
					action outer {
						first start then inner;
						action inner[3] { attribute x : Integer = 1; }
						then done;
					}
					then q;
					action q { assign total := outer.inner.x; }
					then done;
				}
			}`,
			step: "inner", multiplicity: "[3]",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := actionStepMultiplicityDiags(t, test.model)
			if len(got) != 1 {
				t.Fatalf("diagnostics = %+v, want one action-step warning", got)
			}
			d := got[0]
			if d.Severity != diag.SeverityWarning || d.Source != "action-step-multiplicity" || d.Code != test.code {
				t.Fatalf("diagnostic = %+v, want warning source action-step-multiplicity code %s", d, test.code)
			}
			if !strings.Contains(d.Message, test.step) || !strings.Contains(d.Message, test.multiplicity) {
				t.Errorf("message = %q, want step %q and multiplicity %q", d.Message, test.step, test.multiplicity)
			}
		})
	}
}

func TestActionStepMultiplicityPassLeavesSupportedStepsAlone(t *testing.T) {
	tests := []struct {
		name, model string
	}{
		{
			name: "no multiplicity",
			model: `action def A {
				first start then a;
				action a;
				then done;
			}`,
		},
		{
			name: "one performance",
			model: `action def A {
				first start then a;
				action a[1];
				then done;
			}`,
		},
		{
			name: "repeated step between start and done",
			model: `action def A {
				first start then a;
				action a[3];
				then done;
			}`,
		},
		{
			name: "performed action step in flow",
			model: `package P {
				private import ScalarValues::*;
				action def Increment {
					attribute c : Integer = 0;
					first start then increment;
					action increment { assign c := c + 1; }
					then done;
				}
				action def A {
					attribute c : Integer = 0;
					first start then p;
					perform action p[2] : Increment;
					then done;
				}
			}`,
		},
		{
			name: "zero at flow boundary",
			model: `action def A {
				first start then a;
				action a[0];
				then done;
			}`,
		},
		{
			name: "named exact bound",
			model: `package P {
				private import ScalarValues::*;
				attribute three : Integer = 3;
				action def A {
					first start then a;
					action a[three];
					then done;
				}
			}`,
		},
		{
			name: "repeated action inside a state entry body",
			model: `package P {
				private import ScalarValues::*;
				state def Machine {
					attribute counter : Integer = 0;
					entry; then active;
					state active {
						entry action begin {
							first start then tick;
							action tick[2] { assign counter := counter + 1; }
							then done;
						}
					}
				}
			}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := actionStepMultiplicityDiags(t, test.model); len(got) != 0 {
				t.Fatalf("diagnostics = %+v, want none", got)
			}
		})
	}
}

func TestActionStepMultiplicityPassChecksStateBehaviorAndPartPerformance(t *testing.T) {
	tests := []struct {
		name, model, step string
	}{
		{
			name: "state entry multiplicity",
			model: `state def Machine {
				state active {
					entry action enter[2] { }
				}
			}`,
			step: "enter",
		},
		{
			name: "part-level performed action multiplicity",
			model: `package P {
				action def Act { }
				part def Host {
					perform action run[2] : Act;
				}
			}`,
			step: "run",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := actionStepMultiplicityDiags(t, test.model)
			if len(got) != 1 || got[0].Code != "action-step-multiplicity-unsupported" {
				t.Fatalf("diagnostics = %+v, want one unsupported-multiplicity warning", got)
			}
			if !strings.Contains(got[0].Message, test.step) || !strings.Contains(got[0].Message, "[2]") {
				t.Errorf("message = %q, want step %s[2]", got[0].Message, test.step)
			}
		})
	}
}
