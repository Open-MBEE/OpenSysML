package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/flexo"
)

// branchCommand runs the binary against the fake stack like syncCommand, with
// the SysML v2 endpoint set too so the flexo:// shorthand resolves to it.
func branchCommand(stack *fakeStack, binary string, args ...string) *exec.Cmd {
	cmd := syncCommand(stack, binary, args...)
	cmd.Env = append(cmd.Env, flexo.EnvSysMLV2URL+"="+stack.server.URL)
	return cmd
}

func TestConvertReadsABranchAsNotationAndTurtle(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))

	out, code := exitCode(t, branchCommand(stack, binary, "flexo://proj-1/main", "-convert", "sysml"))
	if code != 0 || !strings.Contains(out, "part def Vehicle") {
		t.Fatalf("read a branch as notation: exit %d:\n%s", code, out)
	}

	branchURL := stack.server.URL + "/projects/proj-1/branches/main"
	out, code = exitCode(t, branchCommand(stack, binary, branchURL, "-convert", "ttl"))
	if code != 0 || !strings.Contains(out, "sysml:PartDefinition") && !strings.Contains(out, "8f3a41d0") {
		t.Fatalf("read a branch as Turtle: exit %d:\n%s", code, out)
	}
}

func TestConvertReadsABranchIntoAFileAndRecordsTheHead(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	out_path := filepath.Join(t.TempDir(), "out.sysml")

	out, code := exitCode(t, branchCommand(stack, binary, stack.server.URL+"/projects/proj-1/branches/main", "-convert", "sysml", "-o", out_path))
	if code != 0 {
		t.Fatalf("read a branch to a file: exit %d:\n%s", code, out)
	}
	written, err := os.ReadFile(out_path)
	if err != nil || !strings.Contains(string(written), "part def Vehicle") {
		t.Fatalf("the output file: %v\n%s", err, written)
	}
	state, err := os.ReadFile(out_path + ".sync.json")
	if err != nil || !strings.Contains(string(state), `"lastSeenCommit": "commit-0"`) || !strings.Contains(string(state), `"projectId": "proj-1"`) {
		t.Fatalf("sync state after a branch read: %v\n%s", err, state)
	}
	if !strings.Contains(out, "head commit commit-0 recorded in") {
		t.Errorf("the recorded head is not reported:\n%s", out)
	}
}

func TestConvertPushReplacesTheBranchGraph(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	model := writeModel(t, dir, "model.sysml", renamedModel)
	branchURL := stack.server.URL + "/projects/proj-1/branches/main"

	out, code := exitCode(t, branchCommand(stack, binary, model, "-convert", "ttl", "-o", branchURL))
	if code != 0 {
		t.Fatalf("push: exit %d:\n%s", code, out)
	}
	if len(stack.puts) != 1 {
		t.Fatalf("the push sent %d write(s), want 1", len(stack.puts))
	}
	put := stack.puts[0]
	if !strings.HasPrefix(put.url, "/orgs/sysmlv2/repos/proj-1/branches/main/graph") || !strings.Contains(put.url, "message=") {
		t.Errorf("the push hit %s", put.url)
	}
	if put.ifMatch != `"commit-0"` {
		t.Errorf("the push was conditioned on %q, not the branch etag", put.ifMatch)
	}
	if put.contentType != "text/turtle" {
		t.Errorf("the push went as %q, not Turtle", put.contentType)
	}
	if stack.head != "commit-1" {
		t.Errorf("the push's commit is %q, want commit-1", stack.head)
	}
	state, err := os.ReadFile(model + ".sync.json")
	if err != nil || !strings.Contains(string(state), `"lastSeenCommit": "commit-1"`) || !strings.Contains(string(state), `"projectId": "proj-1"`) {
		t.Fatalf("sync state after the push: %v\n%s", err, state)
	}
	if !strings.Contains(out, "pushed") || !strings.Contains(out, "head commit commit-1 recorded in") {
		t.Errorf("the pushed commit is not reported:\n%s", out)
	}
	if !strings.Contains(string(stack.puts[0].body), "declaredName") || !strings.Contains(string(stack.puts[0].body), "Car") {
		t.Errorf("the pushed graph does not hold the model:\n%s", stack.puts[0].body)
	}
}

