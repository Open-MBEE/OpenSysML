package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/tests/fixtures"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// expectedValue is the fixture encoding of a pb.Value: the oneof field name
// plus its JSON literal.
type expectedValue struct {
	Kind     string          `json:"kind"`
	Value    interface{}     `json:"value"`
	Elements []expectedValue `json:"elements"`
}

// expectedFeatureValue is the fixture encoding of a pb.FeatureValue.
type expectedFeatureValue struct {
	Materialized bool        `json:"materialized"`
	ValueKind    string      `json:"value_kind"`
	Value        interface{} `json:"value"`
	// Error, when set, is a substring the feature value's error must contain.
	Error string `json:"error"`
}

// expectedAttribute is the fixture encoding of a pb.AttributeInfo.
type expectedAttribute struct {
	Type      string      `json:"type"`
	ValueKind string      `json:"value_kind"`
	Value     interface{} `json:"value"`
	Unit      string      `json:"unit"`
}

// conformanceCase is the schema of a <name>.expected.json fixture. See
// testdata/conformance/README.md.
type conformanceCase struct {
	RPC string `json:"rpc"`

	// ApplyEdits
	Operations      []conformanceEditOperation `json:"operations,omitempty"`
	ExpectedContent string                     `json:"expected_content,omitempty"`

	// Evaluate
	Expression      string `json:"expression,omitempty"`
	ContextSymbolID string `json:"context_symbol_id,omitempty"`
	SubjectSymbolID string `json:"subject_symbol_id,omitempty"`

	// EvaluateCalc: positional arguments bound to the calc's parameters.
	Arguments []expectedValue `json:"arguments,omitempty"`

	// GetSymbol, Instantiate, ExecuteAction, ExecuteState, EvaluateCalc
	SymbolID string `json:"symbol_id,omitempty"`

	// Tools names stand-ins under internal/exec/analysis/testdata registered
	// from a manifest written before the service is built.
	Tools []string `json:"tools,omitempty"`

	// ExecuteAction
	Inputs map[string]expectedValue `json:"inputs,omitempty"`

	// ExecuteAction run options, as `sysml -advance`, `-seed`, `-draws` and
	// `-clock-step` take them; absent sends none.
	Advance   *float64 `json:"advance,omitempty"`
	Seed      *uint64  `json:"seed,omitempty"`
	Draws     string   `json:"draws,omitempty"`
	ClockStep float64  `json:"clock_step,omitempty"`

	// GetDiagnostics lint selection and the codes the answer must and must not carry.
	DisableLint   []string `json:"disable_lint,omitempty"`
	EnableLint    []string `json:"enable_lint,omitempty"`
	ExpectedCodes []string `json:"expected_codes,omitempty"`
	AbsentCodes   []string `json:"absent_codes,omitempty"`

	// ExecuteState
	Events []string `json:"events,omitempty"`

	// RunRuns, RecordRun
	Runs             int64    `json:"runs,omitempty"`
	ObserveFeatures  []string `json:"observe_features,omitempty"`
	RecordInto       string   `json:"record_into,omitempty"`
	SelfCheckPackage []string `json:"self_check_package,omitempty"`

	ExpectedRowCount    int      `json:"expected_row_count,omitempty"`
	ExpectedObservables []string `json:"expected_observables,omitempty"`
	ExpectedStatus      string   `json:"expected_status,omitempty"`
	MinChecks           int64    `json:"min_checks,omitempty"`
	ExpectedPackage     string   `json:"expected_package,omitempty"`
	ExpectedRecords     []string `json:"expected_records,omitempty"`

	PerformerSymbolID   string              `json:"performer_symbol_id,omitempty"`
	CheckProperty       []string            `json:"check_property,omitempty"`
	CheckDiverge        []string            `json:"check_diverge,omitempty"`
	ExpectedVerdict     string              `json:"expected_verdict,omitempty"`
	ExpectedViolations  []expectedViolation `json:"expected_violations"`
	ExpectedDivergences map[string][]string `json:"expected_divergences"`

	// RunDocumentQuery: the symbols Instantiate creates objects of first, in
	// order, then the query and its bindings.
	Instantiate     []string          `json:"instantiate,omitempty"`
	QueryID         string            `json:"query_id,omitempty"`
	Bindings        []expectedBinding `json:"bindings,omitempty"`
	ExpectedColumns []string          `json:"expected_columns,omitempty"`
	ExpectedRows    []expectedRow     `json:"expected_rows,omitempty"`

	ExpectedResult        *expectedValue                  `json:"expected_result,omitempty"`
	ExpectedFeatureValues map[string]expectedFeatureValue `json:"expected_feature_values,omitempty"`
	ExpectedInstanceCount int                             `json:"expected_instance_count,omitempty"`
	ExpectedOutputs       map[string]expectedValue        `json:"expected_outputs,omitempty"`
	ExpectedStatesVisited []string                        `json:"expected_states_visited,omitempty"`
	ExpectedFinalContext  map[string]expectedValue        `json:"expected_final_context,omitempty"`

	// GetSymbol
	ExpectedAttributeNames []string                     `json:"expected_attribute_names,omitempty"`
	ExpectedAttributes     map[string]expectedAttribute `json:"expected_attributes,omitempty"`
	// MinWithheldLibraryAttributes requires the response to state at least this
	// many library-inherited attributes withheld from the list.
	MinWithheldLibraryAttributes int32 `json:"min_withheld_library_attributes,omitempty"`

	// ImportData: the imports, in order.
	Imports      []conformanceImport `json:"imports,omitempty"`
	ImportDryRun bool                `json:"import_dry_run,omitempty"`
	// ExpectedAssignments are the assignments' descriptions, in order, over
	// every import; ExpectedContent (shared with ApplyEdits) the edited model.
	ExpectedAssignments []string `json:"expected_assignments,omitempty"`

	// CompareResults: the results index as JSON, the configurations selected,
	// and how the runs draw.
	Results         string            `json:"results,omitempty"`
	Actions         []string          `json:"actions,omitempty"`
	Observe         map[string]string `json:"observe,omitempty"`
	ExpectedConfigs []expectedConfig  `json:"expected_configurations,omitempty"`

	// ExpectedError, when set, requires the RPC to report an in-band error
	// containing this substring (a status error, for RunDocumentQuery).
	ExpectedError string `json:"expected_error,omitempty"`

	// ExecuteAction: the simulation clock the run ended at, checked when set.
	ExpectedFinalTime *float64 `json:"expected_final_time,omitempty"`
}

