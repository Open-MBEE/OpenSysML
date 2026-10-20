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

// TestMCPRunnerManifestAgreesWithTheClientVersion pins the npm runner to the
// client's version: @openmbee/sysml-mcp and @openmbee/opensysml are published
// from one tag at one version, and the runner's five platform packages, which
// npm selects by os/cpu, are pinned at exactly that version with nothing else
// to install.
func TestMCPRunnerManifestAgreesWithTheClientVersion(t *testing.T) {
	var client struct {
		Version string `json:"version"`
	}
	readJSON(t, "../../client/node/package.json", &client)
	var runner struct {
		Name                 string            `json:"name"`
		Version              string            `json:"version"`
		MCPName              string            `json:"mcpName"`
		Dependencies         map[string]string `json:"dependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
		Bin                  map[string]string `json:"bin"`
	}
	readJSON(t, "../../packaging/npm/sysml-mcp/package.json", &runner)
	if runner.Name != "@openmbee/sysml-mcp" {
		t.Errorf("packaging/npm/sysml-mcp/package.json names %q, want @openmbee/sysml-mcp", runner.Name)
	}
	if runner.Version != client.Version {
		t.Errorf("@openmbee/sysml-mcp is %s but @openmbee/opensysml is %s; both are published from one tag at one version", runner.Version, client.Version)
	}
	if runner.MCPName != "org.opensysml/sysml-mcp" {
		t.Errorf("mcpName is %q, want org.opensysml/sysml-mcp, the MCP Registry's ownership proof", runner.MCPName)
	}
	if len(runner.Dependencies) > 0 {
		t.Errorf("@openmbee/sysml-mcp has dependencies %v; the runner installs nothing but its platform package", runner.Dependencies)
	}
	if runner.Bin["sysml-mcp"] == "" {
		t.Error("@openmbee/sysml-mcp declares no sysml-mcp command")
	}
	want := []string{"darwin-arm64", "darwin-x64", "linux-arm64", "linux-x64", "win32-x64"}
	if len(runner.OptionalDependencies) != len(want) {
		t.Errorf("@openmbee/sysml-mcp pins %d platform packages, want %d", len(runner.OptionalDependencies), len(want))
	}
	for _, platform := range want {
		name := "@openmbee/sysml-mcp-" + platform
		pinned, ok := runner.OptionalDependencies[name]
		switch {
		case !ok:
			t.Errorf("@openmbee/sysml-mcp does not pin %s", name)
		case pinned != client.Version:
			t.Errorf("@openmbee/sysml-mcp pins %s at %s, not its own version %s", name, pinned, client.Version)
		}
	}
}
