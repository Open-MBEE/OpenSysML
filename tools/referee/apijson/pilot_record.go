package apijson

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/referee/diff"
)

// recordFile marks a root's recorded export complete and names what it was exported from.
const recordFile = "pilot-record.json"

// pilotRecord is one root's recorded export: the run beside the key of its inputs.
type pilotRecord struct {
	Key string `json:"key"`
	pilotRun
}

// recordKey digests everything the pilot's export of root depends on: the pin, the JVM, the
// bridge, the errata, the root's files and the libraries. Nothing of our writer is in it.
func recordKey(record baseline.Record, java, root string) (string, error) {
	reference := record
	reference.PilotJava = java
	reference.Recorded = ""
	reference.Inputs = nil
	for _, input := range record.Inputs {
		if input.Name == root || input.Name == diff.LibrariesInput {
			reference.Inputs = append(reference.Inputs, input)
		}
	}
	if len(reference.Inputs) != 2 {
		return "", fmt.Errorf("the provenance names no input for root %s beside the libraries", root)
	}
	encoded, err := json.Marshal(reference)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// recordedPilot is root's export from the record under store when its key is the run's, else a
// fresh export recorded there; without a launcher only the record can answer.
func recordedPilot(store, launcher, java string, record baseline.Record, root, dir string, libraries, files []string, timeout time.Duration, log io.Writer) (*pilotExport, error) {
	key, err := recordKey(record, java, root)
	if err != nil {
		return nil, err
	}
	out := filepath.Join(store, root)
	marker := filepath.Join(out, recordFile)
	var held pilotRecord
	content, err := os.ReadFile(marker) // #nosec G304 -- the record directory is the caller's
	switch {
	case err == nil:
		if err := json.Unmarshal(content, &held); err != nil {
			return nil, fmt.Errorf("parse %s: %w", marker, err)
		}
	case !os.IsNotExist(err):
		return nil, err
	}
	if held.Key == key {
		fmt.Fprintf(log, "%s: the pilot's export is the one recorded under %s\n", root, out)
		return readPilot(dir, out, files, held.pilotRun)
	}
	if launcher == "" {
		return nil, fmt.Errorf("%s holds no export of %s for this pin, bridge and corpus, and the pilot JSON exporter is absent: provision it and re-run to record one", store, root)
	}
	if err := os.RemoveAll(out); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(out, 0o750); err != nil {
		return nil, err
	}
	run, err := launchPilot(launcher, dir, out, libraries, files, timeout)
	if err != nil {
		return nil, err
	}
	export, err := readPilot(dir, out, files, run)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(pilotRecord{Key: key, pilotRun: run})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(marker, encoded, 0o600); err != nil {
		return nil, err
	}
	fmt.Fprintf(log, "%s: recorded the pilot's export under %s\n", root, out)
	return export, nil
}
