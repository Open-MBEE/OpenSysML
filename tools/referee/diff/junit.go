package diff

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/junit"
)

// junitReport renders the comparison as JUnit XML: one suite per corpus root,
// one case per file that drew a diagnostic on either side. A file disagreeing
// in any bucket fails; files silent on both sides are omitted.
func junitReport(report *Report) *junit.Testsuites {
	doc := &junit.Testsuites{Name: "pilot-diff"}
	for _, root := range report.Roots {
		suite := junit.Testsuite{Name: root.Name}
		for _, file := range root.Files {
			suite.AddCase(junitFileCase(root, file))
		}
		doc.AddSuite(suite)
	}
	return doc
}

func junitFileCase(root RootReport, file FileReport) junit.Testcase {
	c := junit.Testcase{
		Name:      file.Path,
		Classname: root.Name,
		File:      root.Dir + "/" + file.Path,
	}
	var rows []string
	for _, e := range file.SeverityMismatch {
		rows = append(rows, fmt.Sprintf("line %d %s: ours %s, pilot %s (x%d)",
			e.Line, e.Category, e.OpenSysML, e.Pilot, e.Count))
	}
	for _, e := range file.OpenSysMLOnly {
		rows = append(rows, fmt.Sprintf("line %d %s %s: only OpenSysML (x%d)",
			e.Line, e.Severity, e.Category, e.Count))
	}
	for _, e := range file.PilotOnly {
		rows = append(rows, fmt.Sprintf("line %d %s %s: only the pilot (x%d)",
			e.Line, e.Severity, e.Category, e.Count))
	}
	if len(rows) > 0 {
		c.Failure = &junit.Message{
			Text: fmt.Sprintf("%d disagreeing diagnostic group(s)", len(rows)),
			Body: strings.Join(rows, "\n"),
		}
	}
	return c
}
