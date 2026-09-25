package runtime

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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

const extentFleetSrc = `package test {
	private import SequenceFunctions::*;
	part def Wheel;
	part def Sat {
		attribute wheelCount : ScalarValues::Natural = size(all Wheel);
		assert constraint enough { size(all Wheel) >= 2 }
	}
}`

// A value or verdict decided over the run's extent is the occurrence's own: another
// occurrence of the shape sees the objects made since, not what the first one counted.
func TestExtentIsNotSharedBetweenOccurrences(t *testing.T) {
	ctx, idx := libraryShapeContext(t, extentFleetSrc)
	ctx.SetSharedDefaults(true)
	defer ctx.ShareVerdicts()()
	root := idx.DocumentRoot("<test>")
	make := func(name string) *Instance {
		inst, err := ctx.Instantiate(lookupOne(t, idx, name))
		if err != nil {
			t.Fatalf("instantiate %s: %v", name, err)
		}
		return inst
	}
	verdict := func(sat *Instance) string {
		report, err := ctx.ValidateObject(sat, []*symbols.Scope{root})
		if err != nil {
			t.Fatalf("validate #%d: %v", sat.ID, err)
		}
		if len(report.Verdicts) != 1 {
			t.Fatalf("#%d has %d verdicts, want its one constraint", sat.ID, len(report.Verdicts))
		}
		return report.Verdicts[0].Status.String()
	}
	make("test::Wheel")
	first := make("test::Sat")
	expect(t, ctx, first, "", "wheelCount", "1")
	if got := verdict(first); got != "violated" {
		t.Errorf("enough on the first sat = %s, want violated", got)
	}
	make("test::Wheel")
	second := make("test::Sat")
	expect(t, ctx, second, "", "wheelCount", "2")
	if got := verdict(second); got != "holds" {
		t.Errorf("enough on the second sat = %s, want holds", got)
	}
	expectTaken(t, ctx, 0)
	if taken := ctx.SharedVerdictsTaken(); taken != 0 {
		t.Errorf("shared verdicts taken = %d, want none over the extent", taken)
	}
}

// A materialization that fails after installing the image's shared records takes them
// off again: the destination's shared table is as the failed image found it.
func TestSharedDefaultRecordsUndoneWithFailedImage(t *testing.T) {
	ctx, fleet, _ := sharedFixture(t, imagedFleetSrc, "test::fleet")
	expect(t, ctx, fleet, "sats[1]", "total", "4")
	expect(t, ctx, fleet, "sats[2]", "total", "4")
	img, err := ctx.Image(fleet)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	sound := img.messages
	inBody := Value{Kind: ValFunction, ref: &functionValue{
		shape:     &calcShape{Sym: &symbols.Symbol{Name: "inBody"}, Name: "inBody"},
		enclosing: []frame{{vars: map[string]Value{"k": integerValue(1)}, run: 1}},
	}}
	img.messages = append(slices.Clone(sound), Message{Object: fleet.ID, SignalType: "go", Payload: map[string]Value{"k": inBody}})
	dst := NewContext(ctx.Model(), 10000)
	var notPortable *NotPortableError
	if err := img.Materialize(dst); !errors.As(err, &notPortable) {
		t.Fatalf("Materialize with a message it cannot carry = %v, want a NotPortableError", err)
	}
	if n := len(dst.sharedDefaults); n != 0 {
		t.Errorf("the failed materialization left %d shared records on the destination", n)
	}
	img.messages = sound
	if err := img.Materialize(dst); err != nil {
		t.Fatalf("Materialize after the failure: %v", err)
	}
	restored, ok := dst.Instance(fleet.ID)
	if !ok {
		t.Fatalf("object #%d not materialized from the image", fleet.ID)
	}
	expect(t, dst, restored, "sats[1]", "twice", "8")
	expect(t, dst, restored, "sats[2]", "twice", "8")
	expectTaken(t, dst, 1)
}

// Restoring a snapshot rewinds what occurrences took from the shared table along
// with the values they took: the count reads as it did at the snapshot, and the
// takes since are made again on the way back.
func TestSharedDefaultsTakenRestoredWithSnapshot(t *testing.T) {
	ctx, fleet, _ := sharedFixture(t, fleetSrc, "test::fleet")
	expect(t, ctx, fleet, "sats[1]", "b", "6")
	snapshot, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	defer snapshot.Release()
	expect(t, ctx, fleet, "sats[2]", "b", "6")
	expect(t, ctx, fleet, "sats[3]", "b", "6")
	expectTaken(t, ctx, 2)
	snapshot.Restore()
	expectTaken(t, ctx, 0)
	if at(t, ctx, fleet, "sats[2]").FeatureValues["b"].Materialized {
		t.Fatal("sats[2].b still materialized after the restore")
	}
	expect(t, ctx, fleet, "sats[2]", "b", "6")
	expectTaken(t, ctx, 1)
}

