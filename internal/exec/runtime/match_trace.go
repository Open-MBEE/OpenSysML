package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TraceRow is the initial stable checkpoint (no signal) or the checkpoint after one signal.
// Observe uses the same nonempty scalar projection on every row; it never writes the model.
type TraceRow struct {
	Signal  string
	Args    map[string]Value
	Observe map[string]Value
}

// TraceInputError identifies a row that cannot be interpreted against the model.
type TraceInputError struct {
	Row int
	Err error
}

func (e *TraceInputError) Error() string { return fmt.Sprintf("trace row %d: %v", e.Row, e.Err) }
func (e *TraceInputError) Unwrap() error { return e.Err }

type TraceStatus string

const (
	TraceMatched      TraceStatus = "matched"
	TraceNoMatch      TraceStatus = "no_match"
	TraceInconclusive TraceStatus = "inconclusive"
)

// TraceReport.Row is the length of the longest matching prefix (one-based row
// number, or zero if none match). In a no-match report, Row+1 is impossible.
type TraceReport struct {
	Status  TraceStatus
	Row     int
	Moves   int
	Reason  string
	Witness []string
}

// MatchTrace searches for one untimed machine execution explaining the trace, not an invariant.
// Signals arrive on the machine's own queue; zero bounds use finite defaults.
func MatchTrace(stop context.Context, fresh func() (*Context, error), start Starter, rows []TraceRow, budget CheckBudget) (TraceReport, error) {
	keys, err := traceKeys(rows)
	if err != nil {
		return TraceReport{}, err
	}
	if budget.Depth < 0 || budget.States < 0 {
		return TraceReport{}, fmt.Errorf("trace bounds must be non-negative")
	}
	if budget.Depth == 0 {
		budget.Depth = 512
	}
	if budget.States == 0 {
		budget.States = 10000
	}
	s := &traceSearch{stop: stop, rows: rows, keys: keys, budget: budget}
	starts := [][]int{nil}
	for cursor := 0; cursor < len(starts); cursor++ {
		if !s.allow(0) {
			break
		}
		more, err := s.from(fresh, start, starts[cursor])
		if stop.Err() != nil {
			break
		}
		if err != nil {
			return TraceReport{}, err
		}
		if s.report.Status == TraceMatched {
			return s.report, nil
		}
		starts = append(starts, more...)
	}
	if err := stop.Err(); err != nil {
		s.report.Reason = err.Error()
		s.report.Witness = nil
	}
	if s.report.Reason != "" {
		s.report.Status = TraceInconclusive
	} else {
		s.report.Status = TraceNoMatch
	}
	return s.report, nil
}

type traceSearch struct {
	stop    context.Context
	rows    []TraceRow
	keys    []string
	budget  CheckBudget
	run     *invocationRun
	signals []Message
	visits  int
	report  TraceReport
}

func traceKeys(rows []TraceRow) ([]string, error) {
	if len(rows) == 0 || rows[0].Signal != "" || len(rows[0].Args) != 0 || len(rows[0].Observe) == 0 {
		return nil, fmt.Errorf("trace needs an initial observation without a signal or arguments")
	}
	keys := sortedArgNames(rows[0].Observe)
	for i, row := range rows {
		if i > 0 && row.Signal == "" {
			return nil, fmt.Errorf("trace row %d needs a signal", i+1)
		}
		if len(row.Observe) != len(keys) {
			return nil, fmt.Errorf("trace row %d must observe the same features as the initial row", i+1)
		}
		for _, key := range keys {
			if key == "" || !traceScalar(row.Observe[key]) {
				return nil, fmt.Errorf("trace row %d needs scalar evidence for %q", i+1, key)
			}
		}
		for _, key := range sortedArgNames(row.Args) {
			if key == "" || !traceScalar(row.Args[key]) {
				return nil, fmt.Errorf("trace row %d needs a scalar value for argument %q", i+1, key)
			}
		}
	}
	return keys, nil
}

