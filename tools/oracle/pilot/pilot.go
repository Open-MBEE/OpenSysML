// Package pilot reads what the pilot reference validators print and which
// release they were built against, for every referee that runs one.
package pilot

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// diagnosticLine matches the GNU-format diagnostics the validators print on stderr.
var diagnosticLine = regexp.MustCompile(`^(.+\.(?:sysml|kerml)):(\d+):(\d+): (error|warning|info|ignore): (.*)$`)

// Diagnostic is one diagnostic a validator printed.
type Diagnostic struct {
	Path     string // as printed: relative to the validator's --root
	Line     int
	Column   int
	Severity string // error, warning, info or ignore
	Message  string
}

// ParseDiagnostic reads one line of a validator's stderr, reporting false for
// a line that is not a diagnostic.
func ParseDiagnostic(line string) (Diagnostic, bool) {
	match := diagnosticLine.FindStringSubmatch(line)
	if match == nil {
		return Diagnostic{}, false
	}
	lineNo, err := strconv.Atoi(match[2])
	if err != nil {
		return Diagnostic{}, false
	}
	column, err := strconv.Atoi(match[3])
	if err != nil {
		return Diagnostic{}, false
	}
	return Diagnostic{Path: match[1], Line: lineNo, Column: column, Severity: match[4], Message: match[5]}, true
}

// Noise reports whether a line that is not a diagnostic says nothing about the
// input: a blank line or the logging framework's own notice.
func Noise(line string) bool {
	return strings.TrimSpace(line) == "" || strings.HasPrefix(line, "log4j:")
}

// Version reports which pilot release the validator was built against, read
// from the pin its provisioning script wrote beside it.
func Version(validator string) (string, error) {
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
