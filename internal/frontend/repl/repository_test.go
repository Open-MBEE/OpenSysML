package repl

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
)

const vehicles = `package Vehicles {
	part def Wheel { attribute radius : ScalarValues::Real; }
	part def Car { part wheels : Wheel[4]; }
}
package Spare { part def Boat; }
`

// repoSession is a session addressing a fake server, started with the
// environment the commands read their default from.
func repoSession(t *testing.T) (*Session, *fakeAPI) {
	t.Helper()
	if replext.Repo() == nil {
		t.Fatal("no repository extension is linked")
	}
	api := newFakeAPI()
	t.Cleanup(api.Close)
	t.Setenv("FLEXO_SYSMLV2_URL", api.URL())
	t.Setenv("FLEXO_INTEROP_TOKEN", "")
	return NewSession(), api
}

func runMeta(t *testing.T, s *Session, line string) []string {
	t.Helper()
	out, _, err := s.RunMeta(line)
	if err != nil {
		t.Fatalf("%s: %v\n%s", line, err, strings.Join(out, "\n"))
	}
	return out
}

func metaErr(t *testing.T, s *Session, line string) error {
	t.Helper()
	out, _, err := s.RunMeta(line)
	if err == nil {
		t.Fatalf("%s succeeded:\n%s", line, strings.Join(out, "\n"))
	}
	return err
}

func wantUsage(t *testing.T, err error, line string) {
	t.Helper()
	var usage *UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("%s: %v is no *UsageError", line, err)
	}
	if len(usage.Lines) == 0 || !strings.HasPrefix(usage.Lines[0], "usage: ") {
		t.Errorf("%s: usage error without a usage line: %q", line, usage.Lines)
	}
}

func joined(lines []string) string { return strings.Join(lines, "\n") }

func TestRepoShowsAndSetsTheBaseURL(t *testing.T) {
	s, api := repoSession(t)
	if got := joined(runMeta(t, s, "%repo")); got != "API base path: "+api.URL() {
		t.Errorf("%%repo = %q, want the environment's URL", got)
	}
	if got := joined(runMeta(t, s, "%repo http://127.0.0.1:1/api/")); got != "API base path: http://127.0.0.1:1/api" {
		t.Errorf("%%repo <url> = %q", got)
	}
	if got := joined(runMeta(t, s, "%repo")); got != "API base path: http://127.0.0.1:1/api" {
		t.Errorf("%%repo after setting = %q", got)
	}
	t.Setenv("FLEXO_ALLOW_PLAIN_HTTP", "")
	if err := metaErr(t, s, "%repo http://models.example.com/api"); !strings.Contains(err.Error(), "FLEXO_ALLOW_PLAIN_HTTP") {
		t.Errorf("plain HTTP off the loopback was taken: %v", err)
	}
	if got := joined(runMeta(t, s, "%repo")); got != "API base path: http://127.0.0.1:1/api" {
		t.Errorf("a refused URL was kept: %q", got)
	}
	wantUsage(t, metaErr(t, s, "%repo a b"), "%repo a b")
	if help := joined(runMeta(t, s, "%help")); !strings.Contains(help, "FLEXO_INTEROP_TOKEN") || strings.Contains(help, "secret") {
		t.Error("help does not say the token comes from FLEXO_INTEROP_TOKEN")
	}
}

func TestProjectsListsEveryPage(t *testing.T) {
	s, api := repoSession(t)
	if got := joined(runMeta(t, s, "%projects")); got != "no projects" {
		t.Errorf("empty repository: %q", got)
	}
	for _, name := range []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"} {
		api.addProject(name)
	}
	out := runMeta(t, s, "%projects")
	if len(out) != 5 {
		t.Fatalf("%%projects listed %d of 5 projects over pages of %d:\n%s", len(out), api.pageSize, joined(out))
	}
	if out[0] != "Alpha (project-0001)" || out[4] != "Epsilon (project-0009)" {
		t.Errorf("%%projects lines: %q", out)
	}
	wantUsage(t, metaErr(t, s, "%projects now"), "%projects now")

	runMeta(t, s, "%repo http://127.0.0.1:9/nothing")
	err := metaErr(t, s, "%projects")
	if !strings.Contains(err.Error(), "did not answer") {
		t.Errorf("unreachable server: %v", err)
	}
	if strings.Contains(err.Error(), "goroutine") {
		t.Errorf("a stack trace leaked: %v", err)
	}
}

