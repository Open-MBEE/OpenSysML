package opensysml_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// tradeStudySource lists three engines, two of them of one mass, so a
// minimizing study selects one and ties the other; perCylinder divides by a
// zero cylinder count for one alternative, so its run fails after scoring
// the other.
const tradeStudySource = `package Trade {
	private import ScalarValues::*;
	private import TradeStudies::*;

	part def Engine { attribute mass : Real; attribute cylinders : Integer; }
	part a : Engine { attribute :>> mass = 30.0; attribute :>> cylinders = 6; }
	part b : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 4; }
	part c : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 0; }

	analysis lightest : TradeStudy {
		subject : Engine[1..*] = (a, b, c);
		objective : MinimizeObjective;
		calc :>> evaluationFunction {
			in part e :>> alternative : Engine;
			return :>> result : Real = e.mass;
		}
		return part :>> selectedAlternative : Engine;
	}

	analysis perCylinder : TradeStudy {
		subject : Engine[1..*] = (a, c);
		objective : MinimizeObjective;
		calc :>> evaluationFunction {
			in part e :>> alternative : Engine;
			return :>> result : Real = e.mass / e.cylinders;
		}
		return part :>> selectedAlternative : Engine;
	}
}
`

// analysisCallers are the two implementations of Client, each answering the
// same model: the engine in this process, and the Connect transport.
func analysisCallers(t *testing.T) map[string]func(*testing.T) opensysml.Client {
	t.Helper()
	return map[string]func(*testing.T) opensysml.Client{
		"inprocess": newClient,
		"remote":    func(t *testing.T) opensysml.Client { return dialClient(t, startService(t)) },
	}
}

// evaluationLines renders evaluations as "type=result[selected][tied]", the
// alternative named by the type of the object the analysis resolves it to.
func evaluationLines(analysis *opensysml.Analysis) []string {
	out := make([]string, 0, len(analysis.Evaluations))
	for _, evaluation := range analysis.Evaluations {
		line := evaluation.FunctionID + "("
		for i, argument := range evaluation.Arguments {
			if i > 0 {
				line += ","
			}
			if id, ok := argument.(opensysml.InstanceID); ok {
				if instance := analysis.Instance(id); instance != nil {
					line += instance.TypeSymbolID
					continue
				}
			}
			line += fmt.Sprintf("%#v", argument)
		}
		line += ")="
		if evaluation.Error != "" {
			line += "error:" + evaluation.Error
		} else {
			line += fmt.Sprintf("%v", evaluation.Result)
		}
		if evaluation.Selected {
			line += "[selected]"
		}
		if evaluation.Tied {
			line += "[tied]"
		}
		out = append(out, line)
	}
	return out
}

func TestRunAnalysisReportsEachAlternativesEvaluation(t *testing.T) {
	for name, connect := range analysisCallers(t) {
		t.Run(name, func(t *testing.T) {
			client := connect(t)
			model := parse(t, client, tradeStudySource)

			analysis, err := client.RunAnalysis(context.Background(), model, "Trade::lightest")
			if err != nil {
				t.Fatalf("RunAnalysis: %v", err)
			}
			want := []string{
				"Trade::lightest::evaluationFunction(Trade::a)=30",
				"Trade::lightest::evaluationFunction(Trade::b)=10[selected]",
				"Trade::lightest::evaluationFunction(Trade::c)=10[tied]",
			}
			if got := evaluationLines(analysis); !slices.Equal(got, want) {
				t.Errorf("evaluations =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			selected := analysis.Selected()
			if len(selected) != 1 {
				t.Fatalf("selected = %v, want the one evaluation of b", selected)
			}
			picked, ok := analysis.Output("selectedAlternative")
			if !ok {
				t.Fatal("no selectedAlternative output")
			}
			if picked != selected[0].Arguments[0] {
				t.Errorf("selectedAlternative = %#v, want the selected evaluation's argument %#v",
					picked, selected[0].Arguments[0])
			}
			if instance := analysis.Instance(picked.(opensysml.InstanceID)); instance == nil ||
				instance.TypeSymbolID != "Trade::b" {
				t.Errorf("selectedAlternative resolves to %+v, want Trade::b", instance)
			}
			if !analysis.Holds() {
				t.Errorf("verdicts = %+v, want the objective satisfied", analysis.Verdicts)
			}
		})
	}
}

func TestAFailedRunKeepsWhatItComputed(t *testing.T) {
	for name, connect := range analysisCallers(t) {
		t.Run(name, func(t *testing.T) {
			client := connect(t)
			model := parse(t, client, tradeStudySource)

			analysis, err := client.RunAnalysis(context.Background(), model, "Trade::perCylinder")
			if analysis != nil {
				t.Errorf("analysis = %+v, want none beside the error", analysis)
			}
			var failed *opensysml.AnalysisError
			if !errors.As(err, &failed) {
				t.Fatalf("err = %T (%v), want *AnalysisError", err, err)
			}
			if !strings.Contains(failed.Message, "division by zero") {
				t.Errorf("message = %q, want it to name the division by zero", failed.Message)
			}
			if failed.Reason != opensysml.ReasonEvaluation {
				t.Errorf("reason = %v, want %v", failed.Reason, opensysml.ReasonEvaluation)
			}
			var classified *opensysml.VerifyError
			if !errors.As(err, &classified) || !errors.Is(err, opensysml.ErrFailure) {
				t.Error("an AnalysisError is not recovered as a VerifyError matching ErrFailure")
			}
			partial := failed.Partial
			if partial == nil {
				t.Fatal("partial = nil, want the evaluations made before the failure")
			}
			got := evaluationLines(partial)
			if len(got) != 2 || got[0] != "Trade::perCylinder::evaluationFunction(Trade::a)=5" ||
				!strings.HasPrefix(got[1], "Trade::perCylinder::evaluationFunction(Trade::c)=error:") ||
				!strings.Contains(got[1], "division by zero") {
				t.Errorf("partial evaluations = %v, want a scored and c failing", got)
			}
			if partial.Selected() != nil {
				t.Errorf("selected = %v, want none for a run that picked nothing", partial.Selected())
			}
			if len(partial.Verdicts) == 0 {
				t.Fatal("partial verdicts = none, want the objective undecided")
			}
			for _, verdict := range partial.Verdicts {
				if !verdict.Undecided() {
					t.Errorf("verdict %q = %+v, want undecided", verdict.Element, verdict)
				}
			}
			if partial.Holds() {
				t.Error("a partial analysis holds")
			}
		})
	}
}

func TestTheClientNamesTheCaseEvaluationCapability(t *testing.T) {
	client := newClient(t)
	info, err := client.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if !info.Has(opensysml.CapabilityCaseEvaluations) {
		t.Errorf("capabilities = %v, want it to contain %q",
			info.Capabilities, opensysml.CapabilityCaseEvaluations)
	}
}
