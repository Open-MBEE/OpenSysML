// Calculations in the scalar subset the native compiler accepts, each exercising
// a rule the generated code must keep: unbounded Integer arithmetic, Integer
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

	calc def PosLoc {
		in n : Integer;
		attribute k : Positive = n - 1;
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
		calc def NotS { in x : Boolean[0..*] nonunique; return : Boolean = not x; }
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
		calc def Col2 { in x : Integer[0..*]; return : Integer[0..*] nonunique = x->collect {in v; (v, v)}; }
		calc def Sz { in x : Integer[0..*]; return : Integer = size(x) + x->size(); }
		calc def Hd { in x : Integer[0..*]; return : Integer = head(x); }
		calc def Hd2 { in x : Integer[0..*]; return : Integer[0..1] = head(x); }
		calc def CmpS { in x : Integer[0..*]; in y : Integer[0..*] nonunique; return : Boolean = x == y; }
		calc def Nst { in x : Integer[0..*]; return : Integer[0..*] nonunique = x->collect {in v; x->select {in w; w > v}}; }
		calc def Rng2 { in n : Integer; return : Integer = sum(1..n); }
		calc def M2 { in x : Integer[2..3]; return : Integer = size(x); }
		calc def Ret2 { in x : Integer[0..*]; return : Integer[2..3] = x; }
		calc def Ass { in x : Integer[0..*]; attribute y : Integer[1..2] = x; return : Integer = size(y); }
		calc def LocM1 { in x : Integer[0..*]; attribute y : Integer[1] = x; return : Integer = size(y); }
		calc def LocN { in x : Integer[0..*]; attribute y : Natural[0..*] = x; return : Integer = size(y); }
		calc def LocAny2 { in x : Integer[0..*]; attribute y[2] = x; return : Integer = size(y); }
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
		calc def ChurnFor { in n : Integer; attribute keep : Real[0..*] nonunique = (); for i in 1..n { attribute s : Real[0..*] = (1..100)->collect {in v : Integer; v / 2}; if i % 2 == 0 { assign keep := including(keep, s#(i % 100 + 1)); } } return : Real[0..*] nonunique = keep; }
		calc def UntilSeq { in n : Integer; attribute i : Integer = 0; attribute keep : Integer[0..*] nonunique = (); loop { assign i := i + 1; attribute s : Integer[0..*] = including((), i * 10); assign keep := union(keep, (size(s) + 100, size(s) + 101, size(s) + 102)); } until s#(size(s)) >= n or size(including(s, i)) > 1 and i > 200; return : Integer[0..*] nonunique = keep; }
		calc def ChurnNest { in n : Integer; attribute keep : Integer[0..*] nonunique = (); attribute inner : Integer[0..*] = (); attribute i : Integer = 0; while i < n { assign inner := (); for v in 1..50 { assign inner := including(inner, v); } assign keep := including(keep, size(inner)); for w in keep { assign i := i + 0; } assign i := i + 1; } return : Integer = size(keep) + size(inner); }
		calc def ForB { in n : Integer; attribute s : Integer = 0; for v in 1..n { for w in 1..v { assign s := s + w; } } return : Integer = s; }
		calc def RS { in x : Integer[0..*]; return : Real = RealFunctions::sum(x); }
		calc def MaxS { in x : Real[0..*]; return : Real = x->maximize {in v; v}; }
		// Unique by default: a repeated argument, result or assigned value is refused.
		calc def UniqI { in x : Integer[0..*]; return : Integer[0..*] = including(x, 1); }
		calc def UniqR { in x : Real[0..*]; return : Real[0..*] = x; }
		calc def UniqB { in x : Boolean[0..*]; return : Boolean[0..*] = x; }
		calc def UniqAs { in n : Integer; attribute keep : Integer[0..*] = (); for i in 1..n { assign keep := including(keep, i % 3); } return : Integer = size(keep); }
		calc def UniqLoc { in x : Integer[0..*]; attribute y : Integer[0..*] = x; return : Integer = size(y); }
		calc def UniqLocInit { in x : Integer[0..*] nonunique; attribute y : Integer[0..*] = x; return : Integer = size(y); }
		calc def UniqLocFree { in x : Integer[0..*] nonunique; attribute y : Integer[0..*] nonunique = x; return : Integer = size(y); }
		calc def UniqLocAny { in x : Integer[0..*] nonunique; attribute y = x; return : Integer = size(y); }
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

	// Function values computed at run time: chosen, stored, returned, compared,
	// collected, and closures over the run of the body declaring them.
	package Closure {
		private import ScalarValues::*;
		private import ControlFunctions::*;
		private import SequenceFunctions::*;
		private import Base::*;
		private import Compiled::Fn::Apply;

		calc def Sq { in v : Real; return : Real = v * v; }
		calc def Half { in v : Real; return : Real = v / 2.0; }
		calc def Inc { in n : Integer; return : Integer = n + 1; }
		calc sqU : Sq;
		calc halfU : Half;
		calc incU : Inc;
		calc def Id { in calc f { in v : Real; return : Real; } return r = f; }
		calc def Pick { in b : Boolean; return r = if b ? sqU else halfU; }
		calc def PickId { in b : Boolean; return r = Id(if b ? Sq else Half); }
		calc def PickApply { in b : Boolean; in a : Real; return : Real = Apply(Pick(b), a); }
		calc def Chosen { in b : Boolean; in a : Real; return : Real = Apply(if b ? Sq else Half, a); }
		calc def Stored { in b : Boolean; in a : Real; attribute g = if b ? sqU else halfU; return : Real = Apply(g, a) + Apply(g, 1.0); }
		calc def Reassigned { in b : Boolean; in a : Real; attribute g = sqU; if b { assign g := halfU; } return : Real = Apply(g, a); }
		calc def Eq { in b : Boolean; in c : Boolean; return : Boolean = Pick(b) == Pick(c); }
		calc def Ident { in b : Boolean; in c : Boolean; return : Boolean = Pick(b) === Pick(c); }
		calc def Neq { in b : Boolean; return : Boolean = Pick(b) != sqU; }
		calc def EqNum { in b : Boolean; return : Boolean = Pick(b) == 3; }
		calc def Same { in calc f { in v : Real; return : Real; } in calc g { in v : Real; return : Real; } return : Boolean = f == g; }
		calc def SameSq { in b : Boolean; return : Boolean = Same(Sq, if b ? Sq else Half); }
		calc def Pair { return r : Anything[0..*] = (sqU, halfU); }
		calc def PairUntyped { return r = (sqU, halfU); }
		calc def PairApply { in a : Real; attribute s = (sqU, halfU); return : Real[0..*] = s->collect {in f; Apply(f, a)}; }
		calc def MixedKind { in b : Boolean; in a : Real; return : Real = Apply(if b ? sqU else RealFunctions::sqrt, a); }
		calc def ToStr { in b : Boolean; return : String = BaseFunctions::ToString(Pick(b)); }
		calc def ReturnedApply { in a : Real; return : Real = Apply(Id(Sq), a); }
		calc def NullF { in b : Boolean; return r = if b ? sqU else null; }
		calc def Includes { in b : Boolean; return r = (sqU, halfU)->includes(Pick(b)); }
		calc def Unique { in b : Boolean; attribute s : Anything[0..*] nonunique = (sqU, Pick(b)); attribute u : Anything[0..*] = s; return r = u->size(); }
		calc def SeqFmt { in b : Boolean; return r : Anything[0..*] = (Pick(b), halfU); }
		calc def Indexed { in b : Boolean; attribute s = (Pick(b), halfU); return r = Apply(s#(1), 2.0); }
		calc def Selected { in b : Boolean; attribute s = (sqU, halfU, Pick(b)); return r = s->select {in f; f == sqU}->size(); }
		// Functions are not numbers, Booleans or ordered.
		calc def Lt { in b : Boolean; return : Boolean = Pick(b) < Pick(b); }
		calc def Plus { in b : Boolean; return r = Pick(b) + 1; }
		calc def Neg { in b : Boolean; return r = -Pick(b); }
		calc def Not { in b : Boolean; return r = not Pick(b); }
		calc def MulR { in b : Boolean; return r = 2 * Pick(b); }
		calc def CondF { in b : Boolean; return r = if Pick(b) ? 1 else 2; }
		calc def StmtIf { in b : Boolean; attribute x : Integer = 1; if Pick(b) { assign x := 2; } return r = x; }

		// Closures: a body-local calc reads the bindings of the run declaring it.
		calc def Local { in k : Real; in a : Real; calc scale { in v : Real; return : Real = v * k; } return : Real = Apply(scale, a) + scale(1.0); }
		calc def Mk { in k : Real; calc s { in v : Real; return : Real = v * k; } return r = s; }
		calc def MkApply { in k : Real; in a : Real; return : Real = Apply(Mk(k), a); }
		calc def MkSame { in k : Real; calc s { in v : Real; return : Real = v * k; } return : Boolean = s == s; }
		calc def MkTwice { in k : Real; return : Boolean = Mk(k) == Mk(k); }
		calc def MkPair { in k : Real; attribute f = Mk(k); attribute g = Mk(k + 1.0); return r = Apply(f, 1.0) + Apply(g, 1.0); }
		calc def Late { in k : Real; attribute m : Real = k + 1.0; calc s { in v : Real; return : Real = v * m; } return : Real = Apply(s, 2.0); }
		calc def Rec { in k : Real; calc s { in v : Real; return : Real = if v > 3.0 ? v else s(v + k); } return r = s(1.0); }
		calc def Sib { in k : Real; calc u { in v : Real; return : Real = v + k; } calc s { in v : Real; return r = u; } return r = Apply(s(1.0), 2.0); }
		calc def SibEq { in k : Real; calc u { in v : Real; return : Real = v + k; } calc s { in v : Real; return r = u; } return r = s(1.0) == u; }
		calc def Inner { in k : Real; calc s { in v : Real; calc t { in w : Real; return : Real = w + k + v; } return : Real = t(1.0); } return : Real = s(2.0); }
		calc def ChooseClosure { in b : Boolean; in k : Real; calc s { in v : Real; return : Real = v * k; } attribute g = if b ? s else sqU; return r = Apply(g, 3.0); }
		calc def IntClosure { in n : Integer; calc s { in m : Integer; return : Integer = m + n; } return r = s(n); }
		calc def ClosureSeq { in k : Real; calc s { in v : Real; return : Real = v + k; } attribute fs = (s, sqU, s); return r = fs->collect {in f; Apply(f, 2.0)}; }
		calc def ClosureUnique { in k : Real; attribute fs : Anything[0..*] nonunique = (Mk(k), Mk(k)); attribute u : Anything[0..*] = fs; return r = u->size(); }
		// What a closure captures: every kind of value a body binds.
		calc def ApplyS { in calc f { in n : Integer; return : String; } in n : Integer; return : String = f(n); }
		calc def MkStr { in s : String; calc f { in n : Integer; return : String = StringFunctions::Substring(s, 1, n); } return r = f; }
		calc def StrClosure { in s : String; in n : Integer; return : String = ApplyS(MkStr(s), n); }
		calc def SeqClosure { in k : Real; attribute xs : Real[0..*] = (k, k * 2.0); calc f { in v : Real; return : Real = v * xs#(2); } return : Real = Apply(f, 3.0); }
		calc def FnClosure { in b : Boolean; attribute g = Pick(b); calc f { in v : Real; return : Real = Apply(g, v) + 1.0; } return : Real = Apply(f, 3.0); }
		calc def BoolClosure { in b : Boolean; calc f { in v : Real; return : Real = if b ? v else -v; } return : Real = Apply(f, 2.0); }
		calc def EnumClosure { in c : Compiled::E::Color; calc f { in v : Real; return : Real = if c == Compiled::E::Color::red ? v else 0.0; } return : Real = Apply(f, 2.0); }
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
		// A parameter typed by a calc takes that calc, a usage typed by it, or a
		// specialization of it, and passes it on to another typed parameter.
		calc def SubSq :> Sq;
		calc def ApplyTyped { in calc f : Sq { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
		calc def ApplyTypedOnly { in calc f : Sq; in a : Real; return : Real = ApplyTyped(f, a); }
		calc def TypedSq { in a : Real; return : Real = ApplyTyped(Sq, a) + ApplyTypedOnly(Sq, a); }
		calc def TypedUsage { in a : Real; return : Real = ApplyTyped(sqUsage, a) + ApplyTypedOnly(sqUsage, a); }
		calc def TypedSub { in a : Real; return : Real = ApplyTyped(SubSq, a) + ApplyTypedOnly(SubSq, a); }
		// A parameter is named by its qualified name too, through the calc declaring
		// it, and a member-less specialization runs the body it inherits.
		calc def ApplyQual { in calc f { in v : Real; return : Real; } in a : Real; return : Real = ApplyQual::f(a) + Compiled::Fn::ApplyQual::f(1.0); }
		calc def SubApplyQual :> ApplyQual;
		calc def QualSq { in a : Real; return : Real = ApplyQual(Sq, a) + SubApplyQual(Half, a); }
		calc def QualByName { in calc f { in v : Real; return : Real; } in a : Real; return : Real = QualByName::f(v = a) + Pass(QualByName::f, a); }
		calc def QualByNameSq { in a : Real; return : Real = QualByName(Sq, a); }
		calc def QualRange { in calc f { in v : Real; return : Real; } in xs : Real[0..*]; return : Real[0..*] = Range(Sample(QualRange::f, xs)); }
		calc def QualRecipRange { in xs : Real[0..*]; return : Real[0..*] = QualRange(Recip, xs); }

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
		// Parentheses around `Sample(…)` or a function value are the expression itself.
		calc def ParenSample { in xs : Real[0..*]; attribute s : SampledFunction = (Sample(Sq, xs)); return : Real[0..*] = Range(s); }
		calc def ParenDomain { in xs : Real[0..*]; return : Real[0..*] = SequenceFunctions::union(Domain((Sample(Recip, xs))), Range((Sample((Sq), xs)))); }
		calc def ParenApply { in a : Real; return : Real = Apply((Sq), a); }
		// A literal null domain samples to `[]`, typed by the sampled calc's parameter.
		calc def NullDomain { in a : Real; attribute s : SampledFunction = Sample(Sq, null); return : Real[0..*] = SequenceFunctions::union(Domain(s), Range(s)); }
		calc def NullRange { in n : Integer; return : Integer[0..*] = Range(Sample(Inc, null)); }
		calc def NullSqrtRange { in a : Real; return : Real[0..*] = Range(Sample(RealFunctions::sqrt, null)); }
		calc def NullSum { in a : Real; return : Real = RealFunctions::sum(Range(Sample(Sq, null))) + a; }
		// A library function's declared parameter types its null domain: Integer for IntegerFunctions::abs.
		calc def NullAbsRange { in n : Integer; return : Integer[0..*] = Range(Sample(IntegerFunctions::abs, null)); }
		calc def NullAbsDomain { in n : Integer; attribute s : SampledFunction = Sample(IntegerFunctions::abs, null); return : Integer[0..*] = SequenceFunctions::union(Domain(s), Range(s)); }
		// A SampledFunction a body declares is taken at each read of it, as any body-local is.
		calc def BodySample { in xs : Integer[0..*]; return : Integer[0..*] = xs->ControlFunctions::collect { in x : Integer; attribute s : SampledFunction = Sample(Inc, 1..x); SequenceFunctions::size(Domain(s)) + SequenceFunctions::size(Range(s)) }; }
		calc def BodySampleRecip { in xs : Real[0..*]; return : Real[0..*] = xs->ControlFunctions::collect { in x : Real; attribute s : SampledFunction = Sample(Recip, (x, x - 1.0)); RealFunctions::sum(Range(s)) }; }
		calc def BodySampleUnread { in xs : Real[0..*]; return : Real[0..*] = xs->ControlFunctions::collect { in x : Real; attribute s : SampledFunction = Sample(Recip, x); x }; }
		// Sample, Domain and Range are library calcs: each is one frame against the
		// recursion budget, and each Domain or Range read collects its own sequence.
		calc def Down { in n : Integer; return : Integer = if n == 0 ? 0 else Down(n - 1); }
		calc def DownDirect { in n : Integer; return : Integer = Down(n); }
		calc def DownRange { in n : Integer; return : Integer[0..*] = Range(Sample(Down, n)); }
		calc def DownDomain { in n : Integer; attribute s : SampledFunction = Sample(Down, n); return : Integer[0..*] = Domain(s); }
		// The domain is Sample's argument, so it is computed before Sample's frame is entered.
		calc def Deep { in n : Integer; return : Integer[0..*] = if n == 0 ? (1) else Deep(n - 1); }
		calc def DeepRange { in n : Integer; return : Integer[0..*] = Range(Sample(Inc, Deep(n))); }
		calc def DeepDomain { in n : Integer; attribute s : SampledFunction = Sample(Inc, Deep(n)); return : Integer[0..*] = Domain(s); }
		calc def SizeDomain { in n : Integer; in k : Integer; attribute s : SampledFunction = Sample(Inc, 1..n); return : Integer = SequenceFunctions::size(1..k) + SequenceFunctions::size(Domain(s)); }
		calc def SizeRange { in n : Integer; in k : Integer; attribute s : SampledFunction = Sample(Inc, 1..n); return : Integer = SequenceFunctions::size(1..k) + SequenceFunctions::size(Range(s)); }
		// Each SamplePair taken is three elements: its domainValue, its rangeValue and
		// its place among the samples, charged on top of the domain it is taken over.
		calc def SamplePairs { in k : Integer; attribute s : SampledFunction = Sample(Inc, (1..k)->ControlFunctions::select { in x : Integer; x < 3 }); return : Integer = SequenceFunctions::size(Range(s)); }
	}
	// Integers beyond int64: exact as the interpreter's, refused by the C target
	// wherever a result may leave int64.
	package Wide {
		private import SequenceFunctions::*;
		private import ControlFunctions::*;
		private import NumericalFunctions::*;
		calc def Lit { in x : Integer; return : Integer = 123456789012345678901234567890 * x - 9223372036854775808; }
		calc def Pow { in x : Integer; return : Integer = x ** 70; }
		calc def Halves { in x : Integer; return : Boolean = (x ** 70) / (x ** 69) == x; }
		calc def Mod { in x : Integer; return : Integer = (x ** 70) % 7; }
		calc def Huge { in x : Integer; return : Integer = x ** 2000000; }
		calc def Sum { in xs : Integer[0..*] nonunique; return : Integer = sum(xs); }
		calc def Product { in xs : Integer[0..*] nonunique; return : Integer = product(xs); }
		calc def Idx { in xs : Integer[0..*]; in i : Integer; return : Integer = xs#(i); }
		calc def Rng { in lo : Integer; return : Integer[0..*] = lo..(lo + 2); }
		calc def Uniq { in xs : Integer[0..*]; return : Integer = size(xs); }
		calc def Has { in xs : Integer[0..*]; in x : Integer; return : Boolean = includes(xs, x); }
		calc def Gt { in x : Integer; in r : Real; return : Boolean = x > r; }
		calc def Lt { in r : Real; in x : Integer; return : Boolean = r < x; }
		calc def Widen { in x : Integer; return : Real = x * 1.0; }
		calc def Nat { in x : Natural; return : Natural = x * x; }
		calc def Least { in xs : Integer[0..*]; return : Integer = xs->minimize {in v : Integer; v}; }
		calc def Compare { in a : Integer; in b : Integer; return : Boolean = a < b and b >= a and a != b; }
	}
	// Loops whose Integer arithmetic the C target compiles: a Real counter, and
	// Integers only sized, indexed and ranged.
	package Loop {
		private import SequenceFunctions::*;
		private import ControlFunctions::*;
		calc def Churn { in n : Real; attribute i : Real = 0.0; attribute total : Real = 0.0; attribute keep : Real[0..*] = (); while i < n { attribute s : Integer[0..*] = 1..100; assign total := total + size(s); assign keep := including(keep, i); assign i := i + 1.0; } return : Real = total + size(keep); }
		calc def ChurnFor { in n : Integer; attribute keep : Real[0..*] nonunique = (); for i in 1..n { attribute s : Real[0..*] = (1..100)->collect {in v : Integer; v / 2}; if i % 2 == 0 { assign keep := including(keep, s#(size(s))); } } return : Real[0..*] nonunique = keep; }
		calc def ChurnNest { in n : Real; attribute keep : Integer[0..*] nonunique = (); attribute inner : Integer[0..*] = (); attribute i : Real = 0.0; while i < n { assign inner := (); for v in 1..50 { assign inner := including(inner, v); } assign keep := including(keep, size(inner)); for w in keep { assign i := i + 0.0; } assign i := i + 1.0; } return : Real = size(keep) * 1.0 + size(inner); }
	}
	// Integers written to Real-typed features stay Integers, as the
	// interpreter keeps them: arithmetic, comparison, identity, printing and
	// collections over a value whose kind is known only at run time.
	package Num {
		private import SequenceFunctions::*;
		private import ControlFunctions::*;
		private import NumericalFunctions::*;
		calc def Keep { in a : Real; return : Real = a; }
		calc def Local { in a : Integer; attribute r : Real = a; return : Real = r; }
		calc def Twice { in a : Integer; attribute r : Real = a; return : Real = r * 2 + 1; }
		calc def Plus { in a : Integer; in b : Real; attribute r : Real = a; return : Real = r + b; }
		calc def Half { in a : Integer; attribute r : Real = a; return : Real = r / 2; }
		calc def Recip { in a : Integer; attribute r : Real = a; return : Real = r ** -1; }
		calc def Square { in a : Integer; attribute r : Real = a; return : Real = r ** 2; }
		calc def Same { in a : Integer; attribute r : Real = a; return : Boolean = r === 3; }
		calc def SameReal { in a : Integer; attribute r : Real = a; return : Boolean = r === 3.0; }
		calc def Equal { in a : Integer; attribute r : Real = a; return : Boolean = r == 3.0 and r != 4; }
		calc def Less { in a : Integer; in b : Real; attribute r : Real = a; return : Boolean = r < b; }
		calc def Pick { in c : Boolean; in a : Integer; return : Real = if c ? a else 2.5; }
		calc def Assigned { in n : Integer; attribute r : Real = 0.5; for i in 1..n { assign r := i; } return : Real = r; }
		calc def Identity { in x : Real; in y : Integer; return : Boolean = x === y; }
		calc def MixLit { in a : Integer; return : Real[0..*] = (a, 2.5, 3); }
		calc def Collect { in x : Real[0..*]; return : Real[0..*] = x->collect {in v; v * 2}; }
		calc def Sum { in a : Integer; return : Real = sum((a, 2.5)); }
		calc def SumInts { in a : Integer; return : Real = sum((a, 2)); }
		calc def Least { in a : Integer; return : Real = (a, 2.5)->minimize {in v; v}; }
		calc def Unique { in a : Integer; attribute x : Real[0..*] = (a, 1.0); return : Real[0..*] = x; }
		calc def Has { in a : Integer; return : Boolean = includes((a, 2.5), 3.0); }
		calc def Coalesce { in x : Integer[0..*]; return : Real = x ?? 5.5; }
		calc def Equality { in x : Integer[0..*]; in y : Real[0..*]; return : Boolean = x == y; }
		calc def SameSeq { in x : Integer[0..*]; in y : Real[0..*]; return : Boolean = same(x, y); }
		calc def Union { in x : Integer[0..*]; in y : Real[0..*]; return : Real[0..*] = union(x, y); }
		calc def Floor { in a : Integer; attribute r : Real = a; return : Integer = RealFunctions::floor(r); }
		calc def Max { in a : Integer; attribute r : Real = a; return : Real = max(r, 2.5); }
	}
	package E {
		private import SequenceFunctions::*;
		private import ControlFunctions::*;
		enum def Color { enum red; enum green; enum blue; }
		enum def Shade { enum red; enum dark; }
		calc def Id { in c : Color; return : Color = c; }
		calc def Red { return : Color = Color::red; }
		calc def IsRed { in c : Color; return : Boolean = c == Color::red; }
		calc def NotRed { in c : Color; return : Boolean = c != Color::red; }
		calc def Same { in a : Color; in b : Color; return : Boolean = a === b; }
		calc def Cross { in c : Color; return : Boolean = c == Shade::red; }
		calc def VsInt { in c : Color; return : Boolean = c == 3; }
		calc def VsText { in c : Color; return : Boolean = c != "Color::red"; }
		calc def Pick { in b : Boolean; return : Color = if b ? Color::red else Color::blue; }
		calc def Add { in c : Color; return : Color = c + 1; }
		calc def Mul { in c : Color; in r : Real; return : Real = r * c; }
		calc def Less { in a : Color; in b : Color; return : Boolean = a < b; }
		calc def AtLeast { in a : Color; return : Boolean = 1 >= a; }
		calc def Neg { in c : Color; return : Color = -c; }
		calc def Text { in c : Color; return : String = BaseFunctions::ToString(c); }
		calc def Seq { in c : Color; return : Color[0..*] = (c, Color::green); }
		calc def Unique { in c : Color; attribute x : Color[0..*] = (c, Color::red); return : Color[0..*] = x; }
		calc def Has { in c : Color; return : Boolean = includes((Color::red, Color::blue), c); }
		calc def Count { in x : Color[0..*]; return : Integer = size(x); }
		calc def SeqEq { in x : Color[0..*]; return : Boolean = x == (Color::red, Color::green); }
		calc def Reds { in x : Color[0..*]; return : Color[0..*] = x->select {in v : Color; v == Color::red}; }
	}
	package Str {
		private import SequenceFunctions::*;
		private import ControlFunctions::*;
		private import StringFunctions::*;
		calc def Concat { in a : String; in b : String; return : String = a + b; }
		calc def Lit { return : String = "héllo, \"w\"\\\n" + "!"; }
		calc def Len { in a : String; return : Integer = Length(a); }
		calc def Sub { in a : String; in l : Integer; in u : Integer; return : String = Substring(a, l, u); }
		calc def Less { in a : String; in b : String; return : Boolean = a < b; }
		calc def AtMost { in a : String; in b : String; return : Boolean = a <= b; }
		calc def More { in a : String; in b : String; return : Boolean = a > b; }
		calc def AtLeast { in a : String; in b : String; return : Boolean = a >= b; }
		calc def Eq { in a : String; in b : String; return : Boolean = a == b; }
		calc def Neq { in a : String; in b : String; return : Boolean = a != b; }
		calc def Same { in a : String; in b : String; return : Boolean = a === b; }
		calc def EqNum { in a : String; return : Boolean = a == 3; }
		calc def IntStr { in n : Integer; return : String = IntegerFunctions::ToString(n); }
		calc def NatStr { in n : Integer; return : String = NaturalFunctions::ToString(n); }
		calc def RealStr { in r : Real; return : String = RealFunctions::ToString(r); }
		calc def BoolStr { in b : Boolean; return : String = BooleanFunctions::ToString(b); }
		calc def StrStr { in a : String; return : String = StringFunctions::ToString(a); }
		calc def BaseStr { in n : Integer; return : String = BaseFunctions::ToString(n); }
		calc def Pick { in b : Boolean; return : String = if b ? "yes" else "no"; }
		calc def Local { in a : String; attribute s : String = a + a; return : Integer = Length(s); }
		calc def Seq { in a : String; return : String[0..*] = (a, "x", a); }
		calc def Unique { in a : String; attribute x : String[0..*] = (a, "x"); return : String[0..*] = x; }
		calc def Lengths { in x : String[0..*]; return : Integer[0..*] = x->collect {in s : String; Length(s)}; }
		calc def Has { in a : String; return : Boolean = includes(("p", "q"), a); }
		calc def Joined { in x : String[0..*]; return : String = x->reduce {in a : String; in b : String; a + "," + b}; }
		calc def Build { in a : String; in n : Integer; attribute s : String = ""; while Length(s) < n { assign s := s + a; } return : String = s; }
		calc def Names { in n : Integer; attribute keep : String[0..*] = (); while size(keep) < n { assign keep := including(keep, "n" + IntegerFunctions::ToString(size(keep))); } return : String[0..*] = keep; }
		calc def ForS { in x : String[0..*]; attribute s : String = ""; for v in x { assign s := s + v + ";"; } return : String = s; }
		calc def Ctrl { in a : String; return : String = "\b\t\f\r" + a; }
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
	attribute pool : Integer[2];
	calc def SubsetLocal { in xs : Integer[0..*]; attribute ys :> pool = xs; return : Integer = size(ys); }
	calc def TensorParam { in t : TensorQuantityValue; return : Real = t.num#(1, 1, 1); }
	attribute cubeRef : TensorMeasurementReference { :>> dimensions = (2, 2, 2); :>> mRefs = (m, m, m, m, m, m, m, m); }
	calc def TensorBuilt { in x : Real; return : TensorQuantityValue = TensorCalculations::'['((x, x, x, x, x, x, x, x), cubeRef); }

	// A metaobject reflects a model element, which a native program has no representation of.
	attribute reflected : Integer = 1;
	calc def MetaCast { in x : Integer; return : Integer = SequenceFunctions::size(reflected meta KerML::Feature) + x; }

	attribute def Point { attribute x : Real; attribute y : Real; }
	enum def Color { red; green; }
	calc def Extent { return : Natural = size(all Color); }
	calc def RecordParam { in p : Point; return : Real = p.x + p.y; }
	calc def SelectNonBoolean { in x : Integer[0..*]; return : Integer[0..*] = x->select {in v; v}; }
	calc def CollectNull { in x : Integer[0..*]; return : Integer[0..*] = x->collect {in v; null}; }

	// Function values: a program takes no function on its command line, and a
	// function value is compiled only where it is applied or passed on.
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def Sq2 { in x : Real; return : Real = x * x; }
	calc def Add2 { in x : Real; in y : Real; return : Real = x + y; }
	calc def CalcParam { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc def Identity { in calc f { in v : Real; return : Real; } return r = f; }
	calc def Same { in calc f { in v : Real; return : Real; } in calc g { in v : Real; return : Real; } return : Boolean = f == g; }
	calc def FunctionAsValue { in a : Real; return : Real = Compiled::Fn::Apply(Sq, Sq); }
	calc def ValueAsFunction { in a : Real; return : Real = Compiled::Fn::Apply(a, a); }
	calc def IncR { in n : Integer; return : Integer = n + 1; }
	calc sqR : Sq;
	calc incR : IncR;
	calc def MixedArity { in a : Real; attribute g = if a > 0.0 ? sqR else incR; return r = Compiled::Fn::Apply(g, a); }
	calc def WrongArity { in a : Real; return : Real = Compiled::Fn::Apply(Add2, a); }
	calc def WrongName { in a : Real; return : Real = Compiled::Fn::ByName(Sq2, a); }
	// A parameter typed by a calc refuses a function value of a calc that does not specialize it.
	calc def UnrelatedTyped { in a : Real; return : Real = Compiled::Fn::ApplyTyped(Sq2, a); }
	calc def LibraryTyped { in a : Real; return : Real = Compiled::Fn::ApplyTypedOnly(RealFunctions::sqrt, a); }
	calc def ApplySub { in calc f : Compiled::Fn::SubSq; in a : Real; return : Real = f(a); }
	calc def GeneralForSub { in a : Real; return : Real = ApplySub(Compiled::Fn::Sq, a); }
	calc def ForwardedTyped { in calc g { in v : Real; return : Real; } in a : Real; return : Real = Compiled::Fn::ApplyTypedOnly(g, a); }
	calc def ForwardedUnrelated { in a : Real; return : Real = ForwardedTyped(Sq2, a); }
	// A null domain over a kind-preserving library function is Real, its widest kind.
	calc def IntegerNullRange { in n : Integer; return : Integer[0..*] = SampledFunctions::Range(SampledFunctions::Sample(NumericalFunctions::abs, null)); }
	calc def BodyClosure { in k : Real; in a : Real; calc scale { in v : Real; return : Real = v * k; } return : Real = Compiled::Fn::Apply(scale, a); }
	part def Scaler { attribute k : Real; calc scale { in v : Real; return : Real = v * k; } }
	part twice : Scaler { attribute :>> k = 2.0; }
	calc def OuterClosure { in a : Real; return : Real = Compiled::Fn::Apply(BodyClosure::scale, a); }
	calc def ObjectClosure { in a : Real; return : Real = Compiled::Fn::Apply(twice.scale, a); }
	calc def ObjectCalc { in a : Real; return : Real = Compiled::Fn::Apply(Scaler::scale, a); }
	calc def QualifiedReceiver { in calc f { in v : Real; return : Real; } in a : Real; return : Real = a->QualifiedReceiver::f(); }
	calc def ReceiverQualified { in a : Real; return : Real = QualifiedReceiver(Sq, a); }
	calc def ForeignQualified { in a : Real; return : Real = Compiled::Fn::ApplyQual::f(a); }
	calc def SampleArity { in xs : Real[0..*]; return : Real[0..*] = SampledFunctions::Range(SampledFunctions::Sample(Add2, xs)); }
	calc def SampledValue { in xs : Real[0..*]; attribute s : SampledFunctions::SampledFunction = SampledFunctions::Sample(Sq, xs); return : Integer = CollectionFunctions::size(s); }
	calc def SampledEscapes { in xs : Real[0..*]; return : SampledFunctions::SampledFunction = SampledFunctions::Sample(Sq, xs); }
	calc def UnevaluatedFunction { in xs : Integer[0..*]; return : Integer[0..*] = SampledFunctions::Range(SampledFunctions::Sample(ControlFunctions::collect, xs)); }

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

	// The interpreter judges the Real parameter's value when it is written to the
	// Integer attribute; the compiler would have to pick a representation, so it declines.
	calc def Narrowed {
		in r : Real;
		attribute x : Integer = r;
		return : Integer = x;
	}

	calc def DynamicIntPow {
		in a : Integer;
		in b : Integer;
		return : Integer = a ** b;
	}
}
