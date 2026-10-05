package diff

import (
	"fmt"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
)

// committedBaseline is the record docs/project/pilot-differential.md is
// generated from, and refreshCommand is the only supported way to re-record it.
const (
	committedBaseline = "docs/project/pilot-differential-baseline.json"
	refreshCommand    = "go run -C tools ./cmd/pilot-diff -update"
)

// librariesInput names the provenance input that records the libraries the
// reference validators resolved against; a changed library is a changed measurement.
const librariesInput = "opensysml-libraries"

// provenance identifies everything this oracle compares: the pin, the reference
// bridges, the declared errata, each corpus root's contents and the libraries
// (a repository-relative directory) handed to the reference. A release of ""
// resolves it from the pin, which is what a checkout without the validators has.
func provenance(repo, release, libraries string) (baseline.Record, error) {
	pin, err := baseline.ReadPin(repo)
	if err != nil {
		return baseline.Record{}, err
	}
	if release == "" {
		release = pin.Release()
	}
	tools, err := baseline.Bridges(repo, release)
	if err != nil {
		return baseline.Record{}, err
	}
	overlay, err := errata.Load()
	if err != nil {
		return baseline.Record{}, err
	}
	record := baseline.Record{
		PilotTag:      pin.Tag,
		PilotCommit:   pin.Commit,
		PilotArtifact: pin.Artifact,
		Errata:        baseline.ErrataDigest(overlay.Entries()),
		Tools:         tools,
	}
	for _, root := range defaultRoots {
		files, err := collectFiles(repo, root)
		if err != nil {
			return baseline.Record{}, err
		}
		if len(files) == 0 {
			continue
		}
		digest, err := baseline.DigestFiles(filepath.Join(repo, filepath.FromSlash(root.Dir)), files)
		if err != nil {
			return baseline.Record{}, fmt.Errorf("digest %s: %w", root.Dir, err)
		}
		record.Inputs = append(record.Inputs, baseline.Input{
			Name:   root.Name,
			Dir:    root.Dir,
			Origin: root.origin(),
			Files:  len(files),
			Digest: digest,
		})
	}

	libraryRoot := corpusRoot{Name: librariesInput, Dir: libraries}
	files, err := collectFiles(repo, libraryRoot)
	if err != nil {
		return baseline.Record{}, err
	}
	if len(files) == 0 {
		return baseline.Record{}, fmt.Errorf("no .sysml or .kerml files under the library directory %s", libraries)
	}
	digest, err := baseline.DigestFiles(filepath.Join(repo, filepath.FromSlash(libraries)), files)
	if err != nil {
		return baseline.Record{}, fmt.Errorf("digest %s: %w", libraries, err)
	}
	record.Inputs = append(record.Inputs, baseline.Input{
		Name:   libraryRoot.Name,
		Dir:    libraries,
		Origin: libraryRoot.origin(),
		Files:  len(files),
		Digest: digest,
	})
	return record, nil
}

func (r corpusRoot) origin() string {
	if r.Pinned {
		return baseline.OriginPinned
	}
	return baseline.OriginOurs
}
