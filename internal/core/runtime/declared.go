package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// DeclaredReader reads feature values as the model declares them for a concrete
// carrier, materializing each element once and starting no behavior.
type DeclaredReader struct {
	ctx     *Context
	objects map[*symbols.Symbol]*Instance
}

// NewDeclaredReader creates a reader over a fresh, behavior-free runtime context.
func NewDeclaredReader(model *semantics.Model, resolver *resolve.Resolver) *DeclaredReader {
	ctx := NewContext(NewModel(model, resolver), DefaultMaxSteps)
	ctx.declarative = true
	return &DeclaredReader{ctx: ctx, objects: make(map[*symbols.Symbol]*Instance)}
}

// Read evaluates the named feature of the element sym denotes, resolving every
// leaf through that element's redefinitions. An unbound leaf is a *NoValueError.
func (r *DeclaredReader) Read(sym *symbols.Symbol, name string) (Value, error) {
	inst, err := r.objectOf(sym)
	if err != nil {
		return Value{}, err
	}
	fv, err := inst.GetFeatureValue(r.ctx, name)
	if err != nil {
		return Value{}, err
	}
	val, err := r.ctx.readFeatureValue(fv, name)
	if err != nil {
		return Value{}, err
	}
	if r.ctx.HoldsNoValue(val) {
		return Value{}, &NoValueError{Feature: name, Symbol: fv.Feature.Symbol}
	}
	return val, nil
}

// objectOf materializes the element once, so all its features read from one object.
func (r *DeclaredReader) objectOf(sym *symbols.Symbol) (*Instance, error) {
	if sym == nil {
		return nil, fmt.Errorf("%w: no element to read", ErrUnresolvedReference)
	}
	if inst, ok := r.objects[sym]; ok {
		return inst, nil
	}
	mark := len(r.ctx.created)
	inst, err := r.ctx.materialize(sym, 0, nil, "")
	if err != nil {
		r.ctx.abandonInstancesSince(mark)
		return nil, err
	}
	if r.ctx.registersOccurrence(sym) {
		r.ctx.occurrences[sym] = inst.ID
	}
	r.objects[sym] = inst
	return inst, nil
}
