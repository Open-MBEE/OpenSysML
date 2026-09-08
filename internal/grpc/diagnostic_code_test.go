package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"google.golang.org/protobuf/proto"
)

// codesOf lists the codes of diags, in order.
func codesOf(diags []*pb.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

// requireCode fails unless some diagnostic carries code, and every diagnostic
// carrying it is what its message says it is.
func requireCode(t *testing.T, diags []*pb.Diagnostic, code, messagePrefix string) {
	t.Helper()
	found := false
	for _, d := range diags {
		if d.Code != code {
			continue
		}
		found = true
		if !strings.HasPrefix(d.Message, messagePrefix) {
			t.Errorf("code %q on %q, want a message starting %q", code, d.Message, messagePrefix)
		}
	}
	if !found {
		t.Errorf("no diagnostic coded %q among %v", code, codesOf(diags))
	}
}

// requireEveryCoded fails when any diagnostic crosses without a code.
func requireEveryCoded(t *testing.T, diags []*pb.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if d.Code == "" {
			t.Errorf("diagnostic crossed uncoded: %s %q", d.Severity, d.Message)
		}
	}
}

const codedModel = `
package Coded {
  part def Wheel {
    part hub : Missing;
  }
}
`

// A parse reports a syntax error as "syntax" and a validation finding under the
// code its pass gave it, and GetDiagnostics reports the same codes.
func TestParseFile_DiagnosticCodes(t *testing.T) {
	srv := mustNewService(t, 10)

	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: codedModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	requireCode(t, parseResp.Diagnostics, "unresolved", "unresolved reference")
	requireEveryCoded(t, parseResp.Diagnostics)

	diagResp, err := srv.GetDiagnostics(context.Background(), &pb.DiagnosticsRequest{ModelHash: parseResp.ModelHash})
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if got, want := strings.Join(codesOf(diagResp.Diagnostics), ","), strings.Join(codesOf(parseResp.Diagnostics), ","); got != want {
		t.Errorf("GetDiagnostics codes = %s, want the parse's %s", got, want)
	}

	broken, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: "package P { part def "},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(broken.Diagnostics) == 0 {
		t.Fatal("a syntax error produced no diagnostic")
	}
	for _, d := range broken.Diagnostics {
		if d.Code != SyntaxDiagnosticCode {
			t.Errorf("syntax error coded %q, want %q: %s", d.Code, SyntaxDiagnosticCode, d.Message)
		}
	}
}

// A document set's diagnostics carry their codes as one document's do: a
// validation code when the set parses, "syntax" when a document does not.
func TestParseSources_DiagnosticCodes(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("coded.sysml", codedModel, "empty.sysml", "package Empty {}"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	requireCode(t, resp.Diagnostics, "unresolved", "unresolved reference")
	requireEveryCoded(t, resp.Diagnostics)

	resp, err = srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("coded.sysml", codedModel, "broken.sysml", "package P { part def "),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	requireCode(t, resp.Diagnostics, SyntaxDiagnosticCode, "")
	requireEveryCoded(t, resp.Diagnostics)
}

// An expression that does not parse is refused with "syntax" diagnostics.
func TestEvaluate_DiagnosticCodes(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, "package P {}")

	resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{ModelHash: hash, Expression: "1 +"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if resp.Error == "" || len(resp.Diagnostics) == 0 {
		t.Fatalf("an unparsable expression evaluated: %v", resp)
	}
	for _, d := range resp.Diagnostics {
		if d.Code != SyntaxDiagnosticCode {
			t.Errorf("syntax error coded %q, want %q: %s", d.Code, SyntaxDiagnosticCode, d.Message)
		}
	}
}

// An action run reports its choice points as "choice-point" and the guards it
// could not evaluate as "guard-unevaluable", whether or not the choice was
// located in the model.
func TestExecuteAction_DiagnosticCodes(t *testing.T) {
	srv := mustNewService(t, 10)

	content := `
package Test {
  action tally {
    attribute leftCount : Integer = 0;
    attribute rightCount : Integer = 0;
    first start;
    fork split;
    action left { assign leftCount := leftCount + 1; }
    action right { assign rightCount := rightCount + 10; }
    join sync;
    done;
    succession first start then split;
    succession first split then left;
    succession first split then right;
    succession first left then sync;
    succession first right then sync;
    succession first sync then done;
  }
  action route {
    attribute level : Integer = 75;
    attribute handler : Integer = 0;
    first start;
    then decide select;
      if level > 50 then warn;
      if 1 / (level - 75) > 0 then alarm;
    action warn { assign handler := 1; }
    then done;
    action alarm { assign handler := 2; }
    then done;
  }
}
`
	hash := mustParse(t, srv, content)

	resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: "Test::tally"})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("execution error: %s", resp.Error)
	}
	requireCode(t, resp.Diagnostics, runtime.ChoiceDiagnosticCode, "choice point: ")
	requireEveryCoded(t, resp.Diagnostics)

	resp, err = srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: "Test::route"})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("execution error: %s", resp.Error)
	}
	requireCode(t, resp.Diagnostics, runtime.UnevaluableGuardCode, "guard not evaluable: ")
	requireEveryCoded(t, resp.Diagnostics)
}