func TestPublishCreatesAProjectThenCommitsOnIt(t *testing.T) {
	s, api := repoSession(t)
	s.Submit(vehicles)
	out := runMeta(t, s, "%publish Vehicles")
	if len(out) < 2 || out[0] != "created project Vehicles (project-0001)" || !strings.HasPrefix(out[1], "commit commit-") {
		t.Fatalf("first publish:\n%s", joined(out))
	}
	if !strings.Contains(out[1], "deleted") || strings.Contains(out[1], " 0 created") {
		t.Errorf("first publish reports no creations: %q", out[1])
	}
	held := api.elementCount("project-0001", "branch-0002")
	if held == 0 {
		t.Fatal("the project holds no elements")
	}
	if api.elementCount("project-0001", "branch-0002") != held {
		t.Fatal("unstable count")
	}
	for _, id := range api.order {
		if api.projects[id].name == "Spare" {
			t.Error("Spare, outside Vehicles, was published")
		}
	}

	again := runMeta(t, s, "%publish Vehicles")
	if len(again) != 1 || !strings.HasPrefix(again[0], "nothing to publish") {
		t.Errorf("republishing the same model:\n%s", joined(again))
	}

	s.Submit("package Vehicles { part def Wheel { attribute radius : ScalarValues::Real; } part def Car { part wheels : Wheel[4]; } part def Truck; }")
	out = runMeta(t, s, "%publish Vehicles")
	if len(out) != 1 || !strings.Contains(out[0], "on branch main (branch-0002) of Vehicles (project-0001)") {
		t.Fatalf("second publish:\n%s", joined(out))
	}
	if strings.Contains(out[0], " 0 created") {
		t.Errorf("Truck was not created: %q", out[0])
	}
	if len(api.order) != 1 {
		t.Errorf("%d projects after publishing twice, want 1", len(api.order))
	}

	out = runMeta(t, s, "%publish --project=Fleet Vehicles::Car")
	if out[0] != "created project Fleet ("+api.order[len(api.order)-1]+")" {
		t.Errorf("--project:\n%s", joined(out))
	}
	err := metaErr(t, s, "%publish --branch=release Vehicles")
	if !strings.Contains(err.Error(), "branch release doesn't exist") {
		t.Errorf("unknown branch: %v", err)
	}
	wantUsage(t, metaErr(t, s, "%publish"), "%publish")
	wantUsage(t, metaErr(t, s, "%publish --derived Vehicles"), "%publish --derived")
	wantUsage(t, metaErr(t, s, "%publish Vehicles Spare"), "%publish two names")
	if out := runMeta(t, s, "%publish Nowhere"); len(out) != 1 || !strings.HasPrefix(out[0], "error: ") {
		t.Errorf("unknown element:\n%s", joined(out))
	}

	api.conflict = "head moved"
	err = metaErr(t, s, "%publish -d --project=Fleet Vehicles")
	if !strings.Contains(err.Error(), "Conflict") || !strings.Contains(err.Error(), "head moved") {
		t.Errorf("409 is not reported with its status and the server's message: %v", err)
	}
}

func TestPublishWithDerivedPropertiesSendsMore(t *testing.T) {
	s, api := repoSession(t)
	s.Submit(specialized)
	runMeta(t, s, "%publish Chains")
	plain, plainBytes := api.heldBytes("project-0001", "branch-0002")
	s2 := NewSession()
	s2.Submit(specialized)
	runMeta(t, s2, "%publish -d --project=Derived Chains")
	derived := api.order[len(api.order)-1]
	with, withBytes := api.heldBytes(derived, api.projects[derived].defaults)
	if with != plain {
		t.Errorf("derived properties changed the element count: %d vs %d", with, plain)
	}
	if withBytes <= plainBytes {
		t.Errorf("-d sent %d bytes, no more than the %d without", withBytes, plainBytes)
	}
}

