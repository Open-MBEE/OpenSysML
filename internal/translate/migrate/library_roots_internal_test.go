package migrate

import (
	"strings"
	"testing"
)

// TestLibraryRootRenamed covers top-level packages named like a standard
// library root package: each is written under a fresh name, references to its
// members follow the rename, a LibraryNameAvoided metadata line records the
// source name, and the report approximates it.
func TestLibraryRootRenamed(t *testing.T) {
	r, err := Migrate("roots.xmi", []byte(diagramModel(`
    <packagedElement xmi:type="uml:Package" xmi:id="_reqs" name="Requirements">
      <packagedElement xmi:type="uml:Class" xmi:id="_r1" name="R1"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Package" xmi:id="_views" name="Views"/>
    <packagedElement xmi:type="uml:Package" xmi:id="_use" name="Use">
      <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x" type="_r1"/>
      </packagedElement>
    </packagedElement>`, "")))
	if err != nil {
		t.Fatal(err)
	}
	got := string(r.Notation)
	for _, want := range []string{
		"package RequirementsModel {",
		"package ViewsModel;",
		"RequirementsModel::R1",
		`metadata MigrationMetadata::LibraryNameAvoided about RequirementsModel { sourceName = "Requirements"; }`,
		`metadata MigrationMetadata::LibraryNameAvoided about ViewsModel { sourceName = "Views"; }`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("notation lacks %q:\n%s", want, got)
		}
	}
	var found bool
	for _, e := range r.Report.Entries {
		if e.ID == "_reqs" {
			found = true
			if e.Verdict != Approximated {
				t.Errorf("Requirements verdict = %v, want Approximated", e.Verdict)
			}
			if !strings.Contains(e.Note, "written as RequirementsModel since a root package named Requirements would be hidden by the standard library's Requirements") {
				t.Errorf("Requirements note = %q", e.Note)
			}
		}
	}
	if !found {
		t.Error("no report entry for the Requirements package")
	}
}

// TestLibraryRootUnmappedUnannotated covers a colliding root the migration
// never writes as a declaration: renamed anyway, it still draws no
// LibraryNameAvoided line, which would name a declaration that does not exist.
func TestLibraryRootUnmappedUnannotated(t *testing.T) {
	r, err := Migrate("unmapped.xmi", []byte(diagramModel(`
    <packagedElement xmi:type="uml:Package" xmi:id="_reqs" name="Requirements"/>
    <packagedElement xmi:type="uml:Artifact" xmi:id="_art" name="Views"/>`, "")))
	if err != nil {
		t.Fatal(err)
	}
	got := string(r.Notation)
	if !strings.Contains(got, "about RequirementsModel") {
		t.Errorf("notation lacks the RequirementsModel metadata line:\n%s", got)
	}
	if strings.Contains(got, "ViewsModel") {
		t.Errorf("unmapped root annotated:\n%s", got)
	}
	for _, e := range r.Report.Entries {
		if e.ID == "_art" && e.Verdict != Unmapped {
			t.Errorf("artifact verdict = %v, want Unmapped", e.Verdict)
		}
	}
}

// TestLibraryRootNestedUnchanged covers a nested package named like a library
// root package: only top-level declarations hide the library, so it keeps its
// name and draws no metadata line.
func TestLibraryRootNestedUnchanged(t *testing.T) {
	r, err := Migrate("nested.xmi", []byte(diagramModel(`
    <packagedElement xmi:type="uml:Package" xmi:id="_p" name="P">
      <packagedElement xmi:type="uml:Package" xmi:id="_nested" name="Requirements">
        <packagedElement xmi:type="uml:Class" xmi:id="_r1" name="R1"/>
      </packagedElement>
    </packagedElement>`, "")))
	if err != nil {
		t.Fatal(err)
	}
	got := string(r.Notation)
	if !strings.Contains(got, "package Requirements {") {
		t.Errorf("nested Requirements package renamed:\n%s", got)
	}
	if strings.Contains(got, "LibraryNameAvoided") || strings.Contains(got, "RequirementsModel") {
		t.Errorf("nested package treated as a colliding root:\n%s", got)
	}
}
