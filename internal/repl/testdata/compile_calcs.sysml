// Calculations in the scalar subset the native compiler accepts, each exercising
// a rule the generated code must keep: checked Integer arithmetic, Integer
// division answering a Real, mixed-type widening, short-circuit Booleans,
// branches, loops, early returns and recursion.
package Compiled {
	private import ScalarValues::*;

	calc def Fib {
		in n : Integer;
		return : Integer = if n < 2 ? n else Fib(n - 1) + Fib(n - 2);
	}

	calc def SumTo {
		in n : Integer;
		attribute total : Integer = 0;
		attribute i : Integer = 1;
		while i <= n {
			assign total := total + i;
			assign i := i + 1;
		}
		return : Integer = total;
	}

	calc def Arith {
		in a : Integer;
		in b : Integer;
		return : Integer = (a + b) * (a - b) + a % b - -a;
	}

	calc def Quot {
		in a : Integer;
		in b : Integer;
		return : Real = a / b;
	}

	calc def Mixed {
		in a : Real;
		in b : Integer;
		return : Real = (a + b) / 2 - b * 0.5 + a ** 2;
	}

	calc def Pow {
		in a : Integer;
		return : Integer = a ** 3;
	}

	calc def RealPow {
		in a : Real;
		in b : Real;
		return : Real = a ** b;
	}

	calc def Neg {
		in a : Integer;
		return : Integer = -a;
	}

	calc def Logic {
		in p : Boolean;
		in q : Boolean;
		in n : Integer;
		// The right operands of `and`/`or`/`implies` divide by n only when reached.
		return : Boolean = (p and 10 / n > 1) or (q implies 10 % n == 0) or not p;
	}

	calc def Compare {
		in a : Real;
		in b : Integer;
		return : Boolean = a < b or a == b or a >= b and a != b;
	}

	calc def Sign {
		in x : Integer;
		attribute r : Integer = 0;
		if x > 0 {
			assign r := 1;
		} else if x < 0 {
			assign r := -1;
		}
		return : Integer = r;
	}

	calc def FirstAbove {
		in limit : Integer;
		attribute i : Integer = 0;
		while true {
			assign i := i + 1;
			if i * i > limit {
				return : Integer = i;
			}
		}
	}

	calc def Collatz {
		in n : Integer;
		attribute x : Real = n;
		attribute steps : Integer = 0;
		while x != 1.0 {
			if x % 2.0 == 0.0 {
				assign x := x / 2;
			} else {
				assign x := 3 * x + 1;
			}
			assign steps := steps + 1;
		}
		return : Integer = steps;
	}

	calc def Hypot {
		in a : Real;
		in b : Real;
		attribute s : Real = a * a + b * b;
		return : Real = s ** 0.5;
	}

	calc def Trailing {
		in a : Real;
		in b : Integer;
		(a + b) / 2
	}

	calc def Named {
		in lhs : Integer;
		in rhs : Integer;
		return : Real = Arith(b = rhs, a = lhs) + Quot(lhs, rhs) * 0;
	}

	calc def Ratio {
		in a : Integer;
		in b : Integer;
		return : Real = Quot(a, b) + Quot(b, a);
	}

	calc def Even {
		in n : Integer;
		return : Boolean = if n == 0 ? true else Odd(n - 1);
	}

	calc def Odd {
		in n : Integer;
		return : Boolean = if n == 0 ? false else Even(n - 1);
	}

	calc def Big {
		in a : Integer;
		return : Real = a * 1.0e300 * 1.0e10;
	}

	// Both operands can fail; the left one must be reported, as the interpreter does.
	calc def Order {
		in a : Integer;
		in b : Integer;
		return : Integer = (a % a) + (b + b);
	}

	// The post-condition is tested after each iteration: the body runs at
	// least once, even when the condition already holds.
	calc def UntilLoop {
		in n : Integer;
		attribute i : Integer = 0;
		loop {
			assign i := i + 1;
		} until i >= n;
		return : Integer = i;
	}

	// The post-condition resolves in the body's scope: it reads the body's
	// `done`, not the outer one.
	calc def UntilLocal {
		in n : Integer;
		attribute i : Integer = 0;
		attribute done : Boolean = false;
		loop {
			assign i := i + 1;
			attribute done : Boolean = i >= n;
		} until done;
		return : Integer = i;
	}

	calc def WhileUntil {
		in n : Integer;
		attribute i : Integer = 0;
		while i < n {
			assign i := i + 3;
		} until i > 7;
		return : Integer = i;
	}

	calc fib : Fib;

	calc def Specialized :> Fib;

	calc def ViaUsage {
		in n : Integer;
		return : Integer = fib(n) + Specialized(n = n);
	}

	calc def NamedOrder {
		in a : Integer;
		in b : Integer;
		return : Integer = Arith(b = b + b, a = a % a);
	}

	calc def OrderArgs {
		in a : Integer;
		in b : Integer;
		return : Integer = Arith(a % a, b + b);
	}

	calc def Nat {
		in n : Natural;
		return : Natural = n - 1;
	}

	calc def Pos {
		in n : Positive;
		attribute k : Positive = n;
		assign k := k - 1;
		return : Integer = k;
	}

	calc def One {
		in n : Integer[1];
		attribute m : Integer[1..1] = n;
		return : Integer[1] = m * 2;
	}

	// Names whose naive encodings collide: X::Y against the unrestricted name 'X::Y',
	// a Unicode name against the text of its own escape.
	package X {
		calc def Y { in n : Integer; return : Integer = n + 1; }
	}
	calc def 'X::Y' { in n : Integer; return : Integer = n + 2; }
	calc def 'Größe' { in n : Integer; return : Integer = n + 3; }
	calc def 'Gr_f6__df_e' { in n : Integer; return : Integer = n + 4; }
	calc def Collide {
		in n : Integer;
		return : Integer = X::Y(n) * 1000 + 'X::Y'(n) * 100 + 'Größe'(n) * 10 + 'Gr_f6__df_e'(n);
	}

	// Library functions: qualified and imported names, kind-preserving
	// max/min/abs, Natural parameters, and the extension library's domain errors.
	package Lib {
		private import RealFunctions::floor;
		private import RealFunctions::round;
		private import TrigFunctions::*;
		private import OpenSysMLMathFunctions::*;

		calc def Sqrt { in a : Real; return : Real = RealFunctions::sqrt(a); }
		calc def Floor { in a : Real; return : Integer = floor(a) + round(a); }
		calc def RealExt { in a : Real; in b : Real; return : Real = RealFunctions::max(a, b) - RealFunctions::min(a, b) + RealFunctions::abs(a); }
		calc def IntExt { in a : Integer; in b : Integer; return : Integer = NumericalFunctions::max(a, b) - NumericalFunctions::min(a, b) + NumericalFunctions::abs(a); }
		calc def MixedExt { in a : Integer; in b : Real; return : Real = NumericalFunctions::max(a, b) + RationalFunctions::min(a, b); }
		calc def IntAbs { in a : Integer; return : Integer = IntegerFunctions::abs(a); }
		calc def NatMax { in a : Integer; in b : Integer; return : Integer = NaturalFunctions::max(a, b) + NaturalFunctions::min(b, a); }
		calc def Zero { in a : Real; return : Boolean = NumericalFunctions::isZero(a) or NumericalFunctions::isUnit(a); }
		calc def Trig { in t : Real; return : Real = sin(t) + cos(t) + tan(t) + cot(t); }
		calc def Arc { in x : Real; return : Real = arcsin(x) + arccos(x) + arctan(x); }
		calc def Deg { in t : Real; return : Real = deg(t) + rad(t) + pi; }
		calc def Exp { in x : Real; return : Real = exp(x) + ln(x); }
		calc def Log { in x : Real; in base : Real; return : Real = log(x, base); }
		calc def Atan2 { in y : Real; in x : Real; return : Real = atan2(y, x); }
		calc def NamedLib { in y : Real; in x : Real; return : Real = atan2(x = x, y = y); }
	}

	// Collections: homogeneous sequences with the interpreter's shape rules
	// (null, one value, many), one-based indexing, ranges, the sequence and
	// control library, `for`, and the scalarization of one-valued operands.
	package Seq {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		private import ControlFunctions::*;
		private import NumericalFunctions::*;
		private import CollectionFunctions::contains;

		calc def Sequence { in n : Integer; return : Integer = SequenceFunctions::size(1..n); }
		calc def Library { in a : Real; return : Real = RealFunctions::sum(a); }
		calc def Unbound { in n : Integer; attribute x : Integer; return : Integer = x + n; }
		calc def AddS { in x : Integer[0..*]; return : Integer = x + 1; }
		calc def AddS2 { in x : Integer[0..*]; return : Integer = 1 + x; }
		calc def AddR { in x : Real[0..*]; return : Real = x + 1.0; }
		calc def LtS { in x : Integer[0..*]; return : Boolean = x < 1; }
		calc def AndS { in x : Boolean[0..*]; return : Boolean = x and true; }
		calc def AndS2 { in x : Boolean[0..*]; return : Boolean = true and x; }
		calc def NotS { in x : Boolean[0..*]; return : Boolean = not x; }
		calc def IfS { in x : Boolean[0..*]; return : Integer = if x ? 1 else 2; }
		calc def NegS { in x : Integer[0..*]; return : Integer = -x; }
		calc def EqS { in x : Integer[0..*]; return : Boolean = x == 1; }
		calc def EqS2 { in x : Integer[0..*]; return : Boolean = x == (1); }
		calc def IdS { in x : Integer[0..*]; return : Boolean = x === 1; }
		calc def IdN { in x : Integer[0..*]; return : Boolean = x === null; }
		calc def IdE { in x : Integer[0..*]; in y : Integer[0..*]; return : Boolean = x === y; }
		calc def IxS { in x : Integer[0..*]; in i : Integer; return : Integer = x#(i); }
		calc def PowS { in x : Integer[0..*]; return : Integer = x ** 2; }
		calc def WhS { in x : Boolean[0..*]; attribute n : Integer = 0; while x { assign n := n + 1; } return : Integer = n; }
		calc def RetS { in x : Integer[0..*]; return : Integer = x; }
		calc def LocS { in x : Integer[0..*]; attribute y : Integer = x; return : Integer = y; }
		calc def LocU { in x : Integer[0..*]; attribute y : Integer; return : Integer[0..*] = y; }
		calc def LocU2 { in x : Integer; attribute y : Integer[0..*]; assign y := x; return : Integer[0..*] = y; }
		calc def MinE { in x : Integer[0..*]; return : Integer = x->minimize {in v; v}; }
		calc def MinR { in x : Integer[0..*]; return : Real = x->minimize {in v; v / 2}; }
		calc def RedN { in x : Integer[0..*]; return : Integer = x->reduce {in a; in b; a + b} + 1; }
		calc def FA { in x : Integer[0..*]; return : Boolean = x->forAll {in v; v > 0 or v / 0 > 1}; }
		calc def Col2 { in x : Integer[0..*]; return : Integer[0..*] = x->collect {in v; (v, v)}; }
		calc def Sz { in x : Integer[0..*]; return : Integer = size(x) + x->size(); }
		calc def Hd { in x : Integer[0..*]; return : Integer = head(x); }
		calc def Hd2 { in x : Integer[0..*]; return : Integer[0..1] = head(x); }
		calc def CmpS { in x : Integer[0..*]; in y : Integer[0..*]; return : Boolean = x == y; }
		calc def Nst { in x : Integer[0..*]; return : Integer[0..*] = x->collect {in v; x->select {in w; w > v}}; }
		calc def Rng2 { in n : Integer; return : Integer = sum(1..n); }
		calc def M2 { in x : Integer[2..3]; return : Integer = size(x); }
		calc def Ret2 { in x : Integer[0..*]; return : Integer[2..3] = x; }
		calc def Ass { in x : Integer[0..*]; attribute y : Integer[1..2] = x; return : Integer = size(y); }
		calc def Ret1 { in x : Integer[0..*]; return : Integer[1] = x; }
		calc def RetN { in x : Integer[0..*]; return : Natural[0..*] = x; }
		calc def Coal { in x : Integer[0..*]; return : Integer = x ?? 5; }
		calc def Tern { in b : Boolean; in x : Integer[0..*]; return : Integer[0..*] = if b ? x else 7; }
		calc def Big { in n : Integer; return : Integer = size((1..n)->collect {in v; (v, v)}); }
		calc def Big2 { in n : Integer; attribute s : Integer[0..*] = 1..n; attribute t : Integer[0..*] = 1..n; return : Integer = size(s) + size(t); }
		calc def BigIn { in x : Integer[0..*]; return : Integer = size(x); }
		calc def BigW { in n : Integer; return : Real = RealFunctions::sum(1..n); }
		calc def LazyAnd { in x : Integer[0..*]; in n : Integer; return : Boolean = size(x) > 5 and x#(n) > 0; }
		calc def LazyOr { in x : Integer[0..*]; in n : Integer; return : Boolean = isEmpty(x) or x#(n) > 0; }
		calc def LazyImp { in x : Integer[0..*]; in n : Integer; return : Boolean = notEmpty(x) implies x#(n) > 0; }
		calc def LazyBig { in x : Integer[0..*]; in n : Integer; return : Boolean = isEmpty(x) or size((1..n)->collect {in v; (v, v)}) > 0; }
		calc def BodyLoc { in x : Integer[0..*]; return : Integer[0..*] = x->collect {in v : Integer; attribute d : Integer = v * 2; d + 1}; }
		calc def BodySel { in x : Integer[0..*]; return : Integer[0..*] = x->select {in v : Integer; attribute big : Boolean = v > 1; big}; }
		calc def BodyUnread { in x : Integer[0..*]; return : Integer[0..*] = x->collect {in v : Integer; attribute d : Integer = x#(9); v + 1}; }
		calc def BodyCond { in x : Integer[0..*]; return : Integer[0..*] = x->collect {in v : Integer; attribute d : Integer = x#(9); if v > 2 ? d else v}; }
		calc def BodyDoc { in x : Integer[0..*]; return : Integer[0..*] = x->collect {doc /* doubled, plus one */ in v : Integer; attribute d : Integer = v * 2; d + 1}; }
		calc def Cnt { in x : Integer[0..*]; return : Boolean = contains(x, 2); }
		calc def IncAt { in x : Integer[0..*]; in i : Integer; return : Integer[0..*] = includingAt(x, 9, i); }
		calc def Sub3 { in x : Integer[0..*]; in i : Integer; in j : Integer; return : Integer[0..*] = subsequence(x, i, j); }
		calc def ExAt { in x : Integer[0..*]; in i : Integer; in j : Integer; return : Integer[0..*] = excludingAt(x, i, j); }
		calc def ForR { in x : Integer[0..*]; attribute s : Real = 0.0; for v in x { assign s := s + v; } return : Real = s; }
		calc def Churn { in n : Integer; attribute i : Integer = 0; attribute total : Integer = 0; attribute keep : Integer[0..*] = (); while i < n { attribute s : Integer[0..*] = 1..100; assign total := total + size(s); assign keep := including(keep, i); assign i := i + 1; } return : Integer = total + size(keep); }
		calc def ChurnFor { in n : Integer; attribute keep : Real[0..*] = (); for i in 1..n { attribute s : Real[0..*] = (1..100)->collect {in v : Integer; v / 2}; if i % 2 == 0 { assign keep := including(keep, s#(i % 100 + 1)); } } return : Real[0..*] = keep; }
		calc def UntilSeq { in n : Integer; attribute i : Integer = 0; attribute keep : Integer[0..*] = (); loop { assign i := i + 1; attribute s : Integer[0..*] = including((), i * 10); assign keep := union(keep, (size(s) + 100, size(s) + 101, size(s) + 102)); } until s#(size(s)) >= n or size(including(s, i)) > 1 and i > 200; return : Integer[0..*] = keep; }
		calc def ChurnNest { in n : Integer; attribute keep : Integer[0..*] = (); attribute inner : Integer[0..*] = (); attribute i : Integer = 0; while i < n { assign inner := (); for v in 1..50 { assign inner := including(inner, v); } assign keep := including(keep, size(inner)); for w in keep { assign i := i + 0; } assign i := i + 1; } return : Integer = size(keep) + size(inner); }
		calc def ForB { in n : Integer; attribute s : Integer = 0; for v in 1..n { for w in 1..v { assign s := s + w; } } return : Integer = s; }
		calc def RS { in x : Integer[0..*]; return : Real = RealFunctions::sum(x); }
		calc def MaxS { in x : Real[0..*]; return : Real = x->maximize {in v; v}; }
	}

	// Overloads: the declaration a call compiles to is the one its arguments
	// select, not the first its name is visible as.
	package Overloads {
		package ByFlag { calc def pick { in x : Boolean; return : Integer = 1; } }
		package ByInt { calc def pick { in x : Integer; return : Integer = x * 2; } }
		package ByReal { calc def pick { in x : Real; return : Real = x / 2.0; } }
		package Both {
			public import ByFlag::*;
			public import ByInt::*;
		}
		private import ByFlag::*;
		private import ByReal::*;
		private import ByInt::*;

		calc def PickInt { in n : Integer; return : Integer = pick(n); }
		calc def PickReal { in r : Real; return : Real = pick(r); }
		calc def PickFlag { in b : Boolean; return : Integer = pick(b); }
		calc def PickQualified { in n : Integer; return : Integer = Both::pick(n) + Both::pick(n > 0); }
	}

	// Function values: a calc bound to an `in calc` parameter is compiled once
	// per calc it is applied to, so every call through the parameter is direct.
	package Fn {
		private import ScalarValues::*;
		private import SampledFunctions::*;

		calc def Sq { in v : Real; return : Real = v * v; }
		calc def Half { in v : Real; return : Real = v / 2.0; }
		calc def Recip { in v : Real; return : Real = 1.0 / v; }
		calc def Inc { in n : Integer; return : Integer = n + 1; }
		calc def Quarter { in n : Integer; return : Integer = n * 4611686018427387904; }
		calc def AddBoth { in x : Real; in y : Real; return : Real = x + y; }
		// A calc usage with an unsupplied input reads as a function value.
		calc sqUsage : Sq;

		calc def Apply { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
		calc def ApplySq { in a : Real; return : Real = Apply(Sq, a); }
		calc def ApplyUsage { in a : Real; return : Real = Apply(sqUsage, a); }
		calc def ApplyRecip { in a : Real; return : Real = Apply(Recip, a); }
		calc def ApplySqrt { in a : Real; return : Real = Apply(RealFunctions::sqrt, a); }
		calc def ToInt { in calc f { in v : Real; return : Integer; } in a : Real; return : Integer = f(a); }
		calc def ApplyFloor { in a : Real; return : Integer = ToInt(RealFunctions::floor, a); }
		calc def CondApply { in b : Boolean; in a : Real; return : Real = if b ? Apply(Sq, a) else Apply(Half, a); }
		// The parameter is passed on, and the same calc is applied to two functions.
		calc def Pass { in calc g { in v : Real; return : Real; } in a : Real; return : Real = Apply(g, a) + Apply(Half, a); }
		calc def PassSq { in a : Real; return : Real = Pass(Sq, a); }
		calc def Iterate { in calc f { in v : Real; return : Real; } in n : Integer; in a : Real; return : Real = if n == 0 ? a else Iterate(f, n - 1, f(a)); }
		calc def IterateSq { in n : Integer; in a : Real; return : Real = Iterate(Sq, n, a); }
		calc def Two { in calc f { in v : Real; return : Real; } in calc g { in v : Real; return : Real; } in a : Real; return : Real = f(a) - g(a); }
		calc def TwoSq { in a : Real; return : Real = Two(Sq, Half, a) * Two(Half, Sq, a); }
		// A named argument names the applied calc's own parameter.
		calc def ByName { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(v = a); }
		calc def ByNameSq { in a : Real; return : Real = ByName(a = a, f = Sq); }
		calc def IntApply { in calc f { in n : Integer; return : Integer; } in n : Integer; return : Integer = f(n); }
		calc def IntBoth { in n : Integer; return : Integer = IntApply(Inc, n) + IntApply(Quarter, n); }
		// A two-argument calc applied through a two-argument parameter.
		calc def Fold2 { in calc f { in x : Real; in y : Real; return : Real; } in a : Real; in b : Real; return : Real = f(a, b); }
		calc def Fold2Add { in a : Real; in b : Real; return : Real = Fold2(AddBoth, a, b); }

		// Sampling: the domain in order, the calc at each, failing at the first that fails.
		calc def RangeOf { in calc f { in v : Real; return : Real; } in xs : Real[0..*]; attribute s : SampledFunction = Sample(f, xs); return : Real[0..*] = Range(s); }
		calc def SqRange { in xs : Real[0..*]; return : Real[0..*] = RangeOf(Sq, xs); }
		calc def RecipRange { in xs : Real[0..*]; return : Real[0..*] = RangeOf(Recip, xs); }
		calc def HalfDomain { in xs : Real[0..*]; return : Real[0..*] = Domain(Sample(Half, xs)); }
		calc def IncBoth { in xs : Integer[0..*]; attribute s : SampledFunction = Sample(Inc, xs); return : Integer[0..*] = SequenceFunctions::union(Domain(s), Range(s)); }
		calc def QuarterRange { in xs : Integer[0..*]; return : Integer[0..*] = Range(Sample(Quarter, xs)); }
		calc def SqrtRange { in xs : Real[0..*]; return : Real[0..*] = Range(Sample(RealFunctions::sqrt, xs)); }
		calc def UsageRange { in xs : Real[0..*]; return : Real[0..*] = Range(Sample(sqUsage, xs)); }
		calc def SumRange { in xs : Real[0..*]; return : Real = RealFunctions::sum(Range(Sample(Sq, xs))); }
	}
}

// Calculations outside the subset, each refused for the reason its name states.
package Refused {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	private import ControlFunctions::*;
	private import Collections::*;
	private import Quantities::*;
	private import MeasurementReferences::*;
	private import SI::*;

	// Sets and tensors have no native layout: a calc taking, holding or building one is refused.
	calc def SetParam { in s : Set; return : Integer = CollectionFunctions::size(s); }
	calc def SetElements { in s : Set; return : Integer[0..*] = s.elements; }
	calc def SetLocal { in n : Integer; attribute s : Set { :>> elements = (n, n); } return : Integer = CollectionFunctions::size(s); }
	calc def TensorParam { in t : TensorQuantityValue; return : Real = t.num#(1, 1, 1); }
	attribute cubeRef : TensorMeasurementReference { :>> dimensions = (2, 2, 2); :>> mRefs = (m, m, m, m, m, m, m, m); }
	calc def TensorBuilt { in x : Real; return : TensorQuantityValue = TensorCalculations::'['((x, x, x, x, x, x, x, x), cubeRef); }

	attribute def Point { attribute x : Real; attribute y : Real; }
	enum def Color { red; green; }
	calc def RecordParam { in p : Point; return : Real = p.x + p.y; }
	calc def EnumParam { in c : Color; return : Boolean = c == Color::red; }
	calc def RealIntIdentity { in x : Real; in y : Integer; return : Boolean = x === y; }
	calc def SelectNonBoolean { in x : Integer[0..*]; return : Integer[0..*] = x->select {in v; v}; }
	calc def CollectNull { in x : Integer[0..*]; return : Integer[0..*] = x->collect {in v; null}; }
	calc def CoalesceWidened { in x : Integer[0..*]; return : Real = x ?? 5.5; }
	calc def MixedEquality { in x : Integer[0..*]; in y : Real[0..*]; return : Boolean = x == y; }
	calc def MixedSame { in x : Integer[0..*]; in y : Real[0..*]; return : Boolean = same(x, y); }
	calc def MixedUnion { in x : Integer[0..*]; in y : Real[0..*]; return : Real[0..*] = union(x, y); }

	// Function values: a program takes no function on its command line, and a
	// function value is compiled only where it is applied or passed on.
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def Sq2 { in x : Real; return : Real = x * x; }
	calc def Add2 { in x : Real; in y : Real; return : Real = x + y; }
	calc def CalcParam { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc def Identity { in calc f { in v : Real; return : Real; } return r = f; }
	calc def EscapingParam { in a : Real; return r = Identity(Sq); }
	calc def ReturnedFunction { in a : Real; return : Real = Compiled::Fn::Apply(Identity(Sq), a); }
	calc def Same { in calc f { in v : Real; return : Real; } in calc g { in v : Real; return : Real; } return : Boolean = f == g; }
	calc def FunctionEquality { in a : Real; return : Boolean = Same(Sq, Sq2); }
	calc def FunctionAsValue { in a : Real; return : Real = Compiled::Fn::Apply(Sq, Sq); }
	calc def ValueAsFunction { in a : Real; return : Real = Compiled::Fn::Apply(a, a); }
	calc def ChosenFunction { in b : Boolean; in a : Real; return : Real = Compiled::Fn::Apply(if b ? Sq else Sq2, a); }
	calc def WrongArity { in a : Real; return : Real = Compiled::Fn::Apply(Add2, a); }
	calc def WrongName { in a : Real; return : Real = Compiled::Fn::ByName(Sq2, a); }
	calc def BodyClosure { in k : Real; in a : Real; calc scale { in v : Real; return : Real = v * k; } return : Real = Compiled::Fn::Apply(scale, a); }
	part def Scaler { attribute k : Real; calc scale { in v : Real; return : Real = v * k; } }
	part twice : Scaler { attribute :>> k = 2.0; }
	calc def OuterClosure { in a : Real; return : Real = Compiled::Fn::Apply(BodyClosure::scale, a); }
	calc def ObjectClosure { in a : Real; return : Real = Compiled::Fn::Apply(twice.scale, a); }
	calc def ObjectCalc { in a : Real; return : Real = Compiled::Fn::Apply(Scaler::scale, a); }
	calc def SampleArity { in xs : Real[0..*]; return : Real[0..*] = SampledFunctions::Range(SampledFunctions::Sample(Add2, xs)); }
	calc def SampledValue { in xs : Real[0..*]; attribute s : SampledFunctions::SampledFunction = SampledFunctions::Sample(Sq, xs); return : Integer = CollectionFunctions::size(s); }
	calc def SampledEscapes { in xs : Real[0..*]; return : SampledFunctions::SampledFunction = SampledFunctions::Sample(Sq, xs); }
	calc def UnevaluatedFunction { in xs : Integer[0..*]; return : Integer[0..*] = SampledFunctions::Range(SampledFunctions::Sample(ControlFunctions::collect, xs)); }

	calc def StringResult {
		in s : String;
		return : String = s;
	}

	calc def Defaulted {
		in n : Integer = 3;
		return : Integer = n;
	}

	calc def OutOnly {
		in n : Integer;
		out r : Integer = n;
	}

	calc def RealToNatural {
		in a : Real;
		return : Integer = NaturalFunctions::max(a, 1);
	}

	// Members of its own would redefine what it inherits; that is not compiled.
	calc def Refined :> Compiled::Fib {
		in n : Integer;
	}

	// The interpreter stores the Real quotient in an Integer attribute; the
	// compiler would have to pick a representation, so it declines.
	calc def Narrowed {
		in n : Integer;
		attribute x : Integer = n;
		assign x := x / 2;
		return : Integer = x;
	}

	calc def DynamicIntPow {
		in a : Integer;
		in b : Integer;
		return : Integer = a ** b;
	}
}
