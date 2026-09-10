package export

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Property names in the SysML vocabulary.
const (
	pDeclaredName      = "declaredName"
	pDeclaredShortName = "declaredShortName"
	pQualifiedName     = "qualifiedName"
	pElementID         = "elementId"
	pOwningNamespace   = "owningNamespace"
	pVisibility        = "visibility"
	// The ownership properties of the abstract syntax: an element's owner and
	// the OwningMembership between them, written from both ends.
	pOwner                     = "owner"
	pOwningRelationship        = "owningRelationship"
	pOwningMembership          = "owningMembership"
	pOwnedRelationship         = "ownedRelationship"
	pOwnedMembership           = "ownedMembership"
	pOwnedMember               = "ownedMember"
	pMemberElement             = "memberElement"
	pOwnedMemberElement        = "ownedMemberElement"
	pOwnedRelatedElement       = "ownedRelatedElement"
	pOwningRelatedElement      = "owningRelatedElement"
	pMembershipOwningNamespace = "membershipOwningNamespace"
	pOwnedMemberFeature        = "ownedMemberFeature"
	pOwningType                = "owningType"
	pOwnedFeature              = "ownedFeature"
	pOwnedFeatureMembership    = "ownedFeatureMembership"
	pVariant                   = "variant"
	pVariantMembership         = "variantMembership"
	pOwnedVariantUsage         = "ownedVariantUsage"
	pOwnedImport               = "ownedImport"
	pImportOwningNamespace     = "importOwningNamespace"
	pDirection                 = "direction"
	pLowerBound                = "lowerBound"
	pUpperBound                = "upperBound"
	pValue                     = "value"
	pIsDefault                 = "isDefault"
	pIsInitial                 = "isInitial"
	pImportedNamespace         = "importedNamespace"
	pAliasFor                  = "aliasedElement"
	pClient                    = "client"
	pSupplier                  = "supplier"
	pBody                      = "body"
	pLanguage                  = "language"
	pLocale                    = "locale"
	pAnnotatedElement          = "annotatedElement"
	pIsImportAll               = "isImportAll"
	pSourceFeature             = "sourceFeature"
	pTargetFeature             = "targetFeature"
	pPortionKind               = "portionKind"
)

// Property names in the OpenSysML extension namespace: declaration order,
// body presence, and the source text of the constructs whose head this
// mapping keeps verbatim (see the package doc).
const (
	xMemberIndex     = "memberIndex"
	xHasBody         = "hasBody"
	xSourceText      = "sourceText"
	xSourceTail      = "sourceTail"
	xSourceLanguage  = "sourceLanguage"
	xFilter          = "filter"
	xNamespaceImport = "isNamespaceImport"
	xRecursive       = "isRecursive"
	xExpose          = "isExpose"
	xDeclaredKeyword = "declaredKeyword"
	xDeclaredPrefix  = "declaredPrefix"
	xImplicitKind    = "isKindImplicit"
	xCondition       = "condition"
	xRelatedFeature  = "relatedFeature"
	xEndIndex        = "endIndex"
	xEndRole         = "endRole"
	xEndName         = "endName"
	// The ReferencesKeyword a named end spells, when it is not `::>`.
	xEndReferencesKeyword = "endReferencesKeyword"
	xEndForm              = "endForm"
	xEndVerb              = "endVerb"
	xSourceMember         = "sourceMember"
	xTargetMember         = "targetMember"
	// The identity properties: whether an element's id came from an explicit
	// ElementId annotation, and the ProjectRef provenance of a scope root.
	xDeclaredID = "declaredId"
	xProjectID  = "projectId"
	xBranch     = "branch"
	xOrg        = "org"
)

// The notations a head that binds ends writes its ends in, stated as
// sysx:endForm. See docs/reference/rdf-mapping.md § End-binding heads.
const (
	formTo        = "to"        // connect a to b
	formNary      = "nary"      // connect (a, b, c)
	formEquals    = "equals"    // bind a = b
	formFirstThen = "firstThen" // succession first a then b
	formFromTo    = "fromTo"    // flow of P from a to b
	formFlowTo    = "flowTo"    // flow a to b
	formThen      = "then"      // then b, whose source end is the member before it
	formSatisfy   = "satisfy"   // satisfy R by v, whose requirement is written bare
)

// The two spellings of ReferencesKeyword between a connector end's name and the
// feature it reference-subsets (KerML.xtext:856).
const (
	referencesSymbol = "::>"
	referencesWord   = "references"
)

// dtExpression is the datatype of a relationship target that is not a name but
// an expression, carried as the text it was written as rather than as a name.
const dtExpression = "Expression"

// The metaclasses of the membership elements ownership is materialized as: a
// type owns a feature through a FeatureMembership, a variation its variants
// through a VariantMembership, and every other namespace member is owned
// through an OwningMembership. All three are concrete.
const (
	mOwningMembership  = "OwningMembership"
	mFeatureMembership = "FeatureMembership"
	mVariantMembership = "VariantMembership"
	// The membership a body owns its result expression through, which states
	// the expression as sysml:ownedResultExpression.
	mResultExpressionMembership = "ResultExpressionMembership"
	pOwnedResultExpression      = "ownedResultExpression"
	mDocumentation              = "Documentation"
)

// Metaclass names for the constructs that have no SysML metaclass of their own
// in this mapping.
const (
	mAlias        = "Alias"
	mFilter       = "FilterMember"
	mMultiplicity = "MultiplicityDeclaration"
	// The members that state a condition rather than declaring a feature: the
	// conditions of a constraint body and a requirement's assumptions and
	// required conditions.
	mConstraint = "ConstraintMember"
	mAssume     = "AssumeMember"
	mRequire    = "RequireMember"
)

// boolProperty pairs an RDF property name with the AST flag it mirrors. Only
// true values are written, so an absent property reads as false.
type boolProperty struct {
	name  string
	value bool
}

// UnsupportedError reports a construct the conversion cannot represent. It
// names the element so the user can find it, rather than converting a model
// that silently lost part of itself.
type UnsupportedError struct {
	What string
	Note string
}

func (e *UnsupportedError) Error() string {
	if e.Note == "" {
		return fmt.Sprintf("cannot convert %s", e.What)
	}
	return fmt.Sprintf("cannot convert %s: %s", e.What, e.Note)
}

// ToRDF converts a parsed document into an RDF graph. file must be the source
// the tree was parsed from: every element and expression carries the notation
// it was written as alongside its structural triples, so the conversion needs
// the bytes as well as the tree.
func ToRDF(file *source.SourceFile, root *ast.RootNamespace) (*rdf.Graph, error) {
	e, err := encodeDocument(file, root)
	if err != nil {
		return nil, err
	}
	return e.graph, nil
}

// encodeDocument converts a parsed document, returning the encoder that holds
// the graph and where in file each element was written.
func encodeDocument(file *source.SourceFile, root *ast.RootNamespace) (*encoder, error) {
	if file == nil || root == nil {
		return nil, &UnsupportedError{What: "an empty document", Note: "nothing to convert"}
	}
	e, err := newEncoder(file, root)
	if err != nil {
		return nil, err
	}
	e.src = newAuthoredSource(file)
	if err := e.encode(root.Members, "", rdf.Term{}); err != nil {
		return nil, err
	}
	if e.idErr != nil {
		return nil, e.idErr
	}
	e.sourceText()
	if err := rdf.AnnotateCollections(e.graph); err != nil {
		return nil, err
	}
	return e, nil
}

// languageName is the name a document's grammar is recorded under on its roots,
// so the text is read back in the grammar it was written in. A file with no
// model extension is read as neither grammar exactly, so none is recorded.
func languageName(kind source.Kind) string {
	switch kind {
	case source.KindKerML:
		return "kerml"
	case source.KindSysML:
		return "sysml"
	}
	return ""
}

// sourceText gives every element its lines as written, comments included; one
// with members carries the lines before them and, as its tail, those after.
func (e *encoder) sourceText() {
	for _, subject := range e.graph.Subjects() {
		own, ok := e.regions[subject]
		if !ok {
			continue
		}
		members, ok := e.bodies[subject]
		if !ok {
			e.graph.Add(subject, e.sysx(xSourceText), rdf.String(e.src.region(own)))
			continue
		}
		head, tail := e.src.split(own, members)
		e.graph.Add(subject, e.sysx(xSourceText), rdf.String(head))
		e.graph.Add(subject, e.sysx(xSourceTail), rdf.String(tail))
	}
}

