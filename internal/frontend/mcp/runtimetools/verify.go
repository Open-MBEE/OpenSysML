package runtimetools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// VerifyInput asks whether a constraint or requirement holds.
type VerifyInput struct {
	ModelInput
	SymbolID        string                    `json:"symbolId" jsonschema:"qualified name of the constraint or requirement, definition or usage"`
	SubjectSymbolID string                    `json:"subjectSymbolId,omitempty" jsonschema:"qualified name of a part/usage to instantiate and evaluate against, or a path from one to a nested object; empty evaluates declared values"`
	Engine          string                    `json:"engine,omitempty" jsonschema:"the analysis engine asked, as list_engines names them; empty is auto"`
	Question        string                    `json:"question,omitempty" jsonschema:"evaluate (the default: one evaluation), holds (prove over every free assignment) or satisfiable (find one satisfying assignment)"`
	Arguments       []map[string]any          `json:"arguments,omitempty" jsonschema:"positional values for the element's in parameters, each a protojson Value. Example arms: {\"intValue\":\"7\"}, {\"realValue\":1.5}, {\"boolValue\":true}, {\"stringValue\":\"x\"}, {\"quantity\":{\"realMagnitude\":5.0,\"unit\":\"m\"}}, {\"enumLiteral\":{\"literalId\":\"Colors::red\"}}"`
	NamedArguments  map[string]map[string]any `json:"namedArguments,omitempty" jsonschema:"values for the element's in parameters by name, each a protojson Value. Example arms: {\"intValue\":\"7\"}, {\"realValue\":1.5}, {\"boolValue\":true}, {\"stringValue\":\"x\"}, {\"quantity\":{\"realMagnitude\":5.0,\"unit\":\"m\"}}, {\"enumLiteral\":{\"literalId\":\"Colors::red\"}}"`
}

// VerifySatisfactionInput asks whether the satisfy assertions of a model hold.
type VerifySatisfactionInput struct {
	ModelInput
	SymbolID string `json:"symbolId,omitempty" jsonschema:"qualified name limiting the assertions evaluated: those stated within it, or itself when it is a named satisfy; empty evaluates every assertion the model states"`
	Engine   string `json:"engine,omitempty" jsonschema:"the analysis engine asked; empty is auto"`
	Question string `json:"question,omitempty" jsonschema:"evaluate (the default), holds or satisfiable"`
}

// ValidateInstanceInput validates an object of a part as a whole.
type ValidateInstanceInput struct {
	ModelInput
	SymbolID string `json:"symbolId" jsonschema:"qualified name of the part or usage an object of which is validated"`
	Engine   string `json:"engine,omitempty" jsonschema:"the analysis engine asked; empty is auto"`
}

// EvaluateCalcInput invokes a calculation.
type EvaluateCalcInput struct {
	ModelInput
	SymbolID  string           `json:"symbolId" jsonschema:"qualified name of the calc definition or usage"`
	Arguments []map[string]any `json:"arguments,omitempty" jsonschema:"positional arguments, each a protojson Value; a calc usage given none binds its inputs from its own members and answers every output. Example arms: {\"intValue\":\"7\"}, {\"realValue\":1.5}, {\"boolValue\":true}, {\"stringValue\":\"x\"}, {\"quantity\":{\"realMagnitude\":5.0,\"unit\":\"m\"}}, {\"enumLiteral\":{\"literalId\":\"Colors::red\"}}"`
	Engine    string           `json:"engine,omitempty" jsonschema:"the analysis engine asked; empty is auto"`
}

