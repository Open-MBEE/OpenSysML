package sessiontools

import (
	"encoding/json"
	"fmt"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// valueJSON is an evaluated value in the runtime API's JSON form, decoded so it
// is structured content rather than a string holding JSON.
func valueJSON(ev repl.EvalResult) (any, error) {
	pv := protoconv.ValueToProtoIn(ev.Context, ev.Value, ev.Index)
	if ev.Unset {
		pv = &pb.Value{Kind: &pb.Value_Unset{Unset: true}}
	}
	raw, err := protojson.Marshal(pv)
	if err != nil {
		return nil, fmt.Errorf("encoding the value: %w", err)
	}
	return decoded(raw)
}

// decoded is JSON text as structured content.
func decoded(raw []byte) (any, error) {
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}
	return out, nil
}

// NamedValue is one value a run or check produced, as the prompt prints it.
type NamedValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func namedValues(vs []repl.NamedValue) []NamedValue {
	out := make([]NamedValue, 0, len(vs))
	for _, v := range vs {
		out = append(out, NamedValue{Name: v.Name, Value: v.Value})
	}
	return out
}

// Evaluation is one application a case made of its own calc as a function value.
type Evaluation struct {
	Function  string   `json:"function"`
	Arguments []string `json:"arguments"`
	Result    string   `json:"result,omitempty"`
	Error     string   `json:"error,omitempty"`
	Selected  bool     `json:"selected,omitempty" jsonschema:"the case returned this argument"`
	Tied      bool     `json:"tied,omitempty" jsonschema:"passed over for one evaluating alike"`
}

func evaluations(es []repl.Evaluation) []Evaluation {
	out := make([]Evaluation, 0, len(es))
	for _, e := range es {
		out = append(out, Evaluation{Function: e.Function, Arguments: orEmpty(e.Arguments), Result: e.Result, Error: e.Error, Selected: e.Selected, Tied: e.Tied})
	}
	return out
}

// Verification is the verdict a verification case's body answered about the
// checked requirement.
type Verification struct {
	Case    string `json:"case"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail,omitempty"`
	Subcase bool   `json:"subcase,omitempty"`
}

// Row is one run of a sweep or of repeated runs.
type Row struct {
	Inputs      []NamedValue `json:"inputs"`
	Outputs     []NamedValue `json:"outputs"`
	Verdicts    []NamedValue `json:"verdicts"`
	Evaluations []Evaluation `json:"evaluations,omitempty"`
	Millis      float64      `json:"millis" jsonschema:"how long the run took"`
	Error       string       `json:"error,omitempty" jsonschema:"what stopped the run when it failed"`
}

// Verdict is what a run, check or sweep answered: its status, the values it
// produced, and the prompt's report of it.
type Verdict struct {
	Subject       string         `json:"subject" jsonschema:"what was run or checked, as named"`
	Status        string         `json:"status" jsonschema:"holds, fails or unresolved; unresolved is a question the model did not answer (an unknown name, a run that could not be made)"`
	Values        []NamedValue   `json:"values" jsonschema:"what the run produced: a calc's result, an action's outputs, a case's outputs and verdicts"`
	Verifications []Verification `json:"verifications,omitempty"`
	Rows          []Row          `json:"rows,omitempty" jsonschema:"one per run of a sweep or of repeated runs"`
	Evaluations   []Evaluation   `json:"evaluations,omitempty"`
	Text          string         `json:"text" jsonschema:"the prompt's report"`
}

func verdict(v repl.Verdict) Verdict {
	out := Verdict{
		Subject:     v.Subject,
		Status:      v.Status.String(),
		Values:      namedValues(v.Values),
		Evaluations: evaluations(v.Evaluations),
		Text:        text(v.Lines),
	}
	for _, ver := range v.Verifications {
		out.Verifications = append(out.Verifications, Verification{Case: ver.Case, Kind: ver.Kind, Detail: ver.Detail, Subcase: ver.Subcase})
	}
	for _, r := range v.Rows {
		out.Rows = append(out.Rows, Row{
			Inputs:      namedValues(r.Inputs),
			Outputs:     namedValues(r.Outputs),
			Verdicts:    namedValues(r.Verdicts),
			Evaluations: evaluations(r.Evaluations),
			Millis:      r.Millis,
			Error:       r.Error,
		})
	}
	return out
}

// SolveReport is what a solver answered about one element.
type SolveReport struct {
	Subject     string `json:"subject"`
	Status      string `json:"status" jsonschema:"sat, unsat, unknown, unbounded, no-optimum or unavailable (no solver installed, or the element outside the solvable subset)"`
	Satisfiable bool   `json:"satisfiable"`
	Solver      string `json:"solver,omitempty" jsonschema:"the solver that answered"`
	Text        string `json:"text" jsonschema:"the prompt's report: the assignment, the conflicting conditions, the optimum"`
}

// SolveResult is a solver tool's answer: one report per element asked about.
type SolveResult struct {
	Reports []SolveReport `json:"reports"`
	Text    string        `json:"text"`
}

func solveResult(reports []repl.SolveReport) SolveResult {
	out := SolveResult{Reports: make([]SolveReport, 0, len(reports))}
	var lines []string
	for _, r := range reports {
		out.Reports = append(out.Reports, SolveReport{Subject: r.Subject, Status: r.Status.String(), Satisfiable: r.Satisfiable(), Solver: r.Solver, Text: text(r.Lines)})
		lines = append(lines, r.Lines...)
	}
	out.Text = text(lines)
	return out
}
