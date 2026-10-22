package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// assertRelString checks a string-valued reflective feature.
func assertRelStrings(t *testing.T, model *Model, sym *symbols.Symbol, feature string, want ...string) {
	t.Helper()
	values, ok := model.ReflectiveFeatureValues(sym, feature)
	if !ok {
		t.Fatalf("%s on %s is unsupported", feature, relMetaName(model, sym))
	}
	if len(values) != len(want) {
		t.Fatalf("%s on %s = %v, want %v", feature, relMetaName(model, sym), values, want)
	}
	for i, value := range values {
		if value.Kind != symbols.FilterValueString || value.Str != want[i] {
			t.Errorf("%s on %s: value %d = %v, want %q", feature, relMetaName(model, sym), i, value, want[i])
		}
	}
}

// assertRelRef checks a reference-valued reflective feature by the FQN it names.
func assertRelRef(t *testing.T, model *Model, sym *symbols.Symbol, feature, want string) {
	t.Helper()
	value, ok := model.ReflectiveFeatureValue(sym, feature)
	if !ok {
		t.Fatalf("%s on %s is unsupported", feature, relMetaName(model, sym))
	}
	if value.Kind != symbols.FilterValueRef || value.RefFQN != want {
		t.Errorf("%s on %s = %v, want ref %s", feature, relMetaName(model, sym), value, want)
	}
}

const membershipFixtureSrc = `package P {
    private import ScalarValues::*;
    metadata def Tag;
    part def Base { part c; attribute x : Real; }
    #Tag part def D :> Base {
        doc /* about D */
        part a;
        part b subsets a;
        attribute y : Real redefines x;
        @Tag;
        rep inline language "text" /* d */
    }
    alias E for D;
    comment C about D, Base /* two */
    dependency Dep from D to Base {
        doc /* dep */
    }
    action def Act { in p : Real; out q : Real; return r : Real; }
    part def Empty;
}`

func TestOwnedMembershipsReflectEachOwnedMember(t *testing.T) {
	for _, fixture := range reflectiveFixtures(t, "memberships.sysml", source.KindSysML, membershipFixtureSrc) {
		model := fixture.model
		d := relFixtureSymbol(t, fixture, "D")
		a := relFixtureSymbol(t, fixture, "a")
		b := relFixtureSymbol(t, fixture, "b")
		y := relFixtureSymbol(t, fixture, "y")
		memberships := model.OwnedMemberships(d)
		// doc, a, b, y, @Tag, rep: the #Tag prefix is owned through an Annotation.
		if len(memberships) != 6 {
			t.Fatalf("D owns %d memberships, want 6", len(memberships))
		}
		assertRelMetaclass(t, model, memberships[0], "OwningMembership")
		assertRelMetaclass(t, model, memberships[1], "FeatureMembership")
		assertRelFeature(t, model, memberships[1], "memberElement", a)
		assertRelFeature(t, model, memberships[1], "ownedMemberFeature", a)
		assertRelFeature(t, model, memberships[1], "owningType", d)
		assertRelFeature(t, model, memberships[1], "membershipOwningNamespace", d)
		assertRelFeature(t, model, memberships[1], "relatedElement", d, a)
		assertRelFeature(t, model, memberships[1], "ownedRelatedElement", a)
		assertRelFeature(t, model, memberships[1], "ownedElement")
		assertRelFeature(t, model, memberships[1], "ownedRelationship")
		assertRelFeature(t, model, memberships[1], "owningRelationship")
		assertRelStrings(t, model, memberships[1], "memberName", "a")
		assertRelRef(t, model, memberships[1], "visibility", "KerML::Root::VisibilityKind::public")
		assertRelBool(t, model, memberships[1], "isImplied", false)
		assertRelMetaclass(t, model, memberships[4], "OwningMembership")
		assertRelMetaclass(t, model, memberships[5], "OwningMembership")

		assertRelFeature(t, model, d, "ownedMembership", memberships...)
		assertRelFeature(t, model, d, "ownedFeatureMembership", memberships[1], memberships[2], memberships[3])
		assertRelFeature(t, model, a, "owningMembership", memberships[1])
		assertRelFeature(t, model, a, "owningRelationship", memberships[1])
		if got := model.OwningMembership(b); got != memberships[2] {
			t.Errorf("OwningMembership(b) = %v, want membership 2", got)
		}
		if got, ok := model.ReflectiveElements(y, "owningMembership"); !ok || len(got) != 1 || got[0] != memberships[3] {
			t.Errorf("owningMembership of y = %v, want membership 3", got)
		}
		// Identity is stable across calls and distinct per member.
		if !symbols.SameElement(memberships[1], model.OwnedMemberships(d)[1]) || symbols.SameElement(memberships[1], memberships[2]) {
			t.Error("membership identity is not keyed by owner and ordinal")
		}
		// The Empty definition owns no membership, and a root package none owning it.
		empty := relFixtureSymbol(t, fixture, "Empty")
		assertRelFeature(t, model, empty, "ownedMembership")
		p := relFixtureSymbol(t, fixture, "P")
		assertRelFeature(t, model, p, "owningMembership")
	}
}

