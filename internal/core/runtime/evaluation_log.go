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
// being run and no application binding the same arguments was noted before.
func (log *evaluationLog) record(fn *functionValue, args calcArgs, result Value, err error) {
	if log == nil || fn.shape == nil || !log.calcs[fn.shape.Sym] {
		return
	}
	arguments := fn.shape.argumentsByPosition(args)
	key := log.key(fn.shape.Name, arguments)
	if _, seen := log.index[key]; seen {
		return
	}
	log.index[key] = len(log.entries)
	log.entries = append(log.entries, AnalysisEvaluation{
		Function: fn.shape.Name, Arguments: arguments, Result: result, Error: err,
	})
}

// argumentsByPosition lists an application's arguments by parameter position,
// null where the application binds none before a later one; unknown names follow, sorted.
func (shape *calcShape) argumentsByPosition(args calcArgs) []Value {
	arguments := append(make([]Value, 0, len(shape.ParamNames)), args.positional...)
	var unknown []string
	for name := range args.named {
		position := slices.Index(shape.ParamNames, name)
		if position < 0 {
			unknown = append(unknown, name)
			continue
		}
		for len(arguments) <= position {
			arguments = append(arguments, nullValue())
		}
		arguments[position] = args.named[name]
	}
	sort.Strings(unknown)
	for _, name := range unknown {
		arguments = append(arguments, args.named[name])
	}
	return arguments
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