// newEncoder resolves a parsed document, builds its identity side table and
// records each member's qualified name, so references can be told from names.
func newEncoder(file *source.SourceFile, root *ast.RootNamespace) (*encoder, error) {
	res, model := analyzeDocument(file.Name(), root)
	ids, err := documentIdentity(file.Name(), res, model)
	if err != nil {
		return nil, err
	}
	e := &encoder{
		file:           file,
		graph:          rdf.NewGraph(),
		res:            res,
		declared:       map[string]bool{},
		metadataBodies: map[string]bool{},
		fqn:            map[ast.Node]string{},
		links:          map[*ast.QualifiedName]*symbols.Symbol{},
		preceding:      map[ast.Node]ast.Node{},
		introduced:     map[ast.Node]ast.Node{},
		ids:            ids,
		subjects:       map[string]string{},
		regions:        map[rdf.Term]region{},
		bodies:         map[rdf.Term]region{},
		offsets:        map[string]int{},
	}
	for _, ref := range resolve.References(root, res.Index().DocumentRoot(file.Name())) {
		if sym, ok := res.ProbeReference(ref); ok && sym != nil {
			e.links[ref.QN] = sym
		}
	}
	if err := e.collect(root.Members, ""); err != nil {
		return nil, err
	}
	return e, nil
}

type encoder struct {
	file *source.SourceFile
	// src is the text of file as written, which is what sysx:sourceText carries.
	src   *authoredSource
	graph *rdf.Graph
	// res has resolved the document's names, so a reference links to the element
	// the language reaches from where it is written, not to one of the same name.
	res      *resolve.Resolver
	declared map[string]bool
	// metadataBodies holds the qualified names of the metadata usages and the nested
	// members of their bodies: a keywordless member there is a ReferenceUsage.
	metadataBodies map[string]bool
	// fqn is the qualified name of each member node, which is how a succession
	// end the notation leaves unnamed addresses the member it binds.
	fqn map[ast.Node]string
	// links is what each written reference resolves to, read with the rule of
	// its position (a redefinition among the generals, an edge end as a vertex).
	links map[*ast.QualifiedName]*symbols.Symbol
	// preceding is the member a `then` after each member sequences from: the last
	// member before it that is not itself an edge, as the parser reads it.
	preceding map[ast.Node]ast.Node
	// introduced is the member a member-attached `then` sequences to: the one
	// written right after the keyword, whose edge follows it in the body.
	introduced map[ast.Node]ast.Node
	// ids is the document's identity side table: effective ids, declaredness,
	// scopes, and the annotation nodes consumed into it.
	ids *identityFacts
	// subjects maps each minted IRI — element or membership — to what it
	// stands for, so two ids landing on one IRI are refused rather than merged.
	subjects map[string]string
	// idErr holds a collision found where no error can propagate directly.
	idErr error
	// regions holds each element's lines, bodies the lines its members tile.
	regions map[rdf.Term]region
	bodies  map[rdf.Term]region
	// offsets holds where in file each element's declaration starts.
	offsets map[string]int
}

// claim reserves an IRI for what it stands for, returning the holder it
// collides with, if any.
func (e *encoder) claim(iri, standsFor string) (string, bool) {
	if prior, taken := e.subjects[iri]; taken && prior != standsFor {
		return prior, true
	}
	e.subjects[iri] = standsFor
	return "", false
}

// declaredKeyword records the kind keyword as written when it is a synonym of
// the canonical one, so the notation comes back as the author spelled it rather
// than rewritten. A synonym written in a shape the decoder cannot rebuild is
// refused instead: returning the canonical keyword would be a different model.
// referenced states whether the declaration names an existing feature, the
// shape a keyword with no name of its own is rebuilt from.
func (e *encoder) declaredKeyword(subject rdf.Term, node ast.Node, written, canonical, named string, referenced bool) error {
	if written == "" || written == canonical {
		return nil
	}
	if named == "" {
		// A shorter spelling of a multi-word kind keyword (`verification` for
		// `verification case`) states the same kind, so it needs no name.
		for _, word := range strings.Fields(canonical) {
			if written == word {
				return nil
			}
		}
		// `perform a` takes the feature it names in place of a name; with
		// neither, the keyword has nothing the decoder could write it before.
		if referenceMemberKeyword(written) && !referenced {
			return &UnsupportedError{
				What: fmt.Sprintf("the `%s` declaration at %s", written, e.where(node)),
				Note: fmt.Sprintf("it neither declares a name nor names the feature it refers to, the two shapes `%s` is written in, so the notation cannot be rebuilt from the graph and would come back as `%s`, a different declaration", written, canonical),
			}
		}
	}
	e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(written))
	return nil
}

// provenance emits the ProjectRef binding of a scope root as sysx: triples,
// so a graph carries the project its ids are stable within.
func (e *encoder) provenance(subject rdf.Term, node ast.Node) {
	scope := e.ids.provenance[node]
	if scope == nil {
		return
	}
	if scope.ProjectID != "" {
		e.graph.Add(subject, e.sysx(xProjectID), rdf.String(scope.ProjectID))
	}
	if scope.Branch != "" {
		e.graph.Add(subject, e.sysx(xBranch), rdf.String(scope.Branch))
	}
	if scope.Org != "" {
		e.graph.Add(subject, e.sysx(xOrg), rdf.String(scope.Org))
	}
}

// collect walks the tree recording every qualified name it declares. A name
// declared twice in one namespace is reported: the qualified name is an
// element's identity in the graph, so two such members would merge into one.
func (e *encoder) collect(members []ast.Node, owner string) error {
	for i, member := range e.kept(members) {
		node, _ := unwrapMember(member)
		name, children := declaredNameAndMembers(node)
		fqn := qualify(owner, name, i)
		if fqn == "" {
			continue
		}
		e.fqn[node] = fqn
		if e.declared[fqn] {
			return &UnsupportedError{
				What: fmt.Sprintf("the duplicate declaration of %q at %s", name, e.where(node)),
				Note: "a name identifies an element in the graph, so two members of one namespace cannot share it",
			}
		}
		e.declared[fqn] = true
		if err := e.collect(children, fqn); err != nil {
			return err
		}
		if err := e.collectPrefixes(node, fqn, len(e.kept(children))); err != nil {
			return err
		}
		if err := e.collectCrossFeature(node, fqn); err != nil {
			return err
		}
	}
	return nil
}

// collectCrossFeature records the name of the cross feature an end declares
// ahead of itself (`end x1 [m] feature x`), which it owns after its prefixes.
func (e *encoder) collectCrossFeature(node ast.Node, owner string) error {
	u, ok := node.(*ast.Usage)
	if !ok || u.CrossFeature == nil {
		return nil
	}
	cross := u.CrossFeature
	fqn := qualify(owner, cross.Ident.Name, e.crossFeatureIndex(u))
	if e.declared[fqn] {
		return &UnsupportedError{
			What: fmt.Sprintf("the cross feature at %s", e.where(cross)),
			Note: fmt.Sprintf("it is identified as %s, which a body member is named too, and merging two elements into one subject would be a different model", fqn),
		}
	}
	e.fqn[cross] = fqn
	e.declared[fqn] = true
	return nil
}

// crossFeatureIndex is the position an end's cross feature takes among the
// members it owns: after its kept body members and its prefix annotations.
func (e *encoder) crossFeatureIndex(u *ast.Usage) int {
	index := len(e.kept(bodyMembers(u)))
	for _, prefix := range u.Prefixes {
		if prefix != nil && !e.ids.skip(prefix) {
			index++
		}
	}
	return index
}

// collectPrefixes records the names of a declaration's `#M` prefix annotations,
// which take the positions after its kept body members, and of their bodies.
func (e *encoder) collectPrefixes(node ast.Node, owner string, index int) error {
	for _, prefix := range declaredPrefixes(node) {
		if prefix == nil || e.ids.skip(prefix) {
			continue
		}
		fqn := qualify(owner, "", index)
		// A body member may be named `'@N'`, the position name the prefix takes.
		if e.declared[fqn] {
			return &UnsupportedError{
				What: fmt.Sprintf("the prefix annotation at %s", e.where(prefix)),
				Note: fmt.Sprintf("it is identified by its position as %s, which a body member is named, and merging two elements into one subject would be a different model", fqn),
			}
		}
		e.fqn[prefix] = fqn
		e.declared[fqn] = true
		if err := e.collect(prefix.Body, fqn); err != nil {
			return err
		}
		index++
	}
	return nil
}

// encode walks the members of one namespace, emitting the triples for each.
func (e *encoder) encode(members []ast.Node, owner string, ownerTerm rdf.Term) error {
	kept := e.kept(members)
	spans := make([]source.Span, len(kept))
	for i, member := range kept {
		spans[i] = member.Span()
	}
	regions := e.src.tile(spans)
	// Members written on their owner's own lines, such as an accept's payload,
	// are part of its text: the owner is written whole or rebuilt whole.
	inline := !e.src.wholeLines(regions)
	if ownerTerm.Value == "" && len(regions) > 0 {
		// The document has no subject: roots sharing a line each keep their
		// slice of it, and what follows the last root is that root's.
		inline = false
		e.src.shareLines(regions, spans)
		regions[len(regions)-1].end = len(e.src.text)
	}
	if !inline && len(regions) > 0 && ownerTerm.Value != "" {
		body := region{regions[0].start, regions[len(regions)-1].end}
		if prior, ok := e.bodies[ownerTerm]; ok {
			body = region{min(prior.start, body.start), max(prior.end, body.end)}
		}
		e.bodies[ownerTerm] = body
	}
	return e.encodeMembers(kept, regions, inline, owner, ownerTerm)
}

