package repl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// fakeAPI is an in-memory SysML v2 API server speaking the standard resources
// the pilot's does: projects, branches, commits with change payloads, elements
// and roots, paged by a Link rel="next" header. No token, no Layer 1.
type fakeAPI struct {
	mu       sync.Mutex
	server   *httptest.Server
	projects map[string]*fakeProject
	order    []string // project ids in creation order
	pageSize int      // elements and projects per page
	nextID   int
	// conflict, when set, answers the next commit with 409 and this message.
	conflict string
	requests []string
}

type fakeProject struct {
	id, name string
	branches map[string]*fakeBranch
	commits  map[string]map[string]json.RawMessage // commit id → element id → element
	defaults string
}

type fakeBranch struct {
	id, name, head string
}

func newFakeAPI() *fakeAPI {
	f := &fakeAPI{projects: map[string]*fakeProject{}, pageSize: 2}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	return f
}

func (f *fakeAPI) Close() { f.server.Close() }

func (f *fakeAPI) URL() string { return f.server.URL }

func (f *fakeAPI) id(kind string) string {
	f.nextID++
	return fmt.Sprintf("%s-%04d", kind, f.nextID)
}

// addProject creates an empty project with a default branch, as the API's POST
// /projects does, and returns it.
func (f *fakeAPI) addProject(name string) *fakeProject {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createProject(name, "main")
}

func (f *fakeAPI) createProject(name, branch string) *fakeProject {
	p := &fakeProject{id: f.id("project"), name: name, branches: map[string]*fakeBranch{}, commits: map[string]map[string]json.RawMessage{}}
	b := &fakeBranch{id: f.id("branch"), name: branch}
	p.branches[b.id] = b
	p.defaults = b.id
	f.projects[p.id] = p
	f.order = append(f.order, p.id)
	return p
}

// addBranch gives a project a branch at the head of its default branch.
func (f *fakeAPI) addBranch(project, name string) *fakeBranch {
	p := f.projects[project]
	b := &fakeBranch{id: f.id("branch"), name: name, head: p.branches[p.defaults].head}
	p.branches[b.id] = b
	return b
}

func (f *fakeAPI) projectJSON(p *fakeProject) map[string]any {
	return map[string]any{"@id": p.id, "@type": "Project", "name": p.name, "defaultBranch": map[string]any{"@id": p.defaults}}
}

