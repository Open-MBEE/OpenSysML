package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

const metadataFeatureShadowingFixture = `
private import ScalarValues::*;
part def Tagged {
	attribute isActive : Boolean = true;
	attribute annotations : String = "feature value";
}
part tagged : Tagged;
part plain;
calc def Q :> Query {
	in root : Element;
	Project(
		source = Union(
			source = Named(qualifiedName = "Observatory::tagged"),
			other = Named(qualifiedName = "Observatory::plain")
		),
		properties = ("name", "isActive", "annotations", "@isActive", "@annotations"),
		tolerate = true
	)
}
calc def Objects :> Query {
	in root : Element;
	Project(source = root, properties = ("name", "isActive", "annotations", "@isActive", "@annotations"), tolerate = true)
}
`

// A declared feature keeps its name. The evaluator's properties of the same
// name are reached by the "@" spelling, which a model feature cannot shadow.
func TestQueryDeclaredFeaturesShadowBuiltinsByName(t *testing.T) {
	fixture := loadExecutionFixture(t, metadataFeatureShadowingFixture)
	result, err := fixture.execute(t, "Q", Bindings{
		"root": {ElementValue(fixture.symbol(t, "tagged"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Q: %v", err)
	}
	names := cellStrings(t, result, "name")
	active := cellBooleans(t, result, "isActive")
	annotations := cellStrings(t, result, "annotations")
	aliasedActive := cellStrings(t, result, "@isActive")
	aliasedAnnotations := cellStrings(t, result, "@annotations")
	got := map[string]bool{}
	for i, name := range names {
		if len(name) != 1 {
			t.Fatalf("row %d name = %v", i, name)
		}
		got[name[0]] = len(active[i]) == 1 && active[i][0] &&
			len(annotations[i]) == 1 && annotations[i][0] == "feature value" &&
			len(aliasedActive[i]) == 0 && len(aliasedAnnotations[i]) == 0
	}
	if len(got) != 2 || !got["tagged"] || got["plain"] {
		t.Fatalf("declared-feature rows = %v, want tagged reading its features and plain none", got)
	}
}

// Held features of an object are read before the evaluator's properties of the
// same name, and the unambiguous spelling still reaches the evaluator.
func TestQueryHeldFeaturesShadowBuiltinsByName(t *testing.T) {
	fixture := loadExecutionFixture(t, metadataFeatureShadowingFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	object, err := ctx.Instantiate(fixture.symbol(t, "tagged"))
	if err != nil {
		t.Fatalf("Instantiate tagged: %v", err)
	}
	result, err := Execute(fixture.program(t, "Objects"), Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "tagged", Object: object}},
	}, Bindings{"root": {ObjectValue(object, "tagged")}}, Options{})
	if err != nil {
		t.Fatalf("execute Objects: %v", err)
	}
	if got := cellBooleans(t, result, "isActive"); len(got) != 1 || len(got[0]) != 1 || !got[0][0] {
		t.Errorf("isActive cells = %v, want [[true]]", got)
	}
	if got := cellStrings(t, result, "annotations"); len(got) != 1 || len(got[0]) != 1 || got[0][0] != "feature value" {
		t.Errorf("annotations cells = %q, want [[\"feature value\"]]", got)
	}
	for _, column := range []string{"@isActive", "@annotations"} {
		if got := cellStrings(t, result, column); len(got) != 1 || len(got[0]) != 0 {
			t.Errorf("%s cells = %q, want no value on a non-state, unannotated object", column, got)
		}
	}
}
