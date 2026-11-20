package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	recordsBase = "# rule\troot\tours\tpilot\nA::x\ttraining\t1\t9\nB::y\ttraining\t2\t9\n"
	tracked     = "records.tsv"
)

// git runs the driver the way .gitattributes and the installer configure it: two branches that move
// counts on neighbouring lines, which git's own merge conflicts on, merge.
func TestGitMergesNeighbouringCountsThroughTheDriver(t *testing.T) {
	repo := branches(t,
		strings.Replace(recordsBase, "A::x\ttraining\t1", "A::x\ttraining\t5", 1),
		strings.Replace(recordsBase, "B::y\ttraining\t2", "B::y\ttraining\t7", 1))
	if out, err := git(repo, "merge", "--no-edit", "theirs"); err == nil {
		t.Fatalf("git's line merge no longer conflicts here, so the test proves nothing:\n%s", out)
	}
	mustGit(t, repo, "merge", "--abort")
	configure(t, repo)
	mustGit(t, repo, "merge", "--no-edit", "theirs")
	merged, err := os.ReadFile(filepath.Join(repo, tracked)) // #nosec G304 -- the test's own repository
	if err != nil {
		t.Fatal(err)
	}
	if want := "# rule\troot\tours\tpilot\nA::x\ttraining\t5\t9\nB::y\ttraining\t7\t9\n"; string(merged) != want {
		t.Fatalf("merged %q, want %q", merged, want)
	}
}

// A movement the cells cannot merge leaves the merge in conflict, for the recorder to settle.
func TestGitReportsAConflictTheDriverRefuses(t *testing.T) {
	repo := branches(t,
		strings.Replace(recordsBase, "B::y\ttraining\t2\t9\n", "", 1),
		strings.Replace(recordsBase, "B::y\ttraining\t2", "B::y\ttraining\t7", 1))
	configure(t, repo)
	out, err := git(repo, "merge", "--no-edit", "theirs")
	if err == nil || !strings.Contains(out, "removed") {
		t.Fatalf("want the merge to stop on the removed line, got %v:\n%s", err, out)
	}
	if status := mustGit(t, repo, "status", "--porcelain"); !strings.Contains(status, "UU "+tracked) {
		t.Fatalf("the file is not left in conflict:\n%s", status)
	}
}

func TestUsageAndUnreadableVersions(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"base", "ours"}, &stderr); code != 2 || !strings.Contains(stderr.String(), "usage") {
		t.Errorf("two arguments: code %d, %q", code, stderr.String())
	}
	missing := filepath.Join(t.TempDir(), "absent")
	if code := run([]string{missing, missing, missing}, &stderr); code != 2 {
		t.Errorf("unreadable versions: code %d, want 2", code)
	}
}

// branches makes a repository whose main and theirs branches each changed the tracked file.
func branches(t *testing.T, ours, theirs string) string {
	t.Helper()
	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main")
	write := func(name, content, message string) {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		mustGit(t, repo, "add", name)
		mustGit(t, repo, "commit", "-q", "-m", message)
	}
	write(".gitattributes", tracked+" merge=opensysml-cells\n", "attributes")
	write(tracked, recordsBase, "base")
	mustGit(t, repo, "checkout", "-q", "-b", "theirs")
	write(tracked, theirs, "theirs")
	mustGit(t, repo, "checkout", "-q", "main")
	write(tracked, ours, "ours")
	return repo
}

// configure points the repository's driver at this test binary, which runs the command's own run.
func configure(t *testing.T, repo string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "config", "merge.opensysml-cells.driver", "BASELINE_MERGE_DRIVER=1 '"+self+"' %O %A %B %P")
}

func TestMain(m *testing.M) {
	if os.Getenv("BASELINE_MERGE_DRIVER") == "1" {
		os.Exit(run(os.Args[1:], os.Stderr))
	}
	os.Exit(m.Run())
}

func git(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false"}, args...)...) // #nosec G204 -- fixed git subcommands over the test's own repository
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func mustGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := git(repo, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}
