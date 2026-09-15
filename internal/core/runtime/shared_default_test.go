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

// Every scalar kind held by value — number, string, quantity, complex, enum
// literal — is shared; a value naming an object or a sequence is derived on each.
func TestSharedDefaultKinds(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
	private import SI::*;
	enum def Band { L; S; }
	part def Radio { attribute gain : ScalarValues::Real = 3.0; }
	part def Sat {
		attribute n : ScalarValues::Integer = 2;
		attribute s : ScalarValues::String = "sat-" + "x";
		attribute q = n * 5 [kg];
		attribute z = ComplexFunctions::rect(1.0, n);
		attribute band : Band = Band::S;
		attribute chosen : Band = band;
		part radio : Radio;
		attribute r = radio;
		attribute seq : ScalarValues::Integer[2] = (n, n + 1);
	}
	part def Fleet { part sats : Sat[3]; }
	part fleet : Fleet;
}`))
	ctx.SetSharedDefaults(true)
	fleet, err := ctx.Instantiate(lookupOne(t, idx, "test::fleet"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	shared := map[string]string{"s": `"sat-x"`, "q": "10 [kg]", "z": "1.0 + 2.0i", "chosen": "Band::S"}
	own := map[string]string{"r": "", "seq": "[2, 3]"}
	for name, want := range shared {
		for i := 1; i <= 3; i++ {
			expect(t, ctx, fleet, "sats["+string(rune('0'+i))+"]", name, want)
		}
	}
	expectTaken(t, ctx, int64(2*len(shared)))
	for name, want := range own {
		for i := 1; i <= 3; i++ {
			got := read(t, ctx, fleet, "sats["+string(rune('0'+i))+"]", name)
			if want != "" && got != want {
				t.Errorf("sats[%d].%s = %s, want %s", i, name, got, want)
			}
		}
	}
	expectTaken(t, ctx, int64(2*len(shared)))
	radios := map[string]bool{}
	for i := 1; i <= 3; i++ {
		radios[read(t, ctx, fleet, "sats["+string(rune('0'+i))+"]", "r")] = true
	}
	if len(radios) != 3 {
		t.Errorf("r names %d distinct objects over three occurrences, want 3", len(radios))
	}
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

const classifiedFleetSrc = `package test {
	part def Sat {
		attribute a : ScalarValues::Integer = 2;
		attribute b : ScalarValues::Integer = a * 3;
	}
	part def Heavy :> Sat {
		attribute :>> a = 10;
	}
	part def Fleet {
		part sats : Sat[3];
	}
	part fleet : Fleet;
}`

// A classifier redefining what a taken value read gives the occurrence its own
// value, whether it is classified before or after the take; undoing the
// classification with its change restores the shape's value.
func TestSharedDefaultUnderClassification(t *testing.T) {
	ctx, fleet, idx := sharedFixture(t, classifiedFleetSrc, "test::fleet")
	heavy := lookupOne(t, idx, "test::Heavy")
	expect(t, ctx, fleet, "sats[1]", "b", "6")
	expect(t, ctx, fleet, "sats[2]", "b", "6")
	expectTaken(t, ctx, 1)
	if err := ctx.classify(at(t, ctx, fleet, "sats[2]"), heavy); err != nil {
		t.Fatalf("classify sats[2]: %v", err)
	}
	expect(t, ctx, fleet, "sats[2]", "b", "30")
	expect(t, ctx, fleet, "sats[1]", "b", "6")
	if err := ctx.classify(at(t, ctx, fleet, "sats[3]"), heavy); err != nil {
		t.Fatalf("classify sats[3]: %v", err)
	}
	expect(t, ctx, fleet, "sats[3]", "b", "30")
	expectTaken(t, ctx, 2)

	end := ctx.beginProbe()
	if err := ctx.classify(at(t, ctx, fleet, "sats[1]"), heavy); err != nil {
		t.Fatalf("classify sats[1]: %v", err)
	}
	expect(t, ctx, fleet, "sats[1]", "b", "30")
	end()
	expect(t, ctx, fleet, "sats[1]", "b", "6")
}

const failingDefaultSrc = `package test {
	part def Sat {
		attribute a : ScalarValues::Integer = 2;
		attribute b : ScalarValues::Integer = a / 0;
	}
	part def Fleet {
		part sats : Sat[2];
	}
	part fleet : Fleet;
}`

// A default that fails to derive is never shared: every occurrence reports the
// failure for itself, the same way it would deriving in place.
func TestSharedDefaultFailureIsNotShared(t *testing.T) {
	ctx, fleet, _ := sharedFixture(t, failingDefaultSrc, "test::fleet")
	var errs []string
	for _, path := range []string{"sats[1]", "sats[2]"} {
		_, err := at(t, ctx, fleet, path).GetFeatureValue(ctx, "b")
		if err == nil {
			t.Fatalf("%s.b derived from a division by zero", path)
		}
		errs = append(errs, err.Error())
	}
	if errs[0] != errs[1] {
		t.Errorf("the occurrences fail differently:\n%s\n%s", errs[0], errs[1])
	}
	expectTaken(t, ctx, 0)
}

const imagedFleetSrc = `package test {
	part def Comp {
		attribute m : ScalarValues::Integer = 3;
	}
	part def Sat {
		part c1 : Comp;
		attribute total : ScalarValues::Integer = c1.m + 1;
		attribute twice : ScalarValues::Integer = total * 2;
	}
	part def Heavy :> Sat {
		part :>> c1 { attribute :>> m = 9; }
	}
	part def Fleet {
		part sats : Sat[2];
	}
	part fleet : Fleet;
}`

// An image of an occurrence that took a value shared over its unmaterialized
// subtree materializes as one that derived it in place, still owing that subtree
// and still sharing what its restored values derive: classifying and writing under
// the restored object reach the value.
func TestSharedDefaultSurvivesHeldImage(t *testing.T) {
	ctx, fleet, idx := sharedFixture(t, imagedFleetSrc, "test::fleet")
	expect(t, ctx, fleet, "sats[1]", "total", "4")
	expect(t, ctx, fleet, "sats[2]", "total", "4")
	expectTaken(t, ctx, 1)
	if at(t, ctx, fleet, "sats[2]").FeatureValues["c1"].Materialized {
		t.Fatal("sats[2].c1 was materialized to take a shared total")
	}
	dst := imageInto(t, ctx, fleet)
	restored, ok := dst.Instance(fleet.ID)
	if !ok {
		t.Fatalf("object #%d not materialized from the image", fleet.ID)
	}
	if owed := at(t, dst, restored, "sats[2]").owed; len(owed) != 1 || owed[0].fv != at(t, dst, restored, "sats[2]").FeatureValues["total"] {
		t.Errorf("restored sats[2] owes %d values, want its total", len(owed))
	}
	expect(t, dst, restored, "sats[1]", "twice", "8")
	expect(t, dst, restored, "sats[2]", "twice", "8")
	expectTaken(t, dst, 1)
	if err := dst.classify(at(t, dst, restored, "sats[2]"), lookupOne(t, idx, "test::Heavy")); err != nil {
		t.Fatalf("classify sats[2]: %v", err)
	}
	expect(t, dst, restored, "sats[2]", "total", "10")
	write(t, dst, restored, "sats[1].c1", "m", 10)
	expect(t, dst, restored, "sats[1]", "total", "11")
	expect(t, ctx, fleet, "sats[1]", "total", "4")
	expect(t, ctx, fleet, "sats[2]", "total", "4")
}

func TestPathKeyDistinguishesDottedNames(t *testing.T) {
	quoted, nested := pathKey([]string{"a.b"}), pathKey([]string{"a", "b"})
	if quoted == nested {
		t.Fatalf("pathKey conflates a quoted 'a.b' with the nested path a.b: %q", quoted)
	}
	if pathKey([]string{"a", "b"}) != nested {
		t.Fatal("pathKey is not stable over equal paths")
	}
}

const collectionFleetSrc = `package test {
	part def Comp {
		attribute k : ScalarValues::Integer = 2;
		attribute m : ScalarValues::Integer = k + 1;
	}
	part def HeavyComp :> Comp {
		attribute :>> m = 9;
	}
	part def Sat {
		part comps : Comp[2];
		attribute total : ScalarValues::Integer = comps#(1).m + comps#(2).m;
	}
	part def Tagged :> Sat {
		attribute tag : ScalarValues::Integer = 0;
	}
	part def Fleet {
		part sats : Sat[2];
	}
	part fleet : Fleet;
}`

// A value taken over a collection some of whose elements were read already is owed
// for the elements that were not: a classifier redeclaring the occurrence settles
// them, and one redefining what a lazy element derives reaches the value.
func TestSharedDefaultOwesEveryLazyElement(t *testing.T) {
	ctx, fleet, idx := sharedFixture(t, collectionFleetSrc, "test::fleet")
	expect(t, ctx, fleet, "sats[1]", "total", "6")
	expect(t, ctx, fleet, "sats[2].comps[2]", "m", "3")
	lazy := at(t, ctx, fleet, "sats[2].comps[1]").FeatureValues["m"]
	if lazy.Materialized {
		t.Fatal("sats[2].comps[1].m was materialized by reading comps[2].m")
	}
	expect(t, ctx, fleet, "sats[2]", "total", "6")
	sat := at(t, ctx, fleet, "sats[2]")
	if len(sat.owed) != 1 || sat.owed[0].fv != sat.FeatureValues["total"] {
		t.Fatalf("sats[2] owes %d values, want its total", len(sat.owed))
	}
	if err := ctx.classify(sat, lookupOne(t, idx, "test::Tagged")); err != nil {
		t.Fatalf("classify sats[2]: %v", err)
	}
	if !lazy.Materialized || len(sat.owed) != 0 {
		t.Fatalf("classifying sats[2] left comps[1].m materialized=%v, owing %d", lazy.Materialized, len(sat.owed))
	}
	if err := ctx.classify(at(t, ctx, fleet, "sats[2].comps[1]"), lookupOne(t, idx, "test::HeavyComp")); err != nil {
		t.Fatalf("classify sats[2].comps[1]: %v", err)
	}
	expect(t, ctx, fleet, "sats[2]", "total", "12")
	expect(t, ctx, fleet, "sats[1]", "total", "6")
}
