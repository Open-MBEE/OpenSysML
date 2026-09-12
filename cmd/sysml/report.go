package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
)

// The prefix this command reports a failure under, as `prog: ` does in any Unix
// tool, and the one the prompt renders the same failure under.
const (
	commandPrefix = "sysml: "
	promptPrefix  = "error: "
)

// asCommandProblem restates the lines of a check that could not be made in the
// command's prefix. A line locating a finding in the source
// (`model.sysml:1:42: error: …`) is about the model, and keeps its own shape.
func asCommandProblem(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if rest, ok := strings.CutPrefix(line, promptPrefix); ok {
			line = commandPrefix + rest
		}
		out = append(out, line)
	}
	return out
}

// reporter collects what a check mode run decided and reports it, either as the
// lines the prompt prints or as one JSON document. It also owns the exit status,
// which is the same in both forms: the worst verdict reached, or the status of a
// check that could not be made.
type reporter struct {
	out  io.Writer
	err  io.Writer
	json bool

	verdicts []repl.Verdict
	report   checkReport
	// findings counts the model errors the run itself produced, which are
	// reported as diagnostics and decide no check.
	findings int
}

// checkReport is the JSON document -json writes: what was checked, what analysis
// found, what stopped a check, and the status the exit code reports. It is the
// contract a caller parses, so the fields are always present, `null` for what a
// run produced nothing of.
type checkReport struct {
	// Status is the worst verdict reached: holds, fails or unresolved.
	Status string `json:"status"`
	Exit   int    `json:"exit"`
	// Checks are the verdicts and runs, in the order they were decided.
	Checks []checkResult `json:"checks"`
	// Diagnostics is what analysis of the model found.
	Diagnostics []diagnostic `json:"diagnostics"`
	// Output is what the run printed about itself: loads, evaluations, objects.
	Output []string `json:"output"`
	// Errors is what stopped a check from being made.
	Errors []string `json:"errors"`
}

// diagnostic is one finding about the model, as the JSON report spells it.
type diagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	// File is the model file the finding is in, which Line and Column locate it in.
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Pass   string `json:"pass"`
	Code   string `json:"code"`
}

// namedValue is one value a check or run produced, as the JSON report spells it.
type namedValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// namedValues converts what a verdict produced into the reported form.
func namedValues(values []repl.NamedValue) []namedValue {
	if len(values) == 0 {
		return nil
	}
	out := make([]namedValue, 0, len(values))
	for _, v := range values {
		out = append(out, namedValue{Name: v.Name, Value: v.Value})
	}
	return out
}

// checkResult is one verdict or run in the JSON report.
type checkResult struct {
	Subject string `json:"subject"`
	Status  string `json:"status"`
	// Values are what the check or run produced: a calculation's result, a
	// machine's configuration.
	Values []namedValue `json:"values"`
	// Lines is the verdict as the prompt prints it.
	Lines []string `json:"lines"`
	// Verifications are the verdicts the bodies of the verification cases
	// verifying the requirement produced, reported beside its own.
	Verifications []verificationVerdict `json:"verifications,omitempty"`
	// Evaluations are the applications an analysis run made of its own calcs as
	// values — a trade study's evaluation of each alternative — in the order made.
	Evaluations []caseEvaluation `json:"evaluations,omitempty"`
	// Rows are the runs a sweep made, one per row of its table; `null` for every
	// other kind of check.
	Rows []checkRow `json:"rows"`
	// Outcomes are the distinct outcomes an exploration reached, in canonical
	// order, and Exploration how it ended; only a run under `explore` has them.
	Outcomes    []checkOutcome    `json:"outcomes,omitempty"`
	Exploration *checkExploration `json:"exploration,omitempty"`
	// Plan is how the engines answered and Results what each answered, one entry
	// per engine that ran (empty when every engine refused); only a check put to
	// the engines has them.
	Plan    *checkPlan      `json:"plan,omitempty"`
	Results []checkResultOf `json:"results,omitzero"`
}

// checkPlan is how a check was answered in the JSON report: the selection made,
// every engine consulted in order, the standing that stood, the disagreements resolved
// and what the plan's workers cost.
type checkPlan struct {
	// Engine is the selection: auto, all, or the engine named.
	Engine   string      `json:"engine"`
	Standing string      `json:"standing"`
	Steps    []checkStep `json:"steps"`
	// Disagreements are the contradictions all resolved, each in the interpreter's favor.
	Disagreements []checkDisagreement `json:"disagreements,omitempty"`
	// Workers is how many workers the plan built for its engines' runs, and Warming the
	// wall time in milliseconds building them took; the human-readable report omits both.
	Workers int     `json:"workers"`
	Warming float64 `json:"warming"`
}

