package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
)

// Without a context an expression evaluates at the document root, where a
// package's import does not reach: library functions need their qualified name.
func TestEvaluateResolvesLibraryFunctionsInTheRequestedScope(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 4)
	parsed, err := srv.ParseFile(ctx, &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: complexWireModel},
		ContentHash: "complex-scope",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	bare := srv.mustEvaluateError(t, parsed.ModelHash, "rect(1.0, -1.0)", "")
	for _, want := range []string{"unresolved reference: rect", "ComplexFunctions::rect"} {
		if !strings.Contains(bare, want) {
			t.Errorf("Evaluate(rect) at the root: error %q does not mention %q", bare, want)
		}
	}

	for _, tc := range []struct{ expr, context string }{
		{"ComplexFunctions::rect(1.0, -1.0)", ""},
		{"rect(1.0, -1.0)", "C"},
	} {
		resp, err := srv.Evaluate(ctx, &pb.EvaluateRequest{ModelHash: parsed.ModelHash, Expression: tc.expr, ContextSymbolId: tc.context})
		if err != nil || resp.Error != "" {
			t.Fatalf("Evaluate(%s, context %q): err = %v, error = %q", tc.expr, tc.context, err, resp.GetError())
		}
		if resp.Result.GetComplex() == nil || protoconv.ProtoToComplex(resp.Result.GetComplex()) != complex(1, -1) {
			t.Errorf("Evaluate(%s, context %q) = %v, want complex 1.0 - 1.0i", tc.expr, tc.context, resp.Result)
		}
	}
}

// mustEvaluateError evaluates expr and returns the evaluation error it reports.
func (s *Service) mustEvaluateError(t *testing.T, modelHash, expr, contextID string) string {
	t.Helper()
	resp, err := s.Evaluate(context.Background(), &pb.EvaluateRequest{ModelHash: modelHash, Expression: expr, ContextSymbolId: contextID})
	if err != nil {
		t.Fatalf("Evaluate(%s): %v", expr, err)
	}
	if resp.Error == "" {
		t.Fatalf("Evaluate(%s) = %v, want an error", expr, resp.Result)
	}
	return resp.Error
}
