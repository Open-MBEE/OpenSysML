package libs

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// formatVersion is the on-disk record format version. Bump it whenever the
// persisted shape changes; a change to what a record captures needs no bump,
// since the build ID in the cache key already invalidates records (see buildid.go).
const formatVersion = 27

// factRecord is the derived analysis persisted for one library symbol, named by
// the fully-qualified name it is declared under. It holds no declaration and no
// scope: a library document is parsed on every load path, so everything a
// declaration states is read from it rather than restored.
type factRecord struct {
	FQN       string
	Supers    []string        // FQNs of the semantic direct supertypes (see supersOf)
	Unit      *unitFacts      // for measurement units: their reduction to base units
	Dimension *dimensionFacts // for measurement units: the dimension they measure in
	Abstract  bool            // the declaration is abstract (KerML 7.3.2.2)
}

// unitFacts is the gob-encodable projection of a measurement unit reduced to
// base units.
type unitFacts struct {
	ScaleNum    float64
	ScaleDen    float64
	Factors     []unitFactor
	Irreducible bool // a unit whose reduction its declaration does not yield
}

// unitFactor is one base unit of a reduced measurement unit and its exponent.
type unitFactor struct {
	FQN      string
	Exponent float64
}

// dimensionFacts is the gob-encodable projection of a quantity dimension: the
// base quantities it is a product of powers of.
type dimensionFacts struct {
	Factors []dimensionFactor
}

// dimensionFactor is one base quantity of a dimension and its exponent.
type dimensionFactor struct {
	FQN      string
	Exponent float64
}

// IndexRecord is the derived analysis persisted for one library document.
type IndexRecord struct {
	Name  string
	Facts []factRecord
}

// recordFromIndex derives the facts worth persisting for every symbol the named
// document contributed to idx, and reports whether every specialization target it
// declares resolved. Returns nil if the document is unknown.
// Targets are resolved through r, so a record holds the fully-qualified name of
// each supertype rather than text that only means something in its own file.
// The model is shared across the records of one library, so the whole-index work
// it memoizes is done once rather than once per file.
func recordFromIndex(name string, idx *symbols.Index, model *semantics.Model) (*IndexRecord, bool) {
	root := idx.DocumentRoot(name)
	if root == nil {
		return nil, false
	}
	rec := &IndexRecord{Name: name}
	complete := collectScope(root, "", rec, model, idx)
	return rec, complete
}

// collectScope walks scope's members (and child scopes) appending the facts of
// each. prefix is the fully-qualified name of scope's owner ("" at root).
// Reports whether every specialization target it met resolved.
func collectScope(scope *symbols.Scope, prefix string, rec *IndexRecord, model *semantics.Model, idx *symbols.Index) bool {
	complete := true
	for _, sym := range scope.Members() {
		fqn := sym.Name
		if prefix != "" {
			fqn = prefix + "::" + sym.Name
		}
		supers, resolved := supersOf(sym, idx, model)
		complete = complete && resolved
		facts := factRecord{
			FQN:       fqn,
			Supers:    supers,
			Unit:      unitFactsOf(sym, model, idx),
			Dimension: dimensionFactsOf(sym, model, idx),
			Abstract:  symbols.IsAbstract(sym),
		}
		if !facts.isEmpty() {
			rec.Facts = append(rec.Facts, facts)
		}
		if sym.Scope != nil {
			complete = collectScope(sym.Scope, fqn, rec, model, idx) && complete
		}
	}
	return complete
}

// isEmpty reports whether a record memoizes nothing, in which case persisting it
// would only cost the space: every fact of it derives from the declaration.
func (f factRecord) isEmpty() bool {
	return len(f.Supers) == 0 && f.Unit == nil && f.Dimension == nil && !f.Abstract
}

// unitFactsOf reduces a measurement unit to base units, the dominant cost of a
// cold library load. A unit whose reduction its declaration does not yield is
// recorded as irreducible, which is not the same as not being a unit.
func unitFactsOf(sym *symbols.Symbol, model *semantics.Model, idx *symbols.Index) *unitFacts {
	if !model.IsMeasurementUnit(sym) {
		return nil
	}
	term, err := model.UnitTermOf(sym)
	if err != nil {
		return &unitFacts{Irreducible: true}
	}
	facts := &unitFacts{ScaleNum: term.Scale.Num, ScaleDen: term.Scale.Den}
	for _, f := range term.Factors {
		baseFQN := idx.GetFQN(f.Unit)
		if baseFQN == "" {
			return &unitFacts{Irreducible: true}
		}
		facts.Factors = append(facts.Factors, unitFactor{FQN: baseFQN, Exponent: f.Exponent})
	}
	return facts
}

