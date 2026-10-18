package runtimetools

import (
	"strings"
	"testing"
)

// TestExplainDiagnosticAnswersACode checks that explain_diagnostic prints the
// catalogue entry of a code as text and as structured content, lists the
// catalogue when no code is given, and refuses a code the catalogue lacks.
func TestExplainDiagnosticAnswersACode(t *testing.T) {
	h := attach(t, newService(t))

	res := h.call("explain_diagnostic", map[string]any{"code": "unresolved"})
	if res.IsError {
		t.Fatalf("explain_diagnostic failed: %s", text(res))
	}
	for _, want := range []string{"unresolved: error (name-resolution)", "Specification:"} {
		if !strings.Contains(text(res), want) {
			t.Errorf("explanation is missing %q:\n%s", want, text(res))
		}
	}
	structured, _ := res.StructuredContent.(map[string]any)
	if structured["code"] != "unresolved" || structured["severity"] != "error" || structured["tier"] != "name-resolution" ||
		structured["summary"] == "" || structured["meaning"] == "" {
		t.Errorf("structured content = %v", structured)
	}

	res = h.call("explain_diagnostic", map[string]any{})
	if res.IsError {
		t.Fatalf("explain_diagnostic with no code failed: %s", text(res))
	}
	if !strings.Contains(text(res), "unresolved") || !strings.Contains(text(res), "Syntax") {
		t.Errorf("the listing does not table the codes:\n%s", text(res))
	}
	structured, _ = res.StructuredContent.(map[string]any)
	if list, _ := structured["codes"].([]any); len(list) < 100 {
		t.Errorf("the listing has %d codes", len(list))
	}

	res = h.call("explain_diagnostic", map[string]any{"code": "no-such-code"})
	if !res.IsError || !strings.Contains(text(res), `unknown diagnostic code "no-such-code"`) {
		t.Errorf("an unknown code was not refused: %v %s", res.IsError, text(res))
	}
}
