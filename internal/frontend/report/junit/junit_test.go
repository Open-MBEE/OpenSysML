package junit

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountsFold(t *testing.T) {
	var suite Testsuite
	suite.Name = "s"
	suite.AddCase(Testcase{Name: "pass", Time: 0.5})
	suite.AddCase(Testcase{Name: "fail", Failure: &Message{Text: "boom"}})
	suite.AddCase(Testcase{Name: "err", Error: &Message{Text: "broken"}})
	suite.AddCase(Testcase{Name: "skip", Skipped: &Message{Text: "missing capability"}})

	if suite.Tests != 4 || suite.Failures != 1 || suite.Errors != 1 || suite.Skipped != 1 {
		t.Fatalf("suite counts = %d/%d/%d/%d, want 4/1/1/1",
			suite.Tests, suite.Failures, suite.Errors, suite.Skipped)
	}

	var doc Testsuites
	doc.AddSuite(suite)
	doc.AddSuite(Testsuite{Tests: 2, Failures: 1, Time: 1})
	if doc.Tests != 6 || doc.Failures != 2 || doc.Errors != 1 || doc.Skipped != 1 {
		t.Fatalf("document counts = %d/%d/%d/%d, want 6/2/1/1",
			doc.Tests, doc.Failures, doc.Errors, doc.Skipped)
	}
	if doc.Time != 1.5 {
		t.Fatalf("document time = %v, want 1.5", doc.Time)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	doc := &Testsuites{Name: "conformance"}
	suite := Testsuite{Name: "default/grpc"}
	suite.AddCase(Testcase{Name: "parse-01", Classname: "default/grpc", Time: 0.012})
	suite.AddCase(Testcase{Name: "verify-02", Failure: &Message{Text: "status mismatch", Body: "want OK, got INVALID_ARGUMENT"}})
	doc.AddSuite(suite)

	data, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), xml.Header) {
		t.Fatalf("output lacks the XML declaration:\n%s", data)
	}

	var decoded Testsuites
	if err := xml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output does not parse back: %v", err)
	}
	if decoded.Tests != 2 || decoded.Failures != 1 {
		t.Fatalf("round trip counts = %d/%d, want 2/1", decoded.Tests, decoded.Failures)
	}
	if decoded.Suites[0].Cases[1].Failure == nil ||
		decoded.Suites[0].Cases[1].Failure.Text != "status mismatch" {
		t.Fatalf("round trip lost the failure: %+v", decoded.Suites[0].Cases[1])
	}
	if decoded.Suites[0].Cases[0].Failure != nil {
		t.Fatalf("passing case grew a failure: %+v", decoded.Suites[0].Cases[0])
	}
}

func TestWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.xml")
	if err := WriteFile(path, &Testsuites{Name: "x"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `<testsuites name="x"`) {
		t.Fatalf("unexpected file content:\n%s", data)
	}
}
