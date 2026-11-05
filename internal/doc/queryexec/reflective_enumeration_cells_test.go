package queryexec

import (
	"slices"
	"testing"
)

// A cell and a WhereFeature over an enumeration-valued property read the library literal: a cell
// compares it with a named literal, KerML's or SysML's, and the filter keeps it by its name.
func TestExecuteReadsReflectiveDirectionAsTheLibraryLiteral(t *testing.T) {
	fixture := loadExecutionFixture(t, `
action def Exposure { in shutter : Boolean; out frame : Real; inout gain : Real; }
part def Camera { snapshot shot; timeslice run; }
calc def Q :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "In", cell = { in row : KerML::Core::Feature;
			row.direction == KerML::Core::FeatureDirectionKind::'in'
		}))
	)
}
calc def Portions :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "Snapshot", cell = { in row : SysML::Systems::OccurrenceUsage;
			row.portionKind == SysML::Systems::PortionKind::snapshot
		}))
	)
}
calc def Inputs :> Query {
	in root : Element;
	Project(
		source = WhereFeature(source = Descendants(source = root, maxDepth = 1),
			feature = "direction", operator = "=", value = "in"),
		properties = ("name")
	)
}
`)
	result := quantityRows(t, fixture, "Q", "Exposure")
	if got := cellTexts(t, result, 0); !slices.Equal(got, []string{"shutter", "frame", "gain"}) {
		t.Fatalf("name cells = %q", got)
	}
	if got := cellBooleans(t, result, "In"); !slices.EqualFunc(got, [][]bool{{true}, {false}, {false}}, slices.Equal) {
		t.Errorf("direction == FeatureDirectionKind::'in' cells = %v, want true for shutter only", got)
	}
	portions := quantityRows(t, fixture, "Portions", "Camera")
	if got := cellBooleans(t, portions, "Snapshot"); !slices.EqualFunc(got, [][]bool{{true}, {false}}, slices.Equal) {
		t.Errorf("portionKind == PortionKind::snapshot cells = %v, want true for shot only", got)
	}
	if got := cellTexts(t, quantityRows(t, fixture, "Inputs", "Exposure"), 0); !slices.Equal(got, []string{"shutter"}) {
		t.Errorf("WhereFeature direction = in keeps %q, want [shutter]", got)
	}
}
