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

// ExpectedEvent is one trace line: Execute (an activity started), Fire (an
// action ran), Output or Post (a parameter received a value).
type ExpectedEvent struct {
	Kind      string `json:"kind"`
	Activity  string `json:"activity"`
	Action    string `json:"action,omitempty"`
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

// Refired lists the actions fired more than once within the activity's own
// execution, in first-firing order: fUML's per-token firing, which v2 lacks.
func (a *ExpectedActivity) Refired() []string {
	counts := map[string]int{}
	var order []string
	for _, ev := range a.Events {
		if ev.Kind != "Fire" || ev.Activity != a.Name {
			continue
		}
		if counts[ev.Action] == 0 {
			order = append(order, ev.Action)
		}
		counts[ev.Action]++
	}
	var refired []string
	for _, action := range order {
		if counts[action] > 1 {
			refired = append(refired, action)
		}
	}
	return refired
}
