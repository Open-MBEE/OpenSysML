package queryexec

import (
	"encoding/json"
	"fmt"
	"strings"
)

// StepKind is the operation a QVTS outcome points at, one per constructor of
// lean/QVTS/Outcome.lean's Step.
type StepKind string

const (
	StepNamed        StepKind = "Named"
	StepOwned        StepKind = "Owned"
	StepDescendants  StepKind = "Descendants"
	StepFeature      StepKind = "Feature"
	StepValue        StepKind = "Value"
	StepOfType       StepKind = "OfType"
	StepRelated      StepKind = "Related"
	StepWhereFeature StepKind = "WhereFeature"
	StepProject      StepKind = "Project"
)

// Step is one operation of the query with its argument: the name Named looked
// up, the feature Feature read, the column Project filled.
type Step struct {
	Kind     StepKind
	Argument string
}

// Label spells the step as the Lean and SPARQL sides do: `Named(Observatory::x)`.
func (s Step) Label() string {
	if s.Argument == "" {
		return string(s.Kind)
	}
	return fmt.Sprintf("%s(%s)", s.Kind, s.Argument)
}

func (s Step) MarshalJSON() ([]byte, error) { return json.Marshal(s.Label()) }

// Member is one owned member of the namespace the path stopped at, `(name, metaclass)`.
type Member struct {
	Name      string
	Metaclass string
}

func (m Member) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string{m.Name, m.Metaclass})
}

// FeatureRef is one feature reachable on the element the path stopped at,
// `(name, type)`; Type is empty for an untyped feature.
type FeatureRef struct {
	Name string
	Type string
}

func (f FeatureRef) MarshalJSON() ([]byte, error) {
	if f.Type == "" {
		return json.Marshal([]any{f.Name, nil})
	}
	return json.Marshal([]string{f.Name, f.Type})
}

// Frontier is what the model holds where the path stopped resolving: the last
// element that did resolve (At) and what it offers in place of what was asked.
type Frontier struct {
	At       string
	Members  []Member
	Features []FeatureRef
	Kinds    []string
	Nearest  string
}

func (f Frontier) MarshalJSON() ([]byte, error) {
	out := map[string]any{"at": nullable(f.At), "nearest": nullable(f.Nearest)}
	if f.Members != nil {
		out["members"] = f.Members
	}
	if f.Features != nil {
		out["features"] = f.Features
	}
	if f.Kinds != nil {
		out["kinds"] = f.Kinds
	}
	return json.Marshal(out)
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Failure is one failure of one step on one element: what the step asserted
// (Expected) and what the model holds there (Frontier).
type Failure struct {
	Step     Step     `json:"step"`
	Expected string   `json:"expected"`
	Frontier Frontier `json:"frontier"`
}

// PathError says the query is wrong about this model: every failure with its own step.
type PathError struct {
	Failures []Failure `json:"failures"`
}

// Constraint is what a model error found violated, one per constructor of the Lean Constraint.
type Constraint string

const (
	ConstraintAmbiguousName Constraint = "ambiguousName"
	ConstraintMultiplicity  Constraint = "multiplicity"
	ConstraintTyping        Constraint = "typing"
	ConstraintUnit          Constraint = "unit"
)

// Finding is one finding about Element at Step: a constraint the model violates.
type Finding struct {
	Step       Step       `json:"step"`
	Element    string     `json:"element"`
	Constraint Constraint `json:"constraint"`
	Observed   string     `json:"observed"`
}

// ModelError says the model violates a constraint a legitimate step relied on,
// with the path failures of the elements that failed in the same fan-out.
type ModelError struct {
	Findings     []Finding `json:"findings"`
	PathFailures []Failure `json:"pathFailures"`
}

// Outcome is the QVTS record behind a failed query: exactly one of Path and
// Model is set, as Outcome.pathError / Outcome.modelError are in Lean.
type Outcome struct {
	Path  *PathError
	Model *ModelError
}

// Kind is `pathError` or `modelError`.
func (o Outcome) Kind() string {
	if o.Model != nil {
		return "modelError"
	}
	return "pathError"
}

// Step is the step the outcome is reported at: that of its first failure or finding.
func (o Outcome) Step() Step {
	switch {
	case o.Model != nil && len(o.Model.Findings) > 0:
		return o.Model.Findings[0].Step
	case o.Path != nil && len(o.Path.Failures) > 0:
		return o.Path.Failures[0].Step
	}
	return Step{}
}

// MarshalJSON writes the record in the shape qvts's SPARQL differential records:
// `outcome`, `step`, then the failures or the findings and path failures.
func (o Outcome) MarshalJSON() ([]byte, error) {
	out := map[string]any{"outcome": o.Kind(), "step": o.Step().Label()}
	if o.Model != nil {
		out["findings"] = orEmpty(o.Model.Findings)
		out["pathFailures"] = orEmpty(o.Model.PathFailures)
	} else if o.Path != nil {
		out["failures"] = orEmpty(o.Path.Failures)
	}
	return json.Marshal(out)
}

func orEmpty[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}

// Lines renders the record for a prompt or a CLI, one indented line per
// failure or finding, the frontier on the line after it.
func (o Outcome) Lines() []string {
	var out []string
	if o.Model != nil {
		for _, f := range o.Model.Findings {
			out = append(out, fmt.Sprintf("  model error at %s: %s violates %s: %s", f.Step.Label(), f.Element, f.Constraint, f.Observed))
		}
		for _, f := range o.Model.PathFailures {
			out = append(out, failureLines(f)...)
		}
		return out
	}
	if o.Path != nil {
		for _, f := range o.Path.Failures {
			out = append(out, failureLines(f)...)
		}
	}
	return out
}

func failureLines(f Failure) []string {
	lines := []string{fmt.Sprintf("  path error at %s: expected %s", f.Step.Label(), f.Expected)}
	var parts []string
	if f.Frontier.At != "" {
		parts = append(parts, "at "+f.Frontier.At)
	}
	if f.Frontier.Members != nil {
		names := make([]string, 0, len(f.Frontier.Members))
		for _, m := range f.Frontier.Members {
			names = append(names, m.Name+" ("+m.Metaclass+")")
		}
		parts = append(parts, "members: "+bracketed(names))
	}
	if f.Frontier.Features != nil {
		names := make([]string, 0, len(f.Frontier.Features))
		for _, r := range f.Frontier.Features {
			if r.Type != "" {
				names = append(names, r.Name+" : "+r.Type)
			} else {
				names = append(names, r.Name)
			}
		}
		parts = append(parts, "features: "+bracketed(names))
	}
	if f.Frontier.Kinds != nil {
		parts = append(parts, "kinds: "+bracketed(f.Frontier.Kinds))
	}
	if f.Frontier.Nearest != "" {
		parts = append(parts, "nearest: "+f.Frontier.Nearest)
	}
	if len(parts) > 0 {
		lines = append(lines, "    "+strings.Join(parts, "; "))
	}
	return lines
}

func bracketed(xs []string) string { return "[" + strings.Join(xs, ", ") + "]" }

// OutcomeLines is the record behind err rendered for a prompt, none for an
// error that carries no record.
func OutcomeLines(err error) []string {
	var typed *Error
	if !asError(err, &typed) || typed.Outcome == nil {
		return nil
	}
	return typed.Outcome.Lines()
}
