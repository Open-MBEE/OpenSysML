package docrender

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestElementValuesPrintEffectiveNames locks that an element value — here a
// parameter named by the one it redefines — prints its effective name in both
// backends, the name the `name` property reads, not its qualified name.
func TestElementValuesPrintEffectiveNames(t *testing.T) {
	path := filepath.Join("testdata", "effective_names.sysml")
	markdown := renderFixtureDocument(t, path, "Observatory::ParameterReport")
	if !strings.Contains(markdown, "| scene |") {
		t.Fatalf("Markdown does not print the parameter by its effective name:\n%s", markdown)
	}
	html := renderFixtureHTML(t, path, "Observatory::ParameterReport", HTMLOptions{Fragment: true})
	if !strings.Contains(html, ">scene</span>") {
		t.Fatalf("HTML does not print the parameter by its effective name:\n%s", html)
	}
}
