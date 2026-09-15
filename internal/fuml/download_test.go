package fuml

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// suiteCopy copies the installed suite into a fresh root without its pin stamp,
// so the downloader sees verified files it did not fetch.
func suiteCopy(t *testing.T) (root, dir string) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required to run the downloader")
	}
	src, err := Locate(repoRoot)
	if errors.Is(err, ErrSuiteAbsent) && !Required() {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	root = t.TempDir()
	dir = filepath.Join(root, "suite")
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		copyFile(t, filepath.Join(src, e.Name()), filepath.Join(dir, e.Name()))
	}
	libs, err := os.ReadDir(filepath.Join(src, "lib"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range libs {
		copyFile(t, filepath.Join(src, "lib", e.Name()), filepath.Join(dir, "lib", e.Name()))
	}
	return root, dir
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	content, err := os.ReadFile(from) // #nosec G304 -- the installed suite, under the repository's build directory
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// runDownloader runs the script over dir with a curl that refuses every fetch,
// so any download attempt fails the run instead of touching the network.
func runDownloader(t *testing.T, root, dir string, args ...string) (string, error) {
	t.Helper()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	curl := "#!/usr/bin/env bash\necho 'curl: refused by the test' >&2\nexit 7\n"
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(curl), 0o755); err != nil { // #nosec G306 -- an executable stub
		t.Fatal(err)
	}
	script, err := filepath.Abs(filepath.Join(repoRoot, "scripts", "download-fuml-suite.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", append([]string{script}, args...)...) // #nosec G204 -- the repository's own script
	cmd.Env = append(os.Environ(), "FUML_SUITE_ROOT="+dir, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestDownloaderStampsAVerifiedSuiteWithoutFetching(t *testing.T) {
	clearPinEnv(t)
	root, dir := suiteCopy(t)
	stamp := filepath.Join(dir, ".fuml-pin")
	if _, err := os.Stat(stamp); !os.IsNotExist(err) {
		t.Fatalf("stamp before the run: %v", err)
	}
	out, err := runDownloader(t, root, dir)
	if err != nil {
		t.Fatalf("downloader over verified files without a stamp: %v\n%s", err, out)
	}
	if strings.Contains(out, "Fetching http") {
		t.Fatalf("downloader fetched instead of keeping the verified files:\n%s", out)
	}
	if _, err := os.Stat(stamp); err != nil {
		t.Fatalf("stamp after the run: %v\n%s", err, out)
	}
	out, err = runDownloader(t, root, dir)
	if err != nil || !strings.Contains(out, "Already present") {
		t.Fatalf("second run = %v, want Already present:\n%s", err, out)
	}
}

func TestDownloaderRefetchesOnlyWhatFailsVerification(t *testing.T) {
	clearPinEnv(t)
	pin, err := ReadPin(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	root, dir := suiteCopy(t)
	if out, err := runDownloader(t, root, dir); err != nil {
		t.Fatalf("stamping run: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, TestsFile), []byte("truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runDownloader(t, root, dir)
	if err == nil {
		t.Fatalf("downloader succeeded with a corrupt %s and no network:\n%s", TestsFile, out)
	}
	for _, want := range []string{"Corrupt or incomplete suite", "Fetching http", TestsFile, "could not fetch"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "Fetching http"); n != 1 {
		t.Errorf("fetched %d files, want only the corrupt one:\n%s", n, out)
	}
	if err := pin.Verify(dir, JarFile); err != nil {
		t.Errorf("a verified file was disturbed by the failed refresh: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".fuml-fetch.") {
			t.Errorf("staging directory %s left behind", e.Name())
		}
	}
}

func TestDownloaderReportsAStalePinBeforeRefreshing(t *testing.T) {
	clearPinEnv(t)
	root, dir := suiteCopy(t)
	if err := os.WriteFile(filepath.Join(dir, ".fuml-pin"), []byte("an older pin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runDownloader(t, root, dir)
	if err != nil {
		t.Fatalf("stale stamp over verified files: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Stale pin") || strings.Contains(out, "Fetching http") {
		t.Fatalf("want a stale-pin notice and no fetch:\n%s", out)
	}
	content, err := os.ReadFile(filepath.Join(dir, ".fuml-pin"))
	if err != nil || strings.TrimSpace(string(content)) == "an older pin" {
		t.Fatalf("stamp not refreshed: %q, %v", content, err)
	}
}
