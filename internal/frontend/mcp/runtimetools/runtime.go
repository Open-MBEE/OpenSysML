package runtimetools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// EvaluateInput is an expression to evaluate against a model.
type EvaluateInput struct {
	ModelInput
	Expression      string `json:"expression" jsonschema:"the SysML expression, as 2 + 2 or Demo::sedan.mass"`
	ContextSymbolID string `json:"contextSymbolId,omitempty" jsonschema:"qualified name of the namespace the expression's names resolve in; empty is the model root"`
	SubjectSymbolID string `json:"subjectSymbolId,omitempty" jsonschema:"qualified name of a part/usage to instantiate and evaluate against, so a feature reads that object's value rather than its declared default"`
}

// InstantiateInput names the part or usage to build an object of.
type InstantiateInput struct {
	ModelInput
	SymbolID string `json:"symbolId" jsonschema:"qualified name of the part definition or usage to instantiate"`
}

// ExecuteActionInput runs an action.
type ExecuteActionInput struct {
	ModelInput
	ActionSymbolID    string                    `json:"actionSymbolId" jsonschema:"qualified name of the action definition or usage to run"`
	Inputs            map[string]map[string]any `json:"inputs,omitempty" jsonschema:"input parameter name to value; each value is a protojson Value. Example arms: {\"intValue\":\"7\"}, {\"realValue\":1.5}, {\"boolValue\":true}, {\"stringValue\":\"x\"}, {\"quantity\":{\"realMagnitude\":5.0,\"unit\":\"m\"}}, {\"enumLiteral\":{\"literalId\":\"Colors::red\"}}"`
	Schedule          string                    `json:"schedule,omitempty" jsonschema:"scheduling policy for choice points: declared, reverse (the default), seed:<n>, or explore[:runs=<n>,depth=<d>] which runs every linearization and answers with outcomes"`
	PerformerSymbolID string                    `json:"performerSymbolId,omitempty" jsonschema:"qualified name of a part/usage, or a path from one as Mission::mission.vehicle, whose object the action is performed on; empty performs it outside any object"`
	Advance           *float64                  `json:"advance,omitempty" jsonschema:"simulated time in seconds to run for (sysml -advance): the behavior is started and its clock moved by this much; an action not completed by then reports that in error. Unset runs to completion"`
	Seed              *uint64                   `json:"seed,omitempty" jsonschema:"seed the model's own draws (weighted decisions, random functions) are taken from (sysml -seed); the same seed draws the same run"`
	Draws             string                    `json:"draws,omitempty" jsonschema:"how the run resolves RandomFunctions draws (sysml -draws): random (default, from seed), min, max or average"`
	ClockStep         float64                   `json:"clockStep,omitempty" jsonschema:"the step in seconds the run's clock ticks by (sysml -clock-step); 0 is a continuous clock"`
}

// ExecuteStateInput runs a state machine.
type ExecuteStateInput struct {
	ModelInput
	StateMachineSymbolID string   `json:"stateMachineSymbolId" jsonschema:"qualified name of the state definition or usage to run"`
	Events               []string `json:"events,omitempty" jsonschema:"event names to send, in order"`
	Schedule             string   `json:"schedule,omitempty" jsonschema:"scheduling policy for choice points: declared, reverse (the default), seed:<n>, or explore[:runs=<n>,depth=<d>]"`
	PerformerSymbolID    string   `json:"performerSymbolId,omitempty" jsonschema:"qualified name of a part/usage, or a path from one, whose object exhibits the machine; empty runs it outside any object"`
	Trace                bool     `json:"trace,omitempty" jsonschema:"also answer the run's typed trace (entries, exits, transitions, accepts), up to a failure; unavailable under explore"`
	Advance              *float64 `json:"advance,omitempty" jsonschema:"simulated time in seconds to run for (sysml -advance): the behavior is started and its clock moved by this much; an action not completed by then reports that in error. Unset runs to completion"`
	Seed                 *uint64  `json:"seed,omitempty" jsonschema:"seed the model's own draws (weighted decisions, random functions) are taken from (sysml -seed); the same seed draws the same run"`
	Draws                string   `json:"draws,omitempty" jsonschema:"how the run resolves RandomFunctions draws (sysml -draws): random (default, from seed), min, max or average"`
	ClockStep            float64  `json:"clockStep,omitempty" jsonschema:"the step in seconds the run's clock ticks by (sysml -clock-step); 0 is a continuous clock"`
}

