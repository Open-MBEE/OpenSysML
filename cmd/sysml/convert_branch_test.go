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
	out, code = exitCode(t, syncCommand(stack, binary, branchURL, "-convert", "ttl"))
	if code != 0 || !strings.Contains(out, "sysml:PartDefinition") && !strings.Contains(out, "8f3a41d0") {
		t.Fatalf("read a branch as Turtle: exit %d:\n%s", code, out)
	}
}

func TestConvertReadsABranchIntoAFileAndRecordsTheHead(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	out_path := filepath.Join(t.TempDir(), "out.sysml")

	out, code := exitCode(t, syncCommand(stack, binary, stack.server.URL+"/projects/proj-1/branches/main", "-convert", "sysml", "-o", out_path))
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

	out, code := exitCode(t, syncCommand(stack, binary, model, "-convert", "ttl", "-o", branchURL))
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
	writeModel(t, dir, "model.sysml.sync.json", `{"projectId":"proj-1","branch":"main","lastSeenCommit":"commit-old"}`)
	branchURL := stack.server.URL + "/projects/proj-1/branches/main"

	out, code := exitCode(t, syncCommand(stack, binary, model, "-convert", "ttl", "-o", branchURL))
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

	out, code := exitCode(t, syncCommand(stack, binary, model, "-convert", "sysml", "-o", branchURL))
	if code != 2 || len(stack.puts) != 0 || !strings.Contains(out, "ttl") {
		t.Fatalf("-convert sysml to a branch: exit %d:\n%s", code, out)
	}

	out, code = exitCode(t, syncCommand(stack, binary, "flexo://proj-1/main", "-convert", "ttl", "-o", branchURL))
	if code != 2 || len(stack.puts) != 0 {
		t.Fatalf("branch input and branch output: exit %d:\n%s", code, out)
	}

	out, code = exitCode(t, syncCommand(stack, binary, branchURL, "-convert", "sysml", "-from", "sysml"))
	if code != 2 || !strings.Contains(out, "RDF graph") {
		t.Fatalf("-from sysml on a branch read: exit %d:\n%s", code, out)
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
