package runtime

import (
	"errors"
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
		part def Operator { attribute id : Integer = 7; }
		part op : Operator;
		part op2 : Operator;
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
		if read := mustEvalIn(t, ctx, root, "test::scaledFour.r"); FormatValue(read) != "8" {
			t.Fatalf("scaledFour.r = %s, want 8", FormatValue(read))
		}
	})

	t.Run("a bound actor reads beside the subject and the verdict", func(t *testing.T) {
		for expr, want := range map[string]string{
			"test::checkThree.n":       "3",
			"test::checkThree.who.id":  "7",
			"test::checkThree.limit":   "10",
			"test::checkThree.verdict": "VerdictKind::pass",
		} {
			if got := FormatValue(mustEvalIn(t, ctx, root, expr)); got != want {
				t.Errorf("%s = %s, want %s", expr, got, want)
			}
		}
	})

	t.Run("an actor bound to several values keeps the range it redefines", func(t *testing.T) {
		if got := FormatValue(mustEvalIn(t, ctx, root, "test::checkMany.verdict")); got != "VerdictKind::pass" {
			t.Fatalf("checkMany.verdict = %s, want pass", got)
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

	t.Run("a name the case declares nothing under is an unknown output", func(t *testing.T) {
		_, err := evalIn(t, ctx, root, "test::checkThree.nope")
		if !errors.Is(err, errUnknownOutput) || !strings.Contains(err.Error(), "verdict") {
			t.Fatalf("checkThree.nope: error = %v, want errUnknownOutput naming verdict", err)
		}
	})
}
