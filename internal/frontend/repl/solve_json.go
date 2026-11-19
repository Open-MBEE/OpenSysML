package repl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
)

const (
	// solveJSON is the argument asking %optimize or %configure for its JSON document.
	solveJSON      = "-json"
	optimizeUsage  = "usage: %optimize <name> [-json]"
	configureUsage = "usage: %configure <name> [<variation>=<variant>...] [all [<count>]] [-json]"
)

// SolveValue is one value of a solver's answer: the feature or variation point
// it is of and the value, a quantity's magnitude apart from its unit.
type SolveValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	// Unit names the units a quantity's magnitude is in and Dimension its dimension.
	Unit      string `json:"unit,omitempty"`
	Dimension string `json:"dimension,omitempty"`
	// Raw marks a value the notation has no literal for, written as the solver wrote it.
	Raw bool `json:"raw,omitempty"`
}

// SolveOptimum is what a solver answered about one objective of an analysis case.
type SolveOptimum struct {
	Objective  string `json:"objective"`
	Direction  string `json:"direction"`
	Expression string `json:"expression,omitempty"`
	// Status is attained, bounded, unbounded, unverified or undecided.
	Status string `json:"status"`
	// Value is the optimum, Bound a value approached and never attained, and
	// Feasible the value the reported assignment attains.
	Value     string `json:"value,omitempty"`
	Bound     string `json:"bound,omitempty"`
	Feasible  string `json:"feasible,omitempty"`
	Unit      string `json:"unit,omitempty"`
	Dimension string `json:"dimension,omitempty"`
	// Detail says why no optimum was reported.
	Detail string `json:"detail,omitempty"`
}

// SolveDocument is the one JSON document a solving command writes under -json.
type SolveDocument struct {
	// Command is the command answered, without its `%`.
	Command string        `json:"command"`
	Reports []SolveReport `json:"reports"`
}

// solveValues is a solver's assignment as a report carries it, a variable the
// model gives no meaning left out as the printed lines leave it out.
func solveValues(model []solve.Assignment) []SolveValue {
	var out []SolveValue
	for _, a := range model {
		if a.Dead {
			continue
		}
		value := SolveValue{Name: notationName(a.Var.Name), Value: a.Value, Raw: !a.Rendered}
		if a.Rendered {
			value.Value = solve.ValueText(a.Var, a.Value)
			if a.Var.Sort.Kind == solve.SortReal {
				value.Unit, value.Dimension = a.Var.Unit, a.Var.Dimension
				value.Value = solve.Magnitude(value.Value, value.Unit, value.Dimension)
			}
		}
		out = append(out, value)
	}
	return out
}

// oneSelection is the single selection of variants a witness assignment makes.
func oneSelection(model []solve.Assignment, q *solve.Query) [][]SolveValue {
	return [][]SolveValue{solveValues(selectionOf(model, q.Variations()))}
}

// solveOptima is the objectives' answers as a report carries them.
func solveOptima(optima []solve.Optimum) []SolveOptimum {
	out := make([]SolveOptimum, 0, len(optima))
	for _, o := range optima {
		obj := o.Objective
		magnitude := func(rendered string) string { return solve.Magnitude(rendered, obj.Unit, obj.Dimension) }
		out = append(out, SolveOptimum{
			Objective:  obj.Name,
			Direction:  obj.Direction.String(),
			Expression: obj.Expression,
			Status:     o.Status.String(),
			Value:      magnitude(o.Value),
			Bound:      magnitude(o.Bound),
			Feasible:   magnitude(o.Feasible),
			Unit:       obj.Unit,
			Dimension:  obj.Dimension,
			Detail:     o.Detail,
		})
	}
	return out
}

// solveJSONFlag takes -json out of a solving command's arguments, wherever it stands.
func solveJSONFlag(args []string) ([]string, bool) {
	rest := make([]string, 0, len(args))
	asJSON := false
	for _, arg := range args {
		if arg == solveJSON {
			asJSON = true
			continue
		}
		rest = append(rest, arg)
	}
	return rest, asJSON
}

// solveOutput is what a solving command prints: the reports' lines, or under
// -json the one document and no prose.
func solveOutput(command string, reports []SolveReport, asJSON bool) ([]string, bool, error) {
	if !asJSON {
		var out []string
		for _, r := range reports {
			out = append(out, r.Lines...)
		}
		return out, false, nil
	}
	doc := SolveDocument{Command: command, Reports: make([]SolveReport, 0, len(reports))}
	for _, r := range reports {
		// An absent list is written as an empty one, so a reader iterates either.
		if r.Values == nil {
			r.Values = []SolveValue{}
		}
		if r.Lines == nil {
			r.Lines = []string{}
		}
		doc.Reports = append(doc.Reports, r)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, false, fmt.Errorf("%s %s: %w", command, solveJSON, err)
	}
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n"), false, nil
}