func TestInheritedAndFeatureMemberships(t *testing.T) {
	for _, fixture := range reflectiveFixtures(t, "inherited-memberships.sysml", source.KindSysML, membershipFixtureSrc) {
		model := fixture.model
		d := relFixtureSymbol(t, fixture, "D")
		base := relFixtureSymbol(t, fixture, "Base")
		baseMemberships := model.OwnedMemberships(base)
		// c is inherited (with the library's members); x is masked by y's redefinition.
		inherited, ok := model.ReflectiveElements(d, "inheritedMembership")
		if !ok || !containsElement(inherited, baseMemberships[0]) || !d.Recorded() && containsElement(inherited, baseMemberships[1]) {
			t.Errorf("inheritedMembership of D holds c's membership %t and x's %t, want true and false",
				containsElement(inherited, baseMemberships[0]), containsElement(inherited, baseMemberships[1]))
		}
		owned := model.OwnedMemberships(d)
		features, _ := model.ReflectiveElements(d, "featureMembership")
		if len(features) < 4 || features[0] != owned[1] || features[1] != owned[2] || features[2] != owned[3] ||
			!containsElement(features, baseMemberships[0]) || containsElement(features, owned[0]) {
			t.Errorf("featureMembership of D = %d memberships, want its three features first, then the inherited ones", len(features))
		}
	}
}

func TestAliasIsAMembership(t *testing.T) {
	for _, fixture := range reflectiveFixtures(t, "alias-membership.sysml", source.KindSysML, membershipFixtureSrc) {
		model := fixture.model
		p := relFixtureSymbol(t, fixture, "P")
		d := relFixtureSymbol(t, fixture, "D")
		var alias *symbols.Symbol
		for _, membership := range model.OwnedMemberships(p) {
			if membership.Kind == symbols.SymbolAlias {
				alias = membership
			}
		}
		if alias == nil {
			t.Fatal("alias E is not among P's owned memberships")
		}
		assertRelMetaclass(t, model, alias, "Membership")
		if !alias.Recorded() {
			// The test record carries no alias target.
			assertRelFeature(t, model, alias, "memberElement", d)
		}
		assertRelFeature(t, model, alias, "membershipOwningNamespace", p)
		assertRelFeature(t, model, alias, "owningMembership")
		assertRelStrings(t, model, alias, "memberName", "E")
		assertRelBool(t, model, alias, "isImplied", false)
	}
}