// RunAnalysisInput runs an analysis case.
type RunAnalysisInput struct {
	ModelInput
	SymbolID        string                    `json:"symbolId" jsonschema:"qualified name of the analysis case definition or usage"`
	SubjectSymbolID string                    `json:"subjectSymbolId,omitempty" jsonschema:"qualified name of a part/usage to instantiate as the case's subject, or a path from one; empty leaves the case's own subject binding to supply it"`
	Arguments       []map[string]any          `json:"arguments,omitempty" jsonschema:"positional values for the case's input parameters, each a protojson Value; the subject is never among them. Example arms: {\"intValue\":\"7\"}, {\"realValue\":1.5}, {\"boolValue\":true}, {\"stringValue\":\"x\"}, {\"quantity\":{\"realMagnitude\":5.0,\"unit\":\"m\"}}, {\"enumLiteral\":{\"literalId\":\"Colors::red\"}}"`
	NamedArguments  map[string]map[string]any `json:"namedArguments,omitempty" jsonschema:"values for input parameters by name, each a protojson Value. Example arms: {\"intValue\":\"7\"}, {\"realValue\":1.5}, {\"boolValue\":true}, {\"stringValue\":\"x\"}, {\"quantity\":{\"realMagnitude\":5.0,\"unit\":\"m\"}}, {\"enumLiteral\":{\"literalId\":\"Colors::red\"}}"`
	Schedule        string                    `json:"schedule,omitempty" jsonschema:"scheduling policy for choice points: declared, reverse (the default), seed:<n>, or explore[:runs=<n>,depth=<d>]"`
	Engine          string                    `json:"engine,omitempty" jsonschema:"the analysis engine asked; empty is auto"`
	ToolDryRun      bool                      `json:"toolDryRun,omitempty" jsonschema:"run the case only to the first external tool call its ToolExecution names and answer what the tool would be given in toolDryRun, without starting it (sysml -tool-dry-run); refused under explore"`
}

// RunSweepInput runs a case or calc once per row of a parameter sweep.
type RunSweepInput struct {
	ModelInput
	SymbolID        string                    `json:"symbolId" jsonschema:"qualified name of the analysis case or calc, definition or usage"`
	SubjectSymbolID string                    `json:"subjectSymbolId,omitempty" jsonschema:"qualified name of a part/usage to instantiate as an analysis case's subject, or a path from one"`
	Arguments       []map[string]any          `json:"arguments,omitempty" jsonschema:"positional values every row binds, each a protojson Value. Example arms: {\"intValue\":\"7\"}, {\"realValue\":1.5}, {\"boolValue\":true}, {\"stringValue\":\"x\"}, {\"quantity\":{\"realMagnitude\":5.0,\"unit\":\"m\"}}, {\"enumLiteral\":{\"literalId\":\"Colors::red\"}}"`
	NamedArguments  map[string]map[string]any `json:"namedArguments,omitempty" jsonschema:"values every row binds by name, each a protojson Value. Example arms: {\"intValue\":\"7\"}, {\"realValue\":1.5}, {\"boolValue\":true}, {\"stringValue\":\"x\"}, {\"quantity\":{\"realMagnitude\":5.0,\"unit\":\"m\"}}, {\"enumLiteral\":{\"literalId\":\"Colors::red\"}}"`
	Ranges          []map[string]any          `json:"ranges,omitempty" jsonschema:"the ranges swept, each a protojson SweepRange {\"parameter\":name,\"start\":Value,\"end\":Value,\"step\":Value}; several make one row per point of their cartesian product"`
	Samples         int64                     `json:"samples,omitempty" jsonschema:"rows to draw uniformly from each range instead of stepping through it; 0 steps, above 0 needs seed and refuses a range with a step"`
	Seed            uint64                    `json:"seed,omitempty" jsonschema:"seed the draws are taken from; required with samples"`
	Engine          string                    `json:"engine,omitempty" jsonschema:"the analysis engine asked; empty is auto"`
}