// encodeInline walks members whose lines interleave with their owner's own
// notation, so the owner is written whole or rebuilt whole.
func (e *encoder) encodeInline(members []ast.Node, owner string, ownerTerm rdf.Term) error {
	kept := e.kept(members)
	return e.encodeMembers(kept, make([]region, len(kept)), true, owner, ownerTerm)
}

func (e *encoder) encodeMembers(kept []ast.Node, regions []region, inline bool, owner string, ownerTerm rdf.Term) error {
	// last and beforeLast are the latest members a `then` sequences from; a
	// member-attached `then` follows its target prev, so its source is the
	// latest of them before prev.
	var last, beforeLast, prev ast.Node
	for i, member := range kept {
		node, visibility := unwrapMember(member)
		if node == nil {
			continue
		}
		preceding := last
		if edge, ok := node.(*ast.SuccessionEdge); ok && edge.TargetImplied && prev != nil {
			e.introduced[node] = prev
			if prev == last {
				preceding = beforeLast
			}
		}
		if preceding != nil {
			e.preceding[node] = preceding
		}
		h := memberHead{node: node, visibility: visibility, owner: ownerTerm, index: i,
			lines: regions[i], inline: inline, typeFeature: isTypeFeatureMember(member)}
		if err := e.encodeMember(h, owner); err != nil {
			return err
		}
		if ast.IsSuccessionSource(node) {
			last, beforeLast = node, last
		}
		prev = node
	}
	return nil
}

// kept filters out the identity annotations consumed into the graph's
// identity, so member positions count only what is actually exported.
func (e *encoder) kept(members []ast.Node) []ast.Node {
	out := make([]ast.Node, 0, len(members))
	for _, member := range members {
		if node, _ := unwrapMember(member); node != nil && e.ids.skip(node) {
			continue
		}
		out = append(out, member)
	}
	return out
}

// mint reserves the subject IRI of the element node declares as fqn.
func (e *encoder) mint(node ast.Node, fqn string) (rdf.Term, error) {
	subject := e.ids.subjectForNode(node, fqn)
	if prior, taken := e.claim(subject.Value, fqn); taken {
		return rdf.Term{}, &UnsupportedError{
			What: fmt.Sprintf("the declaration of %s at %s", fqn, e.where(node)),
			Note: fmt.Sprintf("its id lands on the same IRI as %s, and merging two elements into one subject would be a different model", prior),
		}
	}
	return subject, nil
}

// head types an element and states its identity, position and membership in
// its owner; a metaclass this mapping invents is typed in the OpenSysML namespace.
// lines is the text of the member, wrapper and all, unless inline: one written
// on its owner's own lines is part of the owner's text.
type memberHead struct {
	node       ast.Node
	visibility ast.Visibility
	fqn        string
	owner      rdf.Term
	index      int
	metaclass  rdf.Term
	lines      region
	inline     bool
	// typeFeature marks a KerML `member` (TypeFeatureMember): a feature its type
	// owns through a plain OwningMembership rather than a FeatureMembership.
	typeFeature bool
}

func (e *encoder) head(subject rdf.Term, h memberHead) {
	node, visibility, fqn, ownerTerm, index, metaclass, lines, inline :=
		h.node, h.visibility, h.fqn, h.owner, h.index, h.metaclass, h.lines, h.inline
	e.graph.Add(subject, rdf.IRI(rdf.RDFType), metaclass)
	e.graph.Add(subject, e.sysml(pQualifiedName), rdf.String(fqn))
	// The id an API reader addresses the element by, which is the id its own
	// IRI ends in, so the two cannot disagree.
	e.graph.Add(subject, e.sysml(pElementID), rdf.String(rdf.LocalName(subject.Value)))
	e.graph.Add(subject, e.sysx(xMemberIndex), rdf.Int(index))
	e.offsets[subject.Value] = node.Span().Offset
	if !inline {
		e.regions[subject] = lines
	}
	if language := languageName(e.file.Kind()); ownerTerm.Value == "" && language != "" {
		e.graph.Add(subject, e.sysx(xSourceLanguage), rdf.String(language))
	}
	if e.ids.declaredIDAt(node) {
		e.graph.Add(subject, e.sysx(xDeclaredID), rdf.Bool(true))
	}
	e.provenance(subject, node)
	membership := rdf.Term{}
	if ownerTerm.Value != "" {
		// Only a namespace is an owningNamespace; a relationship owner is
		// stated by sysml:owner and the ownership owningMembership wires.
		if !isRelationship(e.metaclassOf(ownerTerm)) {
			e.graph.Add(subject, e.sysml(pOwningNamespace), ownerTerm)
		}
		_, crossing := node.(*ast.CrossFeatureMember)
		membership = e.owningMembership(subject, ownerTerm, fqn, ast.IsExpression(node), e.variantMember(node, ownerTerm), crossing || h.typeFeature)
	}
	if keyword := visibilityKeyword(visibility); keyword != "" {
		// The membership states the visibility a member is declared with; a
		// relationship, such as an import, states its own.
		visible := subject
		if membership.Value != "" {
			visible = membership
		}
		e.graph.Add(visible, e.sysml(pVisibility), rdf.String(keyword))
	}
}

