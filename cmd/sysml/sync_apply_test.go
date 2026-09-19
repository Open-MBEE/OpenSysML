package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/flexo"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// fakeStack stands in for Layer 1 and the service: SPARQL answered from the
// graph at the branch head or at any earlier commit, commits applied as the
// service does (payload replaces whole, null removes).
type fakeStack struct {
	mu       sync.Mutex
	graph    *rdf.Graph
	head     string
	versions map[string]*rdf.Graph
	commits  []json.RawMessage
	foreign  int
	refuse   bool
	drift    bool // every graph read is followed by someone else's commit
	unread   bool // every read fails
	server   *httptest.Server
}

func newFakeStack(t *testing.T, graph *rdf.Graph) *fakeStack {
	t.Helper()
	stack := &fakeStack{graph: graph, head: "commit-0", versions: map[string]*rdf.Graph{"commit-0": graph}}
	stack.server = httptest.NewServer(http.HandlerFunc(stack.serve))
	t.Cleanup(stack.server.Close)
	return stack
}

// edit lands someone else's commit on the branch.
func (s *fakeStack) edit(graph *rdf.Graph) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.foreign++
	s.advance(graph, fmt.Sprintf("foreign-%d", s.foreign))
}

// set changes the stack's behavior between runs.
func (s *fakeStack) set(change func(*fakeStack)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	change(s)
}

// posted is every commit body the stack accepted so far.
func (s *fakeStack) posted() []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]json.RawMessage(nil), s.commits...)
}

func (s *fakeStack) advance(graph *rdf.Graph, commit string) {
	s.graph, s.head = graph, commit
	s.versions[commit] = graph
}

func (s *fakeStack) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer test-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case s.unread:
		http.Error(w, "the fake cannot be read", http.StatusBadGateway)
	case strings.HasSuffix(r.URL.Path, "/branches/main") && r.Method == http.MethodGet:
		fmt.Fprintf(w, `{"@id":"main","@type":"Branch","head":{"@id":%q}}`, s.head)
	case strings.HasSuffix(r.URL.Path, "/query"):
		graph := s.graph
		if at := strings.TrimSuffix(r.URL.Path, "/query"); strings.Contains(at, "/locks/Commit.") {
			commit := at[strings.LastIndex(at, "/locks/Commit.")+len("/locks/Commit."):]
			if graph = s.versions[commit]; graph == nil {
				http.NotFound(w, r)
				return
			}
		}
		_ = json.NewEncoder(w).Encode(sparqlResultsOf(graph))
		if s.drift {
			s.foreign++
			s.advance(s.graph, fmt.Sprintf("foreign-%d", s.foreign))
		}
	case strings.HasSuffix(r.URL.Path, "/commits") && r.Method == http.MethodPost:
		if s.refuse {
			http.Error(w, "the fake refuses every commit", http.StatusConflict)
			return
		}
		var request struct {
			Change []struct {
				Identity struct {
					ID string `json:"@id"`
				} `json:"identity"`
				Payload map[string]json.RawMessage `json:"payload"`
			} `json:"change"`
		}
		body := json.RawMessage{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || json.Unmarshal(body, &request) != nil {
			http.Error(w, "bad commit", http.StatusBadRequest)
			return
		}
		graph := s.graph
		for _, change := range request.Change {
			graph = replaceElement(graph, change.Identity.ID, change.Payload)
		}
		s.commits = append(s.commits, body)
		s.advance(graph, fmt.Sprintf("commit-%d", len(s.commits)))
		fmt.Fprintf(w, `{"@id":%q,"@type":"Commit"}`, s.head)
	default:
		http.NotFound(w, r)
	}
}

// replaceElement drops every triple of the element and adds the payload's, in
// the RDF the service would store for each JSON value.
func replaceElement(g *rdf.Graph, id string, payload map[string]json.RawMessage) *rdf.Graph {
	subject := rdf.IRI(rdf.Element + id)
	next := rdf.NewGraph()
	for _, triple := range g.Triples() {
		if triple.Subject != subject {
			next.AddTriple(triple)
		}
	}
	for name, raw := range payload {
		switch name {
		case "@id":
		case "@type":
			var metaclass string
			_ = json.Unmarshal(raw, &metaclass)
			next.Add(subject, rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+metaclass))
		default:
			for _, object := range rdfValues(raw) {
				next.Add(subject, rdf.IRI(rdf.SysML+name), object)
			}
		}
	}
	return next
}

