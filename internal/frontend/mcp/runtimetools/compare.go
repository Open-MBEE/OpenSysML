package runtimetools

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// ObservablePairInput is one observe entry of compare_results.
type ObservablePairInput struct {
	Stored  string `json:"stored" jsonschema:"the observable the tool stored"`
	Feature string `json:"feature,omitempty" jsonschema:"the feature of the run that answers it; empty for the target's feature of the observable's own name"`
}

// CompareResultsInput is the input of compare_results: the CompareResultsRequest under proto JSON names.
type CompareResultsInput struct {
	ModelInput
	Results   string                `json:"results" jsonschema:"the results sidecar as JSON, as sysml -migration-results writes it"`
	Actions   []string              `json:"actions,omitempty" jsonschema:"the configurations compared, by qualified or simple name (the CLI's -action); every indexed configuration when empty"`
	Runs      int64                 `json:"runs,omitempty" jsonschema:"the runs made of every configuration when positive (the CLI's -runs); else each configuration's own numberOfRuns, or one"`
	Seed      *uint64               `json:"seed,omitempty" jsonschema:"the seed the runs derive theirs from (the CLI's -seed); without one, runs that draw at random are refused unless draws fixes them"`
	Draws     string                `json:"draws,omitempty" jsonschema:"the draw policy replacing every configuration's durationSimulationMode: random, min, max or average (the CLI's -draws)"`
	ClockStep *float64              `json:"clockStep,omitempty" jsonschema:"the clock step replacing every configuration's, in seconds; 0 is a continuous clock (the CLI's -clock-step)"`
	Observe   []ObservablePairInput `json:"observe,omitempty" jsonschema:"the stored observables compared, each with the feature of the run that answers it (the CLI's -observe)"`
	Engine    string                `json:"engine,omitempty" jsonschema:"the engine selection, as list_engines names them"`
}

func (g *group) registerCompareResults(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "compare_results",
		Description: "Run every configuration a SysML v1 tool's results sidecar indexes against the migrated model (the CompareResults " +
			"RPC, the CLI's -compare-results with -action, -runs, -seed, -draws, -clock-step and -observe): each on its target, for " +
			"its recorded run count under its duration mode, or the runs and draws given, and table the tool's and OpenSysML's min, " +
			"mean, p50, p90 and max of each observable with their relative difference. `configurations` carry each one's status " +
			"(holds, fails, unresolved), its rows, the distributions compared, the errors and notes, and the lines the CLI prints. " +
			"The result is the CompareResultsResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Compare results"),
	}, func(ctx context.Context, call *mcp.CallToolRequest, in CompareResultsInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.CompareResultsRequest{}
		var resp *pb.CompareResultsResponse
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			err := withProgress(ctx, call, "compare_results", func() (err error) {
				resp, err = g.svc.CompareResults(ctx, req)
				return err
			})
			return resp, err
		}, func() string {
			var b strings.Builder
			b.WriteString(errorText(resp.GetError()))
			for _, c := range resp.GetConfigurations() {
				for _, line := range c.GetLines() {
					b.WriteString(line)
					b.WriteByte('\n')
				}
			}
			b.WriteString(diagnosticsText(resp.GetDiagnostics()))
			return b.String()
		})
	})
}
