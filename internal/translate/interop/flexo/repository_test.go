package flexo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func triple(subject, predicate string, object rdf.Term) rdf.Triple {
	return rdf.Triple{Subject: rdf.IRI(subject), Predicate: rdf.IRI(predicate), Object: object}
}

func TestCommitRequestSpellsEveryChangeKind(t *testing.T) {
	x := rdf.Element + "X1"
	changes := []reposync.ElementChange{
		{Kind: reposync.KindUpdate, ID: "X1", Content: []rdf.Triple{
			triple(x, rdf.RDFType, rdf.IRI(rdf.SysML+"PartDefinition")),
			triple(x, rdf.SysML+"declaredName", rdf.String("renamed")),
			triple(x, rdf.SysML+"isAbstract", rdf.TypedLiteral("true", rdf.XSD+"boolean")),
			triple(x, rdf.SysML+"ownedMember", rdf.IRI(rdf.Element+"Y1")),
			triple(x, rdf.SysML+"ownedMember", rdf.IRI(rdf.Expression+"Y2")),
			triple(x, rdf.SysML+"count", rdf.TypedLiteral("3", rdf.XSD+"integer")),
			triple(x, rdf.SysML+"mass", rdf.TypedLiteral("1.5", rdf.XSD+"decimal")),
			triple(x, rdf.SysML+"owner", rdf.IRI(rdf.RDFNS+"nil")),
		}},
		{Kind: reposync.KindCreate, ID: "N1", Content: []rdf.Triple{
			triple(rdf.Element+"N1", rdf.RDFType, rdf.IRI(rdf.SysML+"Package")),
		}},
		{Kind: reposync.KindDelete, ID: "D1"},
	}
	body, err := commitRequest(changes, "sync")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"@type":       "Commit",
		"description": "sync",
		"change": []any{
			map[string]any{
				"@type":    "DataVersion",
				"identity": map[string]any{"@id": "X1"},
				"payload": map[string]any{
					"@id":          "X1",
					"@type":        "PartDefinition",
					"declaredName": "renamed",
					"isAbstract":   true,
					"ownedMember":  []any{map[string]any{"@id": "Y1"}, map[string]any{"@id": "Y2"}},
					"count":        float64(3),
					"mass":         1.5,
					"owner":        nil,
				},
			},
			map[string]any{
				"@type":    "DataVersion",
				"identity": map[string]any{"@id": "N1"},
				"payload":  map[string]any{"@id": "N1", "@type": "Package"},
			},
			map[string]any{
				"@type":    "DataVersion",
				"identity": map[string]any{"@id": "D1"},
				"payload":  nil,
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("commit request:\n got %s\nwant %+v", body, want)
	}
}

func TestCommitRequestRefusesWhatTheServiceCannotHold(t *testing.T) {
	x := rdf.Element + "X1"
	typed := triple(x, rdf.RDFType, rdf.IRI(rdf.SysML+"PartDefinition"))
	cases := []struct {
		name    string
		content []rdf.Triple
		want    error
	}{
		{"untyped", []rdf.Triple{triple(x, rdf.SysML+"declaredName", rdf.String("a"))}, ErrUntyped},
		{"two types", []rdf.Triple{typed, triple(x, rdf.RDFType, rdf.IRI(rdf.SysML+"Package"))}, ErrUntyped},
		{"foreign property", []rdf.Triple{typed, triple(x, "urn:opensysml:sysml:memberIndex", rdf.String("0"))}, &UnrepresentableError{}},
		{"language tag", []rdf.Triple{typed, triple(x, rdf.SysML+"declaredName", rdf.Term{Kind: rdf.TermLiteral, Value: "a", Lang: "en"})}, &UnrepresentableError{}},
		{"foreign IRI", []rdf.Triple{typed, triple(x, rdf.SysML+"owner", rdf.IRI("http://example.org/x"))}, &UnrepresentableError{}},
		{"odd datatype", []rdf.Triple{typed, triple(x, rdf.SysML+"when", rdf.TypedLiteral("2020-01-01", rdf.XSD+"date"))}, &UnrepresentableError{}},
		{"non-numeric integer", []rdf.Triple{typed, triple(x, rdf.SysML+"n", rdf.TypedLiteral("+07", rdf.XSD+"integer"))}, &UnrepresentableError{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := commitRequest([]reposync.ElementChange{{Kind: reposync.KindCreate, ID: "X1", Content: tc.content}}, "")
			if err == nil {
				t.Fatal("accepted")
			}
			var unrepresentable *UnrepresentableError
			switch tc.want.(type) {
			case *UnrepresentableError:
				if !errors.As(err, &unrepresentable) {
					t.Errorf("want an UnrepresentableError, got %v", err)
				}
			default:
				if !errors.Is(err, tc.want) {
					t.Errorf("want %v, got %v", tc.want, err)
				}
			}
		})
	}
}

func TestRepresentationCarriesWhatTheServiceStores(t *testing.T) {
	x := rdf.Element + "X1"
	rep := Representation{}
	dropped, ok, err := rep.Carry(triple(x, "urn:opensysml:sysml:memberIndex", rdf.String("0")))
	if err != nil || ok {
		t.Errorf("a foreign property was carried: %+v, %v", dropped, err)
	}
	cases := []struct {
		in, want rdf.Term
	}{
		{rdf.TypedLiteral("a", rdf.XSD+"string"), rdf.String("a")},
		{rdf.TypedLiteral("2", rdf.XSD+"decimal"), rdf.TypedLiteral("2", rdf.XSD+"integer")},
		{rdf.TypedLiteral("2.5", rdf.XSD+"decimal"), rdf.TypedLiteral("2.5", rdf.XSD+"decimal")},
		{rdf.TypedLiteral("-4", rdf.XSD+"integer"), rdf.TypedLiteral("-4", rdf.XSD+"integer")},
		{rdf.TypedLiteral("false", rdf.XSD+"boolean"), rdf.TypedLiteral("false", rdf.XSD+"boolean")},
		{rdf.IRI(rdf.Expression + "E"), rdf.IRI(rdf.Expression + "E")},
		{rdf.IRI(rdf.RDFNS + "nil"), rdf.IRI(rdf.RDFNS + "nil")},
	}
	for _, tc := range cases {
		got, ok, err := rep.Carry(triple(x, rdf.SysML+"p", tc.in))
		if err != nil || !ok {
			t.Errorf("%s: not carried: %v", tc.in, err)
			continue
		}
		if got.Object != tc.want {
			t.Errorf("%s: carried as %s, want %s", tc.in, got.Object, tc.want)
		}
	}
	if _, _, err := rep.Carry(triple(x, rdf.SysML+"p", rdf.TypedLiteral("1e3", rdf.XSD+"decimal"))); err == nil {
		t.Error("a non-JSON number was carried")
	}
}

// fakePut is a whole-graph write the fake Layer 1 received: the path it hit,
// the entity tag it was preconditioned on, the media type and the Turtle body.
type fakePut struct {
	url         string
	ifMatch     string
	contentType string
	body        []byte
}

// fakeStack fakes the two services for one repository: the branch path names
// the head, the Layer 1 branch path serves its etag, the graph path takes a
// conditional whole-graph write, the commit path records what it was sent and
// moves the head, the query path answers any commit with a fixed result set.
type fakeStack struct {
	head      string
	etag      string
	race412   bool   // every graph write answers 412 without writing, as a moved branch does
	commit412 bool   // a graph write commits but still answers 412, as the deployed service does
	putETag   string // a graph write answers 2xx with this ETag and moves no head, as when another commit already stands there
	posted    [][]byte
	puts      []fakePut
}

func stack(t *testing.T, results string) (*Client, *fakeStack) {
	t.Helper()
	fake := &fakeStack{head: "c-0", etag: "etag-0"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		switch {
		case r.URL.Path == "/projects/p/branches/b" && r.Method == http.MethodGet:
			fmt.Fprintf(w, `{"@id":"b","@type":"Branch","head":{"@id":%q}}`, fake.head)
		case r.URL.Path == "/projects/p/commits":
			if r.URL.Query().Get("branchId") != "b" {
				t.Errorf("commit posted to branch %q, want b", r.URL.Query().Get("branchId"))
			}
			fake.posted = append(fake.posted, body)
			fake.head = fmt.Sprintf("c-%d", len(fake.posted))
			fmt.Fprintf(w, `{"@id":%q,"@type":"Commit"}`, fake.head)
		case r.URL.Path == "/orgs/o/repos/p/branches/b" && r.Method == http.MethodGet:
			if r.Header.Get("Accept") != mediaTurtle {
				t.Errorf("branch read without a Turtle Accept: %q", r.Header.Get("Accept"))
			}
			w.Header().Set("ETag", fake.etag)
			fmt.Fprintln(w, `<> <urn:x:etag> "x" .`)
		case r.URL.Path == "/orgs/o/repos/p/branches/b/graph" && r.Method == http.MethodPut:
			fake.puts = append(fake.puts, fakePut{
				url:         r.URL.String(),
				ifMatch:     r.Header.Get("If-Match"),
				contentType: r.Header.Get("Content-Type"),
				body:        body,
			})
			ifMatch := r.Header.Get("If-Match")
			switch {
			case fake.race412:
				w.WriteHeader(http.StatusPreconditionFailed)
			case ifMatch != "*" && ifMatch != `"`+fake.etag+`"`:
				w.WriteHeader(http.StatusPreconditionFailed)
			default:
				if fake.putETag != "" {
					w.Header().Set("ETag", fake.putETag)
					break
				}
				fake.head = fmt.Sprintf("push-%d", len(fake.puts))
				fake.etag = fake.head
				if fake.commit412 {
					// The deployed service answers 412 for a write it just
					// committed; its response ETag is the commit made.
					w.Header().Set("ETag", fake.head)
					w.WriteHeader(http.StatusPreconditionFailed)
				}
			}
		case strings.HasPrefix(r.URL.Path, "/orgs/o/repos/p/locks/Commit.") && strings.HasSuffix(r.URL.Path, "/query"):
			if r.Header.Get("Accept") != "application/sparql-results+json" {
				t.Errorf("query without a JSON results Accept: %q", r.Header.Get("Accept"))
			}
			_, _ = io.WriteString(w, results)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return New(Config{Layer1URL: server.URL, SysMLV2URL: server.URL, Token: "t", Org: "o"}), fake
}

const sparqlFixture = `{"head":{"vars":["s","p","o"]},"results":{"bindings":[
{"s":{"type":"uri","value":"urn:sysmlv2:element:X1"},"p":{"type":"uri","value":"http://www.w3.org/1999/02/22-rdf-syntax-ns#type"},"o":{"type":"uri","value":"https://www.omg.org/spec/SysML#PartDefinition"}},
{"s":{"type":"uri","value":"urn:sysmlv2:element:X1"},"p":{"type":"uri","value":"https://www.omg.org/spec/SysML#declaredName"},"o":{"type":"literal","value":"X"}},
{"s":{"type":"uri","value":"urn:sysmlv2:element:X1"},"p":{"type":"uri","value":"https://www.omg.org/spec/SysML#count"},"o":{"type":"literal","datatype":"http://www.w3.org/2001/XMLSchema#integer","value":"3"}},
{"s":{"type":"uri","value":"urn:sysmlv2:element:X1"},"p":{"type":"uri","value":"https://www.omg.org/spec/SysML#note"},"o":{"type":"literal","xml:lang":"en","value":"hi"}}
]}}`

func TestRepositoryReadsGraphsAndWritesCommits(t *testing.T) {
	client, fake := stack(t, sparqlFixture)
	repo := client.Repository("p", "b")
	ctx := context.Background()

	if repo.Seen() != "" {
		t.Errorf("a repository has seen %q before any read", repo.Seen())
	}
	graph, err := repo.Graph(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Seen() != "c-0" {
		t.Errorf("the read stood at %q, want the head c-0", repo.Seen())
	}
	x := rdf.IRI(rdf.Element + "X1")
	if got, _ := graph.Object(x, rdf.SysML+"count"); got != rdf.TypedLiteral("3", rdf.XSD+"integer") {
		t.Errorf("typed literal read back as %s", got)
	}
	if got, _ := graph.Object(x, rdf.SysML+"declaredName"); got != rdf.String("X") {
		t.Errorf("plain literal read back as %s", got)
	}
	if got, _ := graph.Object(x, rdf.SysML+"note"); got.Lang != "en" {
		t.Errorf("language tag lost: %s", got)
	}
	if at, err := repo.GraphAt(ctx, "c-0"); err != nil || len(at.Triples()) != 4 {
		t.Errorf("commit graph: %d triple(s), %v", len(at.Triples()), err)
	}

	commit, err := repo.Commit(ctx, []reposync.ElementChange{{Kind: reposync.KindDelete, ID: "X1"}}, "bye")
	if err != nil {
		t.Fatal(err)
	}
	if commit != "c-1" || len(fake.posted) != 1 || repo.Seen() != "c-1" {
		t.Errorf("commit %q from %d post(s), seen %q; want c-1 from 1, seen c-1", commit, len(fake.posted), repo.Seen())
	}
	// A second batch follows the repository's own commit without complaint.
	if commit, err = repo.Commit(ctx, []reposync.ElementChange{{Kind: reposync.KindDelete, ID: "X1"}}, "bye"); err != nil || commit != "c-2" {
		t.Errorf("second batch: %q, %v", commit, err)
	}
}

func TestRepositoryRefusesToCommitPastAMovedHead(t *testing.T) {
	client, fake := stack(t, sparqlFixture)
	repo := client.Repository("p", "b")
	ctx := context.Background()
	if _, err := repo.Graph(ctx); err != nil {
		t.Fatal(err)
	}
	fake.head = "c-elsewhere"

	_, err := repo.Commit(ctx, []reposync.ElementChange{{Kind: reposync.KindDelete, ID: "X1"}}, "bye")
	var stale *StaleBranchError
	if !errors.As(err, &stale) || stale.Seen != "c-0" || stale.Head != "c-elsewhere" {
		t.Fatalf("want a StaleBranchError from c-0 to c-elsewhere, got %v", err)
	}
	if len(fake.posted) != 0 {
		t.Errorf("a stale commit was still posted %d time(s)", len(fake.posted))
	}
	if repo.Seen() != "c-0" {
		t.Errorf("the refusal moved what was seen to %q", repo.Seen())
	}

	// Reading again moves to the new head, and the commit goes through.
	if _, err := repo.Graph(ctx); err != nil || repo.Seen() != "c-elsewhere" {
		t.Fatalf("re-read: seen %q, %v", repo.Seen(), err)
	}
	if _, err := repo.Commit(ctx, []reposync.ElementChange{{Kind: reposync.KindDelete, ID: "X1"}}, "bye"); err != nil || len(fake.posted) != 1 {
		t.Errorf("commit after re-read: %d post(s), %v", len(fake.posted), err)
	}
}

func TestSelectGraphRefusesBlankNodes(t *testing.T) {
	client, _ := stack(t, `{"results":{"bindings":[{"s":{"type":"bnode","value":"b0"},"p":{"type":"uri","value":"p"},"o":{"type":"literal","value":"x"}}]}}`)
	if _, err := client.Repository("p", "b").Graph(context.Background()); !errors.Is(err, ErrBlankNode) {
		t.Errorf("want ErrBlankNode, got %v", err)
	}
}
