package smt

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
)

func TestEngineDoesNotCoverAtomicBodyStatementOrder(t *testing.T) {
	tests := []struct {
		name, source, behavior, condition, reason string
	}{
		{
			name:   "action invokes a reordering calc",
			reason: ErrNotEncoded.Error(),
			source: `package test {
				private import ScalarValues::*;
				calc def Ord {
					return : Integer;
					attribute y : Integer := 1;
					assign y := y * 10;
					assign y := y + 2;
					y
				}
				action def Use {
					out attribute r : Integer := 0;
					assign r := Ord();
				}
			}`,
			behavior: "test::Use",
		},
		{
			name:   "calc usage has a reordering body",
			reason: ErrNotEncoded.Error(),
			source: `package test {
				private import ScalarValues::*;
				calc def Ord {
					return : Integer;
					attribute y : Integer := 1;
					assign y := y * 10;
					assign y := y + 2;
					y
				}
				calc def Caller {
					return : Integer;
					calc ord : Ord;
					ord
				}
				action def Use {
					out attribute r : Integer := 0;
					assign r := Caller();
				}
			}`,
			behavior: "test::Use",
		},
		{
			name:   "constraint body has steps",
			reason: "constraint body steps not translatable for solving",
			source: `package test {
				private import ScalarValues::*;
				action def A {
					first start;
					done;
					succession first start then done;
					constraint ok {
						attribute y : Integer := 1;
						assign y := y * 10;
						assign y := y + 2;
						y == 12
					}
				}
			}`,
			behavior:  "test::A",
			condition: "test::A::ok",
		},
	}
	e := New(func() (*solve.Solver, error) {
		return &solve.Solver{Name: "unavailable", Path: "/nonexistent"}, nil
	})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := indexed(t, "atomic_body_order.sysml", test.source)
			result := answer(t, e, d, d.holds(t, test.behavior, test.condition), analysis.Budget{Depth: 4})
			expect(t, result, analysis.ClaimNone, analysis.NotCovered)
			if !strings.Contains(result.Reason, test.reason) {
				t.Fatalf("reason %q does not identify the unsupported encoding", result.Reason)
			}
		})
	}
}