// A probe's takes from the shared table are undone with the values it took: the
// count reads as it did before the probe.
func TestSharedDefaultsTakenUndoneWithProbe(t *testing.T) {
	ctx, fleet, _ := sharedFixture(t, fleetSrc, "test::fleet")
	expect(t, ctx, fleet, "sats[1]", "b", "6")
	end := ctx.beginProbe()
	expect(t, ctx, fleet, "sats[2]", "b", "6")
	expectTaken(t, ctx, 1)
	end()
	expectTaken(t, ctx, 0)
	if at(t, ctx, fleet, "sats[2]").FeatureValues["b"].Materialized {
		t.Fatal("sats[2].b still materialized after the probe")
	}
	expect(t, ctx, fleet, "sats[2]", "b", "6")
	expectTaken(t, ctx, 1)
}

const assumedFleetSrc = `package test {
	part def Comp {
		attribute k : ScalarValues::Integer = 2;
	}
	part def Sat {
		part comps : Comp[2..*];
		attribute n : ScalarValues::Integer = comps#(1).k + 1;
	}
	part def Fleet {
		part sats : Sat[2];
	}
	part fleet : Fleet;
}`

// A population assumed to meet its multiplicity is imaged as assumed: restored, a
// value derived over it stays the occurrence's own, as on the source.
func TestAssumedPopulationSurvivesHeldImage(t *testing.T) {
	ctx, fleet, _ := sharedFixture(t, assumedFleetSrc, "test::fleet")
	expect(t, ctx, fleet, "sats[1]", "n", "3")
	expect(t, ctx, fleet, "sats[2]", "n", "3")
	expectTaken(t, ctx, 0)
	if !at(t, ctx, fleet, "sats[1]").FeatureValues["comps"].Assumed {
		t.Fatal("sats[1].comps is not assumed on the source")
	}
	dst := imageInto(t, ctx, fleet)
	restored, ok := dst.Instance(fleet.ID)
	if !ok {
		t.Fatalf("object #%d not materialized from the image", fleet.ID)
	}
	for _, sat := range []string{"sats[1]", "sats[2]"} {
		if !at(t, dst, restored, sat).FeatureValues["comps"].Assumed {
			t.Errorf("restored %s.comps is no longer assumed", sat)
		}
	}
	expect(t, dst, restored, "sats[1]", "n", "3")
	expect(t, dst, restored, "sats[2]", "n", "3")
	expectTaken(t, dst, 0)
}

