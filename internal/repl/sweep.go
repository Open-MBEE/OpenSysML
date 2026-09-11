package repl

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const (
	sweepUsage   = "usage: %sweep <name>[(<args>)] [<object>] <parameter>=<from>..<to>[:<step>] ..."
	samplesUsage = "usage: %samples <n> <seed> <name>[(<args>)] [<object>] <parameter>=<from>..<to> ..."
)

// sweepSpec is one range as it is written on a command line or at the prompt.
type sweepSpec struct {
	param   string
	from    string
	to      string
	step    string
	hasStep bool
	text    string
}

// doSweep carries out %sweep at the prompt.
func (s *Session) doSweep(tail string) ([]string, bool, error) {
	inv, specs, err := splitSweepTail(tail)
	if err != nil {
		return []string{errPrefix + err.Error(), sweepUsage}, false, nil
	}
	if inv.name == "" {
		return []string{sweepUsage}, false, nil
	}
	return s.withTrace(s.sweepVerdict(inv, specs, sweepDraws{})).Lines, false, nil
}

// doSamples carries out %samples at the prompt: the number of draws and the
// seed they are drawn from, then the invocation a sweep takes.
func (s *Session) doSamples(tail string) ([]string, bool, error) {
	count, seed, rest, err := splitSamplesTail(tail)
	if err != nil {
		return []string{errPrefix + err.Error(), samplesUsage}, false, nil
	}
	inv, specs, err := splitSweepTail(rest)
	if err != nil {
		return []string{errPrefix + err.Error(), samplesUsage}, false, nil
	}
	if inv.name == "" {
		return []string{samplesUsage}, false, nil
	}
	draws := sweepDraws{sampled: true, count: count, seed: seed}
	return s.withTrace(s.sweepVerdict(inv, specs, draws)).Lines, false, nil
}

// sweepDraws is what a sampled sweep adds to one: how many rows to draw and the
// seed they are drawn from.
type sweepDraws struct {
	sampled bool
	count   int64
	seed    uint64
}

// RunSweep runs one invocation once per row of the ranges given and reports the
// table. invocation is what `%analysis` and `%calc` take; each range is written
// `<parameter>=<from>..<to>[:<step>]`.
func (s *Session) RunSweep(invocation string, ranges []string) Verdict {
	defer s.enter()()
	return s.sweepFromText(invocation, ranges, sweepDraws{})
}

// RunSamples runs one invocation once per drawn row, drawing count values for
// each range from seed. The same seed draws the same table.
func (s *Session) RunSamples(invocation string, ranges []string, count int64, seed uint64) Verdict {
	defer s.enter()()
	return s.sweepFromText(invocation, ranges, sweepDraws{sampled: true, count: count, seed: seed})
}

// sweepFromText parses an invocation and its ranges written apart, as a command
// line writes them, and runs the sweep.
func (s *Session) sweepFromText(invocation string, ranges []string, draws sweepDraws) Verdict {
	inv, trailing, err := splitSweepTail(invocation)
	if err != nil {
		return s.withTrace(unresolvedVerdict(invocation, err.Error()))
	}
	specs := make([]sweepSpec, 0, len(ranges)+len(trailing))
	specs = append(specs, trailing...)
	for _, text := range ranges {
		spec, err := parseSweepSpec(text)
		if err != nil {
			return s.withTrace(unresolvedVerdict(invocation, err.Error()))
		}
		specs = append(specs, spec)
	}
	return s.withTrace(s.sweepVerdict(inv, specs, draws))
}

// sweepVerdict runs a sweep and reports its table. A plan that could not be run
// at all is unresolved; a table whose runs all held and whose objectives were
// satisfied holds; any failed run or unsatisfied objective fails it. The rows'
// traces lead the report in plan order, as one run's trace leads its verdict.
func (s *Session) sweepVerdict(inv analysisInvocation, specs []sweepSpec, draws sweepDraws) Verdict {
	label := sweepLabel(inv, draws)
	table, plan, err := s.runSweep(inv, specs, draws)
	if err != nil {
		return standing(unresolvedVerdict(label, err.Error()), plan)
	}
	status, rows := sweepStatus(table)
	return standing(Verdict{
		Subject: label,
		Status:  status,
		Lines:   append(sweepTraces(table), sweepTableLines(table)...),
		Values:  sweepValues(table, rows),
		Rows:    rows,
	}, plan)
}

