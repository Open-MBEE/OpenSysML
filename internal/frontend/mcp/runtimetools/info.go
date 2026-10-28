package runtimetools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// ServerInfoInput takes no arguments.
type ServerInfoInput struct{}

// ServerInfo is what the service reports about itself.
type ServerInfo struct {
	Version      string   `json:"version" jsonschema:"build version of the server, informational only"`
	Capabilities []string `json:"capabilities" jsonschema:"named capabilities this build supports; each is a stable string a caller can require"`
}

// CapabilityTools names, for every capability the service reports, the tool
// that serves it, so a capability server_info advertises is one an agent can
// act on and a capability the service withholds is refused by that tool.
var CapabilityTools = map[string]string{
	sysmlgrpc.CapabilityParseSources:                   "parse",
	sysmlgrpc.CapabilityParseSourcesAffected:           "parse",
	sysmlgrpc.CapabilityRenderDocumentFileName:         "render_model_document",
	sysmlgrpc.CapabilityInlineLanguage:                 "parse",
	sysmlgrpc.CapabilityStrictConformance:              "parse",
	sysmlgrpc.CapabilityDiagnosticCodes:                "parse",
	sysmlgrpc.CapabilityTypeFacts:                      "symbol",
	sysmlgrpc.CapabilitySymbolAttributes:               "symbol",
	sysmlgrpc.CapabilityEvaluateSubject:                "evaluate",
	sysmlgrpc.CapabilityEnumValues:                     "evaluate",
	sysmlgrpc.CapabilityUnsetValue:                     "evaluate",
	sysmlgrpc.CapabilityComplexValues:                  "evaluate",
	sysmlgrpc.CapabilityStructuredValues:               "evaluate",
	sysmlgrpc.CapabilityMeasurementRefs:                "evaluate",
	sysmlgrpc.CapabilityFunctionValues:                 "evaluate",
	sysmlgrpc.CapabilitySetValues:                      "evaluate",
	sysmlgrpc.CapabilityTensorValues:                   "evaluate",
	sysmlgrpc.CapabilityInfinityValue:                  "evaluate",
	sysmlgrpc.CapabilityMetaobjectValues:               "evaluate",
	sysmlgrpc.CapabilityUndeterminedValue:              "evaluate",
	sysmlgrpc.CapabilityBigIntValues:                   "evaluate",
	sysmlgrpc.CapabilityRationalValues:                 "evaluate",
	sysmlgrpc.CapabilityFeatureValues:                  "instantiate",
	sysmlgrpc.CapabilitySchedule:                       "execute_action",
	sysmlgrpc.CapabilityScheduleExplore:                "execute_action",
	sysmlgrpc.CapabilityFinalTime:                      "execute_action",
	sysmlgrpc.CapabilityOrderDependence:                "execute_action",
	sysmlgrpc.CapabilityPerformer:                      "execute_action",
	sysmlgrpc.CapabilityStateTrace:                     "execute_state",
	sysmlgrpc.CapabilityVerification:                   "verify_constraint",
	sysmlgrpc.CapabilityVerificationQuestions:          "verify_constraint",
	sysmlgrpc.CapabilityVerificationArguments:          "verify_constraint",
	sysmlgrpc.CapabilityVerdictNotes:                   "verify_constraint",
	sysmlgrpc.CapabilityVerificationVerdicts:           "verify_requirement",
	sysmlgrpc.CapabilityCaseEvaluations:                "run_analysis",
	sysmlgrpc.CapabilityEngines:                        "list_engines",
	sysmlgrpc.CapabilityEnginesExternal:                "list_engines",
	sysmlgrpc.CapabilityQuery:                          "query",
	sysmlgrpc.CapabilityQueryOrderBy:                   "query",
	sysmlgrpc.CapabilityOSLCQuery:                      "query",
	sysmlgrpc.CapabilityDocumentQuery:                  "run_document_query",
	sysmlgrpc.CapabilityRenderDocument:                 "render_model_document",
	sysmlgrpc.CapabilityRenderDocumentHTML:             "render_model_document",
	sysmlgrpc.CapabilityRenderDocumentAsciiDoc:         "render_model_document",
	sysmlgrpc.CapabilityRenderView:                     "render_model_view",
	sysmlgrpc.CapabilityExportGraphs:                   "export_graphs",
	sysmlgrpc.CapabilityImportData:                     "import_data",
	sysmlgrpc.CapabilityCompareResults:                 "compare_results",
	sysmlgrpc.CapabilityConvert:                        "convert",
	sysmlgrpc.CapabilityConvertDocuments:               "convert",
	sysmlgrpc.CapabilityConvertCompact:                 "convert",
	sysmlgrpc.CapabilityConvertLibraryReferences:       "convert",
	sysmlgrpc.CapabilityConvertLibraryStubs:            "convert",
	sysmlgrpc.CapabilityConvertCheckDiagnostics:        "convert",
	sysmlgrpc.CapabilityMigrate:                        "migrate",
	sysmlgrpc.CapabilityApplyEdits:                     "apply_edits",
	sysmlgrpc.CapabilityEditDocuments:                  "apply_edits",
	sysmlgrpc.CapabilityAuthoring:                      "apply_edits",
	sysmlgrpc.CapabilityConnectionAuthoring:            "apply_edits",
	sysmlgrpc.CapabilitySatisfyAuthoring:               "apply_edits",
	sysmlgrpc.CapabilityRequirementConstraintAuthoring: "apply_edits",
	sysmlgrpc.CapabilityMemberModifiers:                "apply_edits",
	sysmlgrpc.CapabilityTransitionAuthoring:            "apply_edits",
	sysmlgrpc.CapabilityVerificationObjectiveAuthoring: "apply_edits",
	sysmlgrpc.CapabilityRuns:                           "run_runs",
	sysmlgrpc.CapabilitySelfCheck:                      "self_check",
	sysmlgrpc.CapabilityRecordRun:                      "record_run",
	sysmlgrpc.CapabilityMetadataAuthoring:              "apply_edits",
	sysmlgrpc.CapabilityMetadataPrefixAuthoring:        "apply_edits",
	sysmlgrpc.CapabilityImplicitParameters:             "apply_edits",
	sysmlgrpc.CapabilitySequenceAuthoring:              "apply_edits",
	sysmlgrpc.CapabilityActionBodyStatementAuthoring:   "apply_edits",
	sysmlgrpc.CapabilityConstraintBodyAuthoring:        "apply_edits",
	sysmlgrpc.CapabilityStateActionAuthoring:           "apply_edits",
	sysmlgrpc.CapabilityImportAuthoring:                "apply_edits",
	sysmlgrpc.CapabilityDocumentationAuthoring:         "apply_edits",
	sysmlgrpc.CapabilityCommentAuthoring:               "apply_edits",
	sysmlgrpc.CapabilityCheck:                          "check_behavior",
}

