package derived

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// recordStub records the stub checkout's census through a bridge that exports every file.
func recordStub(t *testing.T) options {
	t.Helper()
	opts := stubRepo(t)
	opts.launcher, opts.update = stubBridge(t, "", 0, stubFiles...), true
	if err := os.MkdirAll(filepath.Dir(committedPath(opts)), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := run(opts); err != nil {
		t.Fatal(err)
	}
	opts.update = false
	return opts
}

// moveCell adds one to a field of the records' first line and returns the line's rule and root.
func moveCell(t *testing.T, path string, field int) string {
	t.Helper()
	content, err := os.ReadFile(path) // #nosec G304 -- the stub checkout's records
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		count, err := strconv.Atoi(fields[field])
		if err != nil {
			t.Fatal(err)
		}
		fields[field] = strconv.Itoa(count + 1)
		lines[i] = strings.Join(fields, "\t")
		if err := writeFile(path, strings.Join(lines, "\n")); err != nil {
			t.Fatal(err)
		}
		return fields[0] + "\t" + fields[1]
	}
	t.Fatal("the records hold no line")
	return ""
}

// The check that reuses the recorded pilot column never runs the exporter and reproduces the record.
func TestRecordedPilotCheckReproducesWithoutTheExporter(t *testing.T) {
	opts := recordStub(t)
	opts.launcher, opts.check, opts.recordedPilot = stubBridge(t, "Error: the exporter ran", 78), true, true
	if err := run(opts); err != nil {
		t.Fatalf("the recorded pilot column does not reproduce: %v", err)
	}
}

// A count moved in any column, ours or the pilot's, fails the check that reuses the pilot column.
func TestRecordedPilotCheckFailsOnAMovementInAnyColumn(t *testing.T) {
	for field := 2; field < recordFields; field++ {
		t.Run(strings.Fields(strings.Split(recordsHeader, "\n")[1])[field], func(t *testing.T) {
			opts := recordStub(t)
			moved := moveCell(t, recordsPath(committedPath(opts)), field)
			opts.check, opts.recordedPilot = true, true
			err := run(opts)
			if err == nil {
				t.Fatalf("a movement in field %d of %q reproduces", field, moved)
			}
			if field < pilotField && !strings.Contains(err.Error(), moved) {
				t.Errorf("the failure does not name the moved record %q:\n%v", moved, err)
			}
			if field >= pilotField && !strings.Contains(err.Error(), "pilotRecords") {
				t.Errorf("the failure does not name the pilot records' digest:\n%v", err)
			}
		})
	}
}

// A root's pilot totals are reused with the column, so the digest pins them too.
func TestRecordedPilotCheckFailsOnAMovedPilotTotal(t *testing.T) {
	for _, total := range []string{"pilotElements", "pilotImplied", "pilotDuplicates", "pilotFailed"} {
		t.Run(total, func(t *testing.T) {
			opts := recordStub(t)
			path := committedPath(opts)
			content, err := os.ReadFile(path) // #nosec G304 -- the stub checkout's baseline
			if err != nil {
				t.Fatal(err)
			}
			moved := regexp.MustCompile(`"`+total+`": \d+`).ReplaceAllString(string(content), `"`+total+`": 987654`)
			if moved == string(content) {
				t.Fatalf("the baseline records no %s", total)
			}
			if err := writeFile(path, moved); err != nil {
				t.Fatal(err)
			}
			opts.check, opts.recordedPilot = true, true
			if err := run(opts); err == nil || !strings.Contains(err.Error(), "pilotRecords") {
				t.Errorf("a moved %s reproduces: %v", total, err)
			}
		})
	}
}