// sweepTraces is what the rows' runs traced, in plan order, as trace lines print.
func sweepTraces(table runtime.SweepTable) []string {
	var lines []string
	for _, row := range table.Rows {
		for _, e := range recordedTrace(row.Context) {
			lines = append(lines, tracePrefix+e)
		}
	}
	return lines
}

// sweepLabel names what was run, as the caller wrote it.
func sweepLabel(inv analysisInvocation, draws sweepDraws) string {
	label := inv.name
	if inv.argText != "" {
		label += "(" + strings.TrimSpace(inv.argText) + ")"
	}
	if draws.sampled {
		return "samples " + label
	}
	return "sweep " + label
}

// runSweep resolves the target an invocation names, evaluates its arguments and
// ranges once where the prompt does, and makes one analysis or calc run per row, each
// in a context of the row's own: the arguments are evaluated again there, and the
// subject and the owner of a nested case are objects of their declarations made there.
// The session's state is released while the rows run, as it is for an exploration.
func (s *Session) runSweep(inv analysisInvocation, specs []sweepSpec, draws sweepDraws) (runtime.SweepTable, *analysis.Plan, error) {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return runtime.SweepTable{}, nil, errors.New("no declarations loaded")
	}
	sym, fqn, err := s.lookupSymbolOfKinds(inv.name,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolCalcDef, symbols.SymbolCalcUsage)
	if err != nil {
		return runtime.SweepTable{}, nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return runtime.SweepTable{}, nil, err
	}

	parsed, err := parseAnalysisArgs(inv.argText)
	if err != nil {
		return runtime.SweepTable{}, nil, err
	}
	scope := s.promptScope()
	positional, named, err := evalInvocationArgs(ctx, scope, parsed)
	if err != nil {
		return runtime.SweepTable{}, nil, err
	}

	plan, err := s.sweepPlan(ctx, specs, draws)
	if err != nil {
		return runtime.SweepTable{}, nil, err
	}
	namedNames := make([]string, 0, len(named))
	for name := range named {
		namedNames = append(namedNames, name)
	}
	plan, err = ctx.ResolveSweepPlan(sym, plan, len(positional), namedNames)
	if err != nil {
		return runtime.SweepTable{}, nil, err
	}

	isCase := runtime.IsRunnableCaseSymbol(sym)
	var subject, owner freshRef
	if inv.object != "" {
		if !isCase {
			return runtime.SweepTable{}, nil, fmt.Errorf("%s is a calc, which has no subject", inv.name)
		}
		if subject, err = s.sweptObject(ctx, inv.object); err != nil {
			return runtime.SweepTable{}, nil, err
		}
	}
	if isNestedCase(sym) {
		if owner, err = s.sweptOwner(ctx, fqn); err != nil {
			return runtime.SweepTable{}, nil, err
		}
	}
	runScope := declaringScope(sym, doc.Scope)

	run := func(rt *runtime.Context, bindings []runtime.SweepBinding) (runtime.SweepRunResult, error) {
		positional, named, err := evalInvocationArgs(rt, scope, parsed)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		bound := make(map[string]runtime.Value, len(named)+len(bindings))
		for name, value := range named {
			bound[name] = value
		}
		for _, b := range bindings {
			bound[b.Param] = b.Value
		}
		if !isCase {
			value, err := rt.InvokeCalcWith(sym, positional, bound, runScope)
			if err != nil {
				return runtime.SweepRunResult{}, err
			}
			return runtime.SweepRunResult{
				Outputs: []runtime.CalcOutputValue{{Name: calcResultName, Value: value}},
			}, nil
		}
		args := runtime.AnalysisArgs{Positional: positional, Named: bound}
		if args.Subject, err = subject.instantiate(rt); err != nil {
			return runtime.SweepRunResult{}, err
		}
		self, err := owner.instantiate(rt)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		result, err := rt.RunAnalysis(sym, args, runScope, self)
		return runtime.SweepRunResult{
			Outputs:     result.Outputs,
			Verdicts:    result.Verdicts,
			Subject:     result.Subject,
			Evaluations: result.Evaluations,
		}, err
	}

	model := s.freshModel()
	s.state.Unlock()
	answered, err := s.sweep(fqn, model, plan, run)
	s.state.Lock()
	if err != nil {
		return runtime.SweepTable{}, &answered, err
	}
	return answered.Result.Table(), &answered, nil
}

