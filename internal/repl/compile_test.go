package repl

import (
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/codegen"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// compiledCase invokes one calc; the interpreter is the oracle for what the
// compiled program must print, or which failure it must report.
type compiledCase struct {
	calc string
	args []string
}

var compiledCases = []compiledCase{
	{"Fib", []string{"0"}}, {"Fib", []string{"1"}}, {"Fib", []string{"20"}},
	{"SumTo", []string{"0"}}, {"SumTo", []string{"1000"}}, {"SumTo", []string{"200000"}},
	{"Arith", []string{"7", "3"}}, {"Arith", []string{"-7", "3"}}, {"Arith", []string{"7", "0"}},
	{"Arith", []string{"9223372036854775807", "1"}},
	{"Quot", []string{"7", "2"}}, {"Quot", []string{"1", "3"}}, {"Quot", []string{"-1", "3"}},
	{"Quot", []string{"1", "0"}}, {"Quot", []string{"9007199254740993", "1"}},
	{"Quot", []string{"9223372036854775807", "-9223372036854775808"}},
	{"Mixed", []string{"1.5", "3"}}, {"Mixed", []string{"0.1", "0"}},
	{"Pow", []string{"-4"}}, {"Pow", []string{"3037000500"}},
	{"RealPow", []string{"2.0", "0.5"}}, {"RealPow", []string{"0.0", "-1.0"}}, {"RealPow", []string{"-8.0", "0.5"}},
	{"RealPow", []string{"10.0", "400.0"}},
	{"Neg", []string{"5"}}, {"Neg", []string{"-9223372036854775808"}},
	{"Logic", []string{"false", "true", "0"}}, {"Logic", []string{"false", "false", "0"}},
	{"Logic", []string{"true", "true", "0"}}, {"Logic", []string{"true", "true", "5"}},
	{"Compare", []string{"2.0", "2"}}, {"Compare", []string{"2.5", "2"}}, {"Compare", []string{"0.1", "1"}},
	{"Sign", []string{"-9"}}, {"Sign", []string{"0"}}, {"Sign", []string{"3"}},
	{"FirstAbove", []string{"50"}}, {"FirstAbove", []string{"0"}},
	{"Collatz", []string{"27"}}, {"Collatz", []string{"1"}},
	{"Hypot", []string{"3.0", "4.0"}},
	{"Trailing", []string{"1.5", "2"}},
	{"Named", []string{"7", "3"}},
	{"Ratio", []string{"3", "4"}}, {"Ratio", []string{"0", "4"}},
	{"Even", []string{"7"}}, {"Even", []string{"10"}}, {"Even", []string{"100000"}}, // exceeds the recursion limit
	{"Big", []string{"1"}}, {"Big", []string{"0"}},
	{"Order", []string{"0", "9223372036854775807"}}, {"Order", []string{"3", "9223372036854775807"}},
	{"Order", []string{"3", "4"}},
	{"UntilLoop", []string{"0"}}, {"UntilLoop", []string{"-3"}}, {"UntilLoop", []string{"5"}},
	{"UntilLocal", []string{"0"}}, {"UntilLocal", []string{"3"}},
	{"WhileUntil", []string{"0"}}, {"WhileUntil", []string{"5"}}, {"WhileUntil", []string{"20"}},
	{"Hypot", []string{"1e-400", "4.0"}}, {"Hypot", []string{"1e-320", "4.0"}}, {"Hypot", []string{"1e400", "4.0"}},
	{"Hypot", []string{"0e-400", "4.0"}}, {"Hypot", []string{"3.0E0", "+4.0"}},
	{"fib", []string{"10"}}, {"Specialized", []string{"12"}}, {"ViaUsage", []string{"11"}},
	{"NamedOrder", []string{"0", "9223372036854775807"}}, {"NamedOrder", []string{"3", "4"}},
	{"OrderArgs", []string{"0", "9223372036854775807"}}, {"OrderArgs", []string{"3", "9223372036854775807"}},
	{"Nat", []string{"5"}}, {"Nat", []string{"0"}}, {"Nat", []string{"-1"}},
	{"Pos", []string{"2"}}, {"Pos", []string{"1"}}, {"Pos", []string{"0"}},
	{"PosLoc", []string{"2"}}, {"PosLoc", []string{"1"}},
	{"One", []string{"21"}},
	{"Collide", []string{"1"}},
	{"Lib::Sqrt", []string{"2.0"}}, {"Lib::Sqrt", []string{"0.0"}}, {"Lib::Sqrt", []string{"-1.0"}},
	{"Lib::Floor", []string{"2.5"}}, {"Lib::Floor", []string{"-2.5"}}, {"Lib::Floor", []string{"1e300"}}, {"Lib::Floor", []string{"9.3e18"}},
	{"Lib::RealExt", []string{"-1.5", "2.0"}}, {"Lib::RealExt", []string{"0.0", "-0.0"}},
	{"Lib::IntExt", []string{"-7", "3"}}, {"Lib::IntExt", []string{"-9223372036854775808", "0"}},
	{"Lib::MixedExt", []string{"2", "2.5"}}, {"Lib::MixedExt", []string{"3", "-0.5"}},
	{"Lib::IntAbs", []string{"-4"}}, {"Lib::IntAbs", []string{"-9223372036854775808"}},
	{"Lib::NatMax", []string{"3", "5"}}, {"Lib::NatMax", []string{"3", "-1"}}, {"Lib::NatMax", []string{"-2", "-1"}},
	{"Lib::Zero", []string{"0.0"}}, {"Lib::Zero", []string{"1.0"}}, {"Lib::Zero", []string{"0.5"}},
	{"Lib::Trig", []string{"0.5"}}, {"Lib::Trig", []string{"0.0"}}, {"Lib::Trig", []string{"1e308"}},
	{"Lib::Arc", []string{"0.5"}}, {"Lib::Arc", []string{"2.0"}},
	{"Lib::Deg", []string{"1.0"}}, {"Lib::Deg", []string{"1e308"}},
	{"Lib::Exp", []string{"1.0"}}, {"Lib::Exp", []string{"0.0"}}, {"Lib::Exp", []string{"710.0"}},
	{"Lib::Log", []string{"1000.0", "10.0"}}, {"Lib::Log", []string{"8.0", "2.0"}}, {"Lib::Log", []string{"9.0", "3.0"}},
	{"Lib::Log", []string{"9.0", "1.0"}}, {"Lib::Log", []string{"9.0", "0.0"}}, {"Lib::Log", []string{"-9.0", "3.0"}},
	{"Lib::Atan2", []string{"1.0", "1.0"}}, {"Lib::Atan2", []string{"0.0", "0.0"}}, {"Lib::Atan2", []string{"0.0", "-1.0"}},
	{"Lib::NamedLib", []string{"1.0", "-1.0"}},
	{"Seq::Sequence", []string{"5"}}, {"Seq::Sequence", []string{"0"}}, {"Seq::Library", []string{"2.5"}},
	{"Seq::Unbound", []string{"1"}},
	{"Seq::AddS", []string{"(1,2)"}}, {"Seq::AddS", []string{"null"}}, {"Seq::AddS", []string{"4"}}, {"Seq::AddS", []string{"(3)"}},
	{"Seq::AddS2", []string{"(1,2)"}}, {"Seq::AddS2", []string{"null"}}, {"Seq::AddS2", []string{"(3)"}},
	{"Seq::AddR", []string{"(1.5)"}}, {"Seq::AddR", []string{"null"}},
	{"Seq::LtS", []string{"(1)"}}, {"Seq::LtS", []string{"(1,2)"}}, {"Seq::LtS", []string{"null"}}, {"Seq::LtS", []string{"0"}},
	{"Seq::AndS", []string{"(true)"}}, {"Seq::AndS", []string{"null"}}, {"Seq::AndS", []string{"(true,false)"}},
	{"Seq::AndS2", []string{"(true)"}}, {"Seq::AndS2", []string{"null"}},
	{"Seq::NotS", []string{"(true)"}}, {"Seq::NotS", []string{"null"}}, {"Seq::NotS", []string{"(true,true)"}},
	{"Seq::IfS", []string{"(true)"}}, {"Seq::IfS", []string{"null"}}, {"Seq::IfS", []string{"false"}},
	{"Seq::NegS", []string{"(3)"}}, {"Seq::NegS", []string{"null"}}, {"Seq::NegS", []string{"(1,2)"}},
	{"Seq::EqS", []string{"(1)"}}, {"Seq::EqS", []string{"null"}}, {"Seq::EqS", []string{"(1,2)"}}, {"Seq::EqS", []string{"2"}},
	{"Seq::EqS2", []string{"(1)"}}, {"Seq::EqS2", []string{"null"}}, {"Seq::EqS2", []string{"(1,2)"}},
	{"Seq::IdS", []string{"(1)"}}, {"Seq::IdS", []string{"null"}}, {"Seq::IdS", []string{"(1,2)"}},
	{"Seq::IdN", []string{"null"}}, {"Seq::IdN", []string{"1"}}, {"Seq::IdN", []string{"()"}},
	{"Seq::IdE", []string{"(1,2)", "(1,2)"}}, {"Seq::IdE", []string{"null", "null"}}, {"Seq::IdE", []string{"null", "1"}}, {"Seq::IdE", []string{"1", "(1)"}},
	{"Seq::IxS", []string{"(1,2)", "2"}}, {"Seq::IxS", []string{"(1,2)", "0"}}, {"Seq::IxS", []string{"(1,2)", "3"}}, {"Seq::IxS", []string{"null", "1"}}, {"Seq::IxS", []string{"4", "1"}},
	{"Seq::PowS", []string{"(3)"}}, {"Seq::PowS", []string{"(1,2)"}},
	{"Seq::WhS", []string{"false"}}, {"Seq::WhS", []string{"(false)"}}, {"Seq::WhS", []string{"null"}},
	{"Seq::RetS", []string{"(1,2)"}}, {"Seq::RetS", []string{"null"}}, {"Seq::RetS", []string{"(4)"}},
	{"Seq::LocS", []string{"(1,2)"}}, {"Seq::LocS", []string{"4"}},
	{"Seq::LocU", []string{"4"}}, {"Seq::LocU2", []string{"4"}},
	{"Seq::MinE", []string{"(3,1,2)"}}, {"Seq::MinE", []string{"null"}}, {"Seq::MinE", []string{"7"}},
	{"Seq::MinR", []string{"(3,1)"}},
	{"Seq::RedN", []string{"(1,2,3)"}}, {"Seq::RedN", []string{"null"}}, {"Seq::RedN", []string{"(7)"}},
	{"Seq::FA", []string{"(2,4)"}}, {"Seq::FA", []string{"null"}}, {"Seq::FA", []string{"(-1,0)"}},
	{"Seq::Col2", []string{"(1,2,3)"}}, {"Seq::Col2", []string{"null"}}, {"Seq::Col2", []string{"()"}},
	{"Seq::Sz", []string{"(1,2)"}}, {"Seq::Sz", []string{"null"}}, {"Seq::Sz", []string{"5"}},
	{"Seq::Hd", []string{"(1,2)"}}, {"Seq::Hd", []string{"null"}}, {"Seq::Hd", []string{"()"}},
	{"Seq::Hd2", []string{"(1,2)"}}, {"Seq::Hd2", []string{"null"}}, {"Seq::Hd2", []string{"()"}},
	{"Seq::CmpS", []string{"5", "(5)"}}, {"Seq::CmpS", []string{"5", "(5,5)"}}, {"Seq::CmpS", []string{"null", "null"}},
	{"Seq::CmpS", []string{"(1,2)", "(1,2)"}}, {"Seq::CmpS", []string{"(1,2)", "(1,3)"}}, {"Seq::CmpS", []string{"null", "()"}}, {"Seq::CmpS", []string{"()", "()"}},
	{"Seq::Nst", []string{"(1,2,3)"}}, {"Seq::Nst", []string{"null"}},
	{"Seq::Rng2", []string{"5"}}, {"Seq::Rng2", []string{"-2"}},
	{"Seq::M2", []string{"(1,2)"}}, {"Seq::M2", []string{"(1)"}}, {"Seq::M2", []string{"null"}}, {"Seq::M2", []string{"(1,2,3,4)"}},
	{"Seq::Ret2", []string{"(1,2)"}}, {"Seq::Ret2", []string{"(1)"}}, {"Seq::Ret2", []string{"null"}},
	{"Seq::Ass", []string{"(1,2)"}}, {"Seq::Ass", []string{"(1,2,3)"}}, {"Seq::Ass", []string{"null"}},
	{"Seq::LocM1", []string{"(1)"}}, {"Seq::LocM1", []string{"(1,2)"}}, {"Seq::LocM1", []string{"null"}},
	{"Seq::LocN", []string{"(1,2)"}}, {"Seq::LocN", []string{"(1,-2)"}}, {"Seq::LocN", []string{"null"}},
	{"Seq::LocAny2", []string{"(1,2)"}}, {"Seq::LocAny2", []string{"(1)"}}, {"Seq::LocAny2", []string{"(1,2,3)"}},
	{"Seq::Ret1", []string{"(1)"}}, {"Seq::Ret1", []string{"(1,2)"}}, {"Seq::Ret1", []string{"null"}},
	{"Seq::RetN", []string{"(1,2)"}}, {"Seq::RetN", []string{"(-1,2)"}}, {"Seq::RetN", []string{"null"}},
	{"Seq::Coal", []string{"(1,2)"}}, {"Seq::Coal", []string{"null"}}, {"Seq::Coal", []string{"(3)"}}, {"Seq::Coal", []string{"()"}},
	{"Seq::Tern", []string{"true", "(1,2)"}}, {"Seq::Tern", []string{"false", "(1,2)"}}, {"Seq::Tern", []string{"true", "null"}},
	{"Seq::Big", []string{"1000"}}, {"Seq::Big", []string{"1000000"}}, // the second exceeds the element budget
	{"Seq::Big2", []string{"1000000"}},
	{"Seq::LazyAnd", []string{"(1,2)", "9"}}, {"Seq::LazyAnd", []string{"(1,2,3,4,5,6)", "9"}}, {"Seq::LazyAnd", []string{"(1,2,3,4,5,6)", "2"}},
	{"Seq::LazyOr", []string{"()", "9"}}, {"Seq::LazyOr", []string{"(1,2)", "9"}}, {"Seq::LazyOr", []string{"(1,-2)", "2"}},
	{"Seq::LazyImp", []string{"()", "9"}}, {"Seq::LazyImp", []string{"(1,2)", "9"}}, {"Seq::LazyImp", []string{"(1,2)", "1"}},
	{"Seq::LazyBig", []string{"()", "1000000"}}, {"Seq::LazyBig", []string{"(1)", "1000000"}}, {"Seq::LazyBig", []string{"(1)", "3"}},
	{"Seq::BodyLoc", []string{"(1,2,3)"}}, {"Seq::BodyLoc", []string{"null"}}, {"Seq::BodyLoc", []string{"4"}},
	{"Seq::BodySel", []string{"(1,2,3)"}}, {"Seq::BodySel", []string{"()"}},
	{"Seq::BodyUnread", []string{"(1,2,3)"}}, // the local is never read, so its failing initializer never runs
	{"Seq::BodyCond", []string{"(1,2)"}}, {"Seq::BodyCond", []string{"(1,2,3)"}},
	{"Seq::BodyDoc", []string{"(1,2,3)"}}, {"Seq::BodyDoc", []string{"null"}},
	{"Seq::Cnt", []string{"(1,2,3)"}}, {"Seq::Cnt", []string{"null"}}, {"Seq::Cnt", []string{"(1,3)"}},
	{"Seq::IncAt", []string{"(1,2)", "1"}}, {"Seq::IncAt", []string{"(1,2)", "3"}}, {"Seq::IncAt", []string{"(1,2)", "4"}}, {"Seq::IncAt", []string{"null", "1"}}, {"Seq::IncAt", []string{"null", "2"}},
	{"Seq::Sub3", []string{"(1,2,3)", "1", "2"}}, {"Seq::Sub3", []string{"(1,2,3)", "2", "1"}}, {"Seq::Sub3", []string{"(1,2,3)", "0", "2"}}, {"Seq::Sub3", []string{"(1,2,3)", "1", "4"}}, {"Seq::Sub3", []string{"null", "1", "1"}},
	{"Seq::ExAt", []string{"(1,2,3)", "1", "2"}}, {"Seq::ExAt", []string{"(1,2,3)", "0", "2"}}, {"Seq::ExAt", []string{"null", "1", "1"}}, {"Seq::ExAt", []string{"4", "1", "1"}},
	{"Seq::ForR", []string{"(1,2,3)"}}, {"Seq::ForR", []string{"null"}}, {"Seq::ForR", []string{"4"}},
	{"Seq::Churn", []string{"0"}}, {"Seq::Churn", []string{"7"}},
	{"Seq::ChurnFor", []string{"0"}}, {"Seq::ChurnFor", []string{"9"}},
	{"Seq::ChurnNest", []string{"0"}}, {"Seq::ChurnNest", []string{"5"}},
	{"Seq::UntilSeq", []string{"1"}}, {"Seq::UntilSeq", []string{"50"}}, {"Seq::UntilSeq", []string{"1000"}},
	{"Seq::ForB", []string{"4"}}, {"Seq::ForB", []string{"0"}},
	{"Seq::RS", []string{"(1,2,3)"}}, {"Seq::RS", []string{"null"}}, {"Seq::RS", []string{"4"}},
	{"Seq::MaxS", []string{"(1.5,2.5)"}}, {"Seq::MaxS", []string{"null"}},
	{"Seq::UniqI", []string{"(2,3)"}}, {"Seq::UniqI", []string{"(2,2)"}}, {"Seq::UniqI", []string{"(1,2)"}}, {"Seq::UniqI", []string{"null"}},
	{"Seq::UniqR", []string{"(1.5,2.5)"}}, {"Seq::UniqR", []string{"(0.0,-0.0)"}}, {"Seq::UniqR", []string{"(1.5,2.5,1.5)"}},
	{"Seq::UniqB", []string{"(true,false)"}}, {"Seq::UniqB", []string{"(false,true,false)"}},
	{"Seq::UniqAs", []string{"3"}}, {"Seq::UniqAs", []string{"4"}},
	{"Seq::UniqLoc", []string{"(1,2)"}}, {"Seq::UniqLoc", []string{"(1,1)"}},
	{"Seq::UniqLocInit", []string{"(1,2)"}}, {"Seq::UniqLocInit", []string{"(1,1)"}},
	{"Seq::UniqLocFree", []string{"(1,2)"}}, {"Seq::UniqLocFree", []string{"(1,1)"}},
	{"Seq::UniqLocAny", []string{"(1,1)"}},
	{"Overloads::PickInt", []string{"7"}}, {"Overloads::PickReal", []string{"7.0"}}, {"Overloads::PickFlag", []string{"true"}},
	{"Overloads::PickQualified", []string{"7"}}, {"Overloads::PickQualified", []string{"-7"}},
	{"Fn::ApplySq", []string{"3.0"}}, {"Fn::ApplySq", []string{"1e200"}},
	{"Fn::ApplyUsage", []string{"3.0"}},
	{"Fn::ApplyRecip", []string{"4.0"}}, {"Fn::ApplyRecip", []string{"0.0"}},
	{"Fn::ApplySqrt", []string{"2.0"}}, {"Fn::ApplySqrt", []string{"-1.0"}},
	{"Fn::ApplyFloor", []string{"-2.5"}}, {"Fn::ApplyFloor", []string{"1e300"}},
	{"Fn::CondApply", []string{"true", "3.0"}}, {"Fn::CondApply", []string{"false", "3.0"}},
	{"Fn::PassSq", []string{"4.0"}},
	{"Fn::IterateSq", []string{"3", "2.0"}}, {"Fn::IterateSq", []string{"0", "2.0"}}, {"Fn::IterateSq", []string{"12", "2.0"}},
	{"Fn::TwoSq", []string{"4.0"}},
	{"Fn::ByNameSq", []string{"3.0"}},
	{"Fn::IntBoth", []string{"1"}}, {"Fn::IntBoth", []string{"2"}},
	{"Fn::Fold2Add", []string{"1.5", "2.0"}},
	{"Fn::TypedSq", []string{"3.0"}}, {"Fn::TypedUsage", []string{"3.0"}}, {"Fn::TypedSub", []string{"3.0"}},
	{"Fn::QualSq", []string{"3.0"}}, {"Fn::QualByNameSq", []string{"3.0"}},
	{"Fn::QualRecipRange", []string{"(1.0,2.0)"}}, {"Fn::QualRecipRange", []string{"(1.0,0.0,2.0)"}},
	{"Fn::SqRange", []string{"(1.0,2.0,3.0)"}}, {"Fn::SqRange", []string{"null"}}, {"Fn::SqRange", []string{"2.0"}}, {"Fn::SqRange", []string{"()"}},
	{"Fn::RecipRange", []string{"(1.0,2.0)"}}, {"Fn::RecipRange", []string{"(1.0,0.0,2.0)"}},
	{"Fn::HalfDomain", []string{"(1.0,2.0)"}}, {"Fn::HalfDomain", []string{"null"}}, {"Fn::HalfDomain", []string{"()"}}, {"Fn::HalfDomain", []string{"2.0"}},
	{"Fn::IncBoth", []string{"(1,2)"}}, {"Fn::IncBoth", []string{"null"}},
	{"Fn::QuarterRange", []string{"(1,1)"}}, {"Fn::QuarterRange", []string{"(1,2,3)"}},
	{"Fn::SqrtRange", []string{"(4.0,9.0)"}}, {"Fn::SqrtRange", []string{"(4.0,-1.0)"}},
	{"Fn::UsageRange", []string{"(3.0)"}},
	{"Fn::SumRange", []string{"(1.0,2.0)"}}, {"Fn::SumRange", []string{"null"}},
	{"Fn::ParenSample", []string{"(2.0,3.0)"}}, {"Fn::ParenDomain", []string{"(2.0,4.0)"}}, {"Fn::ParenDomain", []string{"(2.0,0.0)"}}, {"Fn::ParenApply", []string{"3.0"}},
	{"Fn::NullDomain", []string{"1.0"}}, {"Fn::NullRange", []string{"1"}}, {"Fn::NullSqrtRange", []string{"1.0"}}, {"Fn::NullSum", []string{"1.5"}},
	{"Fn::NullAbsRange", []string{"1"}}, {"Fn::NullAbsDomain", []string{"1"}},
	{"Fn::BodySample", []string{"(1,2,3)"}}, {"Fn::BodySample", []string{"null"}}, {"Fn::BodySample", []string{"0"}},
	{"Fn::BodySampleRecip", []string{"(2.0,4.0)"}}, {"Fn::BodySampleRecip", []string{"(2.0,1.0,4.0)"}},
	{"Fn::BodySampleUnread", []string{"(2.0,0.0)"}},
	// The recursion budget is 10000 frames: the entry calc, Sample, and Down n+1 times.
	{"Fn::DownDirect", []string{"9998"}}, {"Fn::DownDirect", []string{"9999"}},
	{"Fn::DownRange", []string{"9997"}}, {"Fn::DownRange", []string{"9998"}},
	{"Fn::DownDomain", []string{"9997"}}, {"Fn::DownDomain", []string{"9998"}},
	// The domain is computed before Sample's frame: the entry calc and Deep n+1 times.
	{"Fn::DeepRange", []string{"9998"}}, {"Fn::DeepRange", []string{"9999"}},
	{"Fn::DeepDomain", []string{"9998"}}, {"Fn::DeepDomain", []string{"9999"}},
}

// transcendental calcs call libm functions whose last bit is the library's, so the
// C target may differ from Go's math by an ulp; every other value must agree exactly.
var transcendental = map[string]bool{
	"Lib::Trig": true, "Lib::Arc": true, "Lib::Exp": true, "Lib::Log": true, "Lib::Atan2": true, "Lib::NamedLib": true,
}

// withinUlps reports whether two printed Reals are at most n float64 steps apart.
func withinUlps(a, b string, n uint64) bool {
	x, err1 := strconv.ParseFloat(a, 64)
	y, err2 := strconv.ParseFloat(b, 64)
	if err1 != nil || err2 != nil || (x < 0) != (y < 0) {
		return false
	}
	bx, by := math.Float64bits(math.Abs(x)), math.Float64bits(math.Abs(y))
	if bx < by {
		bx, by = by, bx
	}
	return bx-by <= n
}

// failureClass is the part of a failure both surfaces spell the same way: the
// class of a scalar fault, else the whole message once the interpreter's
// context labels and the program's calc name are stripped.
func failureClass(calc, msg string) string {
	for _, class := range []string{"arithmetic overflow", "arithmetic domain", "division by zero", "calc recursion limit exceeded", "typed by Natural", "typed by Positive", "requires Natural", "collection element limit exceeded"} {
		if strings.Contains(msg, class) {
			return class
		}
	}
	msg = strings.TrimSpace(msg)
	if _, rest, ok := strings.Cut(msg, "Compiled::"+calc+": "); ok {
		msg = rest
	}
	for again := true; again; {
		again = false
		for _, label := range []string{"evaluating the returned expression: ", "calculation body: ", "result: "} {
			if rest, ok := strings.CutPrefix(msg, label); ok {
				msg, again = rest, true
			}
		}
	}
	return msg
}

// interpreted answers a case with the interpreter: the value, or the failure.
func interpreted(t *testing.T, s *Session, c compiledCase) (value, failure string) {
	t.Helper()
	v := s.RunCalc("Compiled::" + c.calc + "(" + strings.Join(c.args, ", ") + ")")
	if v.Status == VerdictHolds {
		for _, line := range v.Lines {
			if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "= "); ok {
				return rest, ""
			}
		}
		t.Fatalf("%s%v: no value in %q", c.calc, c.args, v.Lines)
	}
	return "", failureClass(c.calc, strings.Join(verdictLines(v), "\n"))
}