// ListEnginesInput asks for the engines, probed or not.
type ListEnginesInput struct {
	Probe bool `json:"probe,omitempty" jsonschema:"start each external engine's process once and check its handshake against its manifest, reporting the outcome as its status (sysml -engines -probe); false starts nothing"`
}

func (g *group) registerInfo(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "server_info",
		Description: "Report the OpenSysML runtime's version and the named capabilities this build supports.",
		Annotations: toolkit.ReadOnly("Server info"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ServerInfoInput) (*mcp.CallToolResult, ServerInfo, error) {
		resp, err := g.svc.GetServerInfo(ctx, &pb.ServerInfoRequest{})
		if err != nil {
			return nil, ServerInfo{}, toolError(err)
		}
		caps := resp.GetCapabilities()
		if caps == nil {
			caps = []string{}
		}
		return nil, ServerInfo{Version: resp.GetVersion(), Capabilities: caps}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "list_engines",
		Description: "List the analysis engines this build registers (the ListEngines RPC, as the CLI's -engines does): each with the " +
			"questions it answers (evaluate, outcomes, holds, sensitive, satisfiable, sweep, compute), the strongest evidence it can " +
			"produce, the bounds it takes, and whether it can run here. The names are what the verification tools' `engine` argument takes; " +
			"empty there means auto. With `probe` each external engine is started once and checked against its manifest, as " +
			"`sysml -engines -probe` does. The result is the ListEnginesResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("List engines"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListEnginesInput) (*mcp.CallToolResult, Result, error) {
		resp, err := g.svc.ListEngines(ctx, &pb.ListEnginesRequest{Probe: in.Probe})
		if err != nil {
			return nil, nil, toolError(err)
		}
		var b strings.Builder
		for _, e := range resp.GetEngines() {
			state := "ready"
			if !e.GetReady() {
				state = "unavailable: " + e.GetUnavailable()
			}
			fmt.Fprintf(&b, "%s (%s) answers %s; authority %s; %s\n", e.GetName(), e.GetKind(), strings.Join(e.GetAnswers(), ", "), e.GetAuthority(), state)
		}
		return answer(resp, model{}, b.String())
	})
}
