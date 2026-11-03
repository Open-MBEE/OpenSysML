// Package llm benchmarks how well language models write SysML v2 textual
// notation when the OpenSysML compiler closes the loop: generate, validate,
// feed the diagnostics back, repair.
package llm

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
)

//go:embed corpus/manifest.json corpus/*.sysml
var corpusFS embed.FS

// ManifestPath is the corpus manifest, relative to this package.
const ManifestPath = "corpus/manifest.json"

// Task is one natural-language modelling task with its gold model.
type Task struct {
	ID         string `json:"id"`
	Family     string `json:"family"`
	Difficulty string `json:"difficulty"`
	Title      string `json:"title"`
	Task       string `json:"task"`
	Gold       string `json:"gold"`
}

// Manifest is the committed corpus description.
type Manifest struct {
	Version     int               `json:"version"`
	Description string            `json:"description"`
	Families    map[string]string `json:"families"`
	Tasks       []Task            `json:"tasks"`
}

// Difficulties are the difficulty levels in order.
var Difficulties = []string{"simple", "medium", "complex"}

// LoadCorpus reads the embedded manifest and checks it against the embedded
// gold models.
func LoadCorpus() (*Manifest, error) {
	data, err := corpusFS.ReadFile(ManifestPath)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", ManifestPath, err)
	}
	if err := m.Check(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Check reports the first inconsistency between the manifest and the gold files.
func (m *Manifest) Check() error {
	seen := map[string]bool{}
	entries, err := corpusFS.ReadDir("corpus")
	if err != nil {
		return err
	}
	golds := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sysml") {
			golds[e.Name()] = true
		}
	}
	for _, t := range m.Tasks {
		switch {
		case t.ID == "" || t.Task == "" || t.Title == "":
			return fmt.Errorf("task %q: id, title and task are required", t.ID)
		case seen[t.ID]:
			return fmt.Errorf("task %q: duplicate id", t.ID)
		case m.Families[t.Family] == "":
			return fmt.Errorf("task %q: unknown family %q", t.ID, t.Family)
		case !slices.Contains(Difficulties, t.Difficulty):
			return fmt.Errorf("task %q: unknown difficulty %q", t.ID, t.Difficulty)
		case !golds[t.Gold]:
			return fmt.Errorf("task %q: gold %q is not in the corpus", t.ID, t.Gold)
		}
		seen[t.ID] = true
		delete(golds, t.Gold)
	}
	if len(golds) > 0 {
		names := make([]string, 0, len(golds))
		for n := range golds {
			names = append(names, n)
		}
		sort.Strings(names)
		return fmt.Errorf("gold models not in the manifest: %s", strings.Join(names, ", "))
	}
	return nil
}

// GoldSource returns the gold model of a task.
func (m *Manifest) GoldSource(t Task) (string, error) {
	data, err := corpusFS.ReadFile(path.Join("corpus", t.Gold))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Select returns the tasks whose ids are listed, or every task for an empty list.
func (m *Manifest) Select(ids []string) ([]Task, error) {
	if len(ids) == 0 {
		return m.Tasks, nil
	}
	byID := map[string]Task{}
	for _, t := range m.Tasks {
		byID[t.ID] = t
	}
	out := make([]Task, 0, len(ids))
	for _, id := range ids {
		t, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown task %q", id)
		}
		out = append(out, t)
	}
	return out, nil
}