// verdictText spells one verdict as the REPL reports it.
func verdictText(v *pb.Verdict) string {
	if v == nil {
		return ""
	}
	var b strings.Builder
	name := v.GetElement()
	if name == "" {
		name = v.GetElementId()
	}
	switch {
	case strings.HasPrefix(name, v.GetKind()+" "):
		b.WriteString(name)
	case name != "":
		b.WriteString(v.GetKind() + " " + name)
	default:
		b.WriteString(v.GetKind())
	}
	if v.GetInstancePath() != "" {
		fmt.Fprintf(&b, " on %s", v.GetInstancePath())
	}
	b.WriteString(": ")
	switch {
	case v.GetStatus() != "":
		b.WriteString(v.GetStatus())
	case v.GetHolds():
		b.WriteString("holds")
	case v.GetError() != "":
		b.WriteString("undecided")
	default:
		b.WriteString("violated")
	}
	if v.GetCondition() != "" {
		fmt.Fprintf(&b, " (%s)", v.GetCondition())
	}
	if v.GetError() != "" {
		fmt.Fprintf(&b, ": %s", v.GetError())
	}
	if v.GetEngine() != "" {
		fmt.Fprintf(&b, " [%s, %s]", v.GetEngine(), v.GetStrength())
	}
	for _, w := range v.GetWitness() {
		fmt.Fprintf(&b, "\n  %s = %s", w.GetFeature(), valueText(w.GetValue()))
		if w.GetExact() != "" {
			fmt.Fprintf(&b, " (%s)", w.GetExact())
		}
	}
	for _, n := range v.GetNotes() {
		fmt.Fprintf(&b, "\n  %s", n.GetText())
	}
	return b.String()
}

func verdictsText(vs []*pb.Verdict) string {
	var b strings.Builder
	for _, v := range vs {
		b.WriteString(verdictText(v))
		b.WriteByte('\n')
	}
	return b.String()
}