func (f *fakeAPI) branchJSON(b *fakeBranch) map[string]any {
	out := map[string]any{"@id": b.id, "@type": "Branch", "name": b.name, "head": nil}
	if b.head != "" {
		out["head"] = map[string]any{"@id": b.head, "@type": "Commit"}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeAPI) fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

// page answers one page of items, linking to the next when there is one.
func (f *fakeAPI) page(w http.ResponseWriter, r *http.Request, items []any, idOf func(any) string) {
	after := r.URL.Query().Get("page[after]")
	if after == "" {
		after = r.URL.Query().Get("pageAfter")
	}
	start := 0
	if after != "" {
		for i, item := range items {
			if idOf(item) == after {
				start = i + 1
			}
		}
	}
	end := start + f.pageSize
	if end > len(items) {
		end = len(items)
	}
	if end < len(items) {
		next := *r.URL
		q := next.Query()
		q.Set("page[after]", idOf(items[end-1]))
		q.Set("page[size]", strconv.Itoa(f.pageSize))
		next.RawQuery = q.Encode()
		w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, next.String()))
	}
	writeJSON(w, http.StatusOK, items[start:end])
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != "" {
		f.fail(w, http.StatusBadRequest, "this server takes no token")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if parts[0] != "projects" {
		f.fail(w, http.StatusNotFound, "no such resource")
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			items := make([]any, 0, len(f.order))
			for _, id := range f.order {
				items = append(items, f.projectJSON(f.projects[id]))
			}
			f.page(w, r, items, func(v any) string { return v.(map[string]any)["@id"].(string) })
		case http.MethodPost:
			var req struct {
				Name          string `json:"name"`
				DefaultBranch *struct {
					Name string `json:"name"`
				} `json:"defaultBranch"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
				f.fail(w, http.StatusBadRequest, "a project needs a name")
				return
			}
			branch := "main"
			if req.DefaultBranch != nil && req.DefaultBranch.Name != "" {
				branch = req.DefaultBranch.Name
			}
			writeJSON(w, http.StatusOK, f.projectJSON(f.createProject(req.Name, branch)))
		default:
			f.fail(w, http.StatusMethodNotAllowed, r.Method)
		}
		return
	}
	p, ok := f.projects[parts[1]]
	if !ok {
		f.fail(w, http.StatusNotFound, "project "+parts[1]+" not found")
		return
	}
	if len(parts) == 2 {
		writeJSON(w, http.StatusOK, f.projectJSON(p))
		return
	}
	switch parts[2] {
	case "branches":
		if len(parts) == 3 {
			ids := make([]string, 0, len(p.branches))
			for id := range p.branches {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			items := make([]any, 0, len(ids))
			for _, id := range ids {
				items = append(items, f.branchJSON(p.branches[id]))
			}
			f.page(w, r, items, func(v any) string { return v.(map[string]any)["@id"].(string) })
			return
		}
		b, ok := p.branches[parts[3]]
		if !ok {
			f.fail(w, http.StatusNotFound, "branch "+parts[3]+" not found")
			return
		}
		writeJSON(w, http.StatusOK, f.branchJSON(b))
	case "commits":
		f.serveCommits(w, r, p, parts[3:])
	default:
		f.fail(w, http.StatusNotFound, "no such resource")
	}
}

func (f *fakeAPI) serveCommits(w http.ResponseWriter, r *http.Request, p *fakeProject, rest []string) {
	if len(rest) == 0 {
		if r.Method != http.MethodPost {
			f.fail(w, http.StatusMethodNotAllowed, r.Method)
			return
		}
		if f.conflict != "" {
			msg := f.conflict
			f.conflict = ""
			f.fail(w, http.StatusConflict, msg)
			return
		}
		b, ok := p.branches[r.URL.Query().Get("branchId")]
		if !ok {
			b = p.branches[p.defaults]
		}
		var req struct {
			Change []struct {
				Identity struct {
					ID string `json:"@id"`
				} `json:"identity"`
				Payload json.RawMessage `json:"payload"`
			} `json:"change"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			f.fail(w, http.StatusBadRequest, err.Error())
			return
		}
		elements := map[string]json.RawMessage{}
		for id, element := range p.commits[b.head] {
			elements[id] = element
		}
		for _, change := range req.Change {
			if string(change.Payload) == "null" || len(change.Payload) == 0 {
				delete(elements, change.Identity.ID)
				continue
			}
			elements[change.Identity.ID] = change.Payload
		}
		commit := f.id("commit")
		p.commits[commit] = elements
		b.head = commit
		writeJSON(w, http.StatusOK, map[string]any{"@id": commit, "@type": "Commit"})
		return
	}
	elements, ok := p.commits[rest[0]]
	if !ok {
		f.fail(w, http.StatusNotFound, "commit "+rest[0]+" not found")
		return
	}
	ids := make([]string, 0, len(elements))
	for id := range elements {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(rest) < 2 {
		f.fail(w, http.StatusNotFound, "no such resource")
		return
	}
	switch rest[1] {
	case "elements":
		if len(rest) == 3 {
			element, ok := elements[rest[2]]
			if !ok {
				f.fail(w, http.StatusNotFound, "element "+rest[2]+" not found")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(element)
			return
		}
		items := make([]any, 0, len(ids))
		for _, id := range ids {
			items = append(items, elements[id])
		}
		f.page(w, r, items, func(v any) string {
			var e struct {
				ID string `json:"@id"`
			}
			_ = json.Unmarshal(v.(json.RawMessage), &e)
			return e.ID
		})
	case "roots":
		var roots []json.RawMessage
		for _, id := range ids {
			var e struct {
				Owner *struct {
					ID string `json:"@id"`
				} `json:"owner"`
			}
			_ = json.Unmarshal(elements[id], &e)
			if e.Owner == nil {
				roots = append(roots, elements[id])
			}
		}
		writeJSON(w, http.StatusOK, roots)
	default:
		f.fail(w, http.StatusNotFound, "no such resource")
	}
}

// elementCount is how many elements a project's branch head holds.
func (f *fakeAPI) elementCount(project, branch string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.projects[project]
	return len(p.commits[p.branches[branch].head])
}

// heldBytes is how many elements a project's branch head holds, and how many
// bytes of JSON they amount to.
func (f *fakeAPI) heldBytes(project, branch string) (count, size int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.projects[project]
	for _, element := range p.commits[p.branches[branch].head] {
		count++
		size += len(element)
	}
	return count, size
}
