// A requirement usage nested in a definition binds its subject from the features
// of the object owning it — `subject s :>> s = sub;`, `subject s = sub;`, a bare
// `subject = sub;`, a subject bound to a sibling part, and a satisfy usage's
// `by` — and a read of the subject or of `result` through the owning object
// answers what the requirement check on that object computes. A subject nothing
// binds stays undetermined, and so does the result read through it.
package test {
	private import ScalarValues::*;

	requirement def R1 { subject s : Integer; require constraint { s > 0 } }
	requirement def RP { subject s : PA; require constraint { s.v > 0 } }
	part def PA { attribute v : Integer = 5; }

	part def P1 {
		attribute sub : Integer = 3;
		attribute neg : Integer = -2;
		part pa : PA;
		requirement rq : R1 { subject s :>> s = sub; }
		requirement rqNamed : R1 { subject s = sub; }
		requirement rqBare : R1 { subject = sub; }
		requirement rqPart : RP { subject s :>> s = pa; }
		requirement rqFail : R1 { subject s :>> s = neg; }
		requirement rqNarrow : R1 { subject s :>> s = sub; require constraint { s < 2 } }
		requirement rqDefault : R1 { subject s :>> s default sub; }
		requirement rqUnbound : R1 { subject s :>> s; }
		requirement rqPlain : R1;
		satisfy requirement sat : RP by pa;
		not satisfy requirement nsat : RP by pa;
	}
	part p1 : P1;

	case def Ca2 { subject s : PA; return rr : Integer = 7; }
	case ca2 : Ca2;
	use case def U1 { subject s : PA; }
	use case u1 : U1;
	part def Holder { use case uq : U1; case cq : Ca2; }
	part h : Holder;

	attribute top : Integer = 7;
	requirement rqPkg : R1 { subject s :>> s = top; }
	satisfy requirement satPkg : RP by p1.pa;
	not satisfy requirement nsatPkg : RP by p1.pa;

	attribute rqS = p1.rq.s;
	attribute rqResult = p1.rq.result;
	attribute rqNamedS = p1.rqNamed.s;
	attribute rqNamedResult = p1.rqNamed.result;
	attribute rqBareS = p1.rqBare.s;
	attribute rqBareResult = p1.rqBare.result;
	attribute rqPartV = p1.rqPart.s.v;
	attribute rqPartResult = p1.rqPart.result;
	attribute rqFailS = p1.rqFail.s;
	attribute rqFailResult = p1.rqFail.result;
	attribute rqNarrowResult = p1.rqNarrow.result;
	attribute rqDefaultS = p1.rqDefault.s;
	attribute rqDefaultResult = p1.rqDefault.result;
	attribute rqUnboundS = p1.rqUnbound.s;
	attribute rqUnboundResult = p1.rqUnbound.result;
	attribute rqPlainResult = p1.rqPlain.result;
	attribute satV = p1.sat.s.v;
	attribute satResult = p1.sat.result;
	attribute rqPkgS = rqPkg.s;
	attribute rqPkgResult = rqPkg.result;
	attribute satPkgV = satPkg.s.v;
	attribute satPkgResult = satPkg.result;
	attribute nsatV = p1.nsat.s.v;
	attribute nsatResult = p1.nsat.result;
	attribute nsatPkgResult = nsatPkg.result;
	attribute ca2ObjSubj = ca2.obj.subj;
	attribute ca2ObjResult = ca2.obj.result;
	attribute u1ObjSubj = u1.obj.subj;
	attribute u1ObjResult = u1.obj.result;
	attribute hUqObjResult = h.uq.obj.result;
	attribute hCqObjResult = h.cq.obj.result;

	calc probe { out one : Integer = 1; }
}
