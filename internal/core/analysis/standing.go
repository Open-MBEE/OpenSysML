package analysis

import (
	"fmt"
	"strings"
)

// Standing is the line a report prints after a verdict: the claim, the strength it earned,
// and what earned it — `holds (observed: 1 run under reverse)`, `not covered (<reason>)`.
func (r Result) Standing() string {
	if !r.Covered() {
		var parts []string
		if r.Reason != "" {
			parts = append(parts, r.Reason)
		}
		for _, b := range r.Bounds {
			if b.Reached {
				parts = append(parts, b.String())
			}
		}
		if len(parts) == 0 {
			return NotCovered.String()
		}
		return fmt.Sprintf("%s (%s)", NotCovered, strings.Join(parts, "; "))
	}
	strength := r.Strength.String()
	if r.Claim.Universal() && r.Strength >= Bounded && r.Question.Free != FreeNothing {
		strength += " over " + over(r.Question.Free)
	}
	return fmt.Sprintf("%s (%s: %s)", r.Claim, strength, r.evidence())
}

// evidence is what earned the strength: the executions made, the witness, and every bound reached.
func (r Result) evidence() string {
	var parts []string
	switch r.Question.Kind {
	case Evaluate:
		parts = append(parts, "1 run under "+r.Question.Schedule.String())
	case Outcomes:
		if x := r.Exploration(); x != nil {
			parts = append(parts, fmt.Sprintf("%s, inputs as written", plural(x.Runs, "linearization")))
		}
	case Sweep:
		parts = append(parts, plural(len(r.Values), "row"))
	case Satisfiable:
		parts = append(parts, fmt.Sprintf("%s by %s", plural(len(r.Values), "query"), r.Engine))
	}
	if r.Witness != nil && len(r.Witness.Choices) > 0 {
		parts = append(parts, fmt.Sprintf("witness of %s replayed", plural(len(r.Witness.Choices), "choice")))
	}
	for _, b := range r.Bounds {
		if b.Reached {
			parts = append(parts, b.String())
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "by "+r.Engine)
	}
	return strings.Join(parts, ", ")
}

// Standing is the plan's result's standing, followed by the plan when it consulted more than
// one engine or was made under all: `; all: run holds (observed), solve refused (…)`.
func (p Plan) Standing() string {
	standing := p.Result.Standing()
	if len(p.Steps) == 0 || (p.Selection.Mode != SelectAll && len(p.Steps) == 1) {
		return standing
	}
	parts := make([]string, len(p.Steps))
	for i, step := range p.Steps {
		parts[i] = step.Standing()
	}
	return fmt.Sprintf("%s; %s: %s", standing, p.Selection, strings.Join(parts, ", "))
}

// Standing is one step as the plan's standing lists it: what the engine answered, refused
// with, faulted on, or was cancelled at.
func (s Step) Standing() string {
	switch {
	case s.Refusal != nil:
		return fmt.Sprintf("%s refused (%s)", s.Engine, s.Refusal)
	case s.Cancelled:
		if len(s.Bounds) == 0 {
			return s.Engine + " cancelled"
		}
		return fmt.Sprintf("%s cancelled at %s", s.Engine, s.Bounds)
	case s.Err != nil:
		return fmt.Sprintf("%s failed (%s)", s.Engine, s.Err)
	case s.Result == nil:
		return s.Engine
	case !s.Result.Covered():
		return fmt.Sprintf("%s %s", s.Engine, s.Result.Standing())
	}
	return fmt.Sprintf("%s %s (%s)", s.Engine, s.Result.Claim, s.Result.Strength)
}

// over spells what a universal claim ranges over.
func over(free Freedom) string {
	var parts []string
	if free.Has(FreeSchedule) {
		parts = append(parts, "schedules")
	}
	if free.Has(FreeInputs) {
		parts = append(parts, "inputs")
	}
	return strings.Join(parts, " and ")
}

// plural counts a noun: `1 run`, `6 runs`, `2 queries`.
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	if strings.HasSuffix(noun, "y") {
		return fmt.Sprintf("%d %sies", n, strings.TrimSuffix(noun, "y"))
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