// conformanceImport is one ImportData import: a name whose extension selects
// the format unless import_format does, its content, and an optional mapping.
type conformanceImport struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Format  string `json:"import_format,omitempty"`
	Map     string `json:"import_map,omitempty"`
}

// expectedConfig is what one CompareResults configuration must answer.
type expectedConfig struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Runs       int64  `json:"runs,omitempty"`
	Completed  int64  `json:"completed,omitempty"`
	StoredRuns int64  `json:"stored_runs,omitempty"`
	// Observables maps each stored observable to whether it was compared.
	Observables map[string]bool `json:"observables,omitempty"`
	// Unresolved is a substring of why the configuration could not run.
	Unresolved string `json:"unresolved,omitempty"`
	// LinesContain are substrings the CLI-formatted lines must contain.
	LinesContain []string `json:"lines_contain,omitempty"`
}

type conformanceEditBody struct {
	text       string
	operations []conformanceEditOperation
}

func (body *conformanceEditBody) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	switch data[0] {
	case '"':
		return json.Unmarshal(data, &body.text)
	case '[':
		return json.Unmarshal(data, &body.operations)
	default:
		return fmt.Errorf("edit body must be a string or operation array")
	}
}

type conformanceEditOperation struct {
	Kind              string                     `json:"kind"`
	Target            string                     `json:"target,omitempty"`
	Value             string                     `json:"value,omitempty"`
	NewName           string                     `json:"new_name,omitempty"`
	Owner             string                     `json:"owner,omitempty"`
	MemberKind        string                     `json:"member_kind,omitempty"`
	MemberName        string                     `json:"member_name,omitempty"`
	Name              string                     `json:"name,omitempty"`
	FromEnd           string                     `json:"from_end,omitempty"`
	ToEnd             string                     `json:"to_end,omitempty"`
	Type              string                     `json:"type,omitempty"`
	Multiplicity      string                     `json:"multiplicity,omitempty"`
	Specializes       []string                   `json:"specializes,omitempty"`
	IsAbstract        bool                       `json:"is_abstract,omitempty"`
	Redefines         []string                   `json:"redefines,omitempty"`
	MetadataPrefixes  []string                   `json:"metadata_prefixes,omitempty"`
	IsDefault         bool                       `json:"is_default,omitempty"`
	Direction         string                     `json:"direction,omitempty"`
	BodyExpression    string                     `json:"body_expression,omitempty"`
	Doc               string                     `json:"doc,omitempty"`
	Body              conformanceEditBody        `json:"body,omitempty"`
	Locale            string                     `json:"locale,omitempty"`
	Replace           bool                       `json:"replace,omitempty"`
	About             []string                   `json:"about,omitempty"`
	Text              string                     `json:"text,omitempty"`
	Requirement       string                     `json:"requirement,omitempty"`
	SatisfyingFeature string                     `json:"satisfying_feature,omitempty"`
	TransitionSource  string                     `json:"source,omitempty"`
	Trigger           string                     `json:"trigger,omitempty"`
	Guard             string                     `json:"guard,omitempty"`
	Effect            string                     `json:"effect,omitempty"`
	Initial           bool                       `json:"initial,omitempty"`
	Keyword           string                     `json:"keyword,omitempty"`
	Ref               string                     `json:"ref,omitempty"`
	After             string                     `json:"after,omitempty"`
	Condition         string                     `json:"condition,omitempty"`
	Via               string                     `json:"via,omitempty"`
	Until             string                     `json:"until,omitempty"`
	Parameter         string                     `json:"parameter,omitempty"`
	ElseBody          []conformanceEditOperation `json:"else_body,omitempty"`
	Visibility        string                     `json:"visibility,omitempty"`
	Recursive         bool                       `json:"is_recursive,omitempty"`
	ImportAll         bool                       `json:"is_import_all,omitempty"`
	Filters           []string                   `json:"filters,omitempty"`
	Asserted          bool                       `json:"is_asserted,omitempty"`
	Negated           bool                       `json:"is_negated,omitempty"`
	Expression        string                     `json:"expression,omitempty"`
	MetadataType      string                     `json:"metadata_type,omitempty"`
	MetadataValues    []conformanceMetadataValue `json:"metadata_values,omitempty"`
	Shorthand         bool                       `json:"shorthand,omitempty"`
	Cascade           bool                       `json:"cascade,omitempty"`
}

type conformanceMetadataValue struct {
	Feature string `json:"feature"`
	Value   string `json:"value"`
}

// TestGRPCConformance is the AGENTS.md §5.2 Layer 2 contract for the gRPC
// service: every runtime RPC is driven end-to-end from a real model file
// (<name>.sysml) through ParseFile and the RPC under test, and its response is
// compared against <name>.expected.json.
func TestGRPCConformance(t *testing.T) {
	conformanceDir := filepath.Join("testdata", "conformance")

	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatalf("read conformance dir: %v", err)
	}

	cases := 0
	for _, entry := range entries {
		caseName, ok := fixtures.CaseName(entry.Name())
		if entry.IsDir() || !ok {
			continue
		}
		cases++
		t.Run(caseName, func(t *testing.T) {
			runGRPCConformanceCase(t, conformanceDir, caseName)
		})
	}

	if cases == 0 {
		t.Fatalf("no conformance cases in %s", conformanceDir)
	}
}

