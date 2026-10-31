package export

import (
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"slices"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// ElementIDs reports the elementId a conversion of a model writes for each of
// its elements, in the qualified id form, from the identity tables Convert mints
// them from (ToRDF's for one document, ModelToRDFWith's for several), so a
// library file's copy, a declared id and scope qualification come out as written.
type ElementIDs struct {
	// byNode keys each id by the declaration the conversion wrote it for: two
	// identity scopes may declare one qualified name, and only the node tells
	// their elements apart.
	byNode   map[ast.Node]string
	bySpan   map[spanKey]string // the same declaration in another parse of the document
	byName   map[string]string
	res      *resolve.Resolver
	encoders []*encoder
	// membershipByNode, membershipBySpan and membershipByName are the ids written
	// for the owning membership of each declared element, keyed as the element's.
	membershipByNode map[ast.Node]string
	membershipBySpan map[spanKey]string
	membershipByName map[string]string
	// conjugateByNode and conjugateBySpan are the ids written for the conjugated
	// port definition each port definition implies, keyed as the port definition's.
	conjugateByNode map[ast.Node]conjugateIDs
	conjugateBySpan map[spanKey]conjugateIDs
	// relationshipByNode and relationshipBySpan are the ids written for the relationship
	// elements a declaration's notation implies (writtenRelationships), by the node each
	// is written for: a head clause, a bound end, a dependency's prefix metadata.
	relationshipByNode map[ast.Node]string
	relationshipBySpan map[spanKey]string
}

// NewElementIDs records the elementId Convert writes for each element of the
// documents' model from the identity tables the ids are minted from — nothing is
// written — refusing what the declarations' identity claims refuse in Convert:
// ids colliding within a document (mintDeclarations) or across documents.
func NewElementIDs(documents []ModelDocument) (*ElementIDs, error) {
	res, encoders, err := modelEncodersOf(documents, IDQualifiedName)
	if err != nil {
		return nil, err
	}
	referenced := make([]bool, len(documents))
	for i, doc := range documents {
		referenced[i] = doc.Referenced
	}
	return identityIDsOf(res, encoders, referenced)
}

// identityIDsOf records each written declaration's id (its subject's local name)
// once the claims Convert makes for the documents (as modelToRDF does) succeed.
func identityIDsOf(res *resolve.Resolver, encoders []*encoder, referenced []bool) (*ElementIDs, error) {
	declaredIn := map[string]mintedSubject{}
	for i, e := range encoders {
		minted := e.declaredSubjects()
		if !referenced[i] {
			if err := e.mintDeclarations(); err != nil {
				return nil, fmt.Errorf("%s: %w", e.file.Name(), err)
			}
			minted = e.minted
		}
		if err := declaredOnce(declaredIn, minted); err != nil {
			return nil, err
		}
	}
	ids := &ElementIDs{byNode: map[ast.Node]string{}, bySpan: map[spanKey]string{}, byName: map[string]string{},
		membershipByNode: map[ast.Node]string{}, membershipBySpan: map[spanKey]string{}, membershipByName: map[string]string{},
		relationshipByNode: map[ast.Node]string{}, relationshipBySpan: map[spanKey]string{},
		conjugateByNode: map[ast.Node]conjugateIDs{}, conjugateBySpan: map[spanKey]conjugateIDs{},
		res: res, encoders: encoders}
	for _, e := range encoders {
		parents := e.parentDeclarations()
		for _, node := range e.writtenDeclarations() {
			fqn := e.fqn[node]
			subject := e.ids.subjectForNode(node, fqn)
			id := rdf.LocalName(subject.Value)
			ids.byNode[node] = id
			ids.bySpan[spanKey{e.file.Name(), node.Span()}] = id
			if fqn != "" {
				ids.byName[fqn] = id
			}
			if membership, ok := e.writtenOwningMembership(node, parents[node], fqn); ok {
				local := rdf.LocalName(membership.Value)
				ids.membershipByNode[node] = local
				ids.membershipBySpan[spanKey{e.file.Name(), node.Span()}] = local
				ids.membershipByName[fqn] = local
			}
			if def, ok := node.(*ast.Definition); ok && def.Kind == ast.DefPort {
				conjugate := e.ids.conjugateIDsOf(subject)
				ids.conjugateByNode[node] = conjugate
				ids.conjugateBySpan[spanKey{e.file.Name(), node.Span()}] = conjugate
			}
			if valued, _, _, ok := ast.ValuePartOf(node); ok {
				// The value expression a declaration's value part writes, and the
				// FeatureValue that owns it (featureValue, expressionOwnership).
				value := e.ids.mintedNode(rdf.ExpressionIRI(subject, pValue), subject, pValue)
				ids.byNode[valued] = rdf.LocalName(value.Value)
				ids.bySpan[spanKey{e.file.Name(), valued.Span()}] = ids.byNode[valued]
				membership := rdf.LocalName(e.ids.minted(rdf.OwningMembershipIRIOf(value), value, rdf.OwningMembershipSuffix).Value)
				ids.membershipByNode[valued] = membership
				ids.membershipBySpan[spanKey{e.file.Name(), valued.Span()}] = membership
			}
			for i, end := range writtenConnectorEnds(node) {
				if end == nil {
					continue
				}
				feature := e.ids.mintedNode(rdf.ExpressionIRI(subject, fmt.Sprintf("end%d", i)), subject, fmt.Sprintf("end%d", i))
				ids.byNode[end] = rdf.LocalName(feature.Value)
				ids.bySpan[spanKey{e.file.Name(), end.Span()}] = ids.byNode[end]
				// The head owns each end through a membership minted beside it (bindingEnds).
				membership := rdf.LocalName(e.ids.minted(rdf.OwningMembershipIRIOf(feature), feature, rdf.OwningMembershipSuffix).Value)
				ids.membershipByNode[end] = membership
				ids.membershipBySpan[spanKey{e.file.Name(), end.Span()}] = membership
			}
			for _, rel := range writtenSatisfySubjects(node) {
				// The subject parameter a `by` clause declares, and its membership (subjectParameter).
				usage := e.ids.minted(rdf.RelationshipIRI(subject, "_subject"), subject, "_subject")
				ids.byNode[rel] = rdf.LocalName(usage.Value)
				ids.bySpan[spanKey{e.file.Name(), rel.Span()}] = ids.byNode[rel]
				membership := rdf.LocalName(e.ids.minted(rdf.OwningMembershipIRIOf(usage), usage, rdf.OwningMembershipSuffix).Value)
				ids.membershipByNode[rel] = membership
				ids.membershipBySpan[spanKey{e.file.Name(), rel.Span()}] = membership
			}
			for key, id := range e.writtenRelationships(node, parents[node], subject, parents) {
				ids.relationshipByNode[key] = id
				ids.relationshipBySpan[spanKey{e.file.Name(), key.Span()}] = id
			}
		}
	}
	return ids, nil
}

// writtenRelationships are the ids of the relationship elements the writer makes for the
// declaration at node, subject, by the node each is written for: one per target of each
// head clause materializeRelationships emits (the clause; the type of a typing the head
// writes without a clause, the multiplicity a `multiplicity m subsets n` names; the usage
// itself for an object no clause states), the ReferenceSubsetting of a bound end or an `end` feature's
// `references` (the end, or the clause), and the Annotation of a dependency's prefix
// metadata (the metadata).
func (e *encoder) writtenRelationships(node, parent ast.Node, subject rdf.Term, parents map[ast.Node]ast.Node) map[ast.Node]string {
	out := map[ast.Node]string{}
	e.writtenEndFeatureReferences(node, subject, out)
	e.writtenDependencyAnnotation(node, parent, subject, out)
	e.writtenClauseRelationships(node, parent, subject, parents, out)
	e.writtenBoundEndReferences(node, subject, out)
	return out
}

// referenceSubsettingID is the id of the ReferenceSubsetting written under feature (referenceSubsetting).
func (e *encoder) referenceSubsettingID(feature rdf.Term) string {
	return rdf.LocalName(e.ids.mintedNode(rdf.ExpressionIRI(feature, "rs"), feature, "rs").Value)
}

// writtenEndFeatureReferences records the ReferenceSubsetting of each `references` clause
// of an `end` feature (endReferences), by the clause.
func (e *encoder) writtenEndFeatureReferences(node ast.Node, subject rdf.Term, out map[ast.Node]string) {
	usage, ok := node.(*ast.Usage)
	if !ok || !usage.IsEnd {
		return
	}
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelReferences && rel.Target != nil {
			out[rel] = e.referenceSubsettingID(subject)
		}
	}
}