func TestOwnedAnnotationsAndAnnotatingElements(t *testing.T) {
	for _, fixture := range reflectiveFixtures(t, "annotations.sysml", source.KindSysML, membershipFixtureSrc) {
		model := fixture.model
		d := relFixtureSymbol(t, fixture, "D")
		base := relFixtureSymbol(t, fixture, "Base")
		dep := relFixtureSymbol(t, fixture, "Dep")
		c := relFixtureSymbol(t, fixture, "C")
		// The #Tag prefix is D's one owned annotation; it owns the metadata feature.
		// A record keeps the prefix as a fact, so the recorded D owns the Annotation
		// but there is no metadata feature to read through it.
		owned := model.OwnedAnnotations(d)
		if len(owned) != 1 {
			t.Fatalf("D owns %d annotations, want 1", len(owned))
		}
		if d.Recorded() {
			tag := owned[0]
			assertRelMetaclass(t, model, tag, "Annotation")
			assertRelFeature(t, model, d, "ownedAnnotation", tag)
			assertRelFeature(t, model, tag, "annotatedElement", d)
			assertRelFeature(t, model, tag, "owningAnnotatedElement", d)
			assertRelUnsupported(t, model, tag, "annotatingElement")
			assertRelUnsupported(t, model, tag, "ownedAnnotatingElement")
		} else {
			tag := owned[0]
			assertRelMetaclass(t, model, tag, "Annotation")
			assertRelFeature(t, model, tag, "annotatedElement", d)
			assertRelFeature(t, model, tag, "owningAnnotatedElement", d)
			assertRelFeature(t, model, tag, "owningAnnotatingElement")
			assertRelFeature(t, model, d, "ownedAnnotation", tag)
			prefix := tag.Implicit.Member
			assertRelFeature(t, model, tag, "ownedAnnotatingElement", prefix)
			assertRelFeature(t, model, tag, "annotatingElement", prefix)
			assertRelFeature(t, model, prefix, "owningAnnotatingRelationship", tag)
			assertRelFeature(t, model, prefix, "annotation", tag)
			assertRelFeature(t, model, prefix, "annotatedElement", d)
			assertRelFeature(t, model, prefix, "owningRelationship", tag)
			assertRelFeature(t, model, prefix, "owningMembership")
		}

		// A doc in a namespace body annotates its namespace through no Annotation.
		docs := model.documentationSymbols(d)
		if len(docs) != 1 {
			t.Fatalf("D has %d docs, want 1", len(docs))
		}
		assertRelFeature(t, model, docs[0], "annotation")
		assertRelFeature(t, model, docs[0], "ownedAnnotatingRelationship")
		assertRelFeature(t, model, docs[0], "annotatedElement", d)
		assertRelFeature(t, model, docs[0], "documentedElement", d)
		assertRelFeature(t, model, docs[0], "owningMembership", model.OwnedMemberships(d)[0])
		assertRelFeature(t, model, base, "ownedAnnotation")

		// `comment C about D, Base` owns one Annotation per named element.
		if c.Recorded() {
			// The test record carries no `about` facts for a comment.
			continue
		}
		anns := model.AnnotatingRelationships(c)
		if len(anns) != 2 {
			t.Fatalf("C owns %d annotating relationships, want 2", len(anns))
		}
		assertRelFeature(t, model, c, "annotation", anns...)
		assertRelFeature(t, model, c, "ownedAnnotatingRelationship", anns...)
		assertRelFeature(t, model, c, "owningAnnotatingRelationship")
		assertRelFeature(t, model, c, "annotatedElement", d, base)
		assertRelFeature(t, model, anns[0], "annotatedElement", d)
		assertRelFeature(t, model, anns[1], "annotatedElement", base)
		assertRelFeature(t, model, anns[0], "annotatingElement", c)
		assertRelFeature(t, model, anns[0], "owningAnnotatingElement", c)
		assertRelFeature(t, model, anns[0], "owningAnnotatedElement")
		assertRelFeature(t, model, anns[0], "ownedAnnotatingElement")
		assertRelFeature(t, model, anns[0], "owner", c)
		assertRelFeature(t, model, c, "ownedRelationship", anns...)

		// A doc in a dependency's body is owned through an Annotation of the dependency.
		depAnns := model.OwnedAnnotations(dep)
		if len(depAnns) != 1 {
			t.Fatalf("Dep owns %d annotations, want 1", len(depAnns))
		}
		assertRelFeature(t, model, dep, "ownedAnnotation", depAnns...)
		assertRelUnsupported(t, model, dep, "ownedMembership")
		depDoc := depAnns[0].Implicit.Member
		assertRelFeature(t, model, depDoc, "owningAnnotatingRelationship", depAnns[0])
		assertRelFeature(t, model, depDoc, "documentedElement", dep)
		assertRelFeature(t, model, dep, "documentation", depDoc)
	}
}

