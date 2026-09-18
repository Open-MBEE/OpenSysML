package reposync_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/interop/reposync"
)

// fakeRepository holds elements by id the way the SysML v2 service does: a
// commit replaces each named element whole, and a null payload removes it.
type fakeRepository struct {
	graph   *rdf.Graph
	commits [][]reposync.ElementChange
	failAt  int  // 1-based commit to refuse; 0 refuses none
	unnamed bool // answer commits without an id
}

func newFakeRepository(graph *rdf.Graph) *fakeRepository {
	if graph == nil {
		graph = rdf.NewGraph()
	}
	return &fakeRepository{graph: graph}
}

func (f *fakeRepository) Commit(_ context.Context, changes []reposync.ElementChange, _ string) (string, error) {
	n := len(f.commits) + 1
	if f.failAt == n {
		return "", fmt.Errorf("commit %d refused by the fake", n)
	}
	replaced := map[string]bool{}
	for _, change := range changes {
		replaced[change.ID] = true
	}
	next := rdf.NewGraph()
	for _, triple := range f.graph.Triples() {
		if !replaced[rdf.LocalName(triple.Subject.Value)] {
			next.AddTriple(triple)
		}
	}
	for _, change := range changes {
		for _, triple := range change.Content {
			triple.Subject = rdf.IRI(rdf.Element + change.ID)
			next.AddTriple(triple)
		}
	}
	f.graph = next
	f.commits = append(f.commits, changes)
	if f.unnamed {
		return "", nil
	}
	return fmt.Sprintf("c%d", n), nil
}

// sent lists every element change the fake was asked for, in order.
func (f *fakeRepository) sent() []reposync.ElementChange {
	var all []reposync.ElementChange
	for _, batch := range f.commits {
		all = append(all, batch...)
	}
	return all
}

func graphFrom(triples []rdf.Triple) *rdf.Graph {
	g := rdf.NewGraph()
	for _, triple := range triples {
		g.AddTriple(triple)
	}
	return g
}

