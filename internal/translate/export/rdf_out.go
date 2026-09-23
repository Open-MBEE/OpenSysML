package export

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
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
	pOwnedMemberParameter      = "ownedMemberParameter"
	pFeatureWithValue          = "featureWithValue"
	pOwnedReferenceSubsetting  = "ownedReferenceSubsetting"
	pOwnedSubsetting           = "ownedSubsetting"
	pOwnedSpecialization       = "ownedSpecialization"
	pReferencingFeature        = "referencingFeature"
	pSubsettingFeature         = "subsettingFeature"
	pSpecific                  = "specific"
	pReferencedFeature         = "referencedFeature"
	pSubsettedFeature          = "subsettedFeature"
	pGeneral                   = "general"
	pRelatedElement            = "relatedElement"
	pOwningFeature             = "owningFeature"
	pVariant                   = "variant"
	pVariantMembership         = "variantMembership"
	pOwnedVariantUsage         = "ownedVariantUsage"
	pOwnedImport               = "ownedImport"
	pImportOwningNamespace     = "importOwningNamespace"
	pDirection                 = "direction"
	pIsImplied                 = "isImplied"
	pLowerBound                = "lowerBound"
	pUpperBound                = "upperBound"
	pValue                     = "value"
	pIsDefault                 = "isDefault"
	pIsInitial                 = "isInitial"
	pIsEnd                     = "isEnd"
	pName                      = "name"
	pReferences                = "references"
	pConnectorEnd              = "connectorEnd"
	pRelatedFeature            = "relatedFeature"
	pChainingFeature           = "chainingFeature"
	pOwnedEndFeature           = "ownedEndFeature"
	pImportedNamespace         = "importedNamespace"
	pImportedMembership        = "importedMembership"
	pAliasFor                  = "aliasedElement" // an older mapping's alias target, read only
	pMemberName                = "memberName"
	pMemberShortName           = "memberShortName"
	pClient                    = "client"
	pSupplier                  = "supplier"
	pBody                      = "body"
	pLanguage                  = "language"
	pLocale                    = "locale"
	pAnnotatedElement          = "annotatedElement"
	pIsImportAll               = "isImportAll"
	pSourceFeature             = "sourceFeature"
	pTargetFeature             = "targetFeature"
	pSource                    = "source"
	pTarget                    = "target"
	pPortionKind               = "portionKind"
)

// Property names in the OpenSysML extension namespace: declaration order,
// body presence, and the source text of the constructs whose head this
// mapping keeps verbatim (see the package doc).
const (
	xMemberIndex    = "memberIndex"
	xHasBody        = "hasBody"
	xSourceText     = "sourceText"
	xSourceTail     = "sourceTail"
	xSourceLanguage = "sourceLanguage"
	xFilter         = "filter"
	// xExpose is only read: an older graph flags an expose on an abstract sysml:Import.
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
	xConjugatedTyping     = "conjugatedTyping"
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
	mOwningMembership          = "OwningMembership"
	mFeatureMembership         = "FeatureMembership"
	mVariantMembership         = "VariantMembership"
	mFeatureValue              = "FeatureValue"
	mFeatureChaining           = "FeatureChaining"
	mParameterMembership       = "ParameterMembership"
	mReturnParameterMembership = "ReturnParameterMembership"
	mFeature                   = "Feature"
	mEndFeatureMembership      = "EndFeatureMembership"
	// The membership a body owns its result expression through, which states
	// the expression as sysml:ownedResultExpression.
	mResultExpressionMembership = "ResultExpressionMembership"
	pOwnedResultExpression      = "ownedResultExpression"
	mDocumentation              = "Documentation"
	// The concrete imports and exposes; sysml:Import and sysml:Expose are
	// abstract, and an older graph's sysml:Import with sysx:isExpose is only read.
	mImport           = "Import"
	mNamespaceImport  = "NamespaceImport"
	mMembershipImport = "MembershipImport"
	mPackage          = "Package"
	mLibraryPackage   = "LibraryPackage"
	pIsStandard       = "isStandard"
	pIsRecursive      = "isRecursive"
	mNamespaceExpose  = "NamespaceExpose"
	mMembershipExpose = "MembershipExpose"
	// The elements a filter package materializes under its import.
	filterPackageSuffix    = "_fp"
	filterImportSuffix     = "_im"
	filterMembershipSuffix = "_efm"
)

// Metaclass names for the constructs that have no SysML metaclass of their own
// in this mapping.
const (
	mAlias             = "Alias"
	mFilter            = "FilterMember"
	mMultiplicity      = "MultiplicityDeclaration"
	mMultiplicityClass = "Multiplicity"
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
	return ToRDFWith(file, root, IDQualifiedName)
}

// ToRDFWith is ToRDF with the id form the caller asks for.
func ToRDFWith(file *source.SourceFile, root *ast.RootNamespace, form IDForm) (*rdf.Graph, error) {
	e, err := encodeDocument(file, root, "", form)
	if err != nil {
		return nil, err
	}
	return e.graph, nil
}

