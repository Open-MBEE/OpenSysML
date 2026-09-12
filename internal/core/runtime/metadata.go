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
	if ec.ctx.model.semantics == nil {
		return Value{}, fmt.Errorf("%w: no model holds the metadata of %s",
			ErrTypeMismatch, ec.ctx.qualifiedSymbolName(sym))
	}
	annotations := ec.ctx.model.semantics.ElementMetadataOf(sym)
	// One access answers every annotation or none: what a failing one wrote, made
	// or started, here or in a behavior it woke, is undone with it.
	commit, rollback := ec.ctx.beginJournal()
	values := make([]Value, 0, len(annotations))
	for i, annotation := range annotations {
		val, err := ec.metadataInstance(metadataAnnotation{element: sym, index: i}, annotation)
		if err != nil {
			rollback()
			return Value{}, err
		}
		values = append(values, val)
	}
	// The annotations are followed by the element's own reflective metaobject
	// (KerML 1.0 §8.3.4.8.15).
	meta, err := ec.reflectiveMetaobject(sym)
	if err != nil {
		rollback()
		return Value{}, err
	}
	seq, err := ec.newSequence(append(values, meta))
	if err != nil {
		rollback()
		return Value{}, err
	}
	commit()
	return seq, nil
}

// metadataSubject is the element `ref.metadata` reads the metadata of. A name
// that resolves to no element, or to something that is a value rather than an
// element, is refused rather than answered with an empty sequence.
func (ec *EvalContext) metadataSubject(n *ast.MetadataAccessExpr) (*symbols.Symbol, error) {
	name := ast.QualifiedText(n.Ref)
	if name == "" {
		return nil, fmt.Errorf("%w: metadata access names no element", ErrTypeMismatch)
	}
	if ec.ctx == nil || ec.ctx.model.resolver == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnresolvedReference, name)
	}
	sym, ok := ec.ctx.model.resolver.ResolveQualified(ec.scope, n.Ref)
	if !ok || sym == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnresolvedReference, name)
	}
	if resolved, aliasOK := ec.ctx.model.resolver.ResolveAliasTarget(sym); aliasOK {
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

// metadataAnnotation names one annotation of one element: the element it
// annotates and its place among that element's annotations.
type metadataAnnotation struct {
	element *symbols.Symbol
	index   int
}

// metadataAnnotationDigest renders the annotation at that place as this context
// reads it: its metadata type and the text it states, so an annotation edited,
// reordered or retyped is not taken for the one an object was made for. The
// empty string says this context has no annotation there.
func (ctx *Context) metadataAnnotationDigest(element *symbols.Symbol, index int) string {
	if ctx.model.semantics == nil || element == nil {
		return ""
	}
	annotations := ctx.model.semantics.ElementMetadataOf(element)
	if index < 0 || index >= len(annotations) {
		return ""
	}
	annotation := annotations[index]
	if annotation.Node == nil {
		return ""
	}
	// An `about` annotation states itself away from the element it annotates, so
	// the text comes from the document stating it.
	return ctx.qualifiedSymbolName(annotation.Type) + " " + annotation.Doc + " " +
		ctx.textIn(annotation.Doc, annotation.Node.Span())
}

// metadataInstance is the object one annotation denotes, of its metadata type,
// with the features its body binds set to the values they are bound to and the
// remaining ones keeping the defaults the type declares. One annotation denotes
// one object, so a second read of it answers the object the first made.
func (ec *EvalContext) metadataInstance(key metadataAnnotation, annotation semantics.ElementMetadata) (Value, error) {
	ctx := ec.ctx
	if id, held := ctx.metadataObjects[key]; held {
		if _, live := ctx.instances[id]; live {
			return Value{Kind: ValInstance, Instance: id}, nil
		}
	}
	inst, err := ctx.materialize(annotation.Type, 0, nil, "")
	if err != nil {
		return Value{}, fmt.Errorf("metadata %s: %w", ctx.qualifiedSymbolName(annotation.Type), err)
	}
	if err := ec.bindMetadataFeatures(annotation.Type, inst, annotation.Bindings); err != nil {
		return Value{}, err
	}
	ctx.metadataObjects[key] = inst.ID
	return Value{Kind: ValInstance, Instance: inst.ID}, nil
}

// bindMetadataFeatures writes the values one body level binds to the object it
// annotates, descending into the object a nested declaration writes through.
func (ec *EvalContext) bindMetadataFeatures(typ *symbols.Symbol, inst *Instance, bindings []semantics.MetadataBinding) error {
	ctx := ec.ctx
	name := ctx.qualifiedSymbolName(typ)
	for _, binding := range bindings {
		if fv, held := inst.FeatureValues[binding.Feature]; !held || fv == nil {
			return fmt.Errorf("%w: metadata %s declares no feature %s",
				ErrTypeMismatch, name, binding.Feature)
		}
		if binding.Value != nil {
			// A value naming a sibling feature reads it off the object being bound.
			val, err := NewEvalContextIn(ctx, binding.Scope, inst).Eval(binding.Value)
			if err != nil {
				return fmt.Errorf("metadata %s: %s: %w", name, binding.Feature, err)
			}
			if err := inst.SetFeatureValue(ctx, binding.Feature, val); err != nil {
				return fmt.Errorf("metadata %s: %w", name, err)
			}
		}
		if len(binding.Nested) == 0 {
			continue
		}
		nested, err := ec.metadataNestedObject(typ, inst, binding.Feature)
		if err != nil {
			return err
		}
		if err := ec.bindMetadataFeatures(typ, nested, binding.Nested); err != nil {
			return err
		}
	}
	return nil
}

// metadataNestedObject is the object a nested body level writes through: the one
// the named feature holds, which a feature holding a value rather than an object
// has none of.
func (ec *EvalContext) metadataNestedObject(typ *symbols.Symbol, inst *Instance, feature string) (*Instance, error) {
	ctx := ec.ctx
	name := ctx.qualifiedSymbolName(typ)
	fv, err := inst.GetFeatureValue(ctx, feature)
	if err != nil {
		return nil, fmt.Errorf("metadata %s: %s: %w", name, feature, err)
	}
	id, isObject := fv.HeldValue().Object()
	if !isObject {
		return nil, fmt.Errorf("%w: metadata %s: feature %s holds no object to bind through",
			ErrTypeMismatch, name, feature)
	}
	nested, live := ctx.instances[id]
	if !live || nested == nil {
		return nil, fmt.Errorf("%w: metadata %s: feature %s holds no object to bind through",
			ErrTypeMismatch, name, feature)
	}
	return nested, nil
}
