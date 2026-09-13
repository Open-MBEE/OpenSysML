package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A binder reached as the document's symbol and as the index's symbol for one
// declaration conforms to itself; that must not read as a rebinding.
func TestBaseTypeReboundIgnoresTwinSymbols(t *testing.T) {
	const name = "t.sysml"
	src := `package P {
		metadata def vehicle { :>> baseType = vehicles; }
		metadata def truck :> vehicle { :>> baseType = trucks; }
	}`
	m, root := buildModelNamed(t, name, src)
	pkg := sym(t, root, "P")
	indexVehicle := sym(t, pkg.Scope, "vehicle")
	indexTruck := sym(t, pkg.Scope, "truck")

	p := parser.New(source.New(name, []byte(src)))
	docScope := symbols.Build(p.ParseFile())
	symbols.SetDocName(docScope, name)
	docVehicle := sym(t, sym(t, docScope, "P").Scope, "vehicle")
	if docVehicle == indexVehicle || !symbols.SameElement(docVehicle, indexVehicle) {
		t.Fatalf("want two symbols for one declaration, got %p and %p", docVehicle, indexVehicle)
	}

	twins := []*symbols.Symbol{docVehicle, indexVehicle}
	for _, twin := range twins {
		if m.baseTypeRebound(twin, twins) {
			t.Errorf("baseTypeRebound(%p, twins) = true, want a symbol not to rebind itself", twin)
		}
	}
	if !m.baseTypeRebound(indexVehicle, []*symbols.Symbol{docVehicle, indexTruck}) {
		t.Errorf("baseTypeRebound(vehicle, {vehicle, truck}) = false, want truck's binding to win")
	}
	if m.baseTypeRebound(indexTruck, []*symbols.Symbol{docVehicle, indexTruck}) {
		t.Errorf("baseTypeRebound(truck, {vehicle, truck}) = true, want truck kept")
	}
}