func runGRPCConformanceCase(t *testing.T, dir, caseName string) {
	t.Helper()

	expectedData, err := os.ReadFile(filepath.Join(dir, caseName+".expected.json"))
	if err != nil {
		t.Fatalf("read expectation: %v", err)
	}
	var tc conformanceCase
	if err := json.Unmarshal(expectedData, &tc); err != nil {
		t.Fatalf("parse expectation: %v", err)
	}

	modelData, err := os.ReadFile(filepath.Join(dir, caseName+".sysml"))
	if err != nil {
		t.Fatalf("read model: %v", err)
	}

	if len(tc.Tools) > 0 {
		t.Setenv(analysis.ToolsEnv, conformanceToolManifest(t, tc.Tools))
	}

	srv, err := grpc.NewService(4, "test")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()

	parseResp, err := srv.ParseFile(ctx, &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: string(modelData)},
		ContentHash: caseName,
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range parseResp.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("model has a diagnostic error: %s", diag.Message)
		}
	}

	switch tc.RPC {
	case "GetSymbol":
		runGetSymbolCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "Evaluate":
		runEvaluateCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "Instantiate":
		runInstantiateCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "ExecuteAction":
		runExecuteActionCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "GetDiagnostics":
		runGetDiagnosticsCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "ExecuteState":
		runExecuteStateCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "ApplyEdits":
		runApplyEditsCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "RunDocumentQuery":
		runRunDocumentQueryCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "EvaluateCalc":
		runEvaluateCalcCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "RunRuns":
		runRunRunsCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "SelfCheck":
		runSelfCheckCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "RecordRun":
		runRecordRunCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "ImportData":
		runImportDataCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "CompareResults":
		runCompareResultsCase(t, srv, ctx, parseResp.ModelHash, tc)
	case "CheckBehavior":
		runCheckBehaviorCase(t, srv, ctx, parseResp.ModelHash, tc)
	default:
		t.Fatalf("unknown rpc %q", tc.RPC)
	}
}

func runImportDataCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()
	req := &pb.ImportDataRequest{ModelHash: modelHash, ImportDryRun: tc.ImportDryRun}
	for _, item := range tc.Imports {
		req.Imports = append(req.Imports, &pb.DataImport{
			Name: item.Name, Content: item.Content, ImportFormat: item.Format, ImportMap: item.Map,
		})
	}
	resp, err := srv.ImportData(ctx, req)
	if err != nil {
		t.Fatalf("ImportData: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		if len(resp.Files) != 0 || resp.ModelHash != "" || resp.Content != "" {
			t.Errorf("a refused import answered files %v, model %q, content %q; want nothing changed", resp.Files, resp.ModelHash, resp.Content)
		}
		return
	}
	if len(resp.Files) != len(tc.Imports) {
		t.Fatalf("files = %d, want one per import (%d)", len(resp.Files), len(tc.Imports))
	}
	var descriptions []string
	for i, file := range resp.Files {
		if file.Name != tc.Imports[i].Name {
			t.Errorf("file %d is %q, want %q", i, file.Name, tc.Imports[i].Name)
		}
		if int(file.Values) != len(file.Assignments) {
			t.Errorf("%s counts %d values over %d assignments", file.Name, file.Values, len(file.Assignments))
		}
		for _, a := range file.Assignments {
			if a.Element == "" || a.Feature == "" || a.Value == "" || a.Where == "" {
				t.Errorf("assignment %v is incomplete", a)
			}
			descriptions = append(descriptions, a.Description)
		}
	}
	if !slices.Equal(descriptions, tc.ExpectedAssignments) {
		t.Errorf("assignments = %q, want %q", descriptions, tc.ExpectedAssignments)
	}
	if tc.ImportDryRun {
		if resp.ModelHash != "" || resp.Content != "" || len(resp.Documents) != 0 {
			t.Errorf("a dry run answered model %q, content %q, documents %d; want the model untouched", resp.ModelHash, resp.Content, len(resp.Documents))
		}
		return
	}
	if resp.ModelHash == "" || resp.ModelHash == modelHash {
		t.Fatalf("the imported model's hash %q is not a new model", resp.ModelHash)
	}
	if resp.Content != tc.ExpectedContent {
		t.Errorf("content:\n%s\nwant:\n%s", resp.Content, tc.ExpectedContent)
	}
	if len(resp.Documents) != 1 || resp.Documents[0].Content != resp.Content {
		t.Errorf("documents = %v; want the one edited document", resp.Documents)
	}
	if sym, err := srv.GetSymbol(ctx, &pb.GetSymbolRequest{ModelHash: resp.ModelHash, SymbolId: resp.Files[0].Assignments[0].Element}); err != nil || sym.Error != "" {
		t.Errorf("the imported model does not answer GetSymbol: %v %s", err, sym.GetError())
	}
}

func runCompareResultsCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()
	req := &pb.CompareResultsRequest{ModelHash: modelHash, Results: tc.Results, Actions: tc.Actions, Runs: tc.Runs, Seed: tc.Seed}
	for _, stored := range slices.Sorted(maps.Keys(tc.Observe)) {
		req.Observe = append(req.Observe, &pb.ObservablePair{Stored: stored, Feature: tc.Observe[stored]})
	}
	resp, err := srv.CompareResults(ctx, req)
	if err != nil {
		t.Fatalf("CompareResults: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if len(resp.Configurations) != len(tc.ExpectedConfigs) {
		t.Fatalf("configurations = %d, want %d", len(resp.Configurations), len(tc.ExpectedConfigs))
	}
	for i, want := range tc.ExpectedConfigs {
		got := resp.Configurations[i]
		if got.Name != want.Name || got.Status != want.Status {
			t.Errorf("configuration %d is %s (%s), want %s (%s)", i, got.Name, got.Status, want.Name, want.Status)
		}
		if got.Runs != want.Runs || got.Completed != want.Completed || got.StoredRuns != want.StoredRuns {
			t.Errorf("%s ran %d (%d completed) beside %d stored, want %d (%d) beside %d", want.Name, got.Runs, got.Completed, got.StoredRuns, want.Runs, want.Completed, want.StoredRuns)
		}
		if len(got.Rows) != int(got.Runs) {
			t.Errorf("%s tables %d rows for %d runs", want.Name, len(got.Rows), got.Runs)
		}
		if want.Unresolved != "" && !strings.Contains(got.Unresolved, want.Unresolved) {
			t.Errorf("%s unresolved = %q, want it to mention %q", want.Name, got.Unresolved, want.Unresolved)
		}
		if tc.Seed != nil && want.Status != "unresolved" && (got.Seed == nil || *got.Seed != *tc.Seed) {
			t.Errorf("%s seed = %v, want %d", want.Name, got.Seed, *tc.Seed)
		}
		if len(got.Observables) != len(want.Observables) {
			t.Errorf("%s observables = %d, want %d", want.Name, len(got.Observables), len(want.Observables))
		}
		for _, o := range got.Observables {
			compared, known := want.Observables[o.Stored]
			if !known {
				t.Errorf("%s compares %q, which the fixture does not expect", want.Name, o.Stored)
				continue
			}
			if o.Compared != compared {
				t.Errorf("%s %s compared = %v, want %v (notes %v)", want.Name, o.Stored, o.Compared, compared, o.Notes)
			}
			if compared && (o.Tool == nil || o.Run == nil || o.Difference == nil || o.Run.Count != got.Completed) {
				t.Errorf("%s %s is compared without both distributions and their difference: %v", want.Name, o.Stored, o)
			}
		}
		if len(got.Lines) == 0 {
			t.Errorf("%s answers no lines", want.Name)
		}
		joined := strings.Join(got.Lines, "\n")
		for _, text := range want.LinesContain {
			if !strings.Contains(joined, text) {
				t.Errorf("%s lines:\n%s\nwant them to contain %q", want.Name, joined, text)
			}
		}
	}
}

func runApplyEditsCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	operations := make([]*pb.EditOperation, 0, len(tc.Operations))
	for _, op := range tc.Operations {
		switch op.Kind {
		case "add_member":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
					Owner: op.Owner, Kind: op.MemberKind, Name: op.MemberName,
					Type: op.Type, Multiplicity: op.Multiplicity,
					Value: op.Value, Specializes: op.Specializes,
					IsAbstract: op.IsAbstract, Redefines: op.Redefines,
					IsDefault: op.IsDefault, Direction: op.Direction,
					MetadataPrefixes: op.MetadataPrefixes,
					BodyExpression:   op.BodyExpression,
					Doc:              op.Doc,
				}},
			})
		case "add_documentation":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddDocumentation{AddDocumentation: &pb.AddDocumentationEdit{
					Target: op.Target, Body: op.Body.text, Name: op.Name,
					Locale: op.Locale, Replace: op.Replace,
				}},
			})
		case "add_comment":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddComment{AddComment: &pb.AddCommentEdit{
					Owner: op.Owner, Body: op.Body.text, Name: op.Name, About: op.About, Locale: op.Locale,
				}},
			})
		case "add_note":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddNote{AddNote: &pb.AddNoteEdit{Target: op.Target, Text: op.Text}},
			})
		case "add_connection":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddConnection{AddConnection: &pb.AddConnectionEdit{
					Owner: op.Owner, Kind: op.MemberKind, FromEnd: op.FromEnd,
					ToEnd: op.ToEnd, Name: op.Name, Type: op.Type,
				}},
			})
		case "add_satisfy":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddSatisfy{AddSatisfy: &pb.AddSatisfyEdit{
					Owner: op.Owner, Requirement: op.Requirement,
					SatisfyingFeature: op.SatisfyingFeature,
					IsAsserted:        op.Asserted, IsNegated: op.Negated,
				}},
			})
		case "add_requirement_constraint":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddRequirementConstraint{
					AddRequirementConstraint: &pb.AddRequirementConstraintEdit{
						Owner: op.Owner, Kind: op.MemberKind, Expression: op.Expression,
						Name: op.Name,
					},
				},
			})
		case "add_transition":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddTransition{AddTransition: &pb.AddTransitionEdit{
					Owner: op.Owner, Name: op.Name, Source: op.TransitionSource,
					Target: op.Target, Trigger: op.Trigger, Guard: op.Guard,
					Effect: op.Effect, Initial: op.Initial,
				}},
			})
		case "add_verify":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddVerify{AddVerify: &pb.AddVerifyEdit{
					Owner: op.Owner, Requirement: op.Requirement,
				}},
			})
		case "add_metadata":
			values := make([]*pb.MetadataFeatureValue, 0, len(op.MetadataValues))
			for _, value := range op.MetadataValues {
				values = append(values, &pb.MetadataFeatureValue{
					Feature: value.Feature, Value: value.Value,
				})
			}
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddMetadata{AddMetadata: &pb.AddMetadataEdit{
					Owner: op.Owner, MetadataType: op.MetadataType, Name: op.Name,
					About: op.About, Values: values, Shorthand: op.Shorthand,
				}},
			})
		case "add_metadata_prefix":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddMetadataPrefix{
					AddMetadataPrefix: &pb.AddMetadataPrefixEdit{
						Target: op.Target, MetadataType: op.MetadataType,
					},
				},
			})
		case "add_sequence":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddSequence{
					AddSequence: conformanceSequenceEdit(op),
				},
			})
		case "add_import":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_AddImport{AddImport: &pb.AddImportEdit{
					Owner: op.Owner, Visibility: op.Visibility, Target: op.Target,
					IsRecursive: op.Recursive, IsImportAll: op.ImportAll,
					Filters: op.Filters,
				}},
			})
		case "delete":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_Delete{Delete: &pb.DeleteEdit{
					Target: op.Target, Cascade: op.Cascade,
				}},
			})
		case "set_value":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_SetValue{SetValue: &pb.SetValueEdit{
					Target: op.Target, Value: op.Value,
				}},
			})
		case "rename":
			operations = append(operations, &pb.EditOperation{
				Operation: &pb.EditOperation_Rename{Rename: &pb.RenameEdit{
					Target: op.Target, NewName: op.NewName,
				}},
			})
		default:
			t.Fatalf("unknown edit operation %q", op.Kind)
		}
	}

	resp, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: modelHash, Operations: operations,
	})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if resp.Content != tc.ExpectedContent {
		t.Errorf("content =\n%s\nwant\n%s", resp.Content, tc.ExpectedContent)
	}
}

