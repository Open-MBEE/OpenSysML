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

// provenance identifies the run's inputs: checkout's pin, bridge and corpora, the
// writer linked into this executable, digested in built, its build checkout, and
// the libraries (an absolute directory inside checkout) the bridge reads beside
// the standard library.
func provenance(checkout, built, libraries string) (baseline.Record, error) {
	pin, err := baseline.ReadPin(checkout)
	if err != nil {
		return baseline.Record{}, err
	}
	bridgeDigest, err := baseline.DigestFile(filepath.Join(checkout, filepath.FromSlash(bridgeSource)))
	if err != nil {
		return baseline.Record{}, fmt.Errorf("digest %s: %w", bridgeSource, err)
	}
	writerDigest, err := baseline.DigestPackage(filepath.Join(built, filepath.FromSlash(writerSource)))
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
		files, err := diff.CollectFiles(checkout, root)
		if err != nil {
			return baseline.Record{}, err
		}
		if len(files) == 0 {
			continue
		}
		digest, err := baseline.DigestFiles(filepath.Join(checkout, filepath.FromSlash(root.Dir)), files)
		if err != nil {
			return baseline.Record{}, fmt.Errorf("digest %s: %w", root.Dir, err)
		}
		origin := baseline.OriginOurs
		if root.Pinned {
			origin = baseline.OriginPinned
		}
		record.Inputs = append(record.Inputs, baseline.Input{Name: root.Name, Dir: root.Dir, Origin: origin, Files: len(files), Digest: digest})
	}
	libraryRoot := diff.Root{Name: diff.LibrariesInput, Dir: diff.RelativeTo(checkout, libraries)}
	files, err := diff.CollectFiles(checkout, libraryRoot)
	if err != nil {
		return baseline.Record{}, err
	}
	if len(files) == 0 {
		return baseline.Record{}, fmt.Errorf("no .sysml or .kerml files under the library directory %s", libraries)
	}
	libraryDigest, err := baseline.DigestFiles(libraries, files)
	if err != nil {
		return baseline.Record{}, fmt.Errorf("digest %s: %w", libraries, err)
	}
	record.Inputs = append(record.Inputs, baseline.Input{Name: libraryRoot.Name, Dir: libraryRoot.Dir, Origin: baseline.OriginOurs, Files: len(files), Digest: libraryDigest})
	return record, nil
}