func traceScalar(v Value) bool {
	if v.Kind == ValString {
		return true
	}
	if v.Kind != ValConst {
		return false
	}
	switch v.Const.Kind {
	case semantics.ValInt, semantics.ValBool:
		return true
	default:
		return false
	}
}

func traceEqual(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == ValString {
		return a.Str() == b.Str()
	}
	if a.Const.Kind != b.Const.Kind {
		return false
	}
	switch a.Const.Kind {
	case semantics.ValInt:
		return semantics.CompareInt(a.Const, b.Const) == 0
	case semantics.ValBool:
		return a.Const.Bool == b.Const.Bool
	default:
		return false
	}
}

func (s *traceSearch) allow(depth int) bool {
	switch {
	case s.stop.Err() != nil:
		s.note(s.stop.Err().Error())
	case depth > s.budget.Depth:
		s.note("reached the move limit (-check-depth)")
	case s.visits >= s.budget.States:
		s.note("reached the state visit limit (-check-states)")
	default:
		return true
	}
	return false
}

func (s *traceSearch) note(reason string) {
	if s.report.Reason == "" {
		s.report.Reason = reason
	}
}

func (s *traceSearch) from(fresh func() (*Context, error), start Starter, picks []int) ([][]int, error) {
	ctx, err := fresh()
	if err != nil {
		return nil, err
	}
	if err = ctx.SetSchedule(checkPolicy(&checkScript{due: -1, picks: picks})); err != nil {
		return nil, err
	}
	previous := ctx.traceStop
	ctx.traceStop = s.stop
	defer func() { ctx.traceStop = previous }()
	run, err := beginInvocation(ctx, start)
	var setup *SetupError
	if errors.As(err, &setup) {
		return nil, err
	}
	if run.inv != nil {
		defer run.inv.Release()
	}
	if s.stop.Err() != nil {
		return nil, nil
	}
	check := run.checking()
	if check.refused != nil {
		return nil, check.refused
	}
	var more [][]int
	for i, c := range check.drawn {
		for alt := 1; alt < len(c.Alternatives); alt++ {
			more = append(more, slices.Concat(picks, make([]int, i), []int{alt}))
		}
	}
	if err != nil {
		// A failed startup is a spent visit, not evidence that another order
		// could not match. Successful startups are charged by walk instead.
		s.visits++
		s.note(fmt.Sprintf("starting model: %v", err))
		return more, nil
	}
	if run.inv == nil {
		return nil, fmt.Errorf("trace starter returned no invocation")
	}
	if err = run.inv.started(ctx); err != nil {
		return nil, err
	}
	if len(run.inv.Actions) != 0 || len(run.inv.States) != 1 || run.inv.States[0].self == nil || run.inv.Horizon != (Horizon{}) {
		return nil, fmt.Errorf(`trace checking needs one untimed machine on a performing object; name it as -state "Machine object"`)
	}
	s.run = run
	s.signals = make([]Message, len(s.rows))
	for i, row := range s.rows[1:] {
		segments, ok := source.QualifiedNameSegments(row.Signal)
		if !ok {
			return nil, &TraceInputError{Row: i + 2, Err: fmt.Errorf("malformed signal name %q", row.Signal)}
		}
		decl := ctx.model.Resolver().ResolveGlobal(segments)
		if len(decl) != 1 || !IsSignalDefinition(decl[0]) {
			return nil, &TraceInputError{Row: i + 2, Err: fmt.Errorf("%q must resolve to one signal definition", row.Signal)}
		}
		msg, err := ctx.SignalMessage(decl[0], row.Args, run.inv.States[0].self)
		if err != nil {
			return nil, &TraceInputError{Row: i + 2, Err: err}
		}
		s.signals[i+1] = msg
	}
	var path []string
	if len(picks) > 0 {
		path = []string{fmt.Sprintf("start picks %v", picks)}
	}
	return more, s.walk(0, 0, path)
}