func conformanceSequenceEdit(op conformanceEditOperation) *pb.AddSequenceEdit {
	sequence := &pb.AddSequenceEdit{
		Owner: op.Owner, Keyword: op.Keyword, Ref: op.Ref,
		MemberKind: op.MemberKind, MemberName: op.MemberName, Type: op.Type,
		After: op.After, Condition: op.Condition, Value: op.Value, Target: op.Target,
		Via: op.Via, Until: op.Until, Multiplicity: op.Multiplicity,
		Parameter: op.Parameter,
	}
	for _, child := range op.Body.operations {
		sequence.Body = append(sequence.Body, conformanceSequenceEdit(child))
	}
	for _, child := range op.ElseBody {
		sequence.ElseBody = append(sequence.ElseBody, conformanceSequenceEdit(child))
	}
	return sequence
}

func runEvaluateCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	resp, err := srv.Evaluate(ctx, &pb.EvaluateRequest{
		ModelHash:       modelHash,
		Expression:      tc.Expression,
		ContextSymbolId: tc.ContextSymbolID,
		SubjectSymbolId: tc.SubjectSymbolID,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if tc.ExpectedResult != nil {
		checkValue(t, "result", *tc.ExpectedResult, resp.Result)
	}
}

// runEvaluateCalcCase invokes a calc through the service, as %calc does at the
// prompt: positional arguments bound, the result and the outputs a usage
// computes compared against the fixture.
func runEvaluateCalcCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	args := make([]*pb.Value, 0, len(tc.Arguments))
	for _, arg := range tc.Arguments {
		args = append(args, toProtoValue(t, arg))
	}

	resp, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{
		ModelHash: modelHash,
		SymbolId:  tc.SymbolID,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("EvaluateCalc: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if tc.ExpectedResult != nil {
		checkValue(t, "result", *tc.ExpectedResult, resp.Result)
	}
	if tc.ExpectedOutputs != nil {
		got := make(map[string]*pb.Value, len(resp.Outputs))
		for _, out := range resp.Outputs {
			got[out.GetName()] = out.GetValue()
		}
		for name, want := range tc.ExpectedOutputs {
			value, ok := got[name]
			if !ok {
				t.Errorf("missing output %q", name)
				continue
			}
			checkValue(t, "output "+name, want, value)
		}
	}
}

// conformanceToolManifest builds each stand-in the fixture names and writes a
// manifest registering it under the tool name the models use.
func conformanceToolManifest(t *testing.T, tools []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, tool := range tools {
		spec, ok := conformanceTools[tool]
		if !ok {
			t.Fatalf("unknown tool stand-in %q", tool)
		}
		entry := `{"kind":"tool","toolName":` + strconv.Quote(spec.toolName) + `,` +
			`"executable":` + strconv.Quote(conformanceToolBinary(t, tool)) + `,` +
			`"variables":[` + strings.Join(spec.variables, `,`) + `]}`
		if err := os.WriteFile(filepath.Join(dir, spec.toolName+".json"), []byte(entry), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// conformanceTools names the stand-ins a fixture can register: the toolName
// their ToolExecution names and the variables the manifest accepts.
var conformanceTools = map[string]struct {
	toolName  string
	variables []string
}{
	"toolcalc": {"Thermo", []string{`"mass"`, `"power"`, `"warn"`, `"Tmax"`}},
}

var conformanceToolBuilds sync.Map // name -> string path or error

// conformanceToolBinary builds a stand-in once per test binary.
func conformanceToolBinary(t *testing.T, name string) string {
	t.Helper()
	if built, ok := conformanceToolBuilds.Load(name); ok {
		switch v := built.(type) {
		case string:
			return v
		case error:
			t.Fatalf("building the %s stand-in: %v", name, v)
		}
	}
	dir, err := os.MkdirTemp("", "conformance-tool")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	build := exec.Command("go", gobuild.Args(path)...)
	build.Dir = filepath.Join("..", "..", "internal", "exec", "analysis", "testdata", name)
	if out, err := build.CombinedOutput(); err != nil {
		err = fmt.Errorf("go build: %v\n%s", err, out)
		conformanceToolBuilds.Store(name, err)
		t.Fatalf("building the %s stand-in: %v", name, err)
	}
	conformanceToolBuilds.Store(name, path)
	return path
}

// runGetSymbolCase pins the static facts a symbol is reported with, and is how
// the attribute set is kept from regressing to empty.
func runGetSymbolCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	resp, err := srv.GetSymbol(ctx, &pb.GetSymbolRequest{
		ModelHash: modelHash,
		SymbolId:  tc.SymbolID,
	})
	if err != nil {
		t.Fatalf("GetSymbol: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if resp.Symbol == nil {
		t.Fatal("expected a symbol")
	}

	attrs := resp.Symbol.GetAttributes()
	if tc.ExpectedAttributeNames != nil {
		var got []string
		for _, attr := range attrs {
			got = append(got, attr.GetName())
		}
		if strings.Join(got, ",") != strings.Join(tc.ExpectedAttributeNames, ",") {
			t.Errorf("attributes = %v, want %v", got, tc.ExpectedAttributeNames)
		}
	}
	if want := tc.MinWithheldLibraryAttributes; want > 0 {
		if got := resp.Symbol.GetWithheldLibraryAttributes(); got < want {
			t.Errorf("withheld library attributes = %d, want at least %d", got, want)
		}
	}

	byName := make(map[string]*pb.AttributeInfo, len(attrs))
	for _, attr := range attrs {
		byName[attr.GetName()] = attr
	}
	for name, want := range tc.ExpectedAttributes {
		got, ok := byName[name]
		if !ok {
			t.Errorf("missing attribute %q", name)
			continue
		}
		if got.GetType() != want.Type {
			t.Errorf("attribute %q: type = %q, want %q", name, got.GetType(), want.Type)
		}
		if got.GetUnit() != want.Unit {
			t.Errorf("attribute %q: unit = %q, want %q", name, got.GetUnit(), want.Unit)
		}
		if want.ValueKind == "" {
			if got.GetValue() != nil {
				kind, value := describeValue(got.GetValue())
				t.Errorf("attribute %q: unexpected value %s %v", name, kind, value)
			}
			continue
		}
		checkValue(t, "attribute "+name, expectedValue{Kind: want.ValueKind, Value: want.Value}, got.GetValue())
	}
}

func runInstantiateCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	resp, err := srv.Instantiate(ctx, &pb.InstantiateRequest{
		ModelHash: modelHash,
		SymbolId:  tc.SymbolID,
	})
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if resp.Instance == nil {
		t.Fatal("expected an instance")
	}
	if resp.Instance.TypeSymbolId != tc.SymbolID {
		t.Errorf("type_symbol_id = %q, want %q", resp.Instance.TypeSymbolId, tc.SymbolID)
	}
	for name, want := range tc.ExpectedFeatureValues {
		fv, ok := resp.Instance.FeatureValues[name]
		if !ok {
			t.Errorf("missing feature value %q", name)
			continue
		}
		if fv.Materialized != want.Materialized {
			t.Errorf("feature value %q: materialized = %v, want %v", name, fv.Materialized, want.Materialized)
		}
		if want.Error == "" {
			if fv.Error != "" {
				t.Errorf("feature value %q: unexpected error %q", name, fv.Error)
			}
		} else if !strings.Contains(fv.Error, want.Error) {
			t.Errorf("feature value %q: error = %q, want it to contain %q", name, fv.Error, want.Error)
		}
		if want.ValueKind != "" {
			checkValue(t, "feature value "+name, expectedValue{Kind: want.ValueKind, Value: want.Value}, fv.Value)
		}
	}
	if tc.ExpectedInstanceCount != 0 && len(resp.Instances) != tc.ExpectedInstanceCount {
		t.Errorf("instances = %d, want %d", len(resp.Instances), tc.ExpectedInstanceCount)
	}
}

func runExecuteActionCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	var inputs map[string]*pb.Value
	if len(tc.Inputs) > 0 {
		inputs = make(map[string]*pb.Value, len(tc.Inputs))
		for name, val := range tc.Inputs {
			inputs[name] = toProtoValue(t, val)
		}
	}

	resp, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{
		ModelHash:         modelHash,
		ActionSymbolId:    tc.SymbolID,
		PerformerSymbolId: tc.PerformerSymbolID,
		Inputs:            inputs,
		Advance:           tc.Advance,
		Seed:              tc.Seed,
		Draws:             tc.Draws,
		ClockStep:         tc.ClockStep,
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if tc.ExpectedFinalTime != nil && resp.FinalTime != *tc.ExpectedFinalTime {
		t.Errorf("final_time = %g, want %g", resp.FinalTime, *tc.ExpectedFinalTime)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	for name, want := range tc.ExpectedOutputs {
		got, ok := resp.Outputs[name]
		if !ok {
			t.Errorf("missing output %q", name)
			continue
		}
		checkValue(t, "output "+name, want, got)
	}
}

// runGetDiagnosticsCase asks for the model's diagnostics under the case's lint
// selection and checks the codes present and absent.
func runGetDiagnosticsCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()
	resp, err := srv.GetDiagnostics(ctx, &pb.DiagnosticsRequest{
		ModelHash:   modelHash,
		DisableLint: tc.DisableLint,
		EnableLint:  tc.EnableLint,
	})
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	codes := map[string]bool{}
	for _, d := range resp.Diagnostics {
		codes[d.GetCode()] = true
	}
	for _, code := range tc.ExpectedCodes {
		if !codes[code] {
			t.Errorf("diagnostics lack %s: %v", code, codes)
		}
	}
	for _, code := range tc.AbsentCodes {
		if codes[code] {
			t.Errorf("diagnostics carry %s under the selection: %v", code, codes)
		}
	}
}

func runExecuteStateCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	resp, err := srv.ExecuteState(ctx, &pb.ExecuteStateRequest{
		ModelHash:            modelHash,
		StateMachineSymbolId: tc.SymbolID,
		Events:               tc.Events,
	})
	if err != nil {
		t.Fatalf("ExecuteState: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if tc.ExpectedStatesVisited != nil {
		if len(resp.StatesVisited) != len(tc.ExpectedStatesVisited) {
			t.Fatalf("states_visited = %v, want %v", resp.StatesVisited, tc.ExpectedStatesVisited)
		}
		for i, want := range tc.ExpectedStatesVisited {
			if resp.StatesVisited[i] != want {
				t.Fatalf("states_visited = %v, want %v", resp.StatesVisited, tc.ExpectedStatesVisited)
			}
		}
	}
	for name, want := range tc.ExpectedFinalContext {
		got, ok := resp.FinalContext[name]
		if !ok {
			t.Errorf("missing final_context entry %q", name)
			continue
		}
		checkValue(t, "final_context "+name, want, got)
	}
}

// checkExpectedError validates the in-band error field and reports whether the
// case is a negative one, in which case no further assertions apply.
func runRunRunsCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()
	req := &pb.RunRunsRequest{
		ModelHash:       modelHash,
		SubjectSymbolId: tc.SubjectSymbolID,
		Runs:            tc.Runs,
		Seed:            tc.Seed,
		Observe:         tc.ObserveFeatures,
		Draws:           tc.Draws,
	}
	if tc.ContextSymbolID != "" {
		req.AnalysisSymbolId = tc.ContextSymbolID
	} else {
		req.ActionSymbolId = tc.SymbolID
	}
	resp, err := srv.RunRuns(ctx, req)
	if err != nil {
		if tc.ExpectedError != "" && strings.Contains(err.Error(), tc.ExpectedError) {
			return
		}
		t.Fatalf("RunRuns: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if len(resp.Rows) != tc.ExpectedRowCount {
		t.Errorf("rows = %d, want %d", len(resp.Rows), tc.ExpectedRowCount)
	}
	for i, row := range resp.Rows {
		if row.Error != "" {
			t.Errorf("run %d failed: %s", i+1, row.Error)
		}
	}
	if !slices.Equal(resp.Observables, tc.ExpectedObservables) {
		t.Errorf("observables = %v, want %v", resp.Observables, tc.ExpectedObservables)
	}
	if len(resp.Distributions) != len(tc.ExpectedObservables) {
		t.Errorf("distributions = %d, want one per observable", len(resp.Distributions))
	}
	for _, d := range resp.Distributions {
		if int(d.Count) != tc.ExpectedRowCount {
			t.Errorf("distribution %s counts %d runs, want %d", d.Name, d.Count, tc.ExpectedRowCount)
		}
	}
	for name, want := range tc.ExpectedOutputs {
		var got *pb.Value
		for _, entry := range resp.Outputs {
			if entry.Name == name {
				got = entry.Value
			}
		}
		if got == nil {
			t.Errorf("output %q is missing from %v", name, resp.Outputs)
			continue
		}
		checkValue(t, "output "+name, want, got)
	}
	if tc.Seed != nil && (resp.Seed == nil || *resp.Seed != *tc.Seed) {
		t.Errorf("seed = %v, want %d", resp.Seed, *tc.Seed)
	}
}

func runSelfCheckCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()
	resp, err := srv.SelfCheck(ctx, &pb.SelfCheckRequest{ModelHash: modelHash, SelfCheckPackage: tc.SelfCheckPackage})
	if err != nil {
		t.Fatalf("SelfCheck: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if resp.Status != tc.ExpectedStatus {
		t.Errorf("status = %q, want %q; findings %v", resp.Status, tc.ExpectedStatus, resp.Findings)
	}
	if resp.Checks < tc.MinChecks {
		t.Errorf("checks = %d, want at least %d", resp.Checks, tc.MinChecks)
	}
	if resp.Elements == 0 || len(resp.Applications) == 0 || !strings.HasPrefix(resp.Summary, "Self-model check: ") {
		t.Errorf("the answer is incomplete: elements %d, applications %d, summary %q", resp.Elements, len(resp.Applications), resp.Summary)
	}
}

func runRecordRunCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()
	resp, err := srv.RecordRun(ctx, &pb.RecordRunRequest{
		ModelHash:        modelHash,
		AnalysisSymbolId: tc.SymbolID,
		SubjectSymbolId:  tc.SubjectSymbolID,
		RecordInto:       tc.RecordInto,
		Runs:             tc.Runs,
		Seed:             tc.Seed,
		AcceptDocuments:  true,
	})
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if resp.Package != tc.ExpectedPackage {
		t.Errorf("package = %q, want %q", resp.Package, tc.ExpectedPackage)
	}
	if !slices.Equal(resp.Records, tc.ExpectedRecords) {
		t.Errorf("records = %v, want %v", resp.Records, tc.ExpectedRecords)
	}
	if (tc.Runs > 0) != (resp.Sample != nil) || (tc.Runs == 0) != (resp.Run != nil) {
		t.Errorf("runs=%d answered run=%v sample=%v", tc.Runs, resp.Run != nil, resp.Sample != nil)
	}
	if resp.ModelHash == "" || resp.ModelHash == modelHash {
		t.Fatalf("the recorded model's hash %q is not a new model", resp.ModelHash)
	}
	if len(resp.Documents) == 0 || resp.Documents[0].Content == "" {
		t.Errorf("the answer carries no recorded document")
	}
	for _, record := range tc.ExpectedRecords {
		sym, err := srv.GetSymbol(ctx, &pb.GetSymbolRequest{ModelHash: resp.ModelHash, SymbolId: record})
		if err != nil || sym.Error != "" {
			t.Errorf("%s is not in the recorded model: %v %s", record, err, sym.GetError())
		}
	}
}

type expectedViolation struct {
	Kind     string `json:"kind"`
	Property string `json:"property"`
	Depth    int32  `json:"depth"`
}

func runCheckBehaviorCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()

	resp, err := srv.CheckBehavior(ctx, &pb.CheckBehaviorRequest{
		ModelHash:         modelHash,
		ActionSymbolId:    tc.SymbolID,
		PerformerSymbolId: tc.PerformerSymbolID,
		CheckProperty:     tc.CheckProperty,
		CheckDiverge:      tc.CheckDiverge,
	})
	if err != nil {
		t.Fatalf("CheckBehavior: %v", err)
	}
	if checkExpectedError(t, tc, resp.Error) {
		return
	}
	if resp.Verdict != tc.ExpectedVerdict {
		t.Errorf("verdict = %q, want %q; lines %v", resp.Verdict, tc.ExpectedVerdict, resp.Lines)
	}
	if len(resp.Lines) == 0 {
		t.Errorf("the answer carries no report lines")
	}
	if len(resp.Violations) != len(tc.ExpectedViolations) {
		t.Fatalf("violations = %v, want %d", resp.Violations, len(tc.ExpectedViolations))
	}
	for i, want := range tc.ExpectedViolations {
		got := resp.Violations[i]
		if got.Kind != want.Kind || got.Property != want.Property || got.Depth != want.Depth {
			t.Errorf("violation %d = %v, want %+v", i, got, want)
		}
		if got.Witness == nil || got.Witness.Schedule == "" {
			t.Errorf("violation %d carries no replay witness", i)
		}
	}
	if len(resp.Divergences) != len(tc.ExpectedDivergences) {
		t.Fatalf("divergences = %v, want %v", resp.Divergences, tc.ExpectedDivergences)
	}
	for _, got := range resp.Divergences {
		want, ok := tc.ExpectedDivergences[got.Feature]
		if !ok {
			t.Errorf("unexpected divergence of %q", got.Feature)
			continue
		}
		var values []string
		for _, v := range got.Values {
			values = append(values, v.Value)
		}
		if !slices.Equal(values, want) {
			t.Errorf("%s ends as %v, want %v", got.Feature, values, want)
		}
	}
}

func checkExpectedError(t *testing.T, tc conformanceCase, actual string) bool {
	t.Helper()

	if tc.ExpectedError == "" {
		if actual != "" {
			t.Fatalf("unexpected error: %s", actual)
		}
		return false
	}
	if !strings.Contains(actual, tc.ExpectedError) {
		t.Fatalf("error = %q, want it to contain %q", actual, tc.ExpectedError)
	}
	return true
}

// toProtoValue converts a fixture value to its pb.Value oneof.
func toProtoValue(t *testing.T, ev expectedValue) *pb.Value {
	t.Helper()

	switch ev.Kind {
	case "int_value":
		return &pb.Value{Kind: &pb.Value_IntValue{IntValue: int64(mustFloat(t, ev))}}
	case "real_value":
		return &pb.Value{Kind: &pb.Value_RealValue{RealValue: mustFloat(t, ev)}}
	case "rational_value":
		text, _ := ev.Value.(string)
		num, den, ok := strings.Cut(text, "/")
		if !ok {
			t.Fatalf("rational_value: %v is not \"<numerator>/<denominator>\"", ev.Value)
		}
		return &pb.Value{Kind: &pb.Value_RationalValue{RationalValue: &pb.Rational{Numerator: num, Denominator: den}}}
	case "bool_value":
		b, ok := ev.Value.(bool)
		if !ok {
			t.Fatalf("bool_value: %v is not a boolean", ev.Value)
		}
		return &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: b}}
	case "string_value":
		s, ok := ev.Value.(string)
		if !ok {
			t.Fatalf("string_value: %v is not a string", ev.Value)
		}
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: s}}
	case "null":
		return &pb.Value{Kind: &pb.Value_Null{Null: ""}}
	case "instance_id":
		return &pb.Value{Kind: &pb.Value_InstanceId{InstanceId: int64(mustFloat(t, ev))}}
	case "sequence":
		seq := &pb.ValueSequence{}
		for _, element := range ev.Elements {
			seq.Elements = append(seq.Elements, toProtoValue(t, element))
		}
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: seq}}
	default:
		t.Fatalf("unknown value kind %q", ev.Kind)
		return nil
	}
}

