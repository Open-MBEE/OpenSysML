package docrender

import (
	"path/filepath"
	"strings"
	"testing"
)

const definitionsFixture = "telescope_report.sysml"

// TestMarkdownDefinitions checks the prose form of a definitions block: one
// paragraph per row, the term strong, an em dash before the description, the
// description's values space-joined with metacharacters escaped; an entry
// missing one side writes the other alone, and one missing both writes nothing.
func TestMarkdownDefinitions(t *testing.T) {
	got := renderFixtureDocument(t, filepath.Join("testdata", definitionsFixture), "Observatory::MassReport")
	want := "**M3\\|\\***\n\n" +
		"**M1** — The primary mirror assembly. Collects light \\| not \\*heat\\* from the target.\n\n" +
		"Actuators that phase the mirror segments.\n\n"
	if !strings.Contains(got, want) {
		t.Errorf("rendering does not contain\n%s\ngot:\n%s", want, got)
	}
	if strings.Contains(got, "\n\n\n") || strings.Contains(got, " — \n") || strings.Contains(got, "\n — ") {
		t.Errorf("an entry without term or description left a dangling separator or blank block:\n%s", got)
	}
	if strings.Contains(got, "missingNotes") || strings.Contains(got, "MissingSubsystems") {
		t.Errorf("a definitions block over an empty result left a trace:\n%s", got)
	}
}

// TestMarkdownDefinitionsEscaping checks a term cannot break out of its
// strong span or open a pipe, and a description cannot open a list.
func TestMarkdownDefinitionsEscaping(t *testing.T) {
	if got := delimited("**", "a**b|c"); got != `**a\*\*b\|c**` {
		t.Errorf("strong term = %q", got)
	}
	if got := blockStart(inline("- not a bullet")); got != `\- not a bullet` {
		t.Errorf("description block start = %q", got)
	}
}

// TestHTMLDefinitions checks the semantic form: a <dl> carrying the content
// facts, one <div> group per row carrying the row's element, its term in <dt>
// and its description values in one <dd>, every value escaped.
func TestHTMLDefinitions(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", definitionsFixture), "Observatory::MassReport", HTMLOptions{})
	for _, want := range []string{
		`<dl class="sysml-definitions" data-content="definitions" data-name="notes" data-query="Observatory::SubsystemNotes">`,
		`<div class="sysml-entry" data-element="Observatory::telescope::optics" data-element-kind="partUsage">` + "\n" +
			`<dt class="sysml-term">M1</dt>` + "\n" +
			`<dd class="sysml-description">The primary mirror assembly. Collects light | not *heat* from the target.</dd>` + "\n" +
			`</div>`,
		`<dt class="sysml-term">M3|*</dt>` + "\n" + `<dd class="sysml-description"></dd>`,
		`<dt class="sysml-term"></dt>` + "\n" + `<dd class="sysml-description">Actuators that phase the mirror segments.</dd>`,
		"</dl>\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
	// Every row is a group, so the list has as many groups as the query rows.
	if groups := strings.Count(got, `<div class="sysml-entry"`); groups != 4 {
		t.Errorf("got %d entry groups, want 4", groups)
	}
	if open, closed := strings.Count(got, "<dt "), strings.Count(got, "</dt>"); open != closed || open != 4 {
		t.Errorf("%d terms opened, %d closed, want 4", open, closed)
	}
	// An empty result keeps its list, as an empty <ul> does, so the block stays
	// addressable by name and query; it just has no groups.
	empty := `<dl class="sysml-definitions" data-content="definitions" data-name="missingNotes" data-query="Observatory::MissingSubsystems">` + "\n</dl>\n"
	if !strings.Contains(got, empty) {
		t.Errorf("rendering does not contain the empty block %q\n%s", empty, got)
	}
}
