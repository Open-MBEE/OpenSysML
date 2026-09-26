package docir

import "testing"

const widthsFixture = `
	part def Sub;
	part telescope {
		part optics : Sub;
		part mount : Sub;
	}
	calc def Named :> Query {
		in root : Element;
		Project(source = OwnedElements(source = root), properties = ("name", "documentation", "qualifiedName"))
	}
	calc def Nested :> Query {
		in root : Element;
		Tree(source = Descendants(source = root))
	}
	part def Report :> Document {
		attribute redefines title = "Report";
		part sized : Table {
			attribute redefines columnWidths = (206, 774);
			calc rows : Named { in root = telescope; }
		}
		part nested : Table {
			calc rows : Nested { in root = telescope; }
		}
	}
`

// TestEvaluateTableCarriesColumnWidthsAndRowDepth locks that a table's stated
// widths reach its columns by position, with the unstated remainder automatic,
// and that a tree query's row depths reach the evaluated rows.
func TestEvaluateTableCarriesColumnWidthsAndRowDepth(t *testing.T) {
	fixture := loadEvaluationFixture(t, widthsFixture)
	document := fixture.mustEvaluate(t, "Report")
	sized := document.Content()[0]
	widths := make([]int, 0, 3)
	for _, column := range sized.Columns() {
		widths = append(widths, column.Width())
	}
	if len(widths) != 3 || widths[0] != 206 || widths[1] != 774 || widths[2] != 0 {
		t.Fatalf("column widths = %v", widths)
	}
	nested := document.Content()[1]
	rows := nested.Rows()
	if len(rows) != 2 || rows[0].Depth() != 0 || rows[1].Depth() != 0 {
		t.Fatalf("flat descendants rows = %d", len(rows))
	}
	for _, column := range nested.Columns() {
		if column.Width() != 0 {
			t.Fatalf("unstated width = %d", column.Width())
		}
	}
}