// Adoption derives a taken value again, so it owes nothing: carried over and imaged
// before it is read, the occurrence derives it on the destination as declared.
func TestAdoptedOccurrenceOwesNothingForAValueDerivedAgain(t *testing.T) {
	prev := contextOver(t, imagedFleetSrc)
	prev.SetSharedDefaults(true)
	fleet, err := prev.Instantiate(lookupOne(t, prev.Resolver().Index(), "test::fleet"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	expect(t, prev, fleet, "sats[1]", "total", "4")
	expect(t, prev, fleet, "sats[2]", "total", "4")
	if owed := at(t, prev, fleet, "sats[2]").owed; len(owed) != 1 {
		t.Fatalf("sats[2] owes %d values before the carry-over, want its total", len(owed))
	}
	shapes := prev.ShapesOf(fleet)
	ctx := contextOver(t, imagedFleetSrc)
	ctx.SetSharedDefaults(true)
	if _, err := ctx.Adopt(prev, shapes, fleet); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if owed := at(t, ctx, fleet, "sats[2]").owed; len(owed) != 0 {
		t.Errorf("adopted sats[2] owes %d values for a total derived again", len(owed))
	}
	dst := imageInto(t, ctx, fleet)
	restored, ok := dst.Instance(fleet.ID)
	if !ok {
		t.Fatalf("object #%d not materialized from the image", fleet.ID)
	}
	if owed := at(t, dst, restored, "sats[2]").owed; len(owed) != 0 {
		t.Errorf("restored sats[2] owes %d values, want none", len(owed))
	}
	expect(t, dst, restored, "sats[2]", "total", "4")
	expect(t, dst, restored, "sats[1]", "total", "4")
	expect(t, ctx, fleet, "sats[2]", "total", "4")
}

// A taken value invalidated by a write under its occurrence is imaged as what it is,
// a value not yet derived: the destination derives it over the written value.
func TestInvalidatedTakenValueIsImagedAsUnderived(t *testing.T) {
	ctx, fleet, _ := sharedFixture(t, imagedFleetSrc, "test::fleet")
	expect(t, ctx, fleet, "sats[1]", "total", "4")
	expect(t, ctx, fleet, "sats[2]", "total", "4")
	write(t, ctx, fleet, "sats[2].c1", "m", 10)
	if at(t, ctx, fleet, "sats[2]").FeatureValues["total"].Materialized {
		t.Fatal("sats[2].total still materialized over a written c1.m")
	}
	dst := imageInto(t, ctx, fleet)
	restored, ok := dst.Instance(fleet.ID)
	if !ok {
		t.Fatalf("object #%d not materialized from the image", fleet.ID)
	}
	if owed := at(t, dst, restored, "sats[2]").owed; len(owed) != 0 {
		t.Errorf("restored sats[2] owes %d values, want none", len(owed))
	}
	expect(t, dst, restored, "sats[2]", "total", "11")
	expect(t, dst, restored, "sats[1]", "total", "4")
	expect(t, ctx, fleet, "sats[2]", "total", "11")
}

const lifetimeFleetSrc = `package test {
	private import OccurrenceFunctions::*;
	requirement def Running {
		subject s : Sat;
		require constraint { isDuring(s.c1) }
	}
	part def Comp;
	part def Sat {
		part c1 : Comp;
		attribute running : ScalarValues::Boolean = isDuring(c1);
	}
	part def Fleet {
		part sats : Sat[2];
	}
	part fleet : Fleet {
		satisfy Running by sats;
	}
}`

// A lifetime is the run's, not the shape's: a default derived over one is the
// occurrence's own, and an occurrence whose part has ended derives its own.
func TestLifetimeReadsAreNotShared(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, lifetimeFleetSrc))
	ctx.SetSharedDefaults(true)
	fleet, err := ctx.Instantiate(lookupOne(t, idx, "test::fleet"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if err := ctx.destroy(at(t, ctx, fleet, "sats[2].c1")); err != nil {
		t.Fatalf("destroy sats[2].c1: %v", err)
	}
	expect(t, ctx, fleet, "sats[1]", "running", "true")
	expect(t, ctx, fleet, "sats[2]", "running", "false")
	expectTaken(t, ctx, 0)
}

const drawingFleetSrc = `package test {
	private import RandomFunctions::*;
	requirement def Bounded {
		subject s : Sat;
		require constraint { uniform(0.0, 1.0) < 2.0 }
	}
	part def Sat {
		attribute jitter : ScalarValues::Real = uniform(0.0, 1.0);
	}
	part def Fleet {
		part sats : Sat[3];
	}
	part fleet : Fleet {
		satisfy Bounded by sats;
	}
}`

// A random draw is the run's, not the shape's: a default or a check that draws is
// evaluated on every occurrence, so the stream is drawn from once per occurrence.
func TestRandomDrawsAreNotShared(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, drawingFleetSrc))
	ctx.SetSharedDefaults(true)
	ctx.SetModelSeed(7)
	fleet, err := ctx.Instantiate(lookupOne(t, idx, "test::fleet"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for i := 1; i <= 3; i++ {
		read(t, ctx, fleet, "sats["+strconv.Itoa(i)+"]", "jitter")
	}
	if draws := ctx.DrawsTaken(); len(draws) != 3 {
		t.Errorf("draws taken = %d after reading three defaults, want 3", len(draws))
	}
	expectTaken(t, ctx, 0)
	done := ctx.ShareVerdicts()
	report, err := ctx.ValidateObject(fleet, []*symbols.Scope{idx.DocumentRoot("<test>")})
	done()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !report.Valid() {
		t.Errorf("report not valid: %+v", report.Verdicts)
	}
	if draws := ctx.DrawsTaken(); len(draws) != 6 {
		t.Errorf("draws taken = %d after checking three occurrences, want 6", len(draws))
	}
	if taken := ctx.SharedVerdictsTaken(); taken != 0 {
		t.Errorf("shared verdicts taken = %d over a check that draws, want 0", taken)
	}
}

const clockFleetSrc = `package test {
	requirement def Early {
		subject s : Sat;
		require constraint { s.localClock.currentTime < 5.0 }
	}
	part def Sat {
		attribute stamp : ScalarValues::Real = localClock.currentTime + 1.0;
	}
	part def Fleet {
		part sats : Sat[3];
	}
	part fleet : Fleet {
		satisfy Early by sats;
	}
}`

// The clock is the run's, not the shape's: a default or a check reading a Clock's
// currentTime is evaluated on every occurrence, at the instant it is read.
func TestClockReadsAreNotShared(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, clockFleetSrc))
	ctx.SetSharedDefaults(true)
	fleet, err := ctx.Instantiate(lookupOne(t, idx, "test::fleet"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if got := read(t, ctx, fleet, "sats["+strconv.Itoa(i)+"]", "stamp"); got != "1.0" {
			t.Errorf("sats[%d].stamp at t=0 = %s, want 1.0", i, got)
		}
	}
	if _, err := ctx.Advance(10); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if got := read(t, ctx, fleet, "sats[3]", "stamp"); got != "11.0" {
		t.Errorf("sats[3].stamp at t=10 = %s, want 11.0", got)
	}
	expectTaken(t, ctx, 0)
	done := ctx.ShareVerdicts()
	report, err := ctx.ValidateObject(fleet, []*symbols.Scope{idx.DocumentRoot("<test>")})
	done()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if report.Valid() {
		t.Errorf("report valid at t=10, want every check failed: %+v", report.Verdicts)
	}
	if taken := ctx.SharedVerdictsTaken(); taken != 0 {
		t.Errorf("shared verdicts taken = %d over a check that reads the clock, want 0", taken)
	}
}
