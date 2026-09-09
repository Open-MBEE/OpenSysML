package runtime

import (
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// evaluationLog records, for the case run under way, each application of one of
// the case's calc features as a function value, once per distinct argument list,
// and each element `selectOne` picked. A trade study's `eval(x)` is one per
// alternative, however often the library's `best` and `selectOne` re-ask it.
type evaluationLog struct {
	calcs     map[*symbols.Symbol]bool
	entries   []AnalysisEvaluation
	calcOf    []*symbols.Symbol // the calc each entry applied
	picks     []Value
	enclosing *evaluationLog
}

// beginEvaluationLog opens the log of caseSym's run, replacing the enclosing
// run's log until endEvaluationLog restores it.
func (ctx *Context) beginEvaluationLog(caseSym *symbols.Symbol) *evaluationLog {
	log := &evaluationLog{calcs: map[*symbols.Symbol]bool{}, enclosing: ctx.evaluations}
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
	if log.seen(fn.shape.Sym, arguments) {
		return
	}
	log.calcOf = append(log.calcOf, fn.shape.Sym)
	log.entries = append(log.entries, AnalysisEvaluation{
		Function: fn.shape.Name, Arguments: arguments, Result: result, Error: err,
	})
}

// pick notes an element `selectOne` picked, the only kind of value the run
// can be said to have selected.
func (log *evaluationLog) pick(element Value) {
	if log == nil || element.Kind == ValNull {
		return
	}
	log.picks = append(log.picks, element)
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

// seen reports whether calc was already applied to arguments equal, value by
// value as `==` judges, to these, so one alternative evaluated twice is noted once.
func (log *evaluationLog) seen(calc *symbols.Symbol, arguments []Value) bool {
	for i, entry := range log.entries {
		if log.calcOf[i] == calc && slices.EqualFunc(entry.Arguments, arguments, valueEqual) {
			return true
		}
	}
	return false
}

// evaluations reports the log with the evaluation of the returned value marked
// selected where `selectOne` picked it, and the same calc's equal results tied.
func (log *evaluationLog) evaluations(returned Value, ok bool) []AnalysisEvaluation {
	if !ok {
		return log.entries
	}
	picked := slices.ContainsFunc(log.picks, func(pick Value) bool { return valueEqual(pick, returned) })
	for i := range log.entries {
		entry := &log.entries[i]
		entry.Selected = picked && slices.ContainsFunc(entry.Arguments, func(arg Value) bool {
			return valueEqual(arg, returned)
		})
	}
	for s, selected := range log.entries {
		if !selected.Selected || selected.Error != nil {
			continue
		}
		for i := range log.entries {
			other := &log.entries[i]
			other.Tied = other.Tied || (!other.Selected && other.Error == nil &&
				log.calcOf[i] == log.calcOf[s] && valueEqual(other.Result, selected.Result))
		}
	}
	return log.entries
}
