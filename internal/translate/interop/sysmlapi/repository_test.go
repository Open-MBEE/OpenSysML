package sysmlapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// TestCommitRefusesAHeadThatAppearedSinceTheRead: a branch read before its
// first commit is still a head the write is held to, so a first commit another
// writer made in between refuses the change set computed against nothing.
func TestCommitRefusesAHeadThatAppearedSinceTheRead(t *testing.T) {
	head, posts := "", 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/projects/p/branches/b":
			branch := map[string]any{"@id": "b", "@type": "Branch", "name": "main", "head": nil}
			if head != "" {
				branch["head"] = map[string]any{"@id": head, "@type": "Commit"}
			}
			_ = json.NewEncoder(w).Encode(branch)
		case r.Method == http.MethodPost && r.URL.Path == "/projects/p/commits":
			posts++
			_ = json.NewEncoder(w).Encode(map[string]any{"@id": "c2", "@type": "Commit"})
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	repo := New(Config{BaseURL: server.URL}).Repository("p", "b")
	graph, err := repo.Graph(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Subjects()) != 0 || repo.Seen() != "" {
		t.Fatalf("a branch without a head read as %d subjects at commit %q", len(graph.Subjects()), repo.Seen())
	}
	changes := []reposync.ElementChange{{Kind: reposync.KindCreate, ID: "X1", Content: []rdf.Triple{
		triple(rdf.Element+"X1", rdf.RDFType, rdf.IRI(rdf.SysML+"Package")),
	}}}

	head = "c1"
	_, err = repo.Commit(context.Background(), changes, "first")
	var stale *StaleBranchError
	if !errors.As(err, &stale) {
		t.Fatalf("committing onto a head that appeared since the read: %v, want a StaleBranchError", err)
	}
	if stale.Seen != "" || stale.Head != "c1" {
		t.Errorf("the refusal says the head moved from %q to %q, want from none to c1", stale.Seen, stale.Head)
	}
	if posts != 0 {
		t.Errorf("%d commit(s) were posted onto the moved branch", posts)
	}

	head = ""
	if commit, err := repo.Commit(context.Background(), changes, "first"); err != nil || commit != "c2" {
		t.Errorf("committing onto a branch still without a head: %q, %v", commit, err)
	}
	if posts != 1 {
		t.Errorf("%d commit(s) posted, want 1", posts)
	}
	if repo.Seen() != "c2" {
		t.Errorf("after the write the repository stands at %q, want c2", repo.Seen())
	}
}