// verdictLines is the verdict without its standing line, which no compiled program prints.
func verdictLines(v Verdict) []string {
	var lines []string
	for _, line := range v.Lines {
		if !strings.HasPrefix(line, standingPrefix) {
			lines = append(lines, line)
		}
	}
	return lines
}

// compiledRun answers a case with the executable: the value, or the failure.
func compiledRun(t *testing.T, exe string, c compiledCase) (value, failure string) {
	t.Helper()
	out, err := exec.Command(exe, c.args...).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err == nil {
		return text, ""
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("%s %v: %v\n%s", exe, c.args, err, text)
	}
	return "", failureClass(c.calc, text)
}

// loadCompileFixture loads the fixture into a session whose step budget is
// lifted: compiled code has none, so the oracle must run each case to its end.
func loadCompileFixture(t testing.TB) *Session {
	t.Helper()
	data, err := os.ReadFile("testdata/compile_calcs.sysml")
	if err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(string(data)).Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture has errors: %v", errs)
	}
	budgets := runtime.DefaultBudgets()
	budgets.MaxSteps = 1 << 40
	if err := s.SetBudgets(budgets); err != nil {
		t.Fatal(err)
	}
	return s
}

// The compiled program computes what the interpreter computes, prints it as
// the interpreter prints it, and fails where the interpreter fails.
func TestCompiledCalcsAgreeWithInterpreter(t *testing.T) {
	for _, target := range codegen.Targets() {
		t.Run(string(target), func(t *testing.T) {
			if target == codegen.TargetC {
				if _, err := exec.LookPath("cc"); err != nil {
					t.Skip("no C compiler on PATH")
				}
			}
			s := loadCompileFixture(t)
			exes := map[string]string{}
			dir := t.TempDir()
			for _, c := range compiledCases {
				exe, built := exes[c.calc]
				if !built {
					program, err := s.CompileCalc("Compiled::" + c.calc)
					if err != nil {
						t.Fatalf("compile %s: %v", c.calc, err)
					}
					exe = filepath.Join(dir, c.calc)
					if err := codegen.Build(program, target, exe); err != nil {
						t.Fatalf("build %s: %v", c.calc, err)
					}
					exes[c.calc] = exe
				}
				wantValue, wantFailure := interpreted(t, s, c)
				gotValue, gotFailure := compiledRun(t, exe, c)
				if gotFailure == wantFailure && gotValue != wantValue && target == codegen.TargetC && transcendental[c.calc] && withinUlps(gotValue, wantValue, 2) {
					continue
				}
				if gotValue != wantValue || gotFailure != wantFailure {
					t.Errorf("%s(%s): compiled = (%q, %q), interpreted = (%q, %q)",
						c.calc, strings.Join(c.args, ", "), gotValue, gotFailure, wantValue, wantFailure)
				}
			}
			for _, repeat := range []string{"0", "-1", "x", "2x", ""} {
				out, err := exec.Command(exes["Fib"], "--repeat", repeat, "10").CombinedOutput()
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 2 {
					t.Errorf("--repeat %q: got %v %q, want usage and exit status 2", repeat, err, out)
				}
			}
			for _, arg := range []string{"inf", "-inf", "nan", "1x", "", "0x1p-2", "0x1.8p1", "1_000.5", " 1.5", "1.5 ", "+", ".", "1e", "1e+", "Infinity"} {
				out, err := exec.Command(exes["Hypot"], arg, "4.0").CombinedOutput()
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(out), "is not a finite Real") {
					t.Errorf("Hypot(%q, 4.0): got %v %q, want not a Real and exit status 2", arg, err, out)
				}
			}
			if out, err := exec.Command(exes["Fib"], "--repeat", "3", "10").Output(); err != nil || strings.TrimSpace(string(out)) != "55" {
				t.Errorf("--repeat 3: got %q, %v", out, err)
			}
		})
	}
}

