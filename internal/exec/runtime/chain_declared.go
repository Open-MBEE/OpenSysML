package runtime

import (
	"errors"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// declaredChainValue reads `usage.member` from a package-level usage no object stands for
// through the object it would materialize, made and taken back under a probe; ok if read so.
func (ec *EvalContext) declaredChainValue(base ast.Node, parts []ast.NameSegment) (Value, bool, error) {
	owner, ok := ec.chainBaseSymbol(base)
	if !ok || len(parts) == 0 || !ec.readsDeclaredMembers(owner) {
		return Value{}, false, nil
	}
	// A member whose value reads this very chain again is a cycle.
	read := chainRead{usage: owner, path: joinSegments(parts)}
	if slices.Contains(ec.ctx.chainStack, read) {
		return Value{}, true, &cyclicBindingError{Usage: owner, Stated: ec.ctx.QualifiedName(owner) + "." + read.path}
	}
	ec.ctx.chainStack = append(ec.ctx.chainStack, read)
	defer func() { ec.ctx.chainStack = ec.ctx.chainStack[:len(ec.ctx.chainStack)-1] }()
	defer ec.ctx.beginProbe()()
	inst, err := ec.ctx.materialize(owner, 0, nil, "")
	if err != nil {
		return Value{}, true, err
	}
	// Recorded as the usage's occurrence while it lasts, so the model-level read
	// asserts only what the model states and makes up no open count.
	prior, hadPrior := ec.ctx.occurrences[owner]
	ec.ctx.noteProbeUndo(func() {
		if hadPrior {
			ec.ctx.occurrences[owner] = prior
		} else {
			delete(ec.ctx.occurrences, owner)
		}
	})
	ec.ctx.occurrences[owner] = []int64{inst.ID}
	val, err := ec.chainMemberValue(Value{Kind: ValInstance, Instance: inst.ID}, parts, owner.Name)
	if errors.Is(err, errUninitializedFeatureValue) || errors.Is(err, ErrNoSuchFeature) {
		return Value{}, false, nil
	}
	if err != nil {
		return Value{}, true, err
	}
	// What the object leaves open, and the objects and calcs it holds, go back with it.
	if val.Kind == ValUndetermined || val.Kind == ValFunction || len(instancesOf(ec.ctx, standingElements(val))) > 0 {
		return Value{}, false, nil
	}
	return val, true, nil
}

// readsDeclaredMembers reports whether a chain through sym is read from the object it would
// materialize: a valueless namespace-level usage of exactly one, not a type, calc or variation.
func (ec *EvalContext) readsDeclaredMembers(sym *symbols.Symbol) bool {
	if sym.OwnerScope == nil || !namespaceScope(sym.OwnerScope) {
		return false
	}
	if _, usage := sym.Decl.(*ast.Usage); !usage || declaresType(sym) || isCalcUsageSymbol(sym) {
		return false
	}
	if ec.ctx.extractDefaultValue(sym) != nil || ec.ctx.model.semantics.IsVariationFeature(sym) {
		return false
	}
	n, exact := ec.ctx.featureMultiplicity(sym, nil).Exactly()
	return exact && n == 1
}

// chainRead is one chain under a model-level read: the usage it starts from and
// the dotted member path it reads.
type chainRead struct {
	usage *symbols.Symbol
	path  string
}

func joinSegments(parts []ast.NameSegment) string {
	names := make([]string, len(parts))
	for i, part := range parts {
		names[i] = part.Text
	}
	return strings.Join(names, ".")
}
