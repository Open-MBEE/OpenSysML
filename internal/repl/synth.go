package repl

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// SolveValues asks a solver for values satisfying the named constraint,
// requirement or satisfaction, with the values the model already fixes — held by
// an object of the session, else declared — asserted, so only what is still free
// is synthesised. Experimental: SysML v2 defines no solving, and the runtime
// evaluator remains normative. Read-only: no object is created.
func (s *Session) SolveValues(name string) []SolveReport {
	defer s.enter()()
	return s.solveValues(name)
}

func (s *Session) solveValues(name string) []SolveReport {
	queries, unfixed, bad := s.solveQueriesWith(name, s.declaredPins(name))
	if bad != nil {
		return []SolveReport{*bad}
	}
	plan, err := s.solveWith(name, queries, synthesise)
	return solveReports(name, queries, plan, err, func(name string, q *solve.Query, solved analysis.Evaluation) SolveReport {
		return synthesisReport(name, q, solved, unfixed)
	})
}

// synthesise asks the solver for one satisfying assignment and, when none is consistent
// with the values fixed, for the conflict, so the report can name the fixed values in it.
func synthesise(solver *solve.Solver, ctx context.Context, q *solve.Query) (*solve.Result, error) {
	result, err := solver.Solve(ctx, q)
	if err != nil || result.Status != solve.StatusUnsat || q.Rounded() || !q.Fixes() {
		return result, err
	}
	return withConflict(solver, ctx, q, result), nil
}

// withConflict attaches to an unsat verdict the conflict behind it. A solver
// that will not explain leaves the verdict standing without one.
func withConflict(solver *solve.Solver, ctx context.Context, q *solve.Query, result *solve.Result) *solve.Result {
	explained, err := solver.Explain(ctx, q)
	if err == nil && explained.Status == solve.StatusUnsat {
		result.Core = explained.Core
	}
	return result
}

// declaredPins reads the values the model already fixes for the element a query
// is about, from the object a verdict about it would be about when the session
// holds one.
func (s *Session) declaredPins(name string) pinner {
	return func(target checkTarget, subject *symbols.Symbol) ([]solve.Pin, []solve.Unfixed, *SolveReport) {
		inst, _, bad := s.checkSubject(name, target)
		if bad != nil {
			report := unavailableReport(name, strings.TrimPrefix(bad.Lines[0], "error: "))
			return nil, nil, &report
		}
		pins, unfixed := solve.FixedFor(target.ctx, solve.Fixing{
			Element:    subject,
			Owner:      owningElement(subject),
			Object:     inst,
			ObjectType: s.documentType(inst),
		})
		return pins, unfixed, nil
	}
}

// documentType is the object's definition as the document declares it: an object
// carried over a submission is bound to the browse index's own scope tree, whose
// symbols are not the ones a query about the document's declarations reads.
func (s *Session) documentType(inst *runtime.Instance) *symbols.Symbol {
	if inst == nil || inst.Type == nil {
		return nil
	}
	if local := scopeSymbolForAny(s.docScopes(), inst.Type.Decl); local != nil {
		return local
	}
	return inst.Type
}

// owningElement is the element a condition was written in, whose declared values
// a verdict about that condition reads.
func owningElement(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// synthesisReport reports one satisfying assignment, fixed values first; unsat over fixed
// values says none exist consistent with those, not that the conditions are unsatisfiable.
func synthesisReport(name string, q *solve.Query, solved analysis.Evaluation, unfixed []solve.Unfixed) SolveReport {
	result, err := solved.Solved, solved.Err
	if err != nil {
		return unavailableReport(name, err.Error())
	}
	subject := solveSubject(q)
	switch result.Status {
	case solve.StatusSat:
		lines := []string{fmt.Sprintf("✓ %s has values satisfying it (%s)", subject, solveDetail(result))}
		lines = append(lines, fixedLines(q)...)
		lines = append(lines, synthesisedLines(q, result)...)
		lines = append(lines, unfixedLines(unfixed)...)
		lines = append(lines, unreadLines(q)...)
		lines = append(lines, "  One witness: a solver may answer with any of the assignments that satisfy it.")
		return SolveReport{Subject: name, Status: SolveSat, Solver: result.Solver, Lines: lines}
	case solve.StatusUnsat:
		if report, rounded := roundedNoValuesReport(name, subject, result, q); rounded {
			return report
		}
		return SolveReport{Subject: name, Status: SolveUnsat, Solver: result.Solver,
			Lines: noValuesLines(subject, q, result, unfixed)}
	default:
		lines := []string{fmt.Sprintf("? %s has no values decided either way (%s)", subject, solveDetail(result))}
		if reason := solveReason(result, q); reason != "" {
			lines = append(lines, "  "+reason)
		}
		lines = append(lines, fixedLines(q)...)
		return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver, Lines: lines}
	}
}

