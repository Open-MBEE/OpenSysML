package runtimetools

import (
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func TestConvertLibraryReferencesAndLegacyStubs(t *testing.T) {
	h := attach(t, newService(t))
	args := map[string]any{"content": "package P { private import SI::*; attribute x = 20 [s]; }", "fromFormat": "sysml", "toFormat": "api-json"}
	for _, legacy := range []bool{false, true} {
		args["includeLibraryStubs"] = legacy
		var response pb.ConvertResponse
		h.ok("convert", args, &response)
		if response.Error != "" || len(response.LibraryReferences) == 0 {
			t.Fatalf("legacy=%v: %v", legacy, &response)
		}
		var elements []struct {
			Library bool `json:"isLibraryElement"`
		}
		if err := json.Unmarshal([]byte(response.Content), &elements); err != nil {
			t.Fatal(err)
		}
		stubs := 0
		for _, element := range elements {
			if element.Library {
				stubs++
			}
		}
		if (stubs > 0) != legacy {
			t.Fatalf("legacy=%v: got %d library stubs", legacy, stubs)
		}
	}
}

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