// evalInvocationArgs evaluates an invocation's positional and named arguments in ctx,
// where scope names what the prompt reaches.
func evalInvocationArgs(ctx *runtime.Context, scope *symbols.Scope, parsed analysisArgs) ([]runtime.Value, map[string]runtime.Value, error) {
	var positional []runtime.Value
	for _, arg := range parsed.positional {
		val, err := ctx.EvalWithScope(arg.expr, scope)
		if err != nil {
			return nil, nil, fmt.Errorf("evaluation of argument %q failed: %w", arg.text, err)
		}
		positional = append(positional, val)
	}
	named, err := evalArgumentsIn(ctx, scope, parsed.named)
	if err != nil {
		return nil, nil, err
	}
	return positional, named, nil
}

// SweptObjectError reports an object of the session a sweep cannot run its rows on:
// each row runs on a fresh object of the held object's declaration, which stands for
// the held one only while that one is as its declaration made it, named plainly.
type SweptObjectError struct {
	Ref    string
	Reason string
}

func (e *SweptObjectError) Error() string {
	return fmt.Sprintf("%s: %s; each row of a sweep runs on an object of its own, made from the declaration, so the held object must be as the declaration made it", e.Ref, e.Reason)
}

// sweptObject resolves the object a sweep names as its subject to the declaration each
// row instantiates: the held object must be named by its declaration and be pristine.
func (s *Session) sweptObject(ctx *runtime.Context, text string) (freshRef, error) {
	inst, label, err := s.resolveObject(text)
	if err != nil {
		return freshRef{}, err
	}
	ref, err := parseObjectRef(text)
	if err != nil {
		return freshRef{}, err
	}
	if ref.id > 0 {
		return freshRef{}, &SweptObjectError{Ref: label, Reason: "it is named by its identity, not by a declaration"}
	}
	for _, seg := range ref.segments {
		if seg.index > 0 || seg.dotted {
			return freshRef{}, &SweptObjectError{Ref: label, Reason: "it is reached through a feature of another object, not by a declaration of its own"}
		}
	}
	sym, fqn, err := s.lookupSymbol(joinTyped(ref.segments))
	if err != nil {
		return freshRef{}, err
	}
	return s.sweptRef(ctx, inst, label, sym, fqn)
}

// sweptOwner resolves the object owning the case usage at fqn, when the session holds
// one, to the declaration each row instantiates as the case's owner; a nested case the
// session holds no owner for runs on none, as the prompt's run does.
func (s *Session) sweptOwner(ctx *runtime.Context, fqn string) (freshRef, error) {
	held, label := s.owningInstance(fqn)
	if held == nil {
		return freshRef{}, nil
	}
	segments := strings.Split(fqn, "::")
	sym, ownerFQN, err := s.lookupSymbol(strings.Join(segments[:len(segments)-1], "::"))
	if err != nil {
		return freshRef{}, err
	}
	return s.sweptRef(ctx, held, label, sym, ownerFQN)
}

