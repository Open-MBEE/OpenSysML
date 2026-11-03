package formal

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
)

// fixtureText is the shape `pdftotext -layout` gives a document: a table of
// contents, running footers, page breaks before headings, a metaclass clause
// with every subsection, a constraint spanning a page, prose glued to its OCL,
// an OCL-only constraint, a TBD placeholder and a heading with a stray space.
const fixtureText = `                      8.3 Abstract Syntax ..................................... 100
                      8.3.2.1.2 Element ....................................... 101
8.2 Textual Notation

checkNotAConstraint

8.3 Abstract Syntax

8.3.2.1.2 Element

Description

An Element is a constituent of a model.

Attributes

owner : Element [0..1]

Operations

escapedName() : String [0..1]

Constraints

deriveElementName

The name of an Element is given by the result of the effectiveName() operation.

name = effectiveName()

validateElementOwner

An Element must have an owner unless it is a root
Namespace.

owner <> null or
Kernel Modeling Language v1.0                                                   102
` + "\f" + `103                                                   Kernel Modeling Language v1.1, Beta 2

    oclIsKindOf(Namespace)

8.3.2.1.3 Enumeration

Description

General Classes

` + "\f" + `8.3.4.5.2 Binding Connector

Description

Constraints

checkBindingConnectorSpecialization

A BindingConnector must directly or indirectly specialize Links::selfLinks from the
Kernel Semantic Library.
specializesFromLibrary('Links::selfLinks')

validateBindingConnectorIsBinary

relatedFeature->size() = 2

checkBindingConnectorTBD

A placeholder for OCL the document does not give.

TBD

8.4 Semantics

validateNotInAbstractSyntax

Not a constraint.
`

func TestParseReadsTheConstraintsSubsections(t *testing.T) {
	got, err := Parse(fixtureText)
	if err != nil {
		t.Fatal(err)
	}
	want := []Constraint{
		{Clause: "8.3.2.1.2", Metaclass: "Element", Kind: "derive", Name: "deriveElementName",
			Checks: "The name of an Element is given by the result of the effectiveName() operation.", OCL: "name = effectiveName()"},
		{Clause: "8.3.2.1.2", Metaclass: "Element", Kind: "validate", Name: "validateElementOwner",
			Checks: "An Element must have an owner unless it is a root Namespace.", OCL: "owner <> null or\n\n    oclIsKindOf(Namespace)"},
		{Clause: "8.3.4.5.2", Metaclass: "BindingConnector", Kind: "check", Name: "checkBindingConnectorSpecialization",
			Checks: "A BindingConnector must directly or indirectly specialize Links::selfLinks from the Kernel Semantic Library.", OCL: "specializesFromLibrary('Links::selfLinks')"},
		{Clause: "8.3.4.5.2", Metaclass: "BindingConnector", Kind: "validate", Name: "validateBindingConnectorIsBinary",
			Checks: "relatedFeature->size() = 2", OCL: "relatedFeature->size() = 2"},
		{Clause: "8.3.4.5.2", Metaclass: "BindingConnector", Kind: "check", Name: "checkBindingConnectorTBD",
			Checks: "A placeholder for OCL the document does not give.", OCL: "TBD"},
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %d constraints, want %d:\n%s", len(got), len(want), encode(t, got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("constraint %d:\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}
	again, err := Parse(fixtureText)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encode(t, got), encode(t, again)) {
		t.Fatal("two parses of the same text differ")
	}
}

func TestParseRefusesADocumentWithoutTheClause(t *testing.T) {
	if _, err := Parse("8.2 Textual Notation\n\nvalidateX\n"); err == nil || !strings.Contains(err.Error(), "8.3") {
		t.Fatalf("got %v, want the missing clause named", err)
	}
	if _, err := Parse(strings.Replace(fixtureText, "8.4 Semantics", "", 1)); err == nil || !strings.Contains(err.Error(), "8.4") {
		t.Fatalf("got %v, want the missing end named", err)
	}
	twice := strings.Replace(fixtureText, "checkBindingConnectorTBD", "deriveElementName", 1)
	if _, err := Parse(twice); err == nil || !strings.Contains(err.Error(), "deriveElementName is stated twice") {
		t.Fatalf("got %v, want the duplicate named", err)
	}
}

func TestExtractRefusesAnotherDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(path, []byte("not the pinned document"), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := manifest.Specification{Name: "KerML", Document: "formal/26-03-01", SHA256: strings.Repeat("0", 64)}
	_, err := Extract("pdftotext", path, "kerml", spec)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") || !strings.Contains(err.Error(), "formal/26-03-01") {
		t.Fatalf("got %v, want the digest mismatch named with the document", err)
	}
}