// checkValue compares a pb.Value against its fixture encoding, including which
// oneof arm is set.
func checkValue(t *testing.T, label string, want expectedValue, got *pb.Value) {
	t.Helper()

	if got == nil {
		t.Errorf("%s: value is nil, want %s %v", label, want.Kind, want.Value)
		return
	}

	gotKind, gotValue := describeValue(got)
	if gotKind != want.Kind {
		t.Errorf("%s: kind = %s, want %s", label, gotKind, want.Kind)
		return
	}

	switch want.Kind {
	case "int_value":
		if got.GetIntValue() != int64(mustFloat(t, want)) {
			t.Errorf("%s: value = %d, want %v", label, got.GetIntValue(), want.Value)
		}
	case "real_value":
		if got.GetRealValue() != mustFloat(t, want) {
			t.Errorf("%s: value = %v, want %v", label, got.GetRealValue(), want.Value)
		}
	case "null", "instance_id", "unset":
		// The oneof arm carries all the information; ids are runtime-assigned.
	case "sequence":
		elements := got.GetSequence().GetElements()
		if len(elements) != len(want.Elements) {
			t.Errorf("%s: %d element(s), want %d", label, len(elements), len(want.Elements))
			return
		}
		for i, element := range want.Elements {
			checkValue(t, fmt.Sprintf("%s element %d", label, i+1), element, elements[i])
		}
	default:
		if fmt.Sprint(gotValue) != fmt.Sprint(want.Value) {
			t.Errorf("%s: value = %v, want %v", label, gotValue, want.Value)
		}
	}
}