// roundedNoValuesReport downgrades a no-values verdict about conditions the
// evaluator rounds: exact-real unsatisfiability does not decide its arithmetic.
func roundedNoValuesReport(name, subject string, result *solve.Result, q *solve.Query) (SolveReport, bool) {
	if !q.Rounded() {
		return SolveReport{}, false
	}
	lines := []string{
		fmt.Sprintf("? %s has no values decided either way (%s)", subject, solveDetail(result)),
		"  " + roundedUnsat,
	}
	lines = append(lines, fixedLines(q)...)
	return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver, Lines: lines}, true
}

// noValuesLines reports that no values satisfy the element, distinguishing values
// that conflict with what is already fixed from conditions that conflict on their
// own, and naming which fixed values take part in the conflict.
func noValuesLines(subject string, q *solve.Query, result *solve.Result, unfixed []solve.Unfixed) []string {
	if !q.Fixes() {
		lines := []string{fmt.Sprintf("✗ %s has no satisfying values: its conditions conflict on their own (%s)",
			subject, solveDetail(result))}
		lines = append(lines, unfixedLines(unfixed)...)
		return lines
	}
	lines := []string{fmt.Sprintf("✗ %s has no values consistent with the %s already fixed (%s)",
		subject, plural(len(q.Pinned), "value", "values"), solveDetail(result))}
	lines = append(lines, fixedLines(q)...)
	lines = append(lines, conflictingFixed(q, result.Core)...)
	return append(lines, unfixedLines(unfixed)...)
}

// conflictingFixed names the fixed values taking part in the conflict behind an unsat
// verdict; without a conflict there is nothing the fixed values above do not already say.
func conflictingFixed(q *solve.Query, core *solve.Core) []string {
	if core == nil {
		return nil
	}
	var conflicting []string
	for _, i := range core.Indices {
		for _, pinned := range q.Pinned {
			if pinned.Index == i {
				conflicting = append(conflicting, notationName(pinned.Var.Name)+" = "+pinned.Value)
			}
		}
	}
	if len(conflicting) == 0 {
		return []string{"  No fixed value is part of the conflict: the conditions conflict among themselves."}
	}
	return []string{fmt.Sprintf("  In the conflict: %s", strings.Join(conflicting, ", "))}
}

// fixedLines lists the values a query fixed, and where each came from.
func fixedLines(q *solve.Query) []string {
	if len(q.Pinned) == 0 {
		return nil
	}
	lines := []string{"  Already fixed:"}
	for _, p := range q.Pinned {
		lines = append(lines, fmt.Sprintf("    %s = %s  (%s)", notationName(p.Var.Name), p.Value, fixedFrom(p)))
	}
	return lines
}

// fixedFrom says where a fixed value came from, naming the object holding it.
func fixedFrom(p solve.PinnedValue) string {
	if p.Source == solve.PinHeld {
		return fmt.Sprintf("held by object %d", p.Object)
	}
	if p.Source == solve.PinChosen {
		return "chosen"
	}
	return "declared"
}

// synthesisedLines lists the values the solver chose for what stayed free.
func synthesisedLines(q *solve.Query, result *solve.Result) []string {
	free := make(map[string]bool, len(q.Vars))
	for _, v := range q.Free() {
		free[v.Name] = true
	}
	var chosen []solve.Assignment
	for _, a := range result.Model {
		if free[a.Var.Name] {
			chosen = append(chosen, a)
		}
	}
	if len(chosen) == 0 {
		return []string{"  Nothing was left to synthesise: every value the conditions read is fixed."}
	}
	return append([]string{"  Synthesised:"}, indent(assignmentLines(chosen))...)
}

// unfixedLines says which declared values could not be read, so a reader knows
// the query left them free rather than took them as fixed.
func unfixedLines(unfixed []solve.Unfixed) []string {
	if len(unfixed) == 0 {
		return nil
	}
	lines := []string{"  Left free, since their declared value could not be read:"}
	for _, u := range unfixed {
		lines = append(lines, fmt.Sprintf("    %s: %s", u.Name, u.Reason))
	}
	return lines
}

// unreadLines says which fixed values no condition reads, so a value asked for
// and not used is reported rather than quietly dropped.
func unreadLines(q *solve.Query) []string {
	var lines []string
	for _, u := range q.Unread {
		if u.Pin.Source != solve.PinChosen {
			// A declared value the conditions do not read says nothing about them.
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s was not used: %s", u.Pin.Name, u.Reason))
	}
	return lines
}

// indent shifts already-indented lines one level further.
func indent(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, "  "+line)
	}
	return out
}

