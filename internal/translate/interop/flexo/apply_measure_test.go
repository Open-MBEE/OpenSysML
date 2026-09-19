package flexo

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// These run everywhere; the stack itself is measured by TestFlexoInteropApply.

func TestApplyReportTextIsDeterministic(t *testing.T) {
	report := &ApplyReport{
		Fixture:  "identity_model.sysml",
		Revision: "identity_model_revised.sysml",
		Rounds: []ApplyRound{
			{Name: "initial", Creates: 2, Applied: true, Commits: 1, Advanced: true, Readable: 2,
				Changes: []string{"create\tA\tPackage", "create\tB\tPartDefinition"},
				Rediff:  "creates=0\tupdates=0\tdeletes=0\tconflicts=0"},
			{Name: "revision", Base: true, Updates: 1, Deletes: 1, Refused: "1 delete(s) need explicit confirmation",
				Applied: true, Commits: 2, Advanced: true, Readable: 1, Emptied: 1,
				Changes:  []string{"update\tB\tPartDefinition", "delete\tA\tPackage"},
				ReadBack: []string{"B\tdeclaredName=\"Cell\""},
				Rediff:   "creates=0\tupdates=0\tdeletes=0\tconflicts=0"},
		},
		Findings: []string{"initial: 2 create(s)"},
	}
	first := report.Text("# header\n")
	if second := report.Text("# header\n"); first != second {
		t.Fatalf("the report renders differently on a second call:\n%s\n---\n%s", first, second)
	}
	for _, want := range []string{
		"\n[apply]\nfixture\tidentity_model.sysml\nrevision\tidentity_model_revised.sysml\n",
		"\n[apply.initial]\nbase\tno\ndiff\tcreates=2\tupdates=0\tdeletes=0\tconflicts=0\nuncarried\t-\nrefused\t-\napplied\tyes\ncommits\t1\nstate.advanced\tyes\n",
		"\n[apply.revision]\nbase\tyes\n",
		"refused\t1 delete(s) need explicit confirmation\n",
		"elements.readable\t1\ndeleted.absent\t0\ndeleted.emptied\t1\ndeleted.present\t0\nrediff\tcreates=0\tupdates=0\tdeletes=0\tconflicts=0\n",
		"\n[apply.revision.changes]\nupdate\tB\tPartDefinition\ndelete\tA\tPackage\n",
		"\n[apply.revision.read-back]\nB\tdeclaredName=\"Cell\"\n",
		"\n[findings]\ninitial: 2 create(s)\n",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("the report lacks %q:\n%s", want, first)
		}
	}
}

// The revision must stage a declared-id rename, a create and a delete, or the
// live gate measures less than it claims.
func TestRevisedFixtureStagesEveryKindOfChange(t *testing.T) {
	original := fixtureGraph(t, applyFixturePath)
	revised := fixtureGraph(t, applyRevisionPath)

	set, err := reposync.Diff(revised, original, reposync.Options{Representation: Representation{}})
	if err != nil {
		t.Fatal(err)
	}
	creates, updates, deletes, conflicts := counts(set)
	if creates == 0 || updates == 0 || deletes == 0 || conflicts != 0 {
		t.Fatalf("the revision stages %d create(s), %d update(s), %d delete(s), %d conflict(s):\n%s",
			creates, updates, deletes, conflicts, set.Text())
	}
	renamed := false
	for _, change := range set.Changes {
		if change.Kind != reposync.KindUpdate || !change.Declared {
			continue
		}
		for _, delta := range change.Deltas {
			renamed = renamed || delta.Property == "declaredName"
		}
	}
	if !renamed {
		t.Errorf("no declared id is renamed, so the retained-id update is not measured:\n%s", set.Text())
	}
	if err := set.Appliable(); err == nil {
		t.Error("the revision's deletes must be gated, so the refusal is measured")
	}
}

// A collection's JSON annotation is what the commit path writes for the array
// the diff posts, so it is carried, not left out.
func TestCollectionAnnotationsAreCarried(t *testing.T) {
	original := fixtureGraph(t, applyFixturePath)
	if !original.HasProperty(rdf.ElementIRIForID("IdentityInterop"), rdf.AnnotationJSON+"ownedMember") {
		t.Fatal("the fixture's package states no ownedMember annotation")
	}
	set, err := reposync.Diff(original, rdf.NewGraph(), reposync.Options{Representation: Representation{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, left := range set.Uncarried {
		if strings.HasPrefix(left.Property, "json:") {
			t.Errorf("%s is reported as uncarried", left.Property)
		}
	}
	graphWritten := writtenFromGraph(original)
	if _, ok := graphWritten["IdentityInterop"].props["json:ownedMember"]; ok {
		t.Error("the harness counts the annotation as a property of its own")
	}
	if got := graphWritten["IdentityInterop"].props["sysml:ownedMember"]; got.kind != "array" || got.count < 2 {
		t.Errorf("sysml:ownedMember is written as %+v, want a multi-valued array", got)
	}
}

func TestEmptiedRecognisesADeletedRead(t *testing.T) {
	deleted := Element{"@id": json.RawMessage(`"X"`), "@type": json.RawMessage(`null`)}
	if !emptied(deleted) {
		t.Error("an id with a null type and nothing else is how a deleted element reads")
	}
	typed := Element{"@id": json.RawMessage(`"X"`), "@type": json.RawMessage(`"Package"`)}
	if emptied(typed) {
		t.Error("an element with a type is present")
	}
	named := Element{"@id": json.RawMessage(`"X"`), "declaredName": json.RawMessage(`"n"`)}
	if emptied(named) {
		t.Error("an element with a property is present")
	}
}

func TestRevisionNameSitsBesideTheFixture(t *testing.T) {
	if got := revisionName("testdata/identity_model.sysml"); got != "identity_model_revised.sysml" {
		t.Errorf("revisionName = %q", got)
	}
}

func fixtureGraph(t *testing.T, path string) *rdf.Graph {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := convert.SysMLToRDF(path, src)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}