// encodeDocument converts a parsed document, returning the encoder that holds
// the graph and where in file each element was written; library is as for analyzeDocument.
func encodeDocument(file *source.SourceFile, root *ast.RootNamespace, library string, form IDForm) (*encoder, error) {
	if file == nil || root == nil {
		return nil, &UnsupportedError{What: "an empty document", Note: "nothing to convert"}
	}
	e, err := newEncoder(file, root, library, form)
	if err != nil {
		return nil, err
	}
	e.src = newAuthoredSource(file)
	if err := e.encode(root.Members, "", rdf.Term{}); err != nil {
		return nil, err
	}
	e.importedMemberships()
	e.materializeNormative()
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
func newEncoder(file *source.SourceFile, root *ast.RootNamespace, library string, form IDForm) (*encoder, error) {
	res, model := analyzeDocument(file, root, library)
	ids, err := documentIdentity(file.Name(), res, model, form)
	if err != nil {
		return nil, err
	}
	e := &encoder{
		file:           file,
		graph:          rdf.NewGraph(),
		res:            res,
		declared:       map[string]bool{},
		metadataBodies: map[string]bool{},
		performed:      map[ast.Node]bool{},
		effects:        map[ast.Node]bool{},
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
	// performed holds the action usages a state's entry/do/exit or a transition's
	// effect declares: each is a PerformActionUsage (SysML.xtext PerformedActionUsage).
	performed map[ast.Node]bool
	// effects holds the members of a transition's `do` effect, which a
	// TransitionFeatureMembership of kind effect owns.
	effects map[ast.Node]bool
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
	// membershipImports are the membership imports, whose imported membership
	// is written once every membership is minted.
	membershipImports []membershipImport
}

// membershipImport is a membership import's subject and the name it imports.
type membershipImport struct {
	subject rdf.Term
	name    *ast.QualifiedName
}

// importedMemberships writes each membership import's sysml:importedMembership:
// an alias written through, else the member's minted membership, else the name.
func (e *encoder) importedMemberships() {
	for _, imp := range e.membershipImports {
		e.graph.Add(imp.subject, e.sysml(pImportedMembership), e.importedMembership(imp.name))
	}
}

// importMetaclass is the concrete class of an import, or of an expose.
func importMetaclass(imported, exposed string, expose bool) string {
	if expose {
		return exposed
	}
	return imported
}

// importTarget types the import subject by what it imports and states the
// target: a namespace directly, a membership once the walk has minted it.
func (e *encoder) importTarget(subject rdf.Term, head func(rdf.Term), n *ast.Import) {
	if n.Kind == ast.ImportNamespace {
		head(rdf.SysMLTerm(importMetaclass(mNamespaceImport, mNamespaceExpose, n.IsExpose)))
		e.graph.Add(subject, e.sysml(pImportedNamespace), e.reference(n.Imported))
		return
	}
	head(rdf.SysMLTerm(importMetaclass(mMembershipImport, mMembershipExpose, n.IsExpose)))
	e.membershipImports = append(e.membershipImports, membershipImport{subject, n.Imported})
}

// filterPackage materializes the filter package `import X::*[c]` stands for
// (SysML.xtext FilterPackage): the import imports an unnamed Package it owns,
// which imports X and owns a private ElementFilterMembership whose condition is
// c. The collapsed sysx:filter on the import still names the condition.
func (e *encoder) filterPackage(subject rdf.Term, within string, n *ast.Import) error {
	pkg := e.ids.minted(rdf.RelationshipIRI(subject, filterPackageSuffix), subject, filterPackageSuffix)
	inner := e.ids.minted(rdf.RelationshipIRI(pkg, filterImportSuffix), pkg, filterImportSuffix)
	membership := e.ids.minted(rdf.RelationshipIRI(pkg, filterMembershipSuffix), pkg, filterMembershipSuffix)
	condition := e.ids.mintedNode(rdf.ExpressionIRI(subject, xFilter), subject, xFilter)
	outerClass := e.metaclassOf(subject)

	e.typed(pkg, mPackage)
	e.graph.Add(pkg, e.sysml(pElementID), rdf.String(rdf.LocalName(pkg.Value)))
	e.graph.Add(subject, e.sysml(pImportedNamespace), pkg)
	e.relationshipOwnership(pkg, subject, outerClass, mPackage)

	if n.Kind == ast.ImportNamespace {
		e.typed(inner, mNamespaceImport)
		e.graph.Add(inner, e.sysml(pImportedNamespace), e.reference(n.Imported))
	} else {
		e.typed(inner, mMembershipImport)
		e.membershipImports = append(e.membershipImports, membershipImport{inner, n.Imported})
	}
	e.graph.Add(inner, e.sysml(pElementID), rdf.String(rdf.LocalName(inner.Value)))
	e.graph.Add(inner, e.sysml(pImportOwningNamespace), pkg)
	e.flags(inner, []boolProperty{{pIsRecursive, n.IsRecursive}})
	e.relationshipOwnership(inner, pkg, mPackage, mNamespaceImport)
	e.graph.Add(pkg, e.sysml(pOwnedImport), inner)

	e.graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
	e.graph.Add(subject, e.sysx(xFilter), condition)
	if err := e.expressionNode(condition, within, n.FilterExpr); err != nil {
		return err
	}
	e.emitMembershipCore(membership, condition, pkg, mElementFilterMembership, true)
	e.graph.Add(membership, e.sysml(pVisibility), rdf.String("private"))
	e.graph.Add(membership, e.sysml(pCondition), condition)
	e.graph.Add(pkg, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(pkg, e.sysml(pOwnedMembership), membership)
	return nil
}

// importedMembership is the membership a membership import names: the one
// owning the alias written, else the one owning the member the name resolves
// to; the name itself where neither is an element of the graph.
func (e *encoder) importedMembership(name *ast.QualifiedName) rdf.Term {
	if qualifiedText(name) == "" {
		return rdf.String("")
	}
	decl, fqn, ok := e.linked(e.res.PartAlias(name, len(name.Parts)-1))
	if !ok {
		decl, fqn, ok = e.referent(name)
	}
	if ok {
		// An alias is itself the Membership an import of it imports.
		if _, isAlias := decl.(*ast.Alias); isAlias {
			return e.ids.subjectForNode(decl, fqn)
		}
		membership := e.ids.owningMembershipOf(decl, e.ids.subjectForNode(decl, fqn))
		if _, minted := e.subjects[membership.Value]; minted || e.ids.normativeMembership(decl) {
			return membership
		}
	}
	return rdf.String(qualifiedText(name))
}

// claimLibrary reserves the IRIs of a library element the document links to,
// and of its owning membership, so no element declared here lands on them.
func (e *encoder) claimLibrary(node ast.Node, fqn string) {
	subject := e.ids.subjectForNode(node, fqn)
	claims := []struct{ iri, standsFor string }{{subject.Value, fqn}}
	if e.ids.normativeMembership(node) {
		claims = append(claims, struct{ iri, standsFor string }{
			e.ids.owningMembershipOf(node, subject).Value, fqn + "'s owning membership",
		})
	}
	for _, c := range claims {
		if prior, taken := e.claim(c.iri, c.standsFor); taken && e.idErr == nil {
			e.idErr = &UnsupportedError{
				What: fmt.Sprintf("the reference to %s", fqn),
				Note: fmt.Sprintf("the id the norm fixes for it lands on the same IRI as %s, and merging two elements into one subject would be a different model", prior),
			}
		}
	}
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
			lines: regions[i], inline: inline, typeFeature: isTypeFeatureMember(member), last: i == len(kept)-1}
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
	// last marks the body's closing member, which a constraint body writes back
	// bare as its result expression.
	last bool
	// membershipClass, when set, is the metaclass the minted membership is
	// typed with instead of FeatureMembership — the SubjectMembership of a
	// `subject` member or the RequirementConstraintMembership of a `require`
	// member — and membershipExtra adds the metaclass's own ends to it.
	membershipClass string
	membershipExtra func(rdf.Term)
	// local is the subject of a declaration inside an expression body, which has
	// no qualified name and no owning namespace: the body positions it.
	local rdf.Term
}

func (e *encoder) head(subject rdf.Term, h memberHead) {
	node, visibility, fqn, ownerTerm, index, metaclass, lines, inline :=
		h.node, h.visibility, h.fqn, h.owner, h.index, h.metaclass, h.lines, h.inline
	e.graph.Add(subject, rdf.IRI(rdf.RDFType), metaclass)
	if h.local.Value == "" {
		e.graph.Add(subject, e.sysml(pQualifiedName), rdf.String(fqn))
		e.offsets[subject.Value] = node.Span().Offset
	}
	// The id an API reader addresses the element by, which is the id its own
	// IRI ends in, so the two cannot disagree.
	e.graph.Add(subject, e.sysml(pElementID), rdf.String(rdf.LocalName(subject.Value)))
	e.graph.Add(subject, e.sysx(xMemberIndex), rdf.Int(index))
	if !inline {
		e.regions[subject] = lines
	}
	if language := languageName(e.file.Kind()); ownerTerm.Value == "" && h.local.Value == "" && language != "" {
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
		membership = e.owningMembership(node, subject, ownerTerm, fqn, ast.IsExpression(node), e.variantMember(node, ownerTerm), crossing || h.typeFeature, h.membershipClass, h.membershipExtra)
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
// owner is the qualified name of the namespace the member is declared in, or of
// the member whose expression body declares it when h.local names it.
func (e *encoder) encodeMember(h memberHead, owner string) error {
	node, ownerTerm, index := h.node, h.owner, h.index
	name, _ := declaredNameAndMembers(node)
	fqn := qualify(owner, name, index)
	subject := h.local
	local := subject.Value != ""
	// within is the member the expressions written here are part of.
	within := fqn
	if local {
		if err := e.localShape(node); err != nil {
			return err
		}
		fqn, within = "", owner
	} else {
		var err error
		if subject, err = e.mint(node, fqn); err != nil {
			return err
		}
	}
	if e.effects[node] {
		h.membershipClass = mTransitionFeatureMembership
		h.membershipExtra = func(membership rdf.Term) {
			e.graph.Add(membership, e.sysml(pKind), rdf.String("effect"))
			e.graph.Add(membership, e.sysml(pTransitionFeature), subject)
			e.graph.Add(ownerTerm, e.sysml(pEffectAction), subject)
		}
	}
	// A bare expression among a body's members is the result the body computes.
	result := ast.IsExpression(node)
	head := func(metaclass rdf.Term) {
		h.fqn, h.metaclass = fqn, metaclass
		e.head(subject, h)
	}
	// A body's members are those of the namespace the member declares; a local
	// member's body declares into the expression body it lies in.
	members := func(members []ast.Node) error {
		if local {
			return e.bodyDeclarations(subject, subject, within, nil, members)
		}
		return e.encode(members, fqn, subject)
	}

	switch n := node.(type) {
	case *ast.Package:
		// `library package` is a LibraryPackage (SysML 8.3.13.3), `standard` its isStandard.
		if n.IsLibrary {
			head(rdf.SysMLTerm(mLibraryPackage))
		} else {
			head(rdf.SysMLTerm(mPackage))
		}
		e.ident(subject, n.Ident)
		e.flags(subject, []boolProperty{{pIsStandard, n.IsStandard}})
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Members); err != nil {
			return err
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return members(n.Members)

	case *ast.Namespace:
		head(rdf.SysMLTerm("Namespace"))
		e.ident(subject, n.Ident)
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Members); err != nil {
			return err
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return members(n.Members)

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
			{"isIndividual", n.IsIndividual},
			{"isParallel", n.IsParallel},
		})
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Members); err != nil {
			return err
		}
		e.relationships(subject, owner, n.Relationships)
		if n.Kind == ast.DefPort {
			// Every port definition owns its conjugate (SysML.xtext
			// ConjugatedPortDefinitionMember), which the `~P` typing of a
			// conjugated port usage resolves to.
			e.conjugatedPortDefinition(subject, fqn, n)
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return members(n.Members)

	case *ast.Usage:
		inBody := !local && e.metadataBodies[owner]
		metaclass, ok := usageMetaclassOf(n, inBody)
		if e.performed[n] {
			metaclass = mPerform
		}
		if !ok {
			return &UnsupportedError{What: fmt.Sprintf("usage kind %q at %s", n.Kind, e.where(n))}
		}
		// An actor, stakeholder or objective member is a parameter its
		// membership owns (SysML.xtext ActorMember & co.): the parameter kind
		// is the membership's metaclass, and the parameter itself is a usage.
		parameterClass, parameterEnd := parameterMembership(n.Kind)
		if n.IsResult {
			// A `return` member is the result parameter its ReturnParameterMembership
			// owns (SysML.xtext ReturnParameterMember).
			parameterClass, parameterEnd = mReturnParameterMembership, pOwnedMemberParameter
		}
		if parameterClass != "" {
			h.membershipClass = parameterClass
			h.membershipExtra = func(membership rdf.Term) {
				e.graph.Add(membership, e.sysml(parameterEnd), subject)
				e.graph.Add(membership, e.sysml(pOwnedMemberParameter), subject)
			}
		}
		// A `render`/`frame` member likewise is a RenderingUsage/ConcernUsage its
		// membership owns (SysML.xtext ViewRenderingMember, FramedConcernMember).
		if class, end, kind := memberOwnedUsage(n.Kind); class != "" {
			h.membershipClass = class
			h.membershipExtra = func(membership rdf.Term) {
				e.graph.Add(membership, e.sysml(end), subject)
				if kind != "" {
					e.graph.Add(membership, e.sysml(pKind), rdf.String(kind))
					e.graph.Add(membership, e.sysml(pOwnedConstraint), subject)
				}
			}
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
			{"isAccept", n.IsAccept},
			{"isResult", n.IsResult},
			{"isParallel", n.IsParallel},
		})
		// `: ~P` types the port by P's conjugate; the usage owns no Conjugation
		// of its own, so Type::isConjugated stays false on it.
		if n.HasConjugatedTyping() {
			e.graph.Add(subject, e.sysx(xConjugatedTyping), rdf.Bool(true))
		}
		if err := e.prefixes(subject, fqn, n.Prefixes, bodyMembers(n)); err != nil {
			return err
		}
		if keyword := directionKeyword(n.Direction); keyword != "" {
			e.graph.Add(subject, e.sysml(pDirection), rdf.String(keyword))
		} else if parameterClass != "" {
			// A parameter member is read `in` when it states no direction of its own.
			e.graph.Add(subject, e.sysml(pDirection), rdf.String("in"))
		}
		if portion := portionKeyword(n.Portion); portion != "" {
			e.graph.Add(subject, e.sysml(pPortionKind), rdf.String(portion))
		}
		if n.IsEnd {
			relationships := make([]*ast.Relationship, 0, len(n.Relationships))
			for _, rel := range n.Relationships {
				if rel != nil && rel.Kind == ast.RelReferences && rel.Target != nil {
					if err := e.endReferences(subject, rel.Target); err != nil {
						return err
					}
					continue
				}
				relationships = append(relationships, rel)
			}
			e.relationships(subject, owner, relationships)
		} else {
			e.relationships(subject, owner, n.Relationships)
		}
		if err := e.multiplicity(subject, within, n.Multiplicity); err != nil {
			return err
		}
		if err := e.crossFeature(subject, fqn, n); err != nil {
			return err
		}
		if err := e.featureValue(subject, within, n.Value, n.ValueIsDefault, n.ValueIsInitial); err != nil {
			return err
		}
		// A `satisfy R by s` head states its subject through a SubjectMembership
		// holding an unnamed reference to s (SysML.xtext SatisfyRequirementUsage).
		if err := e.subjectParameters(subject, within, n.Relationships); err != nil {
			return err
		}
		// A declaration head that binds ends (connect/bind/flow/succession),
		// a transition, an accept action or a satisfy usage states its ends
		// and form structurally, since the properties above do not.
		if verbatimUsage(n) {
			if err := e.bindingEnds(subject, within, n); err != nil {
				return err
			}
			e.endForm(subject, n)
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		if !local && (inBody || n.Kind == ast.UsageMetadata) {
			e.metadataBodies[fqn] = true
		}
		return members(bodyMembers(n))

	case *ast.Import:
		if n.FilterExpr != nil {
			// `import X::*[c]` is a NamespaceImport of a filter package it owns
			// (SysML.xtext FilterPackage): the package imports X and filters it.
			head(rdf.SysMLTerm(importMetaclass(mNamespaceImport, mNamespaceExpose, n.IsExpose)))
			e.flags(subject, []boolProperty{{pIsImportAll, n.IsAll}})
			if err := e.filterPackage(subject, within, n); err != nil {
				return err
			}
		} else {
			// A membership import names a membership, minted once the walk reaches
			// the member, so it is written after the walk.
			e.importTarget(subject, head, n)
			e.flags(subject, []boolProperty{
				{pIsImportAll, n.IsAll},
				{pIsRecursive, n.IsRecursive},
			})
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return members(n.Body)

	case *ast.Alias:
		// An alias is a Membership naming its member (KerML §8.3.2.5, KerML.xtext AliasMember).
		head(rdf.SysMLTerm(mMembership))
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String("alias"))
		if n.Ident.Name != "" {
			e.graph.Add(subject, e.sysml(pMemberName), rdf.String(n.Ident.Name))
		}
		if n.Ident.ShortName != "" {
			e.graph.Add(subject, e.sysml(pMemberShortName), rdf.String(n.Ident.ShortName))
		}
		e.graph.Add(subject, e.sysml(pMemberElement), e.reference(n.For))
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return members(n.Body)

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
		return members(n.Members)

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
		return members(n.Body)

	case *ast.Comment:
		head(rdf.SysMLTerm("Comment"))
		e.ident(subject, n.Ident)
		for _, about := range n.About {
			e.graph.Add(subject, e.sysml(pAnnotatedElement), e.reference(about))
		}
		if n.Locale != "" {
			e.graph.Add(subject, e.sysml(pLocale), rdf.String(source.StringValue(n.Locale)))
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
		e.graph.Add(subject, e.sysml(pLanguage), rdf.String(source.StringValue(n.Language)))
		e.graph.Add(subject, e.sysml(pBody), rdf.String(commentBody(e.src.slice(n.BodySpan))))
		return nil

	case *ast.MultiplicityDecl:
		// The declaration is the Multiplicity itself: a MultiplicityRange when it
		// states bounds, a plain Multiplicity when it only subsets another.
		if n.Range != nil {
			head(rdf.SysMLTerm(mMultiplicityRange))
		} else {
			head(rdf.SysMLTerm(mMultiplicityClass))
		}
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String("multiplicity"))
		e.ident(subject, n.Ident)
		if err := e.multiplicityBounds(subject, subject, within, n.Range); err != nil {
			return err
		}
		// A MultiplicitySubset states its bounds by subsetting, not as a range.
		if n.Subsets != nil {
			e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelSubsets]),
				e.reference(n.Subsets))
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return members(n.Members)

	case *ast.ConstraintMember:
		// A bare condition is an expression element of its own; the one closing
		// the body is the result the constraint evaluates (SysML.xtext
		// ownedResultExpression), owned through a ResultExpressionMembership.
		if n.Keyword == "" && n.Name == "" && n.Body == nil {
			if h.last {
				h.membershipClass = mResultExpressionMembership
			}
			head(rdf.SysMLTerm(expressionMetaclass(n.Expression)))
			e.graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
			if err := e.expressionStructure(subject, within, n.Expression); err != nil {
				return err
			}
			if h.last {
				if membership, ok := e.graph.Object(subject, rdf.SysML+pOwningMembership); ok {
					e.graph.Add(membership, e.sysml(pOwnedResultExpression), subject)
				}
			}
			return nil
		}
		// A keyworded condition is an AssertConstraintUsage: `assert R` states
		// the constraint it references, `assert constraint c { … }` declares a
		// nested one. `assume` has no metaclass of its own and keeps the keyword.
		head(rdf.SysMLTerm(mAssertConstraintUsage))
		if n.Name != "" {
			e.graph.Add(subject, e.sysml(pDeclaredName), rdf.String(n.Name))
		}
		if n.Keyword == "assume" {
			e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String("assume"))
		}
		e.flags(subject, []boolProperty{{"isNegated", n.IsNegated}})
		reference, _ := n.Expression.(*ast.QualifiedName)
		var expr ast.Node
		if reference == nil {
			expr = n.Expression
		}
		return e.condition(subject, fqn, within, expr, reference, n.Body != nil, n.Body)

	case *ast.AssumeMember:
		// An `assume` member owns its constraint usage through a
		// RequirementConstraintMembership whose kind is an assumption.
		h.membershipClass = mRequirementConstraintMembership
		h.membershipExtra = func(membership rdf.Term) {
			e.graph.Add(membership, e.sysml(pKind), rdf.String("assumption"))
			e.graph.Add(membership, e.sysml(pOwnedConstraint), subject)
		}
		head(rdf.SysMLTerm(mConstraintUsage))
		return e.requirementCondition(subject, fqn, within, requirementConditionDecl{
			prefixes: n.Prefixes, ident: n.Ident, relationships: n.Relationships, multiplicity: n.Multiplicity,
			value: n.Value, isDefault: n.ValueIsDefault, isInitial: n.ValueIsInitial,
			expression: n.Expression, reference: n.Reference, hasBody: n.HasBody, body: n.Body,
		})

	case *ast.RequireMember:
		h.membershipClass = mRequirementConstraintMembership
		h.membershipExtra = func(membership rdf.Term) {
			e.graph.Add(membership, e.sysml(pKind), rdf.String("requirement"))
			e.graph.Add(membership, e.sysml(pOwnedConstraint), subject)
		}
		head(rdf.SysMLTerm(mConstraintUsage))
		return e.requirementCondition(subject, fqn, within, requirementConditionDecl{
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
		// A subject parameter is the ReferenceUsage its SubjectMembership owns
		// (SysML v2 8.2.2.16 SysML.xtext SubjectMember), read in by default.
		h.membershipClass = mSubjectMembership
		h.membershipExtra = func(membership rdf.Term) {
			e.graph.Add(membership, e.sysml(pOwnedSubjectParameter), subject)
			e.graph.Add(membership, e.sysml(pOwnedMemberParameter), subject)
		}
		head(rdf.SysMLTerm(mReferenceUsage))
		e.ident(subject, n.Ident)
		e.graph.Add(subject, e.sysml(pDirection), rdf.String("in"))
		if n.TypeRef != nil {
			e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelTyping]), e.reference(n.TypeRef))
		}
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Body); err != nil {
			return err
		}
		e.relationships(subject, owner, n.Relationships)
		if err := e.multiplicity(subject, within, n.Multiplicity); err != nil {
			return err
		}
		if err := e.featureValue(subject, within, n.BindingExpr, n.ValueIsDefault, n.ValueIsInitial); err != nil {
			return err
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Body, fqn, subject)

	case *ast.FilterMember:
		// A `filter <cond>` member is an ElementFilterMembership owning the
		// condition expression (KerML ElementFilterMembership::condition).
		head(rdf.SysMLTerm(mElementFilterMembership))
		return e.expression(subject, e.sysml(pCondition), "filter", within, n.Condition)

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
		if err := e.expressionStructure(subject, within, node); err != nil {
			return err
		}
		if membership, ok := e.graph.Object(subject, rdf.SysML+pOwningMembership); ok {
			e.graph.Add(membership, e.sysml(pOwnedResultExpression), subject)
		}
		return nil
	}
	// A branch of `if` is an ActionUsage parameter the conditional owns through a
	// ParameterMembership (SysML.xtext ActionBodyParameterMember).
	if _, isBranch := node.(*ast.IfBranchNode); isBranch {
		h.membershipClass = mParameterMembership
		h.membershipExtra = func(membership rdf.Term) {
			e.graph.Add(membership, e.sysml(pOwnedMemberParameter), subject)
		}
	}
	// A behavioral node — a control node, statement, loop, conditional, state or
	// transition — is mapped by the behavior half of this encoder.
	if handled, err := e.encodeBehavior(node, head, subject, fqn, within, index); handled {
		return err
	}
	return &UnsupportedError{
		What: fmt.Sprintf("the %s at %s", nodeDescription(node), e.where(node)),
		Note: rdfLimitationsNote,
	}
}

