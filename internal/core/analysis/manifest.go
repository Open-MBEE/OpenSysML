package analysis

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// ToolsEnv names the environment variable holding the tool manifest: a directory with one
// JSON entry per external tool.
const ToolsEnv = "OPENSYSML_TOOLS"

// ToolTimeoutEnv names the environment variable bounding one tool invocation.
const ToolTimeoutEnv = "OPENSYSML_TOOL_TIMEOUT"

// DefaultToolTimeout is how long a tool is given to answer one invocation before the
// performance fails with a timeout: the solver's default.
const DefaultToolTimeout = solve.DefaultTimeout

// ManifestExt is the extension of a manifest entry; other files in the directory are ignored.
const ManifestExt = ".json"

// ToolEntry is one entry of the tool manifest: the tool a ToolExecution names, its version,
// the executable that answers for it, and the tool-variable names it accepts.
type ToolEntry struct {
	// File is the manifest entry the tool was read from; empty for one built in code.
	File string `json:"-"`
	// ToolName is what a ToolExecution's toolName names.
	ToolName string `json:"toolName"`
	// Version is the tool's, reported beside its executable.
	Version string `json:"version,omitempty"`
	// Executable runs the tool: an absolute path, a path relative to the manifest directory,
	// or a name looked up on PATH.
	Executable string `json:"executable"`
	// Variables are the ToolVariable names the tool accepts.
	Variables []string `json:"variables"`
}

// Accepts reports whether the tool accepts the tool variable named.
func (e ToolEntry) Accepts(variable string) bool {
	for _, v := range e.Variables {
		if v == variable {
			return true
		}
	}
	return false
}

// ErrManifest is the typed error every fault in the tool manifest unwraps to.
var ErrManifest = errors.New("tool manifest is malformed")

// ManifestError reports one fault in the tool manifest: the directory or entry it is in
// and what is wrong with it.
type ManifestError struct {
	// Path is the directory or the entry file at fault.
	Path string
	// Detail is what is wrong.
	Detail string
	// Err is the fault as the file system or the decoder reported it, when one did.
	Err error
}

// Error names the path and the fault.
func (e *ManifestError) Error() string {
	text := fmt.Sprintf("%s: %s: %s", ToolsEnv, e.Path, e.Detail)
	if e.Err != nil {
		text += ": " + e.Err.Error()
	}
	return text
}

// Is matches ErrManifest.
func (e *ManifestError) Is(target error) bool { return target == ErrManifest }

// Unwrap returns the underlying report.
func (e *ManifestError) Unwrap() error { return e.Err }

// LoadManifest reads the tool manifest in dir: every `.json` file is one entry. A directory
// that cannot be read, an entry that is not one JSON object of the manifest's fields, an
// entry without a toolName, executable or variables, one listing a variable twice, or two
// entries naming one tool is a ManifestError. Entries come back in tool-name order.
func LoadManifest(dir string) ([]ToolEntry, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, &ManifestError{Path: dir, Detail: "cannot read the manifest directory", Err: err}
	}
	byName := make(map[string]string)
	var entries []ToolEntry
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ManifestExt {
			continue
		}
		path := filepath.Join(dir, file.Name())
		entry, err := readEntry(path)
		if err != nil {
			return nil, err
		}
		if other, dup := byName[entry.ToolName]; dup {
			return nil, &ManifestError{Path: path, Detail: fmt.Sprintf("tool %q is also the entry %s", entry.ToolName, other)}
		}
		byName[entry.ToolName] = path
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ToolName < entries[j].ToolName })
	return entries, nil
}