// sweptRef is the declaration at fqn as the rows instantiate it for held, which must be
// the object the session holds under fqn and stand as the declaration made it.
func (s *Session) sweptRef(ctx *runtime.Context, held *runtime.Instance, label string, sym *symbols.Symbol, fqn string) (freshRef, error) {
	if s.instances[fqn] != held {
		return freshRef{}, &SweptObjectError{Ref: label, Reason: "it is reached through a feature of another object, not by a declaration of its own"}
	}
	if err := ctx.Pristine(held); err != nil {
		return freshRef{}, &SweptObjectError{Ref: label, Reason: err.Error()}
	}
	return freshRef{sym: sym, fqn: fqn, name: s.declaredName(fqn)}, nil
}

// instantiate makes the object of the reference in ctx, none for an empty reference.
func (r freshRef) instantiate(ctx *runtime.Context) (*runtime.Instance, error) {
	if r.sym == nil {
		return nil, nil
	}
	inst, err := ctx.Instantiate(r.sym)
	if err != nil {
		return nil, fmt.Errorf("instantiation of %s failed: %w", r.name, err)
	}
	return inst, nil
}

// calcResultName names a calc's returned value in a table, so a calc row and an
// analysis case's outputs read alike.
const calcResultName = "result"

// sweepPlan evaluates each range's endpoints where the prompt evaluates any
// expression, so a range carries the same literals and units an argument does.
func (s *Session) sweepPlan(ctx *runtime.Context, specs []sweepSpec, draws sweepDraws) (runtime.SweepPlan, error) {
	plan := runtime.SweepPlan{
		Ranges:  make([]runtime.SweepRange, 0, len(specs)),
		Sampled: draws.sampled,
		Samples: draws.count,
		Seed:    draws.seed,
	}
	scope := s.promptScope()
	for _, spec := range specs {
		r := runtime.SweepRange{Param: spec.param, HasStep: spec.hasStep}
		var err error
		if r.From, err = s.evalRangeBound(ctx, scope, spec, "start", spec.from); err != nil {
			return runtime.SweepPlan{}, err
		}
		if r.To, err = s.evalRangeBound(ctx, scope, spec, "end", spec.to); err != nil {
			return runtime.SweepPlan{}, err
		}
		if spec.hasStep {
			if r.Step, err = s.evalRangeBound(ctx, scope, spec, "step", spec.step); err != nil {
				return runtime.SweepPlan{}, err
			}
		}
		plan.Ranges = append(plan.Ranges, r)
	}
	return plan, nil
}

// evalRangeBound evaluates one endpoint of a range.
func (s *Session) evalRangeBound(ctx *runtime.Context, scope *symbols.Scope, spec sweepSpec, what, text string) (runtime.Value, error) {
	expr, err := parseWholeExpr(text)
	if err != nil {
		return runtime.Value{}, fmt.Errorf("%w: range %s: %v", runtime.ErrSweepRange, spec.param, err)
	}
	value, err := ctx.EvalWithScope(expr, scope)
	if err != nil {
		return runtime.Value{}, fmt.Errorf("%w: range %s: %s %q: %v",
			runtime.ErrSweepRange, spec.param, what, text, err)
	}
	return value, nil
}

// sweepStatus judges a table and reports each of its runs: a run that failed or
// an objective that was not satisfied fails the table, an undecided one leaves
// it unresolved.
func sweepStatus(table runtime.SweepTable) (VerdictStatus, []VerdictRow) {
	status := VerdictHolds
	rows := make([]VerdictRow, 0, len(table.Rows))
	for _, row := range table.Rows {
		out := VerdictRow{Millis: float64(row.Elapsed.Nanoseconds()) / 1e6}
		for _, b := range row.Bindings {
			out.Inputs = append(out.Inputs, NamedValue{Name: b.Param, Value: runtime.FormatValue(b.Value)})
		}
		if row.Err != nil {
			out.Error = oneLine(row.Err.Error())
			if status == VerdictHolds {
				status = VerdictFails
			}
		}
		for _, o := range row.Outputs {
			out.Outputs = append(out.Outputs, NamedValue{Name: o.Name, Value: objectText(row.Context, o.Value)})
		}
		// A failed run's error is what the table holds against it; the verdicts
		// it left undecided are reported with it, not counted again.
		for _, v := range row.Verdicts {
			out.Verdicts = append(out.Verdicts, NamedValue{Name: v.Kind + " " + v.Name, Value: v.Status.String()})
			if row.Err != nil {
				continue
			}
			switch v.Status {
			case runtime.VerdictNotSatisfied:
				if status == VerdictHolds {
					status = VerdictFails
				}
			case runtime.VerdictUndecided:
				status = VerdictUnresolved
			}
		}
		for _, e := range row.Evaluations {
			evaluation, _ := evaluationOf(row.Context, table.Target, e)
			out.Evaluations = append(out.Evaluations, evaluation)
		}
		rows = append(rows, out)
	}
	return status, rows
}

