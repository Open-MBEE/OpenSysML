package hygiene

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// earlyExitVersionPipe matches a `--version`/`-version` call piped into head or a quiet grep: either
// reader can close the pipe before the binary finishes writing, which pipefail turns into exit 141.
var earlyExitVersionPipe = regexp.MustCompile(`\s--?version\b[^|]*\|\s*(head\b|grep\s+(-[A-Za-z]+\s+)*-[A-Za-z]*q)`)

// TestVersionChecksDoNotPipeIntoEarlyExitReaders: every script, CircleCI and workflow version check
// captures the output first, so a binary printing more than head or grep -q reads cannot fail the step.
func TestVersionChecksDoNotPipeIntoEarlyExitReaders(t *testing.T) {
	paths := []string{"../../.circleci/config.yml", "../../Makefile"}
	for _, pattern := range []string{"../../scripts/*.sh", "../../scripts/*/*.sh", "../../.github/workflows/*.yml"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, matches...)
	}
	checks := 0
	for _, path := range paths {
		rel := strings.TrimPrefix(path, "../../")
		for i, line := range strings.Split(readRepoFile(t, rel), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "version") {
				continue
			}
			if strings.Contains(trimmed, "-version") {
				checks++
			}
			// An echo's status ignores its substitutions, so a diagnostic may still read the first line.
			if strings.HasPrefix(trimmed, "echo ") {
				continue
			}
			if earlyExitVersionPipe.MatchString(line) {
				t.Errorf("%s:%d pipes a version check into an early-exit reader, which fails with SIGPIPE under pipefail; capture the output and read it from a variable or here-string:\n\t%s", rel, i+1, trimmed)
			}
		}
	}
	if checks == 0 {
		t.Error("no version check found in the scripts or CI configuration; the scan no longer reads them")
	}
}