// specialized has subclassifications, whose owningClassifier is a derived
// property the reader recomputes: the one a publish leaves out unless asked.
const specialized = `package Chains {
	part def A { part b : B; }
	part def B :> A { attribute x : ScalarValues::Real; }
	part def C :> B;
}
`

func TestLoadReadsAProjectIntoTheSession(t *testing.T) {
	s, api := repoSession(t)
	s.Submit(vehicles)
	runMeta(t, s, "%publish Vehicles")

	fresh := NewSession()
	out := runMeta(t, fresh, "%load Vehicles")
	if !strings.HasPrefix(out[0], "loaded project Vehicles (project-0001) at branch main (branch-0002), commit commit-") {
		t.Fatalf("%%load:\n%s", joined(out))
	}
	listed := joined(runMeta(t, fresh, "%list"))
	for _, want := range []string{"package Vehicles", "part def Car", "part def Wheel", "part wheels : Wheel[4]"} {
		if !strings.Contains(listed, want) {
			t.Errorf("%s is not in the loaded session:\n%s", want, listed)
		}
	}
	if strings.Contains(listed, "Spare") {
		t.Errorf("Spare was loaded, and it was never published:\n%s", listed)
	}
	if fresh.repoState == nil || fresh.repoState.ProjectID != "project-0001" || fresh.repoState.Branch != "branch-0002" {
		t.Fatalf("the loaded branch was not recorded: %+v", fresh.repoState)
	}
	printed := joined(runMeta(t, fresh, "%print Vehicles"))
	if !strings.Contains(printed, "part wheels : Wheel[4]") {
		t.Errorf("the loaded notation lost the multiplicity:\n%s", printed)
	}

	// A publish from the loaded session is a commit on the branch it came from.
	fresh.Submit("package Vehicles { part def Wheel { attribute radius : ScalarValues::Real; } part def Car { part wheels : Wheel[4]; } part def Bus; }")
	out = runMeta(t, fresh, "%publish Vehicles")
	if len(out) != 1 || !strings.Contains(out[0], "of Vehicles (project-0001)") {
		t.Fatalf("publish after load:\n%s", joined(out))
	}
	if len(api.order) != 1 {
		t.Errorf("publishing a loaded project created another: %d projects", len(api.order))
	}

	byID := NewSession()
	if out := runMeta(t, byID, "%load --id=project-0001 --branch=main"); !strings.HasPrefix(out[0], "loaded project Vehicles") {
		t.Errorf("--id --branch:\n%s", joined(out))
	}
	if out := runMeta(t, NewSession(), "%load --name=Vehicles --branch=branch-0002"); !strings.HasPrefix(out[0], "loaded project Vehicles") {
		t.Errorf("--name --branch=<id>:\n%s", joined(out))
	}
	if err := metaErr(t, NewSession(), "%load --id=project-9999"); !strings.Contains(err.Error(), "project project-9999 doesn't exist") {
		t.Errorf("404 by id: %v", err)
	}
	if err := metaErr(t, NewSession(), "%load Nowhere"); !strings.Contains(err.Error(), "project Nowhere doesn't exist") {
		t.Errorf("missing name: %v", err)
	}
	if err := metaErr(t, NewSession(), "%load Vehicles --branch=release"); !strings.Contains(err.Error(), "branch release doesn't exist") {
		t.Errorf("missing branch: %v", err)
	}
	empty := api.addProject("Empty")
	if err := metaErr(t, NewSession(), "%load Empty"); !strings.Contains(err.Error(), "has no head commit") {
		t.Errorf("empty project %s: %v", empty.id, err)
	}
	api.addProject("Vehicles")
	err := metaErr(t, NewSession(), "%load Vehicles")
	if !strings.Contains(err.Error(), "project-0001") || strings.Count(err.Error(), "project-00") < 2 {
		t.Errorf("ambiguous name does not list both ids: %v", err)
	}
	wantUsage(t, metaErr(t, s, "%load --id=project-0001 Vehicles"), "two ways to name")
	wantUsage(t, metaErr(t, s, "%load --branch=main"), "branch alone")
	wantUsage(t, metaErr(t, s, "%load --tag=x"), "unknown flag")
	if _, _, err := s.RunMeta("%load /nonexistent/model.sysml"); err == nil || !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("a path still loads files: %v", err)
	}
	if out, _, _ := s.RunMeta("%load"); joined(out) != usageLoadPath {
		t.Errorf("%%load without arguments: %q", out)
	}
}

