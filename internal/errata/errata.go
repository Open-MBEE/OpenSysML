// Package errata is the registry the oracles read: every declared defect in the
// pilot corpora and the bundled library, and the corrected copy of a corpus root.
package errata

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	overlay "github.com/Open-MBEE/OpenSysML/internal/core/libs/errata"
)

// Entry is one defect in published material; see the overlay package.
type Entry = overlay.Entry

// Overlay is a registry indexed by file, ready to apply; see the overlay package.
type Overlay = overlay.Overlay

// IssuesPath is the page every entry must be documented in.
const IssuesPath = overlay.IssuesPath

// Roots are the repository paths holding OMG-published material.
var Roots = []string{
	"examples/pilot-corpora",
	"build/pilot-xpect-corpus",
	overlay.LibraryRoot,
}

// Registry is the declared errata, in report order: the corpus entries, then
// the bundled library's.
func Registry() []Entry {
	return append([]Entry{{
		ID:      "F82",
		Heading: "`radius = 22/2*25.4 + 110 [mm]` adds a dimensionless value to a length",
		Path:    "examples/pilot-corpora/sysml-examples/Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml",
		Line:    38,
		// The published line keeps its trailing space; the correction keeps it too.
		AsPublished: "            :>> radius = 22/2*25.4 + 110 [mm]; ",
		Corrected:   "            :>> radius = (22/2*25.4 + 110) [mm]; ",
		Citation:    "SysML v2 §9.8.9.1",
		Derivation:  "the unit postfix binds to PrimaryExpression (KerMLExpressions.xtext:308), below AdditiveExpression, so `[mm]` qualifies 110 alone and `+` adds a dimensionless value to a length, which §9.8.9.1 forbids.",
	}, {
		ID:          "F83",
		Heading:     "`1/(2 * Cp) * V^2 + T_static` adds L^6 to Θ",
		Path:        "examples/pilot-corpora/sysml-examples/Analysis Examples/Turbojet Stage Analysis.sysml",
		Line:        25,
		AsPublished: "\t    \treturn : TemperatureValue = 1/(2 * Cp) * V^2 + T_static;",
		Citation:    "SysML v2 §9.8.9.1",
		Derivation:  "V is a VolumeValue and Cp dimensionless, so the first operand has dimension L^6 while T_static has Θ; no reading of the published text shares a dimension, so the defect is documented without a correction.",
	}, {
		ID:          "F84",
		Heading:     "`return a : AccelerationValue = tp * dt * tp` returns L^4·M^2·T^-5",
		Path:        "examples/pilot-corpora/sysml-examples/Analysis Examples/Dynamics.sysml",
		Line:        13,
		AsPublished: "\t\treturn a : AccelerationValue = tp * dt * tp;",
		Citation:    "KerML 7.4.9",
		Derivation:  "tp is a PowerValue (L^2·M·T^-3) and dt a TimeValue (T), so the expression the return feature takes its value from has dimension L^4·M^2·T^-5 while AccelerationValue is measured in L·T^-2; no repair follows from the parameters the calculation declares, so the defect is documented without a correction.",
	}}, overlay.LibraryEntries()...)
}

// Load returns the declared registry as an overlay.
func Load() (*Overlay, error) { return overlay.New(Roots, Registry()) }

// SortedPaths returns the keys of an Under result in order.
func SortedPaths(applied map[string][]Entry) []string { return overlay.SortedPaths(applied) }

// Count returns how many entries an Under result holds.
func Count(applied map[string][]Entry) int { return overlay.Count(applied) }

// Materialize copies the corpus root at repo/dir into dst, verifies every entry under it and
// applies the corrections, leaving the published tree untouched. It returns the applied entries
// in order. The copy is all or nothing: on any error dst is absent, never a partly corrected tree.
func Materialize(o *Overlay, repo, dir, dst string) ([]Entry, error) {
	if len(o.Under(dir)) == 0 {
		return nil, fmt.Errorf("no correction lies under %s", dir)
	}
	entries := o.EntriesUnder(dir)
	if err := os.RemoveAll(dst); err != nil {
		return nil, err
	}
	parent := filepath.Dir(dst)
	_, statErr := os.Stat(parent)
	createdParent := errors.Is(statErr, fs.ErrNotExist)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return nil, err
	}
	fail := func(tmp string, err error) ([]Entry, error) {
		if tmp != "" {
			_ = os.RemoveAll(tmp)
		}
		if createdParent {
			_ = os.Remove(parent)
		}
		return nil, err
	}
	tmp, err := os.MkdirTemp(parent, filepath.Base(dst)+".*")
	if err != nil {
		return fail("", err)
	}
	out, err := materializeInto(repo, dir, tmp, entries)
	if err == nil {
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		return fail(tmp, err)
	}
	return out, nil
}

// materializeInto fills the empty directory dst with the corrected copy of repo/dir.
func materializeInto(repo, dir, dst string, entries map[string][]Entry) ([]Entry, error) {
	if err := os.CopyFS(dst, os.DirFS(filepath.Join(repo, filepath.FromSlash(dir)))); err != nil {
		return nil, fmt.Errorf("copy %s: %w", dir, err)
	}
	var out []Entry
	for _, rel := range overlay.SortedPaths(entries) {
		path := filepath.Join(dst, filepath.FromSlash(rel))
		content, err := os.ReadFile(path) // #nosec G304 -- the path is inside the copy this function just made
		if err != nil {
			var pathErr *fs.PathError
			if errors.As(err, &pathErr) {
				err = pathErr.Err
			}
			return nil, fmt.Errorf("%s/%s: %w", dir, rel, err)
		}
		corrected, err := overlay.ApplyAll(entries[rel], content)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, corrected, 0o600); err != nil {
			return nil, err
		}
		for _, entry := range entries[rel] {
			if entry.Corrects() {
				out = append(out, entry)
			}
		}
	}
	return out, nil
}
