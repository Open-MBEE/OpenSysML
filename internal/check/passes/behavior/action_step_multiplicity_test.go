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
		name, code, model, step, multiplicity, reason string
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
			name: "unordered unbounded count",
			code: "action-step-multiplicity-not-fixed",
			model: `action def A {
				action a[0..*];
			}`,
			step: "a", multiplicity: "[0..*]",
		},
		{
			name: "unordered repeated step with outgoing succession",
			code: "action-step-order-unsatisfiable",
			model: `action def A {
				action a[3] { } then q;
				action q { }
			}`,
			step: "a", multiplicity: "[3]",
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
			name: "written ends exclude single-count step",
			code: "action-step-order-unsatisfiable",
			model: `action def A {
				action p;
				action a[1];
				succession first [2] p then [1] a;
			}`,
			step: "a", multiplicity: "[1]",
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
			name: "while block sequences a repeated step",
			code: "action-step-order-unsatisfiable",
			model: `package P {
				private import ScalarValues::*;
				action def A {
					first start then worker;
					action worker {
						attribute i : Integer = 0;
						while i < 1 {
							action tick[3] { }
							assign i := i + 1;
						}
					}
					then done;
				}
			}`,
			step: "tick", multiplicity: "[3]",
			reason: "the succession's end multiplicities exclude the declared step count",
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
			name: "a fork cannot drive every performance",
			code: "action-step-order-unsatisfiable",
			model: `action def A {
				first start then f;
				fork f;
				action a[3];
				succession first f then a;
				then done;
			}`,
			step: "a", multiplicity: "[3]",
		},
		{
			name: "a written wildcard into a join contradicts its mandate",
			code: "action-step-order-unsatisfiable",
			model: `action def A {
				first start then a;
				action a[3];
				succession first [*] a then j;
				join j;
				then done;
			}`,
			step: "a", multiplicity: "[3]",
			reason: "the succession's written end multiplicity contradicts the one SysML requires at a join node",
		},
		{
			name: "a join waits on another incoming succession",
			code: "action-step-order-unsatisfiable",
			model: `action def A {
				first start then b;
				action b;
				action a[3];
				succession first a then j;
				succession first b then j;
				join j;
				then done;
			}`,
			step: "a", multiplicity: "[3]",
		},
		{
			name: "a merge's successor orders under the per-performance count",
			code: "action-step-order-unsatisfiable",
			model: `action def A {
				first start then a;
				action a[3];
				succession first a then m;
				merge m;
				action q;
				succession first m then q;
				then done;
			}`,
			step: "a", multiplicity: "[3]",
		},
		{
			name: "guarded succession out of a repeated step",
			code: "action-step-multiplicity-unsupported",
			model: `action def A {
				first start then a;
				action a[3];
				action q;
				succession first a if true then q;
				succession first [*] a then [1] done;
			}`,
			step: "a", multiplicity: "[3]",
		},
		{
			name: "guarded succession without a written target end",
			code: "action-step-multiplicity-unsupported",
			model: `action def A {
				first start then p;
				action p;
				action a[3];
				succession first p if true then a;
				succession first [*] a then [1] done;
			}`,
			step: "a", multiplicity: "[3]",
		},
		{
			name: "literal false guard into a repeated step",
			code: "action-step-order-open",
			model: `action def A {
				first start then p;
				action p;
				action a[3];
				succession first p if false then [*] a;
				succession first [*] a then [1] done;
			}`,
			step: "a", multiplicity: "[3]",
			reason: "a false guard leaves the performances of the repeated step unordered with respect to its source",
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
			if test.reason != "" && !strings.Contains(d.Message, test.reason) {
				t.Errorf("message = %q, want reason %q", d.Message, test.reason)
			}
		})
	}
}

func TestActionStepMultiplicityPassReportsUnaddressableBoundAsUnsupported(t *testing.T) {
	got := actionStepMultiplicityDiags(t, `package test {
		action def A {
			first start then a;
			action a[2**70];
			then done;
		}
	}`)
	if len(got) != 1 {
		t.Fatalf("diagnostics = %+v, want one action-step warning", got)
	}
	if got[0].Severity != diag.SeverityWarning || got[0].Source != "action-step-multiplicity" ||
		got[0].Code != "action-step-multiplicity-unsupported" {
		t.Fatalf("diagnostic = %+v, want action-step-multiplicity-unsupported warning", got[0])
	}
	if !strings.Contains(got[0].Message, "1180591620717411303424") {
		t.Errorf("diagnostic message = %q, want the exact bound", got[0].Message)
	}
}

