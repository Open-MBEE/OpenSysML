package reject

import (
	"fmt"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
)

// committedBaseline is the record docs/project/pilot-rejection.md is generated
// from, and refreshCommand is the only supported way to re-record it.
const (
	committedBaseline = "docs/project/pilot-rejection-baseline.json"
	refreshCommand    = "go run -C tools ./cmd/pilot-reject -update"
)

// provenance identifies the pin, the reference bridges and the negative corpus
// this run adjudicated. The corpus is ours, so its movement is a movement to
// adjudicate rather than a provisioning defect.
func provenance(repo, release, corpusDir string, overlay *errata.Overlay, files []string) (baseline.Record, error) {
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
	digest, err := baseline.DigestFiles(filepath.Join(repo, filepath.FromSlash(corpusDir)), files)
	if err != nil {
		return baseline.Record{}, fmt.Errorf("digest %s: %w", corpusDir, err)
	}
	return baseline.Record{
		PilotTag:      pin.Tag,
		PilotCommit:   pin.Commit,
		PilotArtifact: pin.Artifact,
		Errata:        baseline.ErrataDigest(overlay.Entries()),
		Tools:         tools,
		Inputs: []baseline.Input{{
			Name:   "negative-corpus",
			Dir:    corpusDir,
			Origin: baseline.OriginOurs,
			Files:  len(files),
			Digest: digest,
		}},
	}, nil
}
