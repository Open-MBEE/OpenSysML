package diff

import (
	"encoding/json"
	"fmt"
	"sort"
)

// SARIF 2.1.0 subset: only the properties this report populates, so the output
// stays deterministic and diffs cleanly between runs.
type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri"`
	Version        string      `json:"version"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string       `json:"id"`
	ShortDescription sarifMessage `json:"shortDescription"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

// The three disagreement buckets, as SARIF rules. Agreement rows are not
// findings and are not emitted.
var sarifRules = []sarifRule{
	{ID: "only-opensysml", ShortDescription: sarifMessage{Text: "A diagnostic only OpenSysML reports (candidate false positive)."}},
	{ID: "only-pilot", ShortDescription: sarifMessage{Text: "A diagnostic only the pilot reports (candidate gap)."}},
	{ID: "severity-mismatch", ShortDescription: sarifMessage{Text: "Both report the same line and category with different severities."}},
}

// sarifReport renders the disagreements as SARIF 2.1.0, one result per
// disagreeing diagnostic group, located on the compared model file.
func sarifReport(report *Report) *sarifLog {
	run := sarifRun{
		Tool: sarifTool{Driver: sarifDriver{
			Name:           "pilot-diff",
			InformationURI: "https://github.com/Open-MBEE/OpenSysML/tree/main/tools/referee/diff",
			Version:        report.Pilot,
			Rules:          sarifRules,
		}},
		Results: []sarifResult{},
	}
	for _, root := range report.Roots {
		for _, file := range root.Files {
			uri := root.Dir + "/" + file.Path
			for _, e := range file.SeverityMismatch {
				run.Results = append(run.Results, sarifResult{
					RuleID: "severity-mismatch",
					Level:  sarifLevel(e.OpenSysML),
					Message: sarifMessage{Text: fmt.Sprintf(
						"%s: OpenSysML reports %s, the pilot %s (x%d)%s",
						e.Category, e.OpenSysML, e.Pilot, e.Count, exampleSuffix(e.Examples))},
					Locations: sarifLocations(uri, e.Line),
				})
			}
			for _, e := range file.OpenSysMLOnly {
				run.Results = append(run.Results, sarifEntry("only-opensysml", "only OpenSysML reports", uri, e))
			}
			for _, e := range file.PilotOnly {
				run.Results = append(run.Results, sarifEntry("only-pilot", "only the pilot reports", uri, e))
			}
		}
	}
	sortSarifResults(run.Results)
	return &sarifLog{
		Version: "2.1.0",
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Runs:    []sarifRun{run},
	}
}

func sarifEntry(rule, side, uri string, e Entry) sarifResult {
	return sarifResult{
		RuleID: rule,
		Level:  sarifLevel(e.Severity),
		Message: sarifMessage{Text: fmt.Sprintf("%s: %s a %s here (x%d)%s",
			e.Category, side, e.Severity, e.Count, exampleSuffix(e.Examples))},
		Locations: sarifLocations(uri, e.Line),
	}
}

func sarifLocations(uri string, line int) []sarifLocation {
	// SARIF regions are 1-based; a whole-file diagnostic reports line 0.
	if line < 1 {
		line = 1
	}
	return []sarifLocation{{PhysicalLocation: sarifPhysicalLocation{
		ArtifactLocation: sarifArtifactLocation{URI: uri},
		Region:           sarifRegion{StartLine: line},
	}}}
}

// sarifLevel maps a validator severity onto SARIF's error/warning/note levels.
func sarifLevel(severity string) string {
	switch severity {
	case "error", "warning":
		return severity
	}
	return "note"
}

func exampleSuffix(examples []string) string {
	if len(examples) == 0 {
		return ""
	}
	return ": " + examples[0]
}

func sortSarifResults(results []sarifResult) {
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i].Locations[0].PhysicalLocation, results[j].Locations[0].PhysicalLocation
		if a.ArtifactLocation.URI != b.ArtifactLocation.URI {
			return a.ArtifactLocation.URI < b.ArtifactLocation.URI
		}
		if a.Region.StartLine != b.Region.StartLine {
			return a.Region.StartLine < b.Region.StartLine
		}
		return results[i].RuleID < results[j].RuleID
	})
}

func marshalSarif(log *sarifLog) ([]byte, error) {
	encoded, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