func TestPublishLoadRoundTripKeepsTheModel(t *testing.T) {
	s, _ := repoSession(t)
	s.Submit(vehicles)
	runMeta(t, s, "%publish Vehicles")
	fresh := NewSession()
	runMeta(t, fresh, "%load Vehicles")
	want := joined(runMeta(t, s, "%print Vehicles"))
	got := joined(runMeta(t, fresh, "%print Vehicles"))
	if normalizeNotation(got) != normalizeNotation(want) {
		t.Errorf("the model changed in the round trip:\n--- published\n%s\n--- loaded\n%s", want, got)
	}
	if fresh.repoState == nil || fresh.repoState.LastSeenCommit == "" {
		t.Error("the loaded session does not know its commit")
	}
}

// normalizeNotation reads notation as its sorted tokens, the ProjectRef the
// load adds aside: the server orders the elements, and layout is the writer's.
func normalizeNotation(text string) string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@IdentityMetadata::ProjectRef") {
			continue
		}
		line = strings.NewReplacer("{", " { ", "}", " } ", ";", " ; ").Replace(line)
		out = append(out, strings.Fields(line)...)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

func TestRepositoryCommandsAreListedAndCompleted(t *testing.T) {
	s, api := repoSession(t)
	api.addProject("Vehicles")
	api.addProject("Vessels")
	help := joined(runMeta(t, s, "%help"))
	for _, want := range []string{"Repository:", "%repo [<base path>]", "%projects", "%publish [-d] [--project=<project name>] [--branch=<branch name>] <name>", "--id=<project id>"} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q", want)
		}
	}
	complete := func(line string) []string { return s.Complete(line, len(line)).Candidates }
	if got := complete("%pub"); joined(got) != "%publish" {
		t.Errorf("%%pub completes to %q", got)
	}
	if got := complete("%load --"); joined(got) != "--branch=\n--cells=\n--id=\n--name=" {
		t.Errorf("%%load -- completes to %q", got)
	}
	if got := complete("%load --c"); joined(got) != "--cells=" {
		t.Errorf("%%load --c completes to %q", got)
	}
	if got := complete("%load --name=Ve"); joined(got) != "--name=Vehicles\n--name=Vessels" {
		t.Errorf("%%load --name=Ve completes to %q", got)
	}
	if got := complete("%load Veh"); joined(got) != "Vehicles" {
		t.Errorf("%%load Veh completes to %q", got)
	}
	if got := complete("%publish -"); joined(got) != "--branch=\n--project=\n-d" {
		t.Errorf("%%publish - completes to %q", got)
	}
	if got := complete("%publish --project=Ves"); joined(got) != "--project=Vessels" {
		t.Errorf("%%publish --project=Ves completes to %q", got)
	}
	if got := complete("%load --id=project-"); len(got) != 2 {
		t.Errorf("%%load --id= completes to %q", got)
	}
}

