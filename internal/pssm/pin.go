package pssm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// PinPath is the script every fetch of the suite sources its pin from.
const PinPath = "scripts/pssm-pin.sh"

// Pin is the suite's identity as scripts/pssm-pin.sh records it.
type Pin struct {
	Document string
	Version  string
	URL      string
	SHA256   string
}

var (
	pinDocumentRe = regexp.MustCompile(`PSSM_DOCUMENT="\$\{PSSM_DOCUMENT:-([^}"]+)\}"`)
	pinVersionRe  = regexp.MustCompile(`PSSM_VERSION="\$\{PSSM_VERSION:-([^}"]+)\}"`)
	pinURLRe      = regexp.MustCompile(`PSSM_SUITE_URL="\$\{PSSM_SUITE_URL:-([^}"]+)\}"`)
	pinSHA256Re   = regexp.MustCompile(`PSSM_SUITE_SHA256="\$\{PSSM_SUITE_SHA256:-([0-9a-f]{64})\}"`)
)

// ReadPin resolves the pin from scripts/pssm-pin.sh under the repository root.
func ReadPin(repo string) (Pin, error) {
	path := filepath.Join(repo, filepath.FromSlash(PinPath))
	content, err := os.ReadFile(path) // #nosec G304 -- the pin is at a fixed path in this repository
	if err != nil {
		return Pin{}, fmt.Errorf("read %s: %w", PinPath, err)
	}
	doc := pinDocumentRe.FindSubmatch(content)
	version := pinVersionRe.FindSubmatch(content)
	url := pinURLRe.FindSubmatch(content)
	sum := pinSHA256Re.FindSubmatch(content)
	if doc == nil || version == nil || url == nil || sum == nil {
		return Pin{}, fmt.Errorf("%s pins no PSSM_DOCUMENT/PSSM_VERSION/PSSM_SUITE_URL/PSSM_SUITE_SHA256", PinPath)
	}
	return Pin{Document: string(doc[1]), Version: string(version[1]), URL: string(url[1]), SHA256: string(sum[1])}, nil
}

// Digest is the sha256 of the suite file at path, as the pin spells it.
func Digest(path string) (string, error) {
	content, err := os.ReadFile(path) // #nosec G304 -- callers name the located suite file
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

// Verify refuses a suite file whose bytes are not the pinned ones, before
// anything reads it.
func (p Pin) Verify(suitePath string) error {
	digest, err := Digest(suitePath)
	if err != nil {
		return err
	}
	if digest != p.SHA256 {
		return fmt.Errorf("%s has sha256 %s, not the pinned %s; re-run ./scripts/download-pssm-suite.sh", suitePath, digest, p.SHA256)
	}
	return nil
}

// Provenance ties a report to the pin, which Verify has matched to the bytes read.
func (p Pin) Provenance(tests int) Provenance {
	return Provenance{Document: p.Document, Version: p.Version, URL: p.URL, Digest: p.SHA256, Tests: tests}
}