// writtenDependencyAnnotation records the Annotation written for a dependency's prefix
// metadata, by the metadata.
func (e *encoder) writtenDependencyAnnotation(node, parent ast.Node, subject rdf.Term, out map[ast.Node]string) {
	if _, ok := node.(*ast.PrefixMetadata); !ok {
		return
	}
	if _, dependency := parent.(*ast.Dependency); dependency {
		out[node] = rdf.LocalName(e.ids.minted(rdf.RelationshipIRI(subject, AnnotationSuffix), subject, AnnotationSuffix).Value)
	}
}

// writtenClauseRelationships records the relationship written for each distinct target of the
// head's clauses (materializeRelationships), by the clause or by the target of an implicit one.
func (e *encoder) writtenClauseRelationships(node, parent ast.Node, subject rdf.Term, parents map[ast.Node]ast.Node, out map[ast.Node]string) {
	_, classifier := node.(*ast.Definition)
	referenceForm := e.referenceFormOf(node)
	clauses := e.writtenHeadClauses(node, parent, parents)
	chains := chainTargets(clauses)
	for _, kind := range materializedKinds {
		objects, at := e.clauseObjects(subject, clauses, kind, chains)
		for rel, i := range at {
			suffix := relationshipSuffix(kind, classifier, referenceForm, len(objects), i)
			key := ast.Node(rel)
			if rel.Span().Len == 0 {
				key = rel.Target
			}
			out[key] = rdf.LocalName(e.ids.minted(rdf.RelationshipIRI(subject, suffix), subject, suffix).Value)
		}
	}
}

