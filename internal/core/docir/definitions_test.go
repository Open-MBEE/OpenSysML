package docir

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const definitionsModel = `
	part def Sub;
	part telescope {
		part <'M1'> mirror : Sub {
			doc /* The primary mirror. */
			doc /* Collects light. */
		}
		part <'M2'> secondary : Sub {
			doc /*
			     * Folds the beam
			     * toward the focal plane.
			     */
		}
		part mount : Sub;
	}
	calc def Documented :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = WhereType(source = OwnedElements(source = root), type = "Observatory::Sub"),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("shortName", "name", "documentation")
		)
	}
`

// TestEvaluateDefinitions locks one entry per row: the term column names it,
// every description value becomes one run, absent values leave no runs, and
// each run keeps the query value it came from.
func TestEvaluateDefinitions(t *testing.T) {
	fixture := loadEvaluationFixture(t, definitionsModel+`
		part def Report :> Document {
			attribute redefines title = "Report";
			part glossary : Definitions {
				attribute redefines term = "shortName";
				attribute redefines description = "documentation";
				calc rows : Documented {
					in root = telescope;
				}
			}
		}
	`)
	document := fixture.mustEvaluate(t, "Report")
	content := document.Content()[0]
	if content.Kind() != ContentDefinitions || content.Name() != "glossary" {
		t.Fatalf("content = %s %q", content.Kind(), content.Name())
	}
	if content.Query() != "Observatory::Documented" || !content.QueryOrigin().Located() {
		t.Fatalf("query = %q origin %+v", content.Query(), content.QueryOrigin())
	}
	entries := content.Definitions()
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	want := []struct {
		term        []string
		description []string
		element     string
	}{
		{[]string{"M1"}, []string{"The primary mirror.", "Collects light."}, "mirror"},
		{nil, nil, "mount"},
		{[]string{"M2"}, []string{"Folds the beam\ntoward the focal plane."}, "secondary"},
	}
	for i, entry := range entries {
		if got := runTexts(entry.Term()); !equalStrings(got, want[i].term) {
			t.Fatalf("entry %d term = %q, want %q", i, got, want[i].term)
		}
		if got := runTexts(entry.Description()); !equalStrings(got, want[i].description) {
			t.Fatalf("entry %d description = %q, want %q", i, got, want[i].description)
		}
		element, ok := entry.Element().Element()
		if !ok || fixture.model.EffectiveNameOf(element) != want[i].element {
			t.Fatalf("entry %d element = %v, want %s", i, entry.Element(), want[i].element)
		}
		if !entry.Origin().Located() {
			t.Fatalf("entry %d has no origin", i)
		}
		for _, run := range append(entry.Term(), entry.Description()...) {
			if run.Kind() != RunPlain || !run.Origin().Located() {
				t.Fatalf("entry %d run %+v must be plain and located", i, run)
			}
		}
	}
}

// TestEvaluateDefinitionsIsImmutable locks that entries handed out are copies.
func TestEvaluateDefinitionsIsImmutable(t *testing.T) {
	fixture := loadEvaluationFixture(t, definitionsModel+`
		part def Report :> Document {
			attribute redefines title = "Report";
			part glossary : Definitions {
				attribute redefines term = "shortName";
				attribute redefines description = "documentation";
				calc rows : Documented {
					in root = telescope;
				}
			}
		}
	`)
	document := fixture.mustEvaluate(t, "Report")
	entries := document.Content()[0].Definitions()
	entries[0].term[0].text = "mutated"
	entries[0].description[0].text = "mutated"
	entries[0] = Definition{}
	fresh := document.Content()[0].Definitions()
	if fresh[0].Term()[0].Text() != "M1" || fresh[0].Description()[0].Text() != "The primary mirror." {
		t.Fatalf("entries were mutated: %+v", fresh[0])
	}
}

// TestEvaluateDefinitionsReportsUnknownColumn locks the typed failure when a
// parameter-driven projection lacks a column the block names.
func TestEvaluateDefinitionsReportsUnknownColumn(t *testing.T) {
	fixture := loadEvaluationFixture(t, definitionsModel+`
		calc def Dynamic :> Query {
			in root : Element;
			in properties : String[1..*];
			Project(source = OwnedElements(source = root), properties = properties)
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part glossary : Definitions {
				attribute redefines term = "name";
				attribute redefines description = "documentation";
				calc rows : Dynamic {
					in root = telescope;
					in properties = ("name", "shortName");
				}
			}
		}
	`)
	_, err := fixture.evaluate(t, "Report")
	var evaluation *Error
	if !errors.As(err, &evaluation) {
		t.Fatalf("error = %v, want *docir.Error", err)
	}
	if evaluation.Kind != ErrorUnknownDefinitionColumn {
		t.Fatalf("kind = %s, want %s", evaluation.Kind, ErrorUnknownDefinitionColumn)
	}
	if evaluation.Column != "documentation" || evaluation.Query != "Observatory::Dynamic" || evaluation.Content != "glossary" {
		t.Fatalf("error = %+v", evaluation)
	}
	if evaluation.Origin == (symbols.Origin{}) || !evaluation.Origin.Located() {
		t.Fatalf("error has no origin: %+v", evaluation)
	}
}