func TestOwnedRelationshipsComposeClausesMembershipsAndImports(t *testing.T) {
	for _, fixture := range reflectiveFixtures(t, "owned-relationships.sysml", source.KindSysML, membershipFixtureSrc) {
		model := fixture.model
		p := relFixtureSymbol(t, fixture, "P")
		d := relFixtureSymbol(t, fixture, "D")
		b := relFixtureSymbol(t, fixture, "b")
		imports := model.OwnedImports(p)
		if len(imports) != 1 {
			t.Fatalf("P owns %d imports, want 1", len(imports))
		}
		assertRelMetaclass(t, model, imports[0], "NamespaceImport")
		assertRelFeature(t, model, imports[0], "importOwningNamespace", p)
		if imported, ok := model.ReflectiveElements(imports[0], "importedNamespace"); !ok || len(imported) != 1 || imported[0].Name != "ScalarValues" {
			t.Errorf("importedNamespace = %v, want ScalarValues", imported)
		}
		assertRelUnsupported(t, model, imports[0], "importedMembership")
		assertRelBool(t, model, imports[0], "isRecursive", false)
		assertRelBool(t, model, imports[0], "isImportAll", false)
		assertRelRef(t, model, imports[0], "visibility", "KerML::Root::VisibilityKind::private")
		assertRelFeature(t, model, p, "ownedImport", imports...)

		rels, ok := model.ReflectiveElements(p, "ownedRelationship")
		if !ok {
			t.Fatal("ownedRelationship of P is unsupported")
		}
		want := append([]*symbols.Symbol{imports[0]}, model.OwnedMemberships(p)...)
		if len(rels) != len(want) {
			t.Fatalf("P owns %d relationships, want %d", len(rels), len(want))
		}
		if rels[0] != imports[0] {
			t.Error("the import does not come first in P's owned relationships")
		}

		// D: the prefix annotation, the written specialization, then its body.
		dRels, _ := model.ReflectiveElements(d, "ownedRelationship")
		owned := model.OwnedAnnotations(d)
		written := model.ImplicitRelationships(d)
		if len(dRels) != len(owned)+len(written)+len(model.OwnedMemberships(d)) || dRels[len(owned)] != written[0] ||
			len(owned) > 0 && dRels[0] != owned[0] {
			t.Errorf("D's owned relationships = %d, want %d prefix annotations, %d clauses and %d memberships",
				len(dRels), len(owned), len(written), len(model.OwnedMemberships(d)))
		}
		// b owns only its subsetting; a relationship object owns nothing.
		assertRelFeature(t, model, b, "ownedRelationship", model.ImplicitRelationships(b)...)
		assertRelFeature(t, model, model.ImplicitRelationships(b)[0], "ownedRelationship")
	}
}

func TestParameterAndEndMemberships(t *testing.T) {
	src := `package P {
    private import ScalarValues::*;
    action def Act { in p : Real; out q : Real; return r : Real; attribute z : Real; }
    part def W; part def Pt { port pt; }
    connection def CD { end a : W; end b : W; }
    part def Asm { part w1 : W; part w2 : W; connect w1 to w2; }
}`
	for _, fixture := range reflectiveFixtures(t, "parameter-memberships.sysml", source.KindSysML, src) {
		model := fixture.model
		act := relFixtureSymbol(t, fixture, "Act")
		kinds := []string{"ParameterMembership", "ParameterMembership", "ReturnParameterMembership", "FeatureMembership"}
		memberships := model.OwnedMemberships(act)
		if len(memberships) != len(kinds) {
			t.Fatalf("Act owns %d memberships, want %d", len(memberships), len(kinds))
		}
		for i, kind := range kinds {
			assertRelMetaclass(t, model, memberships[i], kind)
		}
		assertRelFeature(t, model, memberships[0], "ownedMemberParameter", relFixtureSymbol(t, fixture, "p"))
		assertRelUnsupported(t, model, memberships[3], "ownedMemberParameter")
		cd := relFixtureSymbol(t, fixture, "CD")
		for _, membership := range model.OwnedMemberships(cd) {
			assertRelMetaclass(t, model, membership, "EndFeatureMembership")
		}
		asm := relFixtureSymbol(t, fixture, "Asm")
		connector := model.OwnedMemberships(asm)[2].Implicit.Member
		for _, membership := range model.OwnedMemberships(connector) {
			assertRelMetaclass(t, model, membership, "EndFeatureMembership")
		}
	}
}

