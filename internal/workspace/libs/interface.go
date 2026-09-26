package libs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// interfaceFormatVersion is the on-disk format version of an interface record.
// Bump it whenever InterfaceRecord, symbols.DocumentRecord or
// symbols.LibraryFacts changes shape or meaning.
const interfaceFormatVersion = 1

// ErrUnrecordable reports a document whose interface cannot be written without
// its tree: a fact a reader needs has no name to restore it by. The document is
// held loaded instead.
var ErrUnrecordable = errors.New("libs: document interface cannot be recorded")

// InterfaceRecord is a user document held as its interface: the scope tree
// another document can observe through the language, with facts in place of
// declarations, and the diagnostics its analysis reported. See
// docs/internals/interface-records.md.
type InterfaceRecord struct {
	Name        string
	Kind        source.Kind
	Scope       *symbols.DocumentRecord
	Diagnostics []diag.Diagnostic
}

// InterfaceKey derives the cache key of a document's interface record from its
// content, the digest of the library it was analyzed against, the conformance
// mode, the build and the record format version: a record written under any
// other is never found.
func (c *Cache) InterfaceKey(content []byte, libraryDigest string, mode diag.ConformanceMode) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]) + "-l" + libraryDigest + "-c" + strconv.Itoa(int(mode)) +
		"-b" + buildID() + "-i" + strconv.Itoa(interfaceFormatVersion)
}

// LoadInterface returns the interface record stored under key, or (nil, false)
// on any miss.
func (c *Cache) LoadInterface(key string) (*InterfaceRecord, bool) {
	var rec InterfaceRecord
	if !c.load(key, &rec) || rec.Scope == nil {
		return nil, false
	}
	return &rec, true
}

// StoreInterface writes rec under key, atomically (see Store).
func (c *Cache) StoreInterface(key string, rec *InterfaceRecord) error {
	return c.store(key, rec)
}

// SourceDigest is the digest of a library source's files, the library half of
// an interface record's key.
func SourceDigest(src Source) string {
	return NewLoader(src, nil).setDigest()
}

// WriteInterface writes the interface record of the named document from the
// analysis that just ran over it: its scope tree in idx, the resolver and model
// that analysis read through, the relationships the workspace-wide audits
// gathered from its body, and the diagnostics it reported. It fails with
// ErrUnrecordable when a fact another document can observe has no
// fully-qualified name to restore it by.
func WriteInterface(name string, kind source.Kind, idx *symbols.Index, r *resolve.Resolver,
	model *semantics.Model, gathered *symbols.GatheredRelationships, diags []diag.Diagnostic) (*InterfaceRecord, error) {
	root := idx.DocumentRoot(name)
	if root == nil {
		return nil, fmt.Errorf("%w: %s is not indexed", ErrUnrecordable, name)
	}
	w := &interfaceWriter{idx: idx, r: r, model: model}
	scope, err := symbols.RecordScope(root, keepInInterface, w.facts)
	if err != nil {
		return nil, err
	}
	if w.err != nil {
		return nil, w.err
	}
	scope.Gathered = gathered
	return &InterfaceRecord{Name: name, Kind: kind, Scope: scope, Diagnostics: diags}, nil
}

// keepInInterface says whether a member of a loaded scope belongs in the
// document's interface.
func keepInInterface(sym *symbols.Symbol) bool { return sym != nil }

type interfaceWriter struct {
	idx   *symbols.Index
	r     *resolve.Resolver
	model *semantics.Model
	err   error
}

// fail notes the first fact that could not be recorded.
func (w *interfaceWriter) fail(sym *symbols.Symbol, what string) {
	if w.err == nil {
		w.err = fmt.Errorf("%w: %s of %s", ErrUnrecordable, what, symbols.FQNOf(sym))
	}
}

// ref is the reference a fact restores sym by, failing when none reaches it.
func (w *interfaceWriter) ref(of *symbols.Symbol, sym *symbols.Symbol, what string) symbols.ElementRef {
	ref, ok := w.idx.RefTo(sym)
	if !ok {
		w.fail(of, fmt.Sprintf("%s (%s %q at %s:%d)", what, sym.Kind, sym.Name, sym.DocName, sym.DeclSpan.Offset))
		return symbols.ElementRef{}
	}
	return ref
}

// refs is ref over several targets.
func (w *interfaceWriter) refs(of *symbols.Symbol, syms []*symbols.Symbol, what string) []symbols.ElementRef {
	out := make([]symbols.ElementRef, 0, len(syms))
	for _, sym := range syms {
		if ref := w.ref(of, sym, what); !ref.IsZero() {
			out = append(out, ref)
		}
	}
	return out
}

