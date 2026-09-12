package analysis

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// ToolsEnv names the environment variable holding the tool manifest: a directory with one
// JSON entry per external tool.
const ToolsEnv = "OPENSYSML_TOOLS"

// EnginesEnv names the environment variable holding the engine manifest: a directory with
// one JSON entry per external engine or strategy. Either directory may hold any kind.
const EnginesEnv = "OPENSYSML_ENGINES"

// ToolTimeoutEnv names the environment variable bounding one tool invocation and, for an
// external engine, one message round trip and the grace a cancel is given.
const ToolTimeoutEnv = "OPENSYSML_TOOL_TIMEOUT"

// DefaultToolTimeout is how long a tool is given to answer one invocation before the
// performance fails with a timeout: the solver's default.
const DefaultToolTimeout = solve.DefaultTimeout

// ManifestExt is the extension of a manifest entry; other files in the directory are ignored.
const ManifestExt = ".json"

// EntryKind is what a manifest entry registers: a tool the model names, an engine, or a
// strategy a later build serves.
type EntryKind string

const (
	// KindTool is an executable a ToolExecution names; the default when `kind` is absent.
	KindTool EntryKind = "tool"
	// KindEngine is an external analysis engine answering questions over the protocol.
	KindEngine EntryKind = "engine"
	// KindPolicy is an external scheduling policy, not served in this build.
	KindPolicy EntryKind = "policy"
	// KindSampler is an external sweep sampler, not served in this build.
	KindSampler EntryKind = "sampler"
)

// ToolEntry is one entry of the tool manifest: the tool a ToolExecution names, its version,
// the executable that answers for it, and the tool-variable names it accepts.
type ToolEntry struct {
	// File is the manifest entry the tool was read from; empty for one built in code.
	File string `json:"-"`
	// Kind is `tool` or absent.
	Kind EntryKind `json:"kind,omitempty"`
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

// Manifest is what one manifest directory registers, each list in name order.
type Manifest struct {
	// Dir is the directory read.
	Dir string
	// Env names the variable that named Dir.
	Env string
	// Tools are the `tool` entries.
	Tools []ToolEntry
	// Engines are the `engine`, `policy` and `sampler` entries.
	Engines []EngineEntry
}

// ErrManifest is the typed error every fault in a manifest unwraps to.
var ErrManifest = errors.New("tool manifest is malformed")

// ManifestError reports one fault in a manifest: the directory or entry it is in
// and what is wrong with it.
type ManifestError struct {
	// Env names the variable that named the directory; ToolsEnv when empty.
	Env string
	// Path is the directory or the entry file at fault.
	Path string
	// Detail is what is wrong.
	Detail string
	// Err is the fault as the file system or the decoder reported it, when one did.
	Err error
}

// Error names the variable, the path and the fault.
func (e *ManifestError) Error() string {
	env := e.Env
	if env == "" {
		env = ToolsEnv
	}
	text := fmt.Sprintf("%s: %s: %s", env, e.Path, e.Detail)
	if e.Err != nil {
		text += ": " + e.Err.Error()
	}
	return text
}

// Is matches ErrManifest.
func (e *ManifestError) Is(target error) bool { return target == ErrManifest }

// Unwrap returns the underlying report.
func (e *ManifestError) Unwrap() error { return e.Err }

// LoadManifest reads the tool entries of the manifest in dir: every `.json` file is one
// entry. A directory that cannot be read, an entry that is not one JSON object of its kind's
// fields, an entry without a toolName, executable or variables, one listing a variable
// twice, or two entries naming one tool is a ManifestError. Entries come back in tool-name
// order; entries of the other kinds are read and checked but not returned.
func LoadManifest(dir string) ([]ToolEntry, error) {
	m, err := ReadManifest(dir, ToolsEnv)
	if err != nil {
		return nil, err
	}
	return m.Tools, nil
}

// ReadManifest reads the manifest in dir, named by the environment variable env: every
// `.json` file is one entry of the kind its `kind` field says, `tool` when absent. Every
// fault is a ManifestError; a duplicate name among entries of one kind is one too.
func ReadManifest(dir, env string) (*Manifest, error) {
	fault := func(path, detail string, err error) error {
		return &ManifestError{Env: env, Path: path, Detail: detail, Err: err}
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fault(dir, "cannot read the manifest directory", err)
	}
	if !info.IsDir() {
		return nil, fault(dir, "is not a directory", nil)
	}
	if err := ownerWritableOnly(dir, info); err != nil {
		return nil, fault(dir, err.Error(), nil)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fault(dir, "cannot read the manifest directory", err)
	}
	m := &Manifest{Dir: dir, Env: env}
	tools, engines := make(map[string]string), make(map[string]string)
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ManifestExt {
			continue
		}
		path := filepath.Join(dir, file.Name())
		data, kind, err := readEntryFile(path, env)
		if err != nil {
			return nil, err
		}
		switch kind {
		case KindTool:
			entry, err := readToolEntry(path, env, data)
			if err != nil {
				return nil, err
			}
			if other, dup := tools[entry.ToolName]; dup {
				return nil, fault(path, fmt.Sprintf("tool %q is also the entry %s", entry.ToolName, other), nil)
			}
			tools[entry.ToolName] = path
			m.Tools = append(m.Tools, entry)
		case KindEngine, KindPolicy, KindSampler:
			entry, err := readEngineEntry(path, env, kind, data)
			if err != nil {
				return nil, err
			}
			if other, dup := engines[entry.Name]; dup {
				return nil, fault(path, fmt.Sprintf("%s %q is also the entry %s", entry.Kind, entry.Name, other), nil)
			}
			engines[entry.Name] = path
			m.Engines = append(m.Engines, entry)
		default:
			return nil, fault(path, fmt.Sprintf("kind %q is not one of tool, engine, policy and sampler", kind), nil)
		}
	}
	sort.Slice(m.Tools, func(i, j int) bool { return m.Tools[i].ToolName < m.Tools[j].ToolName })
	sort.Slice(m.Engines, func(i, j int) bool { return m.Engines[i].Name < m.Engines[j].Name })
	return m, nil
}

