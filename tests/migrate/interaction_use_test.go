package migrate_test

import (
	stderrors "errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// testdata/xmi/interaction_use.xmi: an interaction use performs the referenced interaction's
// scenario at its place among the messages, its arguments bound to the scenario's parameters and
// a message through an actual gate carried by the performed scenario; a state invariant is an
// assertion the scenario checks where it is placed. A use of nothing or of the interaction itself
// is refused, and so is an invariant whose condition is prose.
func TestInteractionUsesAndStateInvariants(t *testing.T) {
	r := migrateFixtureFile(t, "interaction_use")
	for _, line := range []string{
		"action 'spin Up' {",
		"in rpm : ScalarValues::Real[1];",
		"perform action spin ::> drive.motor.spin { in rpm[1] = rpm; }",
		"assert constraint fast { drive.motor.speed > limit }",
		"first spin then fast;",
		"first fast then done;",
		"action run {",
		"perform action warm ::> 'spin Up' { in rpm[1] = 30.0; }",
		"first start then warm;",
		"assert constraint at30 { drive.motor.speed == 30.0 }",
		"first warm then at30;",
		"first at30 then slow;",
		"assert constraint slowed { drive.motor.speed < limit }",
		"first slow then slowed;",
		"perform action 'spin Up2' ::> 'spin Up' { in rpm[1] = 5.0; }",
		"first spin then 'spin Up2';",
		"perform action kick ::> drive.kick;",
		"first kick then at20;",
		"/* not migrated: Interaction 'Astray' — the interaction use (_aUse) refers to no interaction (1 refersTo reference(s) resolve to nothing in the document (_gone)) */",
		"/* not migrated: Interaction 'Again' — the interaction use (_gUse) refers to 'Again', which is not migrated: the interaction refers to itself through an interaction use */",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_runUse", migrate.Mapped, "written as the perform warm of the scenario of 'Spin Up'")
	wantNote(t, r, "_oUse", migrate.Mapped, "written as the perform 'spin Up2' of the scenario of 'Spin Up'")
	wantNote(t, r, "_ryUse", migrate.Mapped, "written as the perform kick of the scenario of 'Kick'")
	wantNote(t, r, "_ryGate", migrate.Mapped, "the gate passes its message into the performed scenario kick")
	wantNote(t, r, "_ryCmd", migrate.Approximated, "the arguments it carries into the gate are those the message 'kickSpin' of 'Kick' states")
	wantNote(t, r, "_kickIn", migrate.Mapped, "the gate is the scenario's edge")
	wantNote(t, r, "_runAt30", migrate.Mapped, "written as the assertion at30, which the scenario checks when it reaches it")
	wantNote(t, r, "_runAt30C", migrate.Mapped, "the constraint is the condition of the assertion at30")
	wantNote(t, r, "_suFast", migrate.Mapped, "written as the assertion fast")
	wantNote(t, r, "_runProse", migrate.Unmapped, "the state invariant's condition [{English} the motor runs quietly] is not written")
	wantNote(t, r, "_run", migrate.Approximated, "written as a scenario of 4 steps, one per message, interaction use or state invariant in occurrence order")
	wantNote(t, r, "_again", migrate.Unmapped, "the interaction refers to itself through an interaction use")
	wantNote(t, r, "_astray", migrate.Unmapped, "refers to no interaction")
	for _, d := range errors(t, "interaction_use.sysml", r.Notation) {
		t.Errorf("%v", d)
	}

	ctx, idx := migratedContext(t, r.Notation)
	sym := func(fqn string) *symbols.Symbol {
		t.Helper()
		syms := idx.LookupQualified(fqn)
		if len(syms) != 1 {
			t.Fatalf("%s names %d symbols, want one", fqn, len(syms))
		}
		return syms[0]
	}
	rig, err := ctx.Instantiate(sym("Rig"))
	if err != nil {
		t.Fatalf("instantiate Rig: %v", err)
	}
	speed := func() float64 {
		t.Helper()
		obj := rig
		for _, name := range []string{"drive", "motor"} {
			fv, err := obj.GetFeatureValue(ctx, name)
			if err != nil || fv.Value.Kind != runtime.ValInstance {
				t.Fatalf("read %s: %v, %v", name, fv.Value, err)
			}
			next, ok := ctx.Instance(fv.Value.Instance)
			if !ok {
				t.Fatalf("%s names no instance", name)
			}
			obj = next
		}
		fv, err := obj.GetFeatureValue(ctx, "speed")
		if err != nil || fv.Value.Kind != runtime.ValConst || fv.Value.Const.Kind != semantics.ValReal {
			t.Fatalf("read drive.motor.speed: %v, %v", fv.Value, err)
		}
		return fv.Value.Const.Real
	}

	// The performed scenario spins the motor to 30 and checks it is fast; run's own invariant then
	// holds before its next message slows the motor.
	if _, err := ctx.ExecuteActionPerformedBy(sym("Rig::run"), rig, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := speed(); got != 5.0 {
		t.Errorf("after run, drive.motor.speed = %v, want 5.0", got)
	}
	if _, err := ctx.ExecuteActionPerformedBy(sym("Rig::relay"), rig, nil); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if got := speed(); got != 20.0 {
		t.Errorf("after relay, drive.motor.speed = %v, want 20.0", got)
	}
	// The performed scenario spins the motor to 5, so its invariant fails where it is placed.
	_, err = ctx.ExecuteActionPerformedBy(sym("Rig::overrun"), rig, nil)
	var violation *runtime.ViolationError
	if !stderrors.As(err, &violation) || violation.Element != "fast" {
		t.Fatalf("overrun: err = %v, want the assertion fast violated", err)
	}
	if got := speed(); got != 5.0 {
		t.Errorf("after overrun, drive.motor.speed = %v, want 5.0", got)
	}
}

// migratedContext resolves migrated notation against the standard library for execution.
func migratedContext(t *testing.T, notation []byte) (*runtime.Context, *symbols.Index) {
	t.Helper()
	p := parser.New(source.New("migrated.sysml", notation))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse the migrated notation: %v", p.Diagnostics[0])
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("migrated.sysml", root)
	idx.ExpandWildcardImports()
	res := resolve.New(idx)
	return runtime.NewContext(runtime.NewModel(passes.NewTypedModel(res), res), 10_000_000), idx
}
