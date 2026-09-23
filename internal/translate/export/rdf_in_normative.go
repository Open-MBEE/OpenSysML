package export

// Reading back the materialized relationship elements: they are accepted,
// verified against the collapsed properties they restate, and left out of the
// notation, which writes the collapsed form the head states. A first-class
// element disagreeing with the collapsed property beside it is refused, since
// printing the collapsed form would silently pick one of the two.

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// impliedRelationshipMetaclasses are the relationship elements materialized
// beside the collapsed head properties: minted without a qualified name, owned
// by the element they restate an end of, and carrying no membership of their
// own. A keyword-first relationship member of the same metaclass has all three.
var impliedRelationshipMetaclasses = map[string]bool{
	mFeatureTyping:        true,
	mConjugatedPortTyping: true,
	mSubclassification:    true,
	mSubsetting:           true,
	mSpecialization:       true,
	mRedefinition:         true,
	mReferenceSubsetting:  true,
}

// DerivedSatellite reports whether metaclass is one the encoder derives from
// its side tables rather than a declaration — the materialized relationship
// elements, a conjugated port definition and its conjugation, a multiplicity
// range, or the membership relating an expression to its referent. A member
// element of such a metaclass carries a qualified name and stays addressable.
func DerivedSatellite(metaclass string) bool {
	return derivedSatelliteMetaclasses[metaclass]
}

var derivedSatelliteMetaclasses = map[string]bool{
	mFeatureTyping:            true,
	mConjugatedPortTyping:     true,
	mSubclassification:        true,
	mSubsetting:               true,
	mSpecialization:           true,
	mRedefinition:             true,
	mReferenceSubsetting:      true,
	mPortConjugation:          true,
	mConjugatedPortDefinition: true,
	mMultiplicityRange:        true,
	mMembership:               true,
}

// relationshipSourceEnds are the properties of a materialized relationship
// element that name its source end: the element it is owned by.
var relationshipSourceEnds = []string{
	"typedFeature", "subclassifier", pSubsettingFeature, "redefiningFeature",
	pReferencingFeature, pSpecific, pSource, pOwningFeature, "owningClassifier",
	pOwningType, pOwningRelatedElement,
}

// relationshipTargetEnds are the properties naming the relationship's target.
var relationshipTargetEnds = []string{
	"type", "superclassifier", pSubsettedFeature, pReferencedFeature,
	"redefinedFeature", pGeneral, pTarget,
}

// normativeImplied reports whether el is one of the materialized elements a
// decoder reads back rather than prints, and verifies it agrees with the
// collapsed statement beside it: the target set its ends name is the set the
// head's collapsed properties state, a conjugated definition is the `~` of its
// owner, and a satisfy's subject parameter is the `by` the head states.
func (d *decoder) normativeImplied(el *element, parent *element) (bool, error) {
	ownedByParent := func() bool {
		return parent != nil
	}
	switch {
	case impliedRelationshipMetaclasses[el.metaclass]:
		if !ownedByParent() || d.graph.HasProperty(rdf.IRI(el.iri), rdf.SysML+pQualifiedName) ||
			d.graph.HasProperty(rdf.IRI(el.iri), rdf.OpenSysML+xDeclaredKeyword) {
			return false, nil
		}
		if _, owned := d.owningMembership[el.iri]; owned {
			return false, nil
		}
		return d.impliedRelationship(el, parent)
	case el.metaclass == mPortConjugation:
		if parent == nil || parent.metaclass != mConjugatedPortDefinition {
			return false, nil
		}
		if d.graph.HasProperty(rdf.IRI(el.iri), rdf.SysML+pQualifiedName) {
			return false, nil
		}
		return true, nil
	case el.metaclass == mConjugatedPortDefinition:
		if parent == nil || parent.metaclass != "PortDefinition" {
			return false, nil
		}
		return true, d.verifyConjugated(el, parent)
	case el.metaclass == mReferenceUsage:
		// The subject parameter of a satisfy head: a ReferenceUsage its
		// SubjectMembership owns, whose value is a reference to the `by`
		// target the owner's collapsed subject states.
		m, owned := d.owningMembership[el.iri]
		if !owned || parent == nil || d.metaclass(rdf.IRI(m.iri)) != mSubjectMembership {
			return false, nil
		}
		subjects := d.graph.Objects(rdf.IRI(parent.iri), rdf.SysML+relationshipProperty[ast.RelSubject])
		if len(subjects) == 0 {
			return false, nil
		}
		return true, d.verifySubjectParameter(el, parent, subjects)
	}
	return false, nil
}

