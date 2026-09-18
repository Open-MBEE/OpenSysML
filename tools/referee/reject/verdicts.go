package reject

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
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

// openSysMLErrors validates each case in its own workspace — the cases are
// independent single files — under the conformance mode modes names for it, and
// returns our error-severity messages per file.
func openSysMLErrors(repo, dir string, files []string,
	modes map[string]conformance.Mode) (map[string][]string, error) {
	out := make(map[string][]string, len(files))
	for _, rel := range files {
		// #nosec G304 -- the corpus directory is named on the command line.
		content, err := os.ReadFile(filepath.Join(repo, dir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		ws := model.NewWorkspace(model.WithConformanceMode(modes[rel]))
		ws.Open(rel, content, 1)
		lines := source.New(rel, content).Lines()
		for _, d := range ws.Diagnostics(rel) {
			if d.Severity.String() != "error" {
				continue
			}
			out[rel] = append(out[rel], fmt.Sprintf("line %d: %s", lines.PosAt(d.Span.Offset).Line, d.Message))
		}
	}
	return out, nil
}

// pilotErrors runs a reference validator over the corpus as a single batch and
// returns its error-severity messages per file. Both validators report each
// diagnostic under its path relative to --root.
func pilotErrors(validator, repo, dir string, files []string, timeout time.Duration, log io.Writer) (map[string][]string, error) {
	root, err := filepath.Abs(filepath.Join(repo, dir))
	if err != nil {
		return nil, fmt.Errorf("resolve corpus root: %w", err)
	}

	byPath := make(map[string]bool, len(files))
	args := []string{"--root", root}
	for _, rel := range files {
		byPath[rel] = true
		args = append(args, filepath.Join(root, filepath.FromSlash(rel)))
	}

	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// #nosec G204 -- the validator to run is named on the command line.
	cmd := exec.CommandContext(ctx, validator, args...)
	cmd.Stdout = nil // "Reading <library file>..." progress noise
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", validator, err)
	}

	out := make(map[string][]string, len(files))
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
		if !byPath[match[1]] {
			unattributed = append(unattributed, line)
			continue
		}
		if match[4] != "error" {
			continue
		}
		out[match[1]] = append(out[match[1]], fmt.Sprintf("line %s: %s", match[2], match[5]))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read the validator's output: %w", err)
	}

	// Exit 1 only means the batch had errors; anything else is the validator
	// itself failing, and its verdicts cannot be trusted.
	err = cmd.Wait()
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s failed (%w); stderr:\n%s", validator, err, strings.Join(unattributed, "\n"))
	}
	for _, line := range unattributed {
		// Never dropped silently: an unattributable error would otherwise
		// look like an acceptance.
		fmt.Fprintf(log, "pilot output not attributable to a corpus case: %s\n", line)
	}
	return out, nil
}
