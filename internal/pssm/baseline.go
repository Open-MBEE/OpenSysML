package pssm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BaselinePath is the committed baseline, relative to the repository root.
const BaselinePath = "docs/project/pssm-referee-baseline.json"

// Encode renders a report as the bytes the baseline holds: indented JSON with a
// fixed key order, so two runs with equal results are byte-identical.
func (r *Report) Encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteBaseline records a fresh report as the committed baseline.
func WriteBaseline(path string, r *Report) error {
	fresh, err := r.Encode()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, fresh, 0o600)
}

// ReadBaseline decodes a committed baseline.
func ReadBaseline(path string) (*Report, error) {
	content, err := os.ReadFile(path) // #nosec G304 -- callers name a committed baseline in this repository
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(content, &r); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &r, nil
}

// Reproduces is the gate CI applies: the fresh run must measure the same suite
// as the committed baseline and land the same count in every bucket. The rows
// are not compared — they are for whoever adjudicates a moved count — and the
// date and develop commit say when and on what the baseline was recorded.
func Reproduces(committed, fresh *Report) error {
	var differences []string
	was, now := committed.Provenance, fresh.Provenance
	was.Recorded, now.Recorded = "", ""
	was.Develop, now.Develop = "", ""
	if was != now {
		differences = append(differences, fmt.Sprintf("provenance: baseline measured %+v, this run %+v", was, now))
	}
	for _, b := range Buckets {
		if committed.Buckets[b] != fresh.Buckets[b] {
			differences = append(differences, fmt.Sprintf("%s: baseline %d, this run %d", b, committed.Buckets[b], fresh.Buckets[b]))
		}
	}
	if len(differences) == 0 {
		return nil
	}
	moved := movedTests(committed, fresh)
	if len(moved) > 0 {
		differences = append(differences, "moved: "+strings.Join(moved, ", "))
	}
	if !strings.HasPrefix(differences[0], "provenance") {
		differences = append(differences, "the provenance matches, so this is a movement of the runtime or the translation: adjudicate it, then re-record with -update")
	} else {
		differences = append(differences, "the suite measured is not the baseline's: investigate the provisioning before re-recording")
	}
	return fmt.Errorf("%s does not reproduce:\n  %s", BaselinePath, strings.Join(differences, "\n  "))
}

// movedTests names the tests whose bucket changed, with both buckets.
func movedTests(committed, fresh *Report) []string {
	was := make(map[string]Bucket, len(committed.Tests))
	for _, t := range committed.Tests {
		was[t.Name] = t.Bucket
	}
	var out []string
	for _, t := range fresh.Tests {
		if b, ok := was[t.Name]; ok && b != t.Bucket {
			out = append(out, fmt.Sprintf("%s %s -> %s", t.Name, b, t.Bucket))
		}
	}
	return out
}