// checkStep is one engine's part in the plan.
type checkStep struct {
	Engine string `json:"engine"`
	// Status is answered, refused, failed or cancelled.
	Status string `json:"status"`
	// Detail is the refusal or the fault, empty for an engine that answered.
	Detail string `json:"detail,omitempty"`
	// Bounds is the bound a cancelled engine reached.
	Bounds []checkBound `json:"bounds,omitempty"`
}

// checkDisagreement is one contradiction the composition under all resolved.
type checkDisagreement struct {
	Stands  string `json:"stands"`
	Demoted string `json:"demoted"`
	// Claim and Strength are what the demoted engine answered before it was demoted.
	Claim    string `json:"claim"`
	Strength string `json:"strength"`
	Reason   string `json:"reason"`
}

// checkResultOf is what one engine answered, as the JSON report spells it.
type checkResultOf struct {
	Engine   string       `json:"engine"`
	Claim    string       `json:"claim"`
	Strength string       `json:"strength"`
	Bounds   []checkBound `json:"bounds"`
	// Witness is the execution the interpreter replays to exhibit the claim; `null` without one.
	Witness *checkWitness `json:"witness"`
	// Reason is why nothing is claimed, empty for a covered result.
	Reason   string `json:"reason,omitempty"`
	Standing string `json:"standing"`
	// Check is the search the check engine made; only its results have one.
	Check *checkSearch `json:"check,omitempty"`
}

// checkSearch is how the check engine's search ended in the JSON report: its
// verdict, what it searched, the bounds it hit, and what it found.
type checkSearch struct {
	Verdict string `json:"verdict"`
	States  int    `json:"states"`
	Moves   int    `json:"moves"`
	Depth   int    `json:"depth"`
	// BoundsHit names the bounds the search hit, `[]` when it was exhaustive.
	BoundsHit  []string         `json:"boundsHit"`
	Violations []checkViolation `json:"violations"`
	Divergent  []checkDivergent `json:"divergent"`
	// Outcomes are the distinct final outcomes complete schedules reached.
	Outcomes []string `json:"outcomes"`
}

// checkViolation is one violation the search found, with the schedule reaching it.
type checkViolation struct {
	Kind string `json:"kind"`
	// Name is the property violated, empty for a deadlock or a failure.
	Name string `json:"name,omitempty"`
	// Error is the deadlock or failure as reported, empty for a property.
	Error string `json:"error,omitempty"`
	Depth int    `json:"depth"`
	// Witness is the choices that fix the schedule, and Path where the witness file
	// was written, empty without a directory.
	Witness []string `json:"witness"`
	Path    string   `json:"path,omitempty"`
}

// checkDivergent is one feature the schedule decides the final value of.
type checkDivergent struct {
	Feature string                `json:"feature"`
	Values  []checkDivergentValue `json:"values"`
}

// checkDivergentValue is one final value of a divergent feature and a schedule reaching it.
type checkDivergentValue struct {
	Value   string   `json:"value"`
	Witness []string `json:"witness"`
	Path    string   `json:"path,omitempty"`
}

// checkBound is one limit an engine took, and whether it reached it.
type checkBound struct {
	Name    string `json:"name"`
	Limit   int64  `json:"limit"`
	Reached bool   `json:"reached"`
}

// checkWitness is a replayable execution: the policy it ran under and its choices.
type checkWitness struct {
	Schedule string   `json:"schedule"`
	Choices  []string `json:"choices"`
}

// checkBounds converts the bounds an engine took into the reported form; a
// result's bounds are always present, `[]` for an engine that took none.
func checkBounds(bounds analysis.Bounds) []checkBound {
	out := make([]checkBound, 0, len(bounds))
	for _, b := range bounds {
		out = append(out, checkBound{Name: b.Name, Limit: b.Limit, Reached: b.Reached})
	}
	return out
}

// checkWitnessOf converts a result's witness into the reported form.
func checkWitnessOf(w *analysis.Witness) *checkWitness {
	if w == nil {
		return nil
	}
	choices := make([]string, 0, len(w.Choices))
	for _, c := range w.Choices {
		choices = append(choices, c.String())
	}
	return &checkWitness{Schedule: w.Schedule.String(), Choices: choices}
}