func fixtureManifest() *manifest.Manifest {
	return &manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion, PilotTag: "t", PilotCommit: "c", PilotArtifact: "a",
		Jar:        manifest.JarRecord{Name: "k.jar", Digest: "sha256:abc"},
		Extraction: manifest.ExtractionRecord{Classes: []string{"a"}, Method: "m"},
		Specifications: map[string]manifest.Specification{
			"kerml": {Name: "KerML", Version: "1.0", Document: "formal/26-03-01", URL: "https://example.test", SHA256: strings.Repeat("0", 64)},
		},
		Constraints: []manifest.Constraint{
			{Name: "validateBindingConnectorIsBinary", Source: "kerml", Kind: "validate", Metaclass: "BindingConnector",
				Spec:   &manifest.SpecRef{Document: "kerml", Clause: "8.3.4.5.2", Name: "validateBindingConnectorIsBinary"},
				Checks: "binary", Implementation: "internal/check/passes/a.go:APass.Run", Message: "—", NegativeCase: "none",
				Codes: []string{"a"}, Passes: []string{"constraint"}, Status: manifest.StatusFaithful},
			{Name: "validateElementOwnerRenamed", Source: "kerml", Kind: "validate", Metaclass: "Element",
				Spec:   &manifest.SpecRef{Document: "kerml", Clause: "8.3.2.1.2", Name: "validateElementOwner"},
				Checks: "owned", Implementation: "—", Message: "—", NegativeCase: "none",
				Codes: []string{}, Passes: []string{}, Status: manifest.StatusUnknown, Rationale: "no pass yet"},
		},
		Extensions: []manifest.Extension{}, Errata: []manifest.Erratum{}, SpecIssues: []manifest.SpecIssue{},
	}
}

func parsedFixture(t *testing.T) []Constraint {
	t.Helper()
	extracted, err := Parse(fixtureText)
	if err != nil {
		t.Fatal(err)
	}
	for i := range extracted {
		extracted[i].Document = "kerml"
	}
	return extracted
}

// TestMergeRecordsWhatThePilotDoesNotName: a constraint a pilot row cites by
// the document's name is left to that row; every other one becomes a
// spec-sourced unknown row after the pilot's rows; a second merge is a no-op.
func TestMergeRecordsWhatThePilotDoesNotName(t *testing.T) {
	m := fixtureManifest()
	extracted := parsedFixture(t)
	next, changed, problems := Merge(m, extracted)
	if !changed || len(problems) != 0 {
		t.Fatalf("changed %v, problems %q", changed, problems)
	}
	if err := next.Validate(); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range next.SpecRows() {
		names = append(names, c.Name)
		if c.Status != manifest.StatusUnknown || c.Implementation != "—" || c.Rationale == "" || c.Spec == nil || c.Spec.Name != c.Name {
			t.Errorf("%s is recorded as %+v", c.Name, c)
		}
	}
	if got, want := strings.Join(names, ","), "checkBindingConnectorSpecialization,checkBindingConnectorTBD,deriveElementName"; got != want {
		t.Fatalf("spec-sourced rows %s, want %s", got, want)
	}
	if got := len(next.PilotRows()); got != 2 {
		t.Fatalf("%d pilot rows, want the 2 recorded", got)
	}
	if next.FormalExtraction == nil || next.FormalExtraction.Constraints["kerml"] != 5 || next.FormalExtraction.Method != Method {
		t.Fatalf("extraction record %+v", next.FormalExtraction)
	}
	if _, changed, _ := Merge(next, extracted); changed {
		t.Fatal("merging the same extraction again changed the manifest")
	}
}

// TestMergeKeepsAPromotedRow: the hand-edited fields of a recorded spec-sourced
// row survive a re-extraction; its text, clause and metaclass are refreshed.
func TestMergeKeepsAPromotedRow(t *testing.T) {
	m := fixtureManifest()
	extracted := parsedFixture(t)
	first, _, _ := Merge(m, extracted)
	for i := range first.Constraints {
		if first.Constraints[i].Name == "deriveElementName" {
			first.Constraints[i].Implementation = "internal/semantic/a.go:Name"
			first.Constraints[i].Status = manifest.StatusFaithful
			first.Constraints[i].Rationale = ""
			first.Constraints[i].Checks = "stale text"
		}
	}
	next, changed, _ := Merge(first, extracted)
	if !changed {
		t.Fatal("the stale text was not refreshed")
	}
	row, ok := next.Constraint("deriveElementName")
	if !ok || row.Implementation != "internal/semantic/a.go:Name" || row.Status != manifest.StatusFaithful || !strings.HasPrefix(row.Checks, "The name of an Element") {
		t.Fatalf("promoted row after re-extraction: %+v", row)
	}
}

