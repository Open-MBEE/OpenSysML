// Package baseline is what makes a committed oracle baseline checkable without
// the Java validators: every baseline records which pin, which bridge sources
// and which corpora its run measured, and this package both writes that record
// and compares a committed one against the repository as it stands now.
package baseline

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs/errata"
)

// PinPath is the single source of the pilot pin every oracle resolves its
// validators and corpora from.
const PinPath = "scripts/pilot-pin.sh"

// digestPrefix names the hash every digest this package writes was taken with.
const digestPrefix = "sha256:"

// Origin says who owns an input, which is what decides the action a mismatch
// calls for: our own material moving is a movement to adjudicate, the pinned
// reference moving underneath a baseline is a provisioning defect to investigate.
const (
	OriginOurs   = "ours"
	OriginPinned = "pinned"
)

// Record is the provenance block a baseline carries: the identities of the run's
// inputs, never their machine-specific paths, so any checkout can check it.
type Record struct {
	PilotTag string `json:"pilotTag"`
	// PilotCommit is the commit the tag resolved to when the run's material was
	// fetched; the tag alone is a mutable name for it.
	PilotCommit   string `json:"pilotCommit"`
	PilotArtifact string `json:"pilotArtifact"`
	// Errata is the digest of the declared overlay, which changes what the
	// oracles read and therefore what they measure.
	Errata string  `json:"errataRegistry"`
	Tools  []Tool  `json:"tools,omitempty"`
	Inputs []Input `json:"inputs"`
	// Recorded is the date the baseline was committed, stamped by the -update
	// flag. A plain run leaves it empty so two runs stay byte-identical.
	Recorded string `json:"recorded,omitempty"`
}

// Tool is one reference bridge: the pilot release it was built against, and the
// digest of the in-repository source it was compiled from.
type Tool struct {
	Name         string `json:"name"`
	Source       string `json:"source"`
	SourceDigest string `json:"sourceDigest"`
	Release      string `json:"release"`
}

// Input is one compared corpus root, identified by the digest of the files the
// run actually read rather than by its directory's contents on any one machine.
type Input struct {
	Name   string `json:"name"`
	Dir    string `json:"dir"`
	Origin string `json:"origin"`
	Files  int    `json:"files"`
	Digest string `json:"digest"`
}

// Pin is the resolved pilot pin.
type Pin struct {
	Tag      string
	Commit   string
	Artifact string
}

// Release is the pin as the validators' pilot-pin.txt states it.
func (p Pin) Release() string {
	return fmt.Sprintf("%s (jupyter-sysml-kernel %s)", p.Tag, p.Artifact)
}

var (
	pinTagRe      = regexp.MustCompile(`PILOT_TAG="\$\{PILOT_TAG:-([^}"]*)\}"`)
	pinCommitRe   = regexp.MustCompile(`PILOT_COMMIT="\$\{PILOT_COMMIT:-([0-9a-f]{40})\}"`)
	pinArtifactRe = regexp.MustCompile(`PILOT_ARTIFACT_VERSION="\$\{PILOT_ARTIFACT_VERSION:-([^}"]*)\}"`)
)

// ReadPin resolves the pin from scripts/pilot-pin.sh.
func ReadPin(root string) (Pin, error) {
	path := filepath.Join(root, filepath.FromSlash(PinPath))
	content, err := os.ReadFile(path) // #nosec G304 -- the pin is at a fixed path in this repository
	if err != nil {
		return Pin{}, fmt.Errorf("read %s: %w", PinPath, err)
	}
	tag := pinTagRe.FindSubmatch(content)
	commit := pinCommitRe.FindSubmatch(content)
	artifact := pinArtifactRe.FindSubmatch(content)
	if tag == nil || commit == nil || artifact == nil {
		return Pin{}, fmt.Errorf("%s pins no PILOT_TAG/PILOT_COMMIT/PILOT_ARTIFACT_VERSION", PinPath)
	}
	return Pin{Tag: string(tag[1]), Commit: string(commit[1]), Artifact: string(artifact[1])}, nil
}

