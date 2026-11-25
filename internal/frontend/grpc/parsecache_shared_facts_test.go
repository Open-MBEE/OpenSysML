package grpc

import (
	"slices"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A mission requirement in one document changes another document's verdict by
// a workspace-wide union, not an index dependency on the edited declaration.
const (
	sharedFactHub    = "package Hub {\n\tprivate import OOSEM::*;\n\t#missionRequirement requirement mission;\n}\n"
	sharedFactSys    = "package Sat {\n\tprivate import OOSEM::*;\n\t#systemRequirement requirement sys;\n}\n"
	sharedFactEdited = "package Hub {\n\tprivate import OOSEM::*;\n\t#stakeholderNeed requirement need;\n}\n"
)

func sharedFactRequest(hub, base string) *pb.ParseSourcesRequest {
	return &pb.ParseSourcesRequest{
		Documents:     inlineDocuments("hub.sysml", hub, "sat.sysml", sharedFactSys),
		BaseModelHash: base,
	}
}

func diagnosticsOf(t *testing.T, answers modelAnswers, file string) []string {
	t.Helper()
	var out []string
	for _, d := range answers.diagnostics.Diagnostics {
		if d.GetSpan().GetFile() == file {
			out = append(out, d.GetCode()+": "+d.GetMessage())
		}
	}
	return out
}

// Removing a shared fact from a restored set rechecks and reports the reader of
// that fact, as it does in a live workspace.
func TestPersistentParseCacheRestoresSharedFactsForAnEdit(t *testing.T) {
	live := mustNewService(t, 10)
	liveBase := answersOf(t, live, sharedFactRequest(sharedFactHub, ""))
	if !slices.ContainsFunc(diagnosticsOf(t, liveBase, "sat.sysml"), func(d string) bool {
		return strings.HasPrefix(d, "oosem-requirement-not-derived:")
	}) {
		t.Fatalf("the base has no underived system requirement: %v", diagnosticsOf(t, liveBase, "sat.sysml"))
	}
	liveEdit := answersOf(t, live, sharedFactRequest(sharedFactEdited, liveBase.parse.ModelHash))
	if !slices.Contains(liveEdit.parse.Affected, "sat.sysml") {
		t.Fatalf("in a live workspace sat.sysml is not affected: %v", liveEdit.parse.Affected)
	}
	if slices.ContainsFunc(diagnosticsOf(t, liveEdit, "sat.sysml"), func(d string) bool {
		return strings.HasPrefix(d, "oosem-requirement-not-derived:")
	}) {
		t.Fatalf("in a live workspace the stale verdict remains: %v", diagnosticsOf(t, liveEdit, "sat.sysml"))
	}

	cache, _ := recordCacheIn(t)
	answersOf(t, recordingService(t, cache), sharedFactRequest(sharedFactHub, ""))
	srv, counter := countingService(t, cache)
	base := answersOf(t, srv, sharedFactRequest(sharedFactHub, ""))
	if names := counter.calls(); len(names) != 0 {
		t.Fatalf("the base was not restored: %v", names)
	}
	got := answersOf(t, srv, sharedFactRequest(sharedFactEdited, base.parse.ModelHash))
	if !slices.Equal(got.parse.Affected, liveEdit.parse.Affected) {
		t.Errorf("restored affected = %v, live = %v", got.parse.Affected, liveEdit.parse.Affected)
	}
	if want := diagnosticsOf(t, liveEdit, "sat.sysml"); !slices.Equal(diagnosticsOf(t, got, "sat.sysml"), want) {
		t.Errorf("restored sat.sysml diagnostics = %v, live = %v", diagnosticsOf(t, got, "sat.sysml"), want)
	}
}