// dimensionFactsOf records the dimension a measurement unit measures in, whose
// derivation reads the bound values of the power factors it is declared by.
func dimensionFactsOf(sym *symbols.Symbol, model *semantics.Model, idx *symbols.Index) *dimensionFacts {
	if !model.IsMeasurementUnit(sym) {
		return nil
	}
	derived := model.DimensionFactsOf(sym, idx)
	if derived == nil {
		return nil
	}
	out := &dimensionFacts{}
	for _, f := range derived.Factors {
		out.Factors = append(out.Factors, dimensionFactor{FQN: f.FQN, Exponent: f.Exponent})
	}
	return out
}

// supersOf records the semantic direct-supertype edges of a Definition or Usage,
// while checking that every declared generalization target resolves. Nothing is
// recorded for a symbol with an edge that has no qualified name to restore it by,
// or whose edges are still provisional: those are derived on every load instead.
func supersOf(sym *symbols.Symbol, idx *symbols.Index, model *semantics.Model) ([]string, bool) {
	var rels []*ast.Relationship
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		rels = d.Relationships
	case *ast.Usage:
		rels = d.Relationships
	default:
		return nil, true
	}
	complete := true
	for _, rel := range rels {
		if !semantics.GeneralizationKind(rel.Kind) {
			continue
		}
		if ast.AsQualifiedName(rel.Target) == nil {
			complete = false
			continue
		}
		super := model.RelationshipTarget(sym, rel)
		if super == nil {
			complete = false
			continue
		}
		if super == sym {
			continue
		}
		if idx.GetFQN(super) == "" {
			complete = false
		}
	}
	var out []string
	seen := map[string]bool{}
	derived := model.DirectSupertypes(sym)
	if model.SupertypesProvisional(sym) {
		return nil, false
	}
	for _, super := range derived {
		superFQN := idx.GetFQN(super)
		if superFQN == "" {
			return nil, false
		}
		// A nameless target — an unnamed result parameter — has no name to
		// restore it by, so the whole edge set is derived on load.
		if idx.Declaring(superFQN) != super {
			return nil, complete
		}
		if seen[superFQN] {
			continue
		}
		seen[superFQN] = true
		out = append(out, superFQN)
	}
	return out, complete
}

// libraryFacts projects a persisted record onto the facts installed on an index,
// keyed by fully-qualified name.
func libraryFacts(rec *IndexRecord) map[string]*symbols.LibraryFacts {
	out := make(map[string]*symbols.LibraryFacts, len(rec.Facts))
	for _, f := range rec.Facts {
		out[f.FQN] = &symbols.LibraryFacts{
			Supers:    f.Supers,
			Unit:      unitFactsEntry(f.Unit),
			Dimension: dimensionFactsEntry(f.Dimension),
			Abstract:  f.Abstract,
		}
	}
	return out
}

// unitFactsEntry projects a persisted unit reduction onto its index form.
func unitFactsEntry(facts *unitFacts) *symbols.UnitFacts {
	if facts == nil {
		return nil
	}
	out := &symbols.UnitFacts{ScaleNum: facts.ScaleNum, ScaleDen: facts.ScaleDen, Irreducible: facts.Irreducible}
	for _, f := range facts.Factors {
		out.Factors = append(out.Factors, symbols.UnitFactorFacts{FQN: f.FQN, Exponent: f.Exponent})
	}
	return out
}

// dimensionFactsEntry projects a persisted dimension onto its index form.
func dimensionFactsEntry(facts *dimensionFacts) *symbols.DimensionFacts {
	if facts == nil {
		return nil
	}
	out := &symbols.DimensionFacts{}
	for _, f := range facts.Factors {
		out.Factors = append(out.Factors, symbols.DimensionFactorFacts{FQN: f.FQN, Exponent: f.Exponent})
	}
	return out
}