// sweepValues summarises a table for a caller reporting values rather than
// rows: how many runs it made and how many of them failed.
func sweepValues(table runtime.SweepTable, rows []VerdictRow) []NamedValue {
	failed := 0
	for _, row := range rows {
		if row.Error != "" {
			failed++
		}
	}
	values := []NamedValue{
		{Name: "runs", Value: strconv.Itoa(len(rows))},
		{Name: "failed", Value: strconv.Itoa(failed)},
	}
	if table.Sampled {
		values = append(values, NamedValue{Name: "seed", Value: strconv.FormatUint(table.Seed, 10)})
	}
	return values
}

// oneLine folds a message onto one line, so a row of a table stays a row.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// splitSamplesTail takes the number of draws and the seed off the front of
// `%samples`'s tail.
func splitSamplesTail(tail string) (int64, uint64, string, error) {
	fields := strings.Fields(strings.TrimSpace(tail))
	if len(fields) < 3 {
		return 0, 0, "", errors.New("name the number of samples, the seed, then the case and its ranges")
	}
	count, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || count <= 0 {
		return 0, 0, "", fmt.Errorf("%w: %q is not a number of samples to draw", runtime.ErrSweepSamples, fields[0])
	}
	seed, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("%w: %q is not a seed", runtime.ErrSweepSamples, fields[1])
	}
	rest := strings.TrimSpace(tail)
	for range 2 {
		rest = strings.TrimSpace(rest)
		cut := strings.IndexFunc(rest, unicode.IsSpace)
		rest = strings.TrimSpace(rest[cut:])
	}
	return count, seed, rest, nil
}

// splitSweepTail takes apart a tail written as an invocation followed by its
// ranges: `Case(3.0) ship x=0..10:2`. A range is recognised by the parameter it
// binds, so an endpoint may carry spaces as any other argument may.
func splitSweepTail(tail string) (analysisInvocation, []sweepSpec, error) {
	tail = strings.TrimSpace(tail)
	head, rangeText := tail, ""
	if start := firstSpecStart(tail); start >= 0 {
		head, rangeText = strings.TrimSpace(tail[:start]), tail[start:]
	}
	inv, err := splitAnalysisArgs(head)
	if err != nil {
		return analysisInvocation{}, nil, err
	}
	var specs []sweepSpec
	for _, text := range splitSpecs(rangeText) {
		spec, err := parseSweepSpec(text)
		if err != nil {
			return analysisInvocation{}, nil, err
		}
		specs = append(specs, spec)
	}
	return inv, specs, nil
}

// splitSpecs takes apart ranges written one after another, each beginning with
// the parameter it binds.
func splitSpecs(text string) []string {
	var specs []string
	for text = strings.TrimSpace(text); text != ""; {
		next := specStartAfter(text, 1)
		if next < 0 {
			return append(specs, strings.TrimSpace(text))
		}
		specs = append(specs, strings.TrimSpace(text[:next]))
		text = strings.TrimSpace(text[next:])
	}
	return specs
}

