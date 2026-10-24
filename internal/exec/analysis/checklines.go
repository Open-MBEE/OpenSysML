package analysis

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// CheckStatus is how a check's answer stands: the condition held of every searched
// schedule, some schedule failed it, or nothing was decided.
type CheckStatus int

const (
	CheckUndecided CheckStatus = iota
	CheckHolds
	CheckFails
)

// String spells the status as the service reports it.
func (s CheckStatus) String() string {
	switch s {
	case CheckHolds:
		return "holds"
	case CheckFails:
		return "fails"
	}
	return "undecided"
}

// CheckLines renders what one engine answered to a check of the behavior label
// names, as the `sysml` command prints it: the status line, then each violation
// with its probability and witness, each divergent value, and each outcome of the
// complete schedules; for an answer carrying no search report (the symbolic
// engine's or an external one's) the claim, the inputs and assumptions it rests
// on and its witnesses. Unresolved is true when no engine covered the question.
func CheckLines(label string, result Result) (CheckStatus, []string, bool) {
	checked := result.Check()
	if checked == nil {
		if result.Covered() || result.Engine == SMTEngineName {
			status, lines := decidedLines(label, result)
			return status, lines, false
		}
		return CheckUndecided, []string{fmt.Sprintf("? %s could not be checked", label), "  " + result.Reason}, true
	}
	report := checked.Report
	var status CheckStatus
	var lines []string
	switch {
	case result.Strength == NotCovered:
		lines = append(lines, fmt.Sprintf("? %s: %s", label, report.Status()), "  "+result.Reason)
	case report.Verdict == runtime.CheckViolation, report.Verdict == runtime.CheckDivergent:
		status = CheckFails
		lines = append(lines, fmt.Sprintf("✗ %s: %s", label, report.Status()))
	case report.Verdict == runtime.CheckExhaustive:
		status = CheckHolds
		lines = append(lines, fmt.Sprintf("✓ %s: %s", label, report.Status()))
	default:
		lines = append(lines, fmt.Sprintf("? %s: %s", label, report.Status()))
	}
	for i, violation := range report.Violations {
		mass := runtime.FormatWeight(violation.Mass)
		if report.MassBounded {
			mass = "≥ " + mass
		}
		lines = append(lines, "  violation: "+violation.String()+" (probability "+mass+")"+witnessPath(checked.Violations, i))
	}
	for i, d := range report.Divergent {
		lines = append(lines, "  divergent: "+d.String())
		for j, value := range d.Values {
			var path []string
			if i < len(checked.Divergent) {
				path = checked.Divergent[i]
			}
			lines = append(lines, fmt.Sprintf("    %s = %s%s", d.Feature, value.Value, witnessPath(path, j)))
		}
	}
	for _, final := range report.Finals {
		lines = append(lines, "  outcome: "+final.Outcome)
	}
	return status, lines, false
}

// decidedLines renders an answer carrying no check report: a violation or a
// sensitivity fails, a proof holds, a bounded or uncovered answer is undecided.
func decidedLines(label string, result Result) (CheckStatus, []string) {
	var status CheckStatus
	var lines []string
	switch {
	case !result.Covered():
		lines = append(lines, fmt.Sprintf("? %s: not covered", label), "  "+result.Reason)
	case result.Claim == ClaimViolated:
		status = CheckFails
		lines = append(lines, fmt.Sprintf("✗ %s: %s", label, result.Reason))
	case result.Claim == ClaimSensitive:
		status = CheckFails
		lines = append(lines, fmt.Sprintf("✗ %s: sensitive: %s", label, result.Reason))
	case result.Strength == Proved:
		status = CheckHolds
		lines = append(lines, fmt.Sprintf("✓ %s: %s", label, result.Claim))
	default:
		lines = append(lines, fmt.Sprintf("? %s: %s (%s)", label, result.Claim, result.Strength))
		if result.Question.Kind == Sensitive && result.Reason != "" {
			lines = append(lines, "  "+result.Reason)
		}
	}
	if inputs := inputLines(result.Inputs); inputs != "" {
		lines = append(lines, "  inputs: "+inputs)
	}
	if len(result.Assumptions) > 0 {
		lines = append(lines, "  assumed: "+strings.Join(result.Assumptions, ", "))
	}
	switch w := result.Witness; {
	case w != nil && result.Contrast != nil:
		lines = append(lines, "  witness A: "+WitnessLine(w), "  witness B: "+WitnessLine(result.Contrast))
	case w != nil:
		lines = append(lines, "  witness: "+WitnessLine(w))
	}
	return status, lines
}

func inputLines(inputs []Input) string {
	parts := make([]string, 0, len(inputs))
	for _, in := range inputs {
		parts = append(parts, in.String())
	}
	return strings.Join(parts, ", ")
}

// WitnessLine spells a witness: the file it was written to, else its draws and
// choices as one schedule, "no choices" for an empty one.
func WitnessLine(w *Witness) string {
	if w.Written != "" {
		return w.Written
	}
	if len(w.Draws) == 0 && len(w.Choices) == 0 {
		return "no choices"
	}
	parts := make([]string, 0, len(w.Draws)+len(w.Choices))
	for _, d := range w.Draws {
		parts = append(parts, d.String())
	}
	for _, c := range w.Choices {
		parts = append(parts, c.String())
	}
	return strings.Join(parts, "; ")
}

func witnessPath(paths []string, i int) string {
	if i >= len(paths) || paths[i] == "" {
		return ""
	}
	return " (witness " + paths[i] + ")"
}
