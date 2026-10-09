package jupyter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRepositoryCommandsRunThroughTheKernel drives %repo and %projects against
// a fake SysML v2 API server: the listing streams as text, an argument problem
// is a UsageError, and a server failure a CommandError naming the status.
func TestRepositoryCommandsRunThroughTheKernel(t *testing.T) {
	var projects []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"the database is away"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(projects)
	}))
	defer server.Close()
	t.Setenv("FLEXO_SYSMLV2_URL", server.URL)
	t.Setenv("FLEXO_INTEROP_TOKEN", "")

	e := newEngine(t)
	if got := mustRun(t, e, "%repo").text(); got != "stdout: API base path: "+server.URL+"\n" {
		t.Errorf("%%repo streamed %q", got)
	}
	if got := mustRun(t, e, "%projects").text(); got != "stdout: no projects\n" {
		t.Errorf("%%projects over an empty repository streamed %q", got)
	}
	projects = []map[string]any{{"@id": "p-1", "name": "Rover", "defaultBranch": map[string]any{"@id": "b-1"}}}
	if got := mustRun(t, e, "%projects").text(); got != "stdout: Rover (p-1)\n" {
		t.Errorf("%%projects streamed %q", got)
	}

	if failure := mustFail(t, e, "%projects now"); failure.Name != "UsageError" || !strings.HasPrefix(failure.Value, "usage: %projects") {
		t.Errorf("%%projects now: %s: %s", failure.Name, failure.Value)
	}
	failure := mustFail(t, e, "%load --id=p-1")
	if failure.Name != "CommandError" || !strings.Contains(failure.Value, "Internal Server Error") || !strings.Contains(failure.Value, "the database is away") {
		t.Errorf("a server failure: %s: %s", failure.Name, failure.Value)
	}
}