// With a repository linked, %load still reads notebooks: --cells is the
// notebook's option, not a project's, and an .ipynb name is a path even when
// no such file exists.
func TestLoadWithCellsReadsANotebookNotAProject(t *testing.T) {
	s, _ := repoSession(t)
	out := runMeta(t, s, "%load --cells 1,3-4 "+fixtureNotebook("tagged.ipynb"))
	if joined(out) == "" || !strings.Contains(joined(out), "3 of 4 code cells (--cells 1,3-4)") {
		t.Errorf("--cells was not read as the notebook's option:\n%s", joined(out))
	}
	out = runMeta(t, s, "%load --cells=tag:model "+fixtureNotebook("tagged.ipynb"))
	if !strings.Contains(joined(out), "2 of 4 code cells (--cells tag:model)") {
		t.Errorf("--cells= was not read as the notebook's option:\n%s", joined(out))
	}
	if err := metaErr(t, s, "%load missing.ipynb"); strings.Contains(err.Error(), "project") {
		t.Errorf("a missing notebook was looked up as a project: %v", err)
	}
}

func TestPublishAfterLoadingABranchCommitsOnThatBranch(t *testing.T) {
	s, api := repoSession(t)
	s.Submit(vehicles)
	runMeta(t, s, "%publish Vehicles")
	review := api.addBranch("project-0001", "review")
	main := api.projects["project-0001"].branches["branch-0002"]
	before := main.head

	fresh := NewSession()
	out := runMeta(t, fresh, "%load Vehicles --branch=review")
	if !strings.HasPrefix(out[0], "loaded project Vehicles (project-0001) at branch review ("+review.id+")") {
		t.Fatalf("%%load:\n%s", joined(out))
	}
	fresh.Submit("package Vehicles { part def Wheel { attribute radius : ScalarValues::Real; } part def Car { part wheels : Wheel[4]; } part def Van; }")
	out = runMeta(t, fresh, "%publish Vehicles")
	if len(out) != 1 || !strings.Contains(out[0], "on branch review ("+review.id+") of Vehicles (project-0001)") {
		t.Fatalf("publish after loading a branch:\n%s", joined(out))
	}
	if main.head != before {
		t.Errorf("the default branch moved from %s to %s; the loaded branch was review", before, main.head)
	}
	if review.head == before || !strings.HasPrefix(out[0], "commit "+review.head+" ") {
		t.Errorf("review's head is %s, the publish reported %q", review.head, out[0])
	}
	if fresh.repoState.Branch != review.id {
		t.Errorf("the session now tracks branch %s, not review", fresh.repoState.Branch)
	}

	// --branch still wins over the loaded branch.
	fresh.Submit("package Vehicles { part def Wheel { attribute radius : ScalarValues::Real; } part def Car { part wheels : Wheel[4]; } part def Van; part def Bus; }")
	out = runMeta(t, fresh, "%publish --branch=main Vehicles")
	if len(out) != 1 || !strings.Contains(out[0], "on branch main (branch-0002)") {
		t.Fatalf("publish --branch after loading another:\n%s", joined(out))
	}
	if main.head == before {
		t.Errorf("--branch=main did not move main")
	}
}

func TestPublishingOneRootLeavesTheOthersOnTheBranch(t *testing.T) {
	s, api := repoSession(t)
	s.Submit(vehicles)
	runMeta(t, s, "%publish Vehicles")
	held := api.elementCount("project-0001", "branch-0002")

	out := runMeta(t, s, "%publish --project=Vehicles Spare")
	if len(out) < 2 || !strings.Contains(out[0], " 0 deleted") || !strings.Contains(out[1], "left in place") {
		t.Fatalf("publishing Spare next to Vehicles:\n%s", joined(out))
	}
	if now := api.elementCount("project-0001", "branch-0002"); now <= held {
		t.Errorf("%d elements after adding Spare to %d", now, held)
	}

	// A session holding the branch's state and a Vehicles without Car: the
	// publication deletes Car, inside its root, and leaves Spare alone.
	reduced := NewSession()
	reduced.Submit("package Vehicles { part def Wheel { attribute radius : ScalarValues::Real; } }")
	reduced.repoState = s.repoState
	out = runMeta(t, reduced, "%publish Vehicles")
	if len(out) != 2 || strings.Contains(out[0], " 0 deleted") || !strings.Contains(out[1], "left in place") {
		t.Fatalf("removing Car from Vehicles:\n%s", joined(out))
	}
	if !strings.Contains(out[0], "Vehicles (project-0001)") {
		t.Errorf("Vehicles went elsewhere: %q", out[0])
	}
	fresh := NewSession()
	runMeta(t, fresh, "%load Vehicles")
	if loaded := fresh.text(); !strings.Contains(loaded, "Boat") || strings.Contains(loaded, "Car") {
		t.Errorf("after removing Car, the branch should hold Boat and no Car:\n%s", loaded)
	}
}

