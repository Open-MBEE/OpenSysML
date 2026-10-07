package grpc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/modelform"
)

func behaviorModel(t *testing.T, srv *Service) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "conformance", "fixtures", "behavior.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := parseContent(t, srv, string(content))
	return parsed.ModelHash
}

func TestExportGraphsWritesTheCanonicalForm(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	hash := behaviorModel(t, srv)

	for _, subject := range []string{"Test::race", "Test::Machine"} {
		resp, err := srv.ExportGraphs(context.Background(), &pb.ExportGraphsRequest{ModelHash: hash, Subject: subject})
		if err != nil {
			t.Fatalf("ExportGraphs %s: %v", subject, err)
		}
		if resp.Version != modelform.GraphsVersion || resp.Subject != subject {
			t.Errorf("%s: version %d subject %q, want %d and %q", subject, resp.Version, resp.Subject, modelform.GraphsVersion, subject)
		}
		if !strings.HasSuffix(resp.Content, "\n") || strings.HasSuffix(resp.Content, "\n\n") {
			t.Errorf("%s: content must end in exactly one newline: %q", subject, resp.Content[len(resp.Content)-3:])
		}
		var graphs modelform.Graphs
		if err := json.Unmarshal([]byte(resp.Content), &graphs); err != nil {
			t.Fatalf("%s: content is not graphs JSON: %v", subject, err)
		}
		if graphs.Version != modelform.GraphsVersion || graphs.Subject != subject {
			t.Errorf("%s: JSON says version %d subject %q", subject, graphs.Version, graphs.Subject)
		}
		switch subject {
		case "Test::race":
			if len(graphs.Actions) != 1 || len(graphs.States) != 0 {
				t.Errorf("%s: %d actions and %d states, want one action", subject, len(graphs.Actions), len(graphs.States))
			}
		case "Test::Machine":
			if len(graphs.Actions) != 0 || len(graphs.States) != 1 {
				t.Errorf("%s: %d actions and %d states, want one state machine", subject, len(graphs.Actions), len(graphs.States))
			}
		}
	}

	again, err := srv.ExportGraphs(context.Background(), &pb.ExportGraphsRequest{ModelHash: hash, Subject: "Test::race"})
	if err != nil {
		t.Fatal(err)
	}
	first, _ := srv.ExportGraphs(context.Background(), &pb.ExportGraphsRequest{ModelHash: hash, Subject: "Test::race"})
	if again.Content != first.Content {
		t.Error("two exports of one subject differ; the form must be deterministic")
	}
}

func TestExportGraphsRefusals(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	hash := behaviorModel(t, srv)

	tests := []struct {
		name    string
		req     *pb.ExportGraphsRequest
		code    connect.Code
		message string
	}{
		{"unknown model", &pb.ExportGraphsRequest{ModelHash: "nope", Subject: "Test::race"}, connect.CodeNotFound, "nope"},
		{"no subject", &pb.ExportGraphsRequest{ModelHash: hash}, connect.CodeInvalidArgument, "subject is required"},
		{"unknown subject", &pb.ExportGraphsRequest{ModelHash: hash, Subject: "Test::Missing"}, connect.CodeNotFound, "symbol not found: Test::Missing"},
		{"not a behavior", &pb.ExportGraphsRequest{ModelHash: hash, Subject: "Test"}, connect.CodeInvalidArgument, "no lowered graph"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := srv.ExportGraphs(context.Background(), test.req)
			if connect.CodeOf(err) != test.code {
				t.Fatalf("status = %s, want %s: %v", connect.CodeOf(err), test.code, err)
			}
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Errorf("error %v does not mention %q", err, test.message)
			}
		})
	}
}

// A name the model declares twice denotes no one behavior, so neither graph
// is exported for it; a name of the model's shadows the library's.
func TestExportGraphsRefusesAnAmbiguousSubject(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	parsed, _ := parseContent(t, srv, `package P {
    action Run { action a; }
    action Run { action b; }
    action Base { action c; }
}
`)
	_, err := srv.ExportGraphs(context.Background(), &pb.ExportGraphsRequest{ModelHash: parsed.ModelHash, Subject: "P::Run"})
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "P::Run is ambiguous") {
		t.Fatalf("ambiguous subject: status %s: %v", connect.CodeOf(err), err)
	}
	resp, err := srv.ExportGraphs(context.Background(), &pb.ExportGraphsRequest{ModelHash: parsed.ModelHash, Subject: "P::Base"})
	if err != nil {
		t.Fatalf("P::Base: %v", err)
	}
	if resp.Subject != "P::Base" {
		t.Errorf("subject = %q", resp.Subject)
	}
}