// The element budget charges the arguments a run holds, as the interpreter's
// charges the literals it evaluates them from; a lowered OPENSYSML_MAX_ELEMENTS
// makes that visible with small inputs. The Real copy an Integer collection
// widens to is charged too: the interpreter keeps Integers in Real slots and
// holds no copy, so at the limit the program fails where the interpreter runs,
// never the reverse.
func TestCompiledBudgetChargesInputsAndWidening(t *testing.T) {
	const limit = 10
	cases := []compiledCase{
		{"Seq::BigIn", []string{"(1,2,3,4,5,6,7,8,9,10)"}}, {"Seq::BigIn", []string{"(1,2,3,4,5,6,7,8,9,10,11)"}},
		{"Seq::LazyBig", []string{"()", "100"}}, {"Seq::LazyBig", []string{"(1)", "100"}},
		// A Domain or Range read collects a sequence of its own: 8 held plus 2 more fit, 9 plus 2 do not.
		{"Fn::SizeDomain", []string{"2", "8"}}, {"Fn::SizeDomain", []string{"2", "9"}},
		{"Fn::SizeRange", []string{"2", "8"}}, {"Fn::SizeRange", []string{"2", "9"}},
		// Two pairs are 6 elements over a domain of 2 selected from 2 (10 fit) or from 3 (11 do not).
		{"Fn::SamplePairs", []string{"2"}}, {"Fn::SamplePairs", []string{"3"}},
	}
	// 5 Integers and their 5 Reals fit the budget of 10; 6 and 6 do not.
	widened := []struct{ n, sum, want, failure string }{{"5", "15.0", "15.0", ""}, {"6", "21.0", "", "collection element limit exceeded"}}
	for _, target := range codegen.Targets() {
		t.Run(string(target), func(t *testing.T) {
			if target == codegen.TargetC {
				if _, err := exec.LookPath("cc"); err != nil {
					t.Skip("no C compiler on PATH")
				}
			}
			s := loadCompileFixture(t)
			budgets := runtime.DefaultBudgets()
			budgets.MaxSteps = 1 << 40
			budgets.MaxElements = limit
			if err := s.SetBudgets(budgets); err != nil {
				t.Fatal(err)
			}
			t.Setenv(runtime.MaxElementsEnvVar, strconv.Itoa(limit))
			dir := t.TempDir()
			exes := map[string]string{}
			for _, c := range cases {
				exe, built := exes[c.calc]
				if !built {
					program, err := s.CompileCalc("Compiled::" + c.calc)
					if err != nil {
						t.Fatalf("compile %s: %v", c.calc, err)
					}
					exe = filepath.Join(dir, c.calc)
					if err := codegen.Build(program, target, exe); err != nil {
						t.Fatalf("build %s: %v", c.calc, err)
					}
					exes[c.calc] = exe
				}
				wantValue, wantFailure := interpreted(t, s, c)
				gotValue, gotFailure := compiledRun(t, exe, c)
				if gotValue != wantValue || gotFailure != wantFailure {
					t.Errorf("%s(%s): compiled = (%q, %q), interpreted = (%q, %q)",
						c.calc, strings.Join(c.args, ", "), gotValue, gotFailure, wantValue, wantFailure)
				}
			}
			// The arguments stay charged across repeats: the second run must not find
			// room the first did not have.
			out, err := exec.Command(exes["Seq::BigIn"], "--repeat", "3", "(1,2,3,4,5,6,7,8,9,10,11)").CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(out), "element limit exceeded") {
				t.Errorf("--repeat over budget: got %v %q, want the element limit failure", err, out)
			}
			if out, err := exec.Command(exes["Seq::BigIn"], "--repeat", "3", "(1,2,3,4,5,6,7,8,9,10)").Output(); err != nil || strings.TrimSpace(string(out)) != "10" {
				t.Errorf("--repeat within budget: got %q, %v", out, err)
			}
			program, err := s.CompileCalc("Compiled::Seq::BigW")
			if err != nil {
				t.Fatal(err)
			}
			exe := filepath.Join(dir, "BigW")
			if err := codegen.Build(program, target, exe); err != nil {
				t.Fatal(err)
			}
			for _, w := range widened {
				c := compiledCase{"Seq::BigW", []string{w.n}}
				if v, f := compiledRun(t, exe, c); v != w.want || f != w.failure {
					t.Errorf("BigW(%s): compiled = (%q, %q), want (%q, %q)", w.n, v, f, w.want, w.failure)
				}
				if v, f := interpreted(t, s, c); v != w.sum || f != "" {
					t.Errorf("BigW(%s): interpreted = (%q, %q), want (%q, \"\"): it holds no widened copy", w.n, v, f, w.sum)
				}
			}
		})
	}
}