func rdfValues(raw json.RawMessage) []rdf.Term {
	var value any
	_ = json.Unmarshal(raw, &value)
	switch v := value.(type) {
	case nil:
		return []rdf.Term{rdf.IRI(rdf.RDFNS + "nil")}
	case string:
		return []rdf.Term{rdf.String(v)}
	case bool:
		return []rdf.Term{rdf.TypedLiteral(fmt.Sprint(v), rdf.XSD+"boolean")}
	case json.Number, float64:
		datatype := rdf.XSD + "integer"
		if strings.Contains(string(raw), ".") {
			datatype = rdf.XSD + "decimal"
		}
		return []rdf.Term{rdf.TypedLiteral(string(raw), datatype)}
	case map[string]any:
		if id, ok := v["@id"].(string); ok {
			return []rdf.Term{rdf.IRI(rdf.Element + id)}
		}
	case []any:
		var items []json.RawMessage
		_ = json.Unmarshal(raw, &items)
		var out []rdf.Term
		for _, item := range items {
			out = append(out, rdfValues(item)...)
		}
		return out
	}
	return nil
}

// sparqlResultsOf renders a graph as the SPARQL JSON results of SELECT ?s ?p ?o.
func sparqlResultsOf(g *rdf.Graph) map[string]any {
	term := func(t rdf.Term) map[string]string {
		if t.IsIRI() {
			return map[string]string{"type": "uri", "value": t.Value}
		}
		out := map[string]string{"type": "literal", "value": t.Value}
		if t.Datatype != "" {
			out["datatype"] = t.Datatype
		}
		if t.Lang != "" {
			out["xml:lang"] = t.Lang
		}
		return out
	}
	bindings := []map[string]any{}
	for _, triple := range g.Triples() {
		bindings = append(bindings, map[string]any{"s": term(triple.Subject), "p": term(triple.Predicate), "o": term(triple.Object)})
	}
	return map[string]any{"results": map[string]any{"bindings": bindings}}
}

// syncCommand runs the binary against the fake stack, the token in the
// environment as a real run would carry it.
func syncCommand(stack *fakeStack, binary string, args ...string) *exec.Cmd {
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(),
		flexo.EnvToken+"=test-token",
		flexo.EnvLayer1URL+"="+stack.server.URL)
	return cmd
}

func exitCode(t *testing.T, cmd *exec.Cmd) (string, int) {
	t.Helper()
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	exit, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("%v: %v\n%s", cmd.Args, err, out)
	}
	return string(out), exit.ExitCode()
}

func writeModel(t *testing.T, dir, name, src string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func liveGraph(t *testing.T, src string) *rdf.Graph {
	t.Helper()
	graph, err := convert.SysMLToRDF("repo.sysml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func TestSyncApplyWritesAndRecordsTheCommit(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	model := writeModel(t, dir, "model.sysml", renamedModel)

	// The dry run against the live endpoint reads, reports, and writes nothing.
	out, code := exitCode(t, syncCommand(stack, binary, model, "-sync-diff", stack.server.URL))
	if code != 0 || !strings.Contains(out, "update   8f3a41d0") || len(stack.posted()) != 0 {
		t.Fatalf("dry run: exit %d, %d commit(s):\n%s", code, len(stack.posted()), out)
	}
	if _, err := os.Stat(model + ".sync.json"); !os.IsNotExist(err) {
		t.Error("the dry run wrote sync state")
	}

	out, code = exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL))
	if code != 0 || len(stack.posted()) != 1 {
		t.Fatalf("apply: exit %d, %d commit(s):\n%s", code, len(stack.posted()), out)
	}
	if !strings.Contains(out, "last-seen commit commit-1 recorded in") {
		t.Errorf("the apply does not report the commit it recorded:\n%s", out)
	}
	if !strings.Contains(string(stack.posted()[0]), `"identity":{"@id":"8f3a41d0"}`) ||
		strings.Contains(string(stack.posted()[0]), `"identity":{"@id":"8f3a41d0"},"payload":null`) {
		t.Errorf("the rename did not reach the stack as an update of the retained id:\n%s", stack.posted()[0])
	}
	state, err := os.ReadFile(model + ".sync.json")
	if err != nil || !strings.Contains(string(state), `"lastSeenCommit": "commit-1"`) {
		t.Errorf("sync state after apply: %v\n%s", err, state)
	}
	notation, _ := os.ReadFile(model)
	if strings.Contains(string(notation), "commit-1") {
		t.Error("the commit id leaked into the model text")
	}

	// Idempotence: the applied branch has nothing left to sync.
	out, code = exitCode(t, syncCommand(stack, binary, model, "-sync-diff", stack.server.URL))
	if code != 0 || !strings.Contains(out, "nothing to change") {
		t.Errorf("re-diff after apply: exit %d\n%s", code, out)
	}
	out, code = exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL))
	if code != 0 || len(stack.posted()) != 1 || !strings.Contains(out, "nothing applied") {
		t.Errorf("re-apply wrote again: exit %d, %d commit(s)\n%s", code, len(stack.posted()), out)
	}
}