// readEntryFile reads one entry, checks its permissions and reads its kind: `tool` when
// the field is absent. The kind field's type is checked here; the rest by the kind's decoder.
func readEntryFile(path, env string) ([]byte, EntryKind, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", &ManifestError{Env: env, Path: path, Detail: "cannot read the entry", Err: err}
	}
	if err := ownerWritableOnly(path, info); err != nil {
		return nil, "", &ManifestError{Env: env, Path: path, Detail: err.Error()}
	}
	data, err := os.ReadFile(path) // #nosec G304 -- an entry of a manifest directory the environment names
	if err != nil {
		return nil, "", &ManifestError{Env: env, Path: path, Detail: "cannot read the entry", Err: err}
	}
	var head struct {
		Kind json.RawMessage `json:"kind"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, "", &ManifestError{Env: env, Path: path, Detail: "not one JSON object", Err: err}
	}
	if len(head.Kind) == 0 || jsonNull(head.Kind) {
		return data, KindTool, nil
	}
	var kind string
	if err := json.Unmarshal(head.Kind, &kind); err != nil {
		return nil, "", &ManifestError{Env: env, Path: path, Detail: "kind is not a string", Err: err}
	}
	return data, EntryKind(strings.TrimSpace(kind)), nil
}

// ownerWritableOnly refuses a manifest file or directory writable by anyone but its owner,
// the check ssh makes of its configuration; the bits mean nothing on Windows.
func ownerWritableOnly(path string, info os.FileInfo) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if perm := info.Mode().Perm(); perm&0o022 != 0 {
		return fmt.Errorf("is writable by others (mode %04o); a manifest entry and its directory may be written by their owner alone", perm)
	}
	return nil
}

// readToolEntry reads one tool entry, resolving a relative executable against the entry's directory.
func readToolEntry(path, env string, data []byte) (ToolEntry, error) {
	fault := func(detail string, err error) (ToolEntry, error) {
		return ToolEntry{}, &ManifestError{Env: env, Path: path, Detail: detail, Err: err}
	}
	var entry ToolEntry
	if err := decodeOne(data, &entry); err != nil {
		return fault("not one JSON object of kind, toolName, version, executable and variables", err)
	}
	entry.File = path
	entry.Kind = KindTool
	entry.ToolName = strings.TrimSpace(entry.ToolName)
	entry.Version = strings.TrimSpace(entry.Version)
	entry.Executable = strings.TrimSpace(entry.Executable)
	switch {
	case entry.ToolName == "":
		return fault("toolName is empty", nil)
	case strings.ContainsAny(entry.ToolName, " \t\r\n"):
		return fault(fmt.Sprintf("toolName %q has whitespace in it", entry.ToolName), nil)
	case entry.Executable == "":
		return fault("executable is empty", nil)
	case entry.Variables == nil:
		return fault("variables is missing", nil)
	}
	seen := make(map[string]bool, len(entry.Variables))
	for _, v := range entry.Variables {
		switch {
		case strings.TrimSpace(v) == "":
			return fault("variables has an empty name", nil)
		case seen[v]:
			return fault(fmt.Sprintf("variables lists %q twice", v), nil)
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

// ToolsFromEnv reads the manifest OPENSYSML_TOOLS names as engines, one per tool entry;
// none when it is unset. A manifest that cannot be read is a ManifestError.
func ToolsFromEnv() ([]External, error) {
	m, err := manifestFromEnv(ToolsEnv)
	if err != nil || m == nil {
		return nil, err
	}
	tools := make([]External, len(m.Tools))
	for i, entry := range m.Tools {
		tools[i] = NewTool(entry)
	}
	return tools, nil
}

// ManifestsFromEnv reads both manifest directories the environment names, OPENSYSML_TOOLS
// then OPENSYSML_ENGINES, skipping one unset; a manifest that cannot be read is a ManifestError.
func ManifestsFromEnv() ([]*Manifest, error) {
	var manifests []*Manifest
	for _, env := range []string{ToolsEnv, EnginesEnv} {
		m, err := manifestFromEnv(env)
		if err != nil {
			return nil, err
		}
		if m != nil {
			manifests = append(manifests, m)
		}
	}
	return manifests, nil
}

// manifestFromEnv reads the manifest the variable names, nil when it is unset.
func manifestFromEnv(env string) (*Manifest, error) {
	dir := strings.TrimSpace(os.Getenv(env))
	if dir == "" {
		return nil, nil
	}
	return ReadManifest(dir, env)
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
