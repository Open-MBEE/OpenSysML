package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// heldSubrequirement reports a subrequirement usage (SysML v2 §7.21.2) its container's object holds
// an object of, a value of `RequirementCheck::subrequirements`; an abstract one holds contributions only.
func (ctx *Context) heldSubrequirement(sym *symbols.Symbol) bool {
	return sym != nil && !symbols.IsAbstract(sym) && ctx.model.semantics.IsSubrequirement(sym)
}

// containerSubject is the object holding inst and the name of its subject, which the subject of inst is bound
// to: the container's, for a subrequirement owning no subject declaration (SysML v2 §7.21.2).
func (ctx *Context) containerSubject(inst *Instance) (container *Instance, subject string, ok bool) {
	owner, name := inst.Owner()
	if owner == nil {
		return nil, "", false
	}
	held, has := owner.FeatureValues[name]
	if !has || held.Feature == nil || !ctx.heldSubrequirement(held.Feature.Symbol) {
		return nil, "", false
	}
	if owned, _ := ctx.model.semantics.SubjectsOf(held.Feature.Symbol); len(owned) > 0 {
		return nil, "", false
	}
	for _, subject := range ctx.subjectParameterNames(owner.Type) {
		if _, has := owner.FeatureValues[subject]; has {
			return owner, subject, true
		}
	}
	return nil, "", false
}

// containerSubjectValue is the value the container's subject holds for inst (see containerSubject) and
// the feature value it was read from; not ok where inst reads none or the container's is unbound.
func (ctx *Context) containerSubjectValue(inst *Instance) (Value, *FeatureValue, bool, error) {
	container, subject, ok := ctx.containerSubject(inst)
	if !ok {
		return Value{}, nil, false, nil
	}
	fv, err := container.GetFeatureValue(ctx, subject)
	if err != nil {
		return Value{}, nil, false, fmt.Errorf("subject %s of the requirement containing %s: %w", subject, inst.Type.Name, err)
	}
	val := fv.HeldValue()
	if val.Kind == ValInvalid || val.Kind == ValUndetermined {
		return Value{}, nil, false, nil
	}
	return val, fv, true, nil
}

// materializeFromContainerSubject holds in fv, the subject of a subrequirement's object declaring none
// of its own, the value its container's subject holds. Reports whether it held any.
func (inst *Instance) materializeFromContainerSubject(ctx *Context, fv *FeatureValue, name string) (bool, error) {
	if !isSubjectUsage(fv.Feature.Symbol) {
		return false, nil
	}
	val, read, ok, err := ctx.containerSubjectValue(inst)
	if !ok || err != nil {
		return false, err
	}
	ctx.listRead(read, fv)
	if err := ctx.checkDefault(inst, fv, name, &val, admitDeclared); err != nil {
		return false, err
	}
	if val, err = ctx.holdDeclared(inst, fv, val); err != nil {
		return false, err
	}
	if fv.Feature.Scalar() {
		fv.Value = val
	} else {
		fv.Values = val
	}
	fv.Materialized = true
	return true, nil
}