// encodeMember maps one member: h.node is the declaration inside its membership
// wrapper and h.lines the text of the member, wrapper and all, unless inline;
// owner is the qualified name of the namespace the member is declared in.
func (e *encoder) encodeMember(h memberHead, owner string) error {
	node, ownerTerm, index := h.node, h.owner, h.index
	name, _ := declaredNameAndMembers(node)
	fqn := qualify(owner, name, index)
	subject, err := e.mint(node, fqn)
	if err != nil {
		return err
	}
	// A bare expression among a body's members is the result the body computes.
	result := ast.IsExpression(node)
	head := func(metaclass rdf.Term) {
		h.fqn, h.metaclass = fqn, metaclass
		e.head(subject, h)
	}

	switch n := node.(type) {
	case *ast.Package:
		head(rdf.SysMLTerm("Package"))
		e.ident(subject, n.Ident)
		e.flags(subject, []boolProperty{
			{"isLibraryPackage", n.IsLibrary},
			{"isStandardLibraryPackage", n.IsStandard},
		})
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Members); err != nil {
			return err
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Namespace:
		head(rdf.SysMLTerm("Namespace"))
		e.ident(subject, n.Ident)
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Members); err != nil {
			return err
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Definition:
		metaclass, ok := definitionMetaclass[n.Kind]
		if !ok {
			return &UnsupportedError{What: fmt.Sprintf("definition kind %q at %s", n.Kind, e.where(n))}
		}
		head(rdf.SysMLTerm(metaclass))
		e.ident(subject, n.Ident)
		if err := e.declaredKeyword(subject, n, n.Keyword, definitionKeyword(n.Kind), n.Ident.Name, false); err != nil {
			return err
		}
		e.flags(subject, []boolProperty{
			{"isAbstract", n.IsAbstract},
			{"isVariation", n.IsVariation || n.Kind == ast.DefEnumeration},
			{"isAll", n.IsAll},
			{"isConstant", n.IsConstant},
			{"isEvent", n.IsEvent},
			{"isParallel", n.IsParallel},
		})
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Members); err != nil {
			return err
		}
		e.relationships(subject, owner, n.Relationships)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Usage:
		inBody := e.metadataBodies[owner]
		metaclass, ok := usageMetaclassOf(n, inBody)
		if !ok {
			return &UnsupportedError{What: fmt.Sprintf("usage kind %q at %s", n.Kind, e.where(n))}
		}
		head(rdf.SysMLTerm(metaclass))
		if !shorthandRelationship(n) {
			e.ident(subject, n.Ident)
		}
		switch {
		case verbatimUsage(n):
			// A verbatim head is reproduced as written, so its keyword needs no
			// reconstructing and never has to be refused.
		case bareAcceptNode(n, e.text(n)):
			// The `action` of an accept node is optional and the parser records it
			// either way, so what the author wrote is read from the source.
			e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String("accept"))
		case n.Keyword == "" && !e.wroteKindKeyword(n):
			// A feature written with no kind keyword (`in x : Real;`) takes its kind
			// from its owner; writing that kind's keyword back would declare more.
			e.graph.Add(subject, e.sysx(xImplicitKind), rdf.Bool(true))
		default:
			if err := e.declaredKeyword(subject, n, n.Keyword, usageKeyword(n.Kind), n.Ident.Name, referencesFeature(n)); err != nil {
				return err
			}
		}
		// The prefix a kind keyword was qualified with (`assume constraint c`) is
		// part of the declaration; `assert` is the AssertConstraintUsage metaclass.
		if n.PrefixKeyword != "" && !assertedConstraint(n) && !prefixCarriedByGraph(n) {
			e.graph.Add(subject, e.sysx(xDeclaredPrefix), rdf.String(n.PrefixKeyword))
		}
		e.flags(subject, []boolProperty{
			{"isAbstract", n.IsAbstract},
			{"isVariation", n.IsVariation},
			{"isVariant", e.variantMember(n, ownerTerm)},
			{"isNegated", n.IsNegated},
			{"isReference", n.IsReference},
			{"isAll", n.IsAll},
			{"isEnd", n.IsEnd},
			{"isChain", n.IsChain},
			{"isConstant", n.IsConstant},
			{"isEvent", n.IsEvent && metaclass != mEventOccurrenceUsage},
			{"isIndividual", n.IsIndividual},
			{"isComposite", n.IsComposite},
			// A snapshot or timeslice is a portion by its kind (OccurrenceUsage::portionKind).
			{"isPortion", n.IsPortion || n.Portion != ast.PortionNone},
			{"isDerived", n.IsDerived},
			{"isOrdered", n.IsOrdered},
			{"isNonunique", n.IsNonunique},
			{"isConjugated", n.HasConjugatedTyping()},
			{"isAccept", n.IsAccept},
			{"isResult", n.IsResult},
			{"isParallel", n.IsParallel},
		})
		if err := e.prefixes(subject, fqn, n.Prefixes, bodyMembers(n)); err != nil {
			return err
		}
		if keyword := directionKeyword(n.Direction); keyword != "" {
			e.graph.Add(subject, e.sysml(pDirection), rdf.String(keyword))
		}
		if portion := portionKeyword(n.Portion); portion != "" {
			e.graph.Add(subject, e.sysml(pPortionKind), rdf.String(portion))
		}
		e.relationships(subject, owner, n.Relationships)
		e.multiplicity(subject, owner, n.Multiplicity)
		if err := e.crossFeature(subject, fqn, n); err != nil {
			return err
		}
		e.featureValue(subject, owner, n.Value, n.ValueIsDefault, n.ValueIsInitial)
		// A declaration head that binds ends (connect/bind/flow/succession),
		// a transition, an accept action or a satisfy usage states its ends
		// and form structurally, since the properties above do not.
		if verbatimUsage(n) {
			e.bindingEnds(subject, owner, n)
			e.endForm(subject, n)
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		if inBody || n.Kind == ast.UsageMetadata {
			e.metadataBodies[fqn] = true
		}
		return e.encode(bodyMembers(n), fqn, subject)

	case *ast.Import:
		head(rdf.SysMLTerm("Import"))
		e.graph.Add(subject, e.sysml(pImportedNamespace), e.reference(n.Imported))
		e.flags(subject, []boolProperty{
			{pIsImportAll, n.IsAll},
			{xNamespaceImport, n.Kind == ast.ImportNamespace},
			{xRecursive, n.IsRecursive},
			{xExpose, n.IsExpose},
		})
		e.expression(subject, e.sysx(xFilter), xFilter, owner, n.FilterExpr)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Body, fqn, subject)

	case *ast.Alias:
		head(rdf.OpenSysMLTerm(mAlias))
		e.ident(subject, n.Ident)
		e.graph.Add(subject, e.sysml(pAliasFor), e.reference(n.For))
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Body, fqn, subject)

	case *ast.RelationshipMember:
		form, ok := relationshipElementForm[n.Kind]
		if n.Conjugated {
			form, ok = conjugationForm, true
		}
		if !ok {
			return &UnsupportedError{What: fmt.Sprintf("the %s relationship at %s", n.Keyword, e.where(n))}
		}
		head(rdf.SysMLTerm(form.metaclass))
		e.ident(subject, n.Ident)
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(n.Keyword))
		if n.PrefixKeyword != "" {
			e.graph.Add(subject, e.sysx(xDeclaredPrefix), rdf.String(n.PrefixKeyword))
		}
		e.relationshipEnd(subject, owner, form.source, n.Source)
		e.relationshipEnd(subject, owner, form.target, n.Target)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Dependency:
		head(rdf.SysMLTerm("Dependency"))
		e.ident(subject, n.Ident)
		for _, client := range n.Clients {
			e.graph.Add(subject, e.sysml(pClient), e.reference(client))
		}
		for _, supplier := range n.Suppliers {
			e.graph.Add(subject, e.sysml(pSupplier), e.reference(supplier))
		}
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Body); err != nil {
			return err
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Body, fqn, subject)

	case *ast.Comment:
		head(rdf.SysMLTerm("Comment"))
		e.ident(subject, n.Ident)
		for _, about := range n.About {
			e.graph.Add(subject, e.sysml(pAnnotatedElement), e.reference(about))
		}
		if n.Locale != "" {
			e.graph.Add(subject, e.sysml(pLocale), rdf.String(lexer.StringValue(n.Locale)))
		}
		e.graph.Add(subject, e.sysml(pBody), rdf.String(commentBody(e.src.slice(n.BodySpan))))
		return nil

	case *ast.Documentation:
		head(rdf.SysMLTerm(mDocumentation))
		e.documentation(subject, n)
		return nil

	case *ast.TextualRepresentation:
		head(rdf.SysMLTerm("TextualRepresentation"))
		e.ident(subject, n.Ident)
		e.graph.Add(subject, e.sysml(pLanguage), rdf.String(lexer.StringValue(n.Language)))
		e.graph.Add(subject, e.sysml(pBody), rdf.String(commentBody(e.src.slice(n.BodySpan))))
		return nil

	case *ast.MultiplicityDecl:
		head(rdf.OpenSysMLTerm(mMultiplicity))
		e.ident(subject, n.Ident)
		e.multiplicity(subject, owner, n.Range)
		// A MultiplicitySubset states its bounds by subsetting, not as a range.
		if n.Subsets != nil {
			e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelSubsets]),
				e.reference(n.Subsets))
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.ConstraintMember:
		// A condition of a constraint body, written bare (`x > 0`) or as a
		// nested constraint (`assert constraint [name] { … }`).
		head(rdf.OpenSysMLTerm(mConstraint))
		if n.Name != "" {
			e.graph.Add(subject, e.sysml(pDeclaredName), rdf.String(n.Name))
		}
		if n.Keyword != "" {
			e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(n.Keyword))
		}
		e.flags(subject, []boolProperty{{"isNegated", n.IsNegated}})
		return e.condition(subject, fqn, owner, n.Expression, nil, true, n.Body)

	case *ast.AssumeMember:
		head(rdf.OpenSysMLTerm(mAssume))
		return e.requirementCondition(subject, fqn, owner, requirementConditionDecl{
			prefixes: n.Prefixes, ident: n.Ident, relationships: n.Relationships, multiplicity: n.Multiplicity,
			value: n.Value, isDefault: n.ValueIsDefault, isInitial: n.ValueIsInitial,
			expression: n.Expression, reference: n.Reference, hasBody: n.HasBody, body: n.Body,
		})

	case *ast.RequireMember:
		head(rdf.OpenSysMLTerm(mRequire))
		return e.requirementCondition(subject, fqn, owner, requirementConditionDecl{
			prefixes: n.Prefixes, ident: n.Ident, relationships: n.Relationships, multiplicity: n.Multiplicity,
			value: n.Value, isDefault: n.ValueIsDefault, isInitial: n.ValueIsInitial,
			expression: n.Expression, reference: n.Reference, hasBody: n.HasBody, body: n.Body,
		})

	case *ast.PrefixMetadata:
		// `@M { … }` written as a member annotates its owner, or what its
		// `about` names (SysML v2 MetadataUsage).
		head(rdf.SysMLTerm(usageMetaclass[ast.UsageMetadata]))
		return e.metadataUsage(subject, fqn, owner, n, "@")

	case *ast.SubjectMember:
		// A subject parameter is a usage of its own (SysML v2 8.2.2.16), so it
		// is mapped as the SubjectMembership a `subject s : X` declares.
		head(rdf.SysMLTerm(usageMetaclass[ast.UsageSubject]))
		e.ident(subject, n.Ident)
		if n.TypeRef != nil {
			e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelTyping]), e.reference(n.TypeRef))
		}
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Body); err != nil {
			return err
		}
		e.relationships(subject, owner, n.Relationships)
		e.multiplicity(subject, owner, n.Multiplicity)
		e.featureValue(subject, owner, n.BindingExpr, n.ValueIsDefault, n.ValueIsInitial)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Body, fqn, subject)

	case *ast.FilterMember:
		head(rdf.OpenSysMLTerm(mFilter))
		e.expression(subject, e.sysx(xFilter), xFilter, owner, n.Condition)
		return nil

	case *ast.ErrorNode:
		return &UnsupportedError{
			What: fmt.Sprintf("the malformed declaration at %s", e.where(n)),
			Note: "fix the syntax error before converting",
		}
	}
	// The result is the Expression element itself, owned through its
	// ResultExpressionMembership as the abstract syntax has it.
	if result {
		head(rdf.SysMLTerm(expressionMetaclass(node)))
		e.graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
		e.expressionStructure(subject, owner, node)
		if membership, ok := e.graph.Object(subject, rdf.SysML+pOwningMembership); ok {
			e.graph.Add(membership, e.sysml(pOwnedResultExpression), subject)
		}
		return nil
	}
	// A behavioral node — a control node, statement, loop, conditional, state or
	// transition — is mapped by the behavior half of this encoder.
	if handled, err := e.encodeBehavior(node, head, subject, fqn, owner, index); handled {
		return err
	}
	return &UnsupportedError{
		What: fmt.Sprintf("the %s at %s", nodeDescription(node), e.where(node)),
		Note: rdfLimitationsNote,
	}
}

