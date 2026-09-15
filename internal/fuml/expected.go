package fuml

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
)

// ExpectedPath is the committed record of what the pinned implementation computed,
// relative to the repository root; `make fuml-expected` regenerates it.
const ExpectedPath = "docs/project/fuml-referee-expected.json"

// Expected is the record scripts/fuml-driver writes: the implementation's
// outputs and event trace for every activity the pinned models declare.
type Expected struct {
	Meaning    string             `json:"meaning"`
	Provenance ExpectedProvenance `json:"provenance"`
	Activities []ExpectedActivity `json:"activities"`
}

// ExpectedProvenance identifies the implementation and the models it ran.
type ExpectedProvenance struct {
	RITag           string          `json:"riTag"`
	RICommit        string          `json:"riCommit"`
	JarDigest       string          `json:"jarDigest"`
	LibraryResource string          `json:"libraryResource"`
	LibraryDigest   string          `json:"libraryDigest"`
	Models          []ExpectedModel `json:"models"`
}

// ExpectedModel is one model file the implementation loaded.
type ExpectedModel struct {
	File   string `json:"file"`
	URI    string `json:"uri"`
	Digest string `json:"digest"`
}

// ExpectedActivity is one uml:Activity a model declares; only those packaged
// directly in the model are executed, and Skipped says why the others were not.
type ExpectedActivity struct {
	Model      string              `json:"model"`
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Executed   bool                `json:"executed"`
	Skipped    string              `json:"skipped,omitempty"`
	Parameters []ExpectedParameter `json:"parameters,omitempty"`
	Outputs    []ExpectedOutput    `json:"outputs,omitempty"`
	Error      string              `json:"error,omitempty"`
	Events     []ExpectedEvent     `json:"events,omitempty"`
}

// ExpectedParameter is a parameter of the activity as the implementation
// loaded it. Upper is a natural number or "*".
type ExpectedParameter struct {
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Type      string `json:"type"`
	Lower     int    `json:"lower"`
	Upper     string `json:"upper"`
	IsOrdered bool   `json:"isOrdered"`
	IsUnique  bool   `json:"isUnique"`
}

// ExpectedOutput is the values one output or inout parameter held when the
// activity completed, in the implementation's order.
type ExpectedOutput struct {
	Parameter string          `json:"parameter"`
	Values    []ExpectedValue `json:"values"`
}

// ExpectedValue is one runtime value: primitives and enumerations carry Value,
// a Reference its Referent, and objects, links and signals their types and features.
type ExpectedValue struct {
	Kind      string            `json:"kind"`
	Value     json.RawMessage   `json:"value,omitempty"`
	Type      string            `json:"type,omitempty"`
	ID        string            `json:"id,omitempty"`
	Types     []string          `json:"types,omitempty"`
	Features  []ExpectedFeature `json:"features,omitempty"`
	Referent  *ExpectedValue    `json:"referent,omitempty"`
	Truncated bool              `json:"truncated,omitempty"`
}

// ExpectedFeature is one structural feature of an object and the values it holds.
type ExpectedFeature struct {
	Feature string          `json:"feature"`
	Values  []ExpectedValue `json:"values"`
}

// ExpectedEvent is one trace line: Execute and Complete (an activity execution
// started and ended), Fire (an action ran), Output or Post (a parameter received
// a value). The implementation names elements; ID is the XMI id of the activity
// (Execute, Complete) or action node (Fire) where the name identifies exactly
// one, and empty where it does not.
type ExpectedEvent struct {
	Kind      string `json:"kind"`
	Activity  string `json:"activity"`
	Action    string `json:"action,omitempty"`
	ID        string `json:"id,omitempty"`
	Parameter string `json:"parameter,omitempty"`
	Value     string `json:"value,omitempty"`
}

// ReadExpected decodes the committed record at path.
func ReadExpected(path string) (*Expected, error) {
	content, err := os.ReadFile(path) // #nosec G304 -- callers name the committed record in this repository
	if err != nil {
		return nil, err
	}
	var e Expected
	if err := json.Unmarshal(content, &e); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &e, nil
}

