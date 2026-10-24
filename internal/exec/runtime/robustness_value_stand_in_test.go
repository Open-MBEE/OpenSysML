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
	const outside = `package test {
		private import ScalarValues::*;
		state def Life { out result : Boolean[1]; attribute flag : Boolean[1]; entry; then idle; state idle; }
		part def Host { exhibit state life : Life; }
		part host : Host;
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

	t.Run("rolled_back_outside_write_restores_the_machine_data", func(t *testing.T) {
		ctx, host := standInObject(t, outside, "test::host")
		life := standInHolder(t, ctx, host, "life")
		standIn := standInOf(t, ctx, life, "result")
		_, rollback := ctx.beginJournal()
		if err := life.SetFeatureValue(ctx, "result", constInt(9)); err == nil {
			t.Fatalf("writing 9 to a Boolean feature succeeded")
		}
		if err := life.SetFeatureValue(ctx, "result", boolValue(true)); err != nil {
			t.Fatalf("write result: %v", err)
		}
		if got := machineData(t, host, "life")["result"]; !got.isBool() || !got.Const.Bool {
			t.Fatalf("machine data result = %s inside the journal, want true", FormatValue(got))
		}
		rollback()
		if got := standInOf(t, ctx, life, "result"); got != standIn {
			t.Fatalf("result holds #%d after the rollback, want the stand-in #%d", got, standIn)
		}
		got := machineData(t, host, "life")["result"]
		if got.Kind != ValInstance || got.Instance != standIn {
			t.Fatalf("machine data result = %s after the rollback, want the stand-in #%d", FormatValue(got), standIn)
		}
		if _, err := ctx.Image(host); err != nil {
			t.Fatalf("Image after the rollback: %v", err)
		}
	})

	for _, name := range []string{"result", "flag"} {
		t.Run("outside_write_of_"+name+"_reaches_the_machine_data", func(t *testing.T) {
			ctx, host := standInObject(t, outside, "test::host")
			life := standInHolder(t, ctx, host, "life")
			standIn := standInOf(t, ctx, life, name)
			if err := life.SetFeatureValue(ctx, name, boolValue(true)); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
			if _, live := ctx.Instance(standIn); live {
				t.Fatalf("stand-in #%d is still registered after the write", standIn)
			}
			data := machineData(t, host, "life")
			if got := data[name]; !got.isBool() || !got.Const.Bool {
				t.Fatalf("machine data %s = %s after the write, want true", name, FormatValue(got))
			}
			if _, err := ctx.Image(host); err != nil {
				t.Fatalf("Image after the write: %v", err)
			}
		})
	}

	t.Run("derived_attribute_follows_the_outside_write_and_the_part_round_trips", func(t *testing.T) {
		const derived = `package test {
			private import ScalarValues::*;
			state def Life { attribute flag : Boolean[1]; attribute d : Boolean = flag; entry; then idle; state idle; }
			part def Host { exhibit state life : Life; }
			part host : Host;
		}`
		ctx, host := standInObject(t, derived, "test::host")
		life := standInHolder(t, ctx, host, "life")
		standIn := standInOf(t, ctx, life, "flag")
		if got := heldFeatureValue(t, ctx, life, "d").HeldValue(); got.Kind != ValInstance || got.Instance != standIn {
			t.Fatalf("life.d = %s before the write, want the stand-in #%d", FormatValue(got), standIn)
		}
		if err := life.SetFeatureValue(ctx, "flag", boolValue(true)); err != nil {
			t.Fatalf("write flag: %v", err)
		}
		if got := heldFeatureValue(t, ctx, life, "d").HeldValue(); !got.isBool() || !got.Const.Bool {
			t.Fatalf("life.d = %s after the write, want true", FormatValue(got))
		}
		data := machineData(t, host, "life")
		for _, name := range []string{"flag", "d"} {
			if got := data[name]; !got.isBool() || !got.Const.Bool {
				t.Fatalf("machine data %s = %s after the write, want true", name, FormatValue(got))
			}
		}
		if _, live := ctx.Instance(standIn); live {
			t.Fatalf("stand-in #%d is still registered after the write", standIn)
		}
		copied := imageInto(t, ctx, host)
		if source, copy := heldDigest(ctx, everyObject), heldDigest(copied, everyObject); source != copy {
			t.Fatalf("the copy differs from the source:\n--- source\n%s\n--- copy\n%s", source, copy)
		}
	})

	t.Run("copied_stand_in_is_kept_while_another_feature_names_it", func(t *testing.T) {
		const copied = `package test {
			private import ScalarValues::*;
			part def P { attribute b : Boolean[1]; attribute copy : Boolean default = false; }
			part p : P;
		}`
		ctx, p := standInObject(t, copied, "test::p")
		standIn := standInOf(t, ctx, p, "b")
		if err := p.SetFeatureValue(ctx, "copy", Value{Kind: ValInstance, Instance: standIn}); err != nil {
			t.Fatalf("write copy: %v", err)
		}
		if err := p.SetFeatureValue(ctx, "b", boolValue(true)); err != nil {
			t.Fatalf("write b: %v", err)
		}
		if _, live := ctx.Instance(standIn); !live {
			t.Fatalf("stand-in #%d was dropped while copy names it", standIn)
		}
		if got := heldFeatureValue(t, ctx, p, "copy"); got.HeldValue().Kind != ValInstance || got.HeldValue().Instance != standIn || !got.Written {
			t.Fatalf("copy = %s written=%v after the write to b, want the stand-in #%d it was assigned", FormatValue(got.HeldValue()), got.Written, standIn)
		}
	})

	t.Run("direct_run_data_follows_a_nested_write", func(t *testing.T) {
		const direct = `package test {
			private import ScalarValues::*;
			action def Obs { in ref a : A; first start; then action w assign a.b := true; then done; }
			action def A {
				out b : Boolean[1];
				first start; then action obs : Obs { in ref :>> a = this; } then done;
			}
		}`
		idx, _, ctx := buildRuntimeWithLibraries(t, "<stand-in>", parseAndBuild(t, direct))
		matches := idx.LookupQualified("test::A")
		if len(matches) != 1 {
			t.Fatalf("test::A: %d matching symbols, want 1", len(matches))
		}
		outputs, err := ctx.ExecuteAction(matches[0])
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := outputs["b"]; !got.isBool() || !got.Const.Bool {
			t.Fatalf("b = %s after the nested write, want true", FormatValue(got))
		}
		for _, id := range ctx.InstanceIDs() {
			if ctx.HoldsNoValue(Value{Kind: ValInstance, Instance: id}) {
				t.Errorf("object #%d stands in for a value nothing holds", id)
			}
		}
	})

	t.Run("bound_stand_in_is_dropped_when_the_binding_resolves", func(t *testing.T) {
		const bound = `package test {
			private import ScalarValues::*;
			state def Life { out result : Boolean[1]; entry; then idle; state idle; }
			part def Host { attribute flag : Boolean[1]; exhibit state life : Life; bind life.result = flag; }
			part host : Host;
		}`
		ctx, host := standInObject(t, bound, "test::host")
		life := standInHolder(t, ctx, host, "life")
		standIn := standInOf(t, ctx, life, "result")
		if err := host.SetFeatureValue(ctx, "flag", boolValue(true)); err != nil {
			t.Fatalf("write flag: %v", err)
		}
		if got := heldFeatureValue(t, ctx, life, "result").HeldValue(); !got.isBool() || !got.Const.Bool {
			t.Fatalf("life.result = %s after flag is written, want true", FormatValue(got))
		}
		if _, live := ctx.Instance(standIn); live {
			t.Fatalf("stand-in #%d is still registered once the binding resolved", standIn)
		}
		if _, err := ctx.Image(host); err != nil {
			t.Fatalf("Image: %v", err)
		}
	})

	t.Run("open_collection_stand_in_is_dropped", func(t *testing.T) {
		const open = `package test {
			private import ScalarValues::*;
			state def Life { out results : Boolean[1..*]; entry; then work; state work { entry action e { assign results := (true, false); } } }
			part def Host { exhibit state life : Life; }
			part host : Host;
		}`
		ctx, _ := standInObject(t, open, "test::host")
		if got := ctx.InstanceCount(); got != 2 {
			t.Fatalf("InstanceCount = %d, want 2; ids %v", got, ctx.InstanceIDs())
		}
	})

	t.Run("redefined_parameter_data_agrees_under_both_names", func(t *testing.T) {
		const redefined = `package test {
			private import ScalarValues::*;
			state def Base { out result : Boolean[1]; }
			state def Life :> Base { out res :>> result; entry; then work; state work { entry action e { assign res := true; } } }
			part def Host { exhibit state life : Life; }
			part host : Host;
		}`
		ctx, host := standInObject(t, redefined, "test::host")
		if got := ctx.InstanceCount(); got != 2 {
			t.Fatalf("InstanceCount = %d, want 2; ids %v", got, ctx.InstanceIDs())
		}
		data := machineData(t, host, "life")
		for _, name := range []string{"res", "result"} {
			if got := data[name]; !got.isBool() || !got.Const.Bool {
				t.Fatalf("machine data %s = %s, want true", name, FormatValue(got))
			}
		}
	})
}

// machineData is the data of the state machine inst exhibits under name.
func machineData(t *testing.T, inst *Instance, name string) map[string]Value {
	t.Helper()
	for _, behavior := range inst.behaviors {
		if behavior.State != nil && behavior.Name == name {
			return behavior.State.StateData()
		}
	}
	t.Fatalf("#%d exhibits no machine %s", inst.ID, name)
	return nil
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
