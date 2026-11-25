package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

const objectAnnotationsFixture = `
metadata def Tag { attribute label : String; }
part def Device {
	attribute annotations : String = "feature value";
}
part device : Device;
part plain;
@Tag about device { label = "alpha"; }
@Tag about plain { label = "beta"; }
calc def ObjectRows :> Query {
	Project(
		source = Objects(type = "PartUsage"),
		properties = ("name", "annotations", "@annotations"), tolerate = true
	)
}
calc def AnnotationsFilter :> Query {
	WhereFeature(
		source = Objects(type = "PartUsage"),
		'feature' = "@annotations", operator = "=", value = "Observatory::Tag::label=alpha"
	)
}
`

// An object reads its declaration's annotations like an element row; a held
// feature named annotations still owns the bare name, not the "@" spelling.
func TestQueryObjectRowsReadDeclarationAnnotations(t *testing.T) {
	fixture := loadExecutionFixture(t, objectAnnotationsFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	device, err := ctx.Instantiate(fixture.symbol(t, "device"))
	if err != nil {
		t.Fatalf("Instantiate device: %v", err)
	}
	plain, err := ctx.Instantiate(fixture.symbol(t, "plain"))
	if err != nil {
		t.Fatalf("Instantiate plain: %v", err)
	}
	roots := []Root{{Label: "device", Object: device}, {Label: "plain", Object: plain}}
	result, err := Execute(fixture.program(t, "ObjectRows"), Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx, Roots: roots,
	}, Bindings{}, Options{})
	if err != nil {
		t.Fatalf("execute ObjectRows: %v", err)
	}
	names := cellStrings(t, result, "name")
	annotations := cellStrings(t, result, "annotations")
	aliased := cellStrings(t, result, "@annotations")
	got := map[string][2][]string{}
	for i, name := range names {
		if len(name) != 1 {
			t.Fatalf("row %d name = %v", i, name)
		}
		got[name[0]] = [2][]string{annotations[i], aliased[i]}
	}
	want := map[string][2][]string{
		"device": {{"feature value"}, {"Observatory::Tag::label=alpha"}},
		"plain":  {{"Observatory::Tag::label=beta"}, {"Observatory::Tag::label=beta"}},
	}
	for name, cells := range want {
		found, ok := got[name]
		if !ok {
			t.Fatalf("no row for %s in %v", name, names)
		}
		for i, column := range []string{"annotations", "@annotations"} {
			if len(found[i]) != len(cells[i]) {
				t.Fatalf("%s of %s = %q, want %q", column, name, found[i], cells[i])
			}
			for j := range cells[i] {
				if found[i][j] != cells[i][j] {
					t.Fatalf("%s of %s = %q, want %q", column, name, found[i], cells[i])
				}
			}
		}
	}
}

// The annotations read from an object's declaration filter its row as well as
// projecting into it, without a per-row symbol lookup.
func TestQueryFiltersObjectRowsByDeclarationAnnotations(t *testing.T) {
	fixture := loadExecutionFixture(t, objectAnnotationsFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	device, err := ctx.Instantiate(fixture.symbol(t, "device"))
	if err != nil {
		t.Fatalf("Instantiate device: %v", err)
	}
	plain, err := ctx.Instantiate(fixture.symbol(t, "plain"))
	if err != nil {
		t.Fatalf("Instantiate plain: %v", err)
	}
	result, err := Execute(fixture.program(t, "AnnotationsFilter"), Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "device", Object: device}, {Label: "plain", Object: plain}},
	}, Bindings{}, Options{})
	if err != nil {
		t.Fatalf("execute AnnotationsFilter: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 1 {
		t.Fatalf("filtered rows = %d, want the device alone", len(rows))
	}
	_, label, ok := rows[0].Element().Object()
	if !ok || label != "device" {
		t.Fatalf("filtered row = %q, want device", label)
	}
}
