package export_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// apiJSONOf converts a model text to the API element form, keyed by "@id".
func apiJSONOf(t *testing.T, text string) map[string]map[string]any {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return elementsByID(apiJSON(t, path))
}

// libraryID is the element id the library catalog gives the element named name.
func libraryID(t *testing.T, name string) string {
	t.Helper()
	element, ok := identity.LibraryCatalogWithOpenSysML(libs.SharedBase()).ElementNamed(name)
	if !ok {
		t.Fatalf("the library catalog has no %s", name)
	}
	return element.ID
}

// The element form answers Type::input as the reader does (KerML 8.3.3.1.10): a send action's payload
// and sender parameters and the inherited receiver; the graph's narrower statement is a mapping value.
func TestAPIJSONSendActionInputListsItsInputParameters(t *testing.T) {
	elements := apiJSONOf(t, `package P {
	attribute def Sig { attribute g; }
	port def SP;
	part def T {
		port sp : SP;
		attribute gain;
		action sendRef send new Sig(gain) via sp;
	}
}`)
	send := elements["P__T__sendRef"]
	owned := []string{"P__T__sendRef___400", "P__T__sendRef___401"}
	if got := refIDs(send["parameter"]); !slices.Equal(got, owned) {
		t.Fatalf("parameter = %v, want %v", got, owned)
	}
	want := append(slices.Clone(owned), libraryID(t, "Transfers::SendPerformance::receiver"))
	if got := refIDs(send["input"]); !slices.Equal(got, want) {
		t.Errorf("input = %v, want %v", got, want)
	}
	if got := refIDs(send["directedFeature"]); !slices.Equal(got, want) {
		t.Errorf("directedFeature = %v, want %v as input", got, want)
	}
}
