package analysis

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// Compose is the one result of several engines' answers, taken in engine-name order: a witness
// stands over the universal claim it refutes (a disagreement), else the strongest strength earned.
func Compose(results []Result) (Result, []Disagreement) {
	answered := make([]Result, 0, len(results))
	for _, r := range results {
		if r.Covered() {
			answered = append(answered, r)
		}
	}
	sort.SliceStable(answered, func(i, j int) bool { return answered[i].Engine < answered[j].Engine })
	if len(answered) == 0 {
		return Result{}, nil
	}
	if witness, ok := firstClaiming(answered, ClaimViolated); ok {
		return witness, refuted(witness, answered, ClaimHolds)
	}
	if witness, ok := firstClaiming(answered, ClaimSatisfiable); ok {
		return witness, refuted(witness, answered, ClaimUnsatisfiable)
	}
	if sensitive, ok := sensitivity(answered); ok {
		return sensitive, nil
	}
	strongest := answered[0]
	for _, r := range answered[1:] {
		if r.Strength > strongest.Strength {
			strongest = r
		}
	}
	return strongest, nil
}

// firstClaiming is the first answer, in engine-name order, making the claim.
func firstClaiming(answered []Result, claim Claim) (Result, bool) {
	for _, r := range answered {
		if r.Claim == claim {
			return r, true
		}
	}
	return Result{}, false
}

// refuted is a disagreement for every answer making the universal claim the witness refutes.
func refuted(witness Result, answered []Result, claim Claim) []Disagreement {
	var disagreements []Disagreement
	for _, r := range answered {
		if r.Claim != claim {
			continue
		}
		disagreements = append(disagreements, Disagreement{
			Stands:  witness.Engine,
			Demoted: r.Engine,
			Claimed: r,
			Reason: fmt.Sprintf("%s claimed %s (%s) but %s witnessed %s; the interpreter's witness stands",
				r.Engine, r.Claim, r.Strength, witness.Engine, witness.Claim),
		})
	}
	return disagreements
}

// demotedResult is the demoted engine's result as the plan keeps it: not covered,
// with the disagreement as its reason, its bounds and elapsed time as they were.
func demotedResult(d Disagreement) Result {
	return Result{
		Question: d.Claimed.Question,
		Engine:   d.Claimed.Engine,
		Claim:    ClaimNone,
		Strength: NotCovered,
		Bounds:   d.Claimed.Bounds,
		Reason:   d.Reason,
		Elapsed:  d.Claimed.Elapsed,
	}
}

// sensitivity is the witnessed sensitivity two engines' differing values make, when
// the answers are concrete values and not all of them agree.
func sensitivity(answered []Result) (Result, bool) {
	var valued []Result
	for _, r := range answered {
		if r.Claim == ClaimValue {
			valued = append(valued, r)
		}
	}
	if len(valued) < 2 {
		return Result{}, false
	}
	first := valued[0]
	var differing []string
	for _, r := range valued[1:] {
		if !sameValues(first.Values, r.Values) {
			differing = append(differing, fmt.Sprintf("%s gave %s", r.Engine, renderValues(r.Values)))
		}
	}
	if len(differing) == 0 {
		return Result{}, false
	}
	names := make([]string, len(valued))
	for i, r := range valued {
		names[i] = r.Engine
	}
	return Result{
		Question: first.Question,
		Engine:   strings.Join(names, ", "),
		Claim:    ClaimSensitive,
		Strength: Witnessed,
		Bounds:   first.Bounds,
		Witness:  &Witness{Schedule: first.Question.Schedule},
		Reason:   fmt.Sprintf("%s gave %s; %s", first.Engine, renderValues(first.Values), strings.Join(differing, "; ")),
		Values:   first.Values,
	}, true
}

// sameValues reports whether two value lists name the same values in the same order.
func sameValues(a, b []Evaluation) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || runtime.FormatValue(a[i].Value) != runtime.FormatValue(b[i].Value) {
			return false
		}
	}
	return true
}

// renderValues spells a value list as `x = 1, y = 2`.
func renderValues(values []Evaluation) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = v.Name + " = " + runtime.FormatValue(v.Value)
	}
	return strings.Join(parts, ", ")
}
