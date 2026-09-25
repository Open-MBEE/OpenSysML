package pssm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPinScript = `#!/usr/bin/env bash
PSSM_DOCUMENT="${PSSM_DOCUMENT:-ptc/00-00-00}"
PSSM_VERSION="${PSSM_VERSION:-9.9}"
PSSM_SUITE_URL="${PSSM_SUITE_URL:-https://example.invalid/suite.xmi}"
PSSM_SUITE_SHA256="${PSSM_SUITE_SHA256:-0000000000000000000000000000000000000000000000000000000000000000}"
`

func writePinScript(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	path := filepath.Join(repo, filepath.FromSlash(PinPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(testPinScript), 0o600); err != nil {
		t.Fatal(err)
	}
	return repo
}

func clearPinEnv(t *testing.T) {
	t.Helper()
	for _, env := range []string{DocumentEnv, VersionEnv, URLEnv, SHA256Env} {
		t.Setenv(env, "")
	}
}

func TestReadPinTakesTheScriptsDefaults(t *testing.T) {
	clearPinEnv(t)
	pin, err := ReadPin(writePinScript(t))
	if err != nil {
		t.Fatal(err)
	}
	want := Pin{Document: "ptc/00-00-00", Version: "9.9", URL: "https://example.invalid/suite.xmi", SHA256: strings.Repeat("0", 64)}
	if pin != want {
		t.Fatalf("pin = %+v, want %+v", pin, want)
	}
}

func TestReadPinHonorsTheDownloadersOverrides(t *testing.T) {
	clearPinEnv(t)
	repo := writePinScript(t)
	t.Setenv(DocumentEnv, "local-test")
	t.Setenv(SHA256Env, strings.Repeat("ab", 32))
	pin, err := ReadPin(repo)
	if err != nil {
		t.Fatal(err)
	}
	if pin.Document != "local-test" || pin.SHA256 != strings.Repeat("ab", 32) {
		t.Fatalf("overrides not applied: %+v", pin)
	}
	if pin.Version != "9.9" || pin.URL != "https://example.invalid/suite.xmi" {
		t.Fatalf("unset variables should keep the defaults: %+v", pin)
	}
}

func TestReadPinRejectsAMalformedChecksumOverride(t *testing.T) {
	clearPinEnv(t)
	repo := writePinScript(t)
	t.Setenv(SHA256Env, "not-a-digest")
	if _, err := ReadPin(repo); err == nil || !strings.Contains(err.Error(), SHA256Env) {
		t.Fatalf("err = %v, want one naming %s", err, SHA256Env)
	}
}

func TestVerifyMatchesTheOverriddenChecksum(t *testing.T) {
	clearPinEnv(t)
	repo := writePinScript(t)
	suite := filepath.Join(repo, "suite.xmi")
	if err := os.WriteFile(suite, []byte("<xmi/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := Digest(suite)
	if err != nil {
		t.Fatal(err)
	}
	if pin, err := ReadPin(repo); err != nil || pin.Verify(suite) == nil {
		t.Fatalf("the default pin should reject the file: pin err %v", err)
	}
	t.Setenv(SHA256Env, digest)
	pin, err := ReadPin(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := pin.Verify(suite); err != nil {
		t.Fatal(err)
	}
}
