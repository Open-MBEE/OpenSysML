package runtimetools

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// RunRunsInput runs an action or a Monte Carlo case repeatedly.
type RunRunsInput struct {
	ModelInput
	ActionSymbolID    string                    `json:"actionSymbolId,omitempty" jsonschema:"qualified name of the action to run; give this or analysisSymbolId"`
	AnalysisSymbolID  string                    `json:"analysisSymbolId,omitempty" jsonschema:"qualified name of the Simulation::MonteCarlo analysis case, definition or usage; give this or actionSymbolId"`
	Inputs            map[string]map[string]any `json:"inputs,omitempty" jsonschema:"the action's input parameter name to value, each a protojson Value. Example arms: {\"intValue\":\"7\"}, {\"realValue\":1.5}, {\"boolValue\":true}, {\"stringValue\":\"x\"}"`
	PerformerSymbolID string                    `json:"performerSymbolId,omitempty" jsonschema:"qualified name of a part/usage, or a path from one, whose object performs the action; empty performs it outside any object"`
	SubjectSymbolID   string                    `json:"subjectSymbolId,omitempty" jsonschema:"qualified name of a part/usage to instantiate as the case's subject, or a path from one to a nested object"`
	Arguments         []map[string]any          `json:"arguments,omitempty" jsonschema:"positional values for the case's in parameters, each a protojson Value"`
	NamedArguments    map[string]map[string]any `json:"namedArguments,omitempty" jsonschema:"values for the case's in parameters by name, each a protojson Value"`
	Runs              int64                     `json:"runs" jsonschema:"how many runs to make (-runs); positive"`
	Seed              *uint64                   `json:"seed,omitempty" jsonschema:"seed the runs draw their modeled randomness from, one derived seed per run; required unless draws is min, max or average"`
	Draws             string                    `json:"draws,omitempty" jsonschema:"draw policy for the model's distributions: random (the default), min, max or average (-draws)"`
	ClockStep         float64                   `json:"clockStep,omitempty" jsonschema:"clock step of each run; 0 keeps the default (-clock-step)"`
	Observe           []string                  `json:"observe,omitempty" jsonschema:"features of the action each run reports, clock for the time it completed at; empty reports every feature and the clock (-observe). Not for a case, which observes what it declares"`
	Engine            string                    `json:"engine,omitempty" jsonschema:"the analysis engine asked, as list_engines names them; empty is auto"`
}

func (g *group) registerRuns(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "run_runs",
		Description: "Run an action, or a Simulation::MonteCarlo analysis case, a number of times (the RunRuns RPC, the CLI's -runs/-observe, " +
			"the REPL's %runs): every run is an ordinary run in a context of its own, drawing its modeled randomness from a seed derived " +
			"from `seed`. `rows` is the table, one row per run with the run number as its input and what it observed as its outputs; " +
			"`distributions` summarises each observable over the completed runs (extremes, mean, deviation, percentiles, histogram, or " +
			"the count of each distinct value); for a case `outputs` and `verdicts` are its conclusion over the sample and `unconcluded` " +
			"why it has none. A run that failed is its row's error. The result is the RunRunsResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Run runs"),
	}, func(ctx context.Context, call *mcp.CallToolRequest, in RunRunsInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.RunRunsRequest{}
		var resp *pb.RunRunsResponse
		subject := in.ActionSymbolID
		if subject == "" {
			subject = in.AnalysisSymbolID
		}
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			err := withProgress(ctx, call, "run_runs "+subject, func() (err error) {
				resp, err = g.svc.RunRuns(ctx, req)
				return err
			})
			return resp, err
		}, func() string {
			var b strings.Builder
			b.WriteString(errorText(resp.GetError()))
			b.WriteString(sweepRowsText(resp.GetRows()))
			for _, line := range resp.GetLines() {
				b.WriteString(line)
				b.WriteByte('\n')
			}
			b.WriteString(diagnosticsText(resp.GetDiagnostics()))
			return b.String()
		})
	})
}
