package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const syncedModel = `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
}
`

const renamedModel = `// The fleet's model. Lexical comments must survive annotation write-back.
package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def Car {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
	part def Wheel; /* rolls */
}
`

// syncFixtures writes the renamed model and the repository graph of the
// original, so the diff has a rename and a create to report.
func syncFixtures(t *testing.T, binary string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	original := filepath.Join(dir, "original.sysml")
	model := filepath.Join(dir, "model.sysml")
	repo := filepath.Join(dir, "repo.ttl")
	if err := os.WriteFile(original, []byte(syncedModel), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(model, []byte(renamedModel), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, binary, original, "-convert", "ttl", "-o", repo)
	return model, repo
}

func TestSyncDiffDryRunThroughCLI(t *testing.T) {
	binary := buildCLI(t)
	model, repo := syncFixtures(t, binary)

	out := run(t, binary, model, "-sync-diff", repo)
	for _, want := range []string{
		"sync diff against project proj-1 branch main",
		"update   8f3a41d0",
		"create   P__Wheel",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run report is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "delete") && !strings.Contains(out, "0 delete(s)") {
		t.Errorf("a rename leaked a delete into the report:\n%s", out)
	}
}

// The sync reads the API's element form too: a repository or model committed
// as api-json diffs exactly as the Turtle of the same graph does.
func TestSyncDiffReadsAPIJSONRepositoryThroughCLI(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	original := filepath.Join(dir, "original.sysml")
	model := filepath.Join(dir, "model.sysml")
	for path, content := range map[string]string{original: syncedModel, model: renamedModel} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repoJSON := filepath.Join(dir, "repo.json")
	repoTurtle := filepath.Join(dir, "repo.ttl")
	modelJSON := filepath.Join(dir, "model.json")
	baseJSON := filepath.Join(dir, "base.json")
	run(t, binary, original, "-convert", "api-json", "-o", repoJSON)
	run(t, binary, original, "-convert", "ttl", "-o", repoTurtle)
	run(t, binary, model, "-convert", "api-json", "-o", modelJSON)
	run(t, binary, original, "-convert", "api-json", "-o", baseJSON)

	out := run(t, binary, model, "-sync-diff", repoJSON)
	for _, want := range []string{
		"sync diff against project proj-1 branch main",
		"update   8f3a41d0",
		"create   P__Wheel",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("api-json repository report is missing %q:\n%s", want, out)
		}
	}

	out = run(t, binary, modelJSON, "-sync-diff", repoTurtle)
	for _, want := range []string{"update   8f3a41d0", "create   P__Wheel"} {
		if !strings.Contains(out, want) {
			t.Errorf("api-json model report is missing %q:\n%s", want, out)
		}
	}

	out = run(t, binary, model, "-sync-diff", repoTurtle, "-sync-base", baseJSON)
	if !strings.Contains(out, "create   P__Wheel") {
		t.Errorf("an api-json -sync-base is not read:\n%s", out)
	}
}

func TestSyncDiffReportsUnconfirmedDeletes(t *testing.T) {
	binary := buildCLI(t)
	model, repo := syncFixtures(t, binary)

	// Diffing the repository against itself minus Vehicle: sync the original
	// as the local model against a repo graph that also holds Wheel.
	local := filepath.Join(t.TempDir(), "less.sysml")
	if err := os.WriteFile(local, []byte(syncedModel), 0o644); err != nil {
		t.Fatal(err)
	}
	wheelRepo := filepath.Join(t.TempDir(), "repo.ttl")
	run(t, binary, model, "-convert", "ttl", "-o", wheelRepo)

	out := run(t, binary, local, "-sync-diff", wheelRepo, "-sync-confirm-deletes")
	if !strings.Contains(out, "delete   P__Wheel") {
		t.Errorf("the repository-only element is not reported as a delete:\n%s", out)
	}
	if strings.Contains(out, "needs explicit confirmation") {
		t.Errorf("a confirmed delete still reads as unconfirmed:\n%s", out)
	}
	_ = repo
}

