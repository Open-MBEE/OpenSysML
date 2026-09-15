package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// sharedFixture indexes src with derived-default sharing on and instantiates the
// named part, returning it with the context and index.
func sharedFixture(t *testing.T, src, part string) (*Context, *Instance, *symbols.Index) {
	t.Helper()
	ctx, idx := contextForSource(t, src)
	ctx.SetSharedDefaults(true)
	inst, err := ctx.Instantiate(lookupOne(t, idx, part))
	if err != nil {
		t.Fatalf("instantiate %s: %v", part, err)
	}
	return ctx, inst, idx
}

// at follows a dotted path of features from inst, indexing a collection element
// one-based as `sats.2`, and returns the object reached.
func at(t *testing.T, ctx *Context, inst *Instance, path string) *Instance {
	t.Helper()
	for _, step := range strings.Split(path, ".") {
		if step == "" {
			continue
		}
		name, index := step, 0
		if i := strings.IndexByte(step, '['); i >= 0 {
			name = step[:i]
			for _, c := range step[i+1 : len(step)-1] {
				index = index*10 + int(c-'0')
			}
		}
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		held := elementsOf(fv.HeldValue())
		if index > 0 {
			held = held[index-1 : index]
		}
		if len(held) != 1 {
			t.Fatalf("%s: %s holds %d objects, want one", path, name, len(held))
		}
		id, ok := held[0].Object()
		if !ok {
			t.Fatalf("%s: %s holds no object", path, name)
		}
		inst, _ = ctx.Instance(id)
	}
	return inst
}

// read reads the named feature of the object at path and formats it.
func read(t *testing.T, ctx *Context, inst *Instance, path, name string) string {
	t.Helper()
	fv, err := at(t, ctx, inst, path).GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("%s.%s: %v", path, name, err)
	}
	val, err := fv.ReadValue(name)
	if err != nil {
		t.Fatalf("%s.%s: %v", path, name, err)
	}
	return FormatValue(val)
}

// write sets the named feature of the object at path to an integer.
func write(t *testing.T, ctx *Context, inst *Instance, path, name string, n int64) {
	t.Helper()
	if err := at(t, ctx, inst, path).SetFeatureValue(ctx, name, integerValue(n)); err != nil {
		t.Fatalf("%s.%s = %d: %v", path, name, n, err)
	}
}

// expect fails unless the named feature at path reads as want.
func expect(t *testing.T, ctx *Context, inst *Instance, path, name, want string) {
	t.Helper()
	if got := read(t, ctx, inst, path, name); got != want {
		t.Errorf("%s.%s = %s, want %s", path, name, got, want)
	}
}

// expectTaken fails unless the context took the given number of shared defaults so far.
func expectTaken(t *testing.T, ctx *Context, want int64) {
	t.Helper()
	if got := ctx.SharedDefaultsTaken(); got != want {
		t.Errorf("shared defaults taken = %d, want %d", got, want)
	}
}

const fleetSrc = `package test {
	part def Sat {
		attribute a : ScalarValues::Integer = 2;
		attribute b : ScalarValues::Integer = a * 3;
	}
	part def Fleet {
		part sats : Sat[3];
	}
	part fleet : Fleet;
}`

// A derived default read on one occurrence is taken by the others of the shape
// without deriving again, and reads the same.
func TestSharedDefaultTakenByOccurrencesOfShape(t *testing.T) {
	ctx, fleet, _ := sharedFixture(t, fleetSrc, "test::fleet")
	for i := 1; i <= 3; i++ {
		expect(t, ctx, fleet, "sats["+string(rune('0'+i))+"]", "b", "6")
	}
	expectTaken(t, ctx, 2)
}

// An occurrence whose input diverged before the read derives its own value; one
// diverging after taking a shared value is invalidated like a value derived in place.
func TestSharedDefaultFollowsDivergence(t *testing.T) {
	ctx, fleet, _ := sharedFixture(t, fleetSrc, "test::fleet")
	write(t, ctx, fleet, "sats[2]", "a", 5)
	expect(t, ctx, fleet, "sats[1]", "b", "6")
	expect(t, ctx, fleet, "sats[2]", "b", "15")
	expect(t, ctx, fleet, "sats[3]", "b", "6")
	expectTaken(t, ctx, 1)
	write(t, ctx, fleet, "sats[3]", "a", 7)
	expect(t, ctx, fleet, "sats[3]", "b", "21")
	expect(t, ctx, fleet, "sats[1]", "b", "6")
	expectTaken(t, ctx, 1)
}

// A value derived on a diverged occurrence is that occurrence's own: the shape
// does not take it.
func TestDivergedOccurrenceDoesNotShareItsDefault(t *testing.T) {
	ctx, fleet, _ := sharedFixture(t, fleetSrc, "test::fleet")
	write(t, ctx, fleet, "sats[1]", "a", 5)
	expect(t, ctx, fleet, "sats[1]", "b", "15")
	expect(t, ctx, fleet, "sats[2]", "b", "6")
	expect(t, ctx, fleet, "sats[3]", "b", "6")
	expectTaken(t, ctx, 1)
}

const subtreeSrc = `package test {
	part def Comp {
		attribute m : ScalarValues::Integer = 3;
	}
	part def Sat {
		part c1 : Comp;
		part c2 : Comp {
			attribute :>> m = 4;
		}
		attribute total : ScalarValues::Integer = c1.m + c2.m;
	}
	part def Fleet {
		part sats : Sat[3];
	}
	part fleet : Fleet;
}`

// A default read through the occurrence's own subtree is taken without
// materializing that subtree, and a later write under it still invalidates the value.
func TestSharedDefaultOverSubtreeInvalidatedByLaterWrite(t *testing.T) {
	ctx, fleet, _ := sharedFixture(t, subtreeSrc, "test::fleet")
	expect(t, ctx, fleet, "sats[1]", "total", "7")
	expect(t, ctx, fleet, "sats[2]", "total", "7")
	expectTaken(t, ctx, 1)
	if fv := at(t, ctx, fleet, "sats[2]").FeatureValues["c1"]; fv.Materialized {
		t.Error("sats[2].c1 was materialized to take a shared total")
	}
	write(t, ctx, fleet, "sats[2].c1", "m", 10)
	expect(t, ctx, fleet, "sats[2]", "total", "14")
	expect(t, ctx, fleet, "sats[1]", "total", "7")
	write(t, ctx, fleet, "sats[3].c2", "m", 1)
	expect(t, ctx, fleet, "sats[3]", "total", "4")
	expectTaken(t, ctx, 1)
}

// Turning sharing off leaves every occurrence deriving for itself.
func TestSharedDefaultsOffDerivesEverywhere(t *testing.T) {
	ctx, idx := contextForSource(t, fleetSrc)
	ctx.SetSharedDefaults(false)
	fleet, err := ctx.Instantiate(lookupOne(t, idx, "test::fleet"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		expect(t, ctx, fleet, "sats["+string(rune('0'+i))+"]", "b", "6")
	}
	expectTaken(t, ctx, 0)
}
