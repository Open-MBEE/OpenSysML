package passes

import (
	"strings"
	"testing"
)

// The reference form of an assert constraint usage must reference a constraint
// usage (SysML v2 §7.19, validateAssertConstraintUsageReference).
func TestAssertReferenceToConstraintIsSilent(t *testing.T) {
	src := `constraint def CD; requirement def RD;
part def Base { constraint inherited : CD; }
part def Derived :> Base { constraint c : CD; requirement r : RD; }
part v : Derived;
part ctx {
	constraint local : CD;
	assert local;
	assert not local;
	assert v.c;
	assert not v.c;
	assert v.inherited;
	assert v.r;
	assert v.c { true }
	assert constraint declared : CD;
	assert constraint { true }
}`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}
}

func TestAssertReferenceToNonConstraintRejected(t *testing.T) {
	tests := []struct {
		name, target, found string
	}{
		{"part usage", "assert p;", "partUsage"},
		{"negated part usage", "assert not p;", "partUsage"},
		{"attribute usage", "assert a;", "attributeUsage"},
		{"with body", "assert p { true }", "partUsage"},
		{"chained part usage", "assert v.p;", "partUsage"},
		{"negated chained attribute", "assert not v.a;", "attributeUsage"},
		{"chained inherited part", "assert v.bp;", "partUsage"},
		{"constraint definition", "assert CD;", "constraintDef"},
		{"part definition", "assert PD;", "partDef"},
		{"package", "assert Q;", "package"},
	}
	prefix := `constraint def CD; part def PD; package Q;
part def Base { part bp; }
part def Derived :> Base { part p; attribute a; }
part v : Derived; part p; attribute a;
part ctx { `
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := typeDiags(t, prefix+tc.target+" }")
			if len(diags) != 1 {
				t.Fatalf("expected one type diagnostic, got %v", diags)
			}
			want := "assert target must be a constraint usage, found " + tc.found
			if diags[0].Message != want {
				t.Errorf("got %q, want %q", diags[0].Message, want)
			}
		})
	}
}

// An alias names the element it is for (KerML 8.2.3.2), so the referent-kind
// rule judges the aliased element, directly and through a chain.
func TestAssertReferenceThroughAliasJudgesAliasedElement(t *testing.T) {
	prefix := `constraint def CD; part def PD;
part def Holder { constraint c : CD; part p; alias ac for c; alias ap for p; }
part h : Holder; constraint c : CD; part p;
alias ac for c; alias acc for ac; alias ap for p; alias apd for PD; alias ah for h;
package Q { alias qc for c; }
part ctx { `
	silent := "assert ac; assert not ac; assert acc; assert Q::qc; assert ah.c; assert ah.ac; assert h.ac; }"
	if diags := typeDiags(t, prefix+silent); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}
	tests := []struct {
		name, target, found string
	}{
		{"alias to part usage", "assert ap;", "partUsage"},
		{"alias to part definition", "assert apd;", "partDef"},
		{"chain through alias", "assert ah.p;", "partUsage"},
		{"chain ending in alias", "assert h.ap;", "partUsage"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := typeDiags(t, prefix+tc.target+" }")
			if len(diags) != 1 {
				t.Fatalf("expected one type diagnostic, got %v", diags)
			}
			want := "assert target must be a constraint usage, found " + tc.found
			if diags[0].Message != want {
				t.Errorf("got %q, want %q", diags[0].Message, want)
			}
		})
	}
}

