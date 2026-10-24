package analysis

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// CheckConditions are the constraints and requirements a check evaluates and assumes,
// resolved from the names a caller gave: Properties evaluated at every stable state
// about every object the invocation's behaviors perform on, each holding when it
// holds of each; Conditions the same symbols for the symbolic engine to translate;
// Assume the symbols it assumes; Scope the scope they resolve in.
type CheckConditions struct {
	Properties []runtime.CheckProperty
	Conditions []*symbols.Symbol
	Assume     []*symbols.Symbol
	Scope      *symbols.Scope
}

// ConditionCheck evaluates a constraint or requirement about an object.
type ConditionCheck func(*runtime.Context, *symbols.Symbol, *symbols.Scope, *runtime.Instance) (runtime.CheckResult, error)

// ResolveCheckConditions resolves the property and assumption names through lookup,
// root being the scope a symbol without an owning scope is read in. A name that
// resolves to neither a constraint nor a requirement is an error, as is a name
// given when nothing is loaded (root nil).
func ResolveCheckConditions(lookup func(name string) (*symbols.Symbol, error), root *symbols.Scope, properties, assume []string) (CheckConditions, error) {
	if (len(properties) > 0 || len(assume) > 0) && root == nil {
		return CheckConditions{}, errors.New("no declarations loaded")
	}
	resolved := CheckConditions{
		Properties: make([]runtime.CheckProperty, 0, len(properties)),
		Conditions: make([]*symbols.Symbol, 0, len(properties)),
		Assume:     make([]*symbols.Symbol, 0, len(assume)),
		Scope:      root,
	}
	for _, name := range properties {
		sym, check, err := ResolveCondition(lookup, name)
		if err != nil {
			return CheckConditions{}, err
		}
		resolved.Properties = append(resolved.Properties, checkProperty(name, sym, check, DeclaringScope(sym, root)))
		resolved.Conditions = append(resolved.Conditions, sym)
	}
	for _, name := range assume {
		sym, _, err := ResolveCondition(lookup, name)
		if err != nil {
			return CheckConditions{}, err
		}
		resolved.Assume = append(resolved.Assume, sym)
	}
	return resolved, nil
}

// ResolveCondition resolves name to the constraint or requirement it names and how
// the runtime evaluates it about an object.
func ResolveCondition(lookup func(name string) (*symbols.Symbol, error), name string) (*symbols.Symbol, ConditionCheck, error) {
	sym, err := lookup(name)
	if err != nil {
		return nil, nil, err
	}
	switch {
	case runtime.RequireConstraint(sym) == nil:
		return sym, (*runtime.Context).CheckConstraintOn, nil
	case runtime.RequireRequirement(sym) == nil:
		return sym, (*runtime.Context).CheckRequirementOn, nil
	}
	return nil, nil, fmt.Errorf("%q is not a constraint or requirement", name)
}

// DeclaringScope is the scope sym's names resolve in: its owner's, else root.
func DeclaringScope(sym *symbols.Symbol, root *symbols.Scope) *symbols.Scope {
	if sym != nil && sym.OwnerScope != nil {
		return sym.OwnerScope
	}
	return root
}

// checkProperty holds when the condition holds of every object the invocation
// performs on, or of no object when it performs on none.
func checkProperty(name string, sym *symbols.Symbol, check ConditionCheck, scope *symbols.Scope) runtime.CheckProperty {
	return runtime.CheckProperty{Name: name, Holds: func(ctx *runtime.Context, inv *runtime.Invocation) (bool, error) {
		performers := inv.Performers()
		if len(performers) == 0 {
			performers = []*runtime.Instance{nil}
		}
		for _, self := range performers {
			result, err := check(ctx, sym, scope, self)
			if err != nil && !errors.Is(err, runtime.ErrViolated) {
				return false, err
			}
			if !result.Holds {
				return false, nil
			}
		}
		return true, nil
	}}
}
