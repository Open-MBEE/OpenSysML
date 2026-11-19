package hideo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// PinPath is the script every fetch of the suite sources its pin from.
	PinPath = "scripts/hideo-suite-pin.sh"
	// DefaultRoot is where scripts/download-hideo-suite.sh checks the suite out.
	DefaultRoot = "build/hideo-suite"
	// RequireEnv makes an absent checkout a failure instead of a skip.
	RequireEnv = "OPENSYSML_REQUIRE_HIDEO_SUITE"
	// The script's variables; an environment value overrides each for the
	// downloader, so it overrides here too.
	RepositoryEnv = "HIDEO_SUITE_REPO"
	CommitEnv     = "HIDEO_SUITE_COMMIT"
)

// Pin is the suite's identity as scripts/hideo-suite-pin.sh records it.
type Pin struct {
	Repository string
	Commit     string
}

var (
	pinRepositoryRe = regexp.MustCompile(`HIDEO_SUITE_REPO="\$\{HIDEO_SUITE_REPO:-([^}"]+)\}"`)
	pinCommitRe     = regexp.MustCompile(`HIDEO_SUITE_COMMIT="\$\{HIDEO_SUITE_COMMIT:-([^}"]+)\}"`)
	commitRe        = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// ReadPin resolves the pin as the script does: its defaults under the
// repository root, each replaced by its environment variable when set.
func ReadPin(repo string) (Pin, error) {
	content, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(PinPath))) // #nosec G304 -- the pin is at a fixed path in this repository
	if err != nil {
		return Pin{}, fmt.Errorf("read %s: %w", PinPath, err)
	}
	repository, commit := pinRepositoryRe.FindSubmatch(content), pinCommitRe.FindSubmatch(content)
	if repository == nil || commit == nil {
		return Pin{}, fmt.Errorf("%s pins no %s/%s", PinPath, RepositoryEnv, CommitEnv)
	}
	pin := Pin{Repository: string(repository[1]), Commit: string(commit[1])}
	if v := os.Getenv(RepositoryEnv); v != "" {
		pin.Repository = v
	}
	if v := os.Getenv(CommitEnv); v != "" {
		pin.Commit = v
	}
	if !commitRe.MatchString(pin.Commit) {
		return Pin{}, fmt.Errorf("%s=%q is not a full lowercase commit id", CommitEnv, pin.Commit)
	}
	return pin, nil
}

// Required reports whether an absent checkout must fail rather than skip.
func Required() bool {
	return os.Getenv(RequireEnv) != ""
}

// Verify refuses a checkout that is not the pinned commit with its tracked
// files untouched, before anything in it is run.
func (p Pin) Verify(dir string) error {
	head, err := git(dir, "rev-parse", "HEAD^{commit}")
	if err != nil {
		return err
	}
	if head != p.Commit {
		return fmt.Errorf("%s is at commit %s, not the pinned %s; re-run ./scripts/download-hideo-suite.sh", dir, head, p.Commit)
	}
	changed, err := git(dir, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if changed != "" {
		return fmt.Errorf("%s has modified tracked files, so it is not the pinned suite; re-run ./scripts/download-hideo-suite.sh --force:\n%s", dir, changed)
	}
	return nil
}

func git(dir string, args ...string) (string, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found on PATH: %w", err)
	}
	out, err := exec.Command(path, append([]string{"-C", dir}, args...)...).Output() // #nosec G204 -- git is resolved by LookPath and the arguments are fixed
	if err != nil {
		return "", fmt.Errorf("git -C %s %s: %w", dir, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
