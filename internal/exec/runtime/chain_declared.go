package runtime

import (
	"errors"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// declaredChainValue reads `usage.member` from a package-level usage no object stands for
// through the object it would materialize, made and taken back under a probe; ok if read so.
func (ec *EvalContext) declaredChainValue(base ast.Node, parts []ast.NameSegment) (Value, bool, error) {
	return ec.underDeclaredObject(base, parts, func(obj Value, owner *symbols.Symbol) (Value, error) {
		return ec.chainMemberValue(obj, parts, owner.Name)
	})
}

// underDeclaredObject answers answer over the object a package-level usage no object stands
// for would materialize, made and taken back under a probe: a chain read or an invocation
// through the usage. ok is false when the usage is not read so or the answer is undetermined,
// a function, or an object the probe itself made, which its rollback takes back.
func (ec *EvalContext) underDeclaredObject(base ast.Node, parts []ast.NameSegment, answer func(obj Value, owner *symbols.Symbol) (Value, error)) (Value, bool, error) {
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
	made := ec.ctx.ids.next
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
	val, err := answer(Value{Kind: ValInstance, Instance: inst.ID}, owner)
	if errors.Is(err, errUninitializedFeatureValue) || errors.Is(err, ErrNoSuchFeature) {
		return Value{}, false, nil
	}
	if err != nil {
		return Value{}, true, err
	}
	// What the object leaves open, and the calcs and objects made under the probe it
	// holds, go back with it; an object standing before the probe is kept as read.
	if val.Kind == ValUndetermined || val.Kind == ValFunction || madeUnderProbe(ec.ctx, val, made) {
		return Value{}, false, nil
	}
	return val, true, nil
}

// madeUnderProbe reports whether val holds an object whose identity was taken from
// mark on: one made under the probe begun at mark, which its rollback takes back.
func madeUnderProbe(ctx *Context, val Value, mark int64) bool {
	for _, inst := range instancesOf(ctx, standingElements(val)) {
		if inst.ID >= mark {
			return true
		}
	}
	return false
}

// readsDeclaredMembers reports whether a chain through sym is read from the object it would
// materialize: a valueless namespace-level usage of exactly one, not a type, calc or variation.
func (ec *EvalContext) readsDeclaredMembers(sym *symbols.Symbol) bool {
	if sym.OwnerScope == nil || !namespaceScope(sym.OwnerScope) {
		return false
	}
	if _, usage := sym.Decl.(*ast.Usage); !usage || declaresType(sym) || lower.IsCalcUsageSymbol(sym) {
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
