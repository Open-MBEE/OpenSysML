package lower

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestRunToCompletionInheritanceAndOverrides(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Machine {
				attribute :>> isRunToCompletion = false;
				entry; then outer;
				state outer {
					state inherited;
					state overridden {
						attribute :>> isRunToCompletion = true;
					}
				}
			}
		}
	`, "Machine")

	if config := graph.RunToCompletionOf(nil); config.Value == nil {
		t.Fatal("machine RTC value is absent")
	}
	if config := graph.RunToCompletionOf(stateNamed(graph, "inherited")); config.Value == nil {
		t.Fatal("inherited RTC value is absent")
	}
	if config := graph.RunToCompletionOf(stateNamed(graph, "overridden")); config.Value == nil {
		t.Fatal("substate RTC override is absent")
	}
}

func TestRunToCompletionScopes(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Machine {
				entry; then outer;
				state outer {
					attribute :>> runToCompletionScope = self;
					state inner;
					state localSelf {
						ref :>> runToCompletionScope = self;
					}
					state sibling {
						ref :>> runToCompletionScope = outer;
					}
				}
			}
		}
	`, "Machine")

	outer := stateNamed(graph, "outer")
	inner := stateNamed(graph, "inner")
	localSelf := stateNamed(graph, "localSelf")
	sibling := stateNamed(graph, "sibling")
	if got := graph.RunToCompletionOf(outer).Scope; got != outer {
		t.Fatalf("outer self scope = %v, want outer", got)
	}
	if got := graph.RunToCompletionOf(inner).Scope; got != outer {
		t.Fatalf("inherited outer scope = %v, want outer", got)
	}
	if got := graph.RunToCompletionOf(localSelf).Scope; got != localSelf {
		t.Fatalf("substate self scope = %v, want localSelf", got)
	}
	if got := graph.RunToCompletionOf(sibling).Scope; got != outer {
		t.Fatalf("named ancestor scope = %v, want outer", got)
	}
}

func TestRunToCompletionMachineSelfScopeIsWholeMachine(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Machine {
				ref :>> runToCompletionScope = self;
				entry; then idle;
				state idle;
			}
		}
	`, "Machine")
	if got := graph.RunToCompletionOf(nil).Scope; got != nil {
		t.Fatalf("machine self scope = %v, want nil", got)
	}
}

func TestTransitionOwnerRecordsDeclaringBody(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Machine {
				entry; then qualified;
				state qualified {
					state inner;
				}
				state work {
					state nested;
					state after;
					transition first nested then after;
				}
				state regionWork parallel {
					state left {
						entry; then l;
						state l;
					}
					state right {
						entry; then r2;
						state r2;
					}
					choice pick;
					transition first pick then l;
				}
				state done;
				transition first qualified.inner accept Go then done;
			}
		}
	`, "Machine")

	var machineBody, workBody, regionBody *Transition
	for source, transitions := range graph.Transitions {
		for _, transition := range transitions {
			switch sourceName := source.(type) {
			case *ast.StateNode:
				switch sourceName.Name {
				case "inner":
					machineBody = transition
				case "nested":
					workBody = transition
				}
			case *ast.PseudostateNode:
				if sourceName.Name == "pick" {
					regionBody = transition
				}
			}
		}
	}
	if machineBody == nil || machineBody.Owner != nil {
		t.Fatalf("qualified machine-body transition owner = %v, want nil", machineBody)
	}
	if workBody == nil || workBody.Owner == nil || workBody.Owner.Name != "work" {
		t.Fatalf("work-body transition owner = %v, want work", workBody)
	}
	if regionBody == nil || regionBody.Owner == nil || regionBody.Owner.Name != "regionWork" {
		var ownerName string
		if regionBody != nil && regionBody.Owner != nil {
			ownerName = regionBody.Owner.Name
		}
		t.Fatalf("region transition owner = %q, want regionWork", ownerName)
	}
}

func TestRunToCompletionScopeRefusals(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		notAncestor bool
	}{
		{
			name: "sibling",
			src: `
				package test {
					state def Machine parallel {
						state left { state l; }
						state right {
							ref :>> runToCompletionScope = left::l;
							state r;
						}
					}
				}
			`,
			notAncestor: true,
		},
		{
			name: "attribute",
			src: `
				package test {
					state def Machine {
						attribute target;
						ref :>> runToCompletionScope = target;
						entry; then idle;
						state idle;
					}
				}
			`,
		},
		{
			name: "missing",
			src: `
				package test {
					state def Machine {
						ref :>> runToCompletionScope = Missing;
						entry; then idle;
						state idle;
					}
				}
			`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := stateGraphErr(t, test.src, "Machine")
			var refusal *RunToCompletionRedefinition
			if !errors.As(err, &refusal) {
				t.Fatalf("error = %v, want RunToCompletionRedefinition", err)
			}
			if refusal.NotAncestor != test.notAncestor {
				t.Fatalf("NotAncestor = %t, want %t", refusal.NotAncestor, test.notAncestor)
			}
			if !errors.Is(err, ErrUnsupportedStateContent) {
				t.Fatalf("error = %v, want unsupported state content", err)
			}
		})
	}
}