const requirementFixtureSrc = `package R {
    private import ScalarValues::*;
    part def Rover;
    part def Operator;
    concern def Safety;
    requirement def MassReq {
        doc /* The rover shall be light. */
        doc /* Second paragraph. */
        attribute massLimit : Real;
        subject rover : Rover;
        actor op : Operator;
        stakeholder owner : Operator;
        assume constraint { massLimit > 0.0 }
        require constraint tight { massLimit < 100.0 }
        frame concern safe : Safety;
    }
    requirement massReq : MassReq {
        doc /* Mass requirement for the rover. */
        require constraint extra { massLimit < 50.0 }
    }
    requirement unverified : MassReq;
    requirement bare;
    verification def MassTest {
        subject rover : Rover;
        objective { verify massReq; verify requirement inline : MassReq; }
    }
    verification massTest : MassTest;
    verification def Idle;
    verification idle : Idle;
    part rover : Rover {
        satisfy massReq by rover;
        satisfy unverified;
        assert constraint ac { true }
    }
}`

func TestRequirementTextAndConstraints(t *testing.T) {
	for _, fixture := range reflectiveFixtures(t, "requirements.sysml", source.KindSysML, requirementFixtureSrc) {
		model := fixture.model
		sf := source.New("requirements.sysml", []byte(requirementFixtureSrc))
		model.SetSourceText(func(doc string, span source.Span) string { return sf.Text(span) })
		massReqDef := relFixtureSymbol(t, fixture, "MassReq")
		massReq := relFixtureSymbol(t, fixture, "massReq")
		unverified := relFixtureSymbol(t, fixture, "unverified")
		bare := relFixtureSymbol(t, fixture, "bare")
		tight := relFixtureSymbol(t, fixture, "tight")
		extra := relFixtureSymbol(t, fixture, "extra")
		safe := relFixtureSymbol(t, fixture, "safe")
		op := relFixtureSymbol(t, fixture, "op")
		owner := relFixtureSymbol(t, fixture, "owner")

		if !massReqDef.Recorded() {
			// A record holds no documentation bodies to read text from.
			assertRelStrings(t, model, massReqDef, "text", "The rover shall be light.", "Second paragraph.")
			assertRelStrings(t, model, massReq, "text", "Mass requirement for the rover.")
			assertRelStrings(t, model, bare, "text")
		}
		if _, ok := model.ReflectiveFeatureValues(relFixtureSymbol(t, fixture, "Rover"), "text"); ok {
			t.Error("text is derived for a part definition")
		}

		// requiredConstraint counts the framed concern (kind = requirement); assumed is the assume.
		required, _ := model.ReflectiveElements(massReqDef, "requiredConstraint")
		if len(required) != 2 || required[0] != tight || required[1] != safe {
			t.Errorf("requiredConstraint of MassReq = %v, want [tight safe]", required)
		}
		assumed, _ := model.ReflectiveElements(massReqDef, "assumedConstraint")
		if len(assumed) != 1 || assumed[0] == tight {
			t.Errorf("assumedConstraint of MassReq = %v, want the one assume", assumed)
		}
		assertRelFeature(t, model, massReqDef, "framedConcern", safe)
		assertRelFeature(t, model, massReq, "requiredConstraint", extra)
		assertRelFeature(t, model, massReq, "assumedConstraint")
		// framedConcern reads featureMembership, so the inherited frame counts; required does not.
		assertRelFeature(t, model, massReq, "framedConcern", safe)
		assertRelFeature(t, model, unverified, "requiredConstraint")
		assertRelFeature(t, model, bare, "requiredConstraint")
		assertRelUnsupported(t, model, relFixtureSymbol(t, fixture, "Rover"), "requiredConstraint")

		assertRelFeature(t, model, massReqDef, "actorParameter", op)
		assertRelFeature(t, model, massReqDef, "stakeholderParameter", owner)
		assertRelFeature(t, model, massReq, "actorParameter", op)
		assertRelFeature(t, model, bare, "actorParameter")

		// The memberships themselves: kind and ends.
		var constraintMemberships []*symbols.Symbol
		for _, membership := range model.OwnedMemberships(massReqDef) {
			if model.reflectiveMetaclassConforms(membership, "RequirementConstraintMembership") {
				constraintMemberships = append(constraintMemberships, membership)
			}
		}
		if len(constraintMemberships) != 3 {
			t.Fatalf("MassReq has %d requirement constraint memberships, want 3", len(constraintMemberships))
		}
		assertRelMetaclass(t, model, constraintMemberships[0], "RequirementConstraintMembership")
		assertRelRef(t, model, constraintMemberships[0], "kind", "SysML::Systems::RequirementConstraintKind::assumption")
		assertRelRef(t, model, constraintMemberships[1], "kind", "SysML::Systems::RequirementConstraintKind::requirement")
		assertRelFeature(t, model, constraintMemberships[1], "ownedConstraint", tight)
		assertRelFeature(t, model, constraintMemberships[1], "referencedConstraint", tight)
		assertRelMetaclass(t, model, constraintMemberships[2], "FramedConcernMembership")
		assertRelFeature(t, model, constraintMemberships[2], "ownedConcern", safe)
		assertRelFeature(t, model, constraintMemberships[2], "referencedConcern", safe)
		subject := model.OwnedMemberships(massReqDef)[3]
		assertRelMetaclass(t, model, subject, "SubjectMembership")
		assertRelFeature(t, model, subject, "ownedSubjectParameter", relFixtureSymbol(t, fixture, "rover"))
		assertRelMetaclass(t, model, model.OwnedMemberships(massReqDef)[4], "ActorMembership")
		assertRelMetaclass(t, model, model.OwnedMemberships(massReqDef)[5], "StakeholderMembership")
	}
}