// checkPlanOf converts the plan that answered a check into the reported form.
func checkPlanOf(plan *analysis.Plan) *checkPlan {
	if plan == nil {
		return nil
	}
	out := &checkPlan{
		Engine:   plan.Selection.String(),
		Standing: plan.Standing(),
		Steps:    make([]checkStep, 0, len(plan.Steps)),
		Workers:  plan.Workers,
		Warming:  float64(plan.Warming.Nanoseconds()) / 1e6,
	}
	for _, step := range plan.Steps {
		s := checkStep{Engine: step.Engine}
		switch {
		case step.Refusal != nil:
			s.Status, s.Detail = "refused", step.Refusal.Error()
		case step.Cancelled:
			s.Status = "cancelled"
			if len(step.Bounds) > 0 {
				s.Bounds = checkBounds(step.Bounds)
			}
		case step.Err != nil:
			s.Status, s.Detail = "failed", step.Err.Error()
		default:
			s.Status = "answered"
		}
		out.Steps = append(out.Steps, s)
	}
	for _, d := range plan.Disagreements {
		out.Disagreements = append(out.Disagreements, checkDisagreement{
			Stands:   d.Stands,
			Demoted:  d.Demoted,
			Claim:    d.Claimed.Claim.String(),
			Strength: d.Claimed.Strength.String(),
			Reason:   d.Reason,
		})
	}
	return out
}

// checkResultsOf converts what each engine of a plan answered into the reported form.
func checkResultsOf(plan *analysis.Plan) []checkResultOf {
	if plan == nil {
		return nil
	}
	results := plan.Results()
	out := make([]checkResultOf, 0, len(results))
	for _, r := range results {
		out = append(out, checkResultOf{
			Engine:   r.Engine,
			Claim:    r.Claim.String(),
			Strength: r.Strength.String(),
			Bounds:   checkBounds(r.Bounds),
			Witness:  checkWitnessOf(r.Witness),
			Reason:   r.Reason,
			Standing: r.Standing(),
			Check:    checkSearchOf(r.Check()),
		})
	}
	return out
}

// checkSearchOf converts a check engine's search into the reported form.
func checkSearchOf(checked *analysis.Checked) *checkSearch {
	if checked == nil || checked.Report == nil {
		return nil
	}
	report := checked.Report
	out := &checkSearch{
		Verdict:    report.Verdict.String(),
		States:     report.States,
		Moves:      report.Moves,
		Depth:      report.MaxDepth,
		BoundsHit:  append([]string{}, report.BoundsHit...),
		Violations: make([]checkViolation, 0, len(report.Violations)),
		Divergent:  make([]checkDivergent, 0, len(report.Divergent)),
		Outcomes:   make([]string, 0, len(report.Finals)),
	}
	for i, v := range report.Violations {
		violation := checkViolation{Kind: v.Kind.String(), Name: v.Name, Depth: v.Depth, Witness: choiceStrings(v.Witness.Choices), Path: pathAt(checked.Violations, i)}
		if v.Err != nil {
			violation.Error = v.Err.Error()
		}
		out.Violations = append(out.Violations, violation)
	}
	for i, d := range report.Divergent {
		var paths []string
		if i < len(checked.Divergent) {
			paths = checked.Divergent[i]
		}
		divergent := checkDivergent{Feature: d.Feature, Values: make([]checkDivergentValue, 0, len(d.Values))}
		for j, value := range d.Values {
			divergent.Values = append(divergent.Values, checkDivergentValue{Value: value.Value, Witness: choiceStrings(value.Witness.Choices), Path: pathAt(paths, j)})
		}
		out.Divergent = append(out.Divergent, divergent)
	}
	for _, final := range report.Finals {
		out.Outcomes = append(out.Outcomes, final.Outcome)
	}
	return out
}

// choiceStrings spells a schedule's choices, `[]` for a run facing none.
func choiceStrings(choices []runtime.ChoiceTaken) []string {
	out := make([]string, 0, len(choices))
	for _, c := range choices {
		out = append(out, c.String())
	}
	return out
}

// pathAt is the i'th witness path, "" when none was written.
func pathAt(paths []string, i int) string {
	if i < len(paths) {
		return paths[i]
	}
	return ""
}

// checkOutcome is one distinct outcome of an exploration in the JSON report.
type checkOutcome struct {
	Values []namedValue `json:"values"`
	// Error is what stopped the runs reaching this outcome, empty for one they completed.
	Error          string `json:"error,omitempty"`
	Linearizations int    `json:"linearizations"`
	// Witness is one run's choice sequence, a choice per entry in run order.
	Witness []string `json:"witness"`
}

