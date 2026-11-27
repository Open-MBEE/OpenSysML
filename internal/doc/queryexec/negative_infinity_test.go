package queryexec

import (
	"math"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// TestConstantValueNegativeInfinity: `-*` is a Real cell printing as written,
// while `*` stays the unbounded value.
func TestConstantValueNegativeInfinity(t *testing.T) {
	negative, ok := constantValue(semantics.NegativeInfinity())
	if !ok || negative.Kind() != ValueReal || !math.IsInf(negative.real, -1) {
		t.Fatalf("constantValue(-*) = %+v, %v; want the real -Inf", negative, ok)
	}
	if text, ok := negative.Text(nil); !ok || text != "-*" {
		t.Errorf("text of -* = %q, %v", text, ok)
	}
	positive, ok := constantValue(semantics.PositiveInfinity())
	if !ok || positive.Kind() != ValueInfinity {
		t.Fatalf("constantValue(*) = %+v, %v; want the unbounded value", positive, ok)
	}
	if text, ok := positive.Text(nil); !ok || text != "*" {
		t.Errorf("text of * = %q, %v", text, ok)
	}
}

// TestExecuteProjectsDerivedInfinities: a derived feature whose value is
// computed to an infinity is a cell of that infinity, either sign.
func TestExecuteProjectsDerivedInfinities(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Limits {
	attribute bound : Real = 3.5;
	attribute floor : Real = bound - *;
	attribute ceiling : Real = bound + *;
}
part def Holder {
	part limits : Limits;
}
calc def Bounds :> Query {
	in root : Element[1];
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name", "floor", "ceiling")
	)
}
`)
	result := quantityRows(t, fixture, "Bounds", "Holder")
	for column, want := range map[int]string{1: "-*", 2: "*"} {
		values := result.Rows()[0].Cells()[column].Values()
		if len(values) != 1 {
			t.Fatalf("cell %d has %d values, want 1", column, len(values))
		}
		if text, ok := values[0].Text(nil); !ok || text != want {
			t.Fatalf("cell %d = %q, %v; want %s", column, text, ok, want)
		}
	}
	floor := result.Rows()[0].Cells()[1].Values()[0]
	if floor.Kind() != ValueReal || !math.IsInf(floor.real, -1) {
		t.Fatalf("floor = %+v, want the real -Inf", floor)
	}
}
