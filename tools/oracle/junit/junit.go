// Package junit renders test results in the JUnit XML format that CI systems
// (CircleCI, Jenkins, GitLab, GitHub) render natively as test reports.
package junit

import (
	"encoding/xml"
	"fmt"
	"os"
)

// Testsuites is the document root: one <testsuites> holding one suite per
// logical grouping of cases.
type Testsuites struct {
	XMLName  xml.Name    `xml:"testsuites"`
	Name     string      `xml:"name,attr,omitempty"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Errors   int         `xml:"errors,attr"`
	Skipped  int         `xml:"skipped,attr"`
	Time     float64     `xml:"time,attr"`
	Suites   []Testsuite `xml:"testsuite"`
}

// Testsuite is one group of test cases.
type Testsuite struct {
	Name     string     `xml:"name,attr"`
	Tests    int        `xml:"tests,attr"`
	Failures int        `xml:"failures,attr"`
	Errors   int        `xml:"errors,attr"`
	Skipped  int        `xml:"skipped,attr"`
	Time     float64    `xml:"time,attr"`
	Cases    []Testcase `xml:"testcase"`
}

// Testcase is one case: passing when Failure, Error and Skipped are all nil.
type Testcase struct {
	Name      string   `xml:"name,attr"`
	Classname string   `xml:"classname,attr,omitempty"`
	File      string   `xml:"file,attr,omitempty"`
	Time      float64  `xml:"time,attr"`
	Failure   *Message `xml:"failure,omitempty"`
	Error     *Message `xml:"error,omitempty"`
	Skipped   *Message `xml:"skipped,omitempty"`
}

// Message carries a case's failure, error or skip reason.
type Message struct {
	Text string `xml:"message,attr,omitempty"`
	Body string `xml:",chardata"`
}

// AddSuite appends a suite and folds its counts into the document totals.
func (t *Testsuites) AddSuite(suite Testsuite) {
	t.Tests += suite.Tests
	t.Failures += suite.Failures
	t.Errors += suite.Errors
	t.Skipped += suite.Skipped
	t.Time += suite.Time
	t.Suites = append(t.Suites, suite)
}

// AddCase appends a case and folds it into the suite counts.
func (s *Testsuite) AddCase(c Testcase) {
	s.Tests++
	s.Time += c.Time
	switch {
	case c.Error != nil:
		s.Errors++
	case c.Failure != nil:
		s.Failures++
	case c.Skipped != nil:
		s.Skipped++
	}
	s.Cases = append(s.Cases, c)
}

// Marshal renders the document with the XML declaration CI parsers expect.
func Marshal(t *Testsuites) ([]byte, error) {
	body, err := xml.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(xml.Header)+len(body)+1)
	out = append(out, xml.Header...)
	out = append(out, body...)
	out = append(out, '\n')
	return out, nil
}

// WriteFile renders the document to path.
func WriteFile(path string, t *Testsuites) error {
	data, err := Marshal(t)
	if err != nil {
		return fmt.Errorf("junit: %w", err)
	}
	return os.WriteFile(path, data, 0o644) // #nosec G306 -- a CI artifact is meant to be readable.
}
