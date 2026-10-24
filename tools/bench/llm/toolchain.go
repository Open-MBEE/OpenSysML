package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Diagnostic is one entry of `sysml -validate -json`.
type Diagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Pass     string `json:"pass,omitempty"`
	Code     string `json:"code,omitempty"`
}

// UnresolvedCode is the diagnostic code the resolver gives a name that exists
// neither in the model nor in the library.
const UnresolvedCode = "unresolved"

// Toolchain runs the OpenSysML compiler and, when provisioned, the pinned
// pilot validator.
type Toolchain struct {
	// Sysml is the sysml binary.
	Sysml string
	// Pilot is the validate-sysml-batch script built by
	// scripts/download-pilot-sysml-validator.sh; empty disables the pilot oracle.
	Pilot string
	// PilotLibrary is the extension-library directory handed to the pilot.
	PilotLibrary string
}

// ValidateResult is the compiler's verdict on one document.
type ValidateResult struct {
	OK          bool         `json:"ok"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Errors returns the error-severity diagnostics.
func (r ValidateResult) Errors() []Diagnostic {
	var out []Diagnostic
	for _, d := range r.Diagnostics {
		if d.Severity == "error" {
			out = append(out, d)
		}
	}
	return out
}

// Unresolved returns the distinct names the resolver could not find.
func (r ValidateResult) Unresolved() []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range r.Diagnostics {
		if d.Code != UnresolvedCode {
			continue
		}
		name := unresolvedName(d.Message)
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

func unresolvedName(msg string) string {
	const prefix = "unresolved reference: "
	i := strings.Index(msg, prefix)
	if i < 0 {
		return ""
	}
	rest := msg[i+len(prefix):]
	if j := strings.IndexAny(rest, " —\n"); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}

// Validate writes the document to a scratch file named as the model's package
// and runs `sysml -validate -json` over it.
func (tc Toolchain) Validate(ctx context.Context, source string) (ValidateResult, error) {
	path, cleanup, err := scratch(source)
	if err != nil {
		return ValidateResult{}, err
	}
	defer cleanup()
	cmd := exec.CommandContext(ctx, tc.Sysml, "-validate", "-json", path) // #nosec G204 -- the binary is the caller's toolchain
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	var report struct {
		Status      string       `json:"status"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		var exitErr *exec.ExitError
		if runErr != nil && !errors.As(runErr, &exitErr) {
			return ValidateResult{}, fmt.Errorf("run %s: %w", tc.Sysml, runErr)
		}
		return ValidateResult{}, fmt.Errorf("sysml -validate -json: unreadable report: %w\n%s%s", err, stdout.String(), stderr.String())
	}
	res := ValidateResult{Diagnostics: report.Diagnostics}
	for i := range res.Diagnostics {
		res.Diagnostics[i].File = filepath.Base(res.Diagnostics[i].File)
	}
	res.OK = report.Status == "ok" || (runErr == nil && len(res.Errors()) == 0)
	return res, nil
}

// Export runs `sysml -convert api-json` over the document.
func (tc Toolchain) Export(ctx context.Context, source string) ([]byte, error) {
	path, cleanup, err := scratch(source)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	cmd := exec.CommandContext(ctx, tc.Sysml, "-convert", "api-json", path) // #nosec G204 -- the binary is the caller's toolchain
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("sysml -convert api-json: %w\n%s", err, stderr.String())
	}
	return out, nil
}

// PilotAccepts runs the pinned pilot validator over the document. The second
// value is the pilot's diagnostic output.
func (tc Toolchain) PilotAccepts(ctx context.Context, source string) (bool, string, error) {
	if tc.Pilot == "" {
		return false, "", errors.New("pilot validator not configured")
	}
	path, cleanup, err := scratch(source)
	if err != nil {
		return false, "", err
	}
	defer cleanup()
	args := []string{}
	if tc.PilotLibrary != "" {
		args = append(args, "--extension-library", tc.PilotLibrary)
	}
	args = append(args, "--root", filepath.Dir(path), path)
	cmd := exec.CommandContext(ctx, tc.Pilot, args...) // #nosec G204 -- the script is the caller's toolchain
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	text := pilotDiagnostics(stderr.String())
	if runErr == nil {
		return true, text, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() == 1 {
		return false, text, nil
	}
	return false, text, fmt.Errorf("pilot validator: %w\n%s%s", runErr, stdout.String(), stderr.String())
}

func pilotDiagnostics(stderr string) string {
	var keep []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, ": error:") || strings.Contains(line, ": warning:") {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}

// scratch writes a document to its own directory under a stable file name so
// that diagnostics and the pilot's --root stay comparable across runs.
func scratch(source string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "llm-bench-")
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	return path, func() { _ = os.RemoveAll(dir) }, nil
}
