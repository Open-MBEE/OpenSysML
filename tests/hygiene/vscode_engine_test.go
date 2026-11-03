package hygiene

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestVSCodeTypingMatchesTheEngineFloor: editors/vscode's @types/vscode is at or below its engines.vscode
// floor, major.minor; `vsce package` refuses a typing newer than the engine, so Dependabot must not move it alone.
func TestVSCodeTypingMatchesTheEngineFloor(t *testing.T) {
	var manifest struct {
		Engines         map[string]string `json:"engines"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	readJSON(t, "../../editors/vscode/package.json", &manifest)
	engine, ok := manifest.Engines["vscode"]
	if !ok {
		t.Fatal("editors/vscode/package.json declares no engines.vscode")
	}
	typing, ok := manifest.DevDependencies["@types/vscode"]
	if !ok {
		t.Fatal("editors/vscode/package.json declares no @types/vscode devDependency")
	}
	floor, err := majorMinor(strings.TrimLeft(engine, "^~>="))
	if err != nil {
		t.Fatalf("engines.vscode %q: %v", engine, err)
	}
	pinned, err := majorMinor(typing)
	if err != nil {
		t.Fatalf("@types/vscode %q: %v", typing, err)
	}
	if slices.Compare(pinned[:], floor[:]) > 0 {
		t.Errorf("@types/vscode %s exceeds the engines.vscode floor %s; vsce refuses a typing newer than the engine,\n"+
			"so raise engines.vscode with it or keep the typing at the floor", typing, engine)
	}
}

// majorMinor parses the leading major.minor of a semver string; the engine floor and the typing
// are compared at that granularity because @types/vscode tracks VS Code's major.minor.
func majorMinor(version string) ([2]int, error) {
	var parsed [2]int
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return parsed, fmt.Errorf("want major.minor, got %q", version)
	}
	for i, part := range parts[:2] {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return parsed, fmt.Errorf("component %q is not a non-negative integer", part)
		}
		parsed[i] = n
	}
	return parsed, nil
}

// TestDependabotLeavesTheVSCodeTypingAlone: the npm entry covering editors/vscode ignores @types/vscode
// for every update type, so the weekly update does not re-offer the bump the test above forbids.
func TestDependabotLeavesTheVSCodeTypingAlone(t *testing.T) {
	raw, err := os.ReadFile("../../.github/dependabot.yml")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Updates []struct {
			Ecosystem   string   `yaml:"package-ecosystem"`
			Directory   string   `yaml:"directory"`
			Directories []string `yaml:"directories"`
			Ignore      []struct {
				Name        string   `yaml:"dependency-name"`
				Versions    []string `yaml:"versions"`
				UpdateTypes []string `yaml:"update-types"`
			} `yaml:"ignore"`
		} `yaml:"updates"`
	}
	if err := yaml.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	covering := 0
	for _, update := range config.Updates {
		if update.Ecosystem != "npm" || !slices.Contains(append(update.Directories, update.Directory), "/editors/vscode") {
			continue
		}
		covering++
		ignored := false
		for _, rule := range update.Ignore {
			if rule.Name != "@types/vscode" {
				continue
			}
			if len(rule.Versions) > 0 || len(rule.UpdateTypes) > 0 {
				t.Errorf("the @types/vscode ignore rule is limited to %v %v; it must ignore every update", rule.Versions, rule.UpdateTypes)
			}
			ignored = true
		}
		if !ignored {
			t.Error("the npm Dependabot entry covering /editors/vscode does not ignore @types/vscode")
		}
	}
	if covering == 0 {
		t.Fatal("no npm Dependabot entry covers /editors/vscode")
	}
}
