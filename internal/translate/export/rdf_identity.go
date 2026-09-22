package export

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// elementIdentity is one element's effective repository identity, in the
// encoder's terms: keyed by the qualified name the encoder computes.
type elementIdentity struct {
	id       string
	source   identity.Source
	declared bool
	scope    *identity.Scope
	// membership is the normative id of the owning membership, for a library element.
	membership string
}

// identityFacts is the identity side table of one document translated for the
// encoder: effective ids by qualified name, the annotation nodes consumed
// into identity rather than exported as model content, and each ProjectRef
// scope keyed by the qualified name of the namespace that declares it.
type identityFacts struct {
	byFQN map[string]elementIdentity
	// byNode keys each identity by its declaration node: an unnamed element's
	// symbol name and the encoder's positional name cannot join on FQN.
	byNode     map[ast.Node]elementIdentity
	consumed   map[ast.Node]bool
	provenance map[ast.Node]*identity.Scope
	// qualified reports a multi-scope document, whose scoped elements get
	// IRIs qualified by their scope so ids repeated across scopes stay apart.
	qualified bool
	// model and res read the identity of a library element the document
	// refers to, which the table over its own root does not hold.
	model *semantics.Model
	res   *resolve.Resolver
	// library memoizes those lookups: the qualified name a library symbol's
	// normative id was recorded under, "" where the norm fixes none.
	library map[*symbols.Symbol]string
}

// analyzeDocument indexes one parsed document over the standard library and resolves every
// name it writes; a library file, or a copy in its language rooted at its packages, takes the bundled one's place.
func analyzeDocument(file *source.SourceFile, root *ast.RootNamespace, library string) (*resolve.Resolver, *semantics.Model) {
	name := file.Name()
	idx := libs.NewModelIndex()
	digest := symbols.TextDigest(file.Bytes())
	if library == "" {
		if doc, _, ok := idx.LibraryDocumentByDigest(digest); ok && idx.DocumentKind(doc) == file.Kind() {
			library = doc
		}
	}
	if library == "" {
		library = documentLibrary(file, root)
	}
	tier := idx.DocumentLibraryTier(library)
	if tier.Library() {
		idx.RemoveDocument(library)
	}
	idx.AddDocumentWithKind(name, root, file.Kind())
	if tier.Library() {
		idx.MarkLibraryDocument(name, symbols.LibraryDocument{Tier: tier, Digest: digest})
	}
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	res.ResolveDocument(name, root)
	return res, model
}

// documentIdentity builds the identity side table for one parsed document,
// refusing an id that is not a constant string or is outside the id alphabet.
func documentIdentity(name string, res *resolve.Resolver, model *semantics.Model) (*identityFacts, error) {
	table := identity.Build(model, res, res.Index().DocumentRoot(name))

	facts := &identityFacts{
		byFQN:      map[string]elementIdentity{},
		byNode:     map[ast.Node]elementIdentity{},
		consumed:   map[ast.Node]bool{},
		provenance: map[ast.Node]*identity.Scope{},
		model:      model,
		res:        res,
		library:    map[*symbols.Symbol]string{},
	}
	scopeKeys := map[string]bool{}
	for _, sym := range table.Symbols() {
		info, ok := table.Info(sym)
		if !ok {
			continue
		}
		if info.Annotated {
			if err := exportableID(info); err != nil {
				return nil, err
			}
			for _, d := range info.Declarations {
				facts.consumed[d.Node] = true
			}
		}
		if info.Scope != nil {
			for _, d := range info.Scope.Declarations {
				facts.consumed[d.Node] = true
			}
			facts.provenance[info.Scope.Symbol.Decl] = info.Scope
			scopeKeys[info.Scope.Key()] = true
		}
		el := elementIdentity{
			id:         info.EffectiveID,
			source:     info.Source,
			declared:   info.Source == identity.SourceDeclared,
			scope:      info.Scope,
			membership: info.OwningMembershipID(),
		}
		facts.byFQN[info.FQN] = el
		facts.byNode[sym.Decl] = el
	}
	facts.qualified = len(scopeKeys) > 1
	return facts, nil
}