// referenceFormOf reports whether a head's `subsets` are written as ReferenceSubsettings
// (referenceSubsettingForm): a satisfy, an end-form binding or a verified requirement.
func (e *encoder) referenceFormOf(node ast.Node) bool {
	usage, ok := node.(*ast.Usage)
	if !ok {
		return false
	}
	form, _ := e.endFormOf(usage)
	return usage.Kind == ast.UsageSatisfy || form != "" ||
		(usage.IsVerifiedRequirement() && !usage.DeclaresRequirement)
}

// writtenHeadClauses are headClauses plus the objects a usage states without a clause (encode):
// a metadata body member's implicit redefinition, a bare `variant x;`'s reference.
func (e *encoder) writtenHeadClauses(node, parent ast.Node, parents map[ast.Node]ast.Node) []*ast.Relationship {
	clauses := headClauses(node)
	usage, ok := node.(*ast.Usage)
	if !ok {
		return clauses
	}
	if e.implicitMetadataBodyTarget(inMetadataBody(usage, parent, parents), usage).Value != "" {
		clauses = append(append([]*ast.Relationship{}, clauses...), &ast.Relationship{Kind: ast.RelRedefines, Target: usage})
	}
	if e.variantReferenceTarget(usage).Value != "" {
		clauses = append(append([]*ast.Relationship{}, clauses...), &ast.Relationship{Kind: ast.RelReferences, Target: usage})
	}
	return clauses
}

// clauseObjects are the distinct objects the head's clauses of kind state, in clause order, and
// each clause's position among them: clauses repeating a target (`: T, T`) share one relationship.
func (e *encoder) clauseObjects(subject rdf.Term, clauses []*ast.Relationship, kind ast.RelationshipKind, chains map[*ast.Relationship]int) ([]string, map[*ast.Relationship]int) {
	var objects []string
	at := map[*ast.Relationship]int{}
	for _, rel := range clauses {
		if rel == nil || rel.Target == nil || rel.Kind != kind {
			continue
		}
		object := e.clauseObject(subject, rel, chains)
		i := slices.Index(objects, object)
		if i < 0 {
			i = len(objects)
			objects = append(objects, object)
		}
		at[rel] = i
	}
	return objects, at
}

// writtenBoundEndReferences records the ReferenceSubsetting of each end feature a head
// binds (bindingEnds), by the end and by the `references` clause naming its target.
func (e *encoder) writtenBoundEndReferences(node ast.Node, subject rdf.Term, out map[ast.Node]string) {
	for i, end := range writtenConnectorEnds(node) {
		if end == nil {
			continue
		}
		feature := e.ids.mintedNode(rdf.ExpressionIRI(subject, fmt.Sprintf("end%d", i)), subject, fmt.Sprintf("end%d", i))
		out[end] = e.referenceSubsettingID(feature)
		for _, rel := range end.Relationships {
			if rel != nil && rel.Kind == ast.RelReferences && rel.Target == end.AttachedTarget() {
				out[rel] = out[end]
			}
		}
	}
}