func TestPublishForgetsTheStateOfAnotherServer(t *testing.T) {
	s, api := repoSession(t)
	other := newFakeAPI()
	t.Cleanup(other.Close)
	s.Submit(vehicles)
	runMeta(t, s, "%publish Vehicles")

	runMeta(t, s, "%repo "+other.URL())
	runMeta(t, s, "%publish --project=Vehicles Spare")
	if other.elementCount("project-0001", "branch-0002") == 0 || api.elementCount("project-0001", "branch-0002") == 0 {
		t.Fatal("both servers should now hold a project-0001")
	}
	runMeta(t, s, "%repo "+api.URL())
	runMeta(t, s, "%publish Vehicles")
	runMeta(t, s, "%repo "+other.URL())

	// The same ids on another server are no history of it: nothing it holds
	// may be deleted on the strength of the first server's commits.
	held := other.elementCount("project-0001", "branch-0002")
	out := runMeta(t, s, "%publish Vehicles")
	if len(out) < 1 || !strings.Contains(out[0], " 0 deleted") {
		t.Fatalf("publishing to the other server:\n%s", joined(out))
	}
	if now := other.elementCount("project-0001", "branch-0002"); now < held {
		t.Errorf("the other server lost elements: %d, had %d", now, held)
	}
}

// A %load of --cells alone is the file load's to answer: it wants a notebook,
// or a selection, and no repository option was given; a lone name beginning
// with one dash is still a project's.
func TestLoadWithCellsAloneWantsANotebook(t *testing.T) {
	s, api := repoSession(t)
	api.addProject("-demo")
	if err := metaErr(t, s, "%load -demo"); err == nil || !strings.Contains(err.Error(), "has no head commit") {
		t.Errorf("a lone -demo did not name the project: %v", err)
	}
	if out := joined(runMeta(t, s, "%load --cells=1")); out != usageLoadPath {
		t.Errorf("%%load --cells=1 answered %q, want the file load's usage", out)
	}
	err := metaErr(t, s, "%load --cells")
	if err == nil || !strings.Contains(err.Error(), "--cells needs a selection") {
		t.Errorf("%%load --cells: %v", err)
	}
	err = metaErr(t, s, "%load --tags=x")
	var usage *UsageError
	if !errors.As(err, &usage) || !strings.Contains(joined(usage.Lines), "unknown option --tags") {
		t.Errorf("%%load --tags=x: %v", err)
	}
}

func TestRepositoryOptionsAreGivenOnce(t *testing.T) {
	s, _ := repoSession(t)
	s.Submit(vehicles)
	for _, line := range []string{
		"%load --id=p1 --id=p2",
		"%load --name=Vehicles --name=Spare",
		"%load --branch=main --branch=dev Vehicles",
		"%publish --project=Fleet --project=Other Vehicles",
		"%publish --branch=main --branch=dev Vehicles",
		"%publish -d -d Vehicles",
	} {
		err := metaErr(t, s, line)
		wantUsage(t, err, line)
		var usage *UsageError
		if errors.As(err, &usage) && !strings.Contains(joined(usage.Lines), "was given twice") {
			t.Errorf("%s: %q does not name the repeated option", line, usage.Lines)
		}
	}
}