func diffOf(t *testing.T, local, repository *rdf.Graph, opts reposync.Options) *reposync.ChangeSet {
	t.Helper()
	set, err := reposync.Diff(local, repository, opts)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// assertEmptyDiff checks that a second diff against the repository finds
// nothing: the apply left the branch saying what the model says.
func assertEmptyDiff(t *testing.T, local *rdf.Graph, repo *fakeRepository) {
	t.Helper()
	again := diffOf(t, local, repo.graph, reposync.Options{ConfirmDeletes: true})
	if len(again.Changes) != 0 {
		t.Errorf("re-diff after apply is not empty:\n%s", again.Text())
	}
}

func TestApplyRenameIsOneUpdateOfTheRetainedID(t *testing.T) {
	repo := newFakeRepository(graphOf(t, scoped(vehicle)))
	local := graphOf(t, scoped(`	part def Car {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
`))
	set := diffOf(t, local, repo.graph, reposync.Options{})

	result, err := reposync.Apply(context.Background(), repo, set, reposync.ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sent := repo.sent()
	if len(sent) != 1 || sent[0].Kind != reposync.KindUpdate || sent[0].ID != "8f3a41d0" {
		t.Fatalf("a rename must reach the repository as one update of the retained id, got %+v", sent)
	}
	renamed := false
	for _, triple := range sent[0].Content {
		if triple.Predicate.Value == rdf.SysML+"declaredName" && triple.Object.Value == "Car" {
			renamed = true
		}
	}
	if !renamed {
		t.Errorf("the update does not carry the new name: %+v", sent[0].Content)
	}
	if result.LastCommit() != "c1" || !result.Complete() || len(result.Applied) != 1 {
		t.Errorf("result does not record the one completed commit: %+v", result)
	}
	assertEmptyDiff(t, local, repo)
}

func TestApplyCreatesNewElements(t *testing.T) {
	repo := newFakeRepository(graphOf(t, scoped(vehicle)))
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	set := diffOf(t, local, repo.graph, reposync.Options{})

	if _, err := reposync.Apply(context.Background(), repo, set, reposync.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	created := false
	for _, change := range repo.sent() {
		switch change.Kind {
		case reposync.KindCreate:
			if change.ID == "P__Wheel" {
				created = true
				if _, ok := graphFrom(change.Content).Object(rdf.IRI(rdf.Element+"P__Wheel"), rdf.RDFType); !ok {
					t.Errorf("the create carries no type: %+v", change.Content)
				}
			}
		case reposync.KindDelete:
			t.Errorf("a create-only set sent a delete of %s", change.ID)
		}
	}
	if !created {
		t.Fatalf("the new element was not created: %+v", repo.sent())
	}
	assertEmptyDiff(t, local, repo)
}

func TestApplyRefusesUnconfirmedDeletes(t *testing.T) {
	repo := newFakeRepository(graphOf(t, scoped(vehicle+`	part def Wheel;
`)))
	local := graphOf(t, scoped(vehicle))
	set := diffOf(t, local, repo.graph, reposync.Options{})

	result, err := reposync.Apply(context.Background(), repo, set, reposync.ApplyOptions{})
	var refused *reposync.NotAppliableError
	if !errors.As(err, &refused) {
		t.Fatalf("unconfirmed deletes were not refused with a NotAppliableError: %v", err)
	}
	if refused.UnconfirmedDeletes == 0 || refused.Conflicts != 0 {
		t.Errorf("refusal misreports its reasons: %+v", refused)
	}
	if result != nil || len(repo.commits) != 0 {
		t.Errorf("a refused set still reached the repository: %d commit(s)", len(repo.commits))
	}
}

func TestApplyConfirmedDeletesRemoveElements(t *testing.T) {
	repo := newFakeRepository(graphOf(t, scoped(vehicle+`	part def Wheel;
`)))
	local := graphOf(t, scoped(vehicle))
	set := diffOf(t, local, repo.graph, reposync.Options{ConfirmDeletes: true})

	if _, err := reposync.Apply(context.Background(), repo, set, reposync.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	deleted := false
	for _, change := range repo.sent() {
		if change.Kind == reposync.KindDelete && change.ID == "P__Wheel" {
			deleted = true
			if change.Content != nil {
				t.Errorf("a delete carries content: %+v", change.Content)
			}
		}
	}
	if !deleted {
		t.Fatalf("the confirmed delete was not sent: %+v", repo.sent())
	}
	if _, ok := repo.graph.Object(rdf.IRI(rdf.Element+"P__Wheel"), rdf.RDFType); ok {
		t.Error("the deleted element is still in the repository")
	}
	assertEmptyDiff(t, local, repo)
}

func TestApplyRefusesConflicts(t *testing.T) {
	repo := newFakeRepository(graphOf(t, scoped(`	part def Wheel;
`)))
	local := graphOf(t, scoped(vehicle))
	set := diffOf(t, local, repo.graph, reposync.Options{ConfirmDeletes: true})
	if set.Conflicts() == 0 {
		t.Fatalf("fixture: a declared id the branch lacks must conflict:\n%s", set.Text())
	}

	_, err := reposync.Apply(context.Background(), repo, set, reposync.ApplyOptions{})
	var refused *reposync.NotAppliableError
	if !errors.As(err, &refused) || refused.Conflicts != 1 {
		t.Fatalf("a conflict was not refused with a NotAppliableError naming it: %v", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("a conflicting set reached the repository: %d commit(s)", len(repo.commits))
	}
}

func TestApplyAdvancesStateFromTheRepositoryCommit(t *testing.T) {
	repo := newFakeRepository(graphOf(t, scoped(vehicle)))
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	set := diffOf(t, local, repo.graph, reposync.Options{})
	state := &reposync.State{ProjectID: "proj-1", Branch: "main", LastSeenCommit: "c0"}

	result, err := reposync.Apply(context.Background(), repo, set, reposync.ApplyOptions{BatchSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Advance(result); err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("c%d", len(repo.commits)); state.LastSeenCommit != want || len(repo.commits) < 2 {
		t.Errorf("state advanced to %q, want the newest of %d commits %q", state.LastSeenCommit, len(repo.commits), want)
	}
	assertEmptyDiff(t, local, repo)
}

func TestApplyNoOpWritesNothing(t *testing.T) {
	repo := newFakeRepository(graphOf(t, scoped(vehicle)))
	local := graphOf(t, scoped(vehicle))
	set := diffOf(t, local, repo.graph, reposync.Options{})
	state := &reposync.State{ProjectID: "proj-1", LastSeenCommit: "c0"}

	result, err := reposync.Apply(context.Background(), repo, set, reposync.ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.commits) != 0 || result.LastCommit() != "" || !result.Complete() {
		t.Errorf("an empty set wrote %d commit(s), result %+v", len(repo.commits), result)
	}
	if err := state.Advance(result); err != nil || state.LastSeenCommit != "c0" {
		t.Errorf("a no-op moved the state: %v, %+v", err, state)
	}
}

func TestApplyReportsPartialFailure(t *testing.T) {
	repo := newFakeRepository(graphOf(t, scoped(vehicle)))
	repo.failAt = 2
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
	part def Axle;
	part def Frame;
`))
	set := diffOf(t, local, repo.graph, reposync.Options{})
	if len(set.Changes) < 3 {
		t.Fatalf("fixture: want at least three changes to batch, got %d", len(set.Changes))
	}
	state := &reposync.State{ProjectID: "proj-1", LastSeenCommit: "c0"}

	result, err := reposync.Apply(context.Background(), repo, set, reposync.ApplyOptions{BatchSize: 1})
	var partial *reposync.ApplyError
	if !errors.As(err, &partial) {
		t.Fatalf("a refused batch was not reported as an ApplyError: %v", err)
	}
	if partial.Batch != 2 || partial.Batches != len(set.Changes) {
		t.Errorf("refusal names batch %d of %d, want 2 of %d", partial.Batch, partial.Batches, len(set.Changes))
	}
	if len(result.Applied) != 1 || len(result.Failed) != 1 || len(result.Pending) != len(set.Changes)-2 {
		t.Errorf("fates: applied %d, failed %d, pending %d", len(result.Applied), len(result.Failed), len(result.Pending))
	}
	if result.Applied[0].ID != set.Changes[0].ID || result.Failed[0].ID != set.Changes[1].ID {
		t.Errorf("fates name the wrong changes: applied %s, failed %s", result.Applied[0].ID, result.Failed[0].ID)
	}
	if result.LastCommit() != "c1" || result.Complete() {
		t.Errorf("result claims more than the one commit that landed: %+v", result)
	}
	if err := state.Advance(result); !errors.Is(err, reposync.ErrIncompleteApply) || state.LastSeenCommit != "c0" {
		t.Errorf("state advanced past a partial apply: %v, %+v", err, state)
	}
}

func TestApplyUnnamedCommitIsAnError(t *testing.T) {
	repo := newFakeRepository(graphOf(t, scoped(vehicle)))
	repo.unnamed = true
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	set := diffOf(t, local, repo.graph, reposync.Options{})

	result, err := reposync.Apply(context.Background(), repo, set, reposync.ApplyOptions{})
	if !errors.Is(err, reposync.ErrUnnamedCommit) {
		t.Fatalf("an unnamed commit was not reported: %v", err)
	}
	if len(result.Applied) != len(set.Changes) || result.LastCommit() != "" {
		t.Errorf("the writes that landed are misreported: %+v", result)
	}
	if err := (&reposync.State{}).Advance(result); err != nil {
		t.Errorf("a complete apply the repository did not name refused to advance: %v", err)
	}
}

func TestApplyWritesMintedIDs(t *testing.T) {
	repo := newFakeRepository(graphOf(t, scoped(vehicle)))
	local := graphOf(t, scoped(vehicle+`	part def Wheel;
`))
	set := diffOf(t, local, repo.graph, reposync.Options{
		MintIDs: true,
		NewID:   func() (string, error) { return "minted-uuid-1", nil },
	})

	if _, err := reposync.Apply(context.Background(), repo, set, reposync.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, change := range repo.sent() {
		ids[change.ID] = true
		if change.ID == "P__Wheel" {
			t.Error("the create was sent under the derived id, not the minted one")
		}
	}
	if !ids["minted-uuid-1"] || !ids["minted-uuid-1_om"] {
		t.Errorf("the minted id and its membership were not written: %v", ids)
	}
	minted := rdf.IRI(rdf.Element + "minted-uuid-1")
	if id, _ := repo.graph.Lexical(minted, rdf.SysML+"elementId"); id != "minted-uuid-1" {
		t.Errorf("the repository's elementId is %q, want the minted id", id)
	}
	if !repo.graph.HasProperty(minted, rdf.RDFType) {
		t.Error("the minted element is not in the repository")
	}
	// The annotated model, which now declares the minted id, has nothing left to sync.
	assertEmptyDiff(t, writeBack(t, local, set.Mints()), repo)
}

func TestApplyRefusesContentlessChange(t *testing.T) {
	set := &reposync.ChangeSet{Changes: []reposync.Change{{Kind: reposync.KindCreate, ID: "x", Subject: rdf.Element + "x"}}}
	repo := newFakeRepository(nil)
	_, err := reposync.Apply(context.Background(), repo, set, reposync.ApplyOptions{})
	var unwritable *reposync.UnwritableChangeError
	if !errors.As(err, &unwritable) || unwritable.ID != "x" {
		t.Fatalf("a create without content was not refused: %v", err)
	}
	if len(repo.commits) != 0 {
		t.Error("a refused set reached the repository")
	}
}