func verificationVerdictsText(vs []*pb.VerificationVerdict) string {
	var b strings.Builder
	for _, v := range vs {
		fmt.Fprintf(&b, "verification %s: %s", v.GetCaseId(), v.GetKind())
		if v.GetDetail() != "" {
			fmt.Fprintf(&b, " (%s)", v.GetDetail())
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func outputsText(outputs []*pb.CalcOutput) string {
	var b strings.Builder
	for _, o := range outputs {
		fmt.Fprintf(&b, "%s = %s\n", o.GetName(), valueText(o.GetValue()))
	}
	return b.String()
}

// verify runs one verification RPC over the resolved model, as every
// verification tool does: resolve, build the request, call, summarize.
func (g *group) verify(ctx context.Context, in any, mi ModelInput, req proto.Message, call func() (proto.Message, error), text func() string) (*mcp.CallToolResult, Result, error) {
	m, err := g.resolve(ctx, mi)
	if err != nil {
		return nil, nil, err
	}
	if err := unmarshal(in, m.hash, req); err != nil {
		return nil, nil, err
	}
	resp, err := call()
	if err != nil {
		return nil, nil, toolError(err)
	}
	return answer(resp, m, text())
}

func (g *group) registerVerification(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "verify_constraint",
		Description: "Ask whether a constraint holds (the VerifyConstraint RPC, the REPL's %constraint): against its declared values, or " +
			"against an object of subjectSymbolId built for the call. The answer is a Verdict: `holds`, the violated `condition`, or an " +
			"`error` when nothing was decided, with the engine and strength of evidence. The question holds or satisfiable puts it to a " +
			"solver over the free variables and answers with a witness assignment. The result is the VerifyConstraintResponse as " +
			"protojson writes it.",
		Annotations: toolkit.ReadOnly("Verify constraint"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in VerifyInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.VerifyConstraintRequest{}
		var resp *pb.VerifyConstraintResponse
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			var err error
			resp, err = g.svc.VerifyConstraint(ctx, req)
			return resp, err
		}, func() string {
			return errorText(resp.GetError()) + verdictsText([]*pb.Verdict{resp.GetVerdict()}) + diagnosticsText(resp.GetDiagnostics())
		})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "verify_requirement",
		Description: "Ask whether a requirement is satisfied (the VerifyRequirement RPC, the REPL's %requirement): its require " +
			"constraints evaluated against declared values or against an object of subjectSymbolId. Beside the Verdict the answer carries " +
			"what every verification case verifying the requirement concluded (pass, fail, inconclusive, error). The result is the " +
			"VerifyRequirementResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Verify requirement"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in VerifyInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.VerifyRequirementRequest{}
		var resp *pb.VerifyRequirementResponse
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			var err error
			resp, err = g.svc.VerifyRequirement(ctx, req)
			return resp, err
		}, func() string {
			return errorText(resp.GetError()) + verdictsText([]*pb.Verdict{resp.GetVerdict()}) +
				verificationVerdictsText(resp.GetVerificationVerdicts()) + diagnosticsText(resp.GetDiagnostics())
		})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "verify_satisfaction",
		Description: "Evaluate the satisfy assertions a model states (the VerifySatisfaction RPC, the REPL's %satisfy): each against " +
			"an object of its subject built for the call, answering one Verdict per assertion in declaration order. A model stating no " +
			"assertion answers an empty list, not an error. The result is the VerifySatisfactionResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Verify satisfaction"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in VerifySatisfactionInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.VerifySatisfactionRequest{}
		var resp *pb.VerifySatisfactionResponse
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			var err error
			resp, err = g.svc.VerifySatisfaction(ctx, req)
			return resp, err
		}, func() string {
			text := errorText(resp.GetError()) + verdictsText(resp.GetVerdicts()) +
				verificationVerdictsText(resp.GetVerificationVerdicts()) + diagnosticsText(resp.GetDiagnostics())
			if text == "" {
				text = "no satisfaction assertions\n"
			}
			return text
		})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "validate_instance",
		Description: "Validate an object of a part as a whole (the ValidateInstance RPC, the REPL's %validate): every assertion about " +
			"an object of symbolId and about the objects it holds — asserted constraints of their types, the requirements they carry, the " +
			"satisfactions they are the subject of — each answered on the concrete object, with a `summary` verdict about the object as a " +
			"whole. The result is the ValidateInstanceResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Validate instance"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ValidateInstanceInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.ValidateInstanceRequest{}
		var resp *pb.ValidateInstanceResponse
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			var err error
			resp, err = g.svc.ValidateInstance(ctx, req)
			return resp, err
		}, func() string {
			var b strings.Builder
			b.WriteString(errorText(resp.GetError()))
			b.WriteString(verdictsText(resp.GetVerdicts()))
			if resp.GetSummary() != nil {
				b.WriteString("summary: " + verdictText(resp.GetSummary()) + "\n")
			}
			if resp.GetBounded() {
				b.WriteString("bounded: nesting past the validation's reach was left unvalidated\n")
			}
			b.WriteString(verificationVerdictsText(resp.GetVerificationVerdicts()))
			b.WriteString(diagnosticsText(resp.GetDiagnostics()))
			return b.String()
		})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "evaluate_calc",
		Description: "Invoke a calculation (the EvaluateCalc RPC, the REPL's %calc): a calc definition with positional arguments " +
			"answers its `result`; a calc usage named with no arguments binds its inputs from its own members and answers every output " +
			"feature in `outputs`. The result is the EvaluateCalcResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Evaluate calc"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in EvaluateCalcInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.EvaluateCalcRequest{}
		var resp *pb.EvaluateCalcResponse
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			var err error
			resp, err = g.svc.EvaluateCalc(ctx, req)
			return resp, err
		}, func() string {
			text := errorText(resp.GetError())
			if resp.GetResult() != nil {
				text += valueText(resp.GetResult()) + "\n"
			}
			return text + outputsText(resp.GetOutputs()) + diagnosticsText(resp.GetDiagnostics())
		})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "run_analysis",
		Description: "Run an analysis case (the RunAnalysis RPC, the REPL's %analysis): its subject and inputs bound from the " +
			"arguments and its own declarations, its body run, every declared output answered with the verdict of its objective and of " +
			"each assertion in its body; a verification case also answers its own verdict. A trade study's evaluations of each " +
			"alternative are in `evaluations`. Under an explore schedule the answer is the set of distinct `outcomes`. Bounded by the " +
			"service's OPENSYSML_MAX_* budgets. The result is the RunAnalysisResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Run analysis"),
	}, func(ctx context.Context, call *mcp.CallToolRequest, in RunAnalysisInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.RunAnalysisRequest{}
		var resp *pb.RunAnalysisResponse
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			err := withProgress(ctx, call, "run_analysis "+in.SymbolID, func() (err error) {
				resp, err = g.svc.RunAnalysis(ctx, req)
				return err
			})
			return resp, err
		}, func() string {
			var b strings.Builder
			b.WriteString(errorText(resp.GetError()))
			b.WriteString(outputsText(resp.GetOutputs()))
			b.WriteString(verdictsText(resp.GetVerdicts()))
			b.WriteString(verificationVerdictsText(resp.GetVerificationVerdicts()))
			for _, e := range resp.GetEvaluations() {
				fmt.Fprintf(&b, "%s(%s) = ", e.GetFunctionId(), valuesText(e.GetArguments()))
				if e.GetError() != "" {
					b.WriteString("error: " + e.GetError())
				} else {
					b.WriteString(valueText(e.GetResult()))
				}
				if e.GetSelected() {
					b.WriteString(" (selected)")
				}
				b.WriteByte('\n')
			}
			b.WriteString(explorationText(resp.GetOutcomes(), resp.GetExploration()))
			b.WriteString(diagnosticsText(resp.GetDiagnostics()))
			return b.String()
		})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "run_sweep",
		Description: "Run an analysis case or calc once per row of a parameter sweep (the RunSweep RPC, the CLI's -sweep): each row " +
			"an ordinary run with the swept parameters bound to that row's values, stepped through the ranges' cartesian product or " +
			"drawn with samples and seed. A failed row is a row with `error`, not a failed table. Bounded by the service's " +
			"OPENSYSML_MAX_* budgets. The result is the RunSweepResponse as protojson writes it; the text is the table.",
		Annotations: toolkit.ReadOnly("Run sweep"),
	}, func(ctx context.Context, call *mcp.CallToolRequest, in RunSweepInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.RunSweepRequest{}
		var resp *pb.RunSweepResponse
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			err := withProgress(ctx, call, "run_sweep "+in.SymbolID, func() (err error) {
				resp, err = g.svc.RunSweep(ctx, req)
				return err
			})
			return resp, err
		}, func() string {
			var b strings.Builder
			b.WriteString(errorText(resp.GetError()))
			if len(resp.GetRows()) > 0 {
				fmt.Fprintf(&b, "%d rows over %s\n", len(resp.GetRows()), strings.Join(resp.GetParameters(), ", "))
			}
			b.WriteString(sweepRowsText(resp.GetRows()))
			b.WriteString(diagnosticsText(resp.GetDiagnostics()))
			return b.String()
		})
	})
}

// sweepRowsText spells a table's rows one per line: what each bound, what it
// produced and decided, and the error that stopped it where it failed.
func sweepRowsText(rows []*pb.SweepRow) string {
	var b strings.Builder
	for _, row := range rows {
		cells := make([]string, 0, len(row.GetInputs())+len(row.GetOutputs()))
		for _, in := range row.GetInputs() {
			cells = append(cells, fmt.Sprintf("%s=%s", in.GetName(), valueText(in.GetValue())))
		}
		cells = append(cells, "->")
		for _, out := range row.GetOutputs() {
			cells = append(cells, fmt.Sprintf("%s=%s", out.GetName(), valueText(out.GetValue())))
		}
		for _, v := range row.GetVerdicts() {
			cells = append(cells, verdictText(v))
		}
		if row.GetError() != "" {
			cells = append(cells, "error: "+row.GetError())
		}
		b.WriteString(strings.Join(cells, " ") + "\n")
	}
	return b.String()
}
