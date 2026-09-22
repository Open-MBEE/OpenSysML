package flexo

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseBranchURL(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want BranchRef
	}{
		{"http://localhost:8083/projects/p/branches/main", BranchRef{SysMLV2URL: "http://localhost:8083", Project: "p", Branch: "main"}},
		{"https://mms.example.com/projects/p/branches/b/", BranchRef{SysMLV2URL: "https://mms.example.com", Project: "p", Branch: "b"}},
		{"http://host:8443/base/api/projects/proj-x/branches/rel-1.2", BranchRef{SysMLV2URL: "http://host:8443/base/api", Project: "proj-x", Branch: "rel-1.2"}},
		{"flexo://p/main", BranchRef{Project: "p", Branch: "main"}},
	} {
		got, ok, err := ParseBranchURL(tc.in)
		if err != nil || !ok {
			t.Errorf("%s: ok=%v, err=%v", tc.in, ok, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestParseBranchURLRefusesURLsThatNameNoBranch(t *testing.T) {
	for _, in := range []string{
		"http://localhost:8083",
		"http://localhost:8083/projects",
		"http://localhost:8083/projects/p",
		"http://localhost:8083/projects/p/branches",
		"http://localhost:8083/projects/p/branches/b/commits",
		"http://localhost:8083/projects/p/tags/b",
		"http://localhost:8083/projects/p/branches/b?x=1",
		"flexo://",
		"flexo://p",
		"flexo://p/b/c",
		"flexo://projects/p/branches/b",
	} {
		if ref, ok, err := ParseBranchURL(in); err == nil {
			t.Errorf("%s: accepted as %+v (ok=%v), want an error naming the two forms", in, ref, ok)
		} else if !strings.Contains(err.Error(), "/projects/") || !strings.Contains(err.Error(), "flexo://") {
			t.Errorf("%s: the error does not spell out the accepted forms: %v", in, err)
		}
	}
}

func TestParseBranchURLLLeavesPathsToTheFilesystem(t *testing.T) {
	for _, in := range []string{"model.sysml", "dir/model.ttl", "-", "", "ttl:x"} {
		if ref, ok, err := ParseBranchURL(in); ok || err != nil {
			t.Errorf("%q: ok=%v, ref=%+v, err=%v; want a plain path, not a URL", in, ok, ref, err)
		}
	}
}

func TestPushWritesTheWholeGraphConditionally(t *testing.T) {
	client, fake := stack(t, sparqlFixture)
	repo := client.Repository("p", "b")
	turtle := []byte("<urn:x:s> <urn:x:p> <urn:x:o> .\n")

	head, err := repo.Push(context.Background(), turtle, "a push")
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.puts) != 1 {
		t.Fatalf("the push sent %d write(s), want 1", len(fake.puts))
	}
	put := fake.puts[0]
	if put.url != "/orgs/o/repos/p/branches/b/graph?message=a+push" {
		t.Errorf("the write hit %s", put.url)
	}
	if put.contentType != mediaTurtle {
		t.Errorf("the write went as %q, not Turtle", put.contentType)
	}
	if put.ifMatch != `"etag-0"` {
		t.Errorf("the write was conditioned on %q, not the served etag", put.ifMatch)
	}
	if string(put.body) != string(turtle) {
		t.Errorf("the write carried %s", put.body)
	}
	if head != "push-1" || repo.Seen() != "push-1" || fake.head != "push-1" {
		t.Errorf("the pushed commit is %q (seen %q, stack head %q)", head, repo.Seen(), fake.head)
	}
}

func TestPushRefusesAHeadThatMovedSinceTheRead(t *testing.T) {
	client, fake := stack(t, sparqlFixture)
	repo := client.Repository("p", "b")
	ctx := context.Background()
	if _, err := repo.Graph(ctx); err != nil {
		t.Fatal(err)
	}
	fake.head = "c-elsewhere"

	_, err := repo.Push(ctx, []byte("<s> <p> <o> ."), "a push")
	var stale *StaleBranchError
	if !errors.As(err, &stale) || stale.Seen != "c-0" || stale.Head != "c-elsewhere" {
		t.Fatalf("want a StaleBranchError from c-0 to c-elsewhere, got %v", err)
	}
	if len(fake.puts) != 0 {
		t.Errorf("a stale push still wrote %d time(s)", len(fake.puts))
	}
	if repo.Seen() != "c-0" {
		t.Errorf("the refusal moved what was seen to %q", repo.Seen())
	}
}

func TestPushRefusesAHeadThatMovedPastTheSyncState(t *testing.T) {
	client, fake := stack(t, sparqlFixture)
	repo := client.Repository("p", "b")
	repo.Resume("last-seen")

	_, err := repo.Push(context.Background(), []byte("<s> <p> <o> ."), "a push")
	var stale *StaleBranchError
	if !errors.As(err, &stale) || stale.Seen != "last-seen" || stale.Head != "c-0" {
		t.Fatalf("want a StaleBranchError from last-seen to c-0, got %v", err)
	}
	if len(fake.puts) != 0 {
		t.Errorf("a stale push still wrote %d time(s)", len(fake.puts))
	}
}

func TestPushRefusesAHeadThatMovedAfterTheEtagRead(t *testing.T) {
	client, fake := stack(t, sparqlFixture)
	repo := client.Repository("p", "b")
	repo.Resume("c-0")
	// The branch still serves c-0's etag but its head has already moved: the
	// stale check after the etag read refuses, and no write is sent.
	fake.head = "c-elsewhere"

	_, err := repo.Push(context.Background(), []byte("<s> <p> <o> ."), "a push")
	var stale *StaleBranchError
	if !errors.As(err, &stale) || stale.Seen != "c-0" || stale.Head != "c-elsewhere" {
		t.Fatalf("want a StaleBranchError from c-0 to c-elsewhere, got %v", err)
	}
	if len(fake.puts) != 0 {
		t.Errorf("a stale push still wrote %d time(s)", len(fake.puts))
	}
}

func TestPushTakesACommittedWritePastAnAmbiguousPreconditionAnswer(t *testing.T) {
	client, fake := stack(t, sparqlFixture)
	fake.commit412 = true
	repo := client.Repository("p", "b")

	head, err := repo.Push(context.Background(), []byte("<s> <p> <o> ."), "a push")
	if err != nil {
		t.Fatal(err)
	}
	if head != "push-1" || repo.Seen() != "push-1" {
		t.Errorf("the committed write is head %q, seen %q; want push-1", head, repo.Seen())
	}
}

func TestPushRefusesAFailedPrecondition(t *testing.T) {
	client, fake := stack(t, sparqlFixture)
	fake.race412 = true
	repo := client.Repository("p", "b")

	_, err := repo.Push(context.Background(), []byte("<s> <p> <o> ."), "a push")
	var stale *StaleBranchError
	if !errors.As(err, &stale) {
		t.Fatalf("want a StaleBranchError on a 412, got %v", err)
	}
	if repo.Seen() != "" {
		t.Errorf("the refusal moved what was seen to %q", repo.Seen())
	}
}
