package resolve

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestMemoSizeExemptionsNameMapFields keeps the exempt list honest: a renamed
// or removed field must not leave a stale name that exempts nothing.
func TestMemoSizeExemptionsNameMapFields(t *testing.T) {
	fields := reflect.TypeOf(Resolver{})
	for name := range memoSizeExempt {
		field, ok := fields.FieldByName(name)
		if !ok || field.Type.Kind() != reflect.Map {
			t.Errorf("memoSizeExempt names %q, which is not a map field of Resolver", name)
		}
	}
}

func TestMemoSizeCountsEveryRetainedTable(t *testing.T) {
	r := New(symbols.NewIndex())
	if got := r.MemoSize(); got != 0 {
		t.Fatalf("a new resolver retains %d entries", got)
	}
	r.payloads[nil] = nil
	r.bodyOwners[nil] = nil
	r.resolvingImports[nil] = true
	if got := r.MemoSize(); got != 2 {
		t.Fatalf("MemoSize = %d, want the two retained entries and not the in-progress guard", got)
	}
}