// checkExploration is how an exploration ended in the JSON report.
type checkExploration struct {
	Complete bool `json:"complete"`
	Runs     int  `json:"runs"`
	// BudgetsHit names the budgets hit, `runs` before `depth`; empty when complete.
	BudgetsHit []string `json:"budgetsHit"`
}

// checkOutcomes converts the outcomes of an exploration into the reported form.
func checkOutcomes(outcomes []repl.VerdictOutcome) []checkOutcome {
	if len(outcomes) == 0 {
		return nil
	}
	out := make([]checkOutcome, 0, len(outcomes))
	for _, o := range outcomes {
		out = append(out, checkOutcome{
			Values:         namedValues(o.Values),
			Error:          o.Error,
			Linearizations: o.Linearizations,
			Witness:        append([]string{}, o.Witness...),
		})
	}
	return out
}

// checkExplorationOf converts how an exploration ended into the reported form.
func checkExplorationOf(x *repl.VerdictExploration) *checkExploration {
	if x == nil {
		return nil
	}
	return &checkExploration{
		Complete:   x.Complete,
		Runs:       x.Runs,
		BudgetsHit: append([]string{}, x.BudgetsHit...),
	}
}

// caseEvaluation is one application an analysis run made of a calc as a value.
type caseEvaluation struct {
	Function  string   `json:"function"`
	Arguments []string `json:"arguments"`
	// Result is the value computed; absent when the evaluation failed.
	Result string `json:"result,omitempty"`
	// Error is what stopped the evaluation, empty for one that completed.
	Error string `json:"error,omitempty"`
	// Selected marks the evaluation of the alternative the run selected; Tied one
	// whose result equals the selected one's without being selected.
	Selected bool `json:"selected,omitempty"`
	Tied     bool `json:"tied,omitempty"`
}

// caseEvaluations reports the evaluations of a check as JSON data.
func caseEvaluations(evaluations []repl.Evaluation) []caseEvaluation {
	if len(evaluations) == 0 {
		return nil
	}
	out := make([]caseEvaluation, 0, len(evaluations))
	for _, e := range evaluations {
		arguments := e.Arguments
		if arguments == nil {
			arguments = []string{}
		}
		out = append(out, caseEvaluation{
			Function:  e.Function,
			Arguments: arguments,
			Result:    e.Result,
			Error:     e.Error,
			Selected:  e.Selected,
			Tied:      e.Tied,
		})
	}
	return out
}

// verificationVerdict is one verdict a verification case body produced.
type verificationVerdict struct {
	Case string `json:"case"`
	Kind string `json:"kind"`
	// Detail is why an error or inconclusive verdict decided nothing.
	Detail string `json:"detail,omitempty"`
	// Subcase marks a case another case performed as a step of its body.
	Subcase bool `json:"subcase,omitempty"`
}

// checkRow is one run of a sweep in the JSON report.
type checkRow struct {
	// Inputs are the parameters the row bound, in the order the ranges were given.
	Inputs   []namedValue `json:"inputs"`
	Outputs  []namedValue `json:"outputs"`
	Verdicts []namedValue `json:"verdicts"`
	// Evaluations are the applications that run made of the case's calcs as values.
	Evaluations []caseEvaluation `json:"evaluations,omitempty"`
	// Milliseconds is the wall time of that run.
	Milliseconds float64 `json:"milliseconds"`
	// Error is what stopped the run, empty for one that completed.
	Error string `json:"error"`
}

// checkRows converts the runs of a sweep into the reported form.
func checkRows(rows []repl.VerdictRow) []checkRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]checkRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, checkRow{
			Inputs:       namedValues(row.Inputs),
			Outputs:      namedValues(row.Outputs),
			Verdicts:     namedValues(row.Verdicts),
			Evaluations:  caseEvaluations(row.Evaluations),
			Milliseconds: row.Millis,
			Error:        row.Error,
		})
	}
	return out
}

func newReporter(asJSON bool) *reporter {
	return &reporter{out: os.Stdout, err: os.Stderr, json: asJSON}
}

// info reports what the run did, which is not a verdict about the model.
func (r *reporter) info(lines []string) {
	if len(lines) == 0 {
		return
	}
	if r.json {
		r.report.Output = append(r.report.Output, lines...)
		return
	}
	writeLines(r.out, lines)
}

// problem reports lines about something that stopped the run, which are kept off
// stdout so that redirecting a verdict report cannot hide them.
func (r *reporter) problem(lines []string) {
	if len(lines) == 0 {
		return
	}
	if r.json {
		r.report.Output = append(r.report.Output, lines...)
		return
	}
	writeLines(r.err, lines)
}

