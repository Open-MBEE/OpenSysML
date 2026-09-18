package diff

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// pilotLine matches the GNU-format diagnostics the validators print on stderr.
var pilotLine = regexp.MustCompile(`^(.+\.(?:sysml|kerml)):(\d+):(\d+): (error|warning|info|ignore): (.*)$`)

// pilotVersion reports which pilot release the validator was built against,
// read from the pin its provisioning script wrote beside it.
func pilotVersion(validator string) (string, error) {
	pin, err := os.ReadFile(filepath.Join(filepath.Dir(validator), "pilot-pin.txt"))
	if err != nil {
		return "", fmt.Errorf("read the validator's pilot-pin.txt: %w", err)
	}
	tag := pinnedValue(string(pin), "sysml.release.tag")
	version := pinnedValue(string(pin), "sysml.artifact.version")
	if tag == "" || version == "" {
		return "", fmt.Errorf("%s does not pin sysml.release.tag/sysml.artifact.version", validator)
	}
	return fmt.Sprintf("%s (jupyter-sysml-kernel %s)", tag, version), nil
}

func pinnedValue(pin, name string) string {
	for _, line := range strings.Split(pin, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(key) == name {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// pilotDiagnostics runs a reference validator over one corpus root as a single
// batch and returns its diagnostics per file.
//
// Both validators load every input into one resource set before validating any
// of it, and report each diagnostic under its path relative to --root, so the
// batch needs neither an import ordering nor unique base names.
func pilotDiagnostics(validator, repo, dir string, files []string, timeout time.Duration, log io.Writer) (map[string][]diagnostic, error) {
	root, err := filepath.Abs(filepath.Join(repo, dir))
	if err != nil {
		return nil, fmt.Errorf("resolve corpus root: %w", err)
	}

	byPath := make(map[string]string, len(files))
	args := []string{"--root", root}
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

	cmd := exec.CommandContext(ctx, validator, args...)
	cmd.Stdout = nil // "Reading <library file>..." progress noise
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
		match := pilotLine.FindStringSubmatch(line)
		if match == nil {
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "log4j:") {
				unattributed = append(unattributed, line)
			}
			continue
		}
		rel, ok := byPath[match[1]]
		if !ok {
			unattributed = append(unattributed, line)
			continue
		}
		lineNo, err := strconv.Atoi(match[2])
		if err != nil {
			return fmt.Errorf("pilot reported a non-numeric line in %q", line)
		}
		out[rel] = append(out[rel], diagnostic{
			File:     rel,
			Line:     lineNo,
			Severity: match[4],
			Category: categorize(match[5]),
			Message:  match[5],
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