// A C loop's memory is bounded by what it keeps live, not by how many passes
// it makes: gigabytes of dead temporaries complete under a 64 MB limit.
func TestCompiledCLoopMemoryIsBounded(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("no C compiler on PATH")
	}
	if out, err := exec.Command("sh", "-c", "ulimit -v 65536").CombinedOutput(); err != nil {
		t.Skipf("no address-space limit here: %v %s", err, out)
	}
	s := loadCompileFixture(t)
	dir := t.TempDir()
	for calc, arg := range map[string]string{"Churn": "50000", "ChurnFor": "50000", "ChurnNest": "10000"} {
		program, err := s.CompileCalc("Compiled::Seq::" + calc)
		if err != nil {
			t.Fatal(err)
		}
		exe := filepath.Join(dir, calc)
		if err := codegen.Build(program, codegen.TargetC, exe); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("sh", "-c", `ulimit -v 65536 && exec "$0" "$1"`, exe, arg).CombinedOutput()
		if err != nil {
			t.Errorf("%s(%s) under a 64 MB limit: %v\n%s", calc, arg, err, out)
		}
	}
}

// A calc outside the subset is refused with the reason, never compiled wrong.
func TestCompileRefusesWhatItCannotCompile(t *testing.T) {
	s := loadCompileFixture(t)
	for _, tc := range []struct{ calc, reason string }{
		{"StringResult", "String"},
		{"Defaulted", "default value"},
		{"OutOnly", "`out`"},
		{"RealToNatural", "requires Integer arguments"},
		{"Refined", "members of its own"},
		{"DynamicIntPow", "non-literal Integer exponent"},
		{"Narrowed", "a Real bound to x, which is Integer"},
		{"RecordParam", "type Refused::Point is not Integer, Real or Boolean"},
		{"EnumParam", "type Refused::Color is not Integer, Real or Boolean"},
		{"Extent", "operator 'all'"},
		{"RealIntIdentity", "'===' between Real and Integer"},
		{"SelectNonBoolean", "select whose body yields Integer, not a Boolean"},
		{"CollectNull", "collect whose body yields null"},
		{"CoalesceWidened", "a Real at the right operand of '??', which holds Integer[0..*]"},
		{"MixedEquality", "a Integer[0..*] at the left operand of '==', which holds Real[0..*]"},
		{"MixedSame", "same over Integer and Real collections"},
		{"MixedUnion", "union over Integer and Real collections"},
		{"CalcParam", "parameter f binds a function value, which a program cannot take on its command line"},
		{"EscapingParam", "the function value f escaping as the result"},
		{"ReturnedFunction", "an invocation where a function value is expected"},
		{"FunctionEquality", "the function value f where a value is expected"},
		{"FunctionAsValue", "the function value Refused::Sq where a value is expected"},
		{"ValueAsFunction", "a, a Real, where a function value is expected"},
		{"ChosenFunction", "an `if` choosing a function value at run time"},
		{"WrongArity", "Refused::Add2 takes 2 arguments, 1 given"},
		{"WrongName", "Refused::Sq2 has no parameter v"},
		{"UnrelatedTyped", `in calc Refused::UnrelatedTyped: argument for parameter "f": cannot bind the function value Refused::Sq2 to a parameter typed by Compiled::Fn::Sq`},
		{"LibraryTyped", "cannot bind the function value RealFunctions::sqrt to a parameter typed by Compiled::Fn::Sq"},
		{"GeneralForSub", "cannot bind the function value Compiled::Fn::Sq to a parameter typed by Compiled::Fn::SubSq"},
		{"ForwardedUnrelated", "cannot bind the function value Refused::Sq2 to a parameter typed by Compiled::Fn::Sq"},
		{"IntegerNullRange", "a Real[0..*] at result, which holds Integer[0..*]"},
		{"BodyClosure", "a body-local calc usage"},
		{"OuterClosure", "a calc declared in the body of Refused::BodyClosure, whose function value closes over that run's bindings"},
		{"ObjectClosure", "a calc read off an object through a feature chain, whose function value closes over that object"},
		{"ObjectCalc", "a calc owned by Refused::Scaler, whose function value closes over that object"},
		{"ReceiverQualified", "an invocation of a function value with a receiver (`x->f()`)"},
		{"ForeignQualified", "in calc Compiled::Fn::ApplyQual::f: an `in calc` parameter invoked outside the body of the calc declaring it"},
		{"SampleArity", "Sample of Refused::Add2, which does not take one value argument"},
		{"SampledValue", "s, a SampledFunction, where a value is expected"},
		{"SampledEscapes", "type SampledFunctions::SampledFunction is not Integer, Real or Boolean"},
		{"UnevaluatedFunction", "ControlFunctions::collect binds its arguments unevaluated and cannot be read as a value"},
		{"SetParam", "type Collections::Set is not Integer, Real or Boolean"},
		{"SetElements", "type Collections::Set is not Integer, Real or Boolean"},
		{"SetLocal", "type Collections::Set is not Integer, Real or Boolean"},
		{"SubsetLocal", "attribute ys redefines or subsets a feature, inheriting a shape it does not state"},
		{"TensorParam", "type Quantities::TensorQuantityValue is not Integer, Real or Boolean"},
		{"TensorBuilt", "type Quantities::TensorQuantityValue is not Integer, Real or Boolean"},
		{"MetaCast", "a `meta` cast, whose metaobject reflects a model element and has no native representation"},
	} {
		_, err := s.CompileCalc("Refused::" + tc.calc)
		if err == nil {
			t.Errorf("%s: compiled, want a refusal mentioning %q", tc.calc, tc.reason)
			continue
		}
		if !errors.Is(err, codegen.ErrUnsupported) {
			t.Errorf("%s: %v is not an ErrUnsupported", tc.calc, err)
		}
		if !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("%s: %v does not mention %q", tc.calc, err, tc.reason)
		}
	}
	if _, err := s.CompileCalc("Compiled::Nowhere"); err == nil {
		t.Error("an unknown calc compiled")
	}
}

