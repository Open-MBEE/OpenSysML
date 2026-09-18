package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCommitSHA = "0123456789abcdef0123456789abcdef01234567"

func TestCommitSHASymrefLooseRef(t *testing.T) {
	root := newGitTestRoot(t)
	writeGitTestFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeGitTestFile(t, filepath.Join(root, ".git", "refs", "heads", "main"), testCommitSHA+"\n")

	assertCommitSHA(t, root, testCommitSHA)
}

func TestCommitSHASymrefPackedRefs(t *testing.T) {
	root := newGitTestRoot(t)
	writeGitTestFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeGitTestFile(t, filepath.Join(root, ".git", "packed-refs"),
		"# pack-refs with: peeled fully-peeled\n"+testCommitSHA+" refs/heads/main\n")

	assertCommitSHA(t, root, testCommitSHA)
}

func TestCommitSHADetachedHead(t *testing.T) {
	root := newGitTestRoot(t)
	writeGitTestFile(t, filepath.Join(root, ".git", "HEAD"), testCommitSHA+"\n")

	assertCommitSHA(t, root, testCommitSHA)
}

func TestCommitSHA256DetachedHead(t *testing.T) {
	root := newGitTestRoot(t)
	sha := strings.Repeat("a", 64)
	writeGitTestFile(t, filepath.Join(root, ".git", "HEAD"), sha+"\n")

	assertCommitSHA(t, root, sha)
}

func TestCommitSHAGitdirFile(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, "gitdir")
	if err := os.MkdirAll(gitDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeGitTestFile(t, filepath.Join(root, ".git"), "gitdir: gitdir\n")
	writeGitTestFile(t, filepath.Join(gitDir, "HEAD"), testCommitSHA+"\n")

	assertCommitSHA(t, root, testCommitSHA)
}

func TestCommitSHAMissingHead(t *testing.T) {
	root := newGitTestRoot(t)

	_, err := commitSHA(root)
	assertGitMetadataError(t, err)
	if !strings.Contains(err.Error(), filepath.Join(".git", "HEAD")) {
		t.Fatalf("error %q does not name HEAD", err)
	}
}

func TestCommitSHAMalformedHead(t *testing.T) {
	root := newGitTestRoot(t)
	writeGitTestFile(t, filepath.Join(root, ".git", "HEAD"), "deadbeef\n")

	_, err := commitSHA(root)
	assertGitMetadataError(t, err)
	if !strings.Contains(err.Error(), "invalid commit SHA") {
		t.Fatalf("error %q does not identify the invalid SHA", err)
	}
}

func newGitTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeGitTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertCommitSHA(t *testing.T, root, want string) {
	t.Helper()
	got, err := commitSHA(root)
	if err != nil {
		t.Fatalf("commitSHA: %v", err)
	}
	if got != want {
		t.Fatalf("commitSHA = %q, want %q", got, want)
	}
}

func assertGitMetadataError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("commitSHA returned nil error")
	}
	var metadataErr *gitMetadataError
	if !errors.As(err, &metadataErr) {
		t.Fatalf("error %T is not a gitMetadataError: %v", err, err)
	}
}