// firstSpecStart indexes where the first range in text begins: a bare name
// bound with `=` at the top level. An `=` inside a string, a bracket or an
// argument list belongs to that expression.
func firstSpecStart(text string) int { return specStartAfter(text, 0) }

// specStartAfter indexes the first range beginning at or after from, reading
// text from its start so quotes and brackets are tracked whole.
func specStartAfter(text string, from int) int {
	depth, q := 0, quoteTracker{}
	for i, r := range text {
		switch {
		case q.inside(r):
		case r == '(' || r == '[':
			depth++
		case r == ')' || r == ']':
			depth--
		case depth == 0 && r == '=' && i > 0:
			if start := specNameStart(text, i); start >= from {
				return start
			}
		}
	}
	return -1
}

// specNameStart indexes the start of the name bound at eq, -1 when what
// precedes it is not one name: a bare one, or an unrestricted one in quotes.
func specNameStart(text string, eq int) int {
	start := eq
	if eq > 0 && text[eq-1] == '\'' {
		if start = quotedNameStart(text, eq-1); start < 0 {
			return -1
		}
	} else {
		for start > 0 {
			r, width := utf8.DecodeLastRuneInString(text[:start])
			if !isSpecNameRune(r) {
				break
			}
			start -= width
		}
		if start == eq {
			return -1
		}
		if first, _ := utf8.DecodeRuneInString(text[start:]); unicode.IsDigit(first) {
			return -1
		}
	}
	if start > 0 && !isSpace(text[start-1]) {
		return -1
	}
	return start
}

// quotedNameStart indexes the quote opening the unrestricted name closed at
// end, -1 when no quote opens there.
func quotedNameStart(text string, end int) int {
	open, q := -1, quoteTracker{}
	for i, r := range text[:end+1] {
		was := q.quote
		if !q.inside(r) {
			continue
		}
		if was == 0 && q.quote == '\'' {
			open = i
		} else if was == '\'' && q.quote == 0 {
			if i == end {
				return open
			}
			open = -1
		}
	}
	return -1
}

// isSpecNameRune reports whether r may spell part of a bare parameter name.
func isSpecNameRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// unquoteSpecName is a parameter's name as the model spells it: an unrestricted
// name loses its quotes, as a declared name does when it is parsed.
func unquoteSpecName(name string) string {
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		return name[1 : len(name)-1]
	}
	return name
}

// isSpace reports whether b separates words on a command line.
func isSpace(b byte) bool { return b == ' ' || b == '\t' }

// parseSweepSpec takes apart one range: `<parameter>=<from>..<to>[:<step>]`. The
// separators are read outside strings, brackets and argument lists, so an
// endpoint carrying a unit — `0.0 [SI::m]..10.0 [SI::m]:2.0 [SI::m]` — is read
// as written.
func parseSweepSpec(text string) (sweepSpec, error) {
	text = strings.TrimSpace(text)
	eq := specSeparator(text, "=")
	if eq < 0 {
		return sweepSpec{}, fmt.Errorf("%w: %q is not written as <parameter>=<from>..<to>[:<step>]",
			runtime.ErrSweepRange, text)
	}
	spec := sweepSpec{param: unquoteSpecName(strings.TrimSpace(text[:eq])), text: text}
	if spec.param == "" {
		return sweepSpec{}, fmt.Errorf("%w: %q names no parameter", runtime.ErrSweepParameter, text)
	}
	body := strings.TrimSpace(text[eq+1:])
	dots := specSeparator(body, "..")
	if dots < 0 {
		if name, ok := distributionCall(body); ok {
			return sweepSpec{}, fmt.Errorf(
				"%w: %s asks for the distribution %s; sampling is uniform over a range written <from>..<to>, since no library in this build states a probability distribution",
				runtime.ErrSweepDistribution, spec.param, name)
		}
		return sweepSpec{}, fmt.Errorf("%w: %s=%s states no range; write it as <from>..<to>[:<step>]",
			runtime.ErrSweepRange, spec.param, body)
	}
	spec.from = strings.TrimSpace(body[:dots])
	rest := strings.TrimSpace(body[dots+2:])
	if colon := specSeparator(rest, ":"); colon >= 0 {
		spec.to = strings.TrimSpace(rest[:colon])
		spec.step = strings.TrimSpace(rest[colon+1:])
		spec.hasStep = true
	} else {
		spec.to = rest
	}
	if spec.from == "" || spec.to == "" || (spec.hasStep && spec.step == "") {
		return sweepSpec{}, fmt.Errorf("%w: %s=%s leaves an endpoint of the range empty",
			runtime.ErrSweepRange, spec.param, body)
	}
	return spec, nil
}

