package analysis

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProtocolVersions are the versions of the engine message set this build serves.
var ProtocolVersions = []int{1}

// GraphsVersions are the versions of the `graphs:<v>` model form this build serves.
var GraphsVersions = []int{1}

// TransportStdio is a child process on a pair of pipes, the one transport this build serves.
const TransportStdio = "stdio"

// TransportGRPC is a bidirectional streaming RPC to an address; defined, not yet served.
const TransportGRPC = "grpc"

// WitnessKind is the shape of witness an engine's manifest declares it produces.
type WitnessKind string

const (
	// WitnessNone declares no replayable witness; an existential claim is then not covered.
	WitnessNone WitnessKind = "none"
	// WitnessSchedule declares schedule witnesses in the replay format.
	WitnessSchedule WitnessKind = "schedule"
	// WitnessAssignment declares assignment witnesses: values of the free features.
	WitnessAssignment WitnessKind = "assignment"
)

// ModelForm is one form of the model an engine reads.
type ModelForm string

const (
	// FormSources is every document of the model as text with the library version.
	FormSources ModelForm = "sources"
	// FormRDF is the model as Turtle; defined, served by a later stage.
	FormRDF ModelForm = "rdf"
)

// GraphsForm is the `graphs:<v>` form at version v.
func GraphsForm(v int) ModelForm { return ModelForm("graphs:" + strconv.Itoa(v)) }

// GraphsVersion reads the version of a `graphs:<v>` form; false for any other form.
func (f ModelForm) GraphsVersion() (int, bool) {
	rest, ok := strings.CutPrefix(string(f), "graphs:")
	if !ok {
		return 0, false
	}
	v, err := strconv.Atoi(rest)
	if err != nil || v < 1 {
		return 0, false
	}
	return v, true
}

// EngineEntry is one `engine`, `policy` or `sampler` entry of a manifest, as the framework's
// Description written down: the fields the host checks before any process runs.
type EngineEntry struct {
	// File is the entry read; Dir its directory, which a relative command is confined to.
	File string
	Dir  string
	// Kind is engine, policy or sampler.
	Kind EntryKind
	// Name is the engine's registered name, as -engine selects it.
	Name    string
	Version string
	// Command is the executable and its arguments as written; Executable the resolved path
	// of Command[0], confined to Dir when relative.
	Command    []string
	Executable string
	// Transport is stdio or grpc; Address is where a grpc engine listens.
	Transport string
	Address   string
	// Module is a WebAssembly module in place of Command; defined, served by a later stage.
	Module string
	// Protocol is the version of the message set the engine speaks.
	Protocol int
	// Answers, Subjects, Bounds and Witness are the engine's Description; Subjects empty
	// means any subject.
	Answers  []Kind
	Subjects []string
	Bounds   []string
	Witness  WitnessKind
	// Model names the forms of the model the engine reads; sources is always sent.
	Model []ModelForm
	// Authority is the strongest strength the engine says it produces for a universal claim.
	Authority Strength
	// Concurrent says whether one process may hold several run requests open at once.
	Concurrent bool
}

// engineWire is an engine entry as written, decoded strictly before it is checked.
type engineWire struct {
	Kind       string          `json:"kind"`
	Name       string          `json:"name"`
	Version    string          `json:"version"`
	Command    []string        `json:"command"`
	Address    string          `json:"address"`
	Module     string          `json:"module"`
	Transport  string          `json:"transport"`
	Protocol   json.RawMessage `json:"protocol"`
	Answers    []string        `json:"answers"`
	Subjects   []string        `json:"subjects"`
	Model      []string        `json:"model"`
	Bounds     []string        `json:"bounds"`
	Witness    string          `json:"witness"`
	Authority  string          `json:"authority"`
	Admit      json.RawMessage `json:"admit"`
	Concurrent *bool           `json:"concurrent"`
}

// BoundNames are the bounds an engine entry may take, as the budget spells them.
var BoundNames = []string{"depth", "steps", "runs", "memory"}