func TestActionStepMultiplicityPassReportsUnaddressableConcurrentStartAsUnsupported(t *testing.T) {
	got := actionStepMultiplicityDiags(t, `action def A {
		action a[2**70];
	}`)
	if len(got) != 1 {
		t.Fatalf("diagnostics = %+v, want one action-step warning", got)
	}
	if got[0].Severity != diag.SeverityWarning || got[0].Source != "action-step-multiplicity" ||
		got[0].Code != "action-step-multiplicity-unsupported" {
		t.Fatalf("diagnostic = %+v, want action-step-multiplicity-unsupported warning", got[0])
	}
	if !strings.Contains(got[0].Message, "1180591620717411303424") {
		t.Errorf("diagnostic message = %q, want the exact bound", got[0].Message)
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
			name: "written ends accept single-count succession",
			model: `action def A {
				action p[1];
				action a[1];
				succession first [1] p then [1] a;
				then done;
			}`,
		},
		{
			name: "single-count action in conditional block",
			model: `action def A {
				first start then worker;
				action worker {
					if true {
						action tick[1] { }
					}
				}
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
			name: "unordered repeated step",
			model: `action def A {
				action a[3];
			}`,
		},
		{
			name: "unordered zero-count step",
			model: `action def A {
				action a[0];
			}`,
		},
		{
			name: "unordered repeated step with nested concurrent starts",
			model: `action def A {
				action a[2] {
					action x;
					action y;
				}
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
			name: "state do multiplicity",
			model: `state def Machine {
				state active {
					do action tick[2] { }
				}
			}`,
			step: "tick",
		},
		{
			name: "unordered state do behavior with a sibling",
			model: `state def Machine {
				state active {
					do action tick[2] { }
					do action sibling { }
				}
			}`,
			step: "tick",
		},
		{
			name: "bodiless state do multiplicity",
			model: `state def Machine {
				state active {
					do action tick[2];
				}
			}`,
			step: "tick",
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

func TestActionStepMultiplicityPassSkipsElementsWithLowerTierFailures(t *testing.T) {
	text := `package P {
		private import ScalarValues::*;
		action def Broken {
			attribute total : Integer = 0;
			first start then a;
			action a[3] { attribute x : Integer = 1; }
			then q;
			action q {
				assign total := a.x;
				assign missing := 1;
			}
			then done;
		}
		action def Independent {
			attribute total : Integer = 0;
			first start then a;
			action a[3] { attribute x : Integer = 1; }
			then q;
			action q { assign total := a.x; }
			then done;
		}
	}`
	sf := source.New("t.sysml", []byte(text))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	ctx := passes.NewContext("t.sysml", newTestIndexFromDoc("t.sysml", root), nil)
	registry := passes.NewRegistry()
	registry.Register(passes.NameResolutionPass{})
	registry.Register(behavior.ActionStepMultiplicityPass{})
	got := registry.Run(ctx, "t.sysml", root)

	var lowerErrors, multiplicityWarnings []diag.Diagnostic
	for _, d := range got {
		switch {
		case d.Severity == diag.SeverityError:
			lowerErrors = append(lowerErrors, d)
		case d.Source == "action-step-multiplicity":
			multiplicityWarnings = append(multiplicityWarnings, d)
		}
	}
	if len(lowerErrors) == 0 {
		t.Fatalf("diagnostics = %+v, want a lower-tier failure in Broken", got)
	}
	if len(multiplicityWarnings) != 1 {
		t.Fatalf("multiplicity warnings = %+v, want only the independent element's warning; all diagnostics: %+v", multiplicityWarnings, got)
	}
	independent := strings.Index(text, "action def Independent")
	if multiplicityWarnings[0].Span.Offset < independent {
		t.Fatalf("multiplicity warning = %+v, want it in Independent after offset %d", multiplicityWarnings[0], independent)
	}
}

func TestActionStepMultiplicityPassAcceptsExecutedRepetition(t *testing.T) {
	tests := []struct {
		name  string
		model string
	}{
		{
			name: "unordered step in a while block",
			model: `action def A {
				first start then worker;
				action worker {
					while true {
						action anchor;
						first start then anchor;
						action tick[3] { }
					}
				}
				then done;
			}`,
		},
		{
			name: "while block ignores zero count",
			model: `action def A {
				first start then worker;
				action worker {
					attribute i : Integer = 0;
					while i < 1 {
						action tick[0] { }
						assign i := i + 1;
					}
				}
				then done;
			}`,
		},
		{
			name: "if block ignores repeated count",
			model: `action def A {
				first start then worker;
				action worker {
					if true {
						action tick[3] { }
					}
				}
				then done;
			}`,
		},
		{
			name: "if block ignores zero count",
			model: `action def A {
				first start then worker;
				action worker {
					if true {
						action tick[0] { }
					}
				}
				then done;
			}`,
		},
		{
			name: "nested external feature read",
			model: `package P {
				private import ScalarValues::*;
				private import SequenceFunctions::*;
				action def A {
					attribute total : Integer = 0;
					first start then outer;
					action outer {
						first start then inner;
						action inner[3] { attribute x : Integer = 1; }
						then done;
					}
					then q;
					action q { assign total := size(outer.inner.x); }
					then done;
				}
			}`,
		},
		{
			name: "part-level performed action multiplicity",
			model: `package P {
				action def Act { }
				part def Host {
					perform action run[2] : Act;
				}
			}`,
		},
		{
			name: "part-level performed action nested in part usage",
			model: `package P {
				action def Act { }
				part def Camera { }
				part def Host {
					part camera : Camera {
						perform action takePhoto[2] : Act;
					}
				}
			}`,
		},
		{
			name: "part-level performed action on top-level part usage",
			model: `package P {
				action def Act { }
				part def Camera { }
				part camera : Camera {
					perform action takePhoto[2] : Act;
				}
			}`,
		},
		{
			name: "repeated step behind a fork barrier",
			model: `action def A {
				first start then a;
				action a[3];
				succession first [*] a then f;
				fork f;
				then done;
			}`,
		},
		{
			name: "repeated step behind a decision barrier",
			model: `action def A {
				first start then a;
				action a[3];
				succession first [*] a then d;
				decide d;
				if true then done;
			}`,
		},
		{
			name: "repeated step fanned out of a merge",
			model: `action def A {
				first start then p;
				action p;
				merge m;
				first p then m;
				action a[3];
				succession first m then [*] a;
				then done;
			}`,
		},
		{
			name: "every performance crosses a lone join",
			model: `action def A {
				first start then a;
				action a[3];
				succession first a then j;
				join j;
				then done;
			}`,
		},
		{
			name: "guarded succession with a written target end",
			model: `action def A {
				first start then p;
				action p;
				action a[3];
				succession first p if true then [*] a;
				succession first [*] a then [1] done;
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
