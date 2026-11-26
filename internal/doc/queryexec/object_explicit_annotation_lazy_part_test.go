package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// A lazy part named like the evaluator's annotations property must stay lazy
// when the query asks for the declaration's metadata by its unambiguous name.
const lazyAnnotationsPartFixture = `
metadata def Tag { attribute label : String; }
state def PayloadMachine { state running; }
part def Payload { exhibit state payloadMode : PayloadMachine; }
part def Device { part annotations : Payload[0..1]; }
part device : Device;
@Tag about device { label = "alpha"; }
calc def Metadata :> Query {
	in root : Element;
	Project(source = root, properties = ("@annotations"))
}
calc def NamedPart :> Query {
	in root : Element;
	Project(source = root, properties = ("annotations"), tolerate = true)
}
`

func TestExplicitAnnotationsOnAHeldObjectDoNotMaterializeANamesakePart(t *testing.T) {
	fixture := loadExecutionFixture(t, lazyAnnotationsPartFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	device, err := ctx.Instantiate(fixture.symbol(t, "device"))
	if err != nil {
		t.Fatalf("Instantiate device: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "device", Object: device}},
	}
	lazy := device.FeatureValues["annotations"]
	if lazy == nil || lazy.Materialized {
		t.Fatalf("annotations feature before query = %+v, want an unmaterialized part", lazy)
	}
	result, err := Execute(fixture.program(t, "Metadata"), context,
		Bindings{"root": {ObjectValue(device, "device")}}, Options{})
	if err != nil {
		t.Fatalf("execute Metadata: %v", err)
	}
	if got := cellStrings(t, result, "@annotations"); len(got) != 1 || len(got[0]) != 1 || got[0][0] != "Observatory::Tag::label=alpha" {
		t.Fatalf("@annotations = %q, want the declaration binding", got)
	}
	if lazy.Materialized {
		t.Fatal("projecting @annotations materialized the annotations part")
	}
	// The probe is sensitive: the bare name is the feature, so reading it
	// materializes the part even when the read has no value to report.
	_, err = Execute(fixture.program(t, "NamedPart"), context,
		Bindings{"root": {ObjectValue(device, "device")}}, Options{})
	if err == nil {
		t.Fatal("execute NamedPart succeeded, want an unevaluable feature")
	}
	if !lazy.Materialized {
		t.Fatal("reading the bare annotations name did not materialize the part")
	}
}