func TestConvertPushRefusesAHeadTheStateSaysMoved(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	model := writeModel(t, dir, "model.sysml", renamedModel)
	writeModel(t, dir, "model.sysml.sync.json", `{"org":"sysmlv2","projectId":"proj-1","branch":"main","lastSeenCommit":"commit-old"}`)
	branchURL := stack.server.URL + "/projects/proj-1/branches/main"

	out, code := exitCode(t, branchCommand(stack, binary, model, "-convert", "ttl", "-o", branchURL))
	if code != 1 || len(stack.puts) != 0 {
		t.Fatalf("push against a moved head: exit %d, %d write(s):\n%s", code, len(stack.puts), out)
	}
	if !strings.Contains(out, "refused to push") || !strings.Contains(out, "commit-old") {
		t.Errorf("the stale refusal is not explained:\n%s", out)
	}
}

func TestConvertPushNeedsTurtleAndAUsefulState(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	model := writeModel(t, dir, "model.sysml", renamedModel)
	branchURL := stack.server.URL + "/projects/proj-1/branches/main"

	out, code := exitCode(t, branchCommand(stack, binary, model, "-convert", "sysml", "-o", branchURL))
	if code != 2 || len(stack.puts) != 0 || !strings.Contains(out, "ttl") {
		t.Fatalf("-convert sysml to a branch: exit %d:\n%s", code, out)
	}

	out, code = exitCode(t, branchCommand(stack, binary, "flexo://proj-1/main", "-convert", "ttl", "-o", branchURL))
	if code != 2 || len(stack.puts) != 0 {
		t.Fatalf("branch input and branch output: exit %d:\n%s", code, out)
	}

	out, code = exitCode(t, branchCommand(stack, binary, branchURL, "-convert", "sysml", "-from", "sysml"))
	if code != 2 || !strings.Contains(out, "RDF graph") {
		t.Fatalf("-from sysml on a branch read: exit %d:\n%s", code, out)
	}
}

func TestConvertChecksTheStateAgainstTheConfiguredOrg(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	model := writeModel(t, dir, "model.sysml", renamedModel)

	// A state pinned to the configured org is accepted and the push lands.
	writeModel(t, dir, "model.sysml.sync.json", `{"org":"acme","projectId":"proj-1","branch":"main","lastSeenCommit":"commit-0"}`)
	cmd := branchCommand(stack, binary, model, "-convert", "ttl", "-o", "flexo://proj-1/main")
	cmd.Env = append(cmd.Env, flexo.EnvOrg+"=acme")
	if out, code := exitCode(t, cmd); code != 0 || len(stack.puts) != 1 {
		t.Fatalf("push under the state's org: exit %d, %d write(s):\n%s", code, len(stack.puts), out)
	}

	// A state pinned to another org is refused before any write.
	stack.head = "commit-0"
	writeModel(t, dir, "model.sysml.sync.json", `{"org":"other","projectId":"proj-1","branch":"main","lastSeenCommit":"commit-0"}`)
	cmd = branchCommand(stack, binary, model, "-convert", "ttl", "-o", "flexo://proj-1/main")
	cmd.Env = append(cmd.Env, flexo.EnvOrg+"=acme")
	out, code := exitCode(t, cmd)
	if code != 2 || len(stack.puts) != 1 || !strings.Contains(out, "org other") {
		t.Fatalf("push under another org: exit %d:\n%s", code, out)
	}
}

func TestConvertPushReportsAWriteNoCommitWasNamedFor(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	stack.noPutETag = true
	dir := t.TempDir()
	model := writeModel(t, dir, "model.sysml", renamedModel)

	out, code := exitCode(t, branchCommand(stack, binary, model, "-convert", "ttl", "-o", "flexo://proj-1/main"))
	if code != 1 || len(stack.puts) != 1 || !strings.Contains(out, "the response named no commit") {
		t.Fatalf("a push naming no commit: exit %d, %d write(s):\n%s", code, len(stack.puts), out)
	}
	if _, err := os.Stat(model + ".sync.json"); !os.IsNotExist(err) {
		t.Errorf("an unrecorded push still wrote %s", model+".sync.json")
	}
}