// facts states what sym's declaration yields, as its record carries it.
func (w *interfaceWriter) facts(sym *symbols.Symbol) symbols.LibraryFacts {
	m := w.model
	facts := symbols.LibraryFacts{
		Supers:    w.refs(sym, m.DirectSupertypes(sym), "supertype"),
		Unit:      unitFactsEntry(unitFactsOf(sym, m, w.idx)),
		Dimension: dimensionFactsEntry(dimensionFactsOf(sym, m, w.idx)),
		Abstract:  symbols.IsAbstract(sym),
		Redefines: w.refs(sym, m.RedefinedFeatures(sym), "redefined feature"),
		About:     w.refs(sym, m.AnnotatedElementsOf(sym), "annotated element"),
	}
	if facts.Supers == nil {
		facts.Supers = []symbols.ElementRef{}
	}
	if m.SupertypesProvisional(sym) {
		w.fail(sym, "provisional supertypes")
	}
	if ref := m.ReferencedFeature(sym); ref != nil {
		facts.References = w.ref(sym, ref, "referenced feature")
	}
	if sym.Kind == symbols.SymbolAlias {
		if target, ok := w.r.ResolveAliasTarget(sym); ok && target != nil {
			facts.Alias = w.ref(sym, target, "alias target")
		}
	}
	facts.Multiplicity = m.MultiplicityFactsOf(sym)
	facts.Annotations = m.DeclaredAnnotationFactsOf(sym)
	for _, a := range facts.Annotations {
		for _, v := range a.Values {
			if v.Value.Quantity != nil {
				w.fail(sym, "quantity-valued annotation")
			}
		}
	}
	facts.Direction, facts.Modifiers = declaredTraits(sym.Decl)
	facts.Modifiers |= w.r.DeclarationTraits(sym)
	facts.Node = symbols.NodeKindOf(sym.Decl)
	facts.Keyword = sym.Keyword()
	facts.UsageKind, _ = sym.UsageKind()
	facts.DefKind, _ = sym.DefinitionKind()
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil {
			continue
		}
		rf := symbols.RelationshipFacts{Kind: rel.Kind}
		if target := m.RelationshipTarget(sym, rel); target != nil {
			rf.Target = w.ref(sym, target, rel.Kind.String()+" target")
		}
		facts.Relationships = append(facts.Relationships, rf)
	}
	if semantics.AcceptPayload(sym) {
		facts.Modifiers |= symbols.ModAcceptPayload
	}
	if base, bound, unconditional := m.BaseTypeFacts(sym); bound {
		if !unconditional {
			w.fail(sym, "conditional baseType binding")
		}
		facts.Modifiers |= symbols.ModBindsBaseType
		if base != nil {
			facts.BaseType = w.ref(sym, base, "baseType")
		}
	}
	return facts
}

// declaredTraits reads the direction and boolean modifiers a declaration states.
func declaredTraits(decl ast.Node) (ast.FeatureDirection, symbols.Modifiers) {
	var mods symbols.Modifiers
	set := func(on bool, mod symbols.Modifiers) {
		if on {
			mods |= mod
		}
	}
	switch d := decl.(type) {
	case *ast.Definition:
		set(d.IsVariation, symbols.ModVariation)
		set(d.IsIndividual, symbols.ModIndividual)
		set(d.IsAll, symbols.ModAll)
		set(d.IsConstant, symbols.ModConstant)
		set(d.IsEvent, symbols.ModEvent)
		set(d.IsParallel, symbols.ModParallel)
		return ast.DirNone, mods
	case *ast.Usage:
		set(d.IsEnd, symbols.ModEnd)
		set(d.IsDerived, symbols.ModDerived)
		set(d.ValueIsInitial, symbols.ModInitial)
		set(d.IsReference, symbols.ModReference)
		set(d.IsComposite, symbols.ModComposite)
		set(d.IsPortion, symbols.ModPortion)
		set(d.IsVariation, symbols.ModVariation)
		set(d.IsVariant, symbols.ModVariant)
		set(d.IsIndividual, symbols.ModIndividual)
		set(d.IsResult, symbols.ModResult)
		set(d.IsOrdered, symbols.ModOrdered)
		set(d.IsNonunique, symbols.ModNonunique)
		set(d.IsConstant, symbols.ModConstant)
		set(d.IsVariable, symbols.ModVariable)
		set(d.IsParallel, symbols.ModParallel)
		set(d.ValueIsDefault, symbols.ModDefault)
		set(d.IsNegated, symbols.ModNegated)
		set(d.DeclaresRequirement, symbols.ModDeclaresRequirement)
		set(d.IsAll, symbols.ModAll)
		set(d.IsChain, symbols.ModChain)
		set(d.IsEvent, symbols.ModEvent)
		return d.Direction, mods
	case *ast.CrossFeatureMember:
		set(true, symbols.ModEnd)
		return ast.DirNone, mods
	}
	return ast.DirNone, mods
}