// Today is the stamp -update writes into a baseline.
func Today() string { return time.Now().UTC().Format("2006-01-02") }

// DigestFile is the digest of one file's bytes.
func DigestFile(path string) (string, error) {
	file, err := os.Open(path) // #nosec G304 -- callers name repository files
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", err
	}
	return digestPrefix + hex.EncodeToString(sum.Sum(nil)), nil
}

// DigestFiles is the digest of a named set of files: a hash over each path and
// the digest of its contents, so a renamed, added or edited file all move it.
// The paths are slash-separated and relative to dir.
func DigestFiles(dir string, rel []string) (string, error) {
	sorted := append([]string(nil), rel...)
	sort.Strings(sorted)
	sum := sha256.New()
	for _, name := range sorted {
		digest, err := DigestFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(sum, "%s\x00%s\n", name, digest)
	}
	return digestPrefix + hex.EncodeToString(sum.Sum(nil)), nil
}

// DigestStrings is the digest of a list of already-canonical strings, used for
// registries that live in code rather than on disk.
func DigestStrings(lines []string) string {
	sum := sha256.New()
	for _, line := range lines {
		fmt.Fprintf(sum, "%s\n", line)
	}
	return digestPrefix + hex.EncodeToString(sum.Sum(nil))
}

// bridges are the reference-validator sources this repository compiles against
// the pinned pilot jar, in the order the oracles record them.
var bridges = []Tool{
	{Name: "pilot-sysml-validator", Source: "scripts/pilot-sysml-validator/ValidateSysML.java"},
	{Name: "pilot-kerml-validator", Source: "scripts/pilot-kerml-validator/ValidateKerML.java"},
}

// Bridges identifies the reference validators by the digest of the source they
// are compiled from and by the pilot release the caller resolved them at.
func Bridges(root, release string) ([]Tool, error) {
	out := make([]Tool, 0, len(bridges))
	for _, tool := range bridges {
		digest, err := DigestFile(filepath.Join(root, filepath.FromSlash(tool.Source)))
		if err != nil {
			return nil, fmt.Errorf("digest %s: %w", tool.Source, err)
		}
		tool.SourceDigest = digest
		tool.Release = release
		out = append(out, tool)
	}
	return out, nil
}

// ErrataDigest identifies the declared overlay: what it corrects is part of what
// the oracles read, so a changed registry is a changed measurement.
func ErrataDigest(entries []errata.Entry) string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, fmt.Sprintf("%s\x00%s\x00%d\x00%s\x00%s",
			entry.ID, entry.Path, entry.Line, entry.AsPublished, entry.Corrected))
	}
	sort.Strings(lines)
	return DigestStrings(lines)
}

// Cause classifies a provenance mismatch by the action it calls for.
type Cause int

const (
	// CausePin: the repository's pin moved and the baseline predates it.
	CausePin Cause = iota
	// CauseOurs: material this repository owns moved.
	CauseOurs
	// CausePinned: the pinned reference material changed underneath the baseline.
	CausePinned
)

func (c Cause) action() string {
	switch c {
	case CausePin:
		return "the repository's pilot pin has moved since this baseline was recorded: re-run the oracle against the new pin"
	case CausePinned:
		return "pinned reference material differs from what the baseline measured, at an unchanged pin: investigate the provisioning before re-recording, because the reference is not supposed to move underneath a pin"
	default:
		return "material this repository owns has changed since this baseline was recorded: adjudicate the movement, then re-record"
	}
}

// Mismatch is one field of a committed provenance record that no longer matches
// the repository.
type Mismatch struct {
	Field    string
	Recorded string
	Current  string
	Cause    Cause
}

// toolField names one field of a recorded tool, as a mismatch reports it.
func toolField(name, field string) string { return "tools[" + name + "]" + field }