func TestVerifiedRequirementsAndSatisfaction(t *testing.T) {
	for _, fixture := range reflectiveFixtures(t, "verification.sysml", source.KindSysML, requirementFixtureSrc) {
		model := fixture.model
		massTestDef := relFixtureSymbol(t, fixture, "MassTest")
		massTest := relFixtureSymbol(t, fixture, "massTest")
		idle := relFixtureSymbol(t, fixture, "Idle")
		idleUsage := relFixtureSymbol(t, fixture, "idle")
		massReq := relFixtureSymbol(t, fixture, "massReq")
		inline := relFixtureSymbol(t, fixture, "inline")
		unverified := relFixtureSymbol(t, fixture, "unverified")

		// `verify massReq;` names massReq; `verify requirement inline : MassReq` is the owned requirement itself.
		assertRelFeature(t, model, massTestDef, "verifiedRequirement", massReq, inline)
		assertRelFeature(t, model, massTest, "verifiedRequirement", massReq, inline)
		assertRelFeature(t, model, idle, "verifiedRequirement")
		assertRelFeature(t, model, idleUsage, "verifiedRequirement")
		assertRelUnsupported(t, model, massReq, "verifiedRequirement")

		objectives, _ := model.ObjectivesOf(massTestDef)
		memberships := model.OwnedMemberships(objectives[0])
		if len(memberships) != 2 {
			t.Fatalf("objective owns %d memberships, want 2", len(memberships))
		}
		assertRelMetaclass(t, model, memberships[0], "RequirementVerificationMembership")
		assertRelFeature(t, model, memberships[0], "verifiedRequirement", massReq)
		assertRelFeature(t, model, memberships[0], "ownedRequirement", memberships[0].Implicit.Member)
		assertRelFeature(t, model, memberships[1], "verifiedRequirement", inline)
		assertRelRef(t, model, memberships[0], "kind", "SysML::Systems::RequirementConstraintKind::requirement")
		assertRelMetaclass(t, model, model.OwnedMemberships(massTestDef)[1], "ObjectiveMembership")

		rover := nestedSym(t, fixture.root, "R::rover")
		satisfies := model.OwnedMemberships(rover)
		if len(satisfies) != 3 {
			t.Fatalf("rover owns %d memberships, want 3", len(satisfies))
		}
		byRover := satisfies[0].Implicit.Member
		assertRelFeature(t, model, byRover, "satisfiedRequirement", massReq)
		assertRelFeature(t, model, byRover, "assertedConstraint", massReq)
		assertRelFeature(t, model, byRover, "satisfyingFeature", rover)
		plain := satisfies[1].Implicit.Member
		assertRelFeature(t, model, plain, "satisfiedRequirement", unverified)
		assertRelFeature(t, model, plain, "satisfyingFeature")
		assertRelUnsupported(t, model, satisfies[2].Implicit.Member, "satisfiedRequirement")
	}
}