// clauseObject tells apart the objects a head's clauses state, as relationships writes them:
// a chain by the chain Feature minted for its clause, a name by its referent, else by text.
func (e *encoder) clauseObject(subject rdf.Term, rel *ast.Relationship, chains map[*ast.Relationship]int) string {
	if k, chained := chains[rel]; chained {
		return rdf.ExpressionIRI(subject, fmt.Sprintf("chain%d", k)).Value
	}
	if name, ok := rel.Target.(*ast.QualifiedName); ok && !qualifiedNameHasChain(name) {
		return e.reference(name).Value
	}
	return "expr:" + e.text(rel.Target)
}

// inMetadataBody reports whether usage is a member of a metadata usage's body (`@M { x = 1; }`,
// `metadata m : M { … }`), at any depth of usages, as encode marks metadataBodies.
func inMetadataBody(usage *ast.Usage, parent ast.Node, parents map[ast.Node]ast.Node) bool {
	switch p := parent.(type) {
	case *ast.PrefixMetadata:
		return true
	case *ast.Usage:
		return p.Kind == ast.UsageMetadata || inMetadataBody(p, parents[p], parents)
	}
	return false
}

// headClauses are the clauses of a declaration head the writer states as collapsed
// properties and materializes (relationships), as each declaration form passes them:
// an `end` feature's `references` go to endReferences instead, and a bare
// `require c;` is a `references` clause of its own (condition).
func headClauses(node ast.Node) []*ast.Relationship {
	switch n := node.(type) {
	case *ast.Definition:
		return n.Relationships
	case *ast.Usage:
		rels := nodeRelationships(n, "")
		if !n.IsEnd {
			return rels
		}
		kept := make([]*ast.Relationship, 0, len(rels))
		for _, rel := range rels {
			if rel == nil || rel.Kind != ast.RelReferences || rel.Target == nil {
				kept = append(kept, rel)
			}
		}
		return kept
	case *ast.PrefixMetadata:
		if n.Type == nil {
			return nil
		}
		return []*ast.Relationship{{Kind: ast.RelTyping, Target: n.Type}}
	case *ast.SubjectMember:
		if n.TypeRef == nil {
			return n.Relationships
		}
		typing := &ast.Relationship{Kind: ast.RelTyping, Target: n.TypeRef}
		return append([]*ast.Relationship{typing}, n.Relationships...)
	case *ast.AssumeMember:
		return conditionClauses(n.Relationships, n.Expression)
	case *ast.RequireMember:
		return conditionClauses(n.Relationships, n.Expression)
	case *ast.CrossFeatureMember:
		return n.Relationships
	case *ast.MultiplicityDecl:
		if n.Subsets == nil {
			return nil
		}
		return []*ast.Relationship{{Kind: ast.RelSubsets, Target: n.Subsets}}
	}
	return nil
}

// conditionClauses adds the `references` a bare condition states (`require c;`) to a
// condition member's written clauses, as condition writes it.
func conditionClauses(rels []*ast.Relationship, expr ast.Node) []*ast.Relationship {
	target := conditionReference(expr)
	if target == nil {
		return rels
	}
	return append(append([]*ast.Relationship{}, rels...), &ast.Relationship{Kind: ast.RelReferences, Target: target})
}

// writtenConnectorEnds are the ends of a head that binds them (`connect a to b`),
// by position, as bindingEnds writes them: nil where no end feature is written.
func writtenConnectorEnds(node ast.Node) []*ast.ConnectorEnd {
	usage, ok := node.(*ast.Usage)
	if !ok || !verbatimUsage(usage) || len(usage.ConnectorEnds) == 0 {
		return nil
	}
	ends := make([]*ast.ConnectorEnd, len(usage.ConnectorEnds))
	for i, end := range usage.ConnectorEnds {
		if end != nil && end.AttachedTarget() != nil {
			ends[i] = end
		}
	}
	return ends
}

// spanKey names a declaration by its document and span, across parses of it.
type spanKey struct {
	doc  string
	span source.Span
}

// OfDeclaration is the elementId written for the element declared at decl in
// document doc, named fqn: the node's own, its span's in a reparse, else Of(fqn).
func (ids *ElementIDs) OfDeclaration(doc string, decl ast.Node, fqn string) (string, bool) {
	if ids == nil {
		return "", false
	}
	if implied, ok := decl.(*symbols.ImpliedDecl); ok {
		conjugate, ok := ids.conjugateOf(doc, implied)
		return conjugate.element, ok
	}
	if decl != nil {
		if id, ok := ids.byNode[decl]; ok {
			return id, true
		}
		if id, ok := ids.bySpan[spanKey{doc, decl.Span()}]; ok {
			return id, true
		}
	}
	return ids.Of(fqn)
}

