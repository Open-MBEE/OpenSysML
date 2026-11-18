package hygiene

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The client areas every CI tier can fan out to from a service change. Each of
// them fetches a toolchain or dependencies from a registry on every run.
var clientAreas = []string{"node", "python", "java", "rust", "julia", "matlab", "vscode", "jupyterlab", "emacs", "mdk", "syson"}

// changedAreas runs scripts/ci-changed-areas.sh in a throwaway repository whose head
// adds the given files over an empty base, and returns the area=value lines it prints.
func changedAreas(t *testing.T, files []string, env ...string) map[string]string {
	t.Helper()
	script, err := filepath.Abs("../../scripts/ci-changed-areas.sh")
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=hygiene", "GIT_AUTHOR_EMAIL=hygiene@example.invalid",
			"GIT_COMMITTER_NAME=hygiene", "GIT_COMMITTER_EMAIL=hygiene@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q", "-b", "develop")
	git("commit", "-q", "--allow-empty", "-m", "base")
	git("tag", "base")
	for _, f := range files {
		path := filepath.Join(repo, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", "-A")
	git("commit", "-q", "-m", "change")

	cmd := exec.Command(script, "base", "HEAD")
	cmd.Dir = repo
	cmd.Env = append(append(os.Environ(), "CI_TIER=", "CI_EVENT=", "CI_FULL_LABEL="), env...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("ci-changed-areas.sh: %v\n%s", err, ee.Stderr)
		}
		t.Fatal(err)
	}
	areas := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("ci-changed-areas.sh printed %q, not area=value", line)
		}
		areas[k] = v
	}
	return areas
}

// A merge group runs a client only when that client's own paths changed: the
// fan-out from a service change to every client is the full tier's, which the
// nightly full scan gives develop, so a registry timeout in a client job cannot
// fail a group whose change never touched that client.
func TestQueueTierRunsClientsOnTheirOwnPathsOnly(t *testing.T) {
	service := []string{"internal/semantic/resolve.go"}

	queue := changedAreas(t, service, "CI_EVENT=merge_group")
	if queue["tier"] != "queue" {
		t.Fatalf("merge_group tier = %q, want queue", queue["tier"])
	}
	if queue["go"] != "true" || queue["docs"] != "true" {
		t.Errorf("queue tier on a service change: go=%s docs=%s, want both true", queue["go"], queue["docs"])
	}
	for _, area := range clientAreas {
		if queue[area] != "false" {
			t.Errorf("queue tier on a service change runs %s=%s, want false", area, queue[area])
		}
	}

	full := changedAreas(t, service, "CI_EVENT=schedule")
	if full["tier"] != "full" {
		t.Fatalf("schedule tier = %q, want full", full["tier"])
	}
	for _, area := range clientAreas {
		if full[area] != "true" {
			t.Errorf("full tier on a service change runs %s=%s, want true", area, full[area])
		}
	}

	own := changedAreas(t, []string{"editors/emacs/sysml-mode.el", "client/java/pom.xml"}, "CI_EVENT=merge_group")
	for area, want := range map[string]string{"emacs": "true", "java": "true", "mdk": "true", "syson": "true", "rust": "false", "node": "false", "julia": "false"} {
		if own[area] != want {
			t.Errorf("queue tier on an emacs+java change runs %s=%s, want %s", area, own[area], want)
		}
	}
}