func TestSyncApplyRefusesUnconfirmedDeletes(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, renamedModel))
	model := writeModel(t, t.TempDir(), "model.sysml", syncedModel)

	out, code := exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL))
	if code != 1 || len(stack.posted()) != 0 {
		t.Fatalf("unconfirmed deletes: exit %d, %d commit(s):\n%s", code, len(stack.posted()), out)
	}
	if !strings.Contains(out, "refused to apply") || !strings.Contains(out, "explicit confirmation") {
		t.Errorf("the refusal is not explained:\n%s", out)
	}
	if _, err := os.Stat(model + ".sync.json"); !os.IsNotExist(err) {
		t.Error("a refused apply wrote sync state")
	}

	out, code = exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL, "-sync-confirm-deletes"))
	if code != 0 || len(stack.posted()) != 1 || !strings.Contains(string(stack.posted()[0]), `"payload":null`) {
		t.Fatalf("confirmed deletes: exit %d, %d commit(s):\n%s", code, len(stack.posted()), out)
	}
	out, code = exitCode(t, syncCommand(stack, binary, model, "-sync-diff", stack.server.URL))
	if code != 0 || !strings.Contains(out, "nothing to change") {
		t.Errorf("re-diff after deletes: exit %d\n%s", code, out)
	}
}

func TestSyncApplyRefusesConflicts(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def Other;
}
`))
	model := writeModel(t, t.TempDir(), "model.sysml", syncedModel)

	out, code := exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL, "-sync-confirm-deletes"))
	if code != 1 || len(stack.posted()) != 0 {
		t.Fatalf("conflict: exit %d, %d commit(s):\n%s", code, len(stack.posted()), out)
	}
	if !strings.Contains(out, "conflict 8f3a41d0") || !strings.Contains(out, "refused to apply: 1 conflict(s)") {
		t.Errorf("the conflict is not what refused the apply:\n%s", out)
	}
}

func TestSyncApplyReportsARefusedCommit(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	stack.set(func(s *fakeStack) { s.refuse = true })
	model := writeModel(t, t.TempDir(), "model.sysml", renamedModel)

	out, code := exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL))
	if code != 1 {
		t.Fatalf("a refused commit must exit 1, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "apply failed") || !strings.Contains(out, "nothing was applied") {
		t.Errorf("the failure is not reported as one:\n%s", out)
	}
	if _, err := os.Stat(model + ".sync.json"); !os.IsNotExist(err) {
		t.Error("a failed apply advanced the sync state")
	}
}

func TestSyncApplyWritesTheAnnotationOnlyOnceTheCommitLands(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	stack.set(func(s *fakeStack) { s.refuse = true })
	dir := t.TempDir()
	model := writeModel(t, dir, "model.sysml", renamedModel)
	annotated := filepath.Join(dir, "annotated.sysml")
	mint := []string{model, "-sync-apply", stack.server.URL, "-sync-mint-ids", "-sync-annotate", annotated}

	out, code := exitCode(t, syncCommand(stack, binary, mint...))
	if code != 1 || !strings.Contains(out, "apply failed") {
		t.Fatalf("refused commit: exit %d:\n%s", code, out)
	}
	if _, err := os.Stat(annotated); !os.IsNotExist(err) {
		t.Fatal("a failed apply wrote minted ids the repository never received")
	}

	stack.set(func(s *fakeStack) { s.refuse = false })
	out, code = exitCode(t, syncCommand(stack, binary, mint...))
	if code != 0 || len(stack.posted()) != 1 || !strings.Contains(out, "wrote "+annotated) {
		t.Fatalf("apply with minting: exit %d, %d commit(s):\n%s", code, len(stack.posted()), out)
	}
	data, err := os.ReadFile(annotated)
	if err != nil {
		t.Fatal(err)
	}
	minted := regexp.MustCompile(`about P::Wheel \{ id = "([^"]+)"; \}`).FindSubmatch(data)
	if minted == nil {
		t.Fatalf("the annotated model does not declare Wheel's minted id:\n%s", data)
	}
	if !strings.Contains(string(stack.posted()[0]), `"identity":{"@id":"`+string(minted[1])+`"}`) {
		t.Errorf("the id written back (%s) is not the one the repository received:\n%s", minted[1], stack.posted()[0])
	}

	// The annotated model, synced, is what the repository holds.
	out, code = exitCode(t, syncCommand(stack, binary, annotated, "-sync-diff", stack.server.URL, "-sync-state", model+".sync.json"))
	if code != 0 || !strings.Contains(out, "nothing to change") {
		t.Errorf("re-diff of the annotated model: exit %d\n%s", code, out)
	}
}

