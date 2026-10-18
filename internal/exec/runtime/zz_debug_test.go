package runtime

import "testing"

func TestDebugRef(t *testing.T) {
	src := `package P {
    part def Node { attribute id : ScalarValues::Integer; }
    part def Base { part own : Node { :>> id default = 1; } ref part 'via' ::> own; }
    part def Derived :> Base { part renamed :>> own { :>> id = 2; } }
    part d : Derived;
}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<dbg>", parseAndBuild(t, src))
	root := idx.DocumentRoot("<dbg>")
	val, err := evalIn(t, ctx, root, "P::d")
	t.Logf("d = %v %v", val, err)
	inst, _ := ctx.Instance(val.Instance)
	for k, fv := range inst.FeatureValues {
		t.Logf("held %s sym=%s owner=%s materialized=%v", k, fv.Feature.Symbol.Name, fv.Feature.OwnerType.Name, fv.Materialized)
	}
	via := inst.FeatureValues["via"]
	t.Logf("referencedFeatureName = %q", ctx.referencedFeatureName(inst, via.Feature.Symbol))
	tgt := ctx.model.semantics.ReferencedFeature(via.Feature.Symbol)
	t.Logf("target=%v", tgt != nil)
	for _, typ := range inst.types() {
		m, ok := ctx.model.semantics.LookupMember(typ, "own")
		t.Logf("type %s lookup own ok=%v member=%v isFeatureOf=%v", typ.Name, ok, m != nil && m.Name != "", ctx.isFeatureOf(typ, tgt, via.Feature.Symbol))
	}
	v, err := inst.GetFeatureValue(ctx, "via")
	t.Logf("via = %v %v", v.HeldValue().Kind, err)
}
