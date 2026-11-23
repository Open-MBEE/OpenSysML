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
	// Documents digests each document the run wrote, by file; nil in a record written before it existed.
	Documents map[string]string `json:"documents,omitempty"`
	pilotRun
}

// documentDigests digests the documents under out for files; a file the pilot wrote nothing for has none.
func documentDigests(out string, files []string) (map[string]string, error) {
	digests := make(map[string]string, len(files))
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)+".json")) // #nosec G304 -- under the record directory
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(content)
		digests[rel] = hex.EncodeToString(sum[:])
	}
	return digests, nil
}

// intact reports why the documents under out are not the ones the record digested, or "".
func (r pilotRecord) intact(out string, files []string) (string, error) {
	if r.Documents == nil {
		return "", nil
	}
	now, err := documentDigests(out, files)
	if err != nil {
		return "", err
	}
	for _, rel := range files {
		switch was, recorded := r.Documents[rel]; {
		case recorded && now[rel] == "":
			return "the recorded document of " + rel + " is missing", nil
		case was != now[rel]:
			return "the document of " + rel + " is not the one recorded", nil
		}
	}
	return "", nil
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

// pilotLaunch is how the pilot is run: its launcher, the JVM it runs under, its time limit and its log.
type pilotLaunch struct {
	launcher, java string
	timeout        time.Duration
	log            io.Writer
}

// pilotRoot is one corpus root the pilot exports: its name, directory, libraries and files.
type pilotRoot struct {
	root, dir        string
	libraries, files []string
}

// recordedPilot is root's export from the record under store when its key is the run's, else a
// fresh export recorded there; without a launcher only the record can answer.
func recordedPilot(store string, pilot pilotLaunch, record baseline.Record, corpus pilotRoot) (*pilotExport, error) {
	launcher, java, timeout, log := pilot.launcher, pilot.java, pilot.timeout, pilot.log
	root, dir, libraries, files := corpus.root, corpus.dir, corpus.libraries, corpus.files
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
	damage := ""
	if held.Key == key {
		if damage, err = held.intact(out, files); err != nil {
			return nil, err
		}
		if damage == "" {
			fmt.Fprintf(log, "%s: the pilot's export is the one recorded under %s\n", root, out)
			return readPilot(dir, out, files, held.pilotRun)
		}
		fmt.Fprintf(log, "%s: the export recorded under %s is damaged (%s)\n", root, out, damage)
	}
	if launcher == "" && damage != "" {
		return nil, fmt.Errorf("the export of %s recorded under %s is damaged (%s), and the pilot JSON exporter is absent: provision it and re-run to record one", root, store, damage)
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
	documents, err := documentDigests(out, files)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(pilotRecord{Key: key, Documents: documents, pilotRun: run})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(marker, encoded, 0o600); err != nil {
		return nil, err
	}
	fmt.Fprintf(log, "%s: recorded the pilot's export under %s\n", root, out)
	return export, nil
}