// impliedRelationship classifies a candidate materialized relationship
// element. When none of its target ends is a target the parent's collapsed
// properties state, it is an element another graph declared this way rather
// than a restatement, and it is read as an ordinary element — errors it earns
// surface where the element would, after the members before it. When it
// restates collapsed targets, it is implied, and any end disagreeing with the
// owner or naming a target the collapsed form never states is refused: the
// notation would otherwise pick one of the two statements.
func (d *decoder) impliedRelationship(el *element, parent *element) (bool, error) {
	what := fmt.Sprintf("the %s <%s>", el.metaclass, el.iri)
	stated := map[string]bool{}
	literal := false
	for _, kind := range collapsedKindsOf[el.metaclass] {
		for _, object := range d.graph.Objects(rdf.IRI(parent.iri), rdf.SysML+relationshipProperty[kind]) {
			stated[object.Value] = true
			if !object.IsIRI() {
				literal = true
			}
		}
	}
	actual := map[string]bool{}
	actualIRI := map[string]bool{}
	matched := 0
	for _, property := range relationshipTargetEnds {
		for _, object := range d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+property) {
			actual[object.Value] = true
			actualIRI[object.Value] = object.IsIRI()
		}
	}
	// An IRI end matches a literal collapsed target through its declared name.
	literalName := func(value string) bool {
		for _, property := range []string{pDeclaredName, pDeclaredShortName} {
			for _, name := range d.graph.Objects(rdf.IRI(value), rdf.SysML+property) {
				if stated[name.Value] {
					return true
				}
			}
		}
		return false
	}
	for value := range actual {
		if stated[value] || literalName(value) {
			matched++
		}
	}
	if len(actual) == 0 {
		// An element with no ends cannot be checked against a literal target.
		if literal {
			return true, nil
		}
		return false, nil
	}
	if matched == 0 {
		// A metadata usage's typing has its own check over the collapsed property.
		if parent.metaclass == usageMetaclass[ast.UsageMetadata] {
			if literal {
				return true, nil
			}
			return false, nil
		}
		// Ends naming targets the parent states under other kinds are misassigned.
		other := map[string]bool{}
		for _, kind := range []ast.RelationshipKind{
			ast.RelTyping, ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines, ast.RelReferences,
		} {
			if slices.Contains(collapsedKindsOf[el.metaclass], kind) {
				continue
			}
			for _, object := range d.graph.Objects(rdf.IRI(parent.iri), rdf.SysML+relationshipProperty[kind]) {
				other[object.Value] = true
			}
		}
		misassigned := len(actual) > 0
		var moved []string
		for value := range actual {
			if !other[value] {
				misassigned = false
				break
			}
			moved = append(moved, value)
		}
		if misassigned {
			sort.Strings(moved)
			return false, &UnsupportedError{
				What: what,
				Note: fmt.Sprintf("it names <%s>, which the collapsed properties of <%s> state under a different relationship kind, and writing them would move the targets across kinds", strings.Join(moved, ">, <"), parent.iri),
			}
		}
		// Restating none of the IRI targets, it is a foreign element.
		if !literal {
			return false, nil
		}
		// An unmatched named end is refused; an end the graph names nothing for
		// (a legacy graph's bare uuid) cannot be checked and stays vacuous.
		checkable := false
		for value := range actual {
			if stated[value] || literalName(value) {
				continue
			}
			if !actualIRI[value] {
				checkable = true
				break
			}
			for _, property := range []string{pDeclaredName, pDeclaredShortName, pQualifiedName} {
				if d.graph.HasProperty(rdf.IRI(value), rdf.SysML+property) {
					checkable = true
				}
			}
			if checkable {
				break
			}
		}
		if !checkable {
			return true, nil
		}
	}
	if matched != len(actual) {
		var targets []string
		for value := range actual {
			if !stated[value] {
				targets = append(targets, value)
			}
		}
		sort.Strings(targets)
		return false, &UnsupportedError{
			What: what,
			Note: fmt.Sprintf("it names <%s>, which the collapsed properties of <%s> never state, and writing the collapsed form would drop it", strings.Join(targets, ">, <"), parent.iri),
		}
	}
	for _, property := range relationshipSourceEnds {
		for _, object := range d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+property) {
			if object.Value != parent.iri {
				return false, &UnsupportedError{
					What: what,
					Note: fmt.Sprintf("its %s is <%s> while its owner is <%s>, and writing the owner's collapsed form would pick one of the two", curie(rdf.SysML+property), object.Value, parent.iri),
				}
			}
		}
	}
	return true, nil
}