// owningMembership wires a member to its owner the way the abstract syntax does,
// returning the membership minted between them, or the empty term when no
// membership stands between the two. The API's payloads reach a member through
// its membership, so a compact owner triple alone leaves a client walking down
// from a root with nothing to follow. result marks a body's result expression,
// which a ResultExpressionMembership owns; variant a usage a VariantMembership
// owns; plain a feature its type owns through a plain OwningMembership rather
// than a FeatureMembership: an end's cross feature, or a KerML `member` feature.
func (e *encoder) owningMembership(member, owner rdf.Term, memberFQN string, result, variant, plain bool) rdf.Term {
	ownerClass, memberClass := e.metaclassOf(owner), e.metaclassOf(member)
	// A metadata usage annotates its owner through an OwningMembership whatever
	// the owner is, a relationship included (SysML.xtext PrefixMetadataMember).
	metadata := memberClass == "MetadataUsage"
	switch {
	case isRelationship(ownerClass) && !metadata:
		// A relationship owns its related element itself, as a state's entry
		// membership owns the action it states.
		e.relationshipOwnership(member, owner, ownerClass, memberClass)
		return rdf.Term{}
	case isRelationship(memberClass):
		// A namespace owns a relationship it declares — an import, a dependency,
		// a membership — as an owned relationship, with no membership between.
		e.graph.Add(member, e.sysml(pOwner), owner)
		e.graph.Add(member, e.sysml(pOwningRelatedElement), owner)
		e.graph.Add(owner, e.sysml(pOwnedRelationship), member)
		if ontology.IsAncestorOrSelf(memberClass, "Import") {
			e.graph.Add(member, e.sysml(pImportOwningNamespace), owner)
			e.graph.Add(owner, e.sysml(pOwnedImport), member)
		}
		return rdf.Term{}
	}
	// A type owns a feature through a FeatureMembership, which is the membership
	// the API's payloads carry for it; anything else, a metadata usage included,
	// through an OwningMembership.
	feature := ontology.IsAncestorOrSelf(memberClass, "Feature") && isType(ownerClass) && !metadata
	membership := rdf.OwningMembershipIRIOf(member)
	// The membership shares the element namespace, so its IRI is reserved too.
	if prior, taken := e.claim(membership.Value, memberFQN+"'s owning membership"); taken && e.idErr == nil {
		e.idErr = &UnsupportedError{
			What: fmt.Sprintf("the owning membership of %s", memberFQN),
			Note: fmt.Sprintf("its id lands on the same IRI as %s, and merging two elements into one subject would be a different model", prior),
		}
	}
	e.graph.Add(member, e.sysml(pOwner), owner)
	e.graph.Add(member, e.sysml(pOwningRelationship), membership)
	e.graph.Add(member, e.sysml(pOwningMembership), membership)

	// A variant is a member of its variation, not a feature of it: the metamodel
	// owns it through a VariantMembership, which is an OwningMembership. An end's
	// cross feature (KerML.xtext OwnedCrossingFeatureMember) and a `member`
	// feature (KerML.xtext TypeFeatureMember) are owned the same way.
	if variant || plain {
		feature = false
	}
	metaclass := mOwningMembership
	switch {
	case result:
		metaclass = mResultExpressionMembership
	case variant:
		metaclass = mVariantMembership
	case feature:
		metaclass = mFeatureMembership
	}
	e.graph.Add(membership, rdf.IRI(rdf.RDFType), e.sysml(metaclass))
	e.graph.Add(membership, e.sysml(pElementID), rdf.String(rdf.LocalName(membership.Value)))
	// The namespace owns the membership too, so it is not read as a root.
	e.graph.Add(membership, e.sysml(pOwner), owner)
	e.graph.Add(membership, e.sysml(pMemberElement), member)
	e.graph.Add(membership, e.sysml(pOwnedMemberElement), member)
	e.graph.Add(membership, e.sysml(pOwnedRelatedElement), member)
	e.graph.Add(membership, e.sysml(pOwningRelatedElement), owner)
	// Only a namespace has members; a relationship owner just owns the membership.
	if !isRelationship(ownerClass) {
		e.graph.Add(membership, e.sysml(pMembershipOwningNamespace), owner)
		e.graph.Add(owner, e.sysml(pOwnedMember), member)
		e.graph.Add(owner, e.sysml(pOwnedMembership), membership)
	}
	e.graph.Add(owner, e.sysml(pOwnedRelationship), membership)
	if feature {
		e.graph.Add(membership, e.sysml(pOwnedMemberFeature), member)
		e.graph.Add(membership, e.sysml(pOwningType), owner)
		e.graph.Add(owner, e.sysml(pOwnedFeature), member)
		e.graph.Add(owner, e.sysml(pOwnedFeatureMembership), membership)
	}
	if variant {
		e.graph.Add(membership, e.sysml(pOwnedVariantUsage), member)
		// Only a definition or usage derives its variants; a package that
		// declares one still owns it through the membership the grammar states.
		if ontology.IsAncestorOrSelf(ownerClass, "Definition") || ontology.IsAncestorOrSelf(ownerClass, "Usage") {
			e.graph.Add(owner, e.sysml(pVariant), member)
			e.graph.Add(owner, e.sysml(pVariantMembership), membership)
		}
	}
	return membership
}

// variantMember reports whether node is a variant of its owner: a usage declared
// `variant`, or an enumerated value, which its enumeration definition owns as one.
func (e *encoder) variantMember(node ast.Node, ownerTerm rdf.Term) bool {
	usage, ok := node.(*ast.Usage)
	if !ok {
		return false
	}
	return usage.IsVariant || enumeratedValue(usage, e.metaclassOf(ownerTerm))
}

// enumeratedValue reports whether usage is an enumerated value: an enumeration
// usage that an enumeration definition owns (SysML.xtext EnumerationUsageMember).
func enumeratedValue(usage *ast.Usage, ownerClass string) bool {
	return usage.Kind == ast.UsageEnumeration && ownerClass == definitionMetaclass[ast.DefEnumeration]
}

// relationshipOwnership wires a member owned by a relationship rather than by a
// namespace, such as a state's entry action. A relationship owns its related
// element itself, so no membership is minted between them.
func (e *encoder) relationshipOwnership(member, owner rdf.Term, ownerClass, memberClass string) {
	e.graph.Add(member, e.sysml(pOwner), owner)
	e.graph.Add(owner, e.sysml(pOwnedRelatedElement), member)
	if isRelationship(memberClass) {
		// A relationship states the element that owns it, not an owning
		// relationship of its own.
		e.graph.Add(member, e.sysml(pOwningRelatedElement), owner)
		e.graph.Add(owner, e.sysml(pOwnedRelationship), member)
		return
	}
	e.graph.Add(member, e.sysml(pOwningRelationship), owner)
	if ontology.IsAncestorOrSelf(ownerClass, "Membership") {
		e.graph.Add(member, e.sysml(pOwningMembership), owner)
		e.graph.Add(owner, e.sysml(pMemberElement), member)
	}
	if ontology.IsAncestorOrSelf(ownerClass, "OwningMembership") {
		e.graph.Add(owner, e.sysml(pOwnedMemberElement), member)
	}
	if ontology.IsAncestorOrSelf(ownerClass, mFeatureMembership) && ontology.IsAncestorOrSelf(memberClass, "Feature") {
		e.graph.Add(owner, e.sysml(pOwnedMemberFeature), member)
	}
}

// metaclassOf is the ontology name of the metaclass a subject is typed with,
// which is empty for a metaclass this mapping invents.
func (e *encoder) metaclassOf(subject rdf.Term) string {
	return ontology.LocalName(e.graph.Type(subject))
}

// isRelationship reports whether a metaclass relates elements rather than
// containing them, which decides whether ownership needs a membership.
func isRelationship(metaclass string) bool {
	return ontology.IsAncestorOrSelf(metaclass, "Relationship") && !ontology.IsAncestorOrSelf(metaclass, "Namespace")
}