// plural picks the singular or plural word for a count.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ConfigureVariants answers about the variants an element's conditions permit:
// with no selection it synthesises a consistent one, with selections
// `variation=variant` it checks the ones given, and with `all` it enumerates
// consistent selections up to the bound (limit, when a count follows `all`).
// Experimental, and read-only: no object is created and no variant is bound.
func (s *Session) ConfigureVariants(name string, args []string) []SolveReport {
	defer s.enter()()
	return s.configureVariants(name, args)
}

func (s *Session) configureVariants(name string, args []string) []SolveReport {
	request, err := parseConfigure(args)
	if err != nil {
		return []SolveReport{unavailableReport(name, err.Error())}
	}
	queries, bad := s.solveQueries(name)
	if bad != nil {
		return []SolveReport{*bad}
	}
	plan, err := s.solveWith(name, queries, request.ask(name))
	return solveReports(name, queries, plan, err, func(name string, q *solve.Query, configured analysis.Evaluation) SolveReport {
		return configureReport(name, q, configured, request)
	})
}

// configureRequest is what a %configure asked for: the variants chosen, whether
// every consistent selection was asked for, and how many of them at most.
type configureRequest struct {
	chosen map[string]string
	order  []string
	all    bool
	limit  int
}

// parseConfigure reads the command's arguments: `all [<count>]`, or
// `<variation>=<variant>` for each variation point chosen.
func parseConfigure(args []string) (configureRequest, error) {
	request := configureRequest{chosen: map[string]string{}}
	for i, arg := range args {
		switch {
		case arg == "all":
			if i != 0 {
				return request, fmt.Errorf("`all` asks for every consistent selection, so it stands alone")
			}
			request.all = true
		case request.all:
			if request.limit != 0 {
				return request, fmt.Errorf("`all` takes one count of selections to report, so %s is one word too many", arg)
			}
			n, err := strconv.Atoi(arg)
			if err != nil || n <= 0 {
				return request, fmt.Errorf("`all %s` is not a count of selections to report: give a positive number", arg)
			}
			request.limit = n
		default:
			variation, variant, ok := strings.Cut(arg, "=")
			variation, variant = strings.TrimSpace(variation), strings.TrimSpace(variant)
			if !ok || variation == "" || variant == "" {
				return request, fmt.Errorf("%q is not a selection: write <variation>=<variant>, or `all` for every consistent selection", arg)
			}
			if _, twice := request.chosen[variation]; twice {
				return request, fmt.Errorf("%s is chosen twice: a variation point takes one variant", variation)
			}
			request.chosen[variation] = variant
			request.order = append(request.order, variation)
		}
	}
	return request, nil
}

// ask puts one query to the solver: chosen variants fixed and an inconsistent selection
// explained; for `all` an enumeration, withheld where the evaluator rounds; else one solve.
func (r configureRequest) ask(name string) analysis.Asking {
	return func(solver *solve.Solver, ctx context.Context, q *solve.Query) (*solve.Result, error) {
		variations := q.Variations()
		if len(variations) == 0 {
			return nil, fmt.Errorf("%s: %s. Use %%check %s for satisfiability", solveSubject(q), solve.ErrNoVariations, name)
		}
		switch {
		case len(r.chosen) > 0:
			if err := chooseVariants(q, variations, r); err != nil {
				return nil, err
			}
			result, err := solver.Solve(ctx, q)
			if err != nil || result.Status != solve.StatusUnsat || q.Rounded() {
				return result, err
			}
			return withConflict(solver, ctx, q, result), nil
		case r.all:
			// Enumerating "all" selections over conditions the evaluator rounds would
			// claim exact-real completeness the evaluator's arithmetic may not share.
			if q.Rounded() {
				return nil, nil
			}
			return solver.Configurations(ctx, q, r.limit)
		}
		return solver.Solve(ctx, q)
	}
}

// configureReport renders one query's answer: the selection given checked,
// the consistent selections enumerated, or one synthesised.
func configureReport(name string, q *solve.Query, configured analysis.Evaluation, request configureRequest) SolveReport {
	result, err := configured.Solved, configured.Err
	if err != nil {
		return unavailableReport(name, err.Error())
	}
	switch {
	case len(request.chosen) > 0:
		return selectionReport(name, q, result)
	case request.all:
		return configurationsReport(name, q, result)
	}
	return configurationReport(name, q, result)
}

