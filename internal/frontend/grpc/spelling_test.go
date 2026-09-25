package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// quotedNamesModel declares a package whose name needs quoting, as the OMG
// training corpus does throughout.
const quotedNamesModel = `
package 'My Pkg' {
  part def Car {
    attribute m = 5.0;
  }
  action 'run it' {
    first start;
    done;
    succession first start then done;
  }
  state 'spin up' {
    entry; then init;
    state init;
    succession first init then done;
  }
}
`

// parseQuotedNamesModel parses the fixture and returns its model hash.
func parseQuotedNamesModel(t *testing.T, srv *Service, hash string) string {
	t.Helper()
	resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: quotedNamesModel},
		ContentHash: hash,
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	// The state notation here warns as an OpenSysML extension; only errors would
	// be a fixture defect.
	if errs := errorDiagnostics(resp.Diagnostics); len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	return resp.ModelHash
}

// errorDiagnostics returns only the error-severity diagnostics of diags.
func errorDiagnostics(diags []*pb.Diagnostic) []*pb.Diagnostic {
	var out []*pb.Diagnostic
	for _, d := range diags {
		if strings.EqualFold(d.Severity, "error") {
			out = append(out, d)
		}
	}
	return out
}

// One spelling rule holds on every surface: a symbol ID may be written the way a
// model author writes the name — quoted — as well as in the unquoted spelling the
// index records, which clients already send.
func TestInstantiateAcceptsBothSpellings(t *testing.T) {
	for _, id := range []string{"'My Pkg'::Car", "My Pkg::Car"} {
		// A service of its own per spelling: one object of the type per model, so
		// what is under test is the spelling, not a second materialization.
		srv := mustNewService(t, 10)
		hash := parseQuotedNamesModel(t, srv, "test-spelling-instantiate")
		resp, err := srv.Instantiate(context.Background(), &pb.InstantiateRequest{
			ModelHash: hash,
			SymbolId:  id,
		})
		if err != nil {
			t.Fatalf("Instantiate(%q) failed: %v", id, err)
		}
		if resp.Error != "" {
			t.Fatalf("Instantiate(%q) reported %q", id, resp.Error)
		}
		if resp.Instance == nil {
			t.Fatalf("Instantiate(%q) returned no instance", id)
		}
	}
}

// The same rule holds for every RPC that takes a symbol ID, so no surface is the
// odd one out.
func TestSymbolIDSpellingHoldsOnEveryRPC(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseQuotedNamesModel(t, srv, "test-spelling-rpcs")
	ctx := context.Background()

	symResp, err := srv.GetSymbol(ctx, &pb.GetSymbolRequest{ModelHash: hash, SymbolId: "'My Pkg'::Car"})
	if err != nil || symResp.Error != "" || symResp.Symbol == nil {
		t.Errorf("GetSymbol: err = %v, error = %q, symbol = %v", err, symResp.GetError(), symResp.GetSymbol())
	}

	evalResp, err := srv.Evaluate(ctx, &pb.EvaluateRequest{
		ModelHash:       hash,
		ContextSymbolId: "'My Pkg'::Car",
		Expression:      "m * 2",
	})
	if err != nil || evalResp.Error != "" {
		t.Errorf("Evaluate: err = %v, error = %q", err, evalResp.GetError())
	}

	actResp, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{
		ModelHash:      hash,
		ActionSymbolId: "'My Pkg'::'run it'",
	})
	if err != nil || actResp.Error != "" {
		t.Errorf("ExecuteAction: err = %v, error = %q", err, actResp.GetError())
	}

	stateResp, err := srv.ExecuteState(ctx, &pb.ExecuteStateRequest{
		ModelHash:            hash,
		StateMachineSymbolId: "'My Pkg'::'spin up'",
	})
	if err != nil || stateResp.Error != "" {
		t.Errorf("ExecuteState: err = %v, error = %q", err, stateResp.GetError())
	}
}

// A name nothing declares is still not found, whichever spelling asks for it, and
// a malformed ID is reported rather than panicking.
func TestSymbolIDSpellingDoesNotInventSymbols(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseQuotedNamesModel(t, srv, "test-spelling-missing")

	for _, id := range []string{"'No Pkg'::Car", "No Pkg::Car", "'My", "", "'My Pkg'::"} {
		resp, err := srv.Instantiate(context.Background(), &pb.InstantiateRequest{
			ModelHash: hash,
			SymbolId:  id,
		})
		if err != nil {
			t.Fatalf("Instantiate(%q) failed: %v", id, err)
		}
		if resp.Error == "" {
			t.Errorf("Instantiate(%q) reported no error", id)
		}
	}
}

// The quoting is notation, not part of the name: the ID a client sends is read as
// the name the index records.
func TestUnquotedName(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want string
		ok   bool
	}{
		{"'My Pkg'::Car", "My Pkg::Car", true},
		{"Demo::Vehicle", "Demo::Vehicle", true},
		{"Top::'My Pkg'::Car", "Top::My Pkg::Car", true},
		{"'part'::Widget", "part::Widget", true},
		{"", "", false},
		{"'My", "", false},
		{"Demo::", "", false},
		{"1 + 1", "", false},
	} {
		got, ok := unquotedName(tc.id)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("unquotedName(%q) = %q, %v; want %q, %v", tc.id, got, ok, tc.want, tc.ok)
		}
	}
}