// condition emits the three forms a condition member is written in: an inline
// expression, a reference to the constraint it states (`require R { … }`), or a
// nested constraint stating its conditions in a body.
func (e *encoder) condition(subject rdf.Term, fqn, owner string, expr ast.Node, ref *ast.QualifiedName, hasBody bool, body []ast.Node) error {
	if expr != nil {
		e.expression(subject, e.sysx(xCondition), xCondition, owner, expr)
		return nil
	}
	if ref != nil {
		e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelReferences]), e.reference(ref))
	}
	e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(hasBody))
	return e.encode(body, fqn, subject)
}

// requirementConditionDecl is the head an `assume`/`require` member declares
// for the constraint usage it owns, besides the condition itself.
type requirementConditionDecl struct {
	prefixes      []*ast.PrefixMetadata
	ident         ast.Identification
	relationships []*ast.Relationship
	multiplicity  *ast.Multiplicity
	value         ast.Node
	isDefault     bool
	isInitial     bool
	expression    ast.Node
	reference     *ast.QualifiedName
	hasBody       bool
	body          []ast.Node
}

// requirementCondition emits an `assume`/`require` member together with its
// constraint usage's declaration (`require #goal constraint c : C [1] = true`).
func (e *encoder) requirementCondition(subject rdf.Term, fqn, owner string, n requirementConditionDecl) error {
	e.ident(subject, n.ident)
	// The declaration form is keyed by its keyword: its `references C` is a
	// specialization, where the reference form's `require C` is the head.
	if n.expression == nil && n.reference == nil {
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String("constraint"))
	}
	if err := e.prefixes(subject, fqn, n.prefixes, n.body); err != nil {
		return err
	}
	e.relationships(subject, owner, n.relationships)
	e.multiplicity(subject, owner, n.multiplicity)
	e.featureValue(subject, owner, n.value, n.isDefault, n.isInitial)
	return e.condition(subject, fqn, owner, n.expression, n.reference, n.hasBody, n.body)
}

// shorthandRelationship reports whether a usage's identification is the first
// end of a shorthand head (`bind x = y;`) rather than a name it declares; the
// named form spells the kind out (`binding b bind x = y;`).
func shorthandRelationship(n *ast.Usage) bool {
	return n.Kind == ast.UsageBinding && n.Keyword == "bind"
}

// bodyMembers returns the members written in a usage's body: a payload declared
// inline by `of name : Type` is parsed as a member but belongs to the head.
func bodyMembers(n *ast.Usage) []ast.Node {
	if n.FlowEnds == nil || n.FlowEnds.PayloadDecl == nil {
		return n.Members
	}
	members := make([]ast.Node, 0, len(n.Members))
	for _, m := range n.Members {
		if m != ast.Node(n.FlowEnds.PayloadDecl) {
			members = append(members, m)
		}
	}
	return members
}

// verbatimUsage reports whether a usage's declaration head has to be carried as
// source text rather than rebuilt from properties.
//
// An accept parameter is not verbatim: it is a synthetic member of the accept
// shorthand, fully described by its direction, type and isAccept flag, and the
// printer rebuilds the shorthand from those.
func verbatimUsage(n *ast.Usage) bool {
	if n.IsAccept {
		return false
	}
	if len(n.ConnectorEnds) > 0 || n.FlowEnds != nil {
		return true
	}
	switch n.Kind {
	case ast.UsageConnector, ast.UsageSuccession, ast.UsageBinding, ast.UsageFlow,
		ast.UsageTransition, ast.UsageSatisfy:
		return true
	}
	return false
}

// bindingEnds states the features a binding head relates as structure beside the
// text it is kept as, so a consumer reads the ends without reading notation.
func (e *encoder) bindingEnds(subject rdf.Term, owner string, n *ast.Usage) {
	// A named end (`connect bead ::> t.bead`) relates the feature it attaches
	// to and carries its own name beside it.
	for i, end := range n.ConnectorEnds {
		if end == nil {
			continue
		}
		slot := fmt.Sprintf("end%d", i)
		e.endNode(subject, owner, slot, i, "", end.AttachedTarget(), end.Multiplicity)
		if id, named := end.DeclaredName(); named {
			node := rdf.ExpressionIRI(subject, slot)
			e.graph.Add(node, e.sysx(xEndName), rdf.String(id.Name))
			if keyword := e.referencesKeyword(end); keyword != referencesSymbol {
				e.graph.Add(node, e.sysx(xEndReferencesKeyword), rdf.String(keyword))
			}
		}
	}
	if n.FlowEnds != nil {
		e.endNode(subject, owner, "flowSource", 0, "source", n.FlowEnds.From, nil)
		e.endNode(subject, owner, "flowTarget", 1, "target", n.FlowEnds.To, nil)
		e.endNode(subject, owner, "flowPayload", -1, "payload", n.FlowEnds.Payload, nil)
	}
}

// endNode emits one end as an expression node, tagged with its position.
func (e *encoder) endNode(subject rdf.Term, owner, slot string, index int, role string, target ast.Node, mult *ast.Multiplicity) {
	if target == nil {
		return
	}
	e.expression(subject, e.sysx(xRelatedFeature), slot, owner, target)
	e.endMarks(rdf.ExpressionIRI(subject, slot), owner, index, role, mult)
}

// endMarks tags an end node with its position, role and the bounds of the
// multiplicity written ahead of it (`connect [1] a to [0..1] b`).
func (e *encoder) endMarks(end rdf.Term, owner string, index int, role string, mult *ast.Multiplicity) {
	if index >= 0 {
		e.graph.Add(end, e.sysx(xEndIndex), rdf.Int(index))
	}
	if role != "" {
		e.graph.Add(end, e.sysx(xEndRole), rdf.String(role))
	}
	e.multiplicity(end, owner, mult)
}

func (e *encoder) sysml(name string) rdf.Term { return rdf.SysMLTerm(name) }
func (e *encoder) sysx(name string) rdf.Term  { return rdf.OpenSysMLTerm(name) }

// documentation emits what a `doc` states: its identification, locale and body.
func (e *encoder) documentation(subject rdf.Term, n *ast.Documentation) {
	e.ident(subject, n.Ident)
	if n.Locale != "" {
		e.graph.Add(subject, e.sysml(pLocale), rdf.String(lexer.StringValue(n.Locale)))
	}
	e.graph.Add(subject, e.sysml(pBody), rdf.String(commentBody(e.src.slice(n.BodySpan))))
}

func (e *encoder) ident(subject rdf.Term, ident ast.Identification) {
	if ident.Name != "" {
		e.graph.Add(subject, e.sysml(pDeclaredName), rdf.String(ident.Name))
	}
	if ident.ShortName != "" {
		e.graph.Add(subject, e.sysml(pDeclaredShortName), rdf.String(ident.ShortName))
	}
}

// featureValue emits a feature's value with the `default` and `:=` of its
// operator (FeatureValue::isDefault, isInitial), so the operator converts back.
func (e *encoder) featureValue(subject rdf.Term, owner string, value ast.Node, isDefault, isInitial bool) {
	if value == nil {
		return
	}
	e.expression(subject, e.sysml(pValue), pValue, owner, value)
	e.flags(subject, []boolProperty{
		{pIsDefault, isDefault},
		{pIsInitial, isInitial},
	})
}

func (e *encoder) flags(subject rdf.Term, flags []boolProperty) {
	for _, flag := range flags {
		if !flag.value {
			continue
		}
		property := e.sysml(flag.name)
		if strings.HasPrefix(flag.name, "is") && isExtensionFlag(flag.name) {
			property = e.sysx(flag.name)
		}
		e.graph.Add(subject, property, rdf.Bool(true))
	}
}

// isExtensionFlag reports whether a flag lives in the OpenSysML namespace
// because the SysML metamodel has no such property.
func isExtensionFlag(name string) bool {
	switch name {
	case "isLibraryPackage", "isStandardLibraryPackage", xNamespaceImport, xRecursive, xExpose:
		return true
	}
	return false
}

// prefixes maps the `#M` annotations ahead of a declaration as metadata usages
// it owns after its body members (PrefixMetadataMember), keyed `#` for the writer.
func (e *encoder) prefixes(subject rdf.Term, fqn string, prefixes []*ast.PrefixMetadata, members []ast.Node) error {
	index := len(e.kept(members))
	for _, prefix := range prefixes {
		if prefix == nil || e.ids.skip(prefix) {
			continue
		}
		prefixFQN := e.fqn[prefix]
		prefixSubject, err := e.mint(prefix, prefixFQN)
		if err != nil {
			return err
		}
		// A prefix is written in its owner's head, so its text is the owner's.
		e.head(prefixSubject, memberHead{node: prefix, visibility: ast.VisibilityDefault, fqn: prefixFQN,
			owner: subject, index: index, metaclass: rdf.SysMLTerm(usageMetaclass[ast.UsageMetadata]), inline: true})
		if err := e.metadataUsage(prefixSubject, prefixFQN, fqn, prefix, "#"); err != nil {
			return err
		}
		index++
	}
	return nil
}