// localShape refuses a declaration an expression body cannot hold: one whose parts
// (a prefix, a cross feature, an annotation) need a qualified name to be minted under.
func (e *encoder) localShape(node ast.Node) error {
	unsupported := func(note string) error {
		return &UnsupportedError{
			What: fmt.Sprintf("the %s at %s", nodeDescription(node), e.where(node)),
			Note: note + " inside an expression body; " + rdfLimitationsNote,
		}
	}
	if len(declaredPrefixes(node)) > 0 {
		return unsupported("a `#M` prefix annotation has no qualified name to be minted under")
	}
	switch n := node.(type) {
	case *ast.Usage:
		if n.CrossFeature != nil {
			return unsupported("an end's cross feature has no qualified name to be minted under")
		}
		return nil
	case *ast.PrefixMetadata:
		return unsupported("a `@M` annotation has no qualified name to be minted under")
	case *ast.Package, *ast.Namespace, *ast.Definition, *ast.Import, *ast.Alias,
		*ast.RelationshipMember, *ast.Dependency, *ast.Comment, *ast.Documentation,
		*ast.TextualRepresentation, *ast.MultiplicityDecl, *ast.ErrorNode:
		return nil
	}
	return unsupported("only a namespace, type, feature, relationship or annotation declaration is mapped")
}