// ErrStaleExpected reports that the record was produced from a different pin
// than the one scripts/fuml-pin.sh holds.
var ErrStaleExpected = errors.New("expected-record does not match the pin")

// Check refuses a record produced by anything but the pinned implementation
// over the pinned models, so a moved pin cannot be compared against old truth.
func (p Pin) Check(e *Expected) error {
	var differences []string
	if e.Provenance.RITag != p.Tag {
		differences = append(differences, fmt.Sprintf("riTag %q, pin %q", e.Provenance.RITag, p.Tag))
	}
	if e.Provenance.RICommit != p.Commit {
		differences = append(differences, fmt.Sprintf("riCommit %q, pin %q", e.Provenance.RICommit, p.Commit))
	}
	if e.Provenance.JarDigest != p.Jar {
		differences = append(differences, fmt.Sprintf("jarDigest %s, pin %s", e.Provenance.JarDigest, p.Jar))
	}
	want := map[string]string{TestsFile: p.Tests, ExceptionTestsFile: p.ExceptionTest}
	seen := map[string]bool{}
	for _, m := range e.Provenance.Models {
		sum, ok := want[m.File]
		if !ok {
			differences = append(differences, fmt.Sprintf("model %s is not pinned", m.File))
			continue
		}
		seen[m.File] = true
		if m.Digest != sum {
			differences = append(differences, fmt.Sprintf("%s digest %s, pin %s", m.File, m.Digest, sum))
		}
	}
	for file := range want {
		if !seen[file] {
			differences = append(differences, fmt.Sprintf("model %s was not run", file))
		}
	}
	if len(differences) == 0 {
		return nil
	}
	sort.Strings(differences)
	return fmt.Errorf("%w: %v; run `make fuml-expected`", ErrStaleExpected, differences)
}

// Activity returns the record of the activity with the given XMI id in the
// given model file, or nil.
func (e *Expected) Activity(model, id string) *ExpectedActivity {
	for i := range e.Activities {
		a := &e.Activities[i]
		if a.Model == model && a.ID == id {
			return a
		}
	}
	return nil
}

// Refired lists the actions fired more than once within one execution of the
// activity itself, in first-firing order: fUML's per-token firing, which v2
// lacks. A Fire without an id names nothing in particular and is not counted.
func (a *ExpectedActivity) Refired() []string {
	var refired []string
	for _, act := range a.activations() {
		if act.id != a.ID {
			continue
		}
		for _, f := range act.fires {
			if f.count > 1 {
				refired = append(refired, f.action)
			}
		}
	}
	return refired
}

// activation is one execution of an activity in the trace, with how often each
// of its action nodes fired, in first-firing order.
type activation struct {
	id, name string
	fires    []*fireCount
	byNode   map[string]*fireCount
}

type fireCount struct {
	id, action string
	count      int
}

// activations replays the trace: Execute opens an activation, Complete closes
// the innermost one of that activity, and a Fire counts toward the innermost
// open activation of its activity. Nested and recursive executions thus keep
// their counts apart, and a caller's count resumes when its callee completes.
func (a *ExpectedActivity) activations() []*activation {
	var all, open []*activation
	innermost := func(name string) int {
		for i := len(open) - 1; i >= 0; i-- {
			if open[i].name == name {
				return i
			}
		}
		return -1
	}
	for _, ev := range a.Events {
		switch ev.Kind {
		case "Execute":
			act := &activation{id: ev.ID, name: ev.Activity, byNode: map[string]*fireCount{}}
			all = append(all, act)
			open = append(open, act)
		case "Complete":
			if i := innermost(ev.Activity); i >= 0 {
				open = append(open[:i], open[i+1:]...)
			}
		case "Fire":
			if ev.ID == "" {
				continue
			}
			i := innermost(ev.Activity)
			if i < 0 {
				continue
			}
			act := open[i]
			f := act.byNode[ev.ID]
			if f == nil {
				f = &fireCount{id: ev.ID, action: ev.Action}
				act.byNode[ev.ID] = f
				act.fires = append(act.fires, f)
			}
			f.count++
		}
	}
	return all
}
