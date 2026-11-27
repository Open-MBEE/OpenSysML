package apijson

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// recordStub exports the stub checkout's one root through the record under store.
func recordStub(t *testing.T, opts options, store, launcher string) (*pilotExport, error) {
	t.Helper()
	record, err := provenance(opts.repo, opts.libraries)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(opts.repo, "tests", "testdata")
	return recordedPilot(store, pilotLaunch{launcher, "21", opts.timeout, io.Discard}, record, pilotRoot{"testdata", dir, []string{opts.libraries}, stubFiles})
}

// The recorded export answers a later run without the pilot, and answers what the pilot did.
func TestPilotRecordReplaysTheExportWithoutThePilot(t *testing.T) {
	opts, store := stubRepo(t), t.TempDir()
	fresh, err := recordStub(t, opts, store, stubBridge(t, "sub/b.sysml: unresolved reference to Nowhere", 1, "a.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := recordStub(t, opts, store, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fresh, replayed) {
		t.Fatalf("the replayed export differs from the pilot's:\n%+v\n%+v", fresh, replayed)
	}
	if !reflect.DeepEqual(replayed.Failed, []string{"sub/b.sysml"}) {
		t.Fatalf("failed = %v, want only sub/b.sysml", replayed.Failed)
	}
}

// Every input of the pilot's export is in the record's key: a moved one is never answered from it.
func TestPilotRecordIsNotReusedOnceAnInputMoves(t *testing.T) {
	moves := map[string]func(t *testing.T, opts options){
		"a root file": func(t *testing.T, opts options) {
			rewrite(t, filepath.Join(opts.repo, "tests", "testdata", "a.sysml"), "package A { part def Moved; }\n")
		},
		"a library": func(t *testing.T, opts options) {
			rewrite(t, filepath.Join(opts.libraries, "L.sysml"), "library package L { part def Moved; }\n")
		},
		"the bridge": func(t *testing.T, opts options) {
			rewrite(t, filepath.Join(opts.repo, filepath.FromSlash(bridgeSource)), "class ExportJSON { int moved; }\n")
		},
		"the pin": func(t *testing.T, opts options) {
			path := filepath.Join(opts.repo, "scripts", "pilot-pin.sh")
			pin, err := os.ReadFile(path) // #nosec G304 -- the stub checkout's pin
			if err != nil {
				t.Fatal(err)
			}
			rewrite(t, path, strings.Replace(string(pin), "PILOT_ARTIFACT_VERSION:-", "PILOT_ARTIFACT_VERSION:-9", 1))
		},
	}
	for name, move := range moves {
		t.Run(name, func(t *testing.T) {
			opts, store := stubRepo(t), t.TempDir()
			bridge := stubBridge(t, "", 0, stubFiles...)
			if _, err := recordStub(t, opts, store, bridge); err != nil {
				t.Fatal(err)
			}
			move(t, opts)
			if _, err := recordStub(t, opts, store, ""); err == nil || !strings.Contains(err.Error(), "holds no export of testdata") {
				t.Fatalf("want the stale record refused without the pilot, got %v", err)
			}
			if _, err := recordStub(t, opts, store, bridge); err != nil {
				t.Fatalf("the pilot did not re-record: %v", err)
			}
			if _, err := recordStub(t, opts, store, ""); err != nil {
				t.Fatalf("the re-recorded export was not reused: %v", err)
			}
		})
	}
}

// The JVM the pilot's side is declared under is in the key too.
func TestPilotRecordIsKeyedByTheDeclaredJVM(t *testing.T) {
	opts := stubRepo(t)
	record, err := provenance(opts.repo, opts.libraries)
	if err != nil {
		t.Fatal(err)
	}
	one, err := recordKey(record, "21", "testdata")
	if err != nil {
		t.Fatal(err)
	}
	other, err := recordKey(record, "25", "testdata")
	if err != nil {
		t.Fatal(err)
	}
	if one == other {
		t.Fatal("the key is the same under Java 21 and Java 25")
	}
}

// An export interrupted before its marker was written is not a record.
func TestPilotRecordWithoutItsMarkerIsNotReused(t *testing.T) {
	opts, store := stubRepo(t), t.TempDir()
	if _, err := recordStub(t, opts, store, stubBridge(t, "", 0, stubFiles...)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(store, "testdata", recordFile)); err != nil {
		t.Fatal(err)
	}
	if _, err := recordStub(t, opts, store, ""); err == nil {
		t.Fatal("documents without a marker were read as a record")
	}
}

// A run the bridge never started in records nothing.
func TestPilotRecordKeepsNoFailedRun(t *testing.T) {
	opts, store := stubRepo(t), t.TempDir()
	if _, err := recordStub(t, opts, store, stubBridge(t, "", 1)); err == nil {
		t.Fatal("a run that wrote nothing was accepted")
	}
	if _, err := os.Stat(filepath.Join(store, "testdata", recordFile)); !os.IsNotExist(err) {
		t.Fatalf("want no marker, got %v", err)
	}
}

func rewrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A run over the recorded export pins every committed count: one moved in a fragment or in the
// index fails -check though the pilot never runs.
func TestCheckOverTheRecordedExportFailsOnAMovedCount(t *testing.T) {
	opts := stubRepo(t)
	opts.pilotRecord = t.TempDir()
	opts.launcher = stubBridge(t, "", 0, stubFiles...)
	opts.update = true
	if err := run(opts); err != nil {
		t.Fatal(err)
	}
	opts.update, opts.check, opts.launcher = false, true, ""
	if err := run(opts); err != nil {
		t.Fatalf("the recorded export does not reproduce the baseline it recorded: %v", err)
	}
	committed := committedDir(opts.repo)
	for _, rel := range []string{"index.json", "testdata/a.sysml.json"} {
		t.Run(rel, func(t *testing.T) {
			path := filepath.Join(committed, filepath.FromSlash(rel))
			content, err := os.ReadFile(path) // #nosec G304 -- the stub checkout's baseline
			if err != nil {
				t.Fatal(err)
			}
			moved := strings.Replace(string(content), "\"oursOnly\": ", "\"oursOnly\": 1", 1)
			if moved == string(content) {
				t.Fatalf("%s holds no oursOnly count to move", rel)
			}
			rewrite(t, path, moved)
			defer rewrite(t, path, string(content))
			if err := run(opts); err == nil {
				t.Fatalf("a moved count in %s reproduces", rel)
			}
		})
	}
}

// A recorded document that is gone or altered is not replayed: the pilot re-exports it, and
// without the pilot the run fails instead of scoring the file as one the pilot failed on.
func TestPilotRecordRefusesADamagedExport(t *testing.T) {
	for name, damage := range map[string]func(string) error{
		"missing": os.Remove,
		"altered": func(path string) error { return os.WriteFile(path, []byte("[]"), 0o600) },
	} {
		t.Run(name, func(t *testing.T) {
			opts, store := stubRepo(t), t.TempDir()
			bridge := stubBridge(t, "", 0, "a.sysml", "sub/b.sysml")
			fresh, err := recordStub(t, opts, store, bridge)
			if err != nil {
				t.Fatal(err)
			}
			if err := damage(filepath.Join(store, "testdata", "a.sysml.json")); err != nil {
				t.Fatal(err)
			}
			if _, err := recordStub(t, opts, store, ""); err == nil || !strings.Contains(err.Error(), "damaged") {
				t.Fatalf("a damaged record without the pilot must fail naming the damage, got %v", err)
			}
			again, err := recordStub(t, opts, store, bridge)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(fresh, again) {
				t.Fatalf("the re-export differs from the first:\n%+v\n%+v", fresh, again)
			}
		})
	}
}