// The environment's URL is held to the rule %repo holds a URL it is given to:
// a plaintext server off this machine gets no request, so no token in the clear.
func TestDefaultURLIsHeldToThePlaintextRule(t *testing.T) {
	if replext.Repo() == nil {
		t.Fatal("no repository extension is linked")
	}
	t.Setenv("FLEXO_SYSMLV2_URL", "http://models.example.com/api")
	t.Setenv("FLEXO_INTEROP_TOKEN", "secret")
	t.Setenv("FLEXO_ALLOW_PLAIN_HTTP", "")
	s := NewSession()
	s.Submit(vehicles)
	for _, line := range []string{"%repo", "%projects", "%load --id=project-0001", "%publish Vehicles"} {
		err := metaErr(t, s, line)
		if !strings.Contains(err.Error(), "FLEXO_ALLOW_PLAIN_HTTP") || !strings.Contains(err.Error(), "http://models.example.com/api") {
			t.Errorf("%s: the environment's plaintext URL was taken: %v", line, err)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: the refusal names the token: %v", line, err)
		}
		if strings.Contains(err.Error(), "did not answer") {
			t.Errorf("%s: a request went out: %v", line, err)
		}
	}
	if got := s.Complete("%load Veh", len("%load Veh")).Candidates; len(got) != 0 {
		t.Errorf("completion asked the refused server: %q", got)
	}

	if got := joined(runMeta(t, s, "%repo http://127.0.0.1:1/api")); got != "API base path: http://127.0.0.1:1/api" {
		t.Errorf("%%repo <loopback url> = %q", got)
	}
	if err := metaErr(t, s, "%projects"); !strings.Contains(err.Error(), "did not answer") {
		t.Errorf("a loopback URL set by %%repo was still refused: %v", err)
	}

	t.Setenv("FLEXO_ALLOW_PLAIN_HTTP", "1")
	if got := joined(runMeta(t, NewSession(), "%repo")); got != "API base path: http://models.example.com/api" {
		t.Errorf("with the opt-in, %%repo = %q", got)
	}
}

// A project the session loaded by id is the one a publish by its name means,
// even when another project on the server has the same name.
func TestPublishByNameUpdatesTheLoadedNamesake(t *testing.T) {
	s, api := repoSession(t)
	s.Submit(vehicles)
	runMeta(t, s, "%publish Vehicles")
	namesake := api.addProject("Vehicles")
	held := api.elementCount("project-0001", "branch-0002")

	untracked := NewSession()
	untracked.Submit(vehicles)
	if err := metaErr(t, untracked, "%publish Vehicles"); !strings.Contains(err.Error(), "project-0001") || !strings.Contains(err.Error(), namesake.id) {
		t.Errorf("without a loaded project, the shared name is not refused naming both: %v", err)
	}

	fresh := NewSession()
	runMeta(t, fresh, "%load --id=project-0001")
	fresh.Submit("package Vehicles { part def Wheel { attribute radius : ScalarValues::Real; } part def Car { part wheels : Wheel[4]; } part def Bus; }")
	out := runMeta(t, fresh, "%publish Vehicles")
	if len(out) != 1 || !strings.Contains(out[0], "of Vehicles (project-0001)") {
		t.Fatalf("publish by name after loading by id:\n%s", joined(out))
	}
	if now := api.elementCount("project-0001", "branch-0002"); now <= held {
		t.Errorf("the loaded project holds %d elements, had %d; Bus was not added", now, held)
	}
	if api.elementCount(namesake.id, namesake.defaults) != 0 || len(api.order) != 2 {
		t.Errorf("the namesake received elements or a project was created: %d projects", len(api.order))
	}
	fresh.Submit("package Spare { part def Boat; }")
	out = runMeta(t, fresh, "%publish --project=Vehicles Spare")
	if len(out) < 1 || !strings.Contains(out[0], "of Vehicles (project-0001)") {
		t.Errorf("--project naming the tracked project's name:\n%s", joined(out))
	}
	if len(api.order) != 2 {
		t.Errorf("--project=Vehicles created a project: %d projects", len(api.order))
	}
}
