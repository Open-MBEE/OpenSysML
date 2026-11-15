package hygiene

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// The fUML suite's jars come from Maven Central, which answers a burst of parallel cold fetches
// with HTTP 429: one job fills the cache before the jobs that read it, and a fetch backs off.

const (
	corporaAction   = "./.github/actions/corpora"
	fumlSuiteAction = "./.github/actions/fuml-suite"
	fumlDownload    = "./scripts/download-fuml-suite.sh"
)

func TestFUMLCacheIsFilledBeforeTheJobsThatReadIt(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)

	var warm *actionsStep
	changes := workflow.Jobs["changes"]
	for i, step := range changes.Steps {
		if step.Uses == fumlSuiteAction {
			warm = &changes.Steps[i]
		}
	}
	if warm == nil {
		t.Fatalf("pr.yml changes job has no %s step: a cold cache sends every job to Maven Central", fumlSuiteAction)
	}
	if warm.With["warm-only"] != "true" {
		t.Errorf("the changes job's %s step has warm-only %q, want true", fumlSuiteAction, warm.With["warm-only"])
	}

	readers := 0
	for name, job := range workflow.Jobs {
		reads := false
		for _, step := range job.Steps {
			reads = reads || step.Uses == corporaAction
		}
		if !reads {
			continue
		}
		readers++
		needsChanges := false
		for _, need := range job.needs() {
			needsChanges = needsChanges || need == "changes"
		}
		if !needsChanges {
			t.Errorf("pr.yml job %s provisions the corpora without needing changes, so it can start on a cold fUML cache", name)
		}
		// The warm step's condition has to cover the area that turns the reader on.
		for _, area := range []string{"go", "apijson"} {
			if strings.Contains(job.If, "needs.changes.outputs."+area+" == 'true'") &&
				!strings.Contains(warm.If, "steps.areas.outputs."+area+" == 'true'") {
				t.Errorf("pr.yml job %s runs for the %s area, which the warm step's condition %q leaves out", name, area, warm.If)
			}
		}
	}
	if readers == 0 {
		t.Fatalf("no pr.yml job uses %s", corporaAction)
	}
}

func TestFUMLSuiteActionOwnsTheCacheAndTheDownload(t *testing.T) {
	action, _ := loadCompositeAction(t, "../../.github/actions/fuml-suite/action.yml")
	var cache, download *actionsStep
	for i, step := range action.Runs.Steps {
		switch {
		case strings.HasPrefix(step.Uses, "actions/cache@"):
			cache = &action.Runs.Steps[i]
		case strings.Contains(step.Run, fumlDownload):
			download = &action.Runs.Steps[i]
		}
	}
	if cache == nil || download == nil {
		t.Fatal("the fuml-suite action needs a cache step and a step running " + fumlDownload)
	}
	if cache.With["path"] != "build/fuml" || !strings.Contains(cache.With["lookup-only"], "inputs.warm-only") {
		t.Errorf("the cache step caches %q with lookup-only %q, want build/fuml looked up only under warm-only",
			cache.With["path"], cache.With["lookup-only"])
	}
	for _, pinned := range []string{"scripts/download-fuml-suite.sh", "scripts/fuml-pin.sh"} {
		if !strings.Contains(cache.With["key"], "'"+pinned+"'") {
			t.Errorf("the cache key %q does not hash %s", cache.With["key"], pinned)
		}
	}
	// A warm-only hit restored nothing, so only then may the download be skipped.
	want := "inputs.warm-only != 'true' || steps.cache.outputs.cache-hit != 'true'"
	if download.If != want {
		t.Errorf("the download step runs if %q, want %q", download.If, want)
	}

	// One cache step and one download: a second copy elsewhere would miss the warm entry's key.
	for _, path := range actionsFiles(t) {
		for where, steps := range actionsSteps(t, path) {
			for _, step := range steps {
				if strings.Contains(step.Run, "download-fuml-suite.sh") && !strings.HasSuffix(path, "fuml-suite/action.yml") {
					t.Errorf("%s step %q runs the fUML download outside %s", where, step.Name, fumlSuiteAction)
				}
			}
		}
	}
	uses := false
	for _, steps := range actionsSteps(t, "../../.github/actions/corpora/action.yml") {
		for _, step := range steps {
			uses = uses || step.Uses == fumlSuiteAction
		}
	}
	if !uses {
		t.Errorf("the corpora action does not use %s", fumlSuiteAction)
	}
}

// fetchHTTPS runs scripts/fetch-https.sh against a TLS server answering each request with the next status.
func fetchHTTPS(t *testing.T, statuses ...int) (body string, requests int32, err error) {
	t.Helper()
	if _, lookErr := exec.LookPath("curl"); lookErr != nil {
		t.Skip("curl is not on PATH")
	}
	var served atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		status := statuses[min(int(served.Add(1)), len(statuses))-1]
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write([]byte("pinned\n"))
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	bundle := filepath.Join(dir, "ca.pem")
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if writeErr := os.WriteFile(bundle, cert, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	dest := filepath.Join(dir, "fetched")
	cmd := exec.Command("../../scripts/fetch-https.sh", server.URL+"/file", dest)
	cmd.Env = append(os.Environ(), "CURL_CA_BUNDLE="+bundle)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("fetch-https.sh: %s", out)
	}
	raw, _ := os.ReadFile(dest)
	return string(raw), served.Load(), err
}

func TestFetchHTTPSRetriesARateLimitedHost(t *testing.T) {
	body, requests, err := fetchHTTPS(t, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusOK)
	if err != nil {
		t.Fatalf("a host answering 429, 503, then 200 failed the fetch after %d requests: %v", requests, err)
	}
	if body != "pinned\n" || requests != 3 {
		t.Errorf("fetched %q in %d requests, want the file on the third", body, requests)
	}
}

func TestFetchHTTPSKeepsItsCurlPolicy(t *testing.T) {
	raw, err := os.ReadFile("../../scripts/fetch-https.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{" -fsSL ", "--proto '=https'", "--proto-redir '=https'", "--retry 8 "} {
		if !strings.Contains(string(raw), flag) {
			t.Errorf("scripts/fetch-https.sh no longer passes curl %q", strings.TrimSpace(flag))
		}
	}
}