// exportableID rejects an ElementId annotation the graph cannot carry back.
func exportableID(info *identity.Info) error {
	if !info.Declared {
		return &UnsupportedError{
			What: fmt.Sprintf("the ElementId annotation on %s", info.FQN),
			Note: "its id is not a constant string, so the id it declares cannot be carried into the graph",
		}
	}
	if info.DeclaredID == "" {
		return &UnsupportedError{
			What: fmt.Sprintf("the ElementId annotation on %s", info.FQN),
			Note: "its id is empty, and an element IRI cannot be built from an empty id",
		}
	}
	for i := 0; i < len(info.DeclaredID); i++ {
		c := info.DeclaredID[i]
		if !idByte(c) && c != '_' {
			return &UnsupportedError{
				What: fmt.Sprintf("the ElementId annotation on %s", info.FQN),
				Note: fmt.Sprintf("its id %q holds a byte outside [A-Za-z0-9_-], which an element IRI cannot carry", info.DeclaredID),
			}
		}
	}
	return nil
}

// idByte reports whether c may appear in a declared element id.
func idByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-'
}

// libraryName reports whether fqn names a standard library element the norm
// fixes an id for, so a reference to it links rather than carries text.
func (f *identityFacts) libraryName(fqn string) bool {
	if _, declared := f.byFQN[fqn]; declared {
		return true
	}
	_, ok := identity.LibraryCatalog(f.res.Index()).ElementNamed(fqn)
	return ok
}

// libraryElement records the normative identity of a bundled library symbol on
// first sight and returns its qualified name; false where the norm fixes no id.
func (f *identityFacts) libraryElement(sym *symbols.Symbol) (string, bool) {
	if fqn, seen := f.library[sym]; seen {
		return fqn, fqn != ""
	}
	f.library[sym] = ""
	if _, declared := f.byNode[sym.Decl]; declared || !f.res.Index().Library(sym) {
		return "", false
	}
	info, ok := identity.Of(f.model, f.res, sym)
	if !ok || info.Source != identity.SourceNormative {
		return "", false
	}
	if _, taken := f.byFQN[info.FQN]; taken {
		return "", false
	}
	el := elementIdentity{id: info.EffectiveID, source: info.Source, membership: info.OwningMembershipID()}
	f.byFQN[info.FQN] = el
	f.byNode[sym.Decl] = el
	f.library[sym] = info.FQN
	return info.FQN, true
}

// subjectFor returns the subject IRI of the element with the given qualified
// name: its effective id, scope-qualified when the document is multi-scope.
func (f *identityFacts) subjectFor(fqn string) rdf.Term {
	return f.subjectOf(f.byFQN[fqn], fqn)
}

// subjectForNode returns the subject IRI of the declaration at node, joining
// on the node itself so an unnamed element keeps its annotated identity.
func (f *identityFacts) subjectForNode(node ast.Node, fqn string) rdf.Term {
	el, ok := f.byNode[node]
	if !ok {
		return f.subjectFor(fqn)
	}
	return f.subjectOf(el, fqn)
}

// subjectOf builds the IRI of one identity; a derived id is re-encoded from
// the encoder's name, which positions an unnamed element the table cannot.
func (f *identityFacts) subjectOf(el elementIdentity, fqn string) rdf.Term {
	id := el.id
	if el.source == identity.SourceDerived || id == "" {
		id = rdf.EncodeElementID(fqn)
	}
	if f.qualified && el.scope != nil {
		return rdf.ScopedElementIRIForID(rdf.ScopeQualifier(el.scope.Org, el.scope.ProjectID), id)
	}
	return rdf.ElementIRIForID(id)
}

// owningMembershipOf is the IRI of the membership owning the member declared at
// node: the id the norm fixes for a library element, else derived from the member's.
func (f *identityFacts) owningMembershipOf(node ast.Node, member rdf.Term) rdf.Term {
	if el, ok := f.byNode[node]; ok && el.membership != "" {
		return rdf.ElementIRIForID(el.membership)
	}
	return rdf.OwningMembershipIRIOf(member)
}

// normativeMembership reports whether the norm fixes an id for the membership
// owning the element declared at node.
func (f *identityFacts) normativeMembership(node ast.Node) bool {
	return f.byNode[node].membership != ""
}

// declaredIDAt reports whether the declaration's id came from an explicit
// ElementId annotation, which the graph must record: explicitness is not
// recoverable from the value. An annotation restating a normative id declares
// nothing, and the graph reads that id as the norm's without the record.
func (f *identityFacts) declaredIDAt(node ast.Node) bool {
	return f.byNode[node].declared
}

// skip reports whether node is an identity annotation consumed into the
// graph's identity rather than exported as model content.
func (f *identityFacts) skip(node ast.Node) bool {
	return f.consumed[node]
}
