package layout

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// EnvVar names the dot executable to lay out with, ahead of the one on PATH.
const EnvVar = "OPENSYSML_DOT"

// Tool is the Graphviz executable the layered layout runs.
const Tool = "dot"

// ErrUnavailable is why no layout runs on a host that starts no process: every
// Go WebAssembly target.
var ErrUnavailable = errors.New("a WebAssembly build cannot start external processes")

// NotFoundError is a dot that is nowhere to be found: not at the override,
// when one is set, else not on PATH.
type NotFoundError struct {
	// Override is what EnvVar named, empty when it names nothing.
	Override string
}

func (e *NotFoundError) Error() string {
	if e.Override != "" {
		return fmt.Sprintf("no Graphviz %s at %s=%s", Tool, EnvVar, e.Override)
	}
	return fmt.Sprintf("no Graphviz %s on PATH; install Graphviz or set %s", Tool, EnvVar)
}

// Locate is the absolute path of the dot to run: the one EnvVar names, else
// the one on PATH. On a host that starts no process the error is ErrUnavailable.
func Locate() (string, error) {
	if err := checkSpawn(); err != nil {
		return "", err
	}
	override := strings.TrimSpace(os.Getenv(EnvVar))
	subject := override
	if subject == "" {
		subject = Tool
	}
	path, err := exec.LookPath(subject)
	if err != nil {
		return "", &NotFoundError{Override: override}
	}
	return filepath.Abs(path)
}

var versionLine = regexp.MustCompile(`graphviz version (\S+)`)

// Version is the Graphviz version the dot at path reports with -V, "16.1.0" say.
func Version(path string) (string, error) {
	out, err := exec.Command(path, "-V").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s -V: %w: %s", path, err, strings.TrimSpace(string(out)))
	}
	m := versionLine.FindSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("%s -V answered %q, not a graphviz version", path, strings.TrimSpace(string(out)))
	}
	return string(m[1]), nil
}
