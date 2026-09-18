package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// suiteFile is one scenario file: a list of scenarios sharing a theme.
type suiteFile struct {
	Scenarios []*Scenario `json:"scenarios"`
}

// Model is the source a scenario needs parsed before its call, named by fixture
// so no scenario carries a machine-specific path. Fixture is parsed alone with
// ParseFile; Fixtures are parsed together with ParseSources, each document
// named by its fixture, so an edit can report which one it rewrote.
type Model struct {
	Fixture           string   `json:"fixture,omitempty"`
	Fixtures          []string `json:"fixtures,omitempty"`
	Language          string   `json:"language,omitempty"`
	StrictConformance bool     `json:"strict_conformance,omitempty"`
}

// fixtureNames lists every fixture the model reads.
func (m Model) fixtureNames() []string {
	if len(m.Fixtures) > 0 {
		return m.Fixtures
	}
	return []string{m.Fixture}
}

// key identifies a model for the once-per-run parse cache.
func (m Model) key() string {
	return fmt.Sprintf("%q|%s|%t", m.fixtureNames(), m.Language, m.StrictConformance)
}

// validate refuses a model naming no fixture or both a fixture and fixtures.
func (m Model) validate() error {
	switch {
	case m.Fixture == "" && len(m.Fixtures) == 0:
		return errors.New("a model needs a fixture or fixtures")
	case m.Fixture != "" && len(m.Fixtures) > 0:
		return errors.New("a model names either a fixture or fixtures, not both")
	}
	return nil
}

// Expect is what a call must answer. Every field is optional; an absent Status
// means the call must succeed.
type Expect struct {
	Status                string              `json:"status,omitempty"`
	StatusMessageContains string              `json:"status_message_contains,omitempty"`
	Response              map[string]any      `json:"response,omitempty"`
	Contains              map[string]string   `json:"contains,omitempty"`
	ContainsAll           map[string][]string `json:"contains_all,omitempty"`
	NonEmpty              []string            `json:"non_empty,omitempty"`
	Absent                []string            `json:"absent,omitempty"`
	Counts                map[string]int      `json:"counts,omitempty"`
	MinCounts             map[string]int      `json:"min_counts,omitempty"`
}

// Scenario is one conformance case: a call to make and what it must answer.
type Scenario struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	// RPC is the method, either bare ("Evaluate") or fully qualified
	// ("sysml.SysMLService/Evaluate"). The transport is not named here.
	RPC string `json:"rpc"`
	// Capabilities GetServerInfo must report for Expect to apply.
	RequiresCapabilities []string `json:"requires_capabilities,omitempty"`
	// What the call must answer on a service reporting none of them; unset
	// leaves such a service untested by this scenario.
	ExpectWithoutCapability *Expect `json:"expect_without_capability,omitempty"`
	Model                   *Model  `json:"model,omitempty"`
	// Request as protobuf-JSON. "${model_hash}" stands for Model's hash, and
	// "${fixture:<path>}" for a fixture's contents.
	Request json.RawMessage `json:"request"`
	Expect  Expect          `json:"expect"`

	file string // scenario file this came from, for error messages
}

// loadScenarios reads every scenario file in dir, in file then declaration
// order, so a run reports scenarios in a stable order.
func loadScenarios(dir string) ([]*Scenario, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	if len(entries) == 0 {
		return nil, fmt.Errorf("no scenario files in %s", dir)
	}

	var scenarios []*Scenario
	seen := map[string]string{}
	for _, entry := range entries {
		data, err := os.ReadFile(entry) // #nosec G304 -- scenario files come from the suite directory.
		if err != nil {
			return nil, err
		}
		var suite suiteFile
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		// Expected numbers stay as their literals, so a whole number too large
		// for a float64 is still compared by its digits.
		decoder.UseNumber()
		if err := decoder.Decode(&suite); err != nil {
			return nil, fmt.Errorf("%s: %w", entry, err)
		}
		for _, scenario := range suite.Scenarios {
			if scenario.ID == "" || scenario.RPC == "" {
				return nil, fmt.Errorf("%s: every scenario needs an id and an rpc", entry)
			}
			if scenario.Model != nil {
				if err := scenario.Model.validate(); err != nil {
					return nil, fmt.Errorf("%s: %s: %w", entry, scenario.ID, err)
				}
			}
			if where, dup := seen[scenario.ID]; dup {
				return nil, fmt.Errorf("%s: scenario id %q is already declared in %s", entry, scenario.ID, where)
			}
			seen[scenario.ID] = entry
			scenario.file = entry
			scenarios = append(scenarios, scenario)
		}
	}
	return scenarios, nil
}

// method is the scenario's RPC as a bare method name.
func (s *Scenario) method() string {
	if index := strings.LastIndex(s.RPC, "/"); index >= 0 {
		return s.RPC[index+1:]
	}
	return s.RPC
}