// TestMergeReportsAPilotRowTheDocumentsContradict: a pilot row citing a name
// the documents do not state, or another clause than the one stating it, is
// named as a disagreement.
func TestMergeReportsAPilotRowTheDocumentsContradict(t *testing.T) {
	m := fixtureManifest()
	m.Constraints[0].Spec.Clause = "8.3.2.1.2"
	m.Constraints[1].Spec.Name = "validateElementOwnerRenamed"
	_, _, problems := Merge(m, parsedFixture(t))
	joined := strings.Join(problems, "\n")
	if len(problems) != 2 || !strings.Contains(joined, "validateBindingConnectorIsBinary cites kerml clause 8.3.2.1.2, the documents state validateBindingConnectorIsBinary in kerml clause 8.3.4.5.2") ||
		!strings.Contains(joined, "validateElementOwnerRenamed cites the formal name validateElementOwnerRenamed, which the documents do not state") {
		t.Fatalf("problems:\n%s", joined)
	}
}

// TestMainListUpdateAndCheck drives the command end to end over a fake
// pdftotext that prints the fixture text: -list is deterministic, -update
// records the rows, -check passes afterwards and fails on a hand-edited row.
func TestMainListUpdateAndCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake pdftotext is a shell script")
	}
	root := t.TempDir()
	m := fixtureManifest()
	pdf := filepath.Join(root, "kerml.pdf")
	if err := os.WriteFile(pdf, []byte("pinned"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := digestOf(pdf)
	if err != nil {
		t.Fatal(err)
	}
	spec := m.Specifications["kerml"]
	spec.SHA256 = digest
	m.Specifications["kerml"] = spec
	if err := os.MkdirAll(filepath.Join(root, "docs", "project"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := manifest.Write(root, m); err != nil {
		t.Fatal(err)
	}
	text := filepath.Join(root, "kerml.txt")
	if err := os.WriteFile(text, []byte(fixtureText), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(root, "pdftotext")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\ncat \""+text+"\"\n"), 0o700); err != nil { // #nosec G306 -- an executable fixture
		t.Fatal(err)
	}
	run := func(mode string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := Main([]string{"-repo", root, "-pdftotext", fake, "-kerml", pdf, mode}, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	code, list1, stderr := run("-list")
	if code != 0 {
		t.Fatalf("-list exited %d: %s", code, stderr)
	}
	if _, list2, _ := run("-list"); list1 != list2 {
		t.Fatal("two -list runs differ")
	}
	var listed []Constraint
	if err := json.Unmarshal([]byte(list1), &listed); err != nil || len(listed) != 5 || listed[0].Document != "kerml" {
		t.Fatalf("-list printed %d constraints (%v):\n%s", len(listed), err, list1)
	}
	if code, _, stderr := run("-check"); code == 0 || !strings.Contains(stderr, "deriveElementName is stated by the documents but not recorded") {
		t.Fatalf("-check before -update exited %d: %s", code, stderr)
	}
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifest.Path)))
	if err != nil {
		t.Fatal(err)
	}
	m.Constraints[0].Spec.Clause = "8.3.2.1.2"
	if err := manifest.Write(root, m); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run("-update"); code == 0 || !strings.Contains(stderr, "validateBindingConnectorIsBinary cites kerml clause 8.3.2.1.2") || !strings.Contains(stderr, "nothing written") {
		t.Fatalf("-update over a contradicted pilot row exited %d: %s", code, stderr)
	}
	if recorded, err := manifest.Load(root); err != nil || len(recorded.SpecRows()) != 0 {
		t.Fatalf("-update wrote spec rows despite the disagreement (%v)", err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(manifest.Path)), before, 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, stderr := run("-update"); code != 0 || !strings.Contains(out, "5 constraints: kerml 5 (2 check*, 1 derive*, 2 validate*); 3 spec-sourced rows recorded") {
		t.Fatalf("-update exited %d: %s%s", code, out, stderr)
	}
	if code, out, stderr := run("-check"); code != 0 || !strings.Contains(out, "3 spec-sourced rows are current") {
		t.Fatalf("-check after -update exited %d: %s%s", code, out, stderr)
	}
	recorded, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for i := range recorded.Constraints {
		if recorded.Constraints[i].Name == "checkBindingConnectorTBD" {
			recorded.Constraints[i].Checks = "edited by hand"
		}
	}
	if err := manifest.Write(root, recorded); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run("-check"); code == 0 || !strings.Contains(stderr, "checkBindingConnectorTBD is recorded with another text") {
		t.Fatalf("-check on an edited row exited %d: %s", code, stderr)
	}
	var stderrBuf bytes.Buffer
	if code := Main([]string{"-repo", root, "-pdftotext", fake, "-kerml", pdf, "-list", "-check"}, &bytes.Buffer{}, &stderrBuf); code != 2 || !strings.Contains(stderrBuf.String(), "exactly one of") {
		t.Fatalf("two modes exited %d: %s", code, stderrBuf.String())
	}
}

func encode(t *testing.T, constraints []Constraint) []byte {
	t.Helper()
	b, err := json.Marshal(constraints)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
