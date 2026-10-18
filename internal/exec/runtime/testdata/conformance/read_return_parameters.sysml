// A result parameter an object inherits from the library behavior it enacts
// answers the object's own evaluation: RequirementCheck::result is the
// requirement's required conditions conjoined (Requirements.sysml), Case::result
// the case's own return parameter, which redefines it (Cases.sysml), and
// VerificationCase::verdict the verdict the case's body produced
// (VerificationCases.sysml). An evaluation reading an unbound input — a subject
// nothing supplies — is undetermined, never a made-up value.
package test {
	private import ScalarValues::*;
	private import VerificationCases::*;

	part def P2 { attribute m : Real = 3.0; }
	part p2 : P2;

	requirement def R1 { subject s : P2; attribute lim : Real = 5.0; require constraint { s.m < lim } assume constraint { lim > 0.0 } }
	requirement def R2 { subject s : P2; }
	requirement def R3 { subject s : P2; require constraint { false } }
	requirement def R5 { subject s : P2; assume constraint { false } require constraint { false } }
	requirement def R6 { subject s : P2; assume constraint { true } require constraint { false } }
	requirement def R7 { subject s : P2; assume constraint { s.m > 0.0 } require constraint { true } }
	requirement def R8 { subject s : P2; assume constraint { attribute b : Boolean = true; assign b := false; b } require constraint { false } }
	requirement def R9 { subject s : P2; assume constraint { false } require constraint { 1 / 0 > 0 } }
	requirement def R10 { subject s : P2; assume constraint { s.m > 0.0 } assume constraint { false } require constraint { false } }
	requirement def R11 { subject s : P2; require constraint { s.m > 0.0 } require constraint { false } }
	constraint def C1 { true }
	part def P3 {
		attribute m : Real;
		attribute n : Real = 1.0;
		constraint c { n > 0.0 }
		assert constraint c2 { n > 0.0 }
		constraint c3 { n < 0.0 }
		constraint cu { m > 0.0 }
		constraint cn { assert constraint { n > 0.0 } }
		constraint cnf { assert constraint { n < 0.0 } }
		constraint ca { assume constraint { false } }
		assert not constraint cneg { assume constraint { false } }
	}
	part p3 : P3;
	requirement def R13 { require constraint c { assert constraint { false } } }
	requirement r13 : R13;
	requirement def R12 { subject s : P2; assume constraint { false } require constraint : C1 { false } }
	case def Ca1 { subject s : P2; return r : Integer = 1 + 1; }
	case def Ca2 { subject s : P2; objective obj { require constraint { true } } }
	verification def V1 { subject s : P2; objective { verify r1; } requirement r1 : R1; return verdict : VerdictKind = PassIf(true); }
	verification def V2 { subject s : P2; objective { verify r3; } requirement r3 : R3; }
	verification def V3 { subject s : P2; return verdict : VerdictKind = PassIf(false); }
	verification def V4 { subject s : P2; return verdict : VerdictKind = VerdictKind::error; }
	viewpoint def Vp1 { frame concern con1 : Con1; require constraint { true } }
	concern def Con1 { stakeholder sh : P2; }

	requirement r1 : R1 { subject s :>> s = p2; }
	requirement r1u : R1;
	requirement r3 : R3 { subject s :>> s = p2; }
	case ca1 : Ca1;
	case ca2 : Ca2;
	verification v1 : V1 { subject s :>> s = p2; }
	verification v2 : V2 { subject s :>> s = p2; }
	verification v3 : V3 { subject s :>> s = p2; }
	viewpoint vp1 : Vp1;
	concern con1 : Con1;
	part def Checks { satisfy r1 by p2; satisfy r3 by p2; }

	calc probe { out one : Integer = 1; }

	attribute r1New : Boolean = new R1().result;
	attribute r2New : Boolean = new R2().result;
	attribute r3New : Boolean = new R3().result;
	attribute r5New : Boolean = new R5().result;
	attribute r6New : Boolean = new R6().result;
	attribute r7New : Boolean = new R7().result;
	attribute r8New : Boolean = new R8().result;
	attribute r9New : Boolean = new R9().result;
	attribute r10New : Boolean = new R10().result;
	attribute r11New : Boolean = new R11().result;
	attribute r12New : Boolean = new R12().result;
	attribute c3Usage : Boolean = p3.c;
	attribute c2Usage : Boolean = p3.c2;
	attribute cResultUsage : Boolean = p3.c.result;
	attribute c3False : Boolean = p3.c3;
	attribute cuUnbound : Boolean = p3.cu;
	attribute c2New : Boolean = new P3().c2;
	attribute cNew : Boolean = new P3().c;
	attribute cnNested : Boolean = p3.cn;
	attribute cnfNested : Boolean = p3.cnf;
	attribute r13Nested : Boolean = r13.c;
	attribute caAssumeOnly : Boolean = p3.ca;
	attribute cnegNoCondition : Boolean = p3.cneg;
	attribute v4New = new V4().verdict;
	attribute r1Bound : Boolean = r1.result;
	attribute r1Unbound : Boolean = r1u.result;
	attribute r3Bound : Boolean = r3.result;
	attribute vp1New : Boolean = new Vp1().result;
	attribute vp1Usage : Boolean = vp1.result;
	attribute con1New : Boolean = new Con1().result;
	attribute con1Usage : Boolean = con1.result;
	attribute ca1New = new Ca1().result;
	attribute ca1Usage = ca1.result;
	attribute ca2New = new Ca2().result;
	attribute ca2Usage = ca2.result;
	attribute v1New = new V1().verdict;
	attribute v1Result = new V1().result;
	attribute v1Usage = v1.verdict;
	attribute v2New = new V2().verdict;
	attribute v2Usage = v2.verdict;
	attribute v2UsageResult = v2.result;
	attribute v3New = new V3().verdict;
	attribute v3Usage = v3.verdict;
	attribute v3UsageResult = v3.result;
}