func (s *traceSearch) walk(row, depth int, path []string) error {
	if !s.allow(depth) {
		return nil
	}
	s.visits++
	r := s.run
	if len(r.inv.executors()) != 1 {
		s.note("additional active executors are outside single-machine trace matching")
		return nil
	}
	if len(r.ctx.gaps()) != 0 {
		s.note("runtime schedule coverage gap")
		return nil
	}
	moves := r.enabledMoves()
	if len(moves) == 0 {
		if _, waiting := r.ctx.clock.NextDue(); waiting {
			s.note("pending timed work is outside untimed trace matching")
			return nil
		}
		if err := r.deadlock(); err != nil {
			s.note(err.Error())
			return nil
		}
		matches, err := s.observes(row)
		if stopErr := s.stop.Err(); stopErr != nil {
			s.note(stopErr.Error())
			return nil
		}
		if err != nil || !matches {
			return err
		}
		s.report.Row = max(s.report.Row, row+1)
		if row+1 == len(s.rows) {
			s.report.Status = TraceMatched
			s.report.Reason = ""
			s.report.Witness = slices.Clone(path)
			return nil
		}
		input := s.rows[row+1]
		// Use this machine's queue, not the shared bus.
		// Dispatch, deferral and dropping follow the machine's ordinary semantics.
		r.inv.States[0].enqueueSignal(s.signals[row+1])
		return s.walk(row+1, depth, append(path, input.Signal))
	}
	snap, err := r.inv.snapshot()
	if err != nil {
		return err
	}
	defer snap.Release()
	f := checkFrame{turn: r.turn}
	for i, m := range moves {
		f.all = append(f.all, searchMove{enabledMove: m, name: fmt.Sprint(i)})
	}
	f.moves = slices.Clone(f.all)
	for f.next < len(f.moves) {
		if !s.allow(depth + 1) {
			break
		}
		m := f.moves[f.next]
		f.next++
		snap.Restore()
		r.turn = f.turn
		drawn, err := r.makeMove(m.enabledMove)
		s.report.Moves++
		f.reveal(m, drawn)
		if err != nil {
			s.note(err.Error())
			continue
		}
		if err = s.walk(row, depth+1, append(path, fmt.Sprintf("%s %v", m.Label, m.Picks))); err != nil {
			return err
		}
		if s.report.Status == TraceMatched {
			return nil
		}
	}
	return nil
}

var errTraceObservationOrder = errors.New("reading this observation can follow more than one calculation order")

func (s *traceSearch) observes(row int) (bool, error) {
	r := s.run
	defer r.enter()()
	// Use the native checker probe to roll back reads and identify order-dependent values.
	// Only one evaluation is needed: v1 refuses a read that itself needs scheduler choices.
	var matches bool
	err := r.ctx.sweepStatementOrderVariants(func(sweep *statementOrderSweep) error {
		var err error
		matches, err = s.readObservation(row)
		if len(sweep.choices) > 0 {
			return errTraceObservationOrder
		}
		return err
	})
	if errors.Is(err, errTraceObservationOrder) {
		s.note(fmt.Sprintf("trace row %d: %v", row+1, err))
		return false, nil
	}
	return matches, err
}

func (s *traceSearch) readObservation(row int) (bool, error) {
	r := s.run
	matches := true
	for _, key := range s.keys {
		fv, err := r.inv.States[0].self.GetFeatureValue(r.ctx, key)
		if err != nil {
			var materialization *FeatureValueError
			if errors.Is(err, ErrNoSuchFeature) && !errors.As(err, &materialization) {
				return false, &TraceInputError{Row: row + 1, Err: err}
			}
			s.note(fmt.Sprintf("trace row %d: cannot observe %q: %v", row+1, key, err))
			return false, nil
		}
		if fv.Values.Kind != ValInvalid || !traceScalar(fv.Value) {
			s.note(fmt.Sprintf("trace row %d: %q did not resolve to a supported observation (Boolean, string, or integer)", row+1, key))
			return false, nil
		}
		matches = matches && traceEqual(fv.Value, s.rows[row].Observe[key])
	}
	return matches, nil
}
