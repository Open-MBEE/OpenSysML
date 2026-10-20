package runtime

import (
	"testing"
)

// TestRuntimeRobustnessValueStandIn covers the object standing in for the
// unspecified value of a value-typed feature: dropped once a write gives the
// feature a value, kept by a refused or rolled-back write.
func TestRuntimeRobustnessValueStandIn(t *testing.T) {
	const exhibited = `package test {
		private import ScalarValues::*;
		state def Life {
			out result : Boolean[1];
			entry; then work;
			state work { entry action e { assign result := true; } }
		}
		part def Host { exhibit state life : Life; }
		part host : Host;
	}`
	const attribute = `package test {
		private import ScalarValues::*;
		part def P { attribute b : Boolean[1]; }
		part p : P;
	}`

	t.Run("stand_in_released_once_the_machine_writes", func(t *testing.T) {
		ctx, host := standInObject(t, exhibited, "test::host")
		life := standInHolder(t, ctx, host, "life")
		result := heldFeatureValue(t, ctx, life, "result")
		if !result.HeldValue().isBool() || !result.HeldValue().Const.Bool {
			t.Fatalf("life.result = %s, want true", FormatValue(result.HeldValue()))
		}
		if got := ctx.InstanceCount(); got != 2 {
			t.Fatalf("InstanceCount = %d, want 2 (the part and its state); ids %v", got, ctx.InstanceIDs())
		}
		for _, id := range ctx.InstanceIDs() {
			if inst, _ := ctx.Instance(id); ctx.HoldsNoValue(Value{Kind: ValInstance, Instance: id}) {
				t.Errorf("object #%d (%s) stands in for a value nothing holds", id, SymbolText(inst.Type))
			}
		}
	})

	t.Run("written_stand_in_is_dropped_once", func(t *testing.T) {
		ctx, p := standInObject(t, attribute, "test::p")
		standIn := standInOf(t, ctx, p, "b")
		before := ctx.InstanceCount()
		if err := p.SetFeatureValue(ctx, "b", boolValue(true)); err != nil {
			t.Fatalf("write b: %v", err)
		}
		if _, live := ctx.Instance(standIn); live {
			t.Fatalf("stand-in #%d is still registered after the write", standIn)
		}
		if got := ctx.InstanceCount(); got != before-1 {
			t.Fatalf("InstanceCount = %d after the write, want %d", got, before-1)
		}
		if err := p.SetFeatureValue(ctx, "b", boolValue(false)); err != nil {
			t.Fatalf("second write b: %v", err)
		}
		if got := ctx.InstanceCount(); got != before-1 {
			t.Fatalf("InstanceCount = %d after the second write, want %d", got, before-1)
		}
	})

	t.Run("refused_write_keeps_the_stand_in", func(t *testing.T) {
		ctx, p := standInObject(t, attribute, "test::p")
		standIn := standInOf(t, ctx, p, "b")
		if err := p.SetFeatureValue(ctx, "b", constInt(9)); err == nil {
			t.Fatalf("writing 9 to a Boolean feature succeeded")
		}
		if got := standInOf(t, ctx, p, "b"); got != standIn {
			t.Fatalf("b holds #%d after the refused write, want the stand-in #%d", got, standIn)
		}
	})

	t.Run("rolled_back_write_restores_the_stand_in", func(t *testing.T) {
		ctx, p := standInObject(t, attribute, "test::p")
		standIn := standInOf(t, ctx, p, "b")
		before := ctx.InstanceCount()
		_, rollback := ctx.beginJournal()
		if err := p.SetFeatureValue(ctx, "b", boolValue(true)); err != nil {
			t.Fatalf("write b: %v", err)
		}
		if _, live := ctx.Instance(standIn); live {
			t.Fatalf("stand-in #%d is still registered inside the journal", standIn)
		}
		rollback()
		if got := standInOf(t, ctx, p, "b"); got != standIn {
			t.Fatalf("b holds #%d after the rollback, want the stand-in #%d", got, standIn)
		}
		if got := ctx.InstanceCount(); got != before {
			t.Fatalf("InstanceCount = %d after the rollback, want %d", got, before)
		}
		if _, lived := ctx.lives[standIn]; !lived {
			t.Fatalf("stand-in #%d has no lifetime after the rollback", standIn)
		}
	})
}

// standInObject instantiates the usage fqn of source in a context with the libraries.
func standInObject(t *testing.T, source, fqn string) (*Context, *Instance) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<stand-in>", parseAndBuild(t, source))
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", fqn, len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate %s: %v", fqn, err)
	}
	return ctx, inst
}

// heldFeatureValue reads feature name of inst, materializing it.
func heldFeatureValue(t *testing.T, ctx *Context, inst *Instance, name string) *FeatureValue {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("read %s of #%d: %v", name, inst.ID, err)
	}
	return fv
}

// standInHolder is the object feature name of inst holds.
func standInHolder(t *testing.T, ctx *Context, inst *Instance, name string) *Instance {
	t.Helper()
	val := heldFeatureValue(t, ctx, inst, name).HeldValue()
	held, ok := ctx.Instance(val.Instance)
	if val.Kind != ValInstance || !ok {
		t.Fatalf("%s of #%d = %s, want an object", name, inst.ID, FormatValue(val))
	}
	return held
}

// standInOf is the identity of the object standing in for feature name of inst.
func standInOf(t *testing.T, ctx *Context, inst *Instance, name string) int64 {
	t.Helper()
	val := heldFeatureValue(t, ctx, inst, name).HeldValue()
	if !ctx.HoldsNoValue(val) {
		t.Fatalf("%s of #%d = %s, want a stand-in holding no value", name, inst.ID, FormatValue(val))
	}
	return val.Instance
}
