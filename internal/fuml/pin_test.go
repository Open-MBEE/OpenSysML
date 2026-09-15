package fuml

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPinScript = `#!/usr/bin/env bash
FUML_RI_TAG="${FUML_RI_TAG:-v0.0.0}"
FUML_RI_COMMIT="${FUML_RI_COMMIT:-0123456789abcdef0123456789abcdef01234567}"
FUML_TESTS_FILE="fUML-Tests.uml"
FUML_TESTS_SHA256="${FUML_TESTS_SHA256:-1111111111111111111111111111111111111111111111111111111111111111}"
FUML_EXCEPTION_TESTS_SHA256="${FUML_EXCEPTION_TESTS_SHA256:-2222222222222222222222222222222222222222222222222222222222222222}"
FUML_LIBRARY_SHA256="${FUML_LIBRARY_SHA256:-3333333333333333333333333333333333333333333333333333333333333333}"
FUML_JAR_SHA256="${FUML_JAR_SHA256:-4444444444444444444444444444444444444444444444444444444444444444}"
`

func writePinScript(t *testing.T, script string) string {
	t.Helper()
	repo := t.TempDir()
	path := filepath.Join(repo, filepath.FromSlash(PinPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestReadPinTakesTheScriptsDefaults(t *testing.T) {
	clearPinEnv(t)
	pin, err := ReadPin(writePinScript(t, testPinScript))
	if err != nil {
		t.Fatal(err)
	}
	want := Pin{
		Tag: "v0.0.0", Commit: "0123456789abcdef0123456789abcdef01234567",
		Tests: strings.Repeat("1", 64), ExceptionTest: strings.Repeat("2", 64),
		Library: strings.Repeat("3", 64), Jar: strings.Repeat("4", 64),
	}
	if pin != want {
		t.Fatalf("pin = %+v, want %+v", pin, want)
	}
}

func TestReadPinHonorsTheDownloadersOverrides(t *testing.T) {
	clearPinEnv(t)
	repo := writePinScript(t, testPinScript)
	t.Setenv(TagEnv, "local")
	t.Setenv(JarSHA256Env, strings.Repeat("ab", 32))
	pin, err := ReadPin(repo)
	if err != nil {
		t.Fatal(err)
	}
	if pin.Tag != "local" || pin.Jar != strings.Repeat("ab", 32) {
		t.Fatalf("overrides not applied: %+v", pin)
	}
	if pin.Tests != strings.Repeat("1", 64) {
		t.Fatalf("unset variables should keep the defaults: %+v", pin)
	}
}

func TestReadPinRejectsMalformedOverrides(t *testing.T) {
	clearPinEnv(t)
	repo := writePinScript(t, testPinScript)
	t.Setenv(LibrarySHA256Env, "not-a-digest")
	if _, err := ReadPin(repo); err == nil || !strings.Contains(err.Error(), LibrarySHA256Env) {
		t.Fatalf("err = %v, want one naming %s", err, LibrarySHA256Env)
	}
	t.Setenv(LibrarySHA256Env, "")
	t.Setenv(CommitEnv, "v1.5.0a")
	if _, err := ReadPin(repo); err == nil || !strings.Contains(err.Error(), CommitEnv) {
		t.Fatalf("err = %v, want one naming %s", err, CommitEnv)
	}
}

func TestReadPinRejectsAScriptMissingAVariable(t *testing.T) {
	clearPinEnv(t)
	repo := writePinScript(t, strings.Replace(testPinScript, "FUML_JAR_SHA256=", "OTHER=", 1))
	if _, err := ReadPin(repo); err == nil || !strings.Contains(err.Error(), JarSHA256Env) {
		t.Fatalf("err = %v, want one naming %s", err, JarSHA256Env)
	}
	if _, err := ReadPin(t.TempDir()); err == nil || !strings.Contains(err.Error(), PinPath) {
		t.Fatalf("err = %v, want one naming %s", err, PinPath)
	}
}

func TestVerifyChecksEachSuiteFileAgainstItsOwnDigest(t *testing.T) {
	clearPinEnv(t)
	dir := t.TempDir()
	content := []byte("<xmi/>")
	if err := os.WriteFile(filepath.Join(dir, TestsFile), content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	t.Setenv(TestsSHA256Env, hex.EncodeToString(sum[:]))
	pin, err := ReadPin(writePinScript(t, testPinScript))
	if err != nil {
		t.Fatal(err)
	}
	if err := pin.Verify(dir, TestsFile); err != nil {
		t.Fatalf("Verify(%s) = %v, want nil", TestsFile, err)
	}
	if err := os.WriteFile(filepath.Join(dir, JarFile), content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := pin.Verify(dir, JarFile); err == nil || !strings.Contains(err.Error(), "download-fuml-suite.sh") {
		t.Fatalf("Verify(%s) = %v, want a checksum mismatch with the provisioning hint", JarFile, err)
	}
	if err := pin.Verify(dir, ExceptionTestsFile); err == nil {
		t.Fatal("Verify of an absent file should fail")
	}
	if err := pin.Verify(dir, "other.xmi"); err == nil || !strings.Contains(err.Error(), "not a pinned") {
		t.Fatalf("Verify(other.xmi) = %v, want a not-pinned error", err)
	}
}

// The committed pin must be the one the downloader actually installed, when it has.
func TestCommittedPinMatchesADownloadedSuite(t *testing.T) {
	clearPinEnv(t)
	pin, err := ReadPin(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := Locate(repoRoot)
	if errors.Is(err, ErrSuiteAbsent) && !Required() {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{TestsFile, ExceptionTestsFile, LibraryFile, JarFile} {
		if err := pin.Verify(dir, name); err != nil {
			t.Error(err)
		}
	}
}

func TestLocateReportsAbsenceAndRequirement(t *testing.T) {
	root := t.TempDir()
	if _, err := Locate(root); !errors.Is(err, ErrSuiteAbsent) {
		t.Fatalf("Locate(empty) = %v, want ErrSuiteAbsent", err)
	}
	dir := filepath.Join(root, filepath.FromSlash(SuiteDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{TestsFile, ExceptionTestsFile, LibraryFile} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Locate(root); !errors.Is(err, ErrSuiteAbsent) || !strings.Contains(err.Error(), JarFile) {
		t.Fatalf("Locate(no jar) = %v, want ErrSuiteAbsent naming %s", err, JarFile)
	}
	if err := os.WriteFile(filepath.Join(dir, JarFile), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := Locate(root); err != nil || got != dir {
		t.Fatalf("Locate(full) = %q, %v; want %q", got, err, dir)
	}
	t.Setenv(RequireEnv, "")
	if Required() {
		t.Error("Required() with the variable unset")
	}
	t.Setenv(RequireEnv, "1")
	if !Required() {
		t.Error("Required() with the variable set")
	}
}