// verifyCovered checks the other half of the agreement: once a head carries
// materialized relationship elements, every target its collapsed properties
// state is the target of one of them — a covered set that leaves targets out
// is a conflict the collapsed write would decide silently. A head stating no
// materialized elements, as a graph from before their introduction does,
// needs no coverage.
func (d *decoder) verifyCovered(owner *element) error {
	if owner.implied {
		return nil
	}
	// A metadata usage's typing is ruled by its own check, which reads the
	// collapsed property directly.
	if owner.metaclass == usageMetaclass[ast.UsageMetadata] {
		return nil
	}
	materialized := false
	for _, child := range owner.children {
		if child.implied && impliedRelationshipMetaclasses[child.metaclass] {
			materialized = true
			break
		}
	}
	if !materialized {
		return nil
	}
	stated := map[ast.RelationshipKind]map[string]bool{}
	for _, kind := range []ast.RelationshipKind{
		ast.RelTyping, ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines, ast.RelReferences,
	} {
		for _, object := range d.graph.Objects(rdf.IRI(owner.iri), rdf.SysML+relationshipProperty[kind]) {
			// A literal edge names its target, so no element covers it.
			if object.IsIRI() {
				if stated[kind] == nil {
					stated[kind] = map[string]bool{}
				}
				stated[kind][object.Value] = true
			}
		}
	}
	for _, child := range owner.children {
		if !child.implied || !impliedRelationshipMetaclasses[child.metaclass] {
			continue
		}
		targets := map[string]bool{}
		for _, property := range relationshipTargetEnds {
			for _, object := range d.graph.Objects(rdf.IRI(child.iri), rdf.SysML+property) {
				targets[object.Value] = true
			}
		}
		// A child whose metaclass maps to more than one kind covers a target
		// under one kind only: the first kind still stating it consumes it.
		for _, kind := range collapsedKindsOf[child.metaclass] {
			for value := range targets {
				if stated[kind][value] {
					delete(stated[kind], value)
					delete(targets, value)
				}
			}
		}
	}
	uncovered := map[string]bool{}
	for _, targets := range stated {
		for value := range targets {
			uncovered[value] = true
		}
	}
	if len(uncovered) == 0 {
		return nil
	}
	missing := make([]string, 0, len(uncovered))
	for value := range uncovered {
		missing = append(missing, value)
	}
	sort.Strings(missing)
	return &UnsupportedError{
		What: fmt.Sprintf("the collapsed properties of <%s>", owner.iri),
		Note: fmt.Sprintf("they state <%s>, which no materialized relationship element carries, so writing them would assert a relationship the elements deny", strings.Join(missing, ">, <")),
	}
}

// parameterMember reports whether el is owned through a kind-carrying
// ParameterMembership: its `in` direction is the default a member never
// spells out. A bare ParameterMembership declares no parameter kind, so a
// member's direction is written explicitly.
func (d *decoder) parameterMember(el *element) bool {
	_, kind := d.parameterMembershipKind(el)
	return kind
}

// verifyConjugated checks a conjugated port definition against its owner: it
// is the `~` of the original's name, which the notation derives rather than
// writing.
func (d *decoder) verifyConjugated(el *element, parent *element) error {
	name, _ := d.stringOf(el, rdf.SysML+pDeclaredName)
	original, ok := d.stringOf(parent, rdf.SysML+pDeclaredName)
	if !ok {
		original = parent.qname[strings.LastIndex(parent.qname, "::")+2:]
	}
	expected := "~" + original
	if name != expected {
		return &UnsupportedError{
			What: fmt.Sprintf("the conjugated port definition <%s>", el.iri),
			Note: fmt.Sprintf("it is named %q where the port definition it conjugates is %q, and the conjugate is derived as ~ rather than written", name, expected),
		}
	}
	return nil
}