func TestSyncApplyRefusesMintedIDsItCannotWriteBack(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	model := writeModel(t, dir, "model.sysml", `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
	part : Vehicle;
}
`)
	annotated := filepath.Join(dir, "annotated.sysml")

	out, code := exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL, "-sync-mint-ids", "-sync-annotate", annotated))
	if code != 1 || len(stack.posted()) != 0 {
		t.Fatalf("unnamed minted element: exit %d, %d commit(s):\n%s", code, len(stack.posted()), out)
	}
	if !strings.Contains(out, "refused to apply") || !strings.Contains(out, "have no name") || !strings.Contains(out, "without -sync-mint-ids") {
		t.Errorf("the refusal does not explain the unnamed element:\n%s", out)
	}
	if _, err := os.Stat(annotated); !os.IsNotExist(err) {
		t.Error("a refused apply wrote the annotated model")
	}
	if _, err := os.Stat(model + ".sync.json"); !os.IsNotExist(err) {
		t.Error("a refused apply wrote sync state")
	}

	// Without minting the unnamed part keeps its derived id and the apply goes through.
	out, code = exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL))
	if code != 0 || len(stack.posted()) != 1 {
		t.Fatalf("apply without minting: exit %d, %d commit(s):\n%s", code, len(stack.posted()), out)
	}
}

func TestSyncApplyUsesTheLastSeenCommitAsBaseline(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	model := writeModel(t, dir, "model.sysml", syncedModel)
	// The fake answers every version query with the current graph, so the
	// baseline is read and nothing has changed since.
	state := writeModel(t, dir, "model.sysml.sync.json", `{"projectId":"proj-1","branch":"main","lastSeenCommit":"commit-0"}`)

	out, code := exitCode(t, syncCommand(stack, binary, model, "-sync-diff", stack.server.URL))
	if code != 0 || !strings.Contains(out, "nothing to change") || strings.Contains(out, "-sync-base") {
		t.Errorf("a live diff must read its baseline from the last-seen commit: exit %d\n%s", code, out)
	}
	_ = state
}

func TestSyncApplyRecordsTheHeadWhenNothingChanges(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	model := writeModel(t, t.TempDir(), "model.sysml", syncedModel)

	out, code := exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL))
	if code != 0 || len(stack.posted()) != 0 || !strings.Contains(out, "head commit commit-0 recorded in") {
		t.Fatalf("agreeing apply: exit %d, %d commit(s):\n%s", code, len(stack.posted()), out)
	}
	state, err := os.ReadFile(model + ".sync.json")
	if err != nil || !strings.Contains(string(state), `"lastSeenCommit": "commit-0"`) {
		t.Fatalf("sync state after an agreeing apply: %v\n%s", err, state)
	}
	out, code = exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL))
	if code != 0 || strings.Contains(out, "recorded in") {
		t.Errorf("a second agreeing apply rewrote the state: exit %d\n%s", code, out)
	}

	// Someone else renames the annotated element; the baseline now shows it.
	stack.edit(liveGraph(t, renamedModel))
	out, code = exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL, "-sync-confirm-deletes"))
	if code != 1 || len(stack.posted()) != 0 || !strings.Contains(out, "conflict 8f3a41d0") {
		t.Fatalf("repository edit after the baseline: exit %d, %d commit(s):\n%s", code, len(stack.posted()), out)
	}
	if state, _ = os.ReadFile(model + ".sync.json"); !strings.Contains(string(state), `"lastSeenCommit": "commit-0"`) {
		t.Errorf("a refused apply moved the state:\n%s", state)
	}
}