// readEntry reads one manifest entry, resolving a relative executable against the entry's directory.
func readEntry(path string) (ToolEntry, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- an entry of the manifest directory OPENSYSML_TOOLS names
	if err != nil {
		return ToolEntry{}, &ManifestError{Path: path, Detail: "cannot read the entry", Err: err}
	}
	var entry ToolEntry
	if err := decodeOne(data, &entry); err != nil {
		return ToolEntry{}, &ManifestError{Path: path, Detail: "not one JSON object of toolName, version, executable and variables", Err: err}
	}
	entry.File = path
	entry.ToolName = strings.TrimSpace(entry.ToolName)
	entry.Version = strings.TrimSpace(entry.Version)
	entry.Executable = strings.TrimSpace(entry.Executable)
	switch {
	case entry.ToolName == "":
		return ToolEntry{}, &ManifestError{Path: path, Detail: "toolName is empty"}
	case strings.ContainsAny(entry.ToolName, " \t\r\n"):
		return ToolEntry{}, &ManifestError{Path: path, Detail: fmt.Sprintf("toolName %q has whitespace in it", entry.ToolName)}
	case entry.Executable == "":
		return ToolEntry{}, &ManifestError{Path: path, Detail: "executable is empty"}
	case entry.Variables == nil:
		return ToolEntry{}, &ManifestError{Path: path, Detail: "variables is missing"}
	}
	seen := make(map[string]bool, len(entry.Variables))
	for _, v := range entry.Variables {
		switch {
		case strings.TrimSpace(v) == "":
			return ToolEntry{}, &ManifestError{Path: path, Detail: "variables has an empty name"}
		case seen[v]:
			return ToolEntry{}, &ManifestError{Path: path, Detail: fmt.Sprintf("variables lists %q twice", v)}
		}
		seen[v] = true
	}
	if strings.ContainsAny(entry.Executable, `/\`) && !filepath.IsAbs(entry.Executable) {
		entry.Executable = filepath.Join(filepath.Dir(path), entry.Executable)
	}
	return entry, nil
}

// decodeOne decodes data as exactly one JSON object into v, refusing fields v does not
// declare and anything but whitespace after the object.
func decodeOne(data []byte, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		return errors.New("more than one JSON value")
	}
	return nil
}

// ToolsFromEnv reads the manifest OPENSYSML_TOOLS names as engines, one per entry; none
// when it is unset. A manifest that cannot be read is a ManifestError.
func ToolsFromEnv() ([]External, error) {
	dir := strings.TrimSpace(os.Getenv(ToolsEnv))
	if dir == "" {
		return nil, nil
	}
	entries, err := LoadManifest(dir)
	if err != nil {
		return nil, err
	}
	tools := make([]External, len(entries))
	for i, entry := range entries {
		tools[i] = NewTool(entry)
	}
	return tools, nil
}

// lookExecutable finds an entry's executable: a path as given, a bare name on PATH.
func lookExecutable(entry ToolEntry) (string, error) {
	path, err := exec.LookPath(entry.Executable)
	if err != nil {
		return "", &ToolAbsentError{Tool: entry.ToolName, Executable: entry.Executable, Err: err}
	}
	return path, nil
}

// toolTimeoutFromEnv reads the tool timeout, falling back to DefaultToolTimeout for an
// unset, unparsable or non-positive value.
func toolTimeoutFromEnv() time.Duration {
	text := strings.TrimSpace(os.Getenv(ToolTimeoutEnv))
	if text == "" {
		return DefaultToolTimeout
	}
	d, err := time.ParseDuration(text)
	if err != nil || d <= 0 {
		return DefaultToolTimeout
	}
	return d
}

// ErrToolAbsent is the typed error for a manifest entry whose executable is not found.
var ErrToolAbsent = errors.New("tool's executable is absent")

// ToolAbsentError reports a registered tool whose executable is not found.
type ToolAbsentError struct {
	Tool       string
	Executable string
	Err        error
}

// Error names the tool and its executable.
func (e *ToolAbsentError) Error() string {
	return fmt.Sprintf("tool '%s': executable %s not found", e.Tool, e.Executable)
}

// Is matches ErrToolAbsent.
func (e *ToolAbsentError) Is(target error) bool { return target == ErrToolAbsent }

// Unwrap returns the lookup's report.
func (e *ToolAbsentError) Unwrap() error { return e.Err }
