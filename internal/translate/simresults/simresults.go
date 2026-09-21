// Package simresults is the schema of the result sidecar a SysML v1 migration
// writes (-migration-results) and the comparison of a migrated run configuration
// with its tool's stored runs reads (-compare-results).
package simresults

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
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
	// ClockStep is the step, in seconds, the tool's internal simulation clock ticked by
	// (stepSize in timeUnit, once startTime enables the clock); 0 for the built-in clock.
	ClockStep float64 `json:"clockStep,omitempty"`
	// Target is the part holding the execution target, Behavior the action usage performing it; "" for none.
	Target   string `json:"target,omitempty"`
	Behavior string `json:"behavior,omitempty"`
	// Location is the qualified v1 name of the result package, "" for none.
	Location string `json:"resultLocation,omitempty"`
	// Analysis is the observable the target's Monte Carlo analysis summarises, "" for none.
	Analysis string `json:"analysis,omitempty"`
	// Observables are the properties the snapshots hold numbers for, sorted.
	Observables []string   `json:"observables"`
	Snapshots   []Snapshot `json:"snapshots"`
	// Notes say what of the tool's results has no place in the sidecar.
	Notes []string `json:"notes,omitempty"`
}

// Snapshot is one run the tool stored: the numbers its slots hold, by property;
// or the statistics of several runs, when the tool summarised them in one.
type Snapshot struct {
	ID         string             `json:"id"`
	Name       string             `json:"name,omitempty"`
	Values     map[string]float64 `json:"values"`
	Statistics *Statistics        `json:"statistics,omitempty"`
}

// Statistics are what a tool's Monte Carlo analysis records of one observable
// over the runs of a snapshot: their count, mean and standard deviation.
type Statistics struct {
	Observable string  `json:"observable"`
	Runs       int64   `json:"runs"`
	Mean       float64 `json:"mean"`
	Deviation  float64 `json:"deviation"`
}

// Values are the numbers the snapshots hold for observable run by run, in snapshot
// order; a snapshot summarising the observable holds its mean, which is no run's.
func (c *ConfigurationResults) Values(observable string) []float64 {
	var out []float64
	for _, s := range c.Snapshots {
		if s.Statistics != nil && s.Statistics.Observable == observable {
			continue
		}
		if v, ok := s.Values[observable]; ok {
			out = append(out, v)
		}
	}
	return out
}

// Summarised are the snapshots recording statistics of observable, in snapshot
// order; none when every snapshot stores the observable run by run.
func (c *ConfigurationResults) Summarised(observable string) []Snapshot {
	var out []Snapshot
	for _, s := range c.Snapshots {
		if s.Statistics != nil && s.Statistics.Observable == observable {
			out = append(out, s)
		}
	}
	return out
}

// Disagreeing are the summaries of observable whose mean lies more than three
// standard errors from another's, so they cannot be of one model; in snapshot order.
func (c *ConfigurationResults) Disagreeing(observable string) []Snapshot {
	summarised := c.Summarised(observable)
	var out []Snapshot
	for i, s := range summarised {
		for j, o := range summarised {
			if i != j && s.Statistics.Disagrees(*o.Statistics) {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// Disagrees reports whether s and o cannot summarise runs of one distribution:
// their means lie more than three standard errors of their difference apart.
func (s Statistics) Disagrees(o Statistics) bool {
	if s.Runs < 2 || o.Runs < 2 {
		return false
	}
	se := math.Sqrt(s.Deviation*s.Deviation/float64(s.Runs) + o.Deviation*o.Deviation/float64(o.Runs))
	return math.Abs(s.Mean-o.Mean) > 3*se
}

// StoredRuns are the runs the snapshots stand for: each summarised snapshot
// counts its runs, every other one run.
func (c *ConfigurationResults) StoredRuns() int64 {
	var runs int64
	for _, s := range c.Snapshots {
		if s.Statistics != nil {
			runs += s.Statistics.Runs
			continue
		}
		runs++
	}
	return runs
}

// Repeats notes each summarising snapshot of configuration i that another
// configuration's snapshot repeats — same name, same statistics, no number apart —
// since Monte Carlo runs do not come out alike twice, but a stored result is copied.
func (r *Results) Repeats(i int) []string {
	var notes []string
	for _, s := range r.Configurations[i].Snapshots {
		var twins []string
		for j := range r.Configurations {
			if j == i {
				continue
			}
			for _, o := range r.Configurations[j].Snapshots {
				if s.Repeats(o) {
					twins = append(twins, fmt.Sprintf("%s (in %s)", r.Configurations[j].Name, r.Configurations[j].Location))
				}
			}
		}
		if len(twins) > 0 {
			notes = append(notes, fmt.Sprintf("the snapshot %q bears the name and the statistics of %s of a snapshot of the configuration %s, so one may be a copy of the other", s.Name, s.Statistics.Observable, strings.Join(twins, " and of ")))
		}
	}
	return notes
}

// Repeats reports whether o stores the same summarised runs as s: the same name,
// the same statistics, and no observable both hold a different number of.
func (s Snapshot) Repeats(o Snapshot) bool {
	if s.Name != o.Name || s.Statistics == nil || o.Statistics == nil || *s.Statistics != *o.Statistics {
		return false
	}
	for k, v := range s.Values {
		if w, ok := o.Values[k]; ok && w != v {
			return false
		}
	}
	return true
}

// Summary counts what the sidecar indexes: configurations, those with snapshots,
// snapshots, and the runs they stand for when a snapshot summarises several.
func (r *Results) Summary() string {
	stored, snapshots := 0, 0
	var runs int64
	for _, c := range r.Configurations {
		if len(c.Snapshots) > 0 {
			stored++
		}
		snapshots += len(c.Snapshots)
		runs += c.StoredRuns()
	}
	out := fmt.Sprintf("results of %d run configuration(s): %d with %d stored snapshot(s)", len(r.Configurations), stored, snapshots)
	if runs != int64(snapshots) {
		out += fmt.Sprintf(" standing for %d run(s)", runs)
	}
	return out
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
