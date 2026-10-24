package runtime

import (
	"errors"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"strings"
	"testing"
)

// TestRuntimeRobustnessCaseMemberReads covers reading a case usage's features other
// than its outputs: a subject, actor or input nothing binds is undetermined rather than
// a refusal, a null or a verdict; an actor bound to several values keeps the range it
// redefines; a name the case declares nothing under stays an unknown output.
func TestRuntimeRobustnessCaseMemberReads(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		private import VerificationCases::*;
		part def Operator { attribute id : Integer default = 7; }
		part op : Operator;
		part op2 : Operator { attribute :>> id = 9; }
		verification def CheckPositive {
			subject n : Integer;
			actor who : Operator;
			actor many : Operator[0..*];
			in limit : Integer = 10;
			in open : Integer;
			in vals : Integer[0..*];
			return verdict : VerdictKind = PassIf(n > 0 and n <= limit);
		}
		verification checkThree : CheckPositive { subject n = 3; actor who = op; }
		verification checkNothing : CheckPositive;
		verification checkMany : CheckPositive { subject n = 3; actor many = (op, op2); }
		analysis def Scale {
			subject n : Integer;
			actor who : Operator;
			return r : Integer = n * 2;
		}
		analysis scaledFour : Scale { subject n = 4; }
		verification def Crewed {
			subject n : Integer;
			actor pilot : Operator;
			attribute pilotId : Integer = pilot.id;
			return verdict : VerdictKind = PassIf(n > 0 and pilot.id > 0);
		}
		verification crewed : Crewed { subject n = 3; actor crew = op2; }
		verification def Geared {
			subject n : Integer;
			part gear : Operator { attribute w : Integer = n * 2; }
			return verdict : VerdictKind = PassIf(n > 0);
		}
		part def Holder {
			attribute value : Integer default = 3;
			verification check : Geared { subject n = value; }
		}
		part holder : Holder;
		part other : Holder { :>> value = 5; }
	}`
	ctx, idx := libraryShapeContext(t, src)
	root := idx.DocumentRoot("<test>")

	t.Run("an unbound subject reads undetermined, never pass and never unknown output", func(t *testing.T) {
		read := mustEvalIn(t, ctx, root, "test::checkNothing.n")
		if read.Kind != ValUndetermined {
			t.Fatalf("checkNothing.n = %s, want <undetermined>", FormatValue(read))
		}
		_, err := evalIn(t, ctx, root, "test::checkNothing.verdict")
		if !errors.Is(err, errUnboundSubject) {
			t.Fatalf("checkNothing.verdict: error = %v, want the unbound subject", err)
		}
	})

	t.Run("an unbound actor is undetermined and the case still runs", func(t *testing.T) {
		if read := mustEvalIn(t, ctx, root, "test::scaledFour.who"); read.Kind != ValUndetermined {
			t.Fatalf("scaledFour.who = %s, want <undetermined>", FormatValue(read))
		}
		if read := mustEvalIn(t, ctx, root, "test::scaledFour.r"); FormatValue(read) != "[8]" {
			t.Fatalf("scaledFour.r = %s, want [8]", FormatValue(read))
		}
	})

	t.Run("a bound actor reads beside the subject and the verdict", func(t *testing.T) {
		for expr, want := range map[string]string{
			"test::checkThree.n":       "3",
			"test::checkThree.who.id":  "7",
			"test::checkThree.limit":   "10",
			"test::checkThree.verdict": "[VerdictKind::pass]",
		} {
			if got := FormatValue(mustEvalIn(t, ctx, root, expr)); got != want {
				t.Errorf("%s = %s, want %s", expr, got, want)
			}
		}
	})

	t.Run("an actor bound to several values keeps the range it redefines", func(t *testing.T) {
		if got := FormatValue(mustEvalIn(t, ctx, root, "test::checkMany.verdict")); got != "[VerdictKind::pass]" {
			t.Fatalf("checkMany.verdict = %s, want [pass]", got)
		}
		many := mustEvalIn(t, ctx, root, "test::checkMany.many")
		if many.Kind != ValSequence || len(elementsOf(many)) != 2 {
			t.Fatalf("checkMany.many = %s, want two actors", FormatValue(many))
		}
	})

	t.Run("a parameter nothing supplies reads undetermined whatever its lower bound", func(t *testing.T) {
		for _, expr := range []string{"test::checkThree.open", "test::checkThree.vals", "test::checkThree.many"} {
			if read := mustEvalIn(t, ctx, root, expr); read.Kind != ValUndetermined {
				t.Errorf("%s = %s, want <undetermined> whatever the lower bound", expr, FormatValue(read))
			}
		}
	})

	t.Run("an actor renamed at its position binds the actor it redefines", func(t *testing.T) {
		for expr, want := range map[string]string{
			"test::crewed.crew.id": "9",
			"test::crewed.pilotId": "9",
			"test::crewed.verdict": "[VerdictKind::pass]",
		} {
			if got := FormatValue(mustEvalIn(t, ctx, root, expr)); got != want {
				t.Errorf("%s = %s, want %s", expr, got, want)
			}
		}
	})

	t.Run("a case on two owners reads two parts, each with its owner's values", func(t *testing.T) {
		if got := FormatValue(mustEvalIn(t, ctx, root, "test::holder.check.gear.w")); got != "6" {
			t.Errorf("holder.check.gear.w = %s, want 6", got)
		}
		if got := FormatValue(mustEvalIn(t, ctx, root, "test::other.check.gear.w")); got != "10" {
			t.Errorf("other.check.gear.w = %s, want 10", got)
		}
		if got := FormatValue(mustEvalIn(t, ctx, root, "test::holder.check.gear != test::other.check.gear")); got != "true" {
			t.Errorf("the two owners' gears compare %s, want distinct objects", got)
		}
	})

	t.Run("restoring a snapshot forgets the occurrence made since, so a reuse of its identity is not it", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, src)
		root := idx.DocumentRoot("<test>")
		holder := mustEvalIn(t, ctx, root, "test::holder")
		before, err := ctx.Snapshot()
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		gear := mustEvalIn(t, ctx, root, "test::holder.check.gear")
		gearInst, _ := ctx.Instance(gear.Instance)
		occurrence := gearInst.owner.ID
		before.Restore()
		if _, live := ctx.instances[occurrence]; live {
			t.Fatalf("the check occurrence #%d survives the restore", occurrence)
		}
		taker, err := ctx.Instantiate(namedOrFoundSymbol(t, idx, "test::Operator", root, ast.DefPart, ast.UsagePart))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		if taker.ID != occurrence {
			t.Fatalf("the unrelated object is #%d, want it to take the freed #%d", taker.ID, occurrence)
		}
		if again := mustEvalIn(t, ctx, root, "test::holder"); again.Instance != holder.Instance {
			t.Fatalf("holder after the restore = #%d, want #%d", again.Instance, holder.Instance)
		}
		again := mustEvalIn(t, ctx, root, "test::holder.check.gear")
		againInst, _ := ctx.Instance(again.Instance)
		if againInst.owner == nil || againInst.owner.ID == taker.ID || againInst.owner.Type != gearInst.owner.Type {
			t.Fatalf("holder.check.gear after the restore is held by %s, want a fresh occurrence of the check",
				FormatValue(Value{Kind: ValInstance, Instance: againInst.owner.ID}))
		}
		if got := FormatValue(mustEvalIn(t, ctx, root, "test::holder.check.gear.w")); got != "6" {
			t.Errorf("holder.check.gear.w after the restore = %s, want 6", got)
		}
	})

	t.Run("an image of the owner carries the occurrence its case is", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, src)
		root := idx.DocumentRoot("<test>")
		holder := mustEvalIn(t, ctx, root, "test::other")
		gear := mustEvalIn(t, ctx, root, "test::other.check.gear")
		owner, _ := ctx.Instance(holder.Instance)
		dst := imageInto(t, ctx, owner)
		if _, carried := dst.Instance(gear.Instance); !carried {
			t.Fatalf("gear #%d is not in the image", gear.Instance)
		}
		made := len(dst.created)
		copied := mustEvalIn(t, dst, root, "test::other.check.gear")
		if copied.Instance != gear.Instance || len(dst.created) != made {
			t.Fatalf("other.check.gear in the copy = #%d (%d objects made), want the imported #%d and none made",
				copied.Instance, len(dst.created)-made, gear.Instance)
		}
		if got := FormatValue(mustEvalIn(t, dst, root, "test::other.check.gear.w")); got != "10" {
			t.Errorf("other.check.gear.w in the copy = %s, want 10", got)
		}
	})

	t.Run("a name the case declares nothing under is an unknown output", func(t *testing.T) {
		_, err := evalIn(t, ctx, root, "test::checkThree.nope")
		if !errors.Is(err, errUnknownOutput) || !strings.Contains(err.Error(), "verdict") {
			t.Fatalf("checkThree.nope: error = %v, want errUnknownOutput naming verdict", err)
		}
	})
}