// A call several visible declarations fit equally is refused, naming them,
// as the checker and the interpreter report it.
func TestCompileRefusesAmbiguousCall(t *testing.T) {
	s := NewSession()
	s.Submit(`package Amb {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; return : Integer = 1; } }
		package B { calc def pick { in x : Integer; return : Integer = 2; } }
		private import A::*;
		private import B::*;
		calc def Pick { in n : Integer; return : Integer = pick(n); }
	}`)
	_, err := s.CompileCalc("Amb::Pick")
	if !errors.Is(err, codegen.ErrUnsupported) {
		t.Fatalf("got %v, want an ErrUnsupported refusal", err)
	}
	for _, want := range []string{"ambiguous", "Amb::A::pick", "Amb::B::pick"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%v does not mention %q", err, want)
		}
	}
}

// A call binding one parameter by name twice is refused by the type checker;
// the native target declines it rather than binding the later value.
func TestCompileRefusesAParameterBoundTwice(t *testing.T) {
	s := NewSession()
	res := s.Submit(`package Twice {
		private import ScalarValues::*;
		calc def Add { in a : Integer; in b : Integer; return : Integer = a + b; }
		calc def Dup { in x : Integer; return : Integer = Add(a = x, a = 1, b = 2); }
	}`)
	errs := errorDiagnostics(res.Diagnostics)
	if len(errs) != 1 || !strings.Contains(errs[0].Message, `Add binds parameter "a" twice`) {
		t.Fatalf("diagnostics = %v, want Add binds parameter \"a\" twice", errs)
	}
	_, err := s.CompileCalc("Twice::Dup")
	if !errors.Is(err, codegen.ErrUnsupported) || !strings.Contains(err.Error(), "binds parameter a twice") {
		t.Fatalf("CompileCalc(Twice::Dup) = %v, want an ErrUnsupported naming the parameter", err)
	}
}

// The generated source is deterministic and names the calc it came from.
func TestCompiledSourceNamesTheCalc(t *testing.T) {
	s := loadCompileFixture(t)
	program, err := s.CompileCalc("Compiled::Fib")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range codegen.Targets() {
		src, err := codegen.Source(program, target)
		if err != nil {
			t.Fatal(err)
		}
		again, _ := codegen.Source(program, target)
		if string(src) != string(again) {
			t.Errorf("%s: two renderings differ", target)
		}
		if !strings.Contains(string(src), "Compiled::Fib") {
			t.Errorf("%s: source does not name Compiled::Fib", target)
		}
	}
}
