package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

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