// owningMembership wires a member to its owner the way the abstract syntax does,
// returning the membership minted between them, or the empty term when no
// membership stands between the two. The API's payloads reach a member through
// its membership, so a compact owner triple alone leaves a client walking down
// from a root with nothing to follow. result marks a body's result expression,
// which a ResultExpressionMembership owns; variant a usage a VariantMembership
// owns; plain a feature its type owns through a plain OwningMembership rather
// than a FeatureMembership: an end's cross feature, or a KerML `member` feature.
func (e *encoder) owningMembership(node ast.Node, member, owner rdf.Term, memberFQN string, result, variant, plain bool, membershipClass string, membershipExtra func(rdf.Term)) rdf.Term {
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
	case isRelationship(memberClass) &&
		(ontology.IsAncestorOrSelf(memberClass, "Import") || ontology.IsAncestorOrSelf(memberClass, "Membership")):
		// A namespace owns an import or a membership-family member — an alias —
		// as an owned relationship, with no membership between; every other
		// declared relationship is a member, owned through an OwningMembership.
		e.graph.Add(member, e.sysml(pOwner), owner)
		e.graph.Add(member, e.sysml(pOwningRelatedElement), owner)
		e.graph.Add(owner, e.sysml(pOwnedRelationship), member)
		if ontology.IsAncestorOrSelf(memberClass, "Import") {
			e.graph.Add(member, e.sysml(pImportOwningNamespace), owner)
			e.graph.Add(owner, e.sysml(pOwnedImport), member)
		} else {
			e.graph.Add(member, e.sysml(pMembershipOwningNamespace), owner)
			e.graph.Add(owner, e.sysml(pOwnedMembership), member)
		}
		return rdf.Term{}
	}
	// A type owns a feature through a FeatureMembership, which is the membership
	// the API's payloads carry for it; anything else, a metadata usage included,
	// through an OwningMembership.
	feature := ontology.IsAncestorOrSelf(memberClass, mFeature) && isType(ownerClass) && !metadata
	membership := e.ids.owningMembershipOf(node, member)
	// The membership shares the element namespace, so its IRI is reserved too.
	if prior, taken := e.claim(membership.Value, memberFQN+"'s owning membership"); taken && e.idErr == nil {
		e.idErr = &UnsupportedError{
			What: fmt.Sprintf("the owning membership of %s", memberFQN),
			Note: fmt.Sprintf("its id lands on the same IRI as %s, and merging two elements into one subject would be a different model", prior),
		}
	}
	// A variant is a member of its variation, not a feature of it: the metamodel
	// owns it through a VariantMembership, which is an OwningMembership. An end's
	// cross feature (KerML.xtext OwnedCrossingFeatureMember) and a `member`
	// feature (KerML.xtext TypeFeatureMember) are owned the same way.
	if variant || plain {
		feature = false
	}
	metaclass := mOwningMembership
	switch {
	case membershipClass != "":
		metaclass = membershipClass
	case result:
		metaclass = mResultExpressionMembership
	case variant:
		metaclass = mVariantMembership
	case feature:
		metaclass = mFeatureMembership
	}
	e.emitMembershipCore(membership, member, owner, metaclass, !isRelationship(ownerClass))
	if membershipExtra != nil {
		membershipExtra(membership)
	}
	// Only a namespace has members; a relationship owner just owns the membership.
	if !isRelationship(ownerClass) {
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

// emitMembershipCore writes the shared ownership triples for a membership.
func (e *encoder) emitMembershipCore(membership, member, owner rdf.Term, metaclass string, namespace bool) {
	e.graph.Add(member, e.sysml(pOwner), owner)
	e.graph.Add(member, e.sysml(pOwningRelationship), membership)
	e.graph.Add(member, e.sysml(pOwningMembership), membership)
	e.graph.Add(membership, rdf.IRI(rdf.RDFType), e.sysml(metaclass))
	e.graph.Add(membership, e.sysml(pElementID), rdf.String(rdf.LocalName(membership.Value)))
	e.graph.Add(membership, e.sysml(pOwner), owner)
	e.graph.Add(membership, e.sysml(pMemberElement), member)
	e.graph.Add(membership, e.sysml(pOwnedMemberElement), member)
	e.graph.Add(membership, e.sysml(pOwnedRelatedElement), member)
	e.graph.Add(membership, e.sysml(pOwningRelatedElement), owner)
	if namespace {
		e.graph.Add(membership, e.sysml(pMembershipOwningNamespace), owner)
	}
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
	e.graph.Add(member, e.sysml(pOwner), e.ownerThrough(owner, ownerClass, memberClass))
	if isRelationship(memberClass) {
		// A relationship states the element that owns it, not an owning
		// relationship of its own.
		if isRelationship(ownerClass) {
			e.graph.Add(owner, e.sysml(pOwnedRelatedElement), member)
		}
		e.graph.Add(member, e.sysml(pOwningRelatedElement), owner)
		e.graph.Add(owner, e.sysml(pOwnedRelationship), member)
		return
	}
	e.graph.Add(owner, e.sysml(pOwnedRelatedElement), member)
	e.graph.Add(member, e.sysml(pOwningRelationship), owner)
	// Only an OwningMembership owns its member; a plain Membership (an alias,
	// a `first`) names one elsewhere and owns just its body's annotations.
	if ontology.IsAncestorOrSelf(ownerClass, "OwningMembership") {
		e.graph.Add(member, e.sysml(pOwningMembership), owner)
		e.graph.Add(owner, e.sysml(pMemberElement), member)
		e.graph.Add(owner, e.sysml(pOwnedMemberElement), member)
	}
	if ontology.IsAncestorOrSelf(ownerClass, mFeatureMembership) && ontology.IsAncestorOrSelf(memberClass, mFeature) {
		e.graph.Add(owner, e.sysml(pOwnedMemberFeature), member)
	}
}

// ownerThrough is the owner KerML derives for a member a relationship owns:
// the relationship's own owning element (Element::owner), the relationship
// itself only while it is a membership or owns no element yet.
func (e *encoder) ownerThrough(owner rdf.Term, ownerClass, memberClass string) rdf.Term {
	if isRelationship(memberClass) || !isRelationship(ownerClass) ||
		ontology.IsAncestorOrSelf(ownerClass, mMembership) {
		return owner
	}
	if through := firstIRI(e.graph, owner, pOwningRelatedElement, pOwner); through.Value != "" {
		return through
	}
	return owner
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
		return e.expression(subject, e.sysx(xCondition), xCondition, owner, expr)
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
	if err := e.multiplicity(subject, owner, n.multiplicity); err != nil {
		return err
	}
	if err := e.featureValue(subject, owner, n.value, n.isDefault, n.isInitial); err != nil {
		return err
	}
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
func (e *encoder) bindingEnds(subject rdf.Term, owner string, n *ast.Usage) error {
	endCount := len(n.ConnectorEnds)
	for i, end := range n.ConnectorEnds {
		if end == nil {
			continue
		}
		slot := fmt.Sprintf("end%d", i)
		name := ""
		keyword := ""
		if declared, named := end.DeclaredName(); named {
			name = declared.Name
			keyword = e.referencesKeyword(end)
		}
		if err := e.connectorEnd(subject, connectorEndSpec{owner: owner, slot: slot, index: i, ends: endCount, target: end.AttachedTarget(), mult: end.Multiplicity, name: name, keyword: keyword}); err != nil {
			return err
		}
	}
	if n.FlowEnds == nil {
		return nil
	}
	for i, target := range []ast.Node{n.FlowEnds.From, n.FlowEnds.To} {
		if target == nil {
			continue
		}
		if err := e.connectorEnd(subject, connectorEndSpec{owner: owner, slot: fmt.Sprintf("end%d", i), index: i, ends: 2, target: target}); err != nil {
			return err
		}
	}
	if n.FlowEnds.Payload != nil {
		if err := e.expression(subject, e.sysx(xPayload), xPayload, owner, n.FlowEnds.Payload); err != nil {
			return err
		}
	}
	return nil
}

// connectorEndSpec is one connector end to emit: which end it is, its target,
// its multiplicity and the name and `references` keyword it was written with.
type connectorEndSpec struct {
	owner, slot   string
	index, ends   int
	target        ast.Node
	mult          *ast.Multiplicity
	name, keyword string
}

// connectorEnd emits a standard ConnectorEnd feature and its EndFeatureMembership.
func (e *encoder) connectorEnd(subject rdf.Term, end connectorEndSpec) error {
	if end.target == nil {
		return nil
	}
	feature := e.ids.mintedNode(rdf.ExpressionIRI(subject, end.slot), subject, end.slot)
	membership := e.ids.minted(rdf.OwningMembershipIRIOf(feature), feature, rdf.OwningMembershipSuffix)
	e.graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
	e.typed(feature, crossFeatureMetaclass(false))
	e.graph.Add(feature, e.sysml(pElementID), rdf.String(rdf.LocalName(feature.Value)))
	e.graph.Add(feature, e.sysml(pIsEnd), rdf.Bool(true))
	e.graph.Add(subject, e.sysml(pConnectorEnd), feature)
	e.graph.Add(subject, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(subject, e.sysml(pOwnedMembership), membership)
	e.graph.Add(subject, e.sysml(pOwnedFeatureMembership), membership)
	e.graph.Add(subject, e.sysml(pOwnedFeature), feature)
	e.graph.Add(subject, e.sysml(pOwnedEndFeature), feature)
	if reference, ok := e.endReferenceIRI(end.target); ok {
		e.graph.Add(subject, e.sysml(pRelatedFeature), reference)
		if end.ends == 2 && end.index == 0 {
			e.graph.Add(subject, e.sysml(pSourceFeature), reference)
		} else if end.ends == 2 && end.index == 1 {
			e.graph.Add(subject, e.sysml(pTargetFeature), reference)
		}
	}
	e.emitMembershipCore(membership, feature, subject, mEndFeatureMembership, true)
	e.graph.Add(feature, e.sysx(xSourceText), rdf.String(e.text(end.target)))
	if end.name != "" {
		e.graph.Add(feature, e.sysml(pDeclaredName), rdf.String(end.name))
		e.graph.Add(feature, e.sysml(pName), rdf.String(end.name))
		if end.keyword != "" && end.keyword != referencesSymbol {
			e.graph.Add(feature, e.sysx(xEndReferencesKeyword), rdf.String(end.keyword))
		}
	}
	if err := e.endReferences(feature, end.target); err != nil {
		return err
	}
	return e.multiplicity(feature, end.owner, end.mult)
}

// endReferenceIRI returns a linked simple-name target, excluding chains.
func (e *encoder) endReferenceIRI(target ast.Node) (rdf.Term, bool) {
	name, ok := target.(*ast.QualifiedName)
	if !ok || qualifiedNameHasChain(name) {
		return rdf.Term{}, false
	}
	reference := e.reference(name)
	return reference, reference.IsIRI()
}

// endReferences writes a simple reference, structural chain, or expression target.
func (e *encoder) endReferences(feature rdf.Term, target ast.Node) error {
	switch target := target.(type) {
	case *ast.QualifiedName:
		if qualifiedNameHasChain(target) {
			return e.endChainReferences(feature, target)
		}
		e.referenceSubsetting(feature, e.reference(target))
	case *ast.FeatureChainExpr:
		segments := make([]rdf.Term, 0, len(featureChainSegments(target)))
		for _, segment := range featureChainSegments(target) {
			segments = append(segments, e.reference(segment))
		}
		e.chainFeature(feature, segments)
	default:
		e.referenceSubsetting(feature, rdf.TypedLiteral(e.text(target), rdf.OpenSysML+dtExpression))
	}
	return nil
}

// referenceSubsetting writes the standard relationship that connects an end
// feature to the feature or expression it references.
func (e *encoder) referenceSubsetting(feature, target rdf.Term) {
	subsetting := e.ids.mintedNode(rdf.ExpressionIRI(feature, "rs"), feature, "rs")
	e.typed(subsetting, mReferenceSubsetting)
	e.graph.Add(subsetting, e.sysml(pElementID), rdf.String(rdf.LocalName(subsetting.Value)))
	for _, property := range []string{pReferencingFeature, pSubsettingFeature, pSpecific, pSource, pOwningFeature, pOwningType} {
		e.graph.Add(subsetting, e.sysml(property), feature)
	}
	for _, property := range []string{pReferencedFeature, pSubsettedFeature, pGeneral, pTarget} {
		e.graph.Add(subsetting, e.sysml(property), target)
	}
	e.graph.Add(subsetting, e.sysml(pRelatedElement), feature)
	e.graph.Add(subsetting, e.sysml(pRelatedElement), target)
	e.graph.Add(feature, e.sysml(pOwnedReferenceSubsetting), subsetting)
	e.graph.Add(feature, e.sysml(pOwnedSubsetting), subsetting)
	e.graph.Add(feature, e.sysml(pOwnedSpecialization), subsetting)
	e.relationshipOwnership(subsetting, feature, crossFeatureMetaclass(false), mReferenceSubsetting)
}

// chainFeature creates an owned structural Feature holding ordered chain segments.
func (e *encoder) chainFeature(feature rdf.Term, segments []rdf.Term) {
	chain := e.ids.mintedNode(rdf.ExpressionIRI(feature, "chain"), feature, "chain")
	e.typed(chain, mFeature)
	e.graph.Add(chain, e.sysml(pElementID), rdf.String(rdf.LocalName(chain.Value)))
	for _, segment := range segments {
		e.graph.Add(chain, e.sysml(pChainingFeature), segment)
	}
	membership := e.ids.minted(rdf.OwningMembershipIRIOf(chain), chain, rdf.OwningMembershipSuffix)
	e.emitMembershipCore(membership, chain, feature, mOwningMembership, true)
	e.graph.Add(feature, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(feature, e.sysml(pOwnedMembership), membership)
	e.referenceSubsetting(feature, chain)
}

func (e *encoder) endChainReferences(feature rdf.Term, target *ast.QualifiedName) error {
	segments := append([]rdf.Term(nil), e.qualifiedChainReferences(target)...)
	e.chainFeature(feature, segments)
	return nil
}

// qualifiedNameHasChain reports whether a qualified name contains a chained segment.
func qualifiedNameHasChain(name *ast.QualifiedName) bool {
	for _, part := range name.Parts {
		if part.Chained {
			return true
		}
	}
	return false
}

// qualifiedChainReferences resolves each linked segment of a qualified chain.
func (e *encoder) qualifiedChainReferences(name *ast.QualifiedName) []rdf.Term {
	var terms []rdf.Term
	start := 0
	for i := 1; i < len(name.Parts); i++ {
		if name.Parts[i].Chained {
			terms = append(terms, e.qualifiedNamePartReference(name, start, i))
			start = i
		}
	}
	if start < len(name.Parts) {
		terms = append(terms, e.qualifiedNamePartReference(name, start, len(name.Parts)))
	}
	return terms
}

// qualifiedNamePartReference resolves one contiguous qualified-name chain segment.
func (e *encoder) qualifiedNamePartReference(name *ast.QualifiedName, start, end int) rdf.Term {
	part := &ast.QualifiedName{Parts: append([]ast.NameSegment(nil), name.Parts[start:end]...)}
	sym, ok := e.res.PartSymbol(name, end-1)
	return e.linkOrText(part, sym, ok)
}

// featureChainSegments returns the ordered qualified-name segments of a feature chain.
func featureChainSegments(node *ast.FeatureChainExpr) []*ast.QualifiedName {
	var segments []*ast.QualifiedName
	var walk func(ast.Node)
	walk = func(node ast.Node) {
		switch node := node.(type) {
		case *ast.QualifiedName:
			segments = append(segments, node)
		case *ast.FeatureReference:
			segments = append(segments, node.Name)
		case *ast.FeatureChainExpr:
			walk(node.Operand)
			segments = append(segments, node.Member)
		}
	}
	walk(node)
	return segments
}

func (e *encoder) sysml(name string) rdf.Term { return rdf.SysMLTerm(name) }
func (e *encoder) sysx(name string) rdf.Term  { return rdf.OpenSysMLTerm(name) }

// documentation emits what a `doc` states: its identification, locale and body.
func (e *encoder) documentation(subject rdf.Term, n *ast.Documentation) {
	e.ident(subject, n.Ident)
	if n.Locale != "" {
		e.graph.Add(subject, e.sysml(pLocale), rdf.String(source.StringValue(n.Locale)))
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
func (e *encoder) featureValue(subject rdf.Term, owner string, value ast.Node, isDefault, isInitial bool) error {
	if value == nil {
		return nil
	}
	e.flags(subject, []boolProperty{
		{pIsDefault, isDefault},
		{pIsInitial, isInitial},
	})
	if err := e.expression(subject, e.sysml(pValue), pValue, owner, value); err != nil {
		return err
	}
	return nil
}

func (e *encoder) flags(subject rdf.Term, flags []boolProperty) {
	for _, flag := range flags {
		if !flag.value {
			continue
		}
		e.graph.Add(subject, e.sysml(flag.name), rdf.Bool(true))
	}
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
	return e.multiplicity(crossSubject, crossFQN, cross.Multiplicity)
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

// multiplicity emits the MultiplicityRange a declared multiplicity is
// (SysML.xtext OwnedMultiplicityRangeMember): the bounds are owned by the
// range, which the head's collapsed bounds still state beside it.
func (e *encoder) multiplicity(subject rdf.Term, owner string, mult *ast.Multiplicity) error {
	if mult == nil {
		return nil
	}
	e.graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
	rangeNode := e.ids.mintedNode(rdf.ExpressionIRI(subject, "multiplicity"), subject, "multiplicity")
	membership := e.ids.minted(rdf.OwningMembershipIRIOf(rangeNode), rangeNode, rdf.OwningMembershipSuffix)
	for _, c := range []struct{ iri, standsFor string }{
		{rangeNode.Value, "the multiplicity range of " + rdf.LocalName(subject.Value)},
		{membership.Value, "the owning membership of the multiplicity range"},
	} {
		if prior, taken := e.claim(c.iri, c.standsFor); taken && e.idErr == nil {
			e.idErr = &UnsupportedError{
				What: c.standsFor,
				Note: fmt.Sprintf("its id lands on the same IRI as %s, and merging two elements into one subject would be a different model", prior),
			}
		}
	}
	e.typed(rangeNode, mMultiplicityRange)
	e.graph.Add(rangeNode, e.sysml(pElementID), rdf.String(rdf.LocalName(rangeNode.Value)))
	e.graph.Add(subject, e.sysml(pMultiplicity), rangeNode)
	e.emitMembershipCore(membership, rangeNode, subject, mOwningMembership, true)
	e.graph.Add(subject, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(subject, e.sysml(pOwnedMembership), membership)
	return e.multiplicityBounds(subject, rangeNode, owner, mult)
}

// multiplicityBounds emits the bounds of mult, owned by rangeNode and
// collapsed on subject.
func (e *encoder) multiplicityBounds(subject, rangeNode rdf.Term, owner string, mult *ast.Multiplicity) error {
	if mult == nil {
		return nil
	}
	// The parser puts the single bound of `[n]` in Lower; the language reads
	// that as lower and upper both being n, so it is written as the upper bound
	// alone and the printer renders it back as `[n]`.
	lower, upper := mult.Lower, mult.Upper
	if !mult.IsRange {
		lower, upper = nil, mult.Lower
	}
	for _, bound := range []struct {
		property string
		node     ast.Node
	}{
		{pLowerBound, lower},
		{pUpperBound, upper},
	} {
		if err := e.multiplicityBound(subject, rangeNode, owner, bound.property, bound.node); err != nil {
			return err
		}
	}
	return nil
}

// multiplicityBound emits one bound of a multiplicity range: the literal the
// head states collapsed and the range owns through its own membership.
func (e *encoder) multiplicityBound(subject, rangeNode rdf.Term, owner, property string, node ast.Node) error {
	if node == nil {
		return nil
	}
	bound := e.ids.mintedNode(rdf.ExpressionIRI(subject, property), subject, property)
	membership := e.ids.minted(rdf.OwningMembershipIRIOf(bound), bound, rdf.OwningMembershipSuffix)
	e.graph.Add(subject, e.sysml(property), bound)
	e.graph.Add(rangeNode, e.sysml(property), bound)
	if err := e.expressionNode(bound, owner, node); err != nil {
		return err
	}
	e.emitMembershipCore(membership, bound, rangeNode, mOwningMembership, true)
	e.graph.Add(rangeNode, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(rangeNode, e.sysml(pOwnedMembership), membership)
	return nil
}

// parameterMembership is the membership metaclass and its parameter end for
// the member kinds whose declaration is a usage the membership owns, or ""
// for a member that is not a parameter.
func parameterMembership(kind ast.UsageKind) (class, end string) {
	switch kind {
	case ast.UsageSubject:
		return mSubjectMembership, "ownedSubjectParameter"
	case ast.UsageActor:
		return mActorMembership, "ownedActorParameter"
	case ast.UsageStakeholder:
		return mStakeholderMembership, "ownedStakeholderParameter"
	case ast.UsageObjective:
		return mObjectiveMembership, "ownedObjectiveRequirement"
	}
	return "", ""
}

// memberOwnedUsage is the membership metaclass, its owned end and its
// RequirementConstraintKind for the members whose declaration is a usage
// the membership owns but that is no parameter, or "" for any other kind.
func memberOwnedUsage(kind ast.UsageKind) (class, end, constraintKind string) {
	switch kind {
	case ast.UsageViewRendering:
		return mViewRenderingMembership, pOwnedRendering, ""
	case ast.UsageFramedConcern:
		return mFramedConcernMembership, pOwnedConcern, "requirement"
	}
	return "", "", ""
}

// reference renders a name reference as a link when it resolves to an element
// this document declares or the norm fixes an id for, else as the written name.
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
// declared or effectively, here or in the standard library; a `first x` label
// stands for the member x reaches past it, and a loop variable names none.
func (e *encoder) linked(sym *symbols.Symbol, ok bool) (ast.Node, string, bool) {
	if !ok || sym == nil {
		return nil, "", false
	}
	if label, isLabel := sym.Decl.(*ast.InitialNode); isLabel {
		if sym, ok = e.res.InitialSymbol(label); !ok {
			return nil, "", false
		}
	}
	fqn, declared := e.fqn[sym.Decl]
	if !declared {
		if fqn, declared = e.ids.libraryElement(sym); !declared {
			return nil, "", false
		}
		e.claimLibrary(sym.Decl, fqn)
		return sym.Decl, fqn, true
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
