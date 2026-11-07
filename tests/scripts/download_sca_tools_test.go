package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeOSVScanner answers --version as osv-scanner 2.x does: three lines, the version first,
// the rest written after a pause so a reader that closes the pipe early is caught every time.
const fakeOSVScanner = `#!/usr/bin/env bash
if [[ "$1" == "--version" ]]; then
	echo "osv-scanner version: %s"
	sleep 0.2
	echo "osv-scalibr version: 0.5.3"
	echo "commit: 0000000"
fi
`

// scaToolsTree copies scripts/download-sca-tools.sh into a throwaway repository root, so the
// script's build/sca lands there, and returns that root.
func scaToolsTree(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the script needs bash")
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "download-sca-tools.sh"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, dir := range []string{"scripts", filepath.Join("build", "sca", "bin")} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "download-sca-tools.sh"), script, 0o755); err != nil { // #nosec G306 -- the script under test
		t.Fatal(err)
	}
	return root
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil { // #nosec G306 -- an executable stub
		t.Fatal(err)
	}
}

// runSCATools provisions osv-scanner with the stubs dir first on PATH.
func runSCATools(t *testing.T, root, stubs string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", filepath.Join(root, "scripts", "download-sca-tools.sh"), "osv-scanner") // #nosec G204 -- the repository's own script
	cmd.Env = append(os.Environ(), "PATH="+stubs+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func pinnedOSVScannerVersion(t *testing.T) string {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "download-sca-tools.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(script), "\n") {
		if v, ok := strings.CutPrefix(line, `OSV_SCANNER_VERSION="`); ok {
			return strings.TrimSuffix(v, `"`)
		}
	}
	t.Fatal("the script pins no OSV_SCANNER_VERSION")
	return ""
}

func TestSCAToolsAcceptsAnInstalledScannerWithoutFetching(t *testing.T) {
	root := scaToolsTree(t)
	version := pinnedOSVScannerVersion(t)
	writeExecutable(t, filepath.Join(root, "build", "sca", "bin", "osv-scanner"), strings.ReplaceAll(fakeOSVScanner, "%s", version))
	stubs := t.TempDir()
	writeExecutable(t, filepath.Join(stubs, "curl"), "#!/usr/bin/env bash\necho 'curl: the installed scanner was not accepted' >&2\nexit 7\n")
	out, err := runSCATools(t, root, stubs)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "osv-scanner "+version+" already installed") {
		t.Fatalf("the installed scanner was not recognised:\n%s", out)
	}
}

func TestSCAToolsChecksTheScannerItInstalls(t *testing.T) {
	version := pinnedOSVScannerVersion(t)
	for name, installed := range map[string]string{"pinned": version, "other": "0.0.1"} {
		t.Run(name, func(t *testing.T) {
			root := scaToolsTree(t)
			stubs := t.TempDir()
			fake := filepath.Join(stubs, "fake-osv-scanner")
			writeExecutable(t, fake, strings.ReplaceAll(fakeOSVScanner, "%s", installed))
			// curl -o <out> <url> delivers the fake; sha256sum accepts it, the pin being the real binary's.
			writeExecutable(t, filepath.Join(stubs, "curl"), "#!/usr/bin/env bash\nwhile [[ $# -gt 0 ]]; do if [[ $1 == -o ]]; then cp "+fake+" \"$2\"; shift; fi; shift; done\n")
			writeExecutable(t, filepath.Join(stubs, "sha256sum"), "#!/usr/bin/env bash\ncat >/dev/null\n")
			out, err := runSCATools(t, root, stubs)
			if installed == version {
				if err != nil {
					t.Fatalf("%v\n%s", err, out)
				}
				if _, err := os.Stat(filepath.Join(root, "build", "sca", "bin", "osv-scanner")); err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatalf("a scanner of version %s passed the %s check:\n%s", installed, version, out)
			}
		})
	}
}
