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

// provenance identifies everything this oracle compares: the pin, the reference
// bridges, the declared errata and each corpus root's contents. A release of ""
// resolves it from the pin, which is what a checkout without the validators has.
func provenance(repo, release string) (baseline.Record, error) {
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
	return record, nil
}

func (r corpusRoot) origin() string {
	if r.Pinned {
		return baseline.OriginPinned
	}
	return baseline.OriginOurs
}
