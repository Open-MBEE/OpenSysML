package hygiene

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestNodeLockfileAgreesWithTheClientVersion: client/node's lockfile must not lock an
// @openmbee package at another version than package.json's (npm ci refuses it); absent is fine.
func TestNodeLockfileAgreesWithTheClientVersion(t *testing.T) {
	var manifest struct {
		Version              string            `json:"version"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
		PeerDependencies     map[string]string `json:"peerDependencies"`
	}
	readJSON(t, "../../client/node/package.json", &manifest)
	var lock struct {
		Packages map[string]struct {
			Version              string            `json:"version"`
			OptionalDependencies map[string]string `json:"optionalDependencies"`
			PeerDependencies     map[string]string `json:"peerDependencies"`
		} `json:"packages"`
	}
	readJSON(t, "../../client/node/package-lock.json", &lock)

	want := map[string]string{}
	for name, version := range manifest.OptionalDependencies {
		if strings.HasPrefix(name, "@openmbee/") {
			want[name] = version
		}
	}
	for name, version := range manifest.PeerDependencies {
		if strings.HasPrefix(name, "@openmbee/") {
			want[name] = version
		}
	}
	if len(want) == 0 {
		t.Fatal("client/node/package.json declares no @openmbee dependency; the test no longer checks anything")
	}
	for name, version := range want {
		if version != manifest.Version {
			t.Errorf("package.json pins %s at %s but the client is %s", name, version, manifest.Version)
		}
	}

	root := lock.Packages[""]
	for name, version := range want {
		if got := root.OptionalDependencies[name]; got != "" && got != version {
			t.Errorf("package-lock.json's root pins %s at %s, package.json at %s", name, got, version)
		}
		if got := root.PeerDependencies[name]; got != "" && got != version {
			t.Errorf("package-lock.json's root pins peer %s at %s, package.json at %s", name, got, version)
		}
		entry, locked := lock.Packages["node_modules/"+name]
		if locked && entry.Version != version {
			t.Errorf("package-lock.json locks %s at %s, package.json at %s;\n"+
				"run `npm install --package-lock-only --ignore-scripts` in client/node once the %s packages are on npm",
				name, entry.Version, version, version)
		}
	}
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}
