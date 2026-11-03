package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

// A held-image copy of a machine whose attributes read the machine's own
// occurrence features re-derives them: the image carries no tracking value, so
// the copy's cells evaluate `size(entryAction)` again, on the copy's occurrence.
func TestHeldImageRederivesBehaviorOccurrenceAttributes(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "conformance", "behavior_occurrence_attributes.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, inst, err := instantiateWithLibraries(t, string(src), "test::Machine")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	want, err := objectMachine(t, inst, "modes").StateDataWithError()
	if err != nil {
		t.Fatalf("source state data: %v", err)
	}
	if len(want) != 7 {
		t.Fatalf("source state data = %v, want the seven counts", want)
	}

	img, err := ctx.Image(inst)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	dst := NewContext(ctx.Model(), 10000)
	if err := img.Materialize(dst); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	copied, held := dst.Instance(inst.ID)
	if !held {
		t.Fatalf("the copy holds no object #%d", inst.ID)
	}
	got, err := objectMachine(t, copied, "modes").StateDataWithError()
	if err != nil {
		t.Fatalf("copy state data: %v", err)
	}
	for name, value := range want {
		if !valueEqual(got[name], value) {
			t.Errorf("copy %s = %s, source %s", name, FormatValue(got[name]), FormatValue(value))
		}
	}
	work, ok := copied.behavior("work")
	if !ok || work.Action == nil {
		t.Fatal("the copy runs no performed action work")
	}
	count, ok, err := dst.readBodyCell(work.Action.root.cells, work.Action.root.key("subactionCount"))
	if err != nil || !ok || !valueEqual(count, integerValue(2)) {
		t.Errorf("copy work.subactionCount = %s (held %v, err %v), want 2", FormatValue(count), ok, err)
	}
}
