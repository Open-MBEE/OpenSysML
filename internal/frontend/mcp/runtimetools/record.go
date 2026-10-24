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

// RecordRunInput records runs of an analysis case into the model.
type RecordRunInput struct {
	ModelInput
	AnalysisSymbolID string                    `json:"analysisSymbolId" jsonschema:"qualified name of the analysis case definition or usage to run and record"`
	SubjectSymbolID  string                    `json:"subjectSymbolId,omitempty" jsonschema:"qualified name of a part/usage to instantiate as the subject, or a path from one to a nested object"`
	Arguments        []map[string]any          `json:"arguments,omitempty" jsonschema:"positional values for the case's in parameters, each a protojson Value"`
	NamedArguments   map[string]map[string]any `json:"namedArguments,omitempty" jsonschema:"values for the case's in parameters by name, each a protojson Value"`
	Engine           string                    `json:"engine,omitempty" jsonschema:"the analysis engine asked, as list_engines names them; empty is auto"`
	RecordInto       string                    `json:"recordInto,omitempty" jsonschema:"package the records go into (-record-into); empty is a Records package beside the package enclosing the case's own"`
	Ranges           []map[string]any          `json:"ranges,omitempty" jsonschema:"ranges of the case's parameters to run it over as run_sweep does, recording each completed row; each a protojson SweepRange"`
	Runs             int64                     `json:"runs,omitempty" jsonschema:"run the Simulation::MonteCarlo case this many times as run_runs does and record each completed run and the sample's conclusion (-runs)"`
	Seed             *uint64                   `json:"seed,omitempty" jsonschema:"seed of the sample's draws; required with runs"`
	AcceptDocuments  bool                      `json:"acceptDocuments,omitempty" jsonschema:"read the recorded notation from documents rather than content; required for a model of several documents"`
}

func (g *group) registerRecord(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "record_run",
		Description: "Run an analysis case — once, over the rows of a sweep (`ranges`), or as a Monte Carlo sample (`runs`) — and record " +
			"the runs into the model as AnalysisRecords declarations (the RecordRun RPC, the CLI's -record-run/-record-into, the REPL's " +
			"%record). `run`, `sweep` or `sample` is the runs' own report; `package`, `definition` and `records` name what the model " +
			"gained and `source` is the declarations generated; `documents` (and `content` for a single document) is the notation of " +
			"the document the records merged into, or the records as a document of their own, and `modelHash` names the recorded " +
			"model, cached for the next request. A recording that would error or drop a declaration leaves the model unchanged and is " +
			"the `error`. The result is the RecordRunResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Record run"),
	}, func(ctx context.Context, call *mcp.CallToolRequest, in RecordRunInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.RecordRunRequest{}
		var resp *pb.RecordRunResponse
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			err := withProgress(ctx, call, "record_run "+in.AnalysisSymbolID, func() (err error) {
				resp, err = g.svc.RecordRun(ctx, req)
				return err
			})
			return resp, err
		}, func() string {
			var b strings.Builder
			b.WriteString(errorText(resp.GetError()))
			if len(resp.GetRecords()) > 0 {
				fmt.Fprintf(&b, "recorded %s into %s: %s\n", resp.GetDefinition(), resp.GetPackage(), strings.Join(resp.GetRecords(), ", "))
			}
			if resp.GetSkipped() > 0 {
				fmt.Fprintf(&b, "%d run(s) skipped\n", resp.GetSkipped())
			}
			for _, reason := range resp.GetReasons() {
				b.WriteString(reason)
				b.WriteByte('\n')
			}
			if resp.GetModelHash() != "" {
				fmt.Fprintf(&b, "model %s\n", resp.GetModelHash())
			}
			b.WriteString(diagnosticsText(resp.GetDiagnostics()))
			return b.String()
		})
	})
}
