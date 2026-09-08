package runtime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/envvar"
)

// Default bounds on one run. Each one stops a different kind of runaway, so each
// counts a different thing and has its own variable.
//
// The step and event bounds are sized by how long a runaway takes to report
// rather than by memory: those steps allocate nothing that outlives them, and
// the only thing they make grow is a %trace, at 34-83 bytes an entry. Measured
// rates are ~13.6M evaluation steps/s and ~1.9M events/s, so each default
// reports a runaway within about a second, and a fully traced run at those four
// ceilings holds ~320MB.
//
// Collection elements are the exception, and MaxElements is the bound that reads
// as memory: a materialized element is a 104-byte Value living as long as the
// collection holding it, and `1..10000000` conjures one per step. It counts the
// elements one evaluation holds, not the elements a run produced in total, so a
// loop or a state machine building a small collection each step is not stopped by
// the steps before it.
const (
	// DefaultMaxSteps bounds expression evaluations.
	DefaultMaxSteps int64 = 10000000
	// DefaultMaxActionSteps bounds the token-flow steps one action run performs.
	DefaultMaxActionSteps int64 = 1000000
	// DefaultMaxStateEvents bounds the events one state machine run dispatches.
	DefaultMaxStateEvents int64 = 1000000
	// DefaultMaxDoSteps bounds the do actions one state machine run
	// performs.
	DefaultMaxDoSteps int64 = 5000000
	// DefaultMaxElements bounds the collection elements one evaluation holds,
	// ~104MB of Values.
	DefaultMaxElements int64 = 1000000
	// DefaultMaxCalcDepth bounds the nested calc invocations one evaluation holds
	// on the stack, ~10KB each.
	DefaultMaxCalcDepth int64 = 10000
	// DefaultMaxSweepRuns bounds the runs one parameter sweep or sample makes,
	// each of which is a whole analysis or calc run of its own.
	DefaultMaxSweepRuns int64 = 1000
)

// MaxCalcDepthCeiling is the highest calc depth budget a run may be given: past
// it the goroutine stack, whose exhaustion is fatal, would stop a runaway first.
const MaxCalcDepthCeiling int64 = 25000

// Environment variables overriding the defaults above, following the
// OPENSYSML_LIBRARY_PATH convention. The legacy SYSML_-prefixed names remain
// accepted via envvar.Lookup, with the OPENSYSML_ name winning when both are set.
const (
	MaxStepsEnvVar       = "OPENSYSML_MAX_STEPS"
	MaxActionStepsEnvVar = "OPENSYSML_MAX_ACTION_STEPS"
	MaxStateEventsEnvVar = "OPENSYSML_MAX_EVENTS"
	MaxDoStepsEnvVar     = "OPENSYSML_MAX_DO_STEPS"
	MaxElementsEnvVar    = "OPENSYSML_MAX_ELEMENTS"
	MaxCalcDepthEnvVar   = "OPENSYSML_MAX_CALC_DEPTH"
	MaxSweepRunsEnvVar   = "OPENSYSML_MAX_SWEEP_RUNS"
)

// Budgets bounds one run of the runtime, and how many runs one sweep asks for.
// The bounds count incommensurable things — expression evaluations, action
// token-flow steps, state machine events, do actions, materialized collection
// elements, nested calc invocations and swept runs — so raising one says
// nothing about the others.
type Budgets struct {
	MaxSteps       int64
	MaxActionSteps int64
	MaxStateEvents int64
	MaxDoSteps     int64
	MaxElements    int64
	MaxCalcDepth   int64
	MaxSweepRuns   int64
}

// DefaultBudgets returns the bounds a run uses when the environment names no
// override.
func DefaultBudgets() Budgets {
	return Budgets{
		MaxSteps:       DefaultMaxSteps,
		MaxActionSteps: DefaultMaxActionSteps,
		MaxStateEvents: DefaultMaxStateEvents,
		MaxDoSteps:     DefaultMaxDoSteps,
		MaxElements:    DefaultMaxElements,
		MaxCalcDepth:   DefaultMaxCalcDepth,
		MaxSweepRuns:   DefaultMaxSweepRuns,
	}
}

// budgetVar is one bound: the variable that sets it, its default, and what the
// number counts (used to say what a rejected value should have been).
type budgetVar struct {
	env    string
	def    int64
	counts string
	field  func(*Budgets) *int64
	// ceiling is the highest usable value, or zero where any positive value is.
	ceiling int64
}