// readEngineEntry reads one engine, policy or sampler entry and checks every field the
// manifest fixes: the name, the command's confinement, the protocol, the question fields.
func readEngineEntry(path, env string, kind EntryKind, data []byte) (EngineEntry, error) {
	fault := func(detail string, err error) (EngineEntry, error) {
		return EngineEntry{}, &ManifestError{Env: env, Path: path, Detail: detail, Err: err}
	}
	var wire engineWire
	if err := decodeOne(data, &wire); err != nil {
		return fault(fmt.Sprintf("not one JSON object of the %s entry's fields", kind), err)
	}
	entry := EngineEntry{File: path, Dir: filepath.Dir(path), Kind: kind,
		Name: strings.TrimSpace(wire.Name), Version: strings.TrimSpace(wire.Version),
		Command: wire.Command, Address: strings.TrimSpace(wire.Address), Module: strings.TrimSpace(wire.Module),
		Transport: strings.TrimSpace(wire.Transport), Subjects: wire.Subjects, Bounds: wire.Bounds, Concurrent: true}
	switch {
	case entry.Name == "":
		return fault("name is empty", nil)
	case strings.ContainsAny(entry.Name, " \t\r\n"):
		return fault(fmt.Sprintf("name %q has whitespace in it", entry.Name), nil)
	case strings.HasPrefix(entry.Name, ToolEnginePrefix):
		return fault(fmt.Sprintf("name %q is the tool engines' form; tools are `kind: tool` entries", entry.Name), nil)
	case entry.Name == "auto" || entry.Name == "all":
		return fault(fmt.Sprintf("name %q is a selection, not an engine", entry.Name), nil)
	}
	if len(wire.Admit) > 0 && !jsonNull(wire.Admit) {
		return fault("admit is not honored by this build: it needs a referee record, which the referee-record stage adds", nil)
	}
	if entry.Transport == "" {
		entry.Transport = TransportStdio
	}
	switch entry.Transport {
	case TransportStdio:
		if entry.Address != "" {
			return fault("address is for the grpc transport; a stdio engine has a command", nil)
		}
	case TransportGRPC:
		if entry.Address == "" {
			return fault("transport grpc needs an address", nil)
		}
		if len(entry.Command) > 0 || entry.Module != "" {
			return fault("transport grpc names an address in place of a command or module", nil)
		}
	default:
		return fault(fmt.Sprintf("transport %q is not stdio or grpc", entry.Transport), nil)
	}
	if entry.Transport == TransportStdio {
		switch {
		case entry.Module != "" && len(entry.Command) > 0:
			return fault("module and command are alternatives; an entry has one", nil)
		case entry.Module == "" && len(entry.Command) == 0:
			return fault("command is empty", nil)
		case entry.Module != "":
			resolved, err := confinedPath(entry.Dir, entry.Module)
			if err != nil {
				return fault("module "+err.Error(), nil)
			}
			entry.Module = resolved
		default:
			if strings.TrimSpace(entry.Command[0]) == "" {
				return fault("command's executable is empty", nil)
			}
			resolved, err := confinedPath(entry.Dir, entry.Command[0])
			if err != nil {
				return fault("command "+err.Error(), nil)
			}
			entry.Executable = resolved
		}
	}
	if len(wire.Protocol) == 0 {
		return fault("protocol is missing", nil)
	}
	if err := json.Unmarshal(wire.Protocol, &entry.Protocol); err != nil || entry.Protocol < 1 {
		return fault(fmt.Sprintf("protocol %s is not a positive integer", wire.Protocol), nil)
	}
	if !containsInt(ProtocolVersions, entry.Protocol) {
		return fault(fmt.Sprintf("protocol %d is not served; this build serves %s", entry.Protocol, joinInts(ProtocolVersions)), nil)
	}
	if kind != KindEngine {
		if wire.Answers != nil || wire.Subjects != nil || wire.Model != nil || wire.Bounds != nil || wire.Witness != "" || wire.Authority != "" {
			return fault(fmt.Sprintf("a %s entry has no question fields (answers, subjects, model, bounds, witness, authority)", kind), nil)
		}
		return entry, nil
	}
	if len(wire.Answers) == 0 {
		return fault("answers is empty", nil)
	}
	seen := make(map[string]bool)
	for _, a := range wire.Answers {
		k, ok := ParseKind(a)
		if !ok {
			return fault(fmt.Sprintf("answers names %q, not a kind of question", a), nil)
		}
		if seen[a] {
			return fault(fmt.Sprintf("answers lists %q twice", a), nil)
		}
		seen[a] = true
		entry.Answers = append(entry.Answers, k)
	}
	for _, s := range entry.Subjects {
		if strings.TrimSpace(s) == "" {
			return fault("subjects has an empty name", nil)
		}
	}
	for _, b := range entry.Bounds {
		if !containsString(BoundNames, b) {
			return fault(fmt.Sprintf("bounds names %q, not one of %s", b, strings.Join(BoundNames, ", ")), nil)
		}
	}
	switch WitnessKind(wire.Witness) {
	case "", WitnessNone:
		entry.Witness = WitnessNone
	case WitnessSchedule, WitnessAssignment:
		entry.Witness = WitnessKind(wire.Witness)
	default:
		return fault(fmt.Sprintf("witness %q is not schedule, assignment or none", wire.Witness), nil)
	}
	for _, f := range wire.Model {
		form := ModelForm(f)
		switch {
		case form == FormSources, form == FormRDF:
		default:
			v, ok := form.GraphsVersion()
			if !ok {
				return fault(fmt.Sprintf("model names %q, not sources, graphs:<v> or rdf", f), nil)
			}
			if !containsInt(GraphsVersions, v) {
				return fault(fmt.Sprintf("model form %s is not served; this build serves graphs:%s", f, joinInts(GraphsVersions)), nil)
			}
		}
		entry.Model = append(entry.Model, form)
	}
	authority, ok := ParseStrength(wire.Authority)
	if !ok || authority == NotCovered {
		return fault(fmt.Sprintf("authority %q is not observed, witnessed, bounded or proved", wire.Authority), nil)
	}
	entry.Authority = authority
	if wire.Concurrent != nil {
		entry.Concurrent = *wire.Concurrent
	}
	return entry, nil
}