// verifySubjectParameter checks a satisfy's subject parameter: its value is a
// FeatureReferenceExpression of one of the `by` targets the head states.
func (d *decoder) verifySubjectParameter(el *element, parent *element, subjects []rdf.Term) error {
	what := fmt.Sprintf("the subject parameter <%s>", el.iri)
	iris := false
	for _, subject := range subjects {
		iris = iris || subject.IsIRI()
	}
	if !iris {
		return nil
	}
	referents := false
	for _, object := range d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pValue) {
		for _, property := range []string{pReferent, pTargetFeature} {
			for _, referent := range d.graph.Objects(object, rdf.SysML+property) {
				referents = true
				matched := false
				for _, subject := range subjects {
					matched = matched || referent == subject
				}
				if !matched {
					return &UnsupportedError{
						What: what,
						Note: fmt.Sprintf("its value refers to <%s> while the satisfy head states <%s>, and writing `by` would pick one of the two", referent.Value, subjects[0].Value),
					}
				}
			}
		}
	}
	if referents {
		return nil
	}
	return &UnsupportedError{
		What: what,
		Note: "it values no feature reference, so the `by` the satisfy head states cannot be verified against it",
	}
}

// membershipUsageKind is the usage kind a member's metaclass does not state
// but its owning membership does: the parameter an actor, stakeholder,
// subject or objective member declares, or a `render`/`frame` member's usage.
func (d *decoder) membershipUsageKind(el *element) (ast.UsageKind, bool) {
	if kind, ok := d.parameterMembershipKind(el); ok {
		return kind, true
	}
	m, owned := d.owningMembership[el.iri]
	if !owned {
		return 0, false
	}
	switch d.metaclass(rdf.IRI(m.iri)) {
	case mViewRenderingMembership:
		return ast.UsageViewRendering, true
	case mFramedConcernMembership:
		return ast.UsageFramedConcern, true
	}
	return 0, false
}

// parameterMembershipKind is the kind of the parameter a subject, actor,
// stakeholder or objective membership owns.
func (d *decoder) parameterMembershipKind(el *element) (ast.UsageKind, bool) {
	m, owned := d.owningMembership[el.iri]
	if !owned {
		return 0, false
	}
	switch d.metaclass(rdf.IRI(m.iri)) {
	case mSubjectMembership:
		return ast.UsageSubject, true
	case mActorMembership:
		return ast.UsageActor, true
	case mStakeholderMembership:
		return ast.UsageStakeholder, true
	case mObjectiveMembership:
		return ast.UsageObjective, true
	}
	return 0, false
}

// normativeMemberHead reads the member forms whose kind lives on the owning
// membership or the owner: a `require`/`assume` member's ConstraintUsage under
// a RequirementConstraintMembership, and a constraint body's `assert`/`assume`
// member, which is an AssertConstraintUsage declaring or stating a constraint.
func (d *decoder) normativeMemberHead(el *element) (string, bool, error) {
	m, owned := d.owningMembership[el.iri]
	if !owned {
		return "", false, nil
	}
	mclass := d.metaclass(rdf.IRI(m.iri))
	switch {
	case el.metaclass == mConstraintUsage && mclass == mRequirementConstraintMembership:
		kind, _ := d.graph.Lexical(rdf.IRI(m.iri), rdf.SysML+pKind)
		keyword := "require"
		if kind == "assumption" {
			keyword = "assume"
		}
		head, err := d.conditionHead(el, keyword)
		return head, true, err
	case el.metaclass == mAssertConstraintUsage && d.constraintMemberOwner(el.owner):
		keyword := "assert"
		if written, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword); ok {
			keyword = written
		}
		head, err := d.conditionHead(el, keyword)
		return head, true, err
	}
	return "", false, nil
}

// constraintMemberOwner reports whether el's owner is a constraint body, which
// is where an AssertConstraintUsage is a condition member rather than a usage
// declared `assert`.
func (d *decoder) constraintMemberOwner(owner *element) bool {
	if owner == nil {
		return false
	}
	switch owner.metaclass {
	case "ConstraintDefinition", "Predicate", "BooleanExpression":
		return true
	}
	return owner.metaclass == mConstraintUsage || owner.metaclass == mAssertConstraintUsage
}

