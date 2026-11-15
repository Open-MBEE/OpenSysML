package runtime

import (
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Each performance of a named action node is an occurrence of its own (Performances.kerml
// `Performance :> Occurrence`; Actions.sysml `subactions :> subperformances`), a value of
// the node's feature on the performance around it, holding what the performance's features hold.

// errNodePerformanceOccurrence types a failure to hold a node's performance as an object.
var errNodePerformanceOccurrence = fmt.Errorf("action node performance occurrence")

// nodeFeature is the feature value owner holds the performances of perf's node under:
// the node's own declaration or one redefining it; nil where owner's type has no such member.
func (e *performances) nodeFeature(owner *Instance, perf *actionFrame) *FeatureValue {
	usage, ok := perf.node.(*ast.Usage)
	if !ok || perf.body || usage.IsBodyParameter || usage.Value != nil || usage.Ident.Name == "" || perf.flow == nil {
		return nil
	}
	sym, _ := stepSymbol(perf.flow, perf.node)
	if sym == nil {
		return nil
	}
	fv, held := owner.FeatureValues[usage.Ident.Name]
	if !held || fv.Feature == nil || fv.Feature.Symbol == nil {
		return nil
	}
	if member := fv.Feature.Symbol; member != sym && member.Decl != sym.Decl &&
		!slices.Contains(e.ctx.redefinedFeatures(member, owner.Type), sym) {
		return nil
	}
	return fv
}

// holdNodeOccurrence gives perf the object it runs as, where the performance around it
// runs as one: the first of the node's declared objects no performance has taken, else
// a new one after them, so the node's feature lists every performance in the order begun.
func (e *performances) holdNodeOccurrence(perf *actionFrame) error {
	if perf.occurrence != nil || perf.parent == nil {
		return nil
	}
	owner, err := e.frameObject(perf.parent)
	if owner == nil || err != nil {
		return err
	}
	fv := e.nodeFeature(owner, perf)
	if fv == nil {
		return nil
	}
	name := perf.node.(*ast.Usage).Ident.Name
	if fv, err = owner.getFeatureValue(e.ctx, name, nil); err != nil {
		return fmt.Errorf("%w: read %s of object #%d: %w", errNodePerformanceOccurrence, name, owner.ID, err)
	}
	elements, err := e.ctx.HeldElements(fv.HeldValue())
	if err != nil {
		return fmt.Errorf("%w: read %s of object #%d: %w", errNodePerformanceOccurrence, name, owner.ID, err)
	}
	var inst *Instance
	for _, element := range elements {
		id, ok := element.Object()
		if !ok {
			return nil
		}
		if held, ok := e.ctx.Instance(id); ok && !held.performed {
			inst = held
			break
		}
	}
	if inst == nil {
		if inst, err = e.ctx.materialize(fv.Feature.Symbol, 0, owner, name); err != nil {
			return fmt.Errorf("%w: hold %s of object #%d: %w", errNodePerformanceOccurrence, name, owner.ID, err)
		}
		if err := e.ctx.appendHeld(fv, Value{Kind: ValInstance, Instance: inst.ID}); err != nil {
			return fmt.Errorf("%w: hold %s of object #%d: %w", errNodePerformanceOccurrence, name, owner.ID, err)
		}
	}
	perf.occurrence = inst
	inst.performance, inst.performed = perf, true
	e.ctx.noteProbeUndo(func() { perf.occurrence, inst.performance, inst.performed = nil, nil, false })
	for _, key := range slices.Sorted(maps.Keys(perf.data)) {
		if err := e.holdOnOccurrence(perf, key, perf.data[key]); err != nil {
			return err
		}
	}
	e.ctx.beginPerformanceLife(inst, perf.began)
	if perf.ended {
		e.ctx.endPerformanceLife(inst)
	}
	return nil
}

// holdOnOccurrence has the object perf runs as hold value for the feature the frame
// holds under key, where its type has that feature: a tracked binding through its cell.
func (e *performances) holdOnOccurrence(perf *actionFrame, key string, value Value) error {
	inst := perf.occurrence
	if inst == nil || perf.body || inst.performance != perf {
		return nil
	}
	if _, held := inst.FeatureValues[key]; !held {
		return nil
	}
	var err error
	if cell := perf.cells.existingCell(key); cell != nil && cell.binding != nil && !cell.fv.Written {
		_, err = e.ctx.mirrorBodyCell(inst, key, cell, value)
	} else {
		e.ctx.mirroringBodyCell++
		err = inst.BindFeatureValue(e.ctx, key, value)
		e.ctx.mirroringBodyCell--
	}
	if err != nil {
		return fmt.Errorf("%w: write %s of object #%d: %w", errNodePerformanceOccurrence, key, inst.ID, err)
	}
	return nil
}

// writeFrameValue stores value under key in perf's data and on the object perf runs as.
func (e *performances) writeFrameValue(perf *actionFrame, key string, value Value) error {
	e.ctx.writeBodyValue(perf.cells, perf.data, key, value)
	return e.holdOnOccurrence(perf, key, value)
}

// holdNodeOccurrences gives every performance under frame begun before the run had an
// object the one it runs as, in the order they began.
func (e *performances) holdNodeOccurrences(frame *actionFrame) error {
	var under []*actionFrame
	for _, perf := range frame.subactions {
		under = append(under, perf)
	}
	for _, perfs := range frame.repeatedPerfs {
		for _, perf := range perfs {
			if perf != nil && !slices.Contains(under, perf) {
				under = append(under, perf)
			}
		}
	}
	sort.SliceStable(under, func(i, j int) bool { return under[i].began < under[j].began })
	for _, perf := range under {
		if err := e.holdNodeOccurrence(perf); err != nil {
			return err
		}
		if err := e.holdNodeOccurrences(perf); err != nil {
			return err
		}
	}
	return nil
}

// adoptPerformanceWrite has the performance inst is hold what a write landed on fv, a
// feature of the object, as a write through the performance's frame would.
func (ctx *Context) adoptPerformanceWrite(inst *Instance, fv *FeatureValue) {
	perf := inst.performance
	if perf == nil || perf.occurrence != inst {
		return
	}
	for name, held := range inst.FeatureValues {
		if held != fv {
			continue
		}
		if _, holds := perf.features[perf.key(name)]; holds {
			ctx.writeBodyValue(perf.cells, perf.data, perf.key(name), fv.HeldValue())
		}
	}
}

// calleeOccurrence is the object the action a node performs runs as: the node's own
// performance, where its object is of that action, so the two are one occurrence.
func (e *performances) calleeOccurrence(perf *actionFrame, action *symbols.Symbol) *Instance {
	inst := perf.occurrence
	if inst == nil || inst.performance != perf || !e.ctx.isOrSpecializes(inst.Type, action) {
		return nil
	}
	return inst
}

// declaresFlowNode reports whether sym declares an action usage a flow performs as a
// node of its own: one valued by no expression, no reference and no parameter.
func declaresFlowNode(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.Kind == ast.UsageAction && usage.Value == nil && !usage.IsBodyParameter &&
		!usage.IsReference && usage.Direction == ast.DirNone
}

// endNodeOccurrence ends the object perf runs as with the performance: completed, or
// dropped by a terminate around it.
func (e *performances) endNodeOccurrence(perf *actionFrame) {
	if perf.occurrence != nil && perf.occurrence.performance == perf {
		e.ctx.endPerformanceLife(perf.occurrence)
	}
}
