package runtime

import (
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// evaluationLog records, for the case run under way, each application of one of
// the case's calc features as a function value, once per distinct argument list.
// A trade study's `eval(x)` is one per alternative, however often the library's
// `best` and `selectOne` re-ask it.
type evaluationLog struct {
	calcs     map[*symbols.Symbol]bool
	entries   []AnalysisEvaluation
	index     map[string]int
	enclosing *evaluationLog
}

// beginEvaluationLog opens the log of caseSym's run, replacing the enclosing
// run's log until endEvaluationLog restores it.
func (ctx *Context) beginEvaluationLog(caseSym *symbols.Symbol) *evaluationLog {
	log := &evaluationLog{calcs: map[*symbols.Symbol]bool{}, index: map[string]int{}, enclosing: ctx.evaluations}
	for _, member := range ctx.model.MembersOfIncludingRedefined(caseSym) {
		if isCalcSymbol(member) {
			log.calcs[member] = true
		}
	}
	ctx.evaluations = log
	return log
}

// endEvaluationLog closes log, so the run enclosing it records its own again.
func (ctx *Context) endEvaluationLog(log *evaluationLog) {
	ctx.evaluations = log.enclosing
}

// record notes one application of fn to args, when fn is a calc of the case
// being run and no application to the same arguments was noted before.
func (log *evaluationLog) record(fn *functionValue, args calcArgs, result Value, err error) {
	if log == nil || fn.shape == nil || !log.calcs[fn.shape.Sym] {
		return
	}
	arguments := append([]Value(nil), args.positional...)
	names := make([]string, 0, len(args.named))
	for name := range args.named {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		arguments = append(arguments, args.named[name])
	}
	key := log.key(fn.shape.Name, arguments)
	if _, seen := log.index[key]; seen {
		return
	}
	log.index[key] = len(log.entries)
	log.entries = append(log.entries, AnalysisEvaluation{
		Function: fn.shape.Name, Arguments: arguments, Result: result, Error: err,
	})
}

// key identifies an application by the calc and the arguments as the trace
// spells them, so one alternative evaluated twice is noted once.
func (log *evaluationLog) key(function string, arguments []Value) string {
	parts := make([]string, 0, len(arguments)+1)
	parts = append(parts, function)
	for _, arg := range arguments {
		parts = append(parts, FormatTraceValue(arg))
	}
	return strings.Join(parts, "\x00")
}

// evaluations reports the log with the evaluation of the returned value marked
// selected, and every other evaluation by the same calc computing the same
// result marked tied: `selectOne` took the first, the library ranks no further.
func (log *evaluationLog) evaluations(returned Value, ok bool) []AnalysisEvaluation {
	if !ok {
		return log.entries
	}
	for i := range log.entries {
		entry := &log.entries[i]
		entry.Selected = slices.ContainsFunc(entry.Arguments, func(arg Value) bool {
			return valueEqual(arg, returned)
		})
	}
	for _, selected := range log.entries {
		if !selected.Selected || selected.Error != nil {
			continue
		}
		for i := range log.entries {
			other := &log.entries[i]
			other.Tied = other.Tied || (!other.Selected && other.Error == nil &&
				other.Function == selected.Function && valueEqual(other.Result, selected.Result))
		}
	}
	return log.entries
}