// OwningMembershipOfDeclaration is the elementId written for the owning membership
// of the element declared at decl in doc, named fqn; false for a root or an unknown element.
func (ids *ElementIDs) OwningMembershipOfDeclaration(doc string, decl ast.Node, fqn string) (string, bool) {
	if ids == nil {
		return "", false
	}
	if implied, ok := decl.(*symbols.ImpliedDecl); ok {
		conjugate, ok := ids.conjugateOf(doc, implied)
		return conjugate.membership, ok
	}
	if decl != nil {
		if id, ok := ids.membershipByNode[decl]; ok {
			return id, true
		}
		if id, ok := ids.membershipBySpan[spanKey{doc, decl.Span()}]; ok {
			return id, true
		}
	}
	if id, ok := ids.membershipByName[fqn]; ok {
		return id, true
	}
	// A standard library element the model does not declare: its normative membership,
	// or the one minted beside it, as owningMembershipOf writes a reference to it.
	if !strings.Contains(fqn, "::") || len(ids.encoders) == 0 || ids.res == nil || ids.res.Index() == nil {
		return "", false
	}
	facts := ids.encoders[0].ids
	for _, sym := range ids.res.Index().LookupQualified(fqn) {
		if libraryFQN, ok := facts.libraryElement(sym); ok {
			member := facts.subjectForNode(sym.Decl, libraryFQN)
			return rdf.LocalName(facts.owningMembershipOf(sym.Decl, member).Value), true
		}
	}
	return "", false
}

// RelationshipOf is the elementId written for the relationship element the notation
// implies at node in document doc (writtenRelationships); false where none is written.
func (ids *ElementIDs) RelationshipOf(doc string, node ast.Node) (string, bool) {
	if ids == nil || node == nil {
		return "", false
	}
	if implied, ok := node.(*symbols.ImpliedDecl); ok {
		conjugate, ok := ids.conjugateOf(doc, implied)
		return conjugate.conjugation, ok
	}
	if id, ok := ids.relationshipByNode[node]; ok {
		return id, true
	}
	id, ok := ids.relationshipBySpan[spanKey{doc, node.Span()}]
	return id, ok
}

// Of is the elementId written for the element named fqn: the model's own as
// the conversion wrote it, or, for a standard library element the model does
// not declare, the normative id every reference to it carries. False where the
// conversion writes none, as for a library element a model annotates with an
// id of its own, whose references are written by name.
func (ids *ElementIDs) Of(fqn string) (string, bool) {
	if ids == nil || fqn == "" {
		return "", false
	}
	if id, ok := ids.byName[fqn]; ok {
		return id, true
	}
	if len(ids.encoders) == 0 || ids.res == nil || ids.res.Index() == nil {
		return "", false
	}
	facts := ids.encoders[0].ids
	for _, sym := range ids.res.Index().LookupQualified(fqn) {
		if libraryFQN, ok := facts.libraryElement(sym); ok {
			return rdf.LocalName(facts.subjectForNode(sym.Decl, libraryFQN).Value), true
		}
	}
	return "", false
}

// ElementIDDocument is a document whose elementIds are read: its parse and the
// parse's errors; one given by its text alone is parsed for the conversion.
type ElementIDDocument struct {
	File       *source.SourceFile
	Root       *ast.RootNamespace
	ParseDiags []parser.Diagnostic
}

// ElementIDReader reads the elementId Convert writes for each element of a
// model, from one conversion of the model made on first use. A model Convert
// refuses has no ids: the refusal is kept and every read reports it.
type ElementIDReader struct {
	documents []ElementIDDocument
	once      sync.Once
	ids       *ElementIDs
	err       error
}

// NewElementIDReader reads the elementIds of the model documents make; a
// document without text is not written. Nothing is converted until Read.
func NewElementIDReader(documents []ElementIDDocument) *ElementIDReader {
	return &ElementIDReader{documents: documents}
}

// Read is the elementId written for the element declared at decl in document
// doc, named fqn (ElementIDs.OfDeclaration), or why the model has none.
func (r *ElementIDReader) Read(doc string, decl ast.Node, fqn string) (string, error) {
	if r == nil {
		return "", nil
	}
	r.once.Do(r.convert)
	if r.err != nil {
		return "", r.err
	}
	id, _ := r.ids.OfDeclaration(doc, decl, fqn)
	return id, nil
}

