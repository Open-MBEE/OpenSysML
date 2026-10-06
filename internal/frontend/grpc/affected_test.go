package grpc

import (
	"context"
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const (
	affectedLibrary = "package Lib {\n\tpart def Prime;\n\tpart def Engine :> Prime {\n\t\tattribute power;\n\t}\n}\n"
	affectedTop     = "package Top {\n\tprivate import Lib::*;\n\tpart def Car {\n\t\tpart motor : Engine {\n\t\t\tattribute :>> power = 150;\n\t\t}\n\t}\n}\n"
	affectedOther   = "package Other {\n\tpart def Wheel;\n}\n"
)

// affectedStep is one state of a client's model and what its parse must say
// about the documents an edit from the state before it reaches.
type affectedStep struct {
	what              string
	lib, top, other   string
	affected, unmoved []string
}

// A client's edits, each parsed with the state before it as the base: a comment,
// a rename the application uses and its undoing, a feature moved up to a
// supertype, and an edit to a document nothing else reads.
var affectedSteps = []affectedStep{
	{what: "the library gains a comment", lib: affectedLibrary + "// a comment\n", top: affectedTop, other: affectedOther,
		affected: []string{"lib.sysml"}, unmoved: []string{"other.sysml"}},
	{what: "the library renames what the application uses", lib: "package Lib {\n\tpart def Prime;\n\tpart def Motor :> Prime {\n\t\tattribute power;\n\t}\n}\n", top: affectedTop, other: affectedOther,
		affected: []string{"lib.sysml", "top.sysml"}, unmoved: []string{"other.sysml"}},
	{what: "the rename is undone", lib: affectedLibrary, top: affectedTop, other: affectedOther,
		affected: []string{"lib.sysml", "top.sysml"}, unmoved: []string{"other.sysml"}},
	{what: "the feature moves up to the supertype", lib: "package Lib {\n\tpart def Prime {\n\t\tattribute power;\n\t}\n\tpart def Engine :> Prime;\n}\n", top: affectedTop, other: affectedOther,
		affected: []string{"lib.sysml", "top.sysml"}, unmoved: []string{"other.sysml"}},
	{what: "a document nothing reads changes", lib: "package Lib {\n\tpart def Prime {\n\t\tattribute power;\n\t}\n\tpart def Engine :> Prime;\n}\n", top: affectedTop, other: "package Other {\n\tpart def Wheel;\n\tpart def Tyre;\n}\n",
		affected: []string{"other.sysml"}, unmoved: []string{"lib.sysml", "top.sysml"}},
}

func (step affectedStep) documents() []*pb.SourceDocument {
	return inlineDocuments("lib.sysml", step.lib, "top.sysml", step.top, "other.sysml", step.other)
}

// After each edit the parse names every document it may have changed: one it
// leaves out converts, element by element, exactly as it did in the base model.
// Each step names at least the documents it must and leaves out the ones
// nothing it changed reaches.
func TestParseSourcesAffectedLeavesOutOnlyWhatConvertsAsBefore(t *testing.T) {
	srv := mustNewService(t, 32)
	defer srv.Close()
	parse := func(base string, documents []*pb.SourceDocument) *pb.ParseSourcesResponse {
		t.Helper()
		resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: documents, BaseModelHash: base})
		if err != nil {
			t.Fatalf("ParseSources: %v", err)
		}
		return resp
	}
	// The set is parsed twice before the edits, so they are answered from its
	// lineage; neither state comes back, so none is answered from the cache of a
	// fresh parse.
	parse("", inlineDocuments("lib.sysml", affectedLibrary, "top.sysml", affectedTop, "other.sysml", affectedOther+"// first\n"))
	base := parse("", inlineDocuments("lib.sysml", affectedLibrary+"\n", "top.sysml", affectedTop, "other.sysml", affectedOther)).ModelHash

	for _, step := range affectedSteps {
		resp := parse(base, step.documents())
		for _, name := range step.affected {
			if !slices.Contains(resp.Affected, name) {
				t.Errorf("%s: %s is not affected: %v", step.what, name, resp.Affected)
			}
		}
		for _, name := range step.unmoved {
			if slices.Contains(resp.Affected, name) {
				t.Errorf("%s: %s is affected though nothing it reads changed: %v", step.what, name, resp.Affected)
			}
		}
		for _, name := range []string{"lib.sysml", "top.sysml", "other.sysml"} {
			if slices.Contains(resp.Affected, name) {
				continue
			}
			before, after := convertElements(t, srv, base, name), convertElements(t, srv, resp.ModelHash, name)
			if len(before) != len(after) {
				t.Errorf("%s: %s is not affected, but writes %d elements, %d before", step.what, name, len(after), len(before))
			}
			for id, element := range before {
				if after[id] != element {
					t.Errorf("%s: %s is not affected, but %s was\n%s\nand is\n%s", step.what, name, id, element, after[id])
				}
			}
		}
		base = resp.ModelHash
	}
}

// Without a base the service can compare against, every document is affected:
// no base, one it no longer holds, one answered fresh, or one of another set.
func TestParseSourcesAffectedIsEveryDocumentWithoutAComparableBase(t *testing.T) {
	srv := mustNewService(t, 32)
	defer srv.Close()
	all := []string{"lib.sysml", "top.sysml", "other.sysml"}
	parse := func(base string, lib string) *pb.ParseSourcesResponse {
		t.Helper()
		resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
			Documents:     inlineDocuments("lib.sysml", lib, "top.sysml", affectedTop, "other.sysml", affectedOther),
			BaseModelHash: base,
		})
		if err != nil {
			t.Fatalf("ParseSources: %v", err)
		}
		return resp
	}
	first := parse("", affectedLibrary)
	if !slices.Equal(first.Affected, all) {
		t.Errorf("no base: affected %v, want %v", first.Affected, all)
	}
	// The first parse of a set is answered fresh, so the next has nothing to compare with.
	second := parse(first.ModelHash, affectedLibrary+"// one\n")
	if !slices.Equal(second.Affected, all) {
		t.Errorf("a base answered fresh: affected %v, want %v", second.Affected, all)
	}
	if got := parse("not-a-model", affectedLibrary+"// two\n"); !slices.Equal(got.Affected, all) {
		t.Errorf("an unknown base: affected %v, want %v", got.Affected, all)
	}

	other, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: inlineDocuments("lib.sysml", affectedLibrary, "top.sysml", affectedTop)})
	if err != nil {
		t.Fatal(err)
	}
	if got := parse(other.ModelHash, affectedLibrary+"// three\n"); !slices.Equal(got.Affected, all) {
		t.Errorf("a base of another document set: affected %v, want %v", got.Affected, all)
	}
}

// A service without the capability answers no affected documents, and refuses
// a base it cannot compare against (TestCapabilityGatedRequestsAreRefused).
func TestParseSourcesAffectedNeedsTheCapability(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityParseSourcesAffected)
	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: inlineDocuments("p.sysml", "package P;")})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Affected) != 0 {
		t.Errorf("affected %v from a service without %s", resp.Affected, CapabilityParseSourcesAffected)
	}
}

// The capability is the newest, appended after every one before it.
func TestParseSourcesAffectedCapabilityIsTheNewest(t *testing.T) {
	if all := Capabilities(); all[len(all)-1] != CapabilityParseSourcesAffected {
		t.Errorf("capabilities %v do not end with %q, the newest", all, CapabilityParseSourcesAffected)
	}
}