func TestWrittenClauseRelationshipsBeyondSpecialization(t *testing.T) {
	src := `class A; class B;
class C disjoint from A unions A, B;
class D specializes A { feature f : A featured by B; }
feature g : A inverse of D::f;`
	for _, fixture := range reflectiveFixtures(t, "written-clauses.kerml", source.KindKerML, src) {
		model := fixture.model
		c := relFixtureSymbol(t, fixture, "C")
		a := relFixtureSymbol(t, fixture, "A")
		b := relFixtureSymbol(t, fixture, "B")
		rels := model.ImplicitRelationships(c)
		if len(rels) != 3 {
			t.Fatalf("C writes %d relationship objects, want 3", len(rels))
		}
		assertRelMetaclass(t, model, rels[0], "Disjoining")
		assertRelFeature(t, model, rels[0], "typeDisjoined", c)
		assertRelFeature(t, model, rels[0], "disjoiningType", a)
		assertRelMetaclass(t, model, rels[1], "Unioning")
		assertRelFeature(t, model, rels[1], "unioningType", a)
		assertRelFeature(t, model, rels[2], "unioningType", b)
		assertRelFeature(t, model, c, "ownedRelationship", append(rels, model.OwnedMemberships(c)...)...)
		f := relFixtureSymbol(t, fixture, "f")
		fRels := model.ImplicitRelationships(f)
		if len(fRels) != 2 {
			t.Fatalf("f writes %d relationship objects, want 2", len(fRels))
		}
		assertRelMetaclass(t, model, fRels[1], "TypeFeaturing")
		assertRelFeature(t, model, fRels[1], "featureOfType", f)
		assertRelFeature(t, model, fRels[1], "featuringType", b)
		g := relFixtureSymbol(t, fixture, "g")
		gRels := model.ImplicitRelationships(g)
		assertRelMetaclass(t, model, gRels[1], "FeatureInverting")
		assertRelFeature(t, model, gRels[1], "featureInverted", g)
		assertRelFeature(t, model, gRels[1], "invertingFeature", f)
	}
}

// TestMembershipImportTargetIsTheMembership pins KerML 8.3.2.5 MembershipImport:
// importedMembership redefines target, so both are the Membership the import
// names (an alias being one itself) and importedElement is its member.
func TestMembershipImportTargetIsTheMembership(t *testing.T) {
	const src = `package A { part def X; alias Y for X; }
package B { private import A::X; private import A::Y; }`
	for _, fixture := range reflectiveFixtures(t, "membership-import.sysml", source.KindSysML, src) {
		model := fixture.model
		a := relFixtureSymbol(t, fixture, "A")
		b := relFixtureSymbol(t, fixture, "B")
		x := relFixtureSymbol(t, fixture, "X")
		imports := model.OwnedImports(b)
		if len(imports) != 2 {
			t.Fatalf("B owns %d imports, want 2", len(imports))
		}
		xMembership := model.OwningMembership(x)
		if xMembership == nil {
			t.Fatal("X has no owning membership")
		}
		assertRelMetaclass(t, model, imports[0], "MembershipImport")
		assertRelFeature(t, model, imports[0], "importedMembership", xMembership)
		assertRelFeature(t, model, imports[0], "target", xMembership)
		assertRelFeature(t, model, imports[0], "importedElement", x)
		assertRelFeature(t, model, imports[0], "relatedElement", b, xMembership)
		assertRelUnsupported(t, model, imports[0], "importedNamespace")
		var alias *symbols.Symbol
		a.Scope.ForEachMember(func(member *symbols.Symbol) bool {
			if member.Kind == symbols.SymbolAlias {
				alias = member
			}
			return true
		})
		if alias == nil {
			t.Fatal("A has no alias member")
		}
		assertRelFeature(t, model, imports[1], "importedMembership", alias)
		assertRelFeature(t, model, imports[1], "target", alias)
		assertRelFeature(t, model, imports[1], "importedElement", x)
	}
}
