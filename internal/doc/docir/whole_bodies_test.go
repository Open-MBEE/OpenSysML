package docir

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
)

// A markup-born section evaluates as a part-born one does: nested under its
// parent with its title, an anchor on every section a reference names,
// in this document or from another, and the name its heading derived.
func TestEvaluateWholeBodySections(t *testing.T) {
	fixture := loadEvaluationFixture(t, `
		part def Notes :> Document {
			attribute redefines title = "Notes";
			rep language "markdown" /* # Shared notes
			 *
			 * The [budget](sysml:WholeReport::mass_budget) is kept in the report.
			 */
		}
		part def WholeReport :> Document {
			attribute redefines title = "Whole Report";
			rep language "markdown" /* See [](sysml:mass_budget), [](sysml:Notes::shared_notes) and [](sysml:appendix).
			 *
			 * # Overview
			 *
			 * ## Mass budget
			 *
			 * Mass budget
			 * -----------
			 *
			 * See [](sysml:mass_budget_2).
			 *
			 * ### Margins
			 *
			 * # Design
			 */
			part appendix : Section {
				attribute redefines title = "Appendix";
				rep language "markdown" /* ## Derivation
				 *
				 * Back to [](sysml:overview).
				 */
			}
		}
	`)
	document := fixture.mustEvaluate(t, "WholeReport")
	var lines []string
	var walk func(content []Content, depth int)
	walk = func(content []Content, depth int) {
		for _, node := range content {
			if node.Kind() != ContentSection {
				continue
			}
			lines = append(lines, strings.Repeat("  ", depth)+node.Name()+" "+node.Title()+" #"+node.Anchor())
			walk(node.Children(), depth+1)
		}
	}
	walk(document.Content(), 0)
	want := strings.TrimSpace(`
overview Overview #overview
  mass_budget Mass budget #overview-mass_budget
  mass_budget_2 Mass budget #overview-mass_budget_2
    margins Margins #
design Design #
appendix Appendix #appendix
  derivation Derivation #`)
	if got := strings.Join(lines, "\n"); got != want {
		t.Fatalf("sections:\n%s\nwant:\n%s", got, want)
	}
	notes, err := EvaluateLinked(fixture.plan(t, "Notes"), []*docplan.Plan{fixture.plan(t, "WholeReport")}, fixture.context(), queryexec.Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := notes.Content()[0]; got.Name() != "shared_notes" || got.Anchor() != "shared_notes" {
		t.Errorf("Notes section %q anchor %q, want shared_notes (referenced by the report)", got.Name(), got.Anchor())
	}
}
