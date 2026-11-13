package derived

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// PilotCensus is the pilot's side of one root: what its export states for
// every served rule over the elements it wrote.
type PilotCensus struct {
	Elements int
	// Implied counts the relationships the pilot writes as isImplied.
	Implied int
	// Duplicates counts the copies dropped: the pilot writes an element in every
	// file whose traversal reaches it, and a root's elements are counted once.
	Duplicates int
	Failed     []string
	Rules      map[string]Counts
	seen       map[string]bool
}

// pilotDocument is what the exporter bridge writes per input file.
type pilotDocument struct {
	Elements []struct {
		Payload map[string]any `json:"payload"`
	} `json:"elements"`
}

// metaclasses answers conformance between metaclass names from the standard
// library's metaclass declarations alone, as the symbol-backed Structure does.
type metaclasses struct{ structure *metamodel.SymbolStructure }

func newMetaclasses() metaclasses {
	idx := libs.NewModelIndex()
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	return metaclasses{structure: metamodel.NewSymbolStructure(res, model)}
}

func (m metaclasses) conforms(class, defining string) bool {
	return m.structure.Specializes(class, defining)
}

// runExporter exports one root's files in one pilot process, so the pilot
// resolves cross-file imports as it does for ours, and reads what it wrote.
func runExporter(launcher, dir, libraries string, files []string, timeout time.Duration, classes metaclasses) (*PilotCensus, error) {
	out, err := os.MkdirTemp("", "derived-census-pilot-*")
	if err != nil {
		return nil, fmt.Errorf("create pilot output: %w", err)
	}
	defer os.RemoveAll(out)
	args := []string{"--root", dir, "--out", out, "--extension-library", libraries}
	for _, rel := range files {
		args = append(args, filepath.Join(dir, filepath.FromSlash(rel)))
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// #nosec G204 -- launcher is an explicit CLI override or repo-local artifact.
	command := exec.CommandContext(ctx, launcher, args...)
	var stderr bytes.Buffer
	command.Stdout, command.Stderr = os.Stdout, &stderr
	runErr := command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("pilot export of %s exceeded %s", dir, timeout)
	}
	var exit *exec.ExitError
	if runErr != nil && (!errors.As(runErr, &exit) || exit.ExitCode() != 1) {
		return nil, fmt.Errorf("pilot export of %s: %w\n%s", dir, runErr, strings.TrimSpace(stderr.String()))
	}
	census := &PilotCensus{Rules: map[string]Counts{}, seen: map[string]bool{}}
	rules := metamodel.Rules()
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)+".json")) // #nosec G304 -- the bridge wrote it under our temporary directory
		if err != nil {
			if os.IsNotExist(err) {
				census.Failed = append(census.Failed, rel)
				continue
			}
			return nil, fmt.Errorf("read pilot export %s: %w", rel, err)
		}
		var document pilotDocument
		if err := json.Unmarshal(content, &document); err != nil {
			return nil, fmt.Errorf("parse pilot export %s: %w", rel, err)
		}
		for _, item := range document.Elements {
			countPilotElement(item.Payload, rules, classes, census)
		}
	}
	return census, nil
}

// countPilotElement tallies one exported element against every served rule
// its metaclass carries, once per id. Library elements the pilot writes
// alongside a file's own are not the file's, as our walk never reaches them.
func countPilotElement(payload map[string]any, rules []metamodel.Rule, classes metaclasses, census *PilotCensus) {
	if payload["isLibraryElement"] == true {
		return
	}
	class, _ := payload["@type"].(string)
	id, _ := payload["@id"].(string)
	if class == "" || id == "" {
		return
	}
	if census.seen[id] {
		census.Duplicates++
		return
	}
	census.seen[id] = true
	census.Elements++
	if payload["isImplied"] == true {
		census.Implied++
	}
	for _, rule := range rules {
		if !classes.conforms(class, rule.DefiningClass) {
			continue
		}
		counts := census.Rules[ruleName(rule)]
		counts.Carriers++
		value, present := payload[rule.Property]
		switch {
		case !present:
			counts.NotDerived++
		case isAbsent(value):
			counts.Empty++
		default:
			counts.Value++
		}
		census.Rules[ruleName(rule)] = counts
	}
}

// isAbsent is a payload value stating nothing: null or an empty array.
func isAbsent(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case []any:
		return len(v) == 0
	}
	return false
}
