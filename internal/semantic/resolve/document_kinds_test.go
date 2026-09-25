package resolve

import "testing"

func TestResolveConnectorEndsClean(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"part def Sys { part a; part b; connection c connect a to b; }")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

func TestResolveFlowEndsAndPayloadClean(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"part def Sys { item Fuel; part a { out item outFuel : Fuel; } part b { in item inFuel : Fuel; } flow f of Fuel from a.outFuel to b.inFuel; }")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

func TestResolveUnresolvedConnectorEnd(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"part def Sys { part a; connection c connect a to missing; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved-name diagnostic for the 'missing' end")
	}
}

func TestResolveNamedNaryConnectorEnds(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package T {
		part def L;
		part def P;
		allocation def LtoP { end logical : L; end physical : P; }
		part l : L;
		part p : P;
		allocation a2 : LtoP allocate (logical ::> l, physical ::> p);
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected named n-ary allocation ends to resolve, got %v", r.Diagnostics)
	}
}

func TestResolveKerMLBindingEndsWithLeadingMultiplicityClean(t *testing.T) {
	r := resolveDoc(t, "d.kerml", `package T {
		struct S { feature a; feature b; }
		feature p : S;
		binding [1] p.a = [1] p.b;
		binding [1] e ::> p.a = p.b;
		binding [0..1] f references p.a = [1] p.b;
		binding [1] of p.a = p.b;
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected KerML binding ends to resolve, got %v", r.Diagnostics)
	}
}

func TestResolveNamedConnectorEndReferenceStillReportsMissingTarget(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package T {
		part def L;
		allocation def LtoP { end logical : L; }
		part l : L;
		allocation a : LtoP allocate (logical ::> missing, l);
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected an unresolved-name diagnostic for the named end reference")
	}
}