// A state run's choice among transitions is coded "choice-point".
func TestExecuteState_DiagnosticCodes(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, `
package Test {
  state Dispatcher {
    attribute level : Integer = 8;
    entry; then idle;
    state idle;
    state low;
    state high;
    transition first idle accept Go if level > 5 then low;
    transition first idle accept Go if level > 7 then high;
  }
}
`)

	resp, err := srv.ExecuteState(context.Background(), &pb.ExecuteStateRequest{
		ModelHash: hash, StateMachineSymbolId: "Test::Dispatcher", Events: []string{"Go"},
	})
	if err != nil {
		t.Fatalf("ExecuteState: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("execution error: %s", resp.Error)
	}
	requireCode(t, resp.Diagnostics, runtime.ChoiceDiagnosticCode, "choice point: ")
	requireEveryCoded(t, resp.Diagnostics)
}

// An analysis run's choice points, among tokens and among writes, are coded
// "choice-point".
func TestRunAnalysis_DiagnosticCodes(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, `
package An {
	private import ScalarValues::*;

	action def Tally {
		out r : Integer = 0;
		first start;
		fork split;
		action left { assign r := r + 1; }
		action right { assign r := r + 10; }
		join sync;
		done;
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}

	analysis forked {
		out r : Integer;
		perform action tally : Tally;
		return : Integer = r;
	}
}
`, "analysis-choice-codes")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "An::forked"})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis reported %q", resp.Error)
	}
	if len(resp.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %v, want the two choice points", resp.Diagnostics)
	}
	requireCode(t, resp.Diagnostics, runtime.ChoiceDiagnosticCode, "choice point: ")
	requireEveryCoded(t, resp.Diagnostics)
}

// A conversion refused for a syntax error codes each diagnostic "syntax",
// with a span and without one alike.
func TestConvert_DiagnosticCodes(t *testing.T) {
	srv := mustNewService(t, 10)

	for name, req := range map[string]*pb.ConvertRequest{
		"notation": {
			Source:     &pb.ConvertRequest_Content{Content: "package P { part def "},
			FromFormat: "sysml",
			ToFormat:   "ttl",
		},
		"turtle": {
			Source:     &pb.ConvertRequest_Content{Content: "@prefix : <http://example.org/> .\n:a :b"},
			FromFormat: "ttl",
			ToFormat:   "sysml",
		},
	} {
		resp, err := srv.Convert(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: Convert: %v", name, err)
		}
		if resp.Error == "" || len(resp.Diagnostics) == 0 {
			t.Fatalf("%s: unreadable input converted without diagnostics: %v", name, resp)
		}
		for _, d := range resp.Diagnostics {
			if d.Code != SyntaxDiagnosticCode {
				t.Errorf("%s: syntax error coded %q, want %q: %s", name, d.Code, SyntaxDiagnosticCode, d.Message)
			}
		}
	}
}

// An edit refused for what it would make of the model reports the diagnostics
// that refused it with their codes.
func TestApplyEdits_DiagnosticCodes(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, editModelSource)

	for _, tc := range []struct {
		name  string
		value string
		code  string
	}{
		{"value does not parse", "1050.0[", SyntaxDiagnosticCode},
		{"value does not resolve", "nosuchFeature", "unresolved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
				ModelHash:  hash,
				Operations: []*pb.EditOperation{setValueOp("Demo::SC::unitMass", tc.value)},
			})
			if err != nil {
				t.Fatalf("ApplyEdits: %v", err)
			}
			if resp.Error == "" || len(resp.Diagnostics) == 0 {
				t.Fatalf("refusal carries no diagnostics: %v", resp)
			}
			requireCode(t, resp.Diagnostics, tc.code, "")
			requireEveryCoded(t, resp.Diagnostics)
		})
	}
}