// failed records what stopped a check from being made, which makes the run
// unresolved however the checks themselves came out.
func (r *reporter) failed(message string) {
	r.report.Errors = append(r.report.Errors, message)
	if !r.json {
		fmt.Fprintln(r.err, commandPrefix+message)
	}
}

// finding records a diagnostic the run itself produced — a feature value whose value
// could not be materialized — and reports it like one analysis found. It is a
// model error rather than a verdict, so it leaves the run undecided.
func (r *reporter) finding(message string) {
	r.findings++
	r.report.Diagnostics = append(r.report.Diagnostics, diagnostic{
		Severity: "error",
		Message:  message,
		Pass:     "runtime",
		Code:     "runtime.materialize",
	})
	if !r.json {
		fmt.Fprintln(r.err, promptPrefix+message)
	}
}

// warn records something about the run that is no model error and decides no
// check — a check that was bounded rather than completed.
func (r *reporter) warn(message string) {
	r.report.Diagnostics = append(r.report.Diagnostics, diagnostic{
		Severity: "warning",
		Message:  message,
		Pass:     "runtime",
		Code:     "runtime.materialize.bounded",
	})
	if !r.json {
		fmt.Fprintln(r.err, "warning: "+message)
	}
}

// clean reports whether the run has found nothing wrong with the model so far.
func (r *reporter) clean() bool {
	return r.findings == 0 && len(r.report.Errors) == 0
}

// diags records what analysis found. In JSON it is reported as data; the lines
// the load already produced carry it in the printed form.
func (r *reporter) diags(diags []repl.Diagnostic) {
	for _, d := range diags {
		r.report.Diagnostics = append(r.report.Diagnostics, diagnostic{
			Severity: d.Severity,
			Message:  d.Message,
			File:     d.File,
			Line:     d.Line,
			Column:   d.Column,
			Pass:     d.Pass,
			Code:     d.Code,
		})
	}
}

// verdict records one decided check and, when printing, reports it: a verdict
// the model decided on stdout, and one that was never decided on stderr, where
// the other failures to run go.
func (r *reporter) verdict(v repl.Verdict) {
	r.verdicts = append(r.verdicts, v)
	if r.json {
		// The lines are reported with the check itself, not twice.
		return
	}
	if v.Status == repl.VerdictUnresolved {
		r.problem(asCommandProblem(v.Lines))
		return
	}
	r.info(v.Lines)
}

// finish reports the run's outcome and returns its exit status.
func (r *reporter) finish() int {
	status, exit := r.status()
	if !r.json {
		return exit
	}
	r.report.Status = status.String()
	r.report.Exit = exit
	for _, v := range r.verdicts {
		r.report.Checks = append(r.report.Checks, checkResult{
			Subject:       v.Subject,
			Status:        v.Status.String(),
			Values:        namedValues(v.Values),
			Lines:         v.Lines,
			Verifications: verificationVerdicts(v.Verifications),
			Evaluations:   caseEvaluations(v.Evaluations),
			Rows:          checkRows(v.Rows),
			Outcomes:      checkOutcomes(v.Outcomes),
			Exploration:   checkExplorationOf(v.Exploration),
			Plan:          checkPlanOf(v.Plan),
			Results:       checkResultsOf(v.Plan),
		})
	}
	out, err := json.MarshalIndent(r.report, "", "  ")
	if err != nil {
		// Unreachable: every field of the report marshals.
		fmt.Fprintln(r.err, commandPrefix+err.Error())
		return exitUnevaluable
	}
	fmt.Fprintln(r.out, string(out))
	return exit
}

// status is the outcome of the whole run: a check that could not be made leaves
// it unresolved, and otherwise the worst verdict decided stands.
func (r *reporter) status() (repl.VerdictStatus, int) {
	worst := repl.WorstStatus(r.verdicts)
	if !r.clean() {
		worst = repl.VerdictUnresolved
	}
	switch worst {
	case repl.VerdictFails:
		return worst, exitFailed
	case repl.VerdictUnresolved:
		return worst, exitUnevaluable
	default:
		return worst, exitHolds
	}
}

func writeLines(w io.Writer, lines []string) {
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
}

// verificationVerdicts reports the body verdicts of a check as JSON data.
func verificationVerdicts(verdicts []repl.VerificationVerdict) []verificationVerdict {
	if len(verdicts) == 0 {
		return nil
	}
	out := make([]verificationVerdict, 0, len(verdicts))
	for _, v := range verdicts {
		out = append(out, verificationVerdict{Case: v.Case, Kind: v.Kind, Detail: v.Detail, Subcase: v.Subcase})
	}
	return out
}