// specSeparator indexes the first separator in text that belongs to the range
// rather than to one of its endpoints: outside strings, brackets and argument
// lists, and, for `:`, not part of a qualified name's `::`.
func specSeparator(text, sep string) int {
	depth, q := 0, quoteTracker{}
	for i, r := range text {
		switch {
		case q.inside(r):
		case r == '(' || r == '[':
			depth++
		case r == ')' || r == ']':
			depth--
		case depth != 0:
		case !strings.HasPrefix(text[i:], sep):
		case sep == ":" && (strings.HasPrefix(text[i:], "::") || i > 0 && text[i-1] == ':'):
		case sep == "=" && (strings.HasPrefix(text[i:], "==") || i > 0 && strings.ContainsRune("=<>!", rune(text[i-1]))):
		default:
			return i
		}
	}
	return -1
}

// distributionCall reads a range written as a call — `normal(1.0, 0.2)` — and
// reports the name it calls, which is how a request for a named distribution
// arrives.
func distributionCall(text string) (string, bool) {
	open := strings.IndexByte(text, '(')
	if open <= 0 || !strings.HasSuffix(strings.TrimSpace(text), ")") {
		return "", false
	}
	name := strings.TrimSpace(text[:open])
	for i, r := range name {
		if r == '_' || r == ':' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			continue
		}
		return "", false
	}
	return name, name != ""
}

// sweepTableLines renders a table: a header naming what was run, its ranges and,
// for a sampled table, the seed it was drawn from, then one row per run, each read
// through the context that ran it.
func sweepTableLines(table runtime.SweepTable) []string {
	header := fmt.Sprintf("sweep %s — %d run(s)", table.Target, len(table.Rows))
	if table.Sampled {
		header = fmt.Sprintf("samples %s — %d run(s), seed %d", table.Target, len(table.Rows), table.Seed)
	}
	columns := newSweepColumns(table)
	cells := make([][]string, 0, len(table.Rows)+1)
	cells = append(cells, columns.titles())
	for _, row := range table.Rows {
		cells = append(cells, columns.row(table.Target, row))
	}
	notes := footnoteErrors(columns, cells)
	widths := make([]int, len(cells[0]))
	for _, row := range cells {
		for i, cell := range row {
			if n := len([]rune(cell)); n > widths[i] {
				widths[i] = n
			}
		}
	}
	lines := []string{header, renderSweepRow(cells[0], widths, " | "), sweepRule(widths)}
	for _, row := range cells[1:] {
		lines = append(lines, renderSweepRow(row, widths, " | "))
	}
	lines = append(lines, notes...)
	return append(lines, untypedNotes(table)...)
}

// untypedNotes says which parameters declare no type, since their ranges are
// read as written rather than in a type of the parameter's.
func untypedNotes(table runtime.SweepTable) []string {
	var notes []string
	for i, typ := range table.Types {
		if typ.Untyped {
			notes = append(notes, fmt.Sprintf("note: %s declares no type; its range is read as written", table.Params[i]))
		}
	}
	return notes
}

// footnoteErrors moves each failed run's error out of its cell, which a typed
// error is far too long for; the cell keeps the note's number.
func footnoteErrors(columns sweepColumns, cells [][]string) []string {
	if !columns.failures {
		return nil
	}
	last := len(cells[0]) - 1
	notes := make([]string, 0, len(cells)-1)
	for _, row := range cells[1:] {
		if row[last] == "" {
			continue
		}
		notes = append(notes, fmt.Sprintf("error %d: %s", len(notes)+1, row[last]))
		row[last] = strconv.Itoa(len(notes))
	}
	return notes
}

