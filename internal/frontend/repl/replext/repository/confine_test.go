package repository

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func element(prefix, id string) rdf.Term { return rdf.IRI(rdf.Element + prefix + id) }

func owned(graph *rdf.Graph, prefix, owner, id string) {
	graph.AddTriple(rdf.Triple{Subject: element(prefix, id), Predicate: rdf.IRI(rdf.SysML + "owner"), Object: element(prefix, owner)})
}

func named(graph *rdf.Graph, prefix, id string) {
	graph.AddTriple(rdf.Triple{Subject: element(prefix, id), Predicate: rdf.IRI(rdf.SysML + "name"), Object: rdf.String(id)})
}

// TestConfineFindsTheRootUnderAnyQualifier: a branch whose ids carry a scope
// qualifier still has its root found, so what was removed under it is
// deleted, and what lies under other roots is left alone — on a tracked
// branch; on an unloaded one, nothing is deleted at all.
func TestConfineFindsTheRootUnderAnyQualifier(t *testing.T) {
	for _, qualifier := range []string{"", "org.project:"} {
		cut := rdf.NewGraph()
		named(cut, "", "Vehicles")
		owned(cut, "", "Vehicles", "Wheel")
		remote := rdf.NewGraph()
		named(remote, qualifier, "Vehicles")
		owned(remote, qualifier, "Vehicles", "Wheel")
		owned(remote, qualifier, "Vehicles", "Car")
		owned(remote, qualifier, "Car", "wheels")
		named(remote, qualifier, "Spare")
		owned(remote, qualifier, "Spare", "Boat")
		changes := func() *reposync.ChangeSet {
			return &reposync.ChangeSet{Changes: []reposync.Change{
				{Kind: reposync.KindUpdate, ID: "Wheel"},
				{Kind: reposync.KindDelete, ID: "Car"},
				{Kind: reposync.KindDelete, ID: "wheels"},
				{Kind: reposync.KindDelete, ID: "Spare"},
				{Kind: reposync.KindConflict, ID: "Boat", Conflict: reposync.ConflictRepositoryChanged},
			}}
		}

		tracked := changes()
		result := &replext.PublishResult{}
		confine(tracked, cut, remote, element("", "Vehicles"), true, "Vehicles", result)
		if ids := idsOf(tracked); ids != "Wheel Car wheels" {
			t.Errorf("%q tracked: kept %q, want Wheel Car wheels", qualifier, ids)
		}
		if len(result.Notes) != 1 {
			t.Errorf("%q tracked: notes %q", qualifier, result.Notes)
		}

		unseen := changes()
		result = &replext.PublishResult{}
		confine(unseen, cut, remote, element("", "Vehicles"), false, "Vehicles", result)
		if ids := idsOf(unseen); ids != "Wheel" {
			t.Errorf("%q untracked: kept %q, want Wheel", qualifier, ids)
		}
		if len(result.Notes) != 2 {
			t.Errorf("%q untracked: notes %q", qualifier, result.Notes)
		}
	}
}

func idsOf(set *reposync.ChangeSet) string {
	out := ""
	for i, change := range set.Changes {
		if i > 0 {
			out += " "
		}
		out += change.ID
	}
	return out
}
