package apijson

import (
	"fmt"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
	"github.com/Open-MBEE/OpenSysML/tools/referee/diff"
)

const (
	committedBaseline = "docs/project/api-json-differential-baseline.json"
	refreshCommand    = "go run -C tools ./cmd/pilot-apijson-diff -update"
	bridgeSource      = "scripts/pilot-json-exporter/ExportJSON.java"
	bridgeName        = "pilot-json-exporter"
	writerSource      = "internal/translate/export"
	writerName        = "api-json-writer"
)

// manualExamples is the one root the pilot differential does not compare: the
// handbook's worked examples, whose element forms the manual shows.
var manualExamples = diff.Root{Name: "manual-examples", Dir: "docs/manual/examples"}

// roots are the differential's corpus roots plus the manual's examples.
func roots() []diff.Root { return append(diff.Roots(), manualExamples) }

// provenance identifies the run's inputs: the pilot pin, the sources the pilot
// exporter and our writer are compiled from, and every non-empty root's digest.
func provenance(repo string) (baseline.Record, error) {
	pin, err := baseline.ReadPin(repo)
	if err != nil {
		return baseline.Record{}, err
	}
	bridgeDigest, err := baseline.DigestFile(filepath.Join(repo, filepath.FromSlash(bridgeSource)))
	if err != nil {
		return baseline.Record{}, fmt.Errorf("digest %s: %w", bridgeSource, err)
	}
	writerDigest, err := baseline.DigestPackage(filepath.Join(repo, filepath.FromSlash(writerSource)))
	if err != nil {
		return baseline.Record{}, fmt.Errorf("digest %s: %w", writerSource, err)
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
		Tools: []baseline.Tool{
			{Name: bridgeName, Source: bridgeSource, SourceDigest: bridgeDigest, Release: pin.Release()},
			{Name: writerName, Source: writerSource, SourceDigest: writerDigest},
		},
	}
	for _, root := range roots() {
		files, err := diff.CollectFiles(repo, root)
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
		origin := baseline.OriginOurs
		if root.Pinned {
			origin = baseline.OriginPinned
		}
		record.Inputs = append(record.Inputs, baseline.Input{Name: root.Name, Dir: root.Dir, Origin: origin, Files: len(files), Digest: digest})
	}
	return record, nil
}
