package grpc

import (
	"strings"
	"testing"
)

// TestEvaluateRejectsTextAfterTheExpression: `1 = 1` is a declaration's
// notation, not an equality, and answering the 1 the parser did read would hide
// the rest of the request.
func TestEvaluateRejectsTextAfterTheExpression(t *testing.T) {
	for _, expression := range []string{"1 = 1", "1 == 1 extra", "2 + 3;"} {
		resp := evaluateIn(t, expression, "", "")
		if resp.Error == "" {
			t.Fatalf("Evaluate(%q) = %v, want an error naming the unread text",
				expression, resp.Result)
		}
		if !strings.Contains(resp.Error, "parse failed") {
			t.Errorf("Evaluate(%q) error = %q, want a parse failure", expression, resp.Error)
		}
	}
}

// TestEvaluateAcceptsSurroundingWhitespace: trailing space is not unread text.
func TestEvaluateAcceptsSurroundingWhitespace(t *testing.T) {
	resp := evaluateIn(t, "  1 + 2\n", "", "")
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if got := resp.Result.GetIntValue(); got != 3 {
		t.Errorf("1 + 2 = %v, want 3", got)
	}
}