// verifyNormativeNodes checks the minted nodes no element owns: a Membership
// between an expression and its referent states the same target the
// expression's referent property does, and a MultiplicityRange's bounds are
// the collapsed bounds its feature states.
func (d *decoder) verifyNormativeNodes() error {
	for _, subject := range d.graph.Subjects() {
		switch d.metaclass(subject) {
		case mMembership:
			if err := d.verifyReferentMembership(subject); err != nil {
				return err
			}
		case mMultiplicityRange:
			if err := d.verifyMultiplicityRange(subject); err != nil {
				return err
			}
		}
	}
	return nil
}

// verifyReferentMembership checks a referent Membership: owned by the
// expression it relates, its member is the expression's referent or target
// feature.
func (d *decoder) verifyReferentMembership(subject rdf.Term) error {
	if !d.isExpressionIRI(subject) {
		return nil
	}
	member, ok := d.graph.Object(subject, rdf.SysML+pMemberElement)
	if !ok {
		return nil
	}
	owner, ok := d.graph.Object(subject, rdf.SysML+pOwningRelatedElement)
	if !ok {
		return &UnsupportedError{
			What: fmt.Sprintf("the membership <%s>", subject.Value),
			Note: "it owns no element, so the expression it relates cannot be told",
		}
	}
	stated := false
	for _, property := range []string{pReferent, pTargetFeature, pFunction} {
		objects := d.graph.Objects(owner, rdf.SysML+property)
		stated = stated || len(objects) > 0
		for _, object := range objects {
			if object == member {
				return nil
			}
		}
		for _, object := range objects {
			if !object.IsIRI() {
				// A member is always an element, so a literal edge states no
				// member for this membership to carry: it is foreign, and the
				// verifier leaves it rather than refusing what it cannot check.
				return nil
			}
		}
	}
	if !stated {
		// The membership alone states the referent: the normative-only form.
		return nil
	}
	return &UnsupportedError{
		What: fmt.Sprintf("the membership <%s>", subject.Value),
		Note: fmt.Sprintf("its member is <%s>, which the expression <%s> states no referent, target feature or function for, and the two statements cannot both hold", member.Value, owner.Value),
	}
}

// verifyMultiplicityRange checks a MultiplicityRange's bounds are the ones its
// feature's collapsed bounds state.
func (d *decoder) verifyMultiplicityRange(subject rdf.Term) error {
	if !d.isExpressionIRI(subject) {
		return nil
	}
	// The range is owned through its membership; the feature the bounds are
	// collapsed on is the membership's owner.
	owner, ok := d.rangeOwner(subject)
	if !ok {
		return nil
	}
	for _, property := range []string{pLowerBound, pUpperBound} {
		collapsed := map[string]bool{}
		for _, bound := range d.graph.Objects(owner, rdf.SysML+property) {
			collapsed[bound.Value] = true
		}
		carried := map[string]bool{}
		for _, bound := range d.graph.Objects(subject, rdf.SysML+property) {
			carried[bound.Value] = true
			if !collapsed[bound.Value] {
				return &UnsupportedError{
					What: fmt.Sprintf("the multiplicity range <%s>", subject.Value),
					Note: fmt.Sprintf("its %s is <%s>, which <%s> states no such bound for, and writing the collapsed bound would drop it", curie(rdf.SysML+property), bound.Value, owner.Value),
				}
			}
		}
		for _, bound := range d.graph.Objects(owner, rdf.SysML+property) {
			if !carried[bound.Value] {
				return &UnsupportedError{
					What: fmt.Sprintf("the multiplicity range <%s>", subject.Value),
					Note: fmt.Sprintf("<%s> states %s <%s> it does not carry, and writing the collapsed bound would assert one it denies", owner.Value, curie(rdf.SysML+property), bound.Value),
				}
			}
		}
	}
	return nil
}

// rangeOwner is the element a multiplicity range is owned by, through the
// owning membership recorded for it.
func (d *decoder) rangeOwner(subject rdf.Term) (rdf.Term, bool) {
	m, ok := d.nodeMemberships[subject.Value]
	if !ok {
		return rdf.Term{}, false
	}
	return rdf.IRI(m.owner), true
}