// crossFeature maps the cross feature an end declares ahead of itself as a feature
// it owns through an OwningMembership (KerML.xtext OwnedCrossingFeature).
func (e *encoder) crossFeature(subject rdf.Term, fqn string, n *ast.Usage) error {
	cross := n.CrossFeature
	if cross == nil {
		return nil
	}
	crossFQN := e.fqn[cross]
	crossSubject, err := e.mint(cross, crossFQN)
	if err != nil {
		return err
	}
	metaclass := crossFeatureMetaclass(e.file.Kind() == source.KindKerML)
	// The cross feature is written in the end's head, so its text is the end's.
	e.head(crossSubject, memberHead{node: cross, visibility: ast.VisibilityDefault, fqn: crossFQN,
		owner: subject, index: e.crossFeatureIndex(n), metaclass: rdf.SysMLTerm(metaclass), inline: true})
	e.ident(crossSubject, cross.Ident)
	// The prefix between `end` and the cross feature is the cross feature's own,
	// stated as an end's would be (KerML.xtext OwnedCrossingFeature BasicFeaturePrefix).
	if cross.IsVariable {
		e.graph.Add(crossSubject, e.sysx(xDeclaredPrefix), rdf.String("var"))
	}
	e.flags(crossSubject, []boolProperty{
		{"isAbstract", cross.IsAbstract},
		{"isVariation", cross.IsVariation},
		{"isReference", cross.IsReference},
		{"isConstant", cross.IsConstant},
		{"isComposite", cross.IsComposite},
		{"isPortion", cross.IsPortion},
		{"isDerived", cross.IsDerived},
		{"isOrdered", cross.IsOrdered},
		{"isNonunique", cross.IsNonunique},
	})
	if keyword := directionKeyword(cross.Direction); keyword != "" {
		e.graph.Add(crossSubject, e.sysml(pDirection), rdf.String(keyword))
	}
	e.relationships(crossSubject, fqn, cross.Relationships)
	e.multiplicity(crossSubject, fqn, cross.Multiplicity)
	return nil
}

// metadataUsage states an annotation's definition, the elements it is about and
// its body; the sigil it was written with is its declared keyword.
func (e *encoder) metadataUsage(subject rdf.Term, fqn, owner string, n *ast.PrefixMetadata, sigil string) error {
	if n.Type == nil {
		return &UnsupportedError{
			What: fmt.Sprintf("the metadata annotation at %s", e.where(n)),
			Note: "it names no metadata definition, and an annotation of nothing would be a different model",
		}
	}
	e.ident(subject, n.Ident)
	e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(sigil))
	e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelTyping]), e.reference(n.Type))
	for _, about := range n.About {
		e.graph.Add(subject, e.sysml(pAnnotatedElement), e.reference(about))
	}
	e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
	e.metadataBodies[fqn] = true
	return e.encode(n.Body, fqn, subject)
}

// wroteKindKeyword reports whether the declaration spells its kind keyword out.
// A directed usage (`in attribute speed`) does not record the keyword it wrote,
// so the words ahead of the name are read from the source.
func (e *encoder) wroteKindKeyword(n *ast.Usage) bool {
	keyword := usageKeyword(n.Kind)
	if keyword == "" {
		return false
	}
	start, head := n.Span().Offset, n.Span().End()
	if name := n.Ident.NameSpan; name.Len > 0 && name.Offset < head {
		head = name.Offset
	}
	if short := n.Ident.ShortNameSpan; short.Len > 0 && short.Offset < head {
		head = short.Offset
	}
	if head <= start {
		return false
	}
	// A keyword inside a comment is trivia the declaration does not state, so the
	// comments are dropped before the words are read.
	text := withoutComments(e.file.Text(source.Span{Offset: start, Len: head - start}))
	// An unnamed declaration's keyword, if it wrote one, is ahead of everything
	// its head can state.
	if cut := strings.IndexAny(text, ":=;[{"); cut >= 0 {
		text = text[:cut]
	}
	written := strings.Fields(text)
	for _, word := range strings.Fields(keyword) {
		if !slices.Contains(written, word) {
			return false
		}
	}
	return true
}

// withoutComments replaces every comment with a space, told apart by the lexer
// the parser reads them with, so each shape it scans is excluded by construction.
func withoutComments(text string) string {
	var kept strings.Builder
	lx := lexer.New(source.New("head.sysml", []byte(text)))
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		switch tok.Kind {
		case lexer.SLNote, lexer.MLNote, lexer.RegularComment:
			kept.WriteByte(' ')
		default:
			kept.WriteString(text[tok.Span.Offset:tok.Span.End()])
		}
	}
	return kept.String()
}

// relationships writes a head's clauses in relationshipOrder, not the order the
// notation spelled them in, so the Turtle is the same for every spelling.
func (e *encoder) relationships(subject rdf.Term, owner string, rels []*ast.Relationship) {
	for _, kind := range relationshipOrder {
		property := relationshipProperty[kind]
		for _, rel := range rels {
			if rel == nil || rel.Target == nil || rel.Kind != kind {
				continue
			}
			// A name is mapped as a reference, which links it when this document
			// declares it; a feature chain or other expression is not a name, so it
			// is carried as the text it was written as.
			if name, ok := rel.Target.(*ast.QualifiedName); ok {
				e.graph.Add(subject, e.sysml(property), e.reference(name))
				continue
			}
			e.graph.Add(subject, e.sysml(property), rdf.TypedLiteral(e.text(rel.Target), rdf.OpenSysML+dtExpression))
		}
	}
}

// relationshipEnd writes one end of a keyword-first relationship, as a link
// when it names an element of this graph and as its written text otherwise.
func (e *encoder) relationshipEnd(subject rdf.Term, owner, property string, end ast.Node) {
	if end == nil {
		return
	}
	if name, ok := end.(*ast.QualifiedName); ok {
		e.graph.Add(subject, e.sysml(property), e.reference(name))
		return
	}
	e.graph.Add(subject, e.sysml(property), rdf.TypedLiteral(e.text(end), rdf.OpenSysML+dtExpression))
}

func (e *encoder) multiplicity(subject rdf.Term, owner string, mult *ast.Multiplicity) {
	if mult == nil {
		return
	}
	// The parser puts the single bound of `[n]` in Lower; the language reads
	// that as lower and upper both being n, so it is written as the upper bound
	// alone and the printer renders it back as `[n]`.
	if !mult.IsRange {
		e.expression(subject, e.sysml(pUpperBound), pUpperBound, owner, mult.Lower)
		return
	}
	e.expression(subject, e.sysml(pLowerBound), pLowerBound, owner, mult.Lower)
	e.expression(subject, e.sysml(pUpperBound), pUpperBound, owner, mult.Upper)
}

// reference renders a name reference as a link when it resolves to an element
// this document declares, and as the written name otherwise — a type from the
// standard library is a name, not an element of this graph.
func (e *encoder) reference(name *ast.QualifiedName) rdf.Term {
	if qualifiedText(name) == "" {
		return rdf.String("")
	}
	if decl, fqn, ok := e.referent(name); ok {
		return e.ids.subjectForNode(decl, fqn)
	}
	return rdf.String(qualifiedText(name))
}

// referent is the element reference links a name to, if any.
func (e *encoder) referent(name *ast.QualifiedName) (ast.Node, string, bool) {
	sym, ok := e.links[name]
	if !ok {
		sym, ok = e.res.PartSymbol(name, len(name.Parts)-1)
	}
	return e.linkedElement(name, sym, ok)
}

// edgeReference renders a transition or succession end from what the document
// walk bound it to; an end implied by position is no written reference.
func (e *encoder) edgeReference(name *ast.QualifiedName) rdf.Term {
	if qualifiedText(name) == "" {
		return rdf.String("")
	}
	sym, ok := e.links[name]
	if !ok {
		sym, ok = e.res.EndSymbol(name)
	}
	return e.linkOrText(name, sym, ok)
}

// linkOrText links a resolved name to the element it names here, else to the
// alias declared here it was written through, else carries it as written.
func (e *encoder) linkOrText(name *ast.QualifiedName, sym *symbols.Symbol, ok bool) rdf.Term {
	if decl, fqn, ok := e.linkedElement(name, sym, ok); ok {
		return e.ids.subjectForNode(decl, fqn)
	}
	// The quotes an unrestricted name needs are notation, added when it is
	// written back out.
	return rdf.String(qualifiedText(name))
}

// linkedElement is the element a resolved name links to: the one it names here,
// else the alias declared here it was written through.
func (e *encoder) linkedElement(name *ast.QualifiedName, sym *symbols.Symbol, ok bool) (ast.Node, string, bool) {
	if decl, fqn, ok := e.linked(sym, ok); ok {
		return decl, fqn, true
	}
	return e.linked(e.res.PartAlias(name, len(name.Parts)-1))
}

