package runtime

import "testing"

// openCountMemberModel holds a package-level part whose member's count the model
// leaves open, beside parts a binding, a redefinition or a fixed count close,
// and a holder whose bound members read chains through each of them.
const openCountMemberModel = `package test {
	private import ScalarValues::Real;
	part def Sub { attribute limit : Real = 5.0; }
	part def Req { part some : Sub[1..*]; attribute own : Real = some.limit; }
	part req : Req;
	part def Bound { part some : Sub[1..*] = (a, b); part a : Sub; part b : Sub; }
	part bound : Bound;
	part closed : Req { part :>> some[2]; }
	part def Fixed { part two : Sub[2]; part one : Sub; }
	part fixed : Fixed;
	part def Opt { part maybe : Sub[0..1]; }
	part opt : Opt;
	part def Holder {
		attribute boundLimit : Real = req.some.limit;
		attribute viaOwn : Real = req.own;
		attribute optional : Real[0..1] = opt.maybe.limit;
		attribute fromBinding : Real[*] nonunique = bound.some.limit;
		attribute fromRedefinition : Real[*] nonunique = closed.some.limit;
		attribute fromFixed : Real[*] nonunique = fixed.two.limit;
		attribute fromOne : Real = fixed.one.limit;
	}
	part h : Holder;
}`

// A bound member of an instantiated object reading a chain through a package-level
// usage whose member's count the model leaves open is undetermined, as the chain
// read directly is: nothing in the model picks an element of `some[1..*]`.
func TestBoundMemberOverOpenCountChainIsUndetermined(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, openCountMemberModel))
	scope := oneSymbol(t, idx, "test").Scope
	for src, count := range map[string]string{"req.some.limit": "[1..*]", "req.own": "[1..*]", "opt.maybe.limit": "[0..1]", "h.boundLimit": "[1..*]"} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::h"))
	if err != nil {
		t.Fatalf("instantiate h: %v", err)
	}
	for src, count := range map[string]string{"boundLimit": "[1..*]", "viaOwn": "[1..*]", "optional": "[0..1]"} {
		val, err := evalOn(t, ctx, inst, src)
		wantUndetermined(t, "h."+src, val, err, count)
	}
}

// A chain whose count a binding, a redefinition or a fixed multiplicity closes reads
// its values through a bound member as it does directly.
func TestBoundMemberOverClosedCountChainEvaluates(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, openCountMemberModel))
	scope := oneSymbol(t, idx, "test").Scope
	for src, want := range map[string]string{
		"bound.some.limit": "[5.0, 5.0]", "closed.some.limit": "[5.0, 5.0]", "fixed.two.limit": "[5.0, 5.0]", "fixed.one.limit": "5.0",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::h"))
	if err != nil {
		t.Fatalf("instantiate h: %v", err)
	}
	for src, want := range map[string]string{
		"fromBinding": "[5.0, 5.0]", "fromRedefinition": "[5.0, 5.0]", "fromFixed": "[5.0, 5.0]", "fromOne": "5.0",
	} {
		val, err := evalOn(t, ctx, inst, src)
		wantFormatted(t, "h."+src, val, err, want)
	}
}

// An instantiated object keeps the lower bound it materializes: its own open-count
// member and a feature reading it are definite there, and only the object the
// model alone stands behind reads undetermined.
func TestInstantiatedObjectReadsItsOwnOpenCountMember(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, openCountMemberModel))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::req"))
	if err != nil {
		t.Fatalf("instantiate req: %v", err)
	}
	for src, want := range map[string]string{"some.limit": "[5.0]", "own": "5.0"} {
		val, err := evalOn(t, ctx, inst, src)
		wantFormatted(t, "req."+src, val, err, want)
	}
}