// A named binding is a feature the builder leaves unclassified, and a named
// transition is an action usage; neither is a constraint or a requirement.
func TestAssertReferenceToBindingOrTransitionRejected(t *testing.T) {
	prefix := `part def Holder {
	attribute x; attribute y; binding b bind x = y; alias ab for b;
	state def SD { state s1; state s2; transition t first s1 then s2; }
	state sm : SD;
}
part h : Holder; attribute x; attribute y; binding b bind x = y; alias ab for b;
part ctx { `
	tests := []struct {
		name, target, want string
	}{
		{"binding", "assert b;", "assert target must be a constraint usage, found binding"},
		{"negated binding", "assert not b;", "assert target must be a constraint usage, found binding"},
		{"alias to binding", "assert ab;", "assert target must be a constraint usage, found binding"},
		{"chained binding", "assert h.b;", "assert target must be a constraint usage, found binding"},
		{"chained alias to binding", "assert h.ab;", "assert target must be a constraint usage, found binding"},
		{"chained transition", "assert h.sm.t;", "assert target must be a constraint usage, found actionUsage"},
		{"binding in constraint body", "} constraint def K { assert b;", "assert target must be a constraint usage, found binding"},
		{"satisfy binding", "satisfy b;", "satisfy target must be a requirement usage, found binding"},
		{"satisfy chained binding", "satisfy h.b;", "satisfy target must be a requirement usage, found binding"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := typeDiags(t, prefix+tc.target+" }")
			if len(diags) != 1 {
				t.Fatalf("expected one type diagnostic, got %v", diags)
			}
			if diags[0].Message != tc.want {
				t.Errorf("got %q, want %q", diags[0].Message, tc.want)
			}
		})
	}
	silent := "ref r references b; ref r2 :> b; ref r3 references h.b; }"
	if diags := typeDiags(t, prefix+silent); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}
}

// An unresolved assert target is the name-resolution tier's finding alone.
func TestAssertReferenceUnresolvedIsNotAKindError(t *testing.T) {
	src := "part def Derived; part v : Derived; part ctx { assert missing; assert v.missing; }" +
		" constraint def K { assert missing; assert v.missing; }"
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}
}

// In a constraint body `assert c;` is the same reference form, stated by a
// constraint member rather than a usage, and a bare condition stays a condition.
func TestAssertReferenceInConstraintBodyToConstraintIsSilent(t *testing.T) {
	src := `constraint def CD; requirement def RD;
part def Base { constraint inherited : CD; }
part def Derived :> Base { constraint c : CD; requirement r : RD; }
part v : Derived; attribute flag;
constraint def K {
	constraint local : CD;
	assert local;
	assert not local;
	assert v.c;
	assert v.inherited;
	assert v.r;
	assert constraint { assert local; }
	flag
}
constraint k : K { assert local; }`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}
}

func TestAssertReferenceInConstraintBodyToNonConstraintRejected(t *testing.T) {
	tests := []struct {
		name, target, found string
	}{
		{"part usage", "assert p;", "partUsage"},
		{"negated part usage", "assert not p;", "partUsage"},
		{"attribute usage", "assert flag;", "attributeUsage"},
		{"chained part usage", "assert v.p;", "partUsage"},
		{"chained inherited part", "assert v.bp;", "partUsage"},
		{"constraint definition", "assert CD;", "constraintDef"},
		{"part definition", "assert PD;", "partDef"},
		{"package", "assert Q;", "package"},
		{"nested body", "assert constraint { assert p; }", "partUsage"},
		{"constraint usage body", "} constraint k : K { assert p;", "partUsage"},
	}
	prefix := `constraint def CD; part def PD; package Q;
part def Base { part bp; }
part def Derived :> Base { part p; }
part v : Derived; part p; attribute flag;
constraint def K { `
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := typeDiags(t, prefix+tc.target+" }")
			if len(diags) != 1 {
				t.Fatalf("expected one type diagnostic, got %v", diags)
			}
			want := "assert target must be a constraint usage, found " + tc.found
			if diags[0].Message != want {
				t.Errorf("got %q, want %q", diags[0].Message, want)
			}
		})
	}
}