func (g *group) registerRuntime(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "evaluate",
		Description: "Evaluate a SysML expression against a parsed model (the Evaluate RPC, the REPL's %eval): arithmetic, feature " +
			"reads, quantities with units, collection operators. The result is the EvaluateResponse as protojson writes it: `result` is a " +
			"Value (64-bit integers are JSON strings, as the proto3 JSON mapping writes them), `error` is set when the expression could " +
			"not be evaluated. The text spells the value as the REPL prints it.",
		Annotations: toolkit.ReadOnly("Evaluate expression"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in EvaluateInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		req := &pb.EvaluateRequest{}
		if err := unmarshal(in, m.hash, req); err != nil {
			return nil, nil, err
		}
		resp, err := g.svc.Evaluate(ctx, req)
		if err != nil {
			return nil, nil, toolError(err)
		}
		text := errorText(resp.GetError())
		if resp.GetError() == "" {
			text = valueText(resp.GetResult()) + "\n"
		}
		return answer(resp, m, text+diagnosticsText(resp.GetDiagnostics()))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "instantiate",
		Description: "Build an object of a part definition or usage and report its feature values (the Instantiate RPC, the REPL's " +
			"%instantiate): every attribute's value or default, nested parts as instance ids, with every reachable instance in " +
			"`instances`. The service keeps the object for the model: run_document_query on the same modelHash can bind it. The result " +
			"is the InstantiateResponse as protojson writes it.",
		Annotations: instantiateAnnotations(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in InstantiateInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		resp, err := g.svc.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: m.hash, SymbolId: in.SymbolID})
		if err != nil {
			return nil, nil, toolError(err)
		}
		var b strings.Builder
		b.WriteString(errorText(resp.GetError()))
		if inst := resp.GetInstance(); inst != nil {
			b.WriteString(instanceText(inst))
			if n := len(resp.GetInstances()); n > 1 {
				fmt.Fprintf(&b, "%d instances reachable\n", n)
			}
		}
		b.WriteString(diagnosticsText(resp.GetDiagnostics()))
		return answer(resp, m, b.String())
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "execute_action",
		Description: "Run an action definition or usage to completion and report its output parameters (the ExecuteAction RPC, the " +
			"REPL's %run). Inputs are bound by parameter name. Under an explore schedule every linearization the model admits is run " +
			"and the answer is the set of distinct `outcomes` with how the exploration ended, instead of one run's outputs. The run is " +
			"bounded by the service's OPENSYSML_MAX_* budgets; a budget hit is reported in `error`. `advance`, `seed`, `draws` and " +
			"`clockStep` are the run options of `sysml -advance`, `-seed`, `-draws` and `-clock-step`. The result is the " +
			"ExecuteActionResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Execute action"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ExecuteActionInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		rpc := &pb.ExecuteActionRequest{}
		if err := unmarshal(in, m.hash, rpc); err != nil {
			return nil, nil, err
		}
		var resp *pb.ExecuteActionResponse
		err = withProgress(ctx, req, "execute_action "+in.ActionSymbolID, func() (err error) {
			resp, err = g.svc.ExecuteAction(ctx, rpc)
			return err
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		var b strings.Builder
		b.WriteString(errorText(resp.GetError()))
		if len(resp.GetOutputs()) > 0 {
			b.WriteString(markedValuesText(resp.GetOutputs(), resp.GetOrderDependent()))
		} else if resp.GetError() == "" && resp.GetExploration() == nil {
			b.WriteString("completed with no outputs\n")
		}
		if len(resp.GetPerformerAttributes()) > 0 {
			b.WriteString("performer:\n" + namedValuesText(resp.GetPerformerAttributes()))
		}
		if resp.GetFinalTime() != 0 {
			fmt.Fprintf(&b, "final time %g s\n", resp.GetFinalTime())
		}
		b.WriteString(explorationText(resp.GetOutcomes(), resp.GetExploration()))
		b.WriteString(diagnosticsText(resp.GetDiagnostics()))
		return answer(resp, m, b.String())
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "execute_state",
		Description: "Run a state machine over a sequence of events and report the states visited, the final context and the " +
			"simulation clock (the ExecuteState RPC, the REPL's %run on a state). With trace the answer also carries every record the " +
			"run made (entry, exit, transition, accept, send, choice), up to a failure. Under an explore schedule every linearization " +
			"is run and the answer is the set of distinct `outcomes`. With `advance` the events are sent and the clock then moved by that " +
			"much, as `sysml -state X -advance T` does; without it the machine runs to quiescence. The run is bounded by the service's " +
			"OPENSYSML_MAX_* budgets. The result is the ExecuteStateResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Execute state machine"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ExecuteStateInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		rpc := &pb.ExecuteStateRequest{}
		if err := unmarshal(in, m.hash, rpc); err != nil {
			return nil, nil, err
		}
		var resp *pb.ExecuteStateResponse
		err = withProgress(ctx, req, "execute_state "+in.StateMachineSymbolID, func() (err error) {
			resp, err = g.svc.ExecuteState(ctx, rpc)
			return err
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		var b strings.Builder
		b.WriteString(errorText(resp.GetError()))
		if len(resp.GetStatesVisited()) > 0 {
			fmt.Fprintf(&b, "states: %s\n", strings.Join(resp.GetStatesVisited(), " -> "))
		}
		if len(resp.GetFinalContext()) > 0 {
			b.WriteString("final context:\n" + markedValuesText(resp.GetFinalContext(), resp.GetOrderDependent()))
		}
		if resp.GetFinalTime() != 0 {
			fmt.Fprintf(&b, "final time %g s\n", resp.GetFinalTime())
		}
		if len(resp.GetTrace()) > 0 {
			b.WriteString("trace:\n")
			for _, ev := range resp.GetTrace() {
				fmt.Fprintf(&b, "  %s\n", ev.GetText())
			}
			if resp.GetTraceDropped() > 0 {
				fmt.Fprintf(&b, "  (%d oldest records dropped)\n", resp.GetTraceDropped())
			}
		}
		b.WriteString(explorationText(resp.GetOutcomes(), resp.GetExploration()))
		b.WriteString(diagnosticsText(resp.GetDiagnostics()))
		return answer(resp, m, b.String())
	})
}

// instanceText spells an object as the REPL shows one: its id and type, then
// each feature's value in name order.
func instanceText(inst *pb.Instance) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d : %s\n", inst.GetId(), inst.GetTypeSymbolId())
	names := make([]string, 0, len(inst.GetFeatureValues()))
	for name := range inst.GetFeatureValues() {
		names = append(names, name)
	}
	sortStrings(names)
	for _, name := range names {
		fv := inst.GetFeatureValues()[name]
		switch {
		case fv.GetError() != "":
			fmt.Fprintf(&b, "  %s: error: %s\n", name, fv.GetError())
		case len(fv.GetValues()) > 0:
			fmt.Fprintf(&b, "  %s = (%s)\n", name, valuesText(fv.GetValues()))
		case fv.GetValue() != nil:
			fmt.Fprintf(&b, "  %s = %s\n", name, valueText(fv.GetValue()))
		default:
			fmt.Fprintf(&b, "  %s\n", name)
		}
	}
	return b.String()
}

// explorationText summarizes an explore schedule's outcomes.
func explorationText(outcomes []*pb.Outcome, status *pb.ExplorationStatus) string {
	if status == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "exploration: %d runs, %d outcomes", status.GetRuns(), len(outcomes))
	if status.GetComplete() {
		b.WriteString(", complete")
	} else {
		fmt.Fprintf(&b, ", incomplete (budgets hit: %s)", strings.Join(status.GetBudgetsHit(), ", "))
	}
	if status.GetFailedLinearizations() > 0 {
		fmt.Fprintf(&b, ", %d failed", status.GetFailedLinearizations())
	}
	b.WriteByte('\n')
	for i, o := range outcomes {
		fmt.Fprintf(&b, "outcome %d (%d linearizations", i+1, o.GetLinearizations())
		if o.GetProbabilityRange() != nil {
			fmt.Fprintf(&b, ", probability %g..%g", o.GetProbabilityRange().GetMin(), o.GetProbabilityRange().GetMax())
		}
		b.WriteString("):")
		if o.GetError() != "" {
			fmt.Fprintf(&b, " error: %s", o.GetError())
		}
		if o.GetFinalState() != "" {
			fmt.Fprintf(&b, " final state %s", o.GetFinalState())
		}
		b.WriteByte('\n')
		for _, line := range strings.Split(strings.TrimRight(namedValuesText(o.GetOutputs()), "\n"), "\n") {
			if line != "" {
				fmt.Fprintf(&b, "  %s\n", line)
			}
		}
	}
	return b.String()
}

// instantiateAnnotations says what instantiate does to the server: it adds an
// object the model's later document queries can reach, so it is not read-only,
// but it destroys nothing and every call adds one more object.
func instantiateAnnotations() *mcp.ToolAnnotations {
	closed, destructive := false, false
	return &mcp.ToolAnnotations{Title: "Instantiate", DestructiveHint: &destructive, OpenWorldHint: &closed}
}