// describeValue reports the set oneof arm of a pb.Value and its payload.
func describeValue(v *pb.Value) (string, interface{}) {
	switch k := v.Kind.(type) {
	case *pb.Value_IntValue:
		return "int_value", k.IntValue
	case *pb.Value_RealValue:
		return "real_value", k.RealValue
	case *pb.Value_RationalValue:
		return "rational_value", k.RationalValue.GetNumerator() + "/" + k.RationalValue.GetDenominator()
	case *pb.Value_BoolValue:
		return "bool_value", k.BoolValue
	case *pb.Value_StringValue:
		return "string_value", k.StringValue
	case *pb.Value_InstanceId:
		return "instance_id", k.InstanceId
	case *pb.Value_Sequence:
		return "sequence", k.Sequence
	case *pb.Value_Quantity:
		return "quantity", describeQuantity(k.Quantity)
	case *pb.Value_Null:
		return "null", nil
	case *pb.Value_Unset:
		return "unset", nil
	default:
		return "no arm", nil
	}
}

// describeQuantity renders a quantity as "<magnitude> [<unit as written>] =
// <reduction>", which is every part of it a fixture needs to pin.
func describeQuantity(q *pb.Quantity) string {
	if q == nil {
		return ""
	}
	magnitude := "unset"
	switch m := q.GetMagnitude().(type) {
	case *pb.Quantity_IntMagnitude:
		magnitude = strconv.FormatInt(m.IntMagnitude, 10)
	case *pb.Quantity_RealMagnitude:
		magnitude = strconv.FormatFloat(m.RealMagnitude, 'g', -1, 64)
	case *pb.Quantity_RationalMagnitude:
		if r, ok := semantics.CanonicalRational(m.RationalMagnitude.GetNumerator(), m.RationalMagnitude.GetDenominator()); ok {
			magnitude = r.FormatRational()
		}
	}
	return fmt.Sprintf("%s [%s] = %s", magnitude, q.GetUnit(), unitTermText(q.GetUnitTerm()))
}

// unitTermText renders a unit reduction the way fixtures spell it,
// "1000/3600·SI::m·SI::s^-1", with a scale of one and exponents of one implicit.
func unitTermText(term *pb.UnitTerm) string {
	if term == nil {
		return "absent"
	}
	var parts []string
	if term.GetScaleNum() != term.GetScaleDen() {
		parts = append(parts, fmt.Sprintf("%g/%g", term.GetScaleNum(), term.GetScaleDen()))
	}
	for _, factor := range term.GetFactors() {
		if factor.GetExponent() == 1 {
			parts = append(parts, factor.GetUnitId())
			continue
		}
		parts = append(parts, fmt.Sprintf("%s^%g", factor.GetUnitId(), factor.GetExponent()))
	}
	if len(parts) == 0 {
		return "1"
	}
	return strings.Join(parts, "·")
}

func mustFloat(t *testing.T, ev expectedValue) float64 {
	t.Helper()

	f, ok := ev.Value.(float64)
	if !ok {
		t.Fatalf("%s: %v is not a number", ev.Kind, ev.Value)
	}
	return f
}
