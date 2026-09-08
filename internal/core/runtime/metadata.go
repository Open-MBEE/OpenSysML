package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// evalMetadataAccess evaluates `ref.metadata` (KerML 8.4.4.10 MetadataAccessExpression):
// the metadata annotating the element ref names, as a sequence of objects of the
// annotating metadata types, in the order the annotations are stated. An element
// no metadata annotates reads as the empty sequence.
func (ec *EvalContext) evalMetadataAccess(n *ast.MetadataAccessExpr) (Value, error) {
	sym, err := ec.metadataSubject(n)
	if err != nil {
		return Value{}, err
	}
	if ec.ctx.model == nil {
		return Value{}, fmt.Errorf("%w: no model holds the metadata of %s",
			ErrTypeMismatch, ec.ctx.qualifiedSymbolName(sym))
	}
	annotations := ec.ctx.model.ElementMetadataOf(sym)
	values := make([]Value, 0, len(annotations))
	for _, annotation := range annotations {
		val, err := ec.metadataInstance(annotation)
		if err != nil {
			return Value{}, err
		}
		values = append(values, val)
	}
	return ec.newSequence(values)
}

// metadataSubject is the element `ref.metadata` reads the metadata of. A name
// that resolves to no element, or to something that is a value rather than an
// element, is refused rather than answered with an empty sequence.
func (ec *EvalContext) metadataSubject(n *ast.MetadataAccessExpr) (*symbols.Symbol, error) {
	name := ast.QualifiedText(n.Ref)
	if name == "" {
		return nil, fmt.Errorf("%w: metadata access names no element", ErrTypeMismatch)
	}
	if ec.ctx == nil || ec.ctx.resolver == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnresolvedReference, name)
	}
	sym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, n.Ref)
	if !ok || sym == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnresolvedReference, name)
	}
	if resolved, aliasOK := ec.ctx.resolver.ResolveAliasTarget(sym); aliasOK {
		sym = resolved
	}
	if sym.Decl == nil {
		return nil, fmt.Errorf("%w: metadata access requires an element, but %s declares none",
			ErrTypeMismatch, name)
	}
	return sym, nil
}

// metadataOfAValue reports `.metadata` read from a value rather than an element:
// only an element is annotated, so a scalar has no metadata to answer with.
func metadataOfAValue(value Value, parts []ast.NameSegment) error {
	if len(parts) == 0 || parts[0].Text != "metadata" {
		return nil
	}
	return fmt.Errorf("%w: metadata access requires an element, but %s is a value",
		ErrTypeMismatch, describeValue(value))
}

// metadataInstance materializes one annotation as an object of its metadata
// type, with the features its body binds set to the values they are bound to;
// the remaining features keep the defaults the type declares.
func (ec *EvalContext) metadataInstance(annotation semantics.ElementMetadata) (Value, error) {
	ctx := ec.ctx
	mark := len(ctx.created)
	inst, err := ctx.materialize(annotation.Type, 0, nil, "")
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return Value{}, fmt.Errorf("metadata %s: %w", ctx.qualifiedSymbolName(annotation.Type), err)
	}
	body := NewEvalContext(ctx, annotation.Scope)
	for _, binding := range annotation.Bindings {
		if fv, held := inst.FeatureValues[binding.Feature]; !held || fv == nil {
			ctx.abandonInstancesSince(mark)
			return Value{}, fmt.Errorf("%w: metadata %s declares no feature %s",
				ErrTypeMismatch, ctx.qualifiedSymbolName(annotation.Type), binding.Feature)
		}
		val, err := body.Eval(binding.Value)
		if err != nil {
			ctx.abandonInstancesSince(mark)
			return Value{}, fmt.Errorf("metadata %s: %s: %w",
				ctx.qualifiedSymbolName(annotation.Type), binding.Feature, err)
		}
		if err := inst.SetFeatureValue(ctx, binding.Feature, val); err != nil {
			ctx.abandonInstancesSince(mark)
			return Value{}, fmt.Errorf("metadata %s: %w", ctx.qualifiedSymbolName(annotation.Type), err)
		}
	}
	return Value{Kind: ValInstance, Instance: inst.ID}, nil
}
