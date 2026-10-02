package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestRuntimeRobustnessLazyRequired exercises a lower bound held lazily: the values a
// lower bound requires are all there, counted and addressed exactly, each object made when
// first reached under an identity that never changes; work that must reach every value is
// charged to the element budget, and a lower bound of * stays a typed refusal.
func TestRuntimeRobustnessLazyRequired(t *testing.T) {
	t.Run("large_lower_bound_counts_without_making", testLazyRequiredCountsWithoutMaking)
	t.Run("indexed_read_makes_one_stable_object", testLazyRequiredIndexedReadIsStable)
	t.Run("lower_bound_given_by_an_expression", testLazyRequiredExpressionBound)
	t.Run("infinite_lower_bound_names_the_clause", testLazyRequiredInfiniteLowerBound)
	t.Run("reading_every_value_past_the_element_budget", testLazyRequiredExhaustiveOverBudget)
	t.Run("reading_every_value_within_the_element_budget", testLazyRequiredExhaustiveWithinBudget)
	t.Run("a_billion_required_values", testLazyRequiredBillion)
	t.Run("snapshot_restores_unmade_members", testLazyRequiredSnapshot)
	t.Run("image_carries_the_population", testLazyRequiredImage)
	t.Run("listing_every_value_is_budgeted", testLazyRequiredListing)
	t.Run("namespace_usage_held_lazily", testLazyRequiredNamespaceUsage)
	t.Run("written_member_holds_the_write", testLazyRequiredWrite)
	t.Run("subsetter_is_among_the_values", testLazyRequiredSubsetter)
	t.Run("quantifier_reaches_every_value", testLazyRequiredQuantified)
	t.Run("state_key_spells_the_population", testLazyRequiredStateKey)
}

const lazyHolderSrc = `
	package test {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		private import RealFunctions::sum;
		private import ControlFunctions::*;
		part def C { attribute m : Real = 1.0; }
		part def Holder { part p : C[5000]; }
	}
`

// lazyHolder instantiates test::Holder of src over the libraries.
func lazyHolder(t *testing.T, src string) (*Instance, *Context) {
	t.Helper()
	model, resolver, root := parseAndBuildLibraryModel(t, src)
	holder := resolveSymbol(t, resolveSymbol(t, root, "test").Scope, "Holder")
	ctx := NewContext(typedModel(model, resolver), DefaultMaxSteps)
	inst, err := ctx.Instantiate(holder)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return inst, ctx
}

// evalOn evaluates src with inst as the object its type's features are read on.
func evalOn(t *testing.T, ctx *Context, inst *Instance, src string) (Value, error) {
	t.Helper()
	p := parser.New(source.New("<expr>", []byte(src)))
	expr := p.ParseExpression()
	if expr == nil || len(p.Diagnostics) > 0 {
		t.Fatalf("parse %q: %v", src, p.Diagnostics)
	}
	ec := NewEvalContextIn(ctx, inst.Type.Scope, inst)
	defer ec.beginStep()()
	return ec.Eval(expr)
}