// `satisfy` shares the referent-kind rule, so a chained target is checked too.
func TestSatisfyChainedNonRequirementRejected(t *testing.T) {
	src := "constraint def CD; requirement def RD; part def D { constraint c : CD; requirement r : RD; }" +
		" part v : D; part ctx { satisfy v.r; satisfy v.c; }"
	diags := typeDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "satisfy target must be a requirement usage, found constraintUsage") {
		t.Errorf("got %q", diags[0].Message)
	}
}

// An objective is a requirement usage (SysML v2 §8.3.22.4), so it is a valid
// referent of `assert` and `satisfy` directly, negated, chained and in a body.
func TestAssertReferenceToObjectiveIsSilent(t *testing.T) {
	src := `requirement def RD; constraint def CD;
use case def UCD { subject s; objective obj : RD; }
use case uc : UCD { objective obj : RD; assert obj; assert not obj; }
verification vc { objective vobj : RD; alias aobj for vobj; }
part ctx {
	assert uc.obj;
	assert not uc.obj;
	assert vc.vobj;
	assert vc.aobj;
	assert uc.obj { true }
	satisfy uc.obj;
	satisfy vc.aobj;
	requirement r2 :> uc.obj;
	constraint k : CD { assert uc.obj; }
}`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}
}

// An assertion borrows the name of the feature it references, so a chain through
// its own owner reaches the owner's member of that name, not the assertion.
func TestAssertReferenceThroughOwnChainNamesTheOwnersMember(t *testing.T) {
	src := `constraint def CD; part def H { part q; constraint c : CD; }
part h : H { assert h.q; assert h.c; }
part h2 : H { part local; assert not h2.local; assert not h2.c; }`
	diags := typeDiags(t, src)
	if len(diags) != 2 {
		t.Fatalf("expected two type diagnostics (h.q, h2.local), got %v", diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "assert target must be a constraint usage, found partUsage") {
			t.Errorf("got %q", d.Message)
		}
	}
	got := nameresDiags(t, `part def H { part q; }
part h : H { assert h.missing; }`)
	if len(got) != 1 || !strings.Contains(got[0].Message, "missing") {
		t.Fatalf("got %+v, want one unresolved diagnostic for h.missing", got)
	}
}

// Nor is the assertion a member `q` of its owner for anyone else: `h.q` written
// outside still names `H::q` (the pinned pilot rejects both assertions here).
func TestAssertReferenceNamesNoMemberOfItsOwner(t *testing.T) {
	src := `constraint def CD; part def H { part q; constraint c : CD; }
part h : H { assert q; }
part ctx { assert h.q; assert h.c; }`
	diags := typeDiags(t, src)
	if len(diags) != 2 {
		t.Fatalf("expected two type diagnostics (q, h.q), got %v", diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "assert target must be a constraint usage, found partUsage") {
			t.Errorf("got %q", d.Message)
		}
	}
	if got := nameresDiags(t, src); len(got) != 0 {
		t.Fatalf("got %+v, want no name-resolution diagnostics", got)
	}
}

// The other requirement-parameter usages stay parts and stay rejected.
func TestAssertReferenceToRequirementParameterPartsRejected(t *testing.T) {
	tests := []struct {
		name, target, found string
	}{
		{"subject", "assert uc.s;", "partUsage"},
		{"actor", "assert uc.a;", "partUsage"},
		{"stakeholder", "assert r.sh;", "partUsage"},
		{"satisfy subject", "satisfy uc.s;", "partUsage"},
	}
	prefix := `requirement def RD; part def PD;
use case uc { subject s : PD; actor a : PD; objective obj : RD; }
requirement r : RD { stakeholder sh : PD; }
part ctx { `
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := typeDiags(t, prefix+tc.target+" }")
			if len(diags) != 1 {
				t.Fatalf("expected one type diagnostic, got %v", diags)
			}
			if !strings.Contains(diags[0].Message, "found "+tc.found) {
				t.Errorf("got %q, want found %s", diags[0].Message, tc.found)
			}
		})
	}
}