// linked is the declaration and qualified name of the element a symbol names,
// declared or effectively; a `first start` label or loop variable names none.
func (e *encoder) linked(sym *symbols.Symbol, ok bool) (ast.Node, string, bool) {
	if !ok || sym == nil {
		return nil, "", false
	}
	fqn, declared := e.fqn[sym.Decl]
	if !declared {
		return nil, "", false
	}
	if name, _ := declaredNameAndMembers(sym.Decl); name == "" && !sym.EffectiveName() {
		return nil, "", false
	}
	return sym.Decl, fqn, true
}

// text is the notation of a node as written, without the trivia its span runs
// on over.
func (e *encoder) text(node ast.Node) string {
	if node == nil {
		return ""
	}
	return strings.TrimSpace(e.src.code(node.Span()))
}

// headEnd is the offset of the `{` or `;` ending a usage's head, found after
// stepping over the head's own nodes so a value's braces are not mistaken for it.
func (e *encoder) headEnd(n *ast.Usage) int {
	span := n.Span()
	from := span.Offset
	for _, node := range headNodes(n) {
		if end := node.Span().End(); end > from && end <= span.End() {
			from = end
		}
	}
	tail := e.file.Text(source.Span{Offset: from, Len: span.End() - from})
	lx := lexer.New(source.New("head.sysml", []byte(tail)))
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Kind == lexer.LBrace || tok.Kind == lexer.Semicolon {
			return from + tok.Span.Offset
		}
	}
	return span.End()
}

// headNodes lists the non-nil nodes a usage's head is written from.
func headNodes(n *ast.Usage) []ast.Node {
	var nodes []ast.Node
	add := func(candidates ...ast.Node) {
		for _, node := range candidates {
			if node != nil {
				nodes = append(nodes, node)
			}
		}
	}
	add(n.Value)
	if n.Multiplicity != nil {
		add(n.Multiplicity)
	}
	if n.CrossFeature != nil {
		add(n.CrossFeature)
	}
	for _, prefix := range n.Prefixes {
		if prefix != nil && prefix.Type != nil {
			add(prefix.Type)
		}
	}
	for _, rel := range n.Relationships {
		if rel != nil {
			add(rel, rel.Target)
			if rel.Multiplicity != nil {
				add(rel.Multiplicity)
			}
		}
	}
	for _, end := range n.ConnectorEnds {
		if end != nil {
			add(end, end.Target, end.Reference)
			if end.Multiplicity != nil {
				add(end.Multiplicity)
			}
		}
	}
	if flow := n.FlowEnds; flow != nil {
		add(flow, flow.From, flow.To, flow.Payload)
		if flow.PayloadDecl != nil {
			add(flow.PayloadDecl)
		}
		if flow.PayloadMultiplicity != nil {
			add(flow.PayloadMultiplicity)
		}
	}
	return nodes
}

// rdfLimitationsNote is the remedy for a construct the RDF mapping does not
// represent, as docs/reference/rdf-mapping.md states it.
const rdfLimitationsNote = "save to .sysml or .kerml instead, which writes the source exactly; " +
	"see docs/reference/rdf-mapping.md § Limitations"

// nodeDescription names a construct the way the notation does — "part def",
// "substate member" — so an error about one prints no Go type name.
func nodeDescription(node ast.Node) string {
	switch n := node.(type) {
	case nil:
		return "declaration"
	case *ast.Definition:
		return n.Kind.String() + " def"
	case *ast.Usage:
		return n.Kind.String() + " usage"
	}
	return spacedWords(strings.TrimPrefix(fmt.Sprintf("%T", node), "*ast."))
}

// spacedWords turns a node type's name into lower-case words, so
// "SubstateMember" reads as "substate member".
func spacedWords(name string) string {
	if name == "" {
		return "declaration"
	}
	var b strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte(' ')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (e *encoder) where(node ast.Node) string {
	pos := e.file.Lines().PosAt(node.Span().Offset)
	return fmt.Sprintf("%s:%d:%d", e.file.Name(), pos.Line, pos.Col)
}

// isTypeFeatureMember reports whether a membership wrapper was declared
// `member` (KerML.xtext TypeFeatureMember).
func isTypeFeatureMember(member ast.Node) bool {
	m, ok := member.(*ast.Membership)
	return ok && m.IsTypeFeature
}

// unwrapMember returns the declaration inside a membership wrapper together
// with the visibility the wrapper declared.
func unwrapMember(member ast.Node) (ast.Node, ast.Visibility) {
	switch n := member.(type) {
	case *ast.Membership:
		if n.Member == nil {
			return nil, n.Visibility
		}
		return n.Member, n.Visibility
	case *ast.Import:
		return n, n.Visibility
	case *ast.Alias:
		return n, n.Visibility
	case *ast.RelationshipMember:
		return n, n.Visibility
	}
	return member, ast.VisibilityDefault
}

// referencesFeature reports whether a usage names an existing feature rather
// than declaring one of its own (`perform doIt;`).
func referencesFeature(n *ast.Usage) bool {
	for _, rel := range n.Relationships {
		if rel.Kind == ast.RelReferences && rel.Target != nil {
			return true
		}
	}
	return false
}

// prefixCarriedByGraph reports a prefix the graph already states structurally:
// a state's `entry`/`do`/`exit` by the subaction membership that owns the
// action, an `include` by the inclusion relationship.
func prefixCarriedByGraph(n *ast.Usage) bool {
	switch n.PrefixKeyword {
	case "entry", "do", "exit":
		return n.Kind == ast.UsageAction
	case "include":
		for _, rel := range n.Relationships {
			if rel.Kind == ast.RelIncludes {
				return true
			}
		}
	}
	return false
}

// declaredPrefixes is the `#M` annotations written ahead of a declaration.
func declaredPrefixes(node ast.Node) []*ast.PrefixMetadata {
	switch n := node.(type) {
	case *ast.Package:
		return n.Prefixes
	case *ast.Namespace:
		return n.Prefixes
	case *ast.Dependency:
		return n.Prefixes
	case *ast.Definition:
		return n.Prefixes
	case *ast.Usage:
		return n.Prefixes
	case *ast.SubjectMember:
		return n.Prefixes
	case *ast.AssumeMember:
		return n.Prefixes
	case *ast.RequireMember:
		return n.Prefixes
	}
	return nil
}

// declaredNameAndMembers returns the name a declaration introduces and the
// members it owns, for the node kinds that have either.
func declaredNameAndMembers(node ast.Node) (string, []ast.Node) {
	if name, members, ok := behaviorNameAndMembers(node); ok {
		return name, members
	}
	switch n := node.(type) {
	case *ast.Package:
		return n.Ident.Name, n.Members
	case *ast.Namespace:
		return n.Ident.Name, n.Members
	case *ast.Definition:
		return n.Ident.Name, n.Members
	case *ast.Usage:
		if shorthandRelationship(n) {
			return "", bodyMembers(n)
		}
		return n.Ident.Name, bodyMembers(n)
	case *ast.Import:
		return "", n.Body
	case *ast.Alias:
		return n.Ident.Name, n.Body
	case *ast.Dependency:
		return n.Ident.Name, n.Body
	case *ast.RelationshipMember:
		return n.Ident.Name, n.Members
	case *ast.MultiplicityDecl:
		return n.Ident.Name, n.Members
	case *ast.ConstraintMember:
		return n.Name, n.Body
	case *ast.AssumeMember:
		return n.Ident.Name, n.Body
	case *ast.RequireMember:
		return n.Ident.Name, n.Body
	case *ast.SubjectMember:
		return n.Ident.Name, n.Body
	case *ast.PrefixMetadata:
		return n.Ident.Name, n.Body
	case *ast.Comment:
		return n.Ident.Name, nil
	case *ast.Documentation:
		return n.Ident.Name, nil
	case *ast.TextualRepresentation:
		return n.Ident.Name, nil
	}
	return "", nil
}

// qualify builds the qualified name of a member. An unnamed declaration is
// addressed by its position in its owner, which keeps every element in the
// graph identifiable and the mapping reversible.
func qualify(owner, name string, index int) string {
	if name == "" {
		name = fmt.Sprintf("@%d", index)
	}
	if owner == "" {
		return name
	}
	return owner + "::" + name
}

func qualifiedText(name *ast.QualifiedName) string {
	if name == nil {
		return ""
	}
	parts := make([]string, 0, len(name.Parts))
	for _, part := range name.Parts {
		parts = append(parts, part.Text)
	}
	out := strings.Join(parts, "::")
	if name.Global {
		return "$::" + out
	}
	return out
}

// commentBody strips the /* */ delimiters from a comment token, leaving the
// text the printer re-wraps.
func commentBody(raw string) string {
	raw = strings.TrimPrefix(raw, "/*")
	raw = strings.TrimSuffix(raw, "*/")
	return raw
}
