// Package fuml referees OpenSysML's action execution against the fUML Reference
// Implementation's activity tests and the record of what that implementation computed.
package fuml

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// PinPath is the script every fetch of the suite sources its pin from.
const PinPath = "scripts/fuml-pin.sh"

// SuiteDir is where scripts/download-fuml-suite.sh places the suite, relative
// to the repository root.
const SuiteDir = "build/fuml"

// The suite's file names, as the downloader installs them.
const (
	TestsFile          = "fUML-Tests.uml"
	ExceptionTestsFile = "fUML-Exception-Tests.uml"
	LibraryFile        = "fUML_Library.xmi"
	JarFile            = "fuml-1.5.0a.jar"
)

// RequireEnv names the environment variable a gate sets so an absent suite fails it instead of skipping.
const RequireEnv = "OPENSYSML_REQUIRE_FUML_SUITE"

// ErrSuiteAbsent reports that no suite is installed at the directory looked in.
var ErrSuiteAbsent = errors.New("fUML test suite is absent")

// Locate returns the suite directory under root, or an error wrapping ErrSuiteAbsent.
func Locate(root string) (string, error) {
	dir := filepath.Join(root, filepath.FromSlash(SuiteDir))
	for _, name := range []string{TestsFile, ExceptionTestsFile, LibraryFile, JarFile} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("%w: no %s at %s; run ./scripts/download-fuml-suite.sh to provision it", ErrSuiteAbsent, name, dir)
			}
			return "", err
		}
	}
	return dir, nil
}

// Required reports whether the environment demands the suite be present.
func Required() bool { return os.Getenv(RequireEnv) != "" }

// Pin is the suite's identity as scripts/fuml-pin.sh records it.
type Pin struct {
	Tag           string
	Commit        string
	Tests         string // sha256 of fUML-Tests.uml
	ExceptionTest string // sha256 of fUML-Exception-Tests.uml
	Library       string // sha256 of fUML_Library.xmi
	Jar           string // sha256 of fuml-1.5.0a.jar
	// The namespace URIs the two test models are loaded under.
	TestsURI, ExceptionTestsURI string
}

// The script's variables; an environment value overrides each for the downloader,
// so it overrides here too, or the two would fetch and verify different pins.
const (
	TagEnv           = "FUML_RI_TAG"
	CommitEnv        = "FUML_RI_COMMIT"
	TestsSHA256Env   = "FUML_TESTS_SHA256"
	ExceptionSHA2Env = "FUML_EXCEPTION_TESTS_SHA256"
	LibrarySHA256Env = "FUML_LIBRARY_SHA256"
	JarSHA256Env     = "FUML_JAR_SHA256"
)

// The script's fixed variables: the URIs have no environment override.
const (
	TestsURIVar          = "FUML_TESTS_URI"
	ExceptionTestsURIVar = "FUML_EXCEPTION_TESTS_URI"
)

var (
	sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)
	commitRe = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

func pinDefaultRe(name, value string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(name) + `="\$\{` + regexp.QuoteMeta(name) + `:-(` + value + `)\}"`)
}

func pinFixedRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `="([^"$]+)"$`)
}

// ReadPin resolves the pin as scripts/fuml-pin.sh does: the script's defaults
// under the repository root, each replaced by its environment variable when set.
func ReadPin(repo string) (Pin, error) {
	pin, err := readPinDefaults(repo)
	if err != nil {
		return Pin{}, err
	}
	for _, o := range []struct {
		env   string
		field *string
	}{
		{TagEnv, &pin.Tag}, {CommitEnv, &pin.Commit},
		{TestsSHA256Env, &pin.Tests}, {ExceptionSHA2Env, &pin.ExceptionTest},
		{LibrarySHA256Env, &pin.Library}, {JarSHA256Env, &pin.Jar},
	} {
		if v, ok := os.LookupEnv(o.env); ok && v != "" {
			*o.field = v
		}
	}
	if !commitRe.MatchString(pin.Commit) {
		return Pin{}, fmt.Errorf("%s=%q is not a lowercase hex commit", CommitEnv, pin.Commit)
	}
	for _, d := range []struct{ env, sum string }{
		{TestsSHA256Env, pin.Tests}, {ExceptionSHA2Env, pin.ExceptionTest},
		{LibrarySHA256Env, pin.Library}, {JarSHA256Env, pin.Jar},
	} {
		if !sha256Re.MatchString(d.sum) {
			return Pin{}, fmt.Errorf("%s=%q is not a lowercase hex sha256", d.env, d.sum)
		}
	}
	return pin, nil
}

func readPinDefaults(repo string) (Pin, error) {
	path := filepath.Join(repo, filepath.FromSlash(PinPath))
	content, err := os.ReadFile(path) // #nosec G304 -- the pin is at a fixed path in this repository
	if err != nil {
		return Pin{}, fmt.Errorf("read %s: %w", PinPath, err)
	}
	var pin Pin
	for _, f := range []struct {
		name, value string
		field       *string
	}{
		{TagEnv, `[^}"]+`, &pin.Tag},
		{CommitEnv, `[0-9a-f]{40}`, &pin.Commit},
		{TestsSHA256Env, `[0-9a-f]{64}`, &pin.Tests},
		{ExceptionSHA2Env, `[0-9a-f]{64}`, &pin.ExceptionTest},
		{LibrarySHA256Env, `[0-9a-f]{64}`, &pin.Library},
		{JarSHA256Env, `[0-9a-f]{64}`, &pin.Jar},
	} {
		m := pinDefaultRe(f.name, f.value).FindSubmatch(content)
		if m == nil {
			return Pin{}, fmt.Errorf("%s pins no %s", PinPath, f.name)
		}
		*f.field = string(m[1])
	}
	for _, f := range []struct {
		name  string
		field *string
	}{
		{TestsURIVar, &pin.TestsURI},
		{ExceptionTestsURIVar, &pin.ExceptionTestsURI},
	} {
		m := pinFixedRe(f.name).FindSubmatch(content)
		if m == nil {
			return Pin{}, fmt.Errorf("%s pins no %s", PinPath, f.name)
		}
		*f.field = string(m[1])
	}
	return pin, nil
}

// Digest is the sha256 of the file at path, as the pin spells it.
func Digest(path string) (string, error) {
	content, err := os.ReadFile(path) // #nosec G304 -- callers name the located suite file
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

// Verify refuses a suite file whose bytes are not the pinned ones, before
// anything reads it. Name is one of the *File constants.
func (p Pin) Verify(dir, name string) error {
	want, ok := map[string]string{
		TestsFile: p.Tests, ExceptionTestsFile: p.ExceptionTest, LibraryFile: p.Library, JarFile: p.Jar,
	}[name]
	if !ok {
		return fmt.Errorf("%s is not a pinned suite file", name)
	}
	path := filepath.Join(dir, name)
	digest, err := Digest(path)
	if err != nil {
		return err
	}
	if digest != want {
		return fmt.Errorf("%s has sha256 %s, not the pinned %s; re-run ./scripts/download-fuml-suite.sh", path, digest, want)
	}
	return nil
}