// ReadOwningMembership is the elementId of the owning membership of the element
// declared at decl in doc, named fqn; empty where the conversion writes none.
func (r *ElementIDReader) ReadOwningMembership(doc string, decl ast.Node, fqn string) (string, error) {
	if r == nil {
		return "", nil
	}
	r.once.Do(r.convert)
	if r.err != nil {
		return "", r.err
	}
	id, _ := r.ids.OwningMembershipOfDeclaration(doc, decl, fqn)
	return id, nil
}

// ReadRelationship is the elementId of the relationship element the notation implies
// at node in doc — a head clause's, a bound end's reference subsetting, a dependency's
// prefix-metadata annotation; empty where the conversion writes none.
func (r *ElementIDReader) ReadRelationship(doc string, node ast.Node) (string, error) {
	if r == nil {
		return "", nil
	}
	r.once.Do(r.convert)
	if r.err != nil {
		return "", r.err
	}
	id, _ := r.ids.RelationshipOf(doc, node)
	return id, nil
}

// convert mints the ids as Convert writes them: one document is parsed afresh
// from its text, as ToRDF does; several are taken over the trees read.
func (r *ElementIDReader) convert() {
	documents := make([]ModelDocument, 0, len(r.documents))
	for _, doc := range r.documents {
		if doc.File == nil {
			continue
		}
		root, diags := doc.Root, doc.ParseDiags
		if root == nil {
			p := parser.New(doc.File)
			root, diags = p.ParseFile(), p.Diagnostics
		}
		if syntax := parser.SyntaxErrorOf(doc.File.Name(), doc.File, diags); syntax != nil {
			r.err = syntax
			return
		}
		documents = append(documents, ModelDocument{File: doc.File, Root: root})
	}
	if len(documents) == 1 {
		given := documents[0].File
		// ToRDF reads a document in the language its name gives it; a name
		// giving none keeps the language the document was held under.
		kind := source.KindOf(given.Name())
		if kind == source.KindUnknown {
			kind = given.Kind()
		}
		file := source.NewWithKind(given.Name(), given.Bytes(), kind)
		p := parser.New(file)
		root := p.ParseFile()
		if syntax := parser.SyntaxErrorOf(file.Name(), file, p.Diagnostics); syntax != nil {
			r.err = syntax
			return
		}
		e, err := newEncoder(file, root, "", IDQualifiedName)
		if err != nil {
			r.err = err
			return
		}
		r.ids, r.err = identityIDsOf(e.res, []*encoder{e}, []bool{false})
		return
	}
	r.ids, r.err = NewElementIDs(documents)
}

// writtenSatisfySubjects are the `by` clauses of a satisfy usage the writer emits a
// subject parameter for (subjectParameters).
func writtenSatisfySubjects(node ast.Node) []*ast.Relationship {
	usage, ok := node.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageSatisfy {
		return nil
	}
	var out []*ast.Relationship
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelSubject && rel.Target != nil {
			out = append(out, rel)
		}
	}
	return out
}

// conjugateIDs are the ids written for the conjugated port definition a port
// definition implies: the conjugate, its owning membership and its PortConjugation
// (SysML 8.3.11.3), minted as rdf_normative.go's conjugatedPortDefinition writes them.
type conjugateIDs struct {
	element, membership, conjugation string
}

// conjugateIDsOf mints the conjugate's ids for the port definition written as subject.
func (f *identityFacts) conjugateIDsOf(subject rdf.Term) conjugateIDs {
	conjugated := f.minted(rdf.RelationshipIRI(subject, "_conjugated"), subject, "_conjugated")
	membership := f.minted(rdf.OwningMembershipIRIOf(conjugated), conjugated, rdf.OwningMembershipSuffix)
	conjugation := f.minted(rdf.RelationshipIRI(conjugated, "_pc"), conjugated, "_pc")
	return conjugateIDs{
		element:     rdf.LocalName(conjugated.Value),
		membership:  rdf.LocalName(membership.Value),
		conjugation: rdf.LocalName(conjugation.Value),
	}
}

// conjugateOf finds the ids of the conjugate implied for the port definition
// an ImpliedDecl is keyed to, by node in this parse or by span in another.
func (ids *ElementIDs) conjugateOf(doc string, implied *symbols.ImpliedDecl) (conjugateIDs, bool) {
	if implied == nil || implied.Form != symbols.ImpliedConjugatedPortDefinition || implied.Of == nil {
		return conjugateIDs{}, false
	}
	if c, ok := ids.conjugateByNode[implied.Of]; ok {
		return c, true
	}
	c, ok := ids.conjugateBySpan[spanKey{doc, implied.Of.Span()}]
	return c, ok
}
