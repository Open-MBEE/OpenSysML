package xpect

import (
	"os"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
)

// committedBaseline is the record docs/project/pilot-xpect.md is generated from,
// and refreshCommand is the only supported way to re-record it.
const (
	committedBaseline = "docs/project/pilot-xpect-baseline.json"
	refreshCommand    = "go run -C tools ./cmd/pilot-xpect -update"
)

// provenance identifies what this oracle adjudicated. It runs no validator: the
// reference is the suites' own declared expectations, so the pin and the suite
// contents are its whole identity.
func provenance(repo string, overlay *errata.Overlay, inputs []baseline.Input) (baseline.Record, error) {
	pin, err := baseline.ReadPin(repo)
	if err != nil {
		return baseline.Record{}, err
	}
	return baseline.Record{
		PilotTag:      pin.Tag,
		PilotCommit:   pin.Commit,
		PilotArtifact: pin.Artifact,
		Errata:        baseline.ErrataDigest(overlay.Entries()),
		Inputs:        inputs,
	}, nil
}

// suiteInputs identifies every provisioned suite by the digest of its .xt files.
// An absent suite is not identified, so the check also runs in a checkout that
// has not provisioned the corpus.
func suiteInputs(repo string) ([]baseline.Input, error) {
	var out []baseline.Input
	for _, s := range defaultSuites {
		dir := filepath.Join(repo, filepath.FromSlash(s.Dir))
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		files, err := collectXT(dir)
		if err != nil {
			return nil, err
		}
		digest, err := baseline.DigestFiles(dir, files)
		if err != nil {
			return nil, err
		}
		out = append(out, baseline.Input{
			Name:   s.Name,
			Dir:    filepath.ToSlash(s.Dir),
			Origin: baseline.OriginPinned,
			Files:  len(files),
			Digest: digest,
		})
	}
	return out, nil
}
