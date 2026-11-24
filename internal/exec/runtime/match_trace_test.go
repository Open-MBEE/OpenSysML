package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

func TestMatchTraceCancellationDuringStartupIsInconclusive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := parseExploreModel(t, "package test {}")
	rows := []TraceRow{{Observe: map[string]Value{"screen": NewStringValue("blank")}}}
	starter := func(*Context) (*Invocation, error) {
		cancel()
		return nil, errors.New("startup stopped")
	}

	report, err := MatchTrace(ctx, model.fresh, starter, rows, CheckBudget{})
	if err != nil || report.Status != TraceInconclusive || report.Reason != context.Canceled.Error() {
		t.Fatalf("got report %+v, err %v; want inconclusive cancellation", report, err)
	}
}

func TestMatchTraceRetriesFailedStartupOrder(t *testing.T) {
	model := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		part def Host {
			attribute reading : Integer = -1;
			exhibit state machine {
				entry; then work;
				state work parallel {
					state read {
						entry action setRead { assign reading := 0; }
					}
					state arm {
						entry action setArmed { assign reading := 0; }
					}
				}
			}
		}
		part host : Host;
	}`)
	host := oneSymbol(t, model.idx, "test::host")
	machine := oneSymbol(t, model.idx, "test::Host::machine")
	for _, tc := range []struct {
		name   string
		want   int64
		budget int
		status TraceStatus
		starts int
	}{
		{"matching sibling", 0, 0, TraceMatched, 2},
		{"no matching sibling but one failed", 2, 0, TraceInconclusive, 2},
		{"failed startup consumes state visit", 0, 1, TraceInconclusive, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			starts := 0
			start := func(ctx *Context) (*Invocation, error) {
				starts++
				self, err := ctx.Instantiate(host)
				if err != nil {
					return nil, err
				}
				// Inject a failure after the real scheduler chose the first
				// region-entry order. Choice enumeration should survive it.
				if starts == 1 {
					return nil, errors.New("injected error after first startup choice")
				}
				exhibited := self.ExhibitedStatesOf(machine)
				if len(exhibited) != 1 || exhibited[0].State == nil {
					t.Fatalf("startup %d: expected one exhibited machine: %v", starts, exhibited)
				}
				return &Invocation{States: []*StateExecutor{exhibited[0].State}}, nil
			}
			rows := []TraceRow{{Observe: map[string]Value{"reading": {Kind: ValConst, Const: semantics.IntValue(tc.want)}}}}
			report, err := MatchTrace(context.Background(), model.fresh, start, rows, CheckBudget{States: tc.budget})
			if err != nil || report.Status != tc.status || starts != tc.starts {
				t.Fatalf("got report %+v, err %v, starts %d; want %s and %d starts", report, err, starts, tc.status, tc.starts)
			}
			if tc.status == TraceMatched && (report.Row != 1 || len(report.Witness) == 0 || report.Witness[0] != "start picks [1]") {
				t.Fatalf("matching startup witness: %+v", report)
			}
		})
	}
}

func TestMatchTraceFatalSetupError(t *testing.T) {
	model := parseExploreModel(t, "package test {}")
	rows := []TraceRow{{Observe: map[string]Value{"screen": NewStringValue("blank")}}}
	setup := &SetupError{Err: errors.New("no machine could be created")}
	_, err := MatchTrace(context.Background(), model.fresh, func(*Context) (*Invocation, error) {
		return nil, setup
	}, rows, CheckBudget{})
	if !errors.Is(err, setup) {
		t.Fatalf("got %v; want fatal setup error", err)
	}
}

func TestMatchTraceCancellationReachesEvaluator(t *testing.T) {
	stop, cancel := context.WithCancel(context.Background())
	defer cancel()
	checks := 0
	armed := false
	observed := traceEvalStop{Context: stop, poll: func() {
		if armed {
			checks++
			if checks == 2 {
				cancel()
			}
		}
	}}
	model := parseExploreModel(t, `package test { attribute x = 1; }`)
	rows := []TraceRow{{Observe: map[string]Value{"screen": NewStringValue("blank")}}}
	x := oneSymbol(t, model.idx, "test::x")
	expr := parseExpr(t, "x + 2")
	var runCtx *Context
	starter := func(ctx *Context) (*Invocation, error) {
		runCtx = ctx
		armed = true
		_, err := ctx.EvalWithScope(expr, x.OwnerScope)
		armed = false
		if !errors.Is(err, ErrInterrupted) {
			t.Errorf("evaluation interrupted on its second step = %v; want ErrInterrupted", err)
		}
		return nil, err
	}
	report, err := MatchTrace(observed, model.fresh, starter, rows, CheckBudget{})
	if err != nil || report.Status != TraceInconclusive || report.Reason != context.Canceled.Error() {
		t.Fatalf("got report %+v, err %v; want inconclusive cancellation", report, err)
	}
	if runCtx == nil || checks != 2 {
		t.Fatalf("trace did not reach the second evaluation step: %d checks", checks)
	}
	if _, err := runCtx.EvalWithScope(expr, x.OwnerScope); err != nil {
		t.Fatalf("trace leaked cancellation into later evaluation: %v", err)
	}
}

// traceEvalStop cancels a real context from an evaluation step without racing a timer.
type traceEvalStop struct {
	context.Context
	poll func()
}

func (ctx traceEvalStop) Err() error {
	ctx.poll()
	return ctx.Context.Err()
}