// A pilot cell a merge moved fails the check; re-recording our side must not restate the digest over it.
func TestRecordedPilotUpdateRefusesAMovedPilotColumn(t *testing.T) {
	for field := pilotField; field < recordFields; field++ {
		opts := recordStub(t)
		records := recordsPath(committedPath(opts))
		moveCell(t, records, field)
		moved, err := os.ReadFile(records) // #nosec G304 -- the stub checkout's records
		if err != nil {
			t.Fatal(err)
		}
		opts.update, opts.recordedPilot = true, true
		if err := run(opts); err == nil || !strings.Contains(err.Error(), "pilotRecords") {
			t.Fatalf("field %d: a moved pilot cell was re-recorded as the pilot's: %v", field, err)
		}
		if kept, err := os.ReadFile(records); err != nil || string(kept) != string(moved) { // #nosec G304 -- the stub checkout's records
			t.Errorf("field %d: the refused update rewrote the records (%v)", field, err)
		}
		opts.update, opts.check = false, true
		if err := run(opts); err == nil {
			t.Errorf("field %d: the moved pilot cell reproduces after the refused update", field)
		}
	}
}

// A column exported at another pin is not this checkout's pilot: re-recording around it is refused,
// and one that is reused keeps the date of its export.
func TestRecordedPilotUpdateRefusesAColumnFromOtherInputs(t *testing.T) {
	opts := recordStub(t)
	path := committedPath(opts)
	content, err := os.ReadFile(path) // #nosec G304 -- the stub checkout's baseline
	if err != nil {
		t.Fatal(err)
	}
	dated := regexp.MustCompile(`"recorded": "[^"]*"`).ReplaceAllString(string(content), `"recorded": "2001-02-03"`)
	if err := writeFile(path, dated); err != nil {
		t.Fatal(err)
	}
	opts.update, opts.recordedPilot = true, true
	if err := run(opts); err != nil {
		t.Fatalf("a column from this checkout's inputs is refused: %v", err)
	}
	if kept, err := os.ReadFile(path); err != nil || !strings.Contains(string(kept), `"recorded": "2001-02-03"`) { // #nosec G304 -- the stub checkout's baseline
		t.Errorf("the reused column lost the date of its export (%v)", err)
	}
	stale := regexp.MustCompile(`"pilotArtifact": "[^"]*"`).ReplaceAllString(dated, `"pilotArtifact": "0.0.0"`)
	if err := writeFile(path, stale); err != nil {
		t.Fatal(err)
	}
	if err := run(opts); err == nil || !strings.Contains(err.Error(), "pilotArtifact") {
		t.Errorf("a column exported at another pin was re-recorded as this one's: %v", err)
	}
}

// A rule served since the pilot column was recorded has no pilot count to reuse.
func TestRecordedPilotRefusesARuleThePilotColumnDoesNotRecord(t *testing.T) {
	opts := recordStub(t)
	path := recordsPath(committedPath(opts))
	content, err := os.ReadFile(path) // #nosec G304 -- the stub checkout's records
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(content), "\n")
	gone := strings.SplitN(lines[2], "\t", 2)[0]
	if err := writeFile(path, strings.Join(append(lines[:2:2], lines[3:]...), "\n")); err != nil {
		t.Fatal(err)
	}
	// The column is as its export left it but for the rule, so the baseline states its digest.
	column, err := readCommitted(committedPath(opts))
	if err != nil {
		t.Fatal(err)
	}
	head, err := os.ReadFile(committedPath(opts)) // #nosec G304 -- the stub checkout's baseline
	if err != nil {
		t.Fatal(err)
	}
	stated := regexp.MustCompile(`"pilotRecords": "[^"]*"`).ReplaceAllString(string(head), `"pilotRecords": "`+pilotRecordsDigest(column)+`"`)
	if err := writeFile(committedPath(opts), stated); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"check", "update"} {
		opts.check, opts.update, opts.recordedPilot = mode == "check", mode == "update", true
		if err := run(opts); err == nil || !strings.Contains(err.Error(), "records no pilot column for "+gone) {
			t.Errorf("-%s: want the unrecorded rule %s refused, got %v", mode, gone, err)
		}
	}
}

func TestMainRefusesRecordedPilotWithRequireExporter(t *testing.T) {
	var stderr strings.Builder
	if code := Main([]string{"-check", "-require-exporter", "-recorded-pilot"}, &stderr, &stderr); code != 2 || !strings.Contains(stderr.String(), "exclusive") {
		t.Fatalf("code %d: %s", code, stderr.String())
	}
}