// mustEvalOn is evalOn failing the test on an error.
func mustEvalOn(t *testing.T, ctx *Context, inst *Instance, src string) Value {
	t.Helper()
	val, err := evalOn(t, ctx, inst, src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return val
}

// testLazyRequiredCountsWithoutMaking: `size`, `isEmpty` and `notEmpty` answer from the
// lower bound and make none of the objects they count.
func testLazyRequiredCountsWithoutMaking(t *testing.T) {
	inst, ctx := lazyHolder(t, lazyHolderSrc)
	before := len(ctx.instances)
	for src, want := range map[string]string{"size(p)": "5000", "isEmpty(p)": "false", "notEmpty(p)": "true"} {
		if got := FormatValue(mustEvalOn(t, ctx, inst, src)); got != want {
			t.Errorf("%s = %s, want %s", src, got, want)
		}
	}
	if made := len(ctx.instances) - before; made > 1 {
		t.Errorf("counting 5000 values made %d objects", made)
	}
}

// testLazyRequiredIndexedReadIsStable: `p#(i)` makes the i-th object once; reading it again,
// or through a chain, reaches the same object, and its features read as declared.
func testLazyRequiredIndexedReadIsStable(t *testing.T) {
	inst, ctx := lazyHolder(t, lazyHolderSrc)
	first := mustEvalOn(t, ctx, inst, "p#(4000)")
	before := len(ctx.instances)
	again := mustEvalOn(t, ctx, inst, "p#(4000)")
	if first.Kind != ValInstance || again.Instance != first.Instance {
		t.Fatalf("p#(4000) read %s then %s, want one object", FormatValue(first), FormatValue(again))
	}
	if len(ctx.instances) != before {
		t.Errorf("reading p#(4000) again made %d objects", len(ctx.instances)-before)
	}
	if got := FormatValue(mustEvalOn(t, ctx, inst, "p#(4000).m")); got != "1.0" {
		t.Errorf("p#(4000).m = %s, want 1.0", got)
	}
	if got := FormatValue(mustEvalOn(t, ctx, inst, "p#(4000) == p#(4000)")); got != "true" {
		t.Errorf("p#(4000) == p#(4000) is %s", got)
	}
	if got := FormatValue(mustEvalOn(t, ctx, inst, "p#(4000) == p#(4001)")); got != "false" {
		t.Errorf("p#(4000) == p#(4001) is %s", got)
	}
	if _, err := evalOn(t, ctx, inst, "p#(5001)"); !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("p#(5001) = %v, want %v", err, ErrIndexOutOfRange)
	}
}

// testLazyRequiredExpressionBound: a model-level evaluable bound naming a valued feature
// holds the number it evaluates to.
func testLazyRequiredExpressionBound(t *testing.T) {
	for _, tc := range []struct {
		decl string
		want string
	}{
		{"part p : C[3 + 2];", "5"},
		{"part p : C[k];", "4"},
		{"part p : C[k * 1000];", "4000"},
		{"attribute n : Integer = 7; part p : C[n];", "7"},
		{"part p : C[k..*];", "4"},
	} {
		inst, ctx := lazyHolder(t, `
			package test {
				private import ScalarValues::*;
				private import SequenceFunctions::*;
				attribute k : Integer = 4;
				part def C { attribute m : Real = 1.0; }
				part def Holder { `+tc.decl+` }
			}
		`)
		val, err := evalOn(t, ctx, inst, "size(p)")
		if err != nil || FormatValue(val) != tc.want {
			t.Errorf("%s: size(p) = %s, %v; want %s", tc.decl, FormatValue(val), err, tc.want)
		}
	}
}