// chooseVariants fixes each chosen variant in the query, resolving the names
// written against the variation points the conditions read and the variants they
// offer. A name that matches none, or matches several, is refused rather than
// guessed at.
func chooseVariants(q *solve.Query, variations []*solve.Var, request configureRequest) error {
	for _, written := range request.order {
		v, err := matchVariation(variations, written)
		if err != nil {
			return err
		}
		variant, err := matchVariant(v, request.chosen[written])
		if err != nil {
			return err
		}
		if err := q.FixValue(v, variant, solve.PinChosen); err != nil {
			return err
		}
	}
	return nil
}

// matchVariation is the variation point a written name denotes: the variable whose
// qualified name it is, or whose name it ends at.
func matchVariation(variations []*solve.Var, written string) (*solve.Var, error) {
	var matched []*solve.Var
	for _, v := range variations {
		if namesFeature(v.Name, written) {
			matched = append(matched, v)
		}
	}
	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return nil, fmt.Errorf("%s is not a variation point these conditions read (%s)",
			written, strings.Join(featureNames(variations), ", "))
	default:
		return nil, fmt.Errorf("%s names more than one variation point these conditions read (%s): write the qualified name",
			written, strings.Join(featureNames(matched), ", "))
	}
}

// matchVariant is the variant a written name denotes among the ones a variation
// point offers.
func matchVariant(v *solve.Var, written string) (string, error) {
	var matched []string
	for _, value := range v.Sort.Values {
		if namesFeature(value, written) {
			matched = append(matched, value)
		}
	}
	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return "", fmt.Errorf("%s is not a variant of %s (%s)",
			written, notationName(v.Name), strings.Join(notationNames(v.Sort.Values), ", "))
	default:
		return "", fmt.Errorf("%s names more than one variant of %s (%s): write the qualified name",
			written, notationName(v.Name), strings.Join(notationNames(matched), ", "))
	}
}

// namesFeature reports whether a written name denotes a qualified name: the whole
// of it, or its trailing segments, as any name the prompt takes may be written.
// Segments are compared whole, so a name ending part of one does not name it.
func namesFeature(qualified, written string) bool {
	segments := strings.Split(written, "::")
	return trailingSegments(qualified, segments) || trailingSegments(notationName(qualified), segments)
}

// trailingSegments reports whether the segments written are a qualified name's
// last ones, each segment matching in full.
func trailingSegments(qualified string, written []string) bool {
	have := strings.Split(qualified, "::")
	if len(written) == 0 || len(written) > len(have) {
		return false
	}
	return slices.Equal(have[len(have)-len(written):], written)
}

// featureNames names variables as the notation writes them.
func featureNames(vars []*solve.Var) []string {
	out := make([]string, 0, len(vars))
	for _, v := range vars {
		out = append(out, notationName(v.Name))
	}
	return out
}

// notationNames writes qualified names as the notation does.
func notationNames(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, notationName(name))
	}
	return out
}

// selectionReport reports whether the chosen variants are consistent with the
// element's conditions.
func selectionReport(name string, q *solve.Query, result *solve.Result) SolveReport {
	subject := solveSubject(q)
	switch result.Status {
	case solve.StatusSat:
		lines := []string{fmt.Sprintf("✓ the chosen variants are consistent with %s (%s)", subject, solveDetail(result))}
		lines = append(lines, fixedLines(q)...)
		return SolveReport{Subject: name, Status: SolveSat, Solver: result.Solver,
			Lines: append(lines, synthesisedLines(q, result)...)}
	case solve.StatusUnsat:
		if q.Rounded() {
			lines := []string{fmt.Sprintf("? whether the chosen variants are consistent with %s is undecided (%s)",
				subject, solveDetail(result)), "  " + roundedUnsat}
			return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver,
				Lines: append(lines, fixedLines(q)...)}
		}
		lines := []string{fmt.Sprintf("✗ the chosen variants are not consistent with %s (%s)", subject, solveDetail(result))}
		lines = append(lines, fixedLines(q)...)
		return SolveReport{Subject: name, Status: SolveUnsat, Solver: result.Solver,
			Lines: append(lines, conflictingFixed(q, result.Core)...)}
	default:
		lines := []string{fmt.Sprintf("? whether the chosen variants are consistent with %s is undecided (%s)",
			subject, solveDetail(result))}
		if reason := solveReason(result, q); reason != "" {
			lines = append(lines, "  "+reason)
		}
		return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver, Lines: lines}
	}
}

