package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesWriteAndAnnounce(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "out", "nested")
	files, err := Open(dir, "probe")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	encoded, err := files.JSON(map[string]int{"agree": 2})
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if want := "{\n  \"agree\": 2\n}\n"; string(encoded) != want {
		t.Fatalf("JSON returned %q, want %q", encoded, want)
	}
	onDisk, err := os.ReadFile(files.Path("json"))
	if err != nil {
		t.Fatalf("read probe.json: %v", err)
	}
	if string(onDisk) != string(encoded) {
		t.Fatalf("probe.json holds %q, JSON returned %q", onDisk, encoded)
	}
	if err := files.Text("2 agree\n"); err != nil {
		t.Fatalf("Text: %v", err)
	}
	if err := files.Write("xml", []byte("<testsuites/>\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := []string{filepath.Join(dir, "probe.json"), filepath.Join(dir, "probe.txt"), filepath.Join(dir, "probe.xml")}
	if got := files.Written(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("Written() = %v, want %v", got, want)
	}

	var out strings.Builder
	files.Announce(&out, "2 case(s)")
	wantOut := "wrote " + want[0] + ", " + want[1] + " and " + want[2] + "\n2 case(s)\n"
	if out.String() != wantOut {
		t.Fatalf("Announce wrote %q, want %q", out.String(), wantOut)
	}
}

func TestAnnounceWithoutFiles(t *testing.T) {
	files, err := Open(t.TempDir(), "probe")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	var out strings.Builder
	files.Announce(&out, "nothing to report")
	if out.String() != "nothing to report\n" {
		t.Fatalf("Announce wrote %q", out.String())
	}
}