func TestSyncApplyRefusesAHeadThatMoved(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	stack.set(func(s *fakeStack) { s.drift = true })
	model := writeModel(t, t.TempDir(), "model.sysml", renamedModel)

	out, code := exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL))
	if code != 1 || len(stack.posted()) != 0 {
		t.Fatalf("moved head: exit %d, %d commit(s):\n%s", code, len(stack.posted()), out)
	}
	if !strings.Contains(out, "moved from commit commit-0 to foreign-1") || !strings.Contains(out, "nothing was applied") {
		t.Errorf("the moved head is not what refused the write:\n%s", out)
	}
	if _, err := os.Stat(model + ".sync.json"); !os.IsNotExist(err) {
		t.Error("a refused apply wrote sync state")
	}
}

func TestSyncReportsAnUnreadableRepositoryAsFailed(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	model := writeModel(t, dir, "model.sysml", renamedModel)

	state := writeModel(t, dir, "model.sysml.sync.json", `{"projectId":"proj-1","branch":"main","lastSeenCommit":"commit-gone"}`)
	out, code := exitCode(t, syncCommand(stack, binary, model, "-sync-apply", stack.server.URL))
	if code != 1 || !strings.Contains(out, "last-seen commit commit-gone") {
		t.Errorf("a missing baseline commit: exit %d, want 1:\n%s", code, out)
	}
	if err := os.Remove(state); err != nil {
		t.Fatal(err)
	}

	stack.set(func(s *fakeStack) { s.unread = true })
	for _, flag := range []string{"-sync-diff", "-sync-apply"} {
		out, code := exitCode(t, syncCommand(stack, binary, model, flag, stack.server.URL))
		if code != 1 || !strings.Contains(out, "read the repository") {
			t.Errorf("%s against an unreadable repository: exit %d, want 1:\n%s", flag, code, out)
		}
	}
	if len(stack.posted()) != 0 {
		t.Errorf("a failed read wrote %d commit(s)", len(stack.posted()))
	}
}

func TestSyncApplyNeedsALiveEndpoint(t *testing.T) {
	binary := buildCLI(t)
	stack := newFakeStack(t, liveGraph(t, syncedModel))
	dir := t.TempDir()
	model := writeModel(t, dir, "model.sysml", renamedModel)
	repo := writeModel(t, dir, "repo.ttl", string(rdf.WriteTurtle(liveGraph(t, syncedModel))))

	for _, tc := range []struct {
		name string
		cmd  *exec.Cmd
		want string
	}{
		{"graph file", syncCommand(stack, binary, model, "-sync-apply", repo), "not an http(s) endpoint"},
		{"no token", exec.Command(binary, model, "-sync-apply", stack.server.URL), flexo.EnvToken},
		{"with -sync-diff", syncCommand(stack, binary, model, "-sync-apply", stack.server.URL, "-sync-diff", repo), "one per run"},
		{"minting without write-back", syncCommand(stack, binary, model, "-sync-apply", stack.server.URL, "-sync-mint-ids"), "-sync-annotate"},
		{"empty", exec.Command(binary, model, "-sync-apply", ""), "-sync-apply is empty"},
		{"plain http off this machine", syncCommand(stack, binary, model, "-sync-apply", "http://flexo.example.invalid:8083"), flexo.EnvPlainHTTP},
		{"plain http diff too", syncCommand(stack, binary, model, "-sync-diff", "http://192.0.2.10:8083"), "in the clear"},
	} {
		out, code := exitCode(t, tc.cmd)
		if code != 2 || !strings.Contains(out, tc.want) {
			t.Errorf("%s: exit %d, want 2 mentioning %q:\n%s", tc.name, code, tc.want, out)
		}
	}
	if len(stack.posted()) != 0 {
		t.Errorf("a refused invocation wrote %d commit(s)", len(stack.posted()))
	}
}
