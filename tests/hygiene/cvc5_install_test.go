package hygiene

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

// CI installs cvc5 through .github/actions/cvc5, which keeps the verified release zip in the Actions
// cache and downloads it with retries, so one failed connection to GitHub does not fail a job.

const (
	cvc5Action        = "./.github/actions/cvc5"
	cvc5InstallScript = "scripts/ci-cvc5-install.sh"
	cvc5PinScript     = "scripts/cvc5-pin.sh"
)

// cvc5Zip builds a release zip whose bin/cvc5 answers --version with the banner.
func cvc5Zip(t *testing.T, banner string) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	member, err := writer.Create("cvc5-Linux-x86_64-static/bin/cvc5")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := member.Write([]byte("#!/bin/sh\necho '" + banner + "'\necho 'a second line'\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

// cvc5Host is a stand-in for the release host: it drops its first connections, then serves one asset.
type cvc5Host struct {
	server   *httptest.Server
	bundle   string
	requests atomic.Int32
}

// newCVC5Host takes what it serves before it listens: its handler shares only the request count with the test.
func newCVC5Host(t *testing.T, asset []byte, drops int32) *cvc5Host {
	t.Helper()
	for _, tool := range []string{"curl", "unzip", "sha256sum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH", tool)
		}
	}
	host := &cvc5Host{}
	host.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if host.requests.Add(1) <= drops {
			if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
				_ = conn.Close()
			}
			return
		}
		_, _ = w.Write(asset)
	}))
	t.Cleanup(host.server.Close)
	host.bundle = filepath.Join(t.TempDir(), "ca.pem")
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: host.server.Certificate().Raw})
	if err := os.WriteFile(host.bundle, cert, 0o600); err != nil {
		t.Fatal(err)
	}
	return host
}