// Without "diagnostic_codes" every response family still reports its
// diagnostics, identical but for the code, which is withheld; with it every
// code is populated, so an empty one is a finding none was assigned.
func TestDiagnosticCodes_Capability(t *testing.T) {
	ctx := context.Background()
	current := mustNewService(t, 10)
	withheld := mustNewServiceWithout(t, CapabilityDiagnosticCodes)

	stateMachine := `
package Test {
  state Dispatcher {
    attribute level : Integer = 8;
    entry; then idle;
    state idle;
    state low;
    state high;
    transition first idle accept Go if level > 5 then low;
    transition first idle accept Go if level > 7 then high;
  }
}
`
	families := map[string]func(srv *Service) []*pb.Diagnostic{
		"ParseFile": func(srv *Service) []*pb.Diagnostic {
			resp, err := srv.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: codedModel}})
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			return resp.Diagnostics
		},
		"GetDiagnostics": func(srv *Service) []*pb.Diagnostic {
			resp, err := srv.GetDiagnostics(ctx, &pb.DiagnosticsRequest{ModelHash: mustParse(t, srv, codedModel)})
			if err != nil {
				t.Fatalf("GetDiagnostics: %v", err)
			}
			return resp.Diagnostics
		},
		"ParseSources": func(srv *Service) []*pb.Diagnostic {
			resp, err := srv.ParseSources(ctx, &pb.ParseSourcesRequest{
				Documents: inlineDocuments("coded.sysml", codedModel, "broken.sysml", "package P { part def "),
			})
			if err != nil {
				t.Fatalf("ParseSources: %v", err)
			}
			return resp.Diagnostics
		},
		"Evaluate": func(srv *Service) []*pb.Diagnostic {
			resp, err := srv.Evaluate(ctx, &pb.EvaluateRequest{ModelHash: mustParse(t, srv, "package P {}"), Expression: "1 +"})
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			return resp.Diagnostics
		},
		"ExecuteState": func(srv *Service) []*pb.Diagnostic {
			resp, err := srv.ExecuteState(ctx, &pb.ExecuteStateRequest{
				ModelHash: mustParse(t, srv, stateMachine), StateMachineSymbolId: "Test::Dispatcher", Events: []string{"Go"},
			})
			if err != nil {
				t.Fatalf("ExecuteState: %v", err)
			}
			return resp.Diagnostics
		},
		"Convert": func(srv *Service) []*pb.Diagnostic {
			resp, err := srv.Convert(ctx, &pb.ConvertRequest{
				Source: &pb.ConvertRequest_Content{Content: "package P { part def "}, FromFormat: "sysml", ToFormat: "ttl",
			})
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			return resp.Diagnostics
		},
		"ApplyEdits": func(srv *Service) []*pb.Diagnostic {
			resp, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
				ModelHash:  mustParsedModel(t, srv, editModelSource),
				Operations: []*pb.EditOperation{setValueOp("Demo::SC::unitMass", "nosuchFeature")},
			})
			if err != nil {
				t.Fatalf("ApplyEdits: %v", err)
			}
			return resp.Diagnostics
		},
	}

	for name, call := range families {
		t.Run(name, func(t *testing.T) {
			coded := call(current)
			uncoded := call(withheld)
			if len(coded) == 0 {
				t.Fatal("no diagnostics to compare")
			}
			requireEveryCoded(t, coded)
			if len(uncoded) != len(coded) {
				t.Fatalf("withheld service reported %d diagnostics, want %d", len(uncoded), len(coded))
			}
			for i, d := range uncoded {
				if d.Code != "" {
					t.Errorf("withheld service reported code %q", d.Code)
				}
				want := proto.Clone(coded[i]).(*pb.Diagnostic)
				want.Code = ""
				if !proto.Equal(d, want) {
					t.Errorf("withheld diagnostic = %v, want %v", d, want)
				}
			}
		})
	}
}
