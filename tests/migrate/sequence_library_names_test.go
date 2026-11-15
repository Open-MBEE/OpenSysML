package migrate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func TestSequenceNamesFollowLibraryOverride(t *testing.T) {
	const source = `
<packagedElement xmi:type="uml:Class" xmi:id="_block" name="Block">
  <ownedBehavior xmi:type="uml:Interaction" xmi:id="_seq" name="Sequence">
    <ownedAttribute xmi:type="uml:Property" xmi:id="_property" name="libraryMember" type="_block"/>
    <lifeline xmi:type="uml:Lifeline" xmi:id="_line" represents="_property"/>
  </ownedBehavior>
</packagedElement>`
	const profile = `<sysml:Block xmi:id="_st" base_Class="_block"/>`
	t.Setenv(libs.LibraryPathEnvVar, "")
	migrateDocument(t, source, profile)
	previous := libs.SharedBase()
	for _, member := range []string{"libraryMember", "otherMember"} {
		dir := t.TempDir()
		text := "package Occurrences { abstract class Occurrence { feature " + member + "; } }"
		if err := os.WriteFile(filepath.Join(dir, "Occurrences.kerml"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv(libs.LibraryPathEnvVar, dir)
		idx := libs.SharedBase()
		if idx == previous || len(idx.LookupQualified("Occurrences::Occurrence::"+member)) != 1 {
			t.Fatal("library override did not load a distinct index with its declared member")
		}
		previous = idx
		r := migrateDocument(t, source, profile)
		want := "ref part libraryMember : Block;"
		if member == "libraryMember" {
			want = "ref part libraryMember2 : Block;"
		}
		wantLine(t, r.Notation, want)
	}
}
