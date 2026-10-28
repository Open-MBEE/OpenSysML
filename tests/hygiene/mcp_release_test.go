package hygiene

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// nextJobHeader matches the start of the job (or top-level key) that follows
// the one being scanned in .circleci/config.yml.
var nextJobHeader = regexp.MustCompile(`\n  [a-z][^\n]*:\n|\n[a-z][^\n]*:`)

// mcpPlatforms is every platform the release builds, as the archive names
// spell it.
var mcpPlatforms = []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"}

// readRepoFile reads a file by its path from the repository root.
func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

// requireFragments fails for every fragment the text lacks, naming the file.
func requireFragments(t *testing.T, path, text string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			t.Errorf("%s no longer contains %q", path, fragment)
		}
	}
}

// TestMCPServerShipsAsItsOwnArchive pins how sysml-mcp is released: built for
// every platform by the release job, packed into sysml-mcp-<os>-<arch>.tar.gz
// (.zip on Windows) with its manual page and the LICENSE, hashed into
// SHA256SUMS.txt with the other archives, and kept out of both the
// opensysml-<platform> bundle and the raw binary assets. Dropping any one of
// these silently would leave an installer or a package manager pointing at an
// asset the release no longer has.
func TestMCPServerShipsAsItsOwnArchive(t *testing.T) {
	config := loadCircleConfig(t)
	binaries, ok := config.Jobs["build-release-binaries"]
	if !ok {
		t.Fatal("no build-release-binaries job")
	}
	steps := runSteps(t, binaries.Steps)
	build := stepIndex(steps, "make build-mcp")
	if build < 0 {
		t.Fatal("build-release-binaries has no step running make build-mcp")
	}
	text := steps[build].Command
	for _, platform := range mcpPlatforms {
		goos, goarch, _ := strings.Cut(platform, "-")
		if !containsAll(text, "GOOS="+goos+" GOARCH="+goarch+" make build-mcp") {
			t.Errorf("build-release-binaries does not build sysml-mcp for %s", platform)
		}
		dest := "dist/mcp/sysml-mcp-" + platform
		if goos == "windows" {
			dest += ".exe"
		}
		if !strings.Contains(text, "mv bin/sysml-mcp "+dest) {
			t.Errorf("build-release-binaries does not keep the %s sysml-mcp under dist/mcp (%s), where the per-binary archive loop does not pack it bare", platform, dest)
		}
	}
	requireFragments(t, ".circleci/config.yml (build step)", text,
		`tar czf "sysml-mcp-${platform}.tar.gz" -C "$stage" sysml-mcp LICENSE share`,
		`cp ../packaging/man/man1/sysml-mcp.1 "${stage}/share/man/man1/"`,
		`zip -q ../../sysml-mcp-windows-amd64.zip sysml-mcp.exe LICENSE`,
		// The bundle stays the two editor binaries.
		`tar czf "opensysml-${platform}.tar.gz" -C "$stage" sysml sysml-lsp share`,
		`zip -q ../../opensysml-windows-amd64.zip sysml.exe sysml-lsp.exe`,
	)
	if i := stepIndex(steps, "check-static-binaries.sh"); i < 0 || !containsAll(steps[i].Command, "dist/mcp/sysml-mcp-linux-amd64", "dist/mcp/sysml-mcp-linux-arm64") {
		t.Error("the static-linking check does not cover the Linux sysml-mcp binaries")
	}
	if i := stepIndex(steps, "does not carry the tag"); i < 0 || !containsAll(steps[i].Command, "dist/mcp/sysml-mcp-linux-amd64", "dist/mcp/sysml-mcp-*") {
		t.Error("the tag check does not cover the sysml-mcp binaries")
	}

	// The raw binaries under dist/mcp are never published as release assets:
	// only the archives in dist/ reach the release, through the dist/*.tar.gz
	// and dist/*.zip globs every other per-binary archive travels by. The other
	// jobs that read them are build-npm-package, whose generator derives the
	// alias beside the platform packages it builds from them, and publish-npm,
	// which packages them, verified against their sidecars, as
	// @openmbee/sysml-mcp's platform packages.
	raw := readRepoFile(t, ".circleci/config.yml")
	allowed := [][2]int{jobSpan(t, raw, "build-release-binaries"), jobSpan(t, raw, "build-npm-package"), jobSpan(t, raw, "publish-npm")}
	for i := strings.Index(raw, "dist/mcp"); i >= 0; {
		inside := false
		for _, span := range allowed {
			if i >= span[0] && i < span[1] {
				inside = true
			}
		}
		if !inside {
			line := 1 + strings.Count(raw[:i], "\n")
			t.Errorf(".circleci/config.yml line %d touches dist/mcp outside build-release-binaries, build-npm-package and publish-npm; the raw sysml-mcp binaries are not release assets", line)
		}
		next := strings.Index(raw[i+1:], "dist/mcp")
		if next < 0 {
			break
		}
		i += 1 + next
	}
	if stepIndex(runSteps(t, config.Jobs["publish-github-release"].Steps), "dist/*.tar.gz dist/*.zip dist/release/") < 0 {
		t.Error("publish-github-release no longer moves dist/*.tar.gz and dist/*.zip into the release, which is how the sysml-mcp archives are published")
	}

	// The local rehearsal of the release job builds and packs the same archive.
	script := readRepoFile(t, "scripts/build-release-artifacts.sh")
	requireFragments(t, "scripts/build-release-artifacts.sh", script,
		`build build-mcp sysml-mcp "$platform" "$DIST/mcp/sysml-mcp-${platform}"`,
		`tar czf "sysml-mcp-${platform}.tar.gz" -C "$stage" sysml-mcp LICENSE share`,
		`zip -q "../../sysml-mcp-${platform}.zip" sysml-mcp.exe LICENSE`,
		`cp "$MAN/sysml-mcp.1" "${stage}/share/man/man1/"`,
		`mcp/sysml-mcp-linux-*`,
	)

	// The signed Windows build carries the same layout under the -signed suffix.
	windows := readRepoFile(t, ".github/workflows/release-windows.yml")
	requireFragments(t, ".github/workflows/release-windows.yml", windows,
		"GOOS=windows GOARCH=amd64 make build-mcp",
		"mv bin/sysml-mcp dist/unsigned/sysml-mcp-windows-amd64.exe",
		"for name in sysml sysml-lsp sysml-grpc sysml-mcp; do",
		"zip -q ../../release/sysml-mcp-windows-amd64-signed.zip sysml-mcp.exe LICENSE",
		"dist/unsigned/sysml-grpc-windows-amd64.exe dist/unsigned/sysml-mcp-windows-amd64.exe",
		"dist/signed/sysml-grpc-windows-amd64.exe dist/signed/sysml-mcp-windows-amd64.exe",
	)
	// publish-signed checks nothing out, so the LICENSE the signed zip carries
	// has to arrive as the packaging-inputs artifact the build job uploads.
	publish := windows[strings.Index(windows, "\n  publish-signed:\n"):]
	if next := nextJobHeader.FindStringIndex(publish[1:]); next != nil {
		publish = publish[:1+next[0]]
	}
	requireFragments(t, ".github/workflows/release-windows.yml (publish-signed)", publish,
		"name: ${{ env.MSI_INPUTS_ARTIFACT }}",
		"cp ../../LICENSE stage-mcp/LICENSE",
	)

	// Everything that installs from the release knows the archive.
	for _, probe := range []struct{ path, fragment string }{
		{"install.sh", `known_tools="sysml sysml-lsp sysml-grpc sysml-jupyter-kernel sysml-mcp"`},
		{"install.sh", `mcp_asset="sysml-mcp-$platform.tar.gz"`},
		{"install.sh", `mcp_asset="sysml-mcp-$platform.zip"`},
		{"install.ps1", `'sysml-jupyter-kernel', 'sysml-mcp')`},
		{"install.ps1", `"sysml-mcp-$Platform.zip"`},
		{"install.ps1", `"sysml-mcp-$Platform.tar.gz"`},
		{"install.ps1", `"sysml-mcp-$Platform-signed.zip"`},
		{"scripts/render-homebrew-formula.sh", `sum_for sysml-mcp-darwin-arm64.tar.gz`},
		{"scripts/render-homebrew-formula.sh", `sum_for sysml-mcp-darwin-amd64.tar.gz`},
		{"scripts/render-homebrew-formula.sh", `sum_for sysml-mcp-linux-arm64.tar.gz`},
		{"scripts/render-homebrew-formula.sh", `sum_for sysml-mcp-linux-amd64.tar.gz`},
		{"scripts/render-scoop-manifest.sh", `sum_for sysml-mcp-windows-amd64.zip`},
		{"Makefile", "build: build-sysml build-lsp build-grpc build-jupyter-kernel build-mcp"},
	} {
		requireFragments(t, probe.path, readRepoFile(t, probe.path), probe.fragment)
	}
	formula := readRepoFile(t, "packaging/homebrew/Formula/opensysml.rb")
	for _, platform := range []string{"darwin-arm64", "darwin-amd64", "linux-arm64", "linux-amd64"} {
		requireFragments(t, "packaging/homebrew/Formula/opensysml.rb", formula,
			"/__TAG__/sysml-mcp-"+platform+".tar.gz",
			"__SHA256_MCP_"+strings.ToUpper(strings.ReplaceAll(platform, "-", "_"))+"__")
	}
	requireFragments(t, "packaging/homebrew/Formula/opensysml.rb", formula,
		`resource("sysml-mcp").stage do`, `bin.install "sysml-mcp"`, `man1.install "share/man/man1/sysml-mcp.1"`)
	if got := strings.Count(formula, `resource "sysml-mcp" do`); got != 4 {
		t.Errorf("the Homebrew formula declares the sysml-mcp resource %d times, want one per platform (4)", got)
	}

	var scoop struct {
		Architecture map[string]struct {
			URL  []string `json:"url"`
			Hash []string `json:"hash"`
		} `json:"architecture"`
		Bin []string `json:"bin"`
	}
	if err := json.Unmarshal([]byte(readRepoFile(t, "packaging/scoop/opensysml.json")), &scoop); err != nil {
		t.Fatalf("packaging/scoop/opensysml.json: %v (the 64bit url and hash must be arrays, one entry per archive)", err)
	}
	arch := scoop.Architecture["64bit"]
	wantURL := "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/sysml-mcp-windows-amd64.zip"
	if len(arch.URL) != 2 || arch.URL[1] != wantURL || len(arch.Hash) != 2 || arch.Hash[1] != "__SHA256_MCP_WINDOWS_AMD64__" {
		t.Errorf("the Scoop manifest's 64bit url/hash = %v / %v; want the bundle then %s with __SHA256_MCP_WINDOWS_AMD64__", arch.URL, arch.Hash, wantURL)
	}
	if !containsAll(strings.Join(scoop.Bin, " "), "sysml-mcp.exe") {
		t.Errorf("the Scoop manifest's bin = %v does not shim sysml-mcp.exe", scoop.Bin)
	}
}

// jobSpan is the byte range of the named job in the raw CircleCI config, from
// its header to the next job's.
func jobSpan(t *testing.T, raw, job string) [2]int {
	t.Helper()
	start := strings.Index(raw, "\n  "+job+":\n")
	if start < 0 {
		t.Fatalf("no %s job header in .circleci/config.yml", job)
	}
	end := len(raw)
	if next := nextJobHeader.FindStringIndex(raw[start+1:]); next != nil {
		end = start + 1 + next[0]
	}
	return [2]int{start, end}
}