// configurationReport reports one consistent selection of variants.
func configurationReport(name string, q *solve.Query, result *solve.Result) SolveReport {
	subject := solveSubject(q)
	switch result.Status {
	case solve.StatusSat:
		lines := []string{fmt.Sprintf("✓ %s permits a selection of variants (%s)", subject, solveDetail(result))}
		lines = append(lines, selectionLines(selectionOf(result.Model, q.Variations()))...)
		lines = append(lines, fmt.Sprintf("  One witness: use %%configure %s all for every consistent selection.", name))
		return SolveReport{Subject: name, Status: SolveSat, Solver: result.Solver, Lines: lines}
	case solve.StatusUnsat:
		if q.Rounded() {
			return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver, Lines: []string{
				fmt.Sprintf("? whether %s permits a selection of variants is undecided (%s)", subject, solveDetail(result)),
				"  " + roundedUnsat,
			}}
		}
		return SolveReport{Subject: name, Status: SolveUnsat, Solver: result.Solver, Lines: []string{
			fmt.Sprintf("✗ %s permits no selection of variants (%s)", subject, solveDetail(result)),
		}}
	default:
		lines := []string{fmt.Sprintf("? whether %s permits a selection of variants is undecided (%s)",
			subject, solveDetail(result))}
		if reason := solveReason(result, q); reason != "" {
			lines = append(lines, "  "+reason)
		}
		return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver, Lines: lines}
	}
}

// configurationsReport reports the consistent selections of variants up to the bound, saying
// when it stopped there rather than showing there is no other; withheld is undecided.
func configurationsReport(name string, q *solve.Query, result *solve.Result) SolveReport {
	if result == nil {
		return SolveReport{Subject: name, Status: SolveUnknown, Lines: []string{
			fmt.Sprintf("? which variants %s permits is undecided", solveSubject(q)),
			"  " + roundedClaim,
		}}
	}
	subject := solveSubject(q)
	if result.Status == solve.StatusUnknown {
		lines := []string{fmt.Sprintf("? which variants %s permits is undecided (%s)", subject, solveDetail(result))}
		if reason := solveReason(result, q); reason != "" {
			lines = append(lines, "  "+reason)
		}
		return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver, Lines: lines}
	}
	if len(result.Solutions) == 0 {
		return SolveReport{Subject: name, Status: SolveUnsat, Solver: result.Solver, Lines: []string{
			fmt.Sprintf("✗ %s permits no selection of variants (%s)", subject, solveDetail(result)),
		}}
	}
	lines := []string{fmt.Sprintf("✓ %s permits %d %s of variants%s (%s)",
		subject, len(result.Solutions), plural(len(result.Solutions), "selection", "selections"),
		truncation(result), solveDetail(result))}
	for i, solution := range result.Solutions {
		lines = append(lines, fmt.Sprintf("  %d.", i+1))
		lines = append(lines, indent(selectionLines(solution))...)
	}
	return SolveReport{Subject: name, Status: SolveSat, Solver: result.Solver, Lines: lines}
}

// truncation says whether the selections reported are all there are, or as many
// as the bound allowed.
func truncation(result *solve.Result) string {
	if !result.Truncated {
		return ", which are all of them"
	}
	if result.TimedOut {
		return ", reported before the solver ran out of time"
	}
	if result.Undecided {
		return ", reported before the solver stopped deciding"
	}
	return ", reported up to the bound on how many are enumerated"
}

// selectionOf is the values a model gave the variation points, in their order.
func selectionOf(model []solve.Assignment, variations []*solve.Var) []solve.Assignment {
	wanted := make(map[*solve.Var]bool, len(variations))
	for _, v := range variations {
		wanted[v] = true
	}
	out := make([]solve.Assignment, 0, len(variations))
	for _, a := range model {
		if wanted[a.Var] {
			out = append(out, a)
		}
	}
	return out
}

// selectionLines writes one selection, a variation point and its variant a line.
func selectionLines(selection []solve.Assignment) []string {
	lines := make([]string, 0, len(selection))
	for _, a := range selection {
		lines = append(lines, fmt.Sprintf("  %s = %s", notationName(a.Var.Name), notationName(a.Value)))
	}
	return lines
}

// doSolve carries out %solve.
func (s *Session) doSolve(name string) ([]string, bool, error) {
	var out []string
	for _, r := range s.solveValues(name) {
		out = append(out, r.Lines...)
	}
	return out, false, nil
}

// doConfigure carries out %configure.
func (s *Session) doConfigure(name string, args []string) ([]string, bool, error) {
	var out []string
	for _, r := range s.configureVariants(name, args) {
		out = append(out, r.Lines...)
	}
	return out, false, nil
}