// inputField names one field of a recorded input, as a mismatch reports it.
func inputField(name, field string) string { return "inputs[" + name + "]" + field }

// Compare reports how a committed record differs from the repository's current
// one. An input the current record does not state — a corpus absent from this
// checkout — is not compared: the point of this check is to run without them.
func Compare(recorded, current Record) []Mismatch {
	var out []Mismatch
	add := func(field, was, now string, cause Cause) {
		if was != now {
			out = append(out, Mismatch{Field: field, Recorded: was, Current: now, Cause: cause})
		}
	}
	add("pilotTag", recorded.PilotTag, current.PilotTag, CausePin)
	add("pilotCommit", recorded.PilotCommit, current.PilotCommit, CausePin)
	add("pilotArtifact", recorded.PilotArtifact, current.PilotArtifact, CausePin)
	add("errataRegistry", recorded.Errata, current.Errata, CauseOurs)

	recordedTools := make(map[string]Tool, len(recorded.Tools))
	for _, tool := range recorded.Tools {
		recordedTools[tool.Name] = tool
	}
	for _, tool := range current.Tools {
		was, ok := recordedTools[tool.Name]
		if !ok {
			out = append(out, Mismatch{Field: toolField(tool.Name, ""), Recorded: "not recorded", Current: "present", Cause: CauseOurs})
			continue
		}
		add(toolField(tool.Name, ".source"), was.Source, tool.Source, CauseOurs)
		add(toolField(tool.Name, ".sourceDigest"), was.SourceDigest, tool.SourceDigest, CauseOurs)
		add(toolField(tool.Name, ".release"), was.Release, tool.Release, CausePin)
	}

	recordedInputs := make(map[string]Input, len(recorded.Inputs))
	for _, input := range recorded.Inputs {
		recordedInputs[input.Name] = input
	}
	for _, input := range current.Inputs {
		cause := CauseOurs
		if input.Origin == OriginPinned {
			cause = CausePinned
		}
		was, ok := recordedInputs[input.Name]
		if !ok {
			out = append(out, Mismatch{Field: inputField(input.Name, ""), Recorded: "not recorded", Current: input.Dir, Cause: cause})
			continue
		}
		add(inputField(input.Name, ".dir"), was.Dir, input.Dir, cause)
		add(inputField(input.Name, ".origin"), was.Origin, input.Origin, cause)
		add(inputField(input.Name, ".files"), fmt.Sprint(was.Files), fmt.Sprint(input.Files), cause)
		add(inputField(input.Name, ".digest"), was.Digest, input.Digest, cause)
	}
	return out
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Validate reports what a committed record fails to state. A baseline missing
// its provenance is exactly the stale baseline this package exists to catch.
func Validate(recorded Record) error {
	switch {
	case recorded.PilotTag == "" || recorded.PilotCommit == "" || recorded.PilotArtifact == "":
		return fmt.Errorf("states no pilot pin")
	case recorded.Errata == "":
		return fmt.Errorf("states no errata registry digest")
	case len(recorded.Inputs) == 0:
		return fmt.Errorf("states no compared inputs")
	case !datePattern.MatchString(recorded.Recorded):
		return fmt.Errorf("states no ISO recording date (%q)", recorded.Recorded)
	}
	return nil
}

// Explain is the failure a provenance guard prints: which baseline, which
// fields, what each mismatch means and the command that re-records it.
func Explain(baselinePath, refresh string, mismatches []Mismatch) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s no longer describes this repository:\n", baselinePath)
	for _, m := range mismatches {
		fmt.Fprintf(&b, "  provenance.%s: baseline records %q, repository now has %q\n", m.Field, m.Recorded, m.Current)
		fmt.Fprintf(&b, "    %s\n", m.Cause.action())
	}
	fmt.Fprintf(&b, "re-record it with: %s\n", refresh)
	return b.String()
}