// confinedPath resolves a command or module path: an absolute path as given, a relative one
// joined to dir, followed through every link, and refused unless it stays inside dir.
func confinedPath(dir, path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("%q: the manifest directory cannot be resolved: %v", path, err)
	}
	joined := filepath.Join(dir, path)
	if rel, err := filepath.Rel(dir, joined); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%q names a path outside the manifest directory %s", path, dir)
	}
	resolved, err := filepath.EvalSymlinks(joined)
	switch {
	case err == nil:
		if rel, err := filepath.Rel(root, resolved); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("%q resolves to %s, outside the manifest directory %s", path, resolved, dir)
		}
		return resolved, nil
	case os.IsNotExist(err):
		return joined, nil
	default:
		return "", fmt.Errorf("%q cannot be resolved: %v", path, err)
	}
}

// ErrNotServed is the typed error for a manifest entry this build lists but does not run.
var ErrNotServed = errors.New("manifest entry is not served in this build")

// NotServedError reports an entry a later build serves: listed with the reason, never run.
type NotServedError struct {
	Name string
	Kind EntryKind
	// Reason names what the entry needs that this build does not have.
	Reason string
}

// Error names the entry and the reason.
func (e *NotServedError) Error() string {
	return fmt.Sprintf("%s %q is not served in this build: %s", e.Kind, e.Name, e.Reason)
}

// Is matches ErrNotServed.
func (e *NotServedError) Is(target error) bool { return target == ErrNotServed }

// Served reports whether this build runs the entry, and the typed reason when it does not:
// strategies, WebAssembly modules, the grpc transport and the rdf form are later stages.
func (e EngineEntry) Served() error {
	switch {
	case e.Kind == KindPolicy:
		return &NotServedError{Name: e.Name, Kind: e.Kind, Reason: "scheduling policies are the strategies stage"}
	case e.Kind == KindSampler:
		return &NotServedError{Name: e.Name, Kind: e.Kind, Reason: "sweep samplers are the strategies stage"}
	case e.Module != "":
		return &NotServedError{Name: e.Name, Kind: e.Kind, Reason: "WebAssembly modules are the WebAssembly stage"}
	case e.Transport == TransportGRPC:
		return &NotServedError{Name: e.Name, Kind: e.Kind, Reason: "the grpc transport is defined and deferred until a remote engine needs it"}
	}
	for _, form := range e.Model {
		if form == FormRDF {
			return &NotServedError{Name: e.Name, Kind: e.Kind, Reason: "the rdf model form is the strategies stage, with the RDF export it rests on"}
		}
	}
	return nil
}

// Forms are the model forms a run carries: sources always, then what the entry asked for.
func (e EngineEntry) Forms() []ModelForm {
	forms := []ModelForm{FormSources}
	for _, f := range e.Model {
		if f != FormSources {
			forms = append(forms, f)
		}
	}
	return forms
}

// Description is the entry as the framework describes an engine.
func (e EngineEntry) Description() Description {
	process := e.Executable
	if e.Transport == TransportGRPC {
		process = e.Address
	} else if e.Module != "" {
		process = e.Module
	}
	return Description{Questions: e.Answers, Process: process, Bounds: e.Bounds,
		Replays: e.Witness != WitnessNone, Authority: e.Authority}
}

// Origin is where the entry comes from, as a listing prints it.
func (e EngineEntry) Origin() Origin {
	return Origin{Kind: e.Kind, Version: e.Version, File: e.File, Command: e.Executable,
		Transport: e.Transport, Protocol: e.Protocol}
}

// ParseKind reads a kind of question as Kind.String spells it.
func ParseKind(text string) (Kind, bool) {
	for _, k := range []Kind{Evaluate, Outcomes, Holds, Sensitive, Satisfiable, Sweep, Compute} {
		if k.String() == text {
			return k, true
		}
	}
	return 0, false
}

// ParseStrength reads a strength as Strength.String spells it.
func ParseStrength(text string) (Strength, bool) {
	for _, s := range []Strength{NotCovered, Observed, Witnessed, Bounded, Proved} {
		if s.String() == text {
			return s, true
		}
	}
	return 0, false
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func joinInts(list []int) string {
	parts := make([]string, len(list))
	for i, v := range list {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ", ")
}
