// Package sarif renders findings in the SARIF 2.1.0 format that code-scanning
// services (GitHub, GitLab, Azure DevOps) ingest. Only the properties a report
// populates are modelled, so the output stays deterministic and diffs cleanly.
package sarif

import (
	"encoding/json"
	"sort"
)

// Version is the SARIF version every log this package writes declares, and
// Schema the published schema of that version.
const (
	Version = "2.1.0"
	Schema  = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"
)

// Log is the document root: one or more runs of a tool.
type Log struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []Run  `json:"runs"`
}

// Run is one invocation of one tool and the results it produced.
type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
	// ColumnKind is what the columns count: "utf16CodeUnits" (the default
	// readers assume) or "unicodeCodePoints".
	ColumnKind string      `json:"columnKind,omitempty"`
	Properties *Properties `json:"properties,omitempty"`
}

// Properties is a run's property bag: what the run reports about itself
// beyond its results.
type Properties struct {
	// Suppressed is how many findings a baseline kept out of the results.
	Suppressed int `json:"suppressed,omitempty"`
}

// Tool names the analysis tool a run was made by.
type Tool struct {
	Driver Driver `json:"driver"`
}

// Driver is the tool's primary component: its name, version and rules.
type Driver struct {
	Name           string `json:"name"`
	InformationURI string `json:"informationUri,omitempty"`
	Version        string `json:"version,omitempty"`
	Rules          []Rule `json:"rules"`
}

// Rule describes one kind of finding, keyed by the id its results carry.
type Rule struct {
	ID               string   `json:"id"`
	ShortDescription Message  `json:"shortDescription"`
	HelpURI          string   `json:"helpUri,omitempty"`
	Properties       *RuleSet `json:"properties,omitempty"`
}

// RuleSet carries a rule's property bag: the tags a consumer filters on.
type RuleSet struct {
	Tags []string `json:"tags,omitempty"`
}

// Result is one finding.
type Result struct {
	RuleID    string     `json:"ruleId"`
	RuleIndex int        `json:"ruleIndex"`
	Level     string     `json:"level"`
	Message   Message    `json:"message"`
	Locations []Location `json:"locations,omitempty"`
	Fixes     []Fix      `json:"fixes,omitempty"`
}

// Message is a plain-text message.
type Message struct {
	Text string `json:"text"`
}

// Location places a result in an artifact.
type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

// PhysicalLocation is an artifact and a region within it.
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           *Region          `json:"region,omitempty"`
}

// ArtifactLocation names an artifact by URI, relative to the uriBaseId when
// one is given.
type ArtifactLocation struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"`
}

// Region is a 1-based span of an artifact's text. A zero EndLine or EndColumn
// is omitted, leaving the region to end where the format says a region without
// an end does.
type Region struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn,omitempty"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}

// Fix is one way to resolve a result: a set of edits to artifacts.
type Fix struct {
	Description     Message          `json:"description"`
	ArtifactChanges []ArtifactChange `json:"artifactChanges"`
}

// ArtifactChange is the replacements a fix makes in one artifact.
type ArtifactChange struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Replacements     []Replacement    `json:"replacements"`
}

// Replacement deletes a region and inserts content in its place; an insertion
// deletes an empty region.
type Replacement struct {
	DeletedRegion   Region           `json:"deletedRegion"`
	InsertedContent *ArtifactContent `json:"insertedContent,omitempty"`
}

// ArtifactContent is text inserted by a replacement.
type ArtifactContent struct {
	Text string `json:"text"`
}

// Level maps a validator severity (error, warning, info, hint) onto the SARIF
// levels error, warning and note.
func Level(severity string) string {
	switch severity {
	case "error", "warning":
		return severity
	}
	return "note"
}

// NewLog returns a log of one run by the named tool, with no results yet.
func NewLog(driver Driver) *Log {
	if driver.Rules == nil {
		driver.Rules = []Rule{}
	}
	return &Log{
		Schema:  Schema,
		Version: Version,
		Runs:    []Run{{Tool: Tool{Driver: driver}, Results: []Result{}}},
	}
}

// LocationAt places a result at a 1-based line and column of an artifact; a
// column of 0 leaves the region a whole line. A whole-file finding, reported at
// line 0, is placed on the first line, as SARIF regions start at 1.
func LocationAt(uri string, line, column int) []Location {
	if line < 1 {
		line = 1
	}
	return []Location{{PhysicalLocation: PhysicalLocation{
		ArtifactLocation: ArtifactLocation{URI: uri},
		Region:           &Region{StartLine: line, StartColumn: column},
	}}}
}

// SortResults orders results by artifact, line, column and rule, so that a log
// of the same findings is the same document whatever order they were found in.
func SortResults(results []Result) {
	sort.SliceStable(results, func(i, j int) bool {
		a, b := firstLocation(results[i]), firstLocation(results[j])
		if a.ArtifactLocation.URI != b.ArtifactLocation.URI {
			return a.ArtifactLocation.URI < b.ArtifactLocation.URI
		}
		al, bl := regionOf(a.Region), regionOf(b.Region)
		if al.StartLine != bl.StartLine {
			return al.StartLine < bl.StartLine
		}
		if al.StartColumn != bl.StartColumn {
			return al.StartColumn < bl.StartColumn
		}
		return results[i].RuleID < results[j].RuleID
	})
}

// firstLocation is where a result is placed, an unplaced result sorting first.
func firstLocation(r Result) PhysicalLocation {
	if len(r.Locations) == 0 {
		return PhysicalLocation{}
	}
	return r.Locations[0].PhysicalLocation
}

func regionOf(r *Region) Region {
	if r == nil {
		return Region{}
	}
	return *r
}

// Marshal renders the log as indented JSON ending in a newline.
func Marshal(log *Log) ([]byte, error) {
	encoded, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
