package diff

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/report/sarif"
)

// The three disagreement buckets, as SARIF rules. Agreement rows are not
// findings and are not emitted.
var sarifRules = []sarif.Rule{
	{ID: "only-opensysml", ShortDescription: sarif.Message{Text: "A diagnostic only OpenSysML reports (candidate false positive)."}},
	{ID: "only-pilot", ShortDescription: sarif.Message{Text: "A diagnostic only the pilot reports (candidate gap)."}},
	{ID: "severity-mismatch", ShortDescription: sarif.Message{Text: "Both report the same line and category with different severities."}},
}

var sarifRuleIndex = map[string]int{"only-opensysml": 0, "only-pilot": 1, "severity-mismatch": 2}

// sarifReport renders the disagreements as SARIF 2.1.0, one result per
// disagreeing diagnostic group, located on the compared model file.
func sarifReport(report *Report) *sarif.Log {
	log := sarif.NewLog(sarif.Driver{
		Name:           "pilot-diff",
		InformationURI: "https://github.com/Open-MBEE/OpenSysML/tree/main/tools/referee/diff",
		Version:        report.Pilot,
		Rules:          sarifRules,
	})
	run := &log.Runs[0]
	for _, root := range report.Roots {
		for _, file := range root.Files {
			uri := root.Dir + "/" + file.Path
			for _, e := range file.SeverityMismatch {
				run.Results = append(run.Results, sarif.Result{
					RuleID:    "severity-mismatch",
					RuleIndex: sarifRuleIndex["severity-mismatch"],
					Level:     sarif.Level(e.OpenSysML),
					Message: sarif.Message{Text: fmt.Sprintf(
						"%s: OpenSysML reports %s, the pilot %s (x%d)%s",
						e.Category, e.OpenSysML, e.Pilot, e.Count, exampleSuffix(e.Examples))},
					Locations: sarif.LocationAt(uri, e.Line, 0),
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
	sarif.SortResults(run.Results)
	return log
}

func sarifEntry(rule, side, uri string, e Entry) sarif.Result {
	return sarif.Result{
		RuleID:    rule,
		RuleIndex: sarifRuleIndex[rule],
		Level:     sarif.Level(e.Severity),
		Message: sarif.Message{Text: fmt.Sprintf("%s: %s a %s here (x%d)%s",
			e.Category, side, e.Severity, e.Count, exampleSuffix(e.Examples))},
		Locations: sarif.LocationAt(uri, e.Line, 0),
	}
}

func exampleSuffix(examples []string) string {
	if len(examples) == 0 {
		return ""
	}
	return ": " + examples[0]
}
