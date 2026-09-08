package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// operatorDiags is the type-tier diagnostics under code of src, analysed as a
// document named name against the bundled library; the name's extension
// decides the language.
func operatorDiags(t *testing.T, name, code, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	var out []Diagnostic
	for _, d := range Analyze(name, root, nil, idx) {
		if d.Severity == SeverityError && d.Source != "type" {
			t.Fatalf("%s does not analyse cleanly: %v", name, d)
		}
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

// wantOperatorDiags asserts the diagnostics under code, in source order, carry
// exactly the given "line:col: fragment" verdicts; every one is a warning.
func wantOperatorDiags(t *testing.T, name, code, src string, want ...string) {
	t.Helper()
	diags := operatorDiags(t, name, code, src)
	var got []string
	for _, d := range diags {
		if d.Severity != SeverityWarning {
			t.Errorf("%v is not a warning", d)
		}
		pos := source.New(name, []byte(src)).Lines().PosAt(d.Span.Offset)
		got = append(got, fmt.Sprintf("%d:%d: %s", pos.Line, pos.Col, d.Message))
	}
	if len(got) != len(want) {
		t.Fatalf("want %d %s diagnostics, got %d:\n%s", len(want), code, len(got), strings.Join(got, "\n"))
	}
	for i, w := range want {
		at, fragment, _ := strings.Cut(w, " ")
		if !strings.HasPrefix(got[i], at+": ") || !strings.Contains(got[i], fragment) {
			t.Errorf("diagnostic %d = %q, want %q at %s", i, got[i], fragment, at)
		}
	}
}

// KerML `x[i]` names a function the kernel library leaves abstract: each
// bracket warns, wherever it sits in the tree; `#(...)` is the index.
func TestBracketOperatorInKerML(t *testing.T) {
	wantOperatorDiags(t, "a.kerml", codeBracketOperator, `package P {
	feature xs : ScalarValues::Integer[0..*] ordered;
	feature firstElem = xs[1];
	feature second = xs#(2);
	feature two = xs[1, 2];
	feature twice = xs[1][2];
	feature cast = (xs as ScalarValues::Real)[1];
	function F { in s : ScalarValues::Integer[0..*]; return : ScalarValues::Integer = s[1]; }
	feature units = 10 [SI::m];
}`,
		"3:22 `x[i]` is not an index in KerML",
		"5:16 `x[i]` is not an index in KerML",
		"6:18 `x[i]` is not an index in KerML",
		"6:18 `x[i]` is not an index in KerML",
		"7:17 `x[i]` is not an index in KerML",
		"8:84 `x[i]` is not an index in KerML",
		"9:18 `x[i]` is not an index in KerML",
	)
}

// SysML gives `[` the quantity functions, so a SysML document never gets the
// KerML warning, and a KerML document never gets the SysML one.
func TestBracketOperatorIsKerMLOnly(t *testing.T) {
	sysml := `package P {
	private import SI::*;
	attribute xs : ScalarValues::Integer[0..*] ordered;
	attribute firstElem = xs[1];
	attribute length = 10 [m];
}`
	wantOperatorDiags(t, "a.sysml", codeBracketOperator, sysml)
	wantOperatorDiags(t, "a.sysml", codeQuantityUnit, sysml,
		"4:27 the unit of a quantity must be a measurement reference, found Natural")
	wantOperatorDiags(t, "a.kerml", codeQuantityUnit, `package P {
	feature xs : ScalarValues::Integer[0..*] ordered;
	feature firstElem = xs[1];
}`)
}

const castFixture = `package P {
	private import ScalarValues::*;
	datatype A; datatype B :> A; datatype C; datatype D :> A, C;
	feature a : A;
	feature ab : A, C;
	feature s : String;
	classifier Box { feature base : A; feature str : String; }
	feature box : Box;
	classifier Box2 :> Box { feature :>> base; %s }
	function F { return r : A; }
	classifier Q; classifier R :> Q; classifier CQ ~ Q; feature cq : CQ;
	feature xs : A[*];
	feature d : D; feature b : B; datatype U unions B, C; datatype I intersects A, C; datatype Diff differences A, C; feature u : U; feature dd : Diff;
	feature untyped;
	feature valued = 3;
	%s
}`

// castDiags analyses a KerML cast fixture with member written in Box2 and body
// at package level.
func castDiags(t *testing.T, member, body string, want ...string) {
	t.Helper()
	src := strings.Replace(castFixture, "%s", member, 1)
	src = strings.Replace(src, "%s", body, 1)
	wantOperatorDiags(t, "a.kerml", codeCastConformance, src, want...)
}

// A cast whose argument's types and target specialize one another in neither
// direction selects nothing and warns, for every shape an argument takes.
func TestCastConformanceUnrelatedTypes(t *testing.T) {
	castDiags(t, "", `feature bad = a as C;`, "16:16 cast argument is typed by A, unrelated to the target C")
	castDiags(t, "", `feature bad = s as Integer;`, "16:16 cast argument is typed by String, unrelated to the target Integer")
	castDiags(t, "", `feature bad = 3 as String;`, "16:16 cast argument is typed by Integer, unrelated to the target String")
	castDiags(t, "", `feature bad = valued as String;`, "16:16 cast argument is typed by Integer, unrelated to the target String")
	castDiags(t, "", `feature bad = ab as String;`, "16:16 cast argument is typed by A and C, unrelated to the target String")
	castDiags(t, "", `feature bad = box.base as String;`, "16:16 cast argument is typed by A, unrelated to the target String")
	castDiags(t, "", `feature bad = F() as String;`, "16:16 cast argument is typed by A, unrelated to the target String")
	castDiags(t, "", `feature bad = (a as B) as C;`, "16:16 cast argument is typed by B, unrelated to the target C")
	castDiags(t, "", `feature bad = (1 < 2) as Integer;`, "16:16 cast argument is typed by Boolean, unrelated to the target Integer")
	castDiags(t, "", `feature bad = xs.?{in x; true} as C;`, "16:16 cast argument is typed by A, unrelated to the target C")
	castDiags(t, "", `feature bad = a#(1) as C;`, "16:16 cast argument is typed by A, unrelated to the target C")
	castDiags(t, "", `feature bad = u as String;`, "16:16 cast argument is typed by U, unrelated to the target String")
	castDiags(t, "", `feature bad = s as I;`, "16:16 cast argument is typed by String, unrelated to the target I")
	// Every value of D is one of C's, which the difference subtracts.
	castDiags(t, "", `feature bad = d as Diff;`, "16:16 cast argument is typed by D, unrelated to the target Diff")
	castDiags(t, "", `feature bad = dd as C;`, "16:16 cast argument is typed by Diff, unrelated to the target C")
	// A value of C as well as of A is none of the values A minus C holds.
	castDiags(t, "", `feature bad = ab as Diff;`, "16:16 cast argument is typed by A and C, unrelated to the target Diff")
	castDiags(t, "", `feature bad = a as s;`, "16:16 cast argument is typed by A, unrelated to the target s")
	castDiags(t, "", `feature bad = cq as R;`, "16:16 cast argument is typed by CQ, unrelated to the target R")
	castDiags(t, `feature bad = base as String;`, "", "9:59 cast argument is typed by A, unrelated to the target String")
	castDiags(t, "", `feature two = (a as C) as String;`,
		"16:16 cast argument is typed by C, unrelated to the target String",
		"16:17 cast argument is typed by A, unrelated to the target C")
}

// A cast up, down, or sideways through one of several types conforms; so does
// one whose argument's type is not statically known, or is Anything, and one
// between a composed type and a type it is composed of.
func TestCastConformanceRelatedTypes(t *testing.T) {
	castDiags(t, "", `feature up = a as Base::Anything; feature down = a as B; feature same = a as A; feature self = a as a;`)
	castDiags(t, "", `feature one = ab as B; feature other = ab as C; feature viaD = d as C;`)
	castDiags(t, "", `feature chain = box.base as B; feature call = F() as B; feature nested = (a as B) as A;`)
	castDiags(t, `feature redef = base as B;`, "")
	castDiags(t, "", `feature conjugate = cq as Q;`)
	castDiags(t, "", `feature wide = untyped as String; feature nothing = null as A; feature real = 3 as Real;`)
	castDiags(t, "", `feature data = (1 + 2) as String; feature seq = (1, 2) as Integer; feature body = xs.{in x; x} as C;`)
	castDiags(t, "", `feature cond = (if true ? a else a) as C; feature sel = xs.?{in x; true} as B;`)
	castDiags(t, "", `feature union = a as U; feature member = u as B; feature wider = u as A;`)
	castDiags(t, "", `feature meet = d as I; feature less = b as Diff; feature kept = dd as A;`)
}

// The rule is KerML's, but SysML declares the same operator: a usage cast to an
// unrelated definition warns there too.
func TestCastConformanceInSysML(t *testing.T) {
	wantOperatorDiags(t, "a.sysml", codeCastConformance, `package P {
	private import ScalarValues::*;
	private import ISQ::*;
	private import Quantities::*;
	part def PD { attribute a : A; }
	attribute def A; attribute def B :> A;
	part p : PD;
	attribute q : MassValue;
	attribute chainOk = p.a as B;
	attribute chainBad = p.a as String;
	attribute castOk = q as ScalarQuantityValue;
	attribute castBad = q as LengthValue;
	attribute unitOk = 10 [q as MeasurementReferences::MeasurementUnit];
	attribute unitBad = 10 [(q as Integer) as MeasurementReferences::MeasurementUnit];
}`,
		"10:23 cast argument is typed by A, unrelated to the target String",
		"12:22 cast argument is typed by MassValue, unrelated to the target LengthValue",
		"13:25 cast argument is typed by MassValue, unrelated to the target MeasurementUnit",
		"14:26 cast argument is typed by Integer, unrelated to the target MeasurementUnit",
		"14:27 cast argument is typed by MassValue, unrelated to the target Integer",
	)
}

const quantityImports = "private import ScalarValues::*;\nprivate import ISQ::*;\nprivate import SI::*;\nprivate import MeasurementReferences::*;\nprivate import Quantities::*;\n"

func quantityDiags(t *testing.T, body string, want ...string) {
	t.Helper()
	wantOperatorDiags(t, "a.sysml", codeQuantityUnit, "package P {\n"+quantityImports+body+"\n}", want...)
}

// The unit of a quantity is a measurement reference: a unit, an aliased or
// custom one, a feature typed by one, a frame, and arithmetic or selection
// over them. An operator is judged by the operands it is passed by value, as
// the pilot judges it: `(m, 3)` and `m ?? 3` pass, a conditional does not.
func TestQuantityUnitMeasurementReferences(t *testing.T) {
	quantityDiags(t, `attribute a = 10 [m];
	attribute pair = 10 [m, s];
	alias metre for m;
	attribute aliased = 10 [metre];
	attribute typed : LengthUnit = m;
	attribute viaFeature = 10 [typed];
	attribute frame : CoordinateFrame;
	attribute vector = (1, 2, 3) [frame];
	attribute product = 10 [m * s];
	attribute quotient = 10 [m / s];
	attribute power = 10 [m ** 2];
	attribute scaled = 10 [m * 2];
	attribute prescaled = 10 [2 * m];
	attribute negated = 10 [-m];
	attribute derived = 10 [km / h];
	attribute fallback = 10 [m ?? 3];
	attribute mixed = 10 [(m, 3)];
	attribute cast = 10 [(m * s) as LengthUnit];
	attribute called = 10 [MeasurementRefCalculations::'*'(m, s)];
	attribute def Custom :> MeasurementUnit;
	attribute custom : Custom;
	attribute viaCustom = 10 [custom];
	calc def Convert { in mag : Real; in unit : MeasurementUnit; return : ScalarQuantityValue = mag [unit]; }
	attribute sequence = (1, 2) [m];
	attribute units : MeasurementUnit[0..*] ordered;
	attribute selected = 10 [units#(1)];
	attribute frame3d : CartesianSpatial3dCoordinateFrame;
	attribute component = 10 [frame3d.mRefs#(1)];
	attribute picked = 10 [(m, s)#(2)];`)
}

// Anything else in the unit position warns at the unit: a number, a string, a
// quantity, a dimensionless computation, an untyped feature, a conditional (its
// branches are expression bodies, so its result is Anything); a unit nested in
// the unit is judged on its own.
func TestQuantityUnitNotAMeasurementReference(t *testing.T) {
	quantityDiags(t, `attribute n : Integer;
	attribute q : MassValue;
	attribute product = m * s;
	attribute a = 10 [3];
	attribute b = 10 ["m"];
	attribute c = 10 [n];
	attribute d = 10 [q];
	attribute e = 10 [2 * 3];
	attribute f = 10 [-3];
	attribute g = 10 [2 ** 2];
	attribute h = 10 [if true ? 1 else 2];
	attribute i = 10 [product as Integer];
	attribute j = 10 [ScalarFunctions::'+'(1, 2)];
	calc def Convert { in mag : Real; in unit; return : ScalarQuantityValue = mag [unit]; }
	attribute xs : Integer[0..*] ordered;
	attribute k = xs[1];
	attribute l = 10 [1] [2];
	attribute o = 10 [xs#(1)];
	attribute p = 10 [if true ? m else s];
	attribute r = 10 [3 ?? m];
	attribute t = 10 [2 [m]];
	attribute u = 10 [m * 3 ["s"]];
	attribute v = 10 [2 [3]];`,
		"10:20 found Natural",
		"11:20 found String",
		"12:20 found Integer",
		"13:20 found MassValue",
		"14:20 found the result of `*`, typed by DataValue",
		"15:20 found the result of `-`, typed by DataValue",
		"16:20 found the result of `**`, typed by DataValue",
		"17:20 found the result of `if`, typed by Anything",
		"18:20 found the result of `as`, typed by Integer",
		"19:20 found ScalarValue",
		"20:81 found an untyped feature",
		"22:19 found Natural",
		"23:20 found Natural",
		"23:24 found Natural",
		"24:20 found the result of `#`, typed by Integer",
		"25:20 found the result of `if`, typed by Anything",
		"26:20 found the result of `??`, typed by Anything",
		"27:20 found a quantity in metre",
		"28:27 found String",
		"29:20 found ScalarQuantityValue",
		"29:23 found Natural",
	)
}

// The unit is judged wherever an expression is typed: guards, trigger times,
// assignments, payloads, and the bodies of constraints, calcs and requirements.
func TestQuantityUnitInEveryExpressionPosition(t *testing.T) {
	quantityDiags(t, `constraint def Chk { in v : Real; 3 [m] > v [1] }
	calc def Calc { in v : Real; (v + 1) [2] }
	action def Act {
		attribute z : Real;
		action a1 { accept after 5 [1]; }
		then action a2 { assign z := 5 [3]; }
		then if 3 [1] > 2 [m] { action a3 { send 4 [1] to a2; } }
	}
	state def SM {
		entry; then s1;
		state s1;
		transition first s1 if 3 [1] > 2 [m] then s2;
		state s2;
	}
	requirement def Req { require constraint { 3 [1] > 2 [m] } }`,
		"7:46 found Natural",
		"8:40 found Natural",
		"11:31 found Natural",
		"12:35 found Natural",
		"13:14 found Natural",
		"13:47 found Natural",
		"18:29 found Natural",
		"21:48 found Natural",
	)
}

// A filter condition and a multiplicity bound are judged by rules of their own,
// yet the operators they write get the operator rules too, once each. The pinned
// pilot warns at the same positions in the filter conditions.
func TestOperatorRulesInFilterConditions(t *testing.T) {
	kerml := `package P {
	private import ScalarValues::*;
	datatype C;
	package Q1 { filter (1 as C) == 1; }
	package Q2 { filter (1 as Integer) == 1; }
	package Q3 { filter (1, 2)[1] == 1; }
	package Q4 { private import Q1::*[(1 as C) == 1]; }
}`
	wantOperatorDiags(t, "a.kerml", codeCastConformance, kerml,
		"4:23 cast argument is typed by Integer, unrelated to the target C",
		"7:37 cast argument is typed by Integer, unrelated to the target C")
	wantOperatorDiags(t, "a.kerml", codeBracketOperator, kerml, "6:22 `x[i]` is not an index in KerML")
	wantOperatorDiags(t, "a.sysml", codeCastConformance, `package P {
	private import ScalarValues::*;
	attribute def C;
	package Q1 { filter (1 as C) == 1; }
	package Q2 { filter (1 as Integer) == 1; }
}`, "4:23 cast argument is typed by Integer, unrelated to the target C")
}

func TestOperatorRulesInMultiplicityBounds(t *testing.T) {
	kerml := `package P {
	private import ScalarValues::*;
	datatype C; classifier K;
	feature n : Integer;
	feature a : K[((1 as C) as Integer)..(2 as Integer)];
	feature b : K[n..((n as C) as Natural)];
}`
	wantOperatorDiags(t, "a.kerml", codeCastConformance, kerml,
		"5:17 cast argument is typed by C, unrelated to the target Integer",
		"5:18 cast argument is typed by Integer, unrelated to the target C",
		"6:20 cast argument is typed by C, unrelated to the target Natural",
		"6:21 cast argument is typed by Integer, unrelated to the target C")
	wantOperatorDiags(t, "a.sysml", codeCastConformance, `package P {
	private import ScalarValues::*;
	attribute def C; part def K;
	part a : K[((1 as C) as Integer)..(2 as Integer)];
}`,
		"4:14 cast argument is typed by C, unrelated to the target Integer",
		"4:15 cast argument is typed by Integer, unrelated to the target C")
}

// A calculation or constraint usage named as a value is typed by its definition,
// not by its result, so casting it to the result's type warns, as the pilot
// warns; its invocation and its result parameter are typed by the result.
func TestCastConformanceOfCalculationUsages(t *testing.T) {
	wantOperatorDiags(t, "a.sysml", codeCastConformance, `package P {
	private import ScalarValues::*;
	calc def Name { return : String; }
	calc def Count { return : Integer; }
	calc def NameSub :> Name;
	constraint def K;
	part def PD {
		calc n : Name; calc c : Count; calc ns : NameSub; constraint k : K;
		attribute a1 = n as String;
		attribute a2 = c as String;
		attribute a3 = ns as String;
		attribute a4 = n() as String;
		attribute a5 = c() as String;
		attribute a6 = Name() as Integer;
		attribute a7 = k as Boolean;
		attribute a8 = k() as Boolean;
		attribute b1 = n.result as String;
		attribute b2 = ns.result as String;
		attribute b3 = ns() as String;
		attribute b4 = NameSub() as String;
	}
	calc def Wrap1 { return : String; calc n : Name; n as String }
	calc def Wrap2 { return : String; calc n : Name; n.result as String }
	calc def Wrap3 { return : String; result as String }
}`,
		"9:18 cast argument is typed by Name, unrelated to the target String",
		"10:18 cast argument is typed by Count, unrelated to the target String",
		"11:18 cast argument is typed by NameSub, unrelated to the target String",
		"13:18 cast argument is typed by Integer, unrelated to the target String",
		"14:18 cast argument is typed by String, unrelated to the target Integer",
		"15:18 cast argument is typed by K, unrelated to the target Boolean",
		"22:51 cast argument is typed by Name, unrelated to the target String",
	)
}

// A bound's operators are judged by the type tier, so an unrelated type error
// elsewhere in the document does not silence them.
func TestOperatorRulesInMultiplicityBoundsSurviveUnrelatedErrors(t *testing.T) {
	kerml := `package P {
	private import ScalarValues::*;
	datatype C; classifier K;
	feature bad : Integer = "s";
	feature a : K[0..(2 as C)];
}`
	root := parser.New(source.New("a.kerml", []byte(kerml))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("a.kerml", root)
	idx.ExpandWildcardImports()
	var errs, casts int
	for _, d := range Analyze("a.kerml", root, nil, idx) {
		switch {
		case d.Code == codeCastConformance:
			casts++
		case d.Severity == SeverityError:
			errs++
		}
	}
	if errs != 1 || casts != 1 {
		t.Fatalf("want the binding error and the bound's cast warning, got %d errors and %d cast warnings", errs, casts)
	}
}

// The members an expression body declares are reached wherever the body is
// written: in a filter condition, a bound or an ordinary value, once each.
func TestOperatorRulesInBodyMembers(t *testing.T) {
	kerml := `package P {
	private import ScalarValues::*;
	datatype C; classifier K;
	package Q1 { filter { feature m = 1 as C; m == 1 }; }
	feature a : K[0..{ feature m = 2 as C; m }];
	feature v = { feature m = 3 as C; feature n { feature o = 4 as C; } m };
	package Q2 { filter { in x; feature m = { 5 as C }; m == x }; }
}`
	wantOperatorDiags(t, "a.kerml", codeCastConformance, kerml,
		"4:36 cast argument is typed by Integer, unrelated to the target C",
		"5:33 cast argument is typed by Integer, unrelated to the target C",
		"6:28 cast argument is typed by Integer, unrelated to the target C",
		"6:60 cast argument is typed by Integer, unrelated to the target C",
		"7:44 cast argument is typed by Integer, unrelated to the target C")
	wantOperatorDiags(t, "a.sysml", codeCastConformance, `package P {
	private import ScalarValues::*;
	attribute def C; part def K;
	package Q1 { filter { attribute m = 1 as C; m == 1 }; }
	part a : K[0..{ attribute m = 2 as C; m }];
}`,
		"4:38 cast argument is typed by Integer, unrelated to the target C",
		"5:32 cast argument is typed by Integer, unrelated to the target C")
}

// Every multiplicity the notation writes is reached once by the operator rules:
// a multiplicity declaration's range, a body parameter's bound (in a value, a
// select, a collect or a filter body), a cast's, a definition's, a connector
// end's, a subject's, an assume/require's and a cross feature's.
func TestOperatorRulesInEveryMultiplicityForm(t *testing.T) {
	kerml := `package P {
	private import ScalarValues::*;
	datatype C; classifier K;
	multiplicity m [0..(2 as C)];
	feature f = { in x : Integer[0..(3 as C)]; x };
	feature g = (as K[0..(4 as C)]);
	class D [0..(5 as C)];
	connector c from [0..(6 as C)] f to [0..(7 as C)] g;
	feature n : Natural;
	feature i : K { multiplicity mm [0..(8 as C)]; }
	feature j = f.?{ in y : Integer[0..(10 as C)]; y == 1 };
	feature k = f.{ in w : Integer[0..(11 as C)]; w };
	multiplicity m2 [0..n[1]];
	feature f2 = { in x : Integer[0..n[2]]; x };
	feature g2 = (as K[0..n[3]]);
}`
	wantOperatorDiags(t, "a.kerml", codeCastConformance, kerml,
		"4:22 cast argument is typed by Integer, unrelated to the target C",
		"5:35 cast argument is typed by Integer, unrelated to the target C",
		"6:24 cast argument is typed by Integer, unrelated to the target C",
		"7:15 cast argument is typed by Integer, unrelated to the target C",
		"8:24 cast argument is typed by Integer, unrelated to the target C",
		"8:43 cast argument is typed by Integer, unrelated to the target C",
		"10:39 cast argument is typed by Integer, unrelated to the target C",
		"11:38 cast argument is typed by Integer, unrelated to the target C",
		"12:37 cast argument is typed by Integer, unrelated to the target C")
	wantOperatorDiags(t, "a.kerml", codeBracketOperator, kerml,
		"13:22 `x[i]` is not an index in KerML",
		"14:35 `x[i]` is not an index in KerML",
		"15:24 `x[i]` is not an index in KerML")
	wantOperatorDiags(t, "a.sysml", codeCastConformance, `package P {
	private import ScalarValues::*;
	attribute def C; part def K;
	part a : K; part b : K;
	connection cn connect [0..(2 as C)] a to [0..(3 as C)] b;
	bind [0..(4 as C)] a = b;
	requirement def D { subject s : K[0..(5 as C)]; }
	requirement def R { assume constraint ac [0..(6 as C)] { true } require constraint rc [0..(7 as C)] { true } }
	part e : K[0..(8 as C)] { attribute q = { in v : Integer[0..(9 as C)]; v }; }
	connection def CD { end e1 [0..(10 as C)] feature x : K; end e2 : K; }
	package Q { filter { in z : Integer[0..(11 as C)]; z == 1 }; }
}`,
		"5:29 cast argument is typed by Integer, unrelated to the target C",
		"5:48 cast argument is typed by Integer, unrelated to the target C",
		"6:12 cast argument is typed by Integer, unrelated to the target C",
		"7:40 cast argument is typed by Integer, unrelated to the target C",
		"8:48 cast argument is typed by Integer, unrelated to the target C",
		"8:93 cast argument is typed by Integer, unrelated to the target C",
		"9:17 cast argument is typed by Integer, unrelated to the target C",
		"9:63 cast argument is typed by Integer, unrelated to the target C",
		"10:34 cast argument is typed by Integer, unrelated to the target C",
		"11:42 cast argument is typed by Integer, unrelated to the target C")
}

// Multiplicity and relationship bodies are reached wherever they sit — top level, feature,
// value or filter body, under a definition or package — once each, in their own scope.
func TestOperatorRulesInMultiplicityAndRelationshipBodies(t *testing.T) {
	kerml := `package P {
	private import ScalarValues::*;
	datatype C; classifier K; classifier K2 :> K;
	multiplicity m [0..3] { feature f : K[0..(1 as C)]; feature g : Integer = 2 as C; }
	specialization s subtype K2 :> K { feature h : K[0..(3 as C)]; feature i : Integer = 4 as C; }
	feature outer : K { specialization subtype K2 :> K { feature j : K[0..(5 as C)]; } }
	feature v = { feature x { multiplicity mm [0..(6 as C)] { feature q : K[0..(7 as C)]; } specialization subtype K2 :> K { feature r : K[0..(8 as C)]; } } 1 };
	package Q { filter { feature y { multiplicity mn [0..3] { feature t : K[0..(9 as C)]; } specialization subtype K2 :> K { feature w : K[0..(10 as C)]; } } 1 == 1 }; }
	multiplicity ok [0..3] { feature f : K[0..(1 as Integer)]; feature g : Integer = 2 as Integer; }
	package R1 { filter { classifier D [0..(1 as C)] { feature f : K[0..(2 as C)]; multiplicity m [0..3] { feature g : Integer = 3 as C; } } 1 == 1 }; }
	package R2 { filter { struct S { package N { feature f : K[0..(4 as C)]; } specialization subtype K2 :> K { struct T { feature h : K[0..(5 as C)]; } } } 1 == 1 }; }
}`
	wantOperatorDiags(t, "a.kerml", codeCastConformance, kerml,
		"4:44 cast argument is typed by Integer, unrelated to the target C",
		"4:76 cast argument is typed by Integer, unrelated to the target C",
		"5:55 cast argument is typed by Integer, unrelated to the target C",
		"5:87 cast argument is typed by Integer, unrelated to the target C",
		"6:73 cast argument is typed by Integer, unrelated to the target C",
		"7:49 cast argument is typed by Integer, unrelated to the target C",
		"7:78 cast argument is typed by Integer, unrelated to the target C",
		"7:141 cast argument is typed by Integer, unrelated to the target C",
		"8:78 cast argument is typed by Integer, unrelated to the target C",
		"8:141 cast argument is typed by Integer, unrelated to the target C",
		"10:42 cast argument is typed by Integer, unrelated to the target C",
		"10:71 cast argument is typed by Integer, unrelated to the target C",
		"10:127 cast argument is typed by Integer, unrelated to the target C",
		"11:65 cast argument is typed by Integer, unrelated to the target C",
		"11:139 cast argument is typed by Integer, unrelated to the target C")
	wantOperatorDiags(t, "a.sysml", codeCastConformance, `package P {
	private import ScalarValues::*;
	attribute def C; part def K;
	multiplicity m [0..3] { attribute f : K[0..(1 as C)]; attribute g : Integer = 2 as C; }
	part v : K { multiplicity mm [0..3] { attribute q : K[0..(3 as C)]; } }
	package R { filter { part x { part def D { part f : K[0..(4 as C)]; multiplicity m [0..3] { part g : K[0..(5 as C)]; } } } 1 == 1 }; }
}`,
		"4:46 cast argument is typed by Integer, unrelated to the target C",
		"4:80 cast argument is typed by Integer, unrelated to the target C",
		"5:60 cast argument is typed by Integer, unrelated to the target C",
		"6:60 cast argument is typed by Integer, unrelated to the target C",
		"6:109 cast argument is typed by Integer, unrelated to the target C")
}
