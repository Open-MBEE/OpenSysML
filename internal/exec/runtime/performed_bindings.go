package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// performedPinBindings lowers the bindings the performer and the objects owning it
// write through the performed feature into graph's nodes (`bind run.measure.report =
// theReport;`), which hold at those pins as the performance runs. A failing end
// is left to the model-level read of the pin, which reports it there.
func (ctx *Context) performedPinBindings(performed *symbols.Symbol, graph *lower.ActionGraph, self *Instance) []throughBinding {
	if performed == nil || graph == nil || self == nil || len(graph.Nodes) == 0 {
		return nil
	}
	var through []throughBinding
	prefix := []string{performed.Name}
	for current := self; current != nil; {
		var bindings []lower.Binding
		for _, typ := range current.types() {
			bindings = append(bindings, ctx.objectBindings(typ)...)
		}
		lowered, err := lower.PerformedPinBindings(graph, prefix, bindings)
		if err == nil {
			for _, binding := range lowered {
				through = append(through, throughBinding{PinBinding: binding, on: current})
			}
		}
		owner, ownerFeature := current.Owner()
		if owner == nil || ownerFeature == "" {
			break
		}
		prefix = append([]string{ownerFeature}, prefix...)
		current = owner
	}
	return through
}

// objectEndHeld reads what the feature a binding end names on the object it is
// written on holds, held false while it holds nothing yet.
func (ctx *Context) objectEndHeld(on *Instance, name string) (Value, bool, error) {
	if _, declared := on.FeatureValues[name]; !declared {
		return Value{}, false, fmt.Errorf("%w: %s is no feature of object #%d", errBindingEnd, name, on.ID)
	}
	fv, err := on.GetFeatureValue(ctx, name)
	if err != nil {
		return Value{}, false, err
	}
	value := fv.HeldValue()
	if value.Kind == ValInvalid {
		return Value{}, false, nil
	}
	return value, true, nil
}