func TestConvertAcceptsTheConfiguredEndpointSpelledDifferently(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	// The configured endpoint carries a trailing slash; the branch URL does
	// not. They name the same endpoint, so the read is allowed.
	cmd := syncCommand(stack, binary, stack.server.URL+"/projects/proj-1/branches/main", "-convert", "sysml")
	cmd.Env = append(cmd.Env, flexo.EnvSysMLV2URL+"="+stack.server.URL+"/")
	out, code := exitCode(t, cmd)
	if code != 0 || !strings.Contains(out, "part def Vehicle") {
		t.Fatalf("the endpoint spelled with a trailing slash: exit %d:\n%s", code, out)
	}
}

func TestConvertRefusesAnEndpointOtherThanTheConfigured(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	// syncCommand leaves FLEXO_SYSMLV2_URL at its default, so the URL below
	// names a different endpoint than the one Layer 1 is pointed at.
	out, code := exitCode(t, syncCommand(stack, binary, stack.server.URL+"/projects/proj-1/branches/main", "-convert", "sysml"))
	if code != 2 || !strings.Contains(out, "endpoint other than the configured") || !strings.Contains(out, "flexo://proj-1/main") {
		t.Fatalf("a URL for another endpoint: exit %d:\n%s", code, out)
	}
}

func TestConvertRefusesSyncStateWithoutABranch(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	model := writeModel(t, t.TempDir(), "model.sysml", renamedModel)

	out, code := exitCode(t, syncCommand(stack, binary, model, "-convert", "ttl", "-sync-state", filepath.Join(t.TempDir(), "s.json")))
	if code != 2 || !strings.Contains(out, "-sync-state records a repository branch's head") {
		t.Fatalf("-sync-state on a plain conversion: exit %d:\n%s", code, out)
	}
}

func TestConvertPushRefusesAMigrationReportOverTheInput(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	source, err := os.ReadFile(filepath.Join("..", "..", "tests", "migrate", "testdata", "xmi", "vehicle.xmi"))
	if err != nil {
		t.Fatal(err)
	}
	xmi := writeModel(t, dir, "v1.xmi", string(source))

	out, code := exitCode(t, branchCommand(stack, binary, xmi, "-convert", "ttl", "-o", "flexo://proj-1/main", "-migration-report", xmi))
	if code != 2 || !strings.Contains(out, "names the model being migrated") || len(stack.puts) != 0 {
		t.Fatalf("-migration-report over the pushed model: exit %d, %d write(s):\n%s", code, len(stack.puts), out)
	}
	if after, _ := os.ReadFile(xmi); string(after) != string(source) {
		t.Errorf("the refused push still replaced the input model")
	}
}

func TestConvertReadRefusesOutputOverTheSyncState(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	out_path := writeModel(t, dir, "saved.sysml", "sentinel")

	out, code := exitCode(t, branchCommand(stack, binary, "flexo://proj-1/main", "-convert", "sysml", "-o", out_path, "-sync-state", out_path))
	if code != 2 || !strings.Contains(out, "-o and -sync-state both name") {
		t.Fatalf("-o over the sync state: exit %d:\n%s", code, out)
	}
	if content, _ := os.ReadFile(out_path); string(content) != "sentinel" {
		t.Errorf("the refused read still replaced the file")
	}
}

func TestConvertReadRefusesAStatePinnedElsewhereBeforeWriting(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	state := writeModel(t, dir, "s.sync.json", `{"org":"other","projectId":"proj-1","branch":"main"}`)
	out_path := filepath.Join(dir, "out.sysml")

	cmd := branchCommand(stack, binary, "flexo://proj-1/main", "-convert", "sysml", "-o", out_path, "-sync-state", state)
	cmd.Env = append(cmd.Env, flexo.EnvOrg+"=acme")
	out, code := exitCode(t, cmd)
	if code != 2 || !strings.Contains(out, "org other") {
		t.Fatalf("a state pinned to another org: exit %d:\n%s", code, out)
	}
	if _, err := os.Stat(out_path); !os.IsNotExist(err) {
		t.Errorf("the refused read still wrote %s", out_path)
	}
}

func TestConvertBranchNeedsTheToken(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	cmd := exec.Command(binary, "flexo://proj-1/main", "-convert", "sysml")
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, flexo.EnvToken+"=") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, flexo.EnvLayer1URL+"="+stack.server.URL, flexo.EnvSysMLV2URL+"="+stack.server.URL)
	out, code := exitCode(t, cmd)
	if code != 2 || !strings.Contains(out, flexo.EnvToken) {
		t.Fatalf("a branch read without a token: exit %d:\n%s", code, out)
	}
}
