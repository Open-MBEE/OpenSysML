package hideo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

func TestReadPinReadsTheCommittedScript(t *testing.T) {
	root, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(RepositoryEnv, "")
	t.Setenv(CommitEnv, "")
	pin, err := ReadPin(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(pin.Repository, "hideo67/sysmlv2-tests.git") {
		t.Errorf("repository = %q", pin.Repository)
	}
	committed, err := ReadBaseline(filepath.Join(root, filepath.FromSlash(BaselinePath)))
	if err != nil {
		t.Fatal(err)
	}
	if committed.Pin != pin {
		t.Errorf("%s was measured at %+v, %s pins %+v: re-record with -update", BaselinePath, committed.Pin, PinPath, pin)
	}
	if err := CheckDocTable(filepath.Join(root, filepath.FromSlash(DocPath)), committed); err != nil {
		t.Error(err)
	}
	doc, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(DocPath)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), pin.Commit) {
		t.Errorf("%s does not name the pinned commit %s", DocPath, pin.Commit)
	}
}

func TestReadPinTakesTheEnvironmentAndRefusesAShortCommit(t *testing.T) {
	root, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(RepositoryEnv, testPin.Repository)
	t.Setenv(CommitEnv, testPin.Commit)
	if pin, err := ReadPin(root); err != nil || pin != testPin {
		t.Errorf("pin = %+v, %v", pin, err)
	}
	t.Setenv(CommitEnv, "99eae49")
	if _, err := ReadPin(root); err == nil {
		t.Error("an abbreviated commit was accepted")
	}
}

func TestVerifyRefusesAnotherCommitAndAModifiedTree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := git(dir, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	file := filepath.Join(dir, "case.json")
	if err := os.WriteFile(file, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("init", "--quiet")
	run("add", "case.json")
	run("-c", "user.name=t", "-c", "user.email=t@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "c")
	pin := Pin{Repository: testPin.Repository, Commit: run("rev-parse", "HEAD")}
	if err := pin.Verify(dir); err != nil {
		t.Fatal(err)
	}
	if err := testPin.Verify(dir); err == nil || !strings.Contains(err.Error(), "not the pinned") {
		t.Errorf("another commit: %v", err)
	}
	if err := os.WriteFile(file, []byte("{\"edited\":true}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := pin.Verify(dir); err == nil || !strings.Contains(err.Error(), "modified tracked files") {
		t.Errorf("a modified tree: %v", err)
	}
}