func TestSyncDiffConflictExitsNonzero(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	repo := filepath.Join(dir, "repo.ttl")
	if err := os.WriteFile(model, []byte(syncedModel), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other.sysml")
	if err := os.WriteFile(other, []byte(`package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def Other;
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, binary, other, "-convert", "ttl", "-o", repo)

	cmd := exec.Command(binary, model, "-sync-diff", repo)
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		t.Fatalf("a conflicted diff must exit 1, got %v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "conflict 8f3a41d0") {
		t.Errorf("the missing-id conflict is not reported:\n%s", out)
	}
}

func TestSyncAnnotateWritesMintedIDs(t *testing.T) {
	binary := buildCLI(t)
	model, repo := syncFixtures(t, binary)
	annotated := filepath.Join(t.TempDir(), "annotated.sysml")

	out := run(t, binary, model, "-sync-diff", repo, "-sync-mint-ids", "-sync-annotate", annotated)
	if !strings.Contains(out, "minted id ") {
		t.Errorf("the minted id is not reported:\n%s", out)
	}
	data, err := os.ReadFile(annotated)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "IdentityMetadata::ElementId about P::Wheel") {
		t.Errorf("the annotated notation does not declare the minted id:\n%s", data)
	}
	if !strings.HasPrefix(string(data), "// The fleet's model.") ||
		!strings.Contains(string(data), "/* rolls */") {
		t.Errorf("the annotated notation lost the model's lexical comments:\n%s", data)
	}
}

func TestSyncAnnotateQuotesUnrestrictedNames(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	repo := filepath.Join(dir, "repo.ttl")
	if err := os.WriteFile(model, []byte(`package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def 'Spare Wheel'; // unrestricted name
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repo, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	annotated := filepath.Join(dir, "annotated.sysml")

	run(t, binary, model, "-sync-diff", repo, "-sync-mint-ids", "-sync-annotate", annotated)
	data, err := os.ReadFile(annotated)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "IdentityMetadata::ElementId about P::'Spare Wheel'") {
		t.Errorf("the unrestricted name is not quoted in the annotation:\n%s", data)
	}
	if !strings.Contains(string(data), "// unrestricted name") {
		t.Errorf("the annotated notation lost the model's lexical comment:\n%s", data)
	}
}

func TestSyncAnnotateSkipsUnnamedElements(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	repo := filepath.Join(dir, "repo.ttl")
	if err := os.WriteFile(model, []byte(`package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def W;
	part : W;
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repo, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	annotated := filepath.Join(dir, "annotated.sysml")

	out := run(t, binary, model, "-sync-diff", repo, "-sync-mint-ids", "-sync-annotate", annotated)
	if !strings.Contains(out, "has no name to write an annotation against") {
		t.Errorf("the unnamed element's skipped annotation is not reported:\n%s", out)
	}
	data, err := os.ReadFile(annotated)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "'@") {
		t.Errorf("a positional address leaked into an about clause:\n%s", data)
	}
	if !strings.Contains(string(data), "IdentityMetadata::ElementId about P::W") {
		t.Errorf("the named element's minted id is not annotated:\n%s", data)
	}
}

func TestSyncAnnotateAddressesByShortName(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	repo := filepath.Join(dir, "repo.ttl")
	if err := os.WriteFile(model, []byte(`package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def W;
	part <x> : W {
		part def Rim;
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repo, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	annotated := filepath.Join(dir, "annotated.sysml")

	out := run(t, binary, model, "-sync-diff", repo, "-sync-mint-ids", "-sync-annotate", annotated)
	if strings.Contains(out, "has no name to write an annotation against") {
		t.Errorf("a short-named element was skipped:\n%s", out)
	}
	data, err := os.ReadFile(annotated)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "IdentityMetadata::ElementId about P::x ") {
		t.Errorf("the short-named element's minted id is not annotated:\n%s", data)
	}
	if !strings.Contains(string(data), "IdentityMetadata::ElementId about P::x::Rim") {
		t.Errorf("the named descendant under the short-named owner is not annotated:\n%s", data)
	}
}

// An import has no declaration an annotation can address, so a minted import
// must surface in the unnamed report rather than being dropped silently.
func TestSyncAnnotateReportsMintedImport(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	repo := filepath.Join(dir, "repo.ttl")
	if err := os.WriteFile(model, []byte(`package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	import A::*;
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repo, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	annotated := filepath.Join(dir, "annotated.sysml")

	out := run(t, binary, model, "-sync-diff", repo, "-sync-mint-ids", "-sync-annotate", annotated)
	if !strings.Contains(out, "has no name to write an annotation against") || !strings.Contains(out, "Import") {
		t.Errorf("the minted import's skipped annotation is not reported:\n%s", out)
	}
}

func TestSyncAnnotateNeedsMinting(t *testing.T) {
	binary := buildCLI(t)
	model, repo := syncFixtures(t, binary)

	cmd := exec.Command(binary, model, "-sync-diff", repo, "-sync-annotate", "out.sysml")
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 {
		t.Fatalf("-sync-annotate without -sync-mint-ids must be refused, got %v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "-sync-mint-ids") {
		t.Errorf("the refusal does not name the missing opt-in:\n%s", out)
	}
}

func TestSyncStatePinsTheBranch(t *testing.T) {
	binary := buildCLI(t)
	model, repo := syncFixtures(t, binary)
	state := filepath.Join(t.TempDir(), "state.sync.json")
	if err := os.WriteFile(state, []byte(`{"projectId":"proj-1","branch":"dev","lastSeenCommit":"c0ffee"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, model, "-sync-diff", repo, "-sync-state", state)
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 {
		t.Fatalf("a state pinning another branch must be refused, got %v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "two branches") {
		t.Errorf("the refusal does not explain the branch rule:\n%s", out)
	}
}

func TestSyncDiffRefusesOtherModes(t *testing.T) {
	binary := buildCLI(t)
	model, repo := syncFixtures(t, binary)
	for _, extra := range [][]string{
		{"-render-all", t.TempDir()},
		{"-render-documents", t.TempDir()},
		{"-output", "out.txt"},
		{"-from", "sysml"},
	} {
		args := append([]string{model, "-sync-diff", repo}, extra...)
		cmd := exec.Command(binary, args...)
		out, err := cmd.CombinedOutput()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 2 {
			t.Errorf("-sync-diff with %s must be refused, got %v:\n%s", extra[0], err, out)
			continue
		}
		if !strings.Contains(string(out), "-sync-diff") {
			t.Errorf("the refusal of %s does not name -sync-diff:\n%s", extra[0], out)
		}
	}
}

func TestSyncDiffRefusesAnEmptyValue(t *testing.T) {
	binary := buildCLI(t)
	cmd := exec.Command(binary, "-sync-diff", "")
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 {
		t.Fatalf("an empty -sync-diff must be refused, got %v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "-sync-diff is empty") {
		t.Errorf("the refusal does not explain the empty value:\n%s", out)
	}
}

func TestSyncFlagsNeedSyncDiff(t *testing.T) {
	binary := buildCLI(t)
	cmd := exec.Command(binary, "-sync-mint-ids")
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 {
		t.Fatalf("a sync flag without -sync-diff must be refused, got %v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "-sync-diff") {
		t.Errorf("the refusal does not point at -sync-diff:\n%s", out)
	}
}
