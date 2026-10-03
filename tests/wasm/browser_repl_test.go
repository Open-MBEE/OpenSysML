package wasm

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// replGzipBudget bounds the gzipped sysml_prod js build the CLI page downloads
// for its in-browser REPL.
const replGzipBudget = 13000000

// TestBrowserREPLWalkthroughs runs every walkthrough the CLI page offers
// (docs/assets/repl-walkthroughs.json) through the page's own host,
// docs/assets/sysml-repl.js, on the sysml_prod js build the Makefile ships,
// and requires each step's output to say what the step's narration promises.
func TestBrowserREPLWalkthroughs(t *testing.T) {
	requireNode(t)
	js := wasmTarget{name: "js", goos: "js"}
	bin := filepath.Join(t.TempDir(), "sysml-repl.wasm")
	if out, err := goFor(t, js, "build", "-tags", "sysml_prod", "-ldflags", ldflags, "-o", bin, "./cmd/sysml"); err != nil {
		t.Fatalf("building cmd/sysml -tags sysml_prod for js: %v\n%s", err, out)
	}
	assets := filepath.Join("..", "..", "docs", "assets")
	toursFile := filepath.Join(assets, "repl-walkthroughs.json")
	raw, err := os.ReadFile(toursFile)
	if err != nil {
		t.Fatalf("reading the walkthroughs: %v", err)
	}
	var data struct {
		Tours []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Steps []struct {
				Title  string   `json:"title"`
				Text   string   `json:"text"`
				Input  []string `json:"input"`
				Expect []string `json:"expect"`
			} `json:"steps"`
		} `json:"tours"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("decoding the walkthroughs: %v", err)
	}
	if len(data.Tours) == 0 {
		t.Fatal("the walkthroughs file holds no walkthrough")
	}
	type key struct {
		tour string
		step int
	}
	want := map[key][]string{}
	for _, tour := range data.Tours {
		if tour.ID == "" || tour.Title == "" || len(tour.Steps) == 0 {
			t.Errorf("walkthrough %q needs an id, a title and steps", tour.ID)
		}
		for i, step := range tour.Steps {
			if step.Title == "" || step.Text == "" || len(step.Input) == 0 || len(step.Expect) == 0 {
				t.Errorf("%s step %d needs a title, text, input and what its output must say", tour.ID, i+1)
			}
			if _, dup := want[key{tour.ID, i}]; dup {
				t.Errorf("walkthrough id %q is used twice", tour.ID)
			}
			want[key{tour.ID, i}] = step.Expect
		}
	}

	abs := func(p string) string {
		a, err := filepath.Abs(p)
		if err != nil {
			t.Fatalf("resolving %s: %v", p, err)
		}
		return a
	}
	got := nodeRun(t, fixture(t, "browser-repl.mjs"), wasmExecJS(t), abs(filepath.Join(assets, "sysml-repl.js")),
		bin, abs(toursFile), abs(filepath.Join("..", "..", "examples", "runtime-showcase")))
	seen := 0
	for _, line := range strings.Split(got.output, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var step struct {
			Tour   string `json:"tour"`
			Step   int    `json:"step"`
			Output string `json:"output"`
		}
		if err := json.Unmarshal([]byte(line), &step); err != nil {
			t.Fatalf("decoding a step report: %v\n%s", err, line)
		}
		expect, ok := want[key{step.Tour, step.Step}]
		if !ok {
			t.Errorf("the host reported a step the walkthroughs do not have: %s %d", step.Tour, step.Step)
			continue
		}
		seen++
		for _, w := range expect {
			if !strings.Contains(step.Output, w) {
				t.Errorf("%s step %d output is missing %q:\n%s", step.Tour, step.Step+1, w, step.Output)
			}
		}
		if strings.Contains(step.Output, "panic:") {
			t.Errorf("%s step %d panicked:\n%s", step.Tour, step.Step+1, step.Output)
		}
	}
	if seen != len(want) {
		t.Errorf("the host ran %d of the %d walkthrough steps:\n%s", seen, len(want), got.output)
	}

	t.Run("fits the browser REPL size budget", func(t *testing.T) {
		data, err := os.ReadFile(bin)
		if err != nil {
			t.Fatalf("reading %s: %v", bin, err)
		}
		var compressed bytes.Buffer
		w, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
		if err != nil {
			t.Fatalf("gzip writer: %v", err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("compressing: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("compressing: %v", err)
		}
		if size := compressed.Len(); size > replGzipBudget {
			t.Errorf("gzipped sysml_prod sysml.wasm is %d bytes, over the %d-byte budget the CLI page downloads", size, replGzipBudget)
		}
	})
}
