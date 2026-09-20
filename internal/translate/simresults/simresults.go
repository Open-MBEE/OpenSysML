// Package simresults is the schema of the result sidecar a SysML v1 migration
// writes (-migration-results) and the comparison of a migrated run configuration
// with its tool's stored runs reads (-compare-results).
package simresults

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// A simulation tool stores each run of a configuration as a result snapshot: an
// instance under the resultLocation package whose slots hold the values observed.
// Results indexes them per configuration for the JSON sidecar -migration-results writes.

// Results are the result snapshots of every run configuration of a document,
// in the order the configurations are written.
type Results struct {
	// Source names the v1 document migrated.
	Source         string                 `json:"source"`
	Configurations []ConfigurationResults `json:"configurations"`
}

// ConfigurationResults are one configuration's run settings and the snapshots the tool stored.
type ConfigurationResults struct {
	// ID is the configuration's xmi:id, Name the qualified name of its action def.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Runs is numberOfRuns (0 for none); Draws is durationSimulationMode as a draw policy ("" for none).
	Runs  int64  `json:"runs,omitempty"`
	Draws string `json:"draws,omitempty"`
	// Target is the part holding the execution target, Behavior the action usage performing it; "" for none.
	Target   string `json:"target,omitempty"`
	Behavior string `json:"behavior,omitempty"`
	// Location is the qualified v1 name of the result package, "" for none.
	Location string `json:"resultLocation,omitempty"`
	// Observables are the properties the snapshots hold numbers for, sorted.
	Observables []string   `json:"observables"`
	Snapshots   []Snapshot `json:"snapshots"`
	// Notes say what of the tool's results has no place in the sidecar.
	Notes []string `json:"notes,omitempty"`
}

// Snapshot is one run the tool stored: the numbers its slots hold, by property.
type Snapshot struct {
	ID     string             `json:"id"`
	Name   string             `json:"name,omitempty"`
	Values map[string]float64 `json:"values"`
}

// Values are the numbers every snapshot holds for observable, in snapshot order.
func (c *ConfigurationResults) Values(observable string) []float64 {
	var out []float64
	for _, s := range c.Snapshots {
		if v, ok := s.Values[observable]; ok {
			out = append(out, v)
		}
	}
	return out
}

// Summary counts what the sidecar indexes: configurations, those with snapshots, and snapshots.
func (r *Results) Summary() string {
	stored, snapshots := 0, 0
	for _, c := range r.Configurations {
		if len(c.Snapshots) > 0 {
			stored++
		}
		snapshots += len(c.Snapshots)
	}
	return fmt.Sprintf("results of %d run configuration(s): %d with %d stored snapshot(s)", len(r.Configurations), stored, snapshots)
}

// Read reads a sidecar -migration-results wrote: exactly one JSON document
// indexing at least one configuration; content after it is an error.
func Read(r io.Reader) (*Results, error) {
	var out Results
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("the results are not the JSON -migration-results writes: %w", err)
	}
	var trailing json.RawMessage
	switch err := dec.Decode(&trailing); {
	case err == nil:
		return nil, fmt.Errorf("the results are not the JSON -migration-results writes: content follows the document")
	case !errors.Is(err, io.EOF):
		return nil, fmt.Errorf("the results are not the JSON -migration-results writes: %w", err)
	}
	if out.Source == "" || out.Configurations == nil {
		return nil, fmt.Errorf("the results are not the JSON -migration-results writes: no source or configurations")
	}
	return &out, nil
}
