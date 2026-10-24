package formal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
)

// Extract reads the constraints of the pinned document at path: the file must
// have the digest the manifest pins, and its text is read with pdftotext.
func Extract(pdftotext, path, key string, spec manifest.Specification) ([]Constraint, error) {
	if err := verifyDigest(path, spec.SHA256); err != nil {
		return nil, fmt.Errorf("%s: %w", spec.Document, err)
	}
	text, err := textOf(pdftotext, path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.Document, err)
	}
	constraints, err := Parse(text)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.Document, err)
	}
	for i := range constraints {
		constraints[i].Document = key
	}
	return constraints, nil
}

func verifyDigest(path, want string) error {
	got, err := digestOf(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%s has SHA-256 %s, the manifest pins %s", path, got, want)
	}
	return nil
}

// digestOf is the hex SHA-256 of a file.
func digestOf(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- the operator names the PDF
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func textOf(pdftotext, path string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(pdftotext, "-layout", "-enc", "UTF-8", path, "-") // #nosec G204 -- the operator names the program
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s -layout %s: %w\n%s", pdftotext, path, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