// budgetVars is every configurable bound, in the order they are reported.
var budgetVars = []budgetVar{
	{MaxStepsEnvVar, DefaultMaxSteps, "evaluation steps", func(b *Budgets) *int64 { return &b.MaxSteps }, 0},
	{MaxActionStepsEnvVar, DefaultMaxActionSteps, "action token-flow steps", func(b *Budgets) *int64 { return &b.MaxActionSteps }, 0},
	{MaxStateEventsEnvVar, DefaultMaxStateEvents, "state machine events", func(b *Budgets) *int64 { return &b.MaxStateEvents }, 0},
	{MaxDoStepsEnvVar, DefaultMaxDoSteps, "do action steps", func(b *Budgets) *int64 { return &b.MaxDoSteps }, 0},
	{MaxElementsEnvVar, DefaultMaxElements, "collection elements", func(b *Budgets) *int64 { return &b.MaxElements }, 0},
	{MaxCalcDepthEnvVar, DefaultMaxCalcDepth, "nested calc invocations", func(b *Budgets) *int64 { return &b.MaxCalcDepth }, MaxCalcDepthCeiling},
	{MaxSweepRunsEnvVar, DefaultMaxSweepRuns, "runs of one sweep", func(b *Budgets) *int64 { return &b.MaxSweepRuns }, 0},
}

// Validate reports every bound that is not positive, which would let a run make
// no progress at all, or above its ceiling, which would fail unrecoverably.
func (b Budgets) Validate() error {
	var errs []error
	for _, v := range budgetVars {
		n := *v.field(&b)
		if n <= 0 {
			errs = append(errs, fmt.Errorf("%s budget must be greater than zero, got %d (%s)", v.counts, n, v.env))
			continue
		}
		if v.ceiling > 0 && n > v.ceiling {
			errs = append(errs, fmt.Errorf("%s budget must be at most %d, got %d (%s)", v.counts, v.ceiling, n, v.env))
		}
	}
	return errors.Join(errs...)
}

// BudgetsFromEnv returns the bounds the environment asks for: for each variable
// the positive integer it holds, or the default when it is unset or empty. Every
// unusable value is reported, naming its variable and the value, so a typo is
// reported instead of silently leaving the default in place.
func BudgetsFromEnv() (Budgets, error) {
	return budgetsFromLookup(envvar.Lookup)
}

// budgetsFromLookup is BudgetsFromEnv over an explicit lookup, so the parsing
// rules are testable without the process environment.
func budgetsFromLookup(lookup func(string) string) (Budgets, error) {
	budgets := DefaultBudgets()
	var errs []error
	for _, v := range budgetVars {
		n, err := budgetFromValue(v, lookup(v.env))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		*v.field(&budgets) = n
	}
	if err := errors.Join(errs...); err != nil {
		return Budgets{}, err
	}
	return budgets, nil
}

// budgetFromValue parses one bound's value, defaulting on unset or empty.
func budgetFromValue(v budgetVar, raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return v.def, nil
	}
	n, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an integer: set it to a positive number of %s (default %d)", v.env, raw, v.counts, v.def)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s=%q must be greater than zero: the budget is what stops a runaway run (default %d)", v.env, raw, v.def)
	}
	if v.ceiling > 0 && n > v.ceiling {
		return 0, fmt.Errorf("%s=%q must be at most %d: past that a runaway run would exhaust the stack instead of reporting the budget (default %d)", v.env, raw, v.ceiling, v.def)
	}
	return n, nil
}

// Budgets returns the bounds this context runs under.
func (ctx *Context) Budgets() Budgets {
	return Budgets{
		MaxSteps:       ctx.maxSteps,
		MaxActionSteps: ctx.maxActionSteps,
		MaxStateEvents: ctx.maxStateEvents,
		MaxDoSteps:     ctx.maxDoSteps,
		MaxElements:    ctx.maxElements,
		MaxCalcDepth:   ctx.maxCalcDepth,
		MaxSweepRuns:   ctx.maxSweepRuns,
	}
}

// SetBudgets replaces the bounds this context runs under, rejecting a set that
// holds a non-positive bound. The evaluation step counter already spent is left
// alone: the budget is a bound on the run, not a reset of it.
func (ctx *Context) SetBudgets(b Budgets) error {
	if err := b.Validate(); err != nil {
		return err
	}
	ctx.maxSteps = b.MaxSteps
	ctx.maxActionSteps = b.MaxActionSteps
	ctx.maxStateEvents = b.MaxStateEvents
	ctx.maxDoSteps = b.MaxDoSteps
	ctx.maxElements = b.MaxElements
	ctx.maxCalcDepth = b.MaxCalcDepth
	ctx.maxSweepRuns = b.MaxSweepRuns
	return nil
}
