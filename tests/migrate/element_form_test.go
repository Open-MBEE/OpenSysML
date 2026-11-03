package migrate_test

import (
	"bytes"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A document in XMI element serialization — scalar properties and references
// written as child elements (XMI 2.5.1 § 9.4.4) — migrates to the notation
// and the report its attribute-form twin does.
func TestElementFormMigratesAsAttributeForm(t *testing.T) {
	element := migrateFixtureFile(t, "element_form")
	attribute := migrateFixtureFile(t, "attribute_form")
	checkGolden(t, "testdata/xmi/element_form.golden.sysml", element.Notation)
	if !bytes.Equal(element.Notation, attribute.Notation) {
		t.Errorf("notation differs between the forms:\n--- element form\n%s\n--- attribute form\n%s", element.Notation, attribute.Notation)
	}
	if len(element.Report.Entries) != len(attribute.Report.Entries) {
		t.Fatalf("report entries: %d in element form, %d in attribute form", len(element.Report.Entries), len(attribute.Report.Entries))
	}
	for i, got := range element.Report.Entries {
		if want := attribute.Report.Entries[i]; got != want {
			t.Errorf("report entry %d differs:\n element form   %+v\n attribute form %+v", i, got, want)
		}
	}
	if got, want := element.Report.Summary(), attribute.Report.Summary(); got != want {
		t.Errorf("summary = %q, attribute form %q", got, want)
	}
	// The model's elements all migrate, none approximated for its serialization.
	for _, e := range element.Report.Entries {
		if e.Verdict != migrate.Mapped {
			t.Errorf("%s (%s %s): %s %s", e.ID, e.Kind, e.Name, e.Verdict, e.Note)
		}
	}
}