// install runs the script against the host with the pinned zip's digest, and reports its output.
func (h *cvc5Host) install(t *testing.T, pinned []byte, cache, bin string) (string, error) {
	t.Helper()
	digest := sha256.Sum256(pinned)
	cmd := exec.Command("../../" + cvc5InstallScript)
	cmd.Env = append(os.Environ(), "CURL_CA_BUNDLE="+h.bundle, "CVC5_URL="+h.server.URL+"/cvc5.zip",
		"CVC5_SHA256="+hex.EncodeToString(digest[:]), "CI_CVC5_CACHE="+cache, "CI_CVC5_BIN="+bin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestCVC5InstallRidesOutAFailedConnection: a dropped connection is retried, the verified zip is kept,
// and a run that has it, or has a damaged copy of it, asks the host nothing, or once.
func TestCVC5InstallRidesOutAFailedConnection(t *testing.T) {
	pinned := cvc5Zip(t, "This is cvc5 version pinned")
	host := newCVC5Host(t, pinned, 1)
	cache, bin := filepath.Join(t.TempDir(), "cache"), t.TempDir()
	kept := filepath.Join(cache, "cvc5-Linux-x86_64-static.zip")

	out, err := host.install(t, pinned, cache, bin)
	if err != nil {
		t.Fatalf("a host that drops one connection failed the install: %v\n%s", err, out)
	}
	if got := host.requests.Load(); got != 2 {
		t.Errorf("the host was asked %d times, want the zip on the second", got)
	}
	if !strings.HasSuffix(out, "This is cvc5 version pinned\n") {
		t.Errorf("the install does not end on the first line of the version banner:\n%s", out)
	}
	if raw, _ := os.ReadFile(kept); !bytes.Equal(raw, pinned) {
		t.Errorf("%s does not hold the pinned zip", kept)
	}

	if err := os.Remove(filepath.Join(bin, "cvc5")); err != nil {
		t.Fatal(err)
	}
	if out, err := host.install(t, pinned, cache, bin); err != nil {
		t.Fatalf("an install from the kept zip failed: %v\n%s", err, out)
	}
	if got := host.requests.Load(); got != 2 {
		t.Errorf("an install from the kept zip asked the host; %d requests, want 2", got)
	}
	if _, err := os.Stat(filepath.Join(bin, "cvc5")); err != nil {
		t.Errorf("an install from the kept zip installed nothing: %v", err)
	}

	if err := os.WriteFile(kept, []byte("damaged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := host.install(t, pinned, cache, bin); err != nil {
		t.Fatalf("an install over a damaged kept zip failed: %v\n%s", err, out)
	}
	if got := host.requests.Load(); got != 3 {
		t.Errorf("a damaged kept zip was not downloaded again once; %d requests, want 3", got)
	}
	if raw, _ := os.ReadFile(kept); !bytes.Equal(raw, pinned) {
		t.Errorf("%s still holds the damaged zip", kept)
	}
}

// TestCVC5InstallRefusesASubstitutedAsset: a zip that is not the pinned one fails the install at
// once, with nothing kept and nothing installed.
func TestCVC5InstallRefusesASubstitutedAsset(t *testing.T) {
	host := newCVC5Host(t, cvc5Zip(t, "This is cvc5 version substituted"), 0)
	cache, bin := filepath.Join(t.TempDir(), "cache"), t.TempDir()

	out, err := host.install(t, cvc5Zip(t, "This is cvc5 version pinned"), cache, bin)
	if err == nil {
		t.Fatalf("a substituted zip was installed:\n%s", out)
	}
	if got := host.requests.Load(); got != 1 {
		t.Errorf("a substituted zip was asked for %d times; a digest mismatch is not retried", got)
	}
	if !strings.Contains(out, "is not the pinned cvc5") {
		t.Errorf("the failure does not name the digest mismatch:\n%s", out)
	}
	for _, left := range []string{filepath.Join(cache, "cvc5-Linux-x86_64-static.zip"), filepath.Join(bin, "cvc5")} {
		if _, err := os.Stat(left); err == nil {
			t.Errorf("%s was left behind by a refused install", left)
		}
	}
}

// TestCVC5InstallsGoThroughTheCachedAction: the pin has one source, no CI file downloads cvc5 itself,
// and every workflow that installs it uses the action.
func TestCVC5InstallsGoThroughTheCachedAction(t *testing.T) {
	sourceOfTruth(t, "../../"+cvc5PinScript, regexp.MustCompile(`(?m)^CVC5_SHA256="\$\{CVC5_SHA256:-([0-9a-f]{64})\}"$`))
	if found := pinnedIn(t, regexp.MustCompile(`(github\.com/cvc5/cvc5/releases/download/\S+)`)); len(found) != 0 {
		t.Errorf("%v download cvc5 themselves; install it through `uses: %s`, or %s outside GitHub Actions", found, cvc5Action, cvc5InstallScript)
	}
	circle, err := os.ReadFile("../../.circleci/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(circle), "command: "+cvc5InstallScript+"\n") {
		t.Errorf(".circleci/config.yml does not install cvc5 through %s", cvc5InstallScript)
	}

	uses := 0
	for _, path := range actionsFiles(t) {
		if strings.HasSuffix(filepath.ToSlash(path), "/.github/actions/cvc5/action.yml") {
			continue
		}
		for where, steps := range actionsSteps(t, path) {
			for _, step := range steps {
				if step.Uses == cvc5Action {
					uses++
				}
				if strings.Contains(step.Run, cvc5InstallScript) {
					t.Errorf("%s step %q runs %s directly; use `uses: %s` so the zip is cached", where, step.Name, cvc5InstallScript, cvc5Action)
				}
			}
		}
	}
	if uses < 3 {
		t.Errorf("found %d uses of %s; the scan no longer finds the callers", uses, cvc5Action)
	}
}

// TestCVC5ActionInstallsFromTheRestoredZip: the action restores, by the pin's exact key alone, the
// directory the script installs from, and saves it under that key once the install succeeded.
func TestCVC5ActionInstallsFromTheRestoredZip(t *testing.T) {
	action, _ := loadCompositeAction(t, "../../.github/actions/cvc5/action.yml")
	if action.Runs.Using != "composite" {
		t.Errorf("the action runs using %q, not composite", action.Runs.Using)
	}
	var pin, restore, install, save *actionsStep
	for i := range action.Runs.Steps {
		step := &action.Runs.Steps[i]
		switch {
		case strings.HasPrefix(step.Uses, "actions/cache/restore@"):
			restore = step
		case strings.HasPrefix(step.Uses, "actions/cache/save@"):
			save = step
		case strings.Contains(step.Run, cvc5InstallScript+" --pin"):
			pin = step
		case strings.Contains(step.Run, cvc5InstallScript):
			install = step
		}
	}
	if pin == nil || restore == nil || install == nil || save == nil {
		t.Fatalf("the action lacks a pin (%v), a cache restore (%v), an install (%v) or a cache save (%v) step",
			pin != nil, restore != nil, install != nil, save != nil)
	}
	cachePinsAgreeWithTheWorkflows(t, "/.github/actions/cvc5/action.yml", restore, save)

	path := restore.With["path"]
	if path == "" {
		t.Error("the restore step names no path")
	}
	if save.With["path"] != path {
		t.Errorf("the save step saves %q, the restore step restored %q", save.With["path"], path)
	}
	if install.Env["CI_CVC5_CACHE"] != path {
		t.Errorf("the install step's CI_CVC5_CACHE is %q, the restored path is %q", install.Env["CI_CVC5_CACHE"], path)
	}
	key := restore.With["key"]
	for _, part := range []string{"runner.os", "runner.arch", "steps." + pin.ID + ".outputs.version", "steps." + pin.ID + ".outputs.sha256"} {
		if !strings.Contains(key, part) {
			t.Errorf("the restore key %q does not vary with %s", key, part)
		}
	}
	if save.With["key"] != key {
		t.Errorf("the save key %q is not the restore key %q", save.With["key"], key)
	}
	if restore.With["restore-keys"] != "" {
		t.Errorf("the restore step falls back to %q, a zip saved for another pin", restore.With["restore-keys"])
	}
	if save.If != "steps."+restore.ID+".outputs.cache-hit != 'true'" {
		t.Errorf("the save step runs if %q, not on a cache miss alone", save.If)
	}

	// The script, not the action, verifies the zip and bounds the download through the shared fetch.
	script, err := os.ReadFile("../../" + cvc5InstallScript)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sha256sum --check --strict", `scripts/fetch-https.sh" "$CVC5_URL"`, `source "$repo_root/` + cvc5PinScript + `"`} {
		if !strings.Contains(string(script), want) {
			t.Errorf("%s lacks %q", cvc5InstallScript, want)
		}
	}
	fetch, err := os.ReadFile("../../scripts/fetch-https.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--connect-timeout 30 ", "--retry-all-errors "} {
		if !strings.Contains(string(fetch), flag) {
			t.Errorf("scripts/fetch-https.sh no longer passes curl %q, which a failed connection is retried by", strings.TrimSpace(flag))
		}
	}
}
