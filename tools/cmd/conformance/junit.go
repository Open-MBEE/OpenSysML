package main

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/junit"
)

// junitReport renders the report as JUnit XML: one suite per configuration and
// protocol, one case per scenario.
func junitReport(report *Report) *junit.Testsuites {
	doc := &junit.Testsuites{Name: "conformance"}
	for _, config := range report.Configurations {
		for _, protocol := range config.Protocols {
			suite := junit.Testsuite{Name: config.Name + "/" + protocol.Protocol}
			for _, result := range protocol.Results {
				suite.AddCase(junitCase(suite.Name, result))
			}
			doc.AddSuite(suite)
		}
	}
	return doc
}

func junitCase(suiteName string, result *Result) junit.Testcase {
	c := junit.Testcase{
		Name:      result.ID,
		Classname: suiteName,
		Time:      result.DurationMS / 1000,
	}
	switch result.Outcome {
	case "fail":
		c.Failure = &junit.Message{Text: result.Reason, Body: strings.Join(result.Failures, "\n")}
	case "error":
		c.Error = &junit.Message{Text: result.Reason, Body: strings.Join(result.Failures, "\n")}
	case "skip":
		c.Skipped = &junit.Message{Text: result.Reason}
	}
	return c
}
