package docplan

import (
	"reflect"
	"testing"
)

// TestCompileTableColumnLabels locks that a table's columnLabels are read in
// order, that a single literal is one label, and that an unstated attribute
// leaves every column headed by its name.
func TestCompileTableColumnLabels(t *testing.T) {
	fixture := loadPlanningFixture(t, widthsBody+`
		part def Report :> Document {
			attribute redefines title = "Report";
			part labelled : Table {
				attribute redefines columnLabels = ("Item", "", "Text");
				calc rows : Named { in root = telescope; }
			}
			part single : Table {
				attribute redefines columnLabels = "Item";
				calc rows : Named { in root = telescope; }
			}
			part named : Table {
				calc rows : Named { in root = telescope; }
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
	if got := content[0].ColumnLabels(); !reflect.DeepEqual(got, []string{"Item", "", "Text"}) {
		t.Fatalf("labels = %v", got)
	}
	if got := content[1].ColumnLabels(); !reflect.DeepEqual(got, []string{"Item"}) {
		t.Fatalf("single label = %v", got)
	}
	if got := content[2].ColumnLabels(); got != nil {
		t.Fatalf("unstated labels = %v", got)
	}
}

// TestCompileTableRejectsInvalidColumnLabels locks that a label that is not a
// string literal is refused with its table named.
func TestCompileTableRejectsInvalidColumnLabels(t *testing.T) {
	for name, value := range map[string]string{
		"integer": `("Item", 10)`,
		"boolean": `true`,
	} {
		t.Run(name, func(t *testing.T) {
			fixture := loadPlanningFixture(t, widthsBody+`
				part def Report :> Document {
					attribute redefines title = "Report";
					part labelled : Table {
						attribute redefines columnLabels = `+value+`;
						calc rows : Named { in root = telescope; }
					}
				}
			`)
			_, err := fixture.compile(t, "Report")
			planning := planningError(t, err)
			if planning.Kind != ErrorInvalidColumnLabels || planning.Content != "Observatory::Report::labelled" {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
