package diff

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/pilot"
)

// pilotDiagnostics runs a reference validator over one corpus root as a single
// batch and returns its diagnostics per file. libraries, when not empty, is a
// library directory the validator loads beside the standard library, resolved
// against but never validated, so its files report nothing of their own.
//
// Both validators load every input into one resource set before validating any
// of it, and report each diagnostic under its path relative to --root, so the
// batch needs neither an import ordering nor unique base names.
func pilotDiagnostics(validator, libraries, repo, dir string, files []string, timeout time.Duration, log io.Writer) (map[string][]diagnostic, error) {
	root, err := filepath.Abs(filepath.Join(repo, dir))
	if err != nil {
		return nil, fmt.Errorf("resolve corpus root: %w", err)
	}

	byPath := make(map[string]string, len(files))
	var args []string
	if libraries != "" {
		args = append(args, "--extension-library", libraries)
	}
	args = append(args, "--root", root)
	for _, rel := range files {
		byPath[rel] = rel
		args = append(args, filepath.Join(root, filepath.FromSlash(rel)))
	}

	out := make(map[string][]diagnostic, len(files))
	if err := runPilot(validator, args, byPath, out, timeout, categorizePilot, log); err != nil {
		return nil, err
	}
	return out, nil
}

// runPilot reads GNU-format diagnostics from a validator's stderr. categorize
// is the mapping for that validator's diagnostic vocabulary; log gets the
// lines that name no corpus file.
func runPilot(validator string, args []string, byPath map[string]string, out map[string][]diagnostic, timeout time.Duration, categorize func(string) Category, log io.Writer) error {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, validator, args...) // #nosec G204 -- the pinned pilot validator
	cmd.Stdout = nil                                    // "Reading <library file>..." progress noise
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", validator, err)
	}

	var unattributed []string
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		found, ok := pilot.ParseDiagnostic(line)
		if !ok {
			if !pilot.Noise(line) {
				unattributed = append(unattributed, line)
			}
			continue
		}
		rel, ok := byPath[found.Path]
		if !ok {
			unattributed = append(unattributed, line)
			continue
		}
		out[rel] = append(out[rel], diagnostic{
			File:     rel,
			Line:     found.Line,
			Severity: found.Severity,
			Category: categorize(found.Message),
			Message:  found.Message,
		})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read the validator's output: %w", err)
	}

	// Exit 1 only means the batch had errors; anything else is the validator
	// itself failing, and its diagnostics cannot be trusted.
	err = cmd.Wait()
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("%s failed (%w); stderr:\n%s", validator, err, strings.Join(unattributed, "\n"))
	}
	for _, line := range unattributed {
		// Never dropped silently: an unattributable diagnostic would otherwise
		// look like agreement.
		fmt.Fprintf(log, "pilot output not attributable to a corpus file: %s\n", line)
	}
	return nil
}
