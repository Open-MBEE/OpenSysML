package runtimetools

import (
	"strings"
	"testing"
)

// TestConvertIncludesCheckDiagnostics checks that convert passes
// includeCheckDiagnostics through, so the check's findings come back with the
// converted document, and that without it only the document comes back.
func TestConvertIncludesCheckDiagnostics(t *testing.T) {
	h := attach(t, newService(t))
	args := map[string]any{"content": "package P { part x : NoSuchType; }", "fromFormat": "sysml", "toFormat": "api-json"}

	res := h.call("convert", args)
	if res.IsError || strings.Contains(text(res), "unresolved") {
		t.Fatalf("convert without includeCheckDiagnostics: error %v, text %s", res.IsError, text(res))
	}

	args["includeCheckDiagnostics"] = true
	res = h.call("convert", args)
	if res.IsError {
		t.Fatalf("convert with includeCheckDiagnostics failed: %s", text(res))
	}
	for _, want := range []string{`"@type"`, "error: unresolved reference: NoSuchType [unresolved]"} {
		if !strings.Contains(text(res), want) {
			t.Errorf("the conversion is missing %s:\n%s", want, text(res))
		}
	}
}
