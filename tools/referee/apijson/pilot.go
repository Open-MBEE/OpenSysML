package apijson

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

// pilotExport is one corpus root exported through the pinned pilot.
type pilotExport struct {
	Elements []*element
	// Failed names the files the pilot read or exported with an exception.
	Failed []string
	// Library counts the standard-library elements the pilot wrote, which this
	// repository's writer never does; each file that refers to one writes it again.
	Library int
	// Duplicates counts the identical copies dropped; Ambiguous the elements two
	// of which share a key but differ; Shared names the first such keys.
	Duplicates int
	Ambiguous  int
	Shared     []string
	// Unkeyable counts the elements the bridge's key table does not address,
	// Unkeyed per metaclass.
	Unkeyable int
	Unkeyed   map[string]int
	// Log keeps the launcher's diagnostics for the text report.
	Log string
}

// pilotDocument is what the bridge writes per input file.
type pilotDocument struct {
	File     string `json:"file"`
	Elements []struct {
		Payload map[string]any `json:"payload"`
	} `json:"elements"`
	Keys idKeys `json:"keys"`
}

// readPilotDocument decodes one bridge document and spells its keys unquoted
// (see canonicalKey).
func readPilotDocument(content []byte) (pilotDocument, error) {
	var document pilotDocument
	if err := json.Unmarshal(content, &document); err != nil {
		return pilotDocument{}, err
	}
	for id, key := range document.Keys {
		document.Keys[id] = canonicalKey(key)
	}
	return document, nil
}

// pilotArgs spells the bridge's command line: the pinned standard library is
// the launcher's, every extra library directory is named explicitly, then the
// root's files.
func pilotArgs(dir, out string, libraries, files []string) []string {
	args := []string{"--root", dir, "--out", out}
	for _, library := range libraries {
		args = append(args, "--extension-library", library)
	}
	for _, rel := range files {
		args = append(args, filepath.Join(dir, filepath.FromSlash(rel)))
	}
	return args
}

// runPilot exports one root's files in one pilot process, so the pilot
// resolves cross-file imports the way it resolves ours, then reads the per-file
// documents it wrote.
func runPilot(launcher, dir string, libraries, files []string, timeout time.Duration) (*pilotExport, error) {
	out, err := os.MkdirTemp("", toolName+"-pilot-*")
	if err != nil {
		return nil, fmt.Errorf("create pilot output: %w", err)
	}
	defer os.RemoveAll(out)
	args := pilotArgs(dir, out, libraries, files)
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
	result := &pilotExport{Log: pilotLog(stderr.String()), Unkeyed: make(map[string]int)}
	for _, rel := range files {
		path := filepath.Join(out, filepath.FromSlash(rel)+".json")
		content, err := os.ReadFile(path) // #nosec G304 -- the bridge wrote it under our temporary directory
		if err != nil {
			if os.IsNotExist(err) {
				result.Failed = append(result.Failed, rel)
				continue
			}
			return nil, fmt.Errorf("read pilot export %s: %w", rel, err)
		}
		document, err := readPilotDocument(content)
		if err != nil {
			return nil, fmt.Errorf("parse pilot export %s: %w", rel, err)
		}
		result.Elements = append(result.Elements, keyPilot(document, result)...)
	}
	result.Elements, result.Duplicates, result.Ambiguous, result.Shared = dropAmbiguous(result.Elements)
	return result, nil
}

// pilotLog keeps the bridge's per-file diagnostics and drops the logging
// framework's start-up noise.
func pilotLog(stderr string) string {
	var lines []string
	for _, line := range strings.Split(stderr, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "log4j:") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// keyPilot turns one bridge document into keyed elements: the bridge's key
// table addresses every element and reference it wrote, library ones included.
func keyPilot(document pilotDocument, result *pilotExport) []*element {
	elements := make([]*element, 0, len(document.Elements))
	for _, item := range document.Elements {
		payload := item.Payload
		id, _ := payload["@id"].(string)
		key := document.Keys[id]
		typ, _ := payload["@type"].(string)
		if payload["isLibraryElement"] == true {
			result.Library++
			continue
		}
		if key == "" {
			result.Unkeyable++
			result.Unkeyed[typ]++
			continue
		}
		e := &element{Key: key, Type: typ, File: document.File, Implied: payload["isImplied"] == true, Props: make(map[string]any, len(payload))}
		for property, value := range payload {
			if property == "@id" || property == "@type" {
				continue
			}
			e.Props[property] = normalizeValue(property, value, document.Keys)
		}
		elements = append(elements, e)
	}
	return elements
}

// ambiguousSamples is how many shared keys a side's report names.
const ambiguousSamples = 8

// dropAmbiguous keeps one of several identical copies of an element (the pilot
// writes an element in every file whose traversal reaches it) and removes every
// element whose key another, different element shares, since neither could be
// matched with confidence; it counts both and names the first shared keys.
func dropAmbiguous(elements []*element) (kept []*element, duplicates, ambiguous int, shared []string) {
	byKey := make(map[string][]*element, len(elements))
	for _, e := range elements {
		byKey[e.Key] = append(byKey[e.Key], e)
	}
	kept = make([]*element, 0, len(elements))
	seen := make(map[string]bool, len(byKey))
	for _, e := range elements {
		if seen[e.Key] {
			continue
		}
		seen[e.Key] = true
		copies := byKey[e.Key]
		if len(copies) == 1 {
			kept = append(kept, e)
			continue
		}
		identical := true
		for _, other := range copies[1:] {
			if other.Type != e.Type || !reflect.DeepEqual(other.Props, e.Props) {
				identical = false
				break
			}
		}
		if identical {
			duplicates += len(copies) - 1
			kept = append(kept, e)
			continue
		}
		ambiguous += len(copies)
		if len(shared) < ambiguousSamples {
			shared = append(shared, e.Key+" ("+e.File+")")
		}
	}
	return kept, duplicates, ambiguous, shared
}