// sweepColumns are the columns a table needs: its parameters, every output any
// run produced, verdicts and errors where there are any, and the run's time.
type sweepColumns struct {
	params      []string
	outputs     []string
	verdicts    bool
	evaluations bool
	failures    bool
}

// newSweepColumns decides a table's columns from what its runs produced.
func newSweepColumns(table runtime.SweepTable) sweepColumns {
	cols := sweepColumns{params: table.Params}
	seen := make(map[string]bool)
	for _, row := range table.Rows {
		for _, out := range row.Outputs {
			if !seen[out.Name] {
				seen[out.Name] = true
				cols.outputs = append(cols.outputs, out.Name)
			}
		}
		cols.verdicts = cols.verdicts || len(row.Verdicts) > 0
		cols.evaluations = cols.evaluations || len(row.Evaluations) > 0
		cols.failures = cols.failures || row.Err != nil
	}
	return cols
}

// titles names each column.
func (c sweepColumns) titles() []string {
	titles := make([]string, 0, len(c.params)+len(c.outputs)+4)
	titles = append(titles, c.params...)
	titles = append(titles, c.outputs...)
	if c.verdicts {
		titles = append(titles, "verdict")
	}
	if c.evaluations {
		titles = append(titles, "evaluations")
	}
	titles = append(titles, "time")
	if c.failures {
		titles = append(titles, "error")
	}
	return titles
}

// row renders one run under the columns, in the context that made it.
func (c sweepColumns) row(target string, row runtime.SweepRow) []string {
	ctx := row.Context
	cells := make([]string, 0, len(c.params)+len(c.outputs)+4)
	bound := make(map[string]runtime.Value, len(row.Bindings))
	for _, b := range row.Bindings {
		bound[b.Param] = b.Value
	}
	for _, param := range c.params {
		cells = append(cells, formatValue(ctx, bound[param]))
	}
	produced := make(map[string]string, len(row.Outputs))
	for _, out := range row.Outputs {
		produced[out.Name] = objectText(ctx, out.Value)
	}
	for _, name := range c.outputs {
		cells = append(cells, produced[name])
	}
	if c.verdicts {
		verdicts := make([]string, 0, len(row.Verdicts))
		for _, v := range row.Verdicts {
			verdicts = append(verdicts, v.Name+": "+v.Status.String())
		}
		cells = append(cells, strings.Join(verdicts, "; "))
	}
	if c.evaluations {
		evaluations := make([]string, 0, len(row.Evaluations))
		for _, e := range row.Evaluations {
			_, text := evaluationOf(ctx, target, e)
			evaluations = append(evaluations, text)
		}
		cells = append(cells, strings.Join(evaluations, "; "))
	}
	cells = append(cells, formatElapsed(row))
	if c.failures {
		text := ""
		if row.Err != nil {
			text = oneLine(row.Err.Error())
		}
		cells = append(cells, text)
	}
	return cells
}

// formatElapsed renders how long one run took, in milliseconds, so two runs of
// the same table are read against each other.
func formatElapsed(row runtime.SweepRow) string {
	return fmt.Sprintf("%.3fms", float64(row.Elapsed.Nanoseconds())/1e6)
}

// renderSweepRow pads each cell to its column's width.
func renderSweepRow(cells []string, widths []int, sep string) string {
	padded := make([]string, len(cells))
	for i, cell := range cells {
		padded[i] = cell + strings.Repeat(" ", widths[i]-len([]rune(cell)))
	}
	return strings.TrimRight(strings.Join(padded, sep), " ")
}

// sweepRule rules the header off from the rows.
func sweepRule(widths []int) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strings.Repeat("-", w)
	}
	return strings.Join(parts, "-+-")
}
