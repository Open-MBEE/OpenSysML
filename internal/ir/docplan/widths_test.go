package docplan

import (
	"reflect"
	"testing"
)

const widthsBody = `
	part telescope;
	calc def Named :> Query {
		in root : Element;
		Project(source = OwnedElements(source = root), properties = ("name", "documentation"))
	}
`

// TestCompileTableColumnWidths locks that a table's columnWidths are read in
// order, that a single literal is one width, and that an unstated attribute
// leaves every column automatic.
func TestCompileTableColumnWidths(t *testing.T) {
	fixture := loadPlanningFixture(t, widthsBody+`
		part def Report :> Document {
			attribute redefines title = "Report";
			part sized : Table {
				attribute redefines columnWidths = (206, 0, 105);
				calc rows : Named { in root = telescope; }
			}
			part single : Table {
				attribute redefines columnWidths = 35;
				calc rows : Named { in root = telescope; }
			}
			part automatic : Table {
				calc rows : Named { in root = telescope; }
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
	if got := content[0].ColumnWidths(); !reflect.DeepEqual(got, []int{206, 0, 105}) {
		t.Fatalf("sized widths = %v", got)
	}
	if got := content[1].ColumnWidths(); !reflect.DeepEqual(got, []int{35}) {
		t.Fatalf("single width = %v", got)
	}
	if got := content[2].ColumnWidths(); got != nil {
		t.Fatalf("automatic widths = %v", got)
	}
}

// TestCompileTableRejectsInvalidColumnWidths locks that a width that is not a
// non-negative integer literal is refused with its table named.
func TestCompileTableRejectsInvalidColumnWidths(t *testing.T) {
	for name, value := range map[string]string{
		"string":   `("wide", 10)`,
		"negative": `(10, -1)`,
		"real":     `10.5`,
	} {
		t.Run(name, func(t *testing.T) {
			fixture := loadPlanningFixture(t, widthsBody+`
				part def Report :> Document {
					attribute redefines title = "Report";
					part sized : Table {
						attribute redefines columnWidths = `+value+`;
						calc rows : Named { in root = telescope; }
					}
				}
			`)
			_, err := fixture.compile(t, "Report")
			planning := planningError(t, err)
			if planning.Kind != ErrorInvalidColumnWidths || planning.Content != "Observatory::Report::sized" {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
