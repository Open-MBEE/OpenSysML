package reposync_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/interop/reposync"
)

// graphOf converts notation to the identity-carrying RDF graph the sync diffs.
func graphOf(t *testing.T, src string) *rdf.Graph {
	t.Helper()
	graph, err := export.SysMLToRDF("m.sysml", []byte(src))
	if err != nil {
		t.Fatalf("convert to RDF: %v", err)
	}
	return graph
}

// scoped wraps a body in the scoped package every fixture shares.
func scoped(body string) string {
	return `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
` + body + `}
`
}

const vehicle = `	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
`

// writeBack is reposync.WriteBack, failing the test rather than returning an error.
func writeBack(t *testing.T, g *rdf.Graph, mints map[string]string) *rdf.Graph {
	t.Helper()
	out, err := reposync.WriteBack(g, mints)
	if err != nil {
		t.Fatalf("write back: %v", err)
	}
	return out
}

// byKind indexes a change set's entries by kind.
func byKind(set *reposync.ChangeSet, kind reposync.Kind) []reposync.Change {
	var out []reposync.Change
	for _, change := range set.Changes {
		if change.Kind == kind {
			out = append(out, change)
		}
	}
	return out
}

func TestRenameIsAnUpdateKeyedByID(t *testing.T) {
	repository := graphOf(t, scoped(vehicle))
	local := graphOf(t, scoped(`	part def Car {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
`))
	set, err := reposync.Diff(local, repository, reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(byKind(set, reposync.KindDelete)) + len(byKind(set, reposync.KindCreate)); n != 0 {
		t.Errorf("a rename produced %d delete/create entries, want 0:\n%s", n, set.Text())
	}
	updates := byKind(set, reposync.KindUpdate)
	if len(updates) != 1 || updates[0].ID != "8f3a41d0" {
		t.Fatalf("a rename must be one update of the annotated id:\n%s", set.Text())
	}
	renamed := false
	for _, delta := range updates[0].Deltas {
		if delta.Property == "declaredName" {
			renamed = true
		}
	}
	if !renamed {
		t.Errorf("the update does not carry the name change:\n%s", set.Text())
	}
}

func TestRetypeIsAnUpdate(t *testing.T) {
	repository := graphOf(t, scoped(vehicle))
	local := graphOf(t, scoped(`	item def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
`))
	set, err := reposync.Diff(local, repository, reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	updates := byKind(set, reposync.KindUpdate)
	if len(updates) != 1 || updates[0].ID != "8f3a41d0" {
		t.Fatalf("a retype must be one update of the annotated id:\n%s", set.Text())
	}
	if updates[0].Deltas[0].Property != "rdf:type" {
		t.Errorf("the update does not lead with the metaclass change:\n%s", set.Text())
	}
}

func TestNewElementIsACreate(t *testing.T) {
	repository := graphOf(t, scoped(vehicle))
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	set, err := reposync.Diff(local, repository, reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	creates := byKind(set, reposync.KindCreate)
	found := false
	for _, change := range creates {
		if change.QualifiedName == "P::Wheel" {
			found = true
			if change.MintedID != "" {
				t.Errorf("an id was minted without the opt-in: %s", change.MintedID)
			}
		}
	}
	if !found {
		t.Errorf("the new element is not a create:\n%s", set.Text())
	}
	if n := set.Conflicts(); n != 0 {
		t.Errorf("a plain create raised %d conflict(s):\n%s", n, set.Text())
	}
}

func TestDeleteIsGatedBehindConfirmation(t *testing.T) {
	repository := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	local := graphOf(t, scoped(vehicle))
	set, err := reposync.Diff(local, repository, reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	deletes := byKind(set, reposync.KindDelete)
	if len(deletes) == 0 {
		t.Fatalf("the repository-only element is not reported as a delete:\n%s", set.Text())
	}
	for _, change := range deletes {
		if !change.RequiresConfirmation {
			t.Errorf("delete of %s is not gated behind confirmation", change.ID)
		}
	}
	if set.Appliable() == nil {
		t.Error("an unconfirmed delete did not refuse the apply")
	}

	confirmed, err := reposync.Diff(local, repository, reposync.Options{ConfirmDeletes: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range byKind(confirmed, reposync.KindDelete) {
		if change.RequiresConfirmation {
			t.Errorf("a confirmed delete still requires confirmation: %s", change.ID)
		}
	}
	if err := confirmed.Appliable(); err != nil {
		t.Errorf("a confirmed change set refused the apply: %v", err)
	}
}

func TestMissingIDIsAConflict(t *testing.T) {
	repository := graphOf(t, scoped(`	part def Other;
`))
	local := graphOf(t, scoped(vehicle))
	set, err := reposync.Diff(local, repository, reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	conflicts := byKind(set, reposync.KindConflict)
	if len(conflicts) != 1 || conflicts[0].Conflict != reposync.ConflictMissingID {
		t.Fatalf("a declared id the branch does not have must be a missing-id conflict:\n%s", set.Text())
	}
	if conflicts[0].ID != "8f3a41d0" {
		t.Errorf("the conflict names %s, want the annotated id", conflicts[0].ID)
	}
	if set.Appliable() == nil {
		t.Error("a conflicted change set did not refuse the apply")
	}
}

// TestNeverSeenDeclaredIDIsACreate gives the diff the last-seen graph: a
// declared id that never existed there is a genuinely new element.
func TestNeverSeenDeclaredIDIsACreate(t *testing.T) {
	base := graphOf(t, scoped(`	part def Other;
`))
	repository := graphOf(t, scoped(`	part def Other;
`))
	local := graphOf(t, scoped(vehicle))
	set, err := reposync.Diff(local, repository, reposync.Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if n := set.Conflicts(); n != 0 {
		t.Fatalf("a never-pushed declared id raised %d conflict(s):\n%s", n, set.Text())
	}
	found := false
	for _, change := range byKind(set, reposync.KindCreate) {
		if change.ID == "8f3a41d0" {
			found = true
		}
	}
	if !found {
		t.Errorf("the never-pushed declared id is not a create:\n%s", set.Text())
	}
}

func TestRepositoryChangeSinceLastSeenIsAConflict(t *testing.T) {
	base := graphOf(t, scoped(vehicle))
	repository := graphOf(t, scoped(`	part def Truck {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
`))
	local := graphOf(t, scoped(`	part def Car {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
`))
	set, err := reposync.Diff(local, repository, reposync.Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	conflicts := byKind(set, reposync.KindConflict)
	if len(conflicts) != 1 || conflicts[0].Conflict != reposync.ConflictRepositoryChanged {
		t.Fatalf("a repository change since the last-seen commit must be a conflict:\n%s", set.Text())
	}

	// Without the base graph the divergence cannot be seen; it reads as an update.
	unaware, err := reposync.Diff(local, repository, reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if n := unaware.Conflicts(); n != 0 {
		t.Errorf("without a base graph the diff invented %d conflict(s):\n%s", n, unaware.Text())
	}
}

// TestRepositoryAdditionSinceLastSeenIsAConflict: an element only the
// repository has, absent from the last-seen graph, was added there since —
// deleting it would erase someone else's addition.
func TestRepositoryAdditionSinceLastSeenIsAConflict(t *testing.T) {
	base := graphOf(t, scoped(vehicle))
	repository := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	local := graphOf(t, scoped(vehicle))
	set, err := reposync.Diff(local, repository, reposync.Options{Base: base, ConfirmDeletes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(byKind(set, reposync.KindDelete)) != 0 {
		t.Errorf("a repository-side addition was classified as a local delete:\n%s", set.Text())
	}
	if set.Conflicts() == 0 {
		t.Errorf("a repository-side addition raised no conflict:\n%s", set.Text())
	}
}

// TestRepositoryDeletionSinceLastSeenIsAConflict: an element only the local
// model has, but present in the last-seen graph, was deleted repository-side —
// re-creating it would undo that deletion.
func TestRepositoryDeletionSinceLastSeenIsAConflict(t *testing.T) {
	base := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	repository := graphOf(t, scoped(vehicle))
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	set, err := reposync.Diff(local, repository, reposync.Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if len(byKind(set, reposync.KindCreate)) != 0 {
		t.Errorf("a repository-side deletion was classified as a local create:\n%s", set.Text())
	}
	if set.Conflicts() == 0 {
		t.Errorf("a repository-side deletion raised no conflict:\n%s", set.Text())
	}
}

// TestScopedAndUnscopedIRIsCorrelateByID: the same effective id under a
// scope-qualified and a plain element IRI is one element, not create+delete.
func TestScopedAndUnscopedIRIsCorrelateByID(t *testing.T) {
	local := rdf.NewGraph()
	local.Add(rdf.ElementIRIForID("8f3a41d0"), rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"PartDefinition"))
	local.Add(rdf.ElementIRIForID("8f3a41d0"), rdf.IRI(rdf.SysML+"declaredName"), rdf.String("Car"))
	repository := rdf.NewGraph()
	scopedIRI := rdf.ScopedElementIRIForID(rdf.ScopeQualifier("", "proj-1"), "8f3a41d0")
	repository.Add(scopedIRI, rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"PartDefinition"))
	repository.Add(scopedIRI, rdf.IRI(rdf.SysML+"declaredName"), rdf.String("Vehicle"))
	set, err := reposync.Diff(local, repository, reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(byKind(set, reposync.KindCreate)) + len(byKind(set, reposync.KindDelete)); n != 0 {
		t.Errorf("one effective id under two IRI forms produced %d create/delete entries:\n%s", n, set.Text())
	}
	updates := byKind(set, reposync.KindUpdate)
	if len(updates) != 1 || updates[0].ID != "8f3a41d0" {
		t.Fatalf("one effective id under two IRI forms must be one update:\n%s", set.Text())
	}
}

// TestDuplicateEffectiveIDIsAnError: two subjects carrying one effective id in
// a single graph cannot be correlated and must be refused, not overwritten.
func TestDuplicateEffectiveIDIsAnError(t *testing.T) {
	local := rdf.NewGraph()
	local.Add(rdf.ElementIRIForID("8f3a41d0"), rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"PartDefinition"))
	scopedIRI := rdf.ScopedElementIRIForID(rdf.ScopeQualifier("", "proj-1"), "8f3a41d0")
	local.Add(scopedIRI, rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"PartDefinition"))
	_, err := reposync.Diff(local, rdf.NewGraph(), reposync.Options{})
	if err == nil || !strings.Contains(err.Error(), "effective id") {
		t.Fatalf("a duplicated effective id was not refused: %v", err)
	}
}

// TestConcurrentAdditionIsAConflict: both sides added the same id since the
// last-seen graph with differing content; writing over it would lose the
// repository's addition.
func TestConcurrentAdditionIsAConflict(t *testing.T) {
	build := func(name string) *rdf.Graph {
		g := rdf.NewGraph()
		subject := rdf.ElementIRIForID("8f3a41d0")
		g.Add(subject, rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"PartDefinition"))
		g.Add(subject, rdf.IRI(rdf.SysML+"declaredName"), rdf.String(name))
		return g
	}
	set, err := reposync.Diff(build("Mine"), build("Theirs"), reposync.Options{Base: rdf.NewGraph()})
	if err != nil {
		t.Fatal(err)
	}
	conflicts := byKind(set, reposync.KindConflict)
	if len(conflicts) != 1 || conflicts[0].Conflict != reposync.ConflictRepositoryChanged {
		t.Fatalf("a concurrent addition must be a repository-changed conflict:\n%s", set.Text())
	}
	// Agreeing concurrent additions stay a no-op.
	same, err := reposync.Diff(build("Same"), build("Same"), reposync.Options{Base: rdf.NewGraph()})
	if err != nil {
		t.Fatal(err)
	}
	if len(same.Changes) != 0 {
		t.Fatalf("agreeing concurrent additions must not change anything:\n%s", same.Text())
	}
}

// TestScopedReferencesAreNotUpdates: a reference compares by its target's
// effective id, so scoped and unscoped spellings of one link are equal.
func TestScopedReferencesAreNotUpdates(t *testing.T) {
	ref := func(iri rdf.Term) *rdf.Graph {
		g := rdf.NewGraph()
		subject := rdf.ElementIRIForID("8f3a41d0")
		g.Add(subject, rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"PartDefinition"))
		g.Add(subject, rdf.IRI(rdf.SysML+"owner"), iri)
		return g
	}
	local := ref(rdf.ElementIRIForID("P__Owner"))
	repository := ref(rdf.ScopedElementIRIForID(rdf.ScopeQualifier("", "proj-1"), "P__Owner"))
	set, err := reposync.Diff(local, repository, reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Changes) != 0 {
		t.Errorf("one reference under two IRI spellings produced %d change(s):\n%s", len(set.Changes), set.Text())
	}
}

// TestSameNamedPredicatesAreNotConflated: two predicates sharing a local name
// compare independently, so a change to one is its own delta.
func TestSameNamedPredicatesAreNotConflated(t *testing.T) {
	build := func(a, b string) *rdf.Graph {
		g := rdf.NewGraph()
		subject := rdf.ElementIRIForID("8f3a41d0")
		g.Add(subject, rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"PartDefinition"))
		g.Add(subject, rdf.IRI(rdf.SysML+"value"), rdf.String(a))
		g.Add(subject, rdf.IRI("https://example.org/other#value"), rdf.String(b))
		return g
	}
	set, err := reposync.Diff(build("1", "x"), build("2", "x"), reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	updates := byKind(set, reposync.KindUpdate)
	if len(updates) != 1 || len(updates[0].Deltas) != 1 || updates[0].Deltas[0].Property != "value" {
		t.Fatalf("changing one of two same-named predicates must be exactly its own delta:\n%s", set.Text())
	}
}

// TestWriteBackLeavesPrefixSharingNeighborsAlone: an element whose id merely
// begins with a minted id plus `_om`/`_p` text is not that element's satellite.
func TestWriteBackLeavesPrefixSharingNeighborsAlone(t *testing.T) {
	g := rdf.NewGraph()
	minted := rdf.ElementIRIForID("P__A")
	g.Add(minted, rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"PartDefinition"))
	for _, neighbor := range []string{"P__A_omg", "P__A_persist"} {
		g.Add(rdf.ElementIRIForID(neighbor), rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"PartDefinition"))
		g.Add(rdf.ElementIRIForID(neighbor), rdf.IRI(rdf.SysML+"elementId"), rdf.String(neighbor))
	}
	membership := rdf.OwningMembershipIRIOf(minted)
	g.Add(membership, rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"OwningMembership"))

	out := writeBack(t, g, map[string]string{"P__A": "11111111-2222-4333-8444-555555555555"})
	for _, neighbor := range []string{"P__A_omg", "P__A_persist"} {
		if _, ok := out.Object(rdf.ElementIRIForID(neighbor), rdf.RDFType); !ok {
			t.Errorf("write-back moved the unrelated element %s", neighbor)
		}
	}
	moved := rdf.ElementIRIForID("11111111-2222-4333-8444-555555555555_om")
	if _, ok := out.Object(moved, rdf.RDFType); !ok {
		t.Error("write-back did not move the minted element's own membership")
	}
}

func TestBaselineScopeMismatchIsAnError(t *testing.T) {
	local := graphOf(t, scoped(vehicle))
	repository := graphOf(t, scoped(vehicle))
	otherProject := graphOf(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-2"; branch = "main"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
}
`)
	if _, err := reposync.Diff(local, repository, reposync.Options{Base: otherProject}); err == nil {
		t.Error("a baseline from another project was not refused")
	}
	otherBranch := graphOf(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "dev"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
}
`)
	if _, err := reposync.Diff(local, repository, reposync.Options{Base: otherBranch}); err == nil {
		t.Error("a baseline from another branch was not refused")
	}
}

func TestMintingIsExplicit(t *testing.T) {
	repository := graphOf(t, scoped(vehicle))
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	minted, err := reposync.Diff(local, repository, reposync.Options{
		MintIDs: true,
		NewID:   func() (string, error) { return "minted-uuid-1", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	mints := minted.Mints()
	if mints["P__Wheel"] != "minted-uuid-1" {
		t.Fatalf("the unannotated create was not minted an id:\n%s", minted.Text())
	}
	for _, change := range minted.Changes {
		if change.MintedID != "" && strings.HasSuffix(change.ID, "_om") {
			t.Errorf("a membership was minted an id of its own: %s", change.ID)
		}
	}
}

func TestWriteBackDeclaresMintedIDs(t *testing.T) {
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	rewritten := writeBack(t, local, map[string]string{"P__Wheel": "11111111-2222-4333-8444-555555555555"})
	turtle := rdf.WriteTurtle(rewritten)
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("read the rewritten graph back: %v\n%s", err, turtle)
	}
	if !strings.Contains(string(back), `@IdentityMetadata::ElementId { id = "11111111-2222-4333-8444-555555555555"; }`) {
		t.Errorf("the minted id was not written back as an annotation:\n%s", back)
	}
	// The rewritten notation must round-trip: the minted id is now declared.
	again := graphOf(t, string(back))
	if _, ok := again.Object(rdf.IRI(rdf.Element+"11111111-2222-4333-8444-555555555555"), rdf.RDFType); !ok {
		t.Errorf("the annotated notation does not carry the minted id:\n%s", back)
	}
}

// The collection annotations follow their typed triples onto the minted ids, the
// owning membership's derived id included, so the two spellings still agree.
func TestWriteBackMovesCollectionAnnotations(t *testing.T) {
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	minted := "11111111-2222-4333-8444-555555555555"
	out := writeBack(t, local, map[string]string{"P__Wheel": minted})
	for key, want := range map[string]string{
		"ownedMember":       `{"@id":"` + minted + `"}`,
		"ownedMembership":   `{"@id":"` + minted + `_om"}`,
		"ownedRelationship": `{"@id":"` + minted + `_om"}`,
	} {
		object, ok := out.Object(rdf.ElementIRIForID("P"), rdf.AnnotationJSON+key)
		if !ok || !strings.Contains(object.Value, want) || strings.Contains(object.Value, "P__Wheel") {
			t.Errorf("json:%s = %s, want a member %s and no P__Wheel", key, object.Value, want)
		}
	}
}

// A declared element id that extends a minted id with `_p` text is not that
// element's expression node: the annotation follows the typed triple, which keeps
// it, so the written-back graph still reads and diffs clean.
func TestWriteBackKeepsDeclaredSiblingsInAnnotations(t *testing.T) {
	local := graphOf(t, scoped(`	part def A;
	part def B {
		@IdentityMetadata::ElementId { id = "P__A_pchild"; }
	}
`))
	minted := "11111111-2222-4333-8444-555555555555"
	out := writeBack(t, local, map[string]string{"P__A": minted})
	object, ok := out.Object(rdf.ElementIRIForID("P"), rdf.AnnotationJSON+"ownedMember")
	if want := `[{"@id":"` + minted + `"},{"@id":"P__A_pchild"}]`; !ok || object.Value != want {
		t.Errorf("json:ownedMember = %s, want %s", object.Value, want)
	}
	if _, err := rdf.ReconcileCollections(out); err != nil {
		t.Errorf("the written-back graph does not reconcile: %v", err)
	}
	turtle := rdf.WriteTurtle(out)
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("read the rewritten graph back: %v\n%s", err, turtle)
	}
	if !strings.Contains(string(back), `id = "P__A_pchild"`) || !strings.Contains(string(back), `id = "`+minted+`"`) {
		t.Errorf("the notation lost a declared or minted id:\n%s", back)
	}
}

// A collection stated by the annotation alone is written back with the typed
// triples it stands for, both on the minted ids.
func TestWriteBackMintsAnnotationOnlyCollections(t *testing.T) {
	local := withoutTypedCollections(graphOf(t, scoped(vehicle+`	part def Wheel;
`)))
	minted := "11111111-2222-4333-8444-555555555555"
	out := writeBack(t, local, map[string]string{"P__Wheel": minted})
	packageIRI := rdf.ElementIRIForID("P")
	members := out.Objects(packageIRI, rdf.SysML+"ownedMember")
	if len(members) != 2 || members[1] != rdf.ElementIRIForID(minted) {
		t.Errorf("sysml:ownedMember = %v, want the vehicle then %s", members, minted)
	}
	object, _ := out.Object(packageIRI, rdf.AnnotationJSON+"ownedMember")
	if !strings.Contains(object.Value, `{"@id":"`+minted+`"}`) || strings.Contains(object.Value, "P__Wheel") {
		t.Errorf("json:ownedMember = %s, want the minted id and no P__Wheel", object.Value)
	}
}

func TestTwoBranchesInOneDocumentIsAnError(t *testing.T) {
	local := graphOf(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "a"; }
	part def A {
		@IdentityMetadata::ElementId { id = "id-a"; }
	}
}
package Q {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "b"; }
	part def B {
		@IdentityMetadata::ElementId { id = "id-b"; }
	}
}
`)
	_, err := reposync.Diff(local, rdf.NewGraph(), reposync.Options{})
	if err == nil {
		t.Fatal("two branches in one document were not refused")
	}
	if !strings.Contains(err.Error(), "two branches") {
		t.Errorf("two branches refused for the wrong reason: %v", err)
	}
}

func TestBranchMismatchAgainstRepositoryIsAnError(t *testing.T) {
	local := graphOf(t, scoped(vehicle))
	repository := graphOf(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "dev"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
}
`)
	_, err := reposync.Diff(local, repository, reposync.Options{})
	if err == nil {
		t.Fatal("a branch mismatch between model and repository graph was not refused")
	}
}

func TestUnchangedModelIsEmptyDiff(t *testing.T) {
	graph := graphOf(t, scoped(vehicle))
	set, err := reposync.Diff(graph, graphOf(t, scoped(vehicle)), reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Changes) != 0 {
		t.Errorf("identical graphs produced %d change(s):\n%s", len(set.Changes), set.Text())
	}
}

// withoutTypedCollections drops the typed triples of every annotated collection,
// leaving the graph a reader of the annotation alone would produce.
func withoutTypedCollections(g *rdf.Graph) *rdf.Graph {
	out := rdf.NewGraph()
	for _, triple := range g.Triples() {
		key, isSysML := strings.CutPrefix(triple.Predicate.Value, rdf.SysML)
		if isSysML && g.HasProperty(triple.Subject, rdf.AnnotationJSON+key) {
			continue
		}
		out.AddTriple(triple)
	}
	return out
}

func TestAnnotationOnlyCollectionsCompareEqual(t *testing.T) {
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	repository := withoutTypedCollections(local)
	if repository.Len() == local.Len() {
		t.Fatal("the fixture states no collection")
	}
	set, err := reposync.Diff(local, repository, reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Changes) != 0 || len(set.Uncarried) != 0 {
		t.Errorf("a collection stated by annotation alone differs from one stated by both:\n%s", set.Text())
	}
}

func TestConflictingCollectionAnnotationIsAnError(t *testing.T) {
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	broken := rdf.NewGraph()
	for _, triple := range local.Triples() {
		if triple.Predicate.Value == rdf.AnnotationJSON+"ownedMember" {
			triple.Object = rdf.String(`[{"@id":"P__Wheel"}]`)
		}
		broken.AddTriple(triple)
	}
	_, err := reposync.Diff(broken, local, reposync.Options{})
	var conflict *rdf.CollectionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want a CollectionConflictError, got %v", err)
	}
	if conflict.Key != "ownedMember" || !strings.HasSuffix(conflict.Subject, ":P") {
		t.Errorf("the error names <%s> sysml:%s, want <…:P> sysml:ownedMember", conflict.Subject, conflict.Key)
	}
}

func TestFirstPushIsAllCreates(t *testing.T) {
	local := graphOf(t, scoped(vehicle))
	set, err := reposync.Diff(local, rdf.NewGraph(), reposync.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if n := set.Conflicts(); n != 0 {
		t.Errorf("a first push into an empty repository raised %d conflict(s):\n%s", n, set.Text())
	}
	if len(byKind(set, reposync.KindCreate)) != len(set.Changes) {
		t.Errorf("a first push must be creates only:\n%s", set.Text())
	}
}

func TestMintUUIDShape(t *testing.T) {
	id, err := reposync.MintUUID()
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(id, "-")
	want := []int{8, 4, 4, 4, 12}
	if len(parts) != len(want) {
		t.Fatalf("minted id %q is not a UUID", id)
	}
	for i, part := range parts {
		if len(part) != want[i] {
			t.Fatalf("minted id %q is not a UUID", id)
		}
	}
	if id[14] != '4' {
		t.Errorf("minted id %q is not version 4", id)
	}
}

func TestTextIsDeterministic(t *testing.T) {
	repository := graphOf(t, scoped(vehicle+`	part def Gone;
`))
	local := graphOf(t, scoped(`	part def Car {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
	part def Wheel;
`))
	first := ""
	for i := 0; i < 5; i++ {
		set, err := reposync.Diff(local, repository, reposync.Options{
			MintIDs: true,
			NewID:   func() (string, error) { return fmt.Sprintf("stable-%d", 0), nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = set.Text()
			continue
		}
		if set.Text() != first {
			t.Fatalf("the report is not deterministic:\n--- first ---\n%s--- now ---\n%s", first, set.Text())
		}
	}
}

// A standard-library element carries the id the norm fixes for it: not an
// annotation, so the repository lacking it is a create, and not minted for.
func TestNormativeLibraryIDIsNeitherDeclaredNorMinted(t *testing.T) {
	const real = "14c0aa22-5489-59b5-b438-ded26e83ba31" // ScalarValues::Real
	local := rdf.NewGraph()
	subject := rdf.ElementIRIForID(real)
	local.Add(subject, rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"DataType"))
	local.Add(subject, rdf.IRI(rdf.SysML+"qualifiedName"), rdf.String("ScalarValues::Real"))
	set, err := reposync.Diff(local, rdf.NewGraph(), reposync.Options{
		MintIDs: true,
		NewID:   func() (string, error) { return "minted-uuid-1", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	creates := byKind(set, reposync.KindCreate)
	if len(creates) != 1 || set.Conflicts() != 0 {
		t.Fatalf("the library element must be one plain create:\n%s", set.Text())
	}
	if change := creates[0]; change.Declared || !change.Normative || change.MintedID != "" {
		t.Errorf("declared %v normative %v minted %q; want the norm's id as is", change.Declared, change.Normative, change.MintedID)
	}
}

// A user element that carries a library uuid, as a foreign graph may state
// without declaredId, is not the library element: its id was declared, not fixed
// by the norm, and stays declared rather than derived on write-back.
func TestUserElementWithLibraryIDIsDeclaredNotNormative(t *testing.T) {
	for _, id := range []string{
		"14c0aa22-5489-59b5-b438-ded26e83ba31", // ScalarValues::Real
		"ab72a695-5fe9-58a3-9d48-9e9a8711862d", // ScalarValues::Real's owning membership
	} {
		local := rdf.NewGraph()
		subject := rdf.ElementIRIForID(id)
		local.Add(subject, rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+"PartDefinition"))
		local.Add(subject, rdf.IRI(rdf.SysML+"qualifiedName"), rdf.String("P::A"))
		set, err := reposync.Diff(local, rdf.NewGraph(), reposync.Options{
			MintIDs: true,
			NewID:   func() (string, error) { return "minted-uuid-1", nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		creates := byKind(set, reposync.KindCreate)
		if len(creates) != 1 || set.Conflicts() != 0 {
			t.Fatalf("P::A must be one plain create:\n%s", set.Text())
		}
		if change := creates[0]; !change.Declared || change.Normative || change.MintedID != "" {
			t.Errorf("%s on P::A: declared %v normative %v minted %q; want a declared id", id, change.Declared, change.Normative, change.MintedID)
		}
	}
}
