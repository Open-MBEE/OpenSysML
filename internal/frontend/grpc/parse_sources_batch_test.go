package grpc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A model of many documents is analyzed as one batch, as a workspace analyzes
// one: analyzing each document in a context of its own gathered the
// workspace-wide audits over every document again, so 320 documents took 15 s
// where the command line validates them in under one, and 800 did not finish
// in 10 s. The one unresolved reference is still reported, in the document
// that holds it.
func TestParseSourcesAnalyzesManyDocumentsPromptly(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	// Under the race detector a deadline says nothing about the algorithm, so
	// only the diagnostic is checked, on fewer documents.
	n, deadline := 800, 10*time.Second
	if raceBuild {
		n, deadline = 100, time.Hour
	}
	var parts []string
	for k := 0; k < n; k++ {
		var src strings.Builder
		fmt.Fprintf(&src, "package P%d {\n", k)
		if k > 0 {
			fmt.Fprintf(&src, "    private import P%d::*;\n    part q : D%d;\n", k-1, k-1)
		}
		fmt.Fprintf(&src, "    part def D%d { attribute a : ScalarValues::Real; }\n    part p : D%d;\n", k, k)
		if k == n-1 {
			src.WriteString("    part missing : Nowhere;\n")
		}
		src.WriteString("}\n")
		parts = append(parts, fmt.Sprintf("p%03d.sysml", k), src.String())
	}

	type result struct {
		resp *pb.ParseSourcesResponse
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: inlineDocuments(parts...)})
		done <- result{resp, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("ParseSources: %v", r.err)
		}
		if len(r.resp.Diagnostics) != 1 {
			t.Fatalf("got %d diagnostics, want the one unresolved reference: %v", len(r.resp.Diagnostics), r.resp.Diagnostics)
		}
		d := r.resp.Diagnostics[0]
		if d.Span.GetFile() != fmt.Sprintf("p%03d.sysml", n-1) || !strings.Contains(d.Message, "Nowhere") {
			t.Errorf("the diagnostic is %q in %q, want the unresolved Nowhere in the last document", d.Message, d.Span.GetFile())
		}
	case <-time.After(deadline):
		t.Fatalf("ParseSources of %d documents did not finish in %v", n, deadline)
	}
}
