package hygiene

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var setupGoUse = regexp.MustCompile(`uses: (actions/setup-go@[0-9a-f]{40}) # v(\d+)\.`)

// TestSetupGoInstallsTheToolchainGoModNames holds every setup-go step to v6 reading go.mod: v5 installed
// the go line's release, whose `go env` downloaded the toolchain line's where the cache restore then collided.
func TestSetupGoInstallsTheToolchainGoModNames(t *testing.T) {
	workflows, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	actions, err := filepath.Glob("../../.github/actions/*/action.yml")
	if err != nil {
		t.Fatal(err)
	}
	uses, pin := 0, ""
	for _, path := range append(workflows, actions...) {
		raw, err := os.ReadFile(path) // #nosec G304 -- a workflow of this repository
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
		// setup-go v6 reads the toolchain line only while GOTOOLCHAIN is not already local.
		if strings.Contains(string(raw), "GOTOOLCHAIN") {
			t.Errorf("%s sets GOTOOLCHAIN: setup-go sets it to local itself, after reading go.mod's toolchain line", name)
		}
		lines := strings.Split(string(raw), "\n")
		for i, line := range lines {
			if !strings.Contains(line, "actions/setup-go@") {
				continue
			}
			uses++
			at := name + ":" + strconv.Itoa(i+1)
			match := setupGoUse.FindStringSubmatch(line)
			if match == nil {
				t.Errorf("%s: setup-go is not pinned as <sha> # v<major>.<minor>.<patch>: %s", at, strings.TrimSpace(line))
				continue
			}
			if major, _ := strconv.Atoi(match[2]); major < 6 {
				t.Errorf("%s: setup-go v%d installs go.mod's go line, not its toolchain line; want v6 or later", at, major)
			}
			if pin == "" {
				pin = match[1]
			}
			if match[1] != pin {
				t.Errorf("%s: %s differs from %s used elsewhere", at, match[1], pin)
			}
			if !setupGoReadsGoMod(lines[i+1:]) {
				t.Errorf("%s: setup-go does not take its version from go-version-file: go.mod", at)
			}
		}
	}
	if uses == 0 {
		t.Fatal("no workflow uses actions/setup-go")
	}
}

// setupGoReadsGoMod reads a step's remaining lines, up to the next step.
func setupGoReadsGoMod(rest []string) bool {
	for _, line := range rest {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			return false
		}
		if trimmed == "go-version-file: go.mod" {
			return true
		}
	}
	return false
}