// testLazyRequiredInfiniteLowerBound: `[*..*]` requires no finite number of values, a
// multiplicity violation naming the clause rather than a population of no size.
func testLazyRequiredInfiniteLowerBound(t *testing.T) {
	inst, ctx := lazyHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def C { attribute m : Real = 1.0; }
			part def Holder { part p : C[*..*]; }
		}
	`)
	before := len(ctx.instances)
	_, err := inst.GetFeatureValue(ctx, "p")
	if !errors.Is(err, ErrInfiniteLowerBound) || !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("p = %v, want %v", err, ErrInfiniteLowerBound)
	}
	if !strings.Contains(err.Error(), "§7.4.12") {
		t.Errorf("error %q names no clause", err)
	}
	if len(ctx.instances) != before {
		t.Errorf("the refusal left %d objects", len(ctx.instances)-before)
	}
}

// testLazyRequiredExhaustiveOverBudget: a chain, an extent and a reduction over every value
// reach past the element budget and are refused, leaving no object they made; counting and
// indexing still answer.
func testLazyRequiredExhaustiveOverBudget(t *testing.T) {
	inst, ctx := lazyHolder(t, lazyHolderSrc)
	mustEvalOn(t, ctx, inst, "size(p)")
	ctx.maxElements = 100
	before := len(ctx.instances)
	for _, src := range []string{"p.m", "sum(p.m)", "size(all C)", "p"} {
		_, err := evalOn(t, ctx, inst, src)
		if !errors.Is(err, ErrElementLimitExceeded) {
			t.Errorf("%s = %v, want %v", src, err, ErrElementLimitExceeded)
		}
		if len(ctx.instances) != before {
			t.Errorf("%s left %d objects behind", src, len(ctx.instances)-before)
		}
	}
	if got := FormatValue(mustEvalOn(t, ctx, inst, "size(p)")); got != "5000" {
		t.Errorf("size(p) past the budget = %s, want 5000", got)
	}
	if got := FormatValue(mustEvalOn(t, ctx, inst, "p#(2500).m")); got != "1.0" {
		t.Errorf("p#(2500).m past the budget = %s, want 1.0", got)
	}
}

// testLazyRequiredExhaustiveWithinBudget: within the budget every value is reached, each
// object once, in the order of the collection.
func testLazyRequiredExhaustiveWithinBudget(t *testing.T) {
	inst, ctx := lazyHolder(t, lazyHolderSrc)
	mid := mustEvalOn(t, ctx, inst, "p#(2500)")
	if got := FormatValue(mustEvalOn(t, ctx, inst, "sum(p.m)")); got != "5000.0" {
		t.Errorf("sum(p.m) = %s, want 5000.0", got)
	}
	if got := FormatValue(mustEvalOn(t, ctx, inst, "size(all C)")); got != "5000" {
		t.Errorf("size(all C) = %s, want 5000", got)
	}
	all := mustEvalOn(t, ctx, inst, "p")
	if got := ElementCount(all); got != 5000 {
		t.Fatalf("p holds %d values, want 5000", got)
	}
	at, err := ctx.ElementAt(all, 2499)
	if err != nil || at.Instance != mid.Instance {
		t.Errorf("the 2500th of p is %s, %v; want the object p#(2500) read, #%d", FormatValue(at), err, mid.Instance)
	}
}

// testLazyRequiredBillion: a lower bound of a billion is counted and addressed at both ends
// without allocating it; reading every value is refused by the budget.
func testLazyRequiredBillion(t *testing.T) {
	inst, ctx := lazyHolder(t, strings.Replace(lazyHolderSrc, "C[5000]", "C[1000000000]", 1))
	if got := FormatValue(mustEvalOn(t, ctx, inst, "size(p)")); got != "1000000000" {
		t.Errorf("size(p) = %s", got)
	}
	last := mustEvalOn(t, ctx, inst, "p#(1000000000)")
	if again := mustEvalOn(t, ctx, inst, "p#(1000000000)"); again.Instance != last.Instance {
		t.Errorf("the last value read as #%d then #%d", last.Instance, again.Instance)
	}
	if _, err := evalOn(t, ctx, inst, "p.m"); !errors.Is(err, ErrElementLimitExceeded) {
		t.Errorf("p.m = %v, want %v", err, ErrElementLimitExceeded)
	}
	if n := len(ctx.instances); n > 10 {
		t.Errorf("a billion required values made %d objects", n)
	}
}

// testLazyRequiredSnapshot: restoring a snapshot takes back a member made since, and reading
// it again makes it under the identity it had.
func testLazyRequiredSnapshot(t *testing.T) {
	inst, ctx := lazyHolder(t, lazyHolderSrc)
	mustEvalOn(t, ctx, inst, "size(p)")
	snapshot, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	defer snapshot.Release()
	made := mustEvalOn(t, ctx, inst, "p#(3000)")
	snapshot.Restore()
	if _, live := ctx.instances[made.Instance]; live {
		t.Errorf("object #%d made after the snapshot is still held", made.Instance)
	}
	if again := mustEvalOn(t, ctx, inst, "p#(3000)"); again.Instance != made.Instance {
		t.Errorf("p#(3000) is #%d after the restore, want #%d", again.Instance, made.Instance)
	}
	if got := FormatValue(mustEvalOn(t, ctx, inst, "size(p)")); got != "5000" {
		t.Errorf("size(p) after the restore = %s", got)
	}
}

// testLazyRequiredImage: an image carries the population, made members and unmade, so the
// destination counts it whole and reaches each member under its identity.
func testLazyRequiredImage(t *testing.T) {
	inst, ctx := lazyHolder(t, lazyHolderSrc)
	made := mustEvalOn(t, ctx, inst, "p#(10)")
	mustEvalOn(t, ctx, inst, "p#(10).m")
	dst := imageInto(t, ctx, inst)
	restored, ok := dst.Instance(inst.ID)
	if !ok {
		t.Fatalf("object #%d not materialized from the image", inst.ID)
	}
	if got := FormatValue(mustEvalOn(t, dst, restored, "size(p)")); got != "5000" {
		t.Errorf("restored size(p) = %s", got)
	}
	if got := mustEvalOn(t, dst, restored, "p#(10)"); got.Instance != made.Instance {
		t.Errorf("restored p#(10) is #%d, want #%d", got.Instance, made.Instance)
	}
	far := mustEvalOn(t, ctx, inst, "p#(4999)")
	if got := mustEvalOn(t, dst, restored, "p#(4999)"); got.Instance != far.Instance {
		t.Errorf("restored p#(4999) is #%d, want #%d", got.Instance, far.Instance)
	}
	if n := dst.InstanceCount(); n > 10 {
		t.Errorf("the image made %d objects", n)
	}
}

// testLazyRequiredListing: listing every value of a population past the element budget is
// refused, while each member is still addressed by position.
func testLazyRequiredListing(t *testing.T) {
	inst, ctx := lazyHolder(t, lazyHolderSrc)
	fv, err := inst.GetFeatureValue(ctx, "p")
	if err != nil {
		t.Fatalf("p: %v", err)
	}
	ctx.maxElements = 100
	if _, err := ctx.HeldElements(fv.Values); !errors.Is(err, ErrElementLimitExceeded) {
		t.Errorf("HeldElements = %v, want %v", err, ErrElementLimitExceeded)
	}
	if _, err := ctx.ElementAt(fv.Values, 4999); err != nil {
		t.Errorf("ElementAt(4999): %v", err)
	}
	if _, err := ctx.ElementAt(fv.Values, 5000); !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("ElementAt(5000) = %v, want %v", err, ErrIndexOutOfRange)
	}
	ctx.maxElements = 10000
	held, err := ctx.HeldElements(fv.Values)
	if err != nil || len(held) != 5000 {
		t.Errorf("HeldElements within the budget = %d values, %v", len(held), err)
	}
}

// testLazyRequiredNamespaceUsage: a namespace usage of thousands of occurrences is counted
// and indexed without making them, and its extent is charged to the element budget.
func testLazyRequiredNamespaceUsage(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, `package P {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		part def Wheel { attribute r : Real = 0.5; }
		part many : Wheel[20000];
		calc manyCount { return : Natural = size(many); }
		calc lastRadius { return : Real = many#(20000).r; }
		calc wheelCount { return : Natural = size(all Wheel); }
	}`)
	pkg := resolveSymbol(t, root, "P")
	ctx := NewContext(typedModel(model, resolver), DefaultMaxSteps)
	for calc, want := range map[string]string{"manyCount": "20000", "lastRadius": "0.5"} {
		got, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, calc), nil, pkg.Scope)
		if err != nil || FormatValue(got) != want {
			t.Errorf("%s = %s, %v; want %s", calc, FormatValue(got), err, want)
		}
	}
	if n := len(ctx.instances); n > 10 {
		t.Errorf("counting and indexing 20000 occurrences made %d objects", n)
	}
	ctx.maxElements = 100
	held := len(ctx.instances)
	if _, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, "wheelCount"), nil, pkg.Scope); !errors.Is(err, ErrElementLimitExceeded) {
		t.Errorf("size(all Wheel) = %v, want %v", err, ErrElementLimitExceeded)
	}
	if len(ctx.instances) != held {
		t.Errorf("the refused extent left %d objects", len(ctx.instances)-held)
	}
	ctx.maxElements = 100000
	got, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, "wheelCount"), nil, pkg.Scope)
	if err != nil || FormatValue(got) != "20000" {
		t.Errorf("size(all Wheel) within the budget = %s, %v; want 20000", FormatValue(got), err)
	}
}

// testLazyRequiredWrite: a value written on a member holds on the object every later read
// reaches, and an exhaustive read within the budget sees it among the rest.
func testLazyRequiredWrite(t *testing.T) {
	inst, ctx := lazyHolder(t, lazyHolderSrc)
	ctx.maxElements = 10000
	member := mustEvalOn(t, ctx, inst, "p#(3000)")
	obj, ok := ctx.Instance(member.Instance)
	if !ok {
		t.Fatalf("p#(3000) is no object: %s", FormatValue(member))
	}
	if err := obj.SetFeatureValue(ctx, "m", mustEvalOn(t, ctx, inst, "2.5")); err != nil {
		t.Fatalf("write p#(3000).m: %v", err)
	}
	if got := FormatValue(mustEvalOn(t, ctx, inst, "p#(3000).m")); got != "2.5" {
		t.Errorf("p#(3000).m = %s after the write, want 2.5", got)
	}
	if got := FormatValue(mustEvalOn(t, ctx, inst, "sum(p.m)")); got != "5001.5" {
		t.Errorf("sum(p.m) = %s, want 5001.5", got)
	}
}

// testLazyRequiredSubsetter: a subsetting feature is one of the values, so the required
// members only make up the rest of the lower bound.
func testLazyRequiredSubsetter(t *testing.T) {
	inst, ctx := lazyHolder(t, `
		package test {
			private import ScalarValues::*;
			private import SequenceFunctions::*;
			part def C { attribute m : Real = 1.0; }
			part def Holder { part p : C[5000]; part s : C :> p { attribute :>> m = 7.0; } }
		}
	`)
	if got := FormatValue(mustEvalOn(t, ctx, inst, "size(p)")); got != "5000" {
		t.Errorf("size(p) = %s, want 5000", got)
	}
	if got := FormatValue(mustEvalOn(t, ctx, inst, "p#(1) == s")); got != "true" {
		t.Errorf("p#(1) == s is %s, want the subsetter first", got)
	}
	if got := FormatValue(mustEvalOn(t, ctx, inst, "includes(p, s)")); got != "true" {
		t.Errorf("includes(p, s) is %s", got)
	}
	if len(ctx.instances) > 100 {
		t.Errorf("made %d objects to hold one subsetter and 4999 required values", len(ctx.instances))
	}
}

// testLazyRequiredQuantified: a quantifier and `all T` reach every value, made once each.
func testLazyRequiredQuantified(t *testing.T) {
	inst, ctx := lazyHolder(t, lazyHolderSrc)
	ctx.maxElements = 10000
	if got := FormatValue(mustEvalOn(t, ctx, inst, "p->forAll{in x : C; x.m > 0.0}")); got != "true" {
		t.Errorf("p->forAll = %s, want true", got)
	}
	made := len(ctx.instances)
	if got := FormatValue(mustEvalOn(t, ctx, inst, "size(p->select{in x : C; x.m > 0.0})")); got != "5000" {
		t.Errorf("size(p->select) = %s, want 5000", got)
	}
	if len(ctx.instances) != made {
		t.Errorf("reading every value again made %d more objects", len(ctx.instances)-made)
	}
}

// testLazyRequiredStateKey: the canonical state spells an unmade population by its
// identities, and a member once made by its path, alike in every run that made it.
func testLazyRequiredStateKey(t *testing.T) {
	spell := func(read string) string {
		inst, ctx := lazyHolder(t, lazyHolderSrc)
		if read != "" {
			mustEvalOn(t, ctx, inst, read)
		}
		fv, err := inst.GetFeatureValue(ctx, "p")
		if err != nil {
			t.Fatalf("p: %v", err)
		}
		s := &stateSpeller{ctx: ctx, paths: make(map[int64]string)}
		return s.value(fv.Values)
	}
	unmade, made := spell(""), spell("p#(3000)")
	if strings.Contains(unmade, "[2999]") || !strings.Contains(unmade, "4999 unmade C") {
		t.Errorf("an unmade population spells %s, want its unmade members counted", unmade)
	}
	if unmade == made {
		t.Errorf("making p#(3000) left the state spelled %s", made)
	}
	if !strings.Contains(made, "[2999]") {
		t.Errorf("p#(3000) is not spelled by its path: %s", made)
	}
	if again := spell("p#(3000)"); again != made {
		t.Errorf("two runs making p#(3000) spell %s and %s", made, again)
	}
}
