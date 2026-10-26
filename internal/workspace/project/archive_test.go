package project

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zipOf writes a ZIP holding the members in order, names kept as given so a
// test can hand the reader a path it must refuse.
func zipOf(t *testing.T, members ...[2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, m := range members {
		f, err := w.CreateHeader(&zip.FileHeader{Name: m[0], Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(m[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const (
	probeProject = `{"name": "Probe", "version": "1.0.0", "usage": [{"resource": "pkg:sysand/acme/gears"}]}`
	probeModel   = "package Probe { part def P; }\n"
	// probeMeta records the SHA256 of probeModel.
	probeMeta = `{"index": {"Probe": "Probe.sysml"}, "created": "2026-01-01T00:00:00Z",
	"checksum": {"Probe.sysml": {"value": "4044ca78bfbbcdd72b63cd877ad948c20b758dd30cbb4a0bf8c9e0af30c98bda", "algorithm": "SHA256"}}}`
)

func probeArchive(t *testing.T) []byte {
	t.Helper()
	return zipOf(t, [2]string{"Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", probeMeta}, [2]string{"docs/README.md", "notes\n"})
}

func TestParseArchiveReadsAValidArchive(t *testing.T) {
	a, err := ParseArchive("lib/probe.kpar", probeArchive(t))
	if err != nil {
		t.Fatal(err)
	}
	if a.Project.Name != "Probe" || a.Project.Version != "1.0.0" || len(a.Project.Usage) != 1 || a.Project.Usage[0].Resource != "pkg:sysand/acme/gears" {
		t.Errorf("project = %+v", a.Project)
	}
	if a.Meta.Created != "2026-01-01T00:00:00Z" || a.Meta.Index["Probe"] != "Probe.sysml" {
		t.Errorf("meta = %+v", a.Meta)
	}
	if got, want := a.Names(), []string{".meta.json", ".project.json", "Probe.sysml", "docs/README.md"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Names() = %q, want %q", got, want)
	}
	if got := a.Files(); len(got) != 1 || got[0] != "lib/probe.kpar/Probe.sysml" {
		t.Errorf("Files() = %q, want the model member under the archive path", got)
	}
	if content, err := a.Read("Probe.sysml"); err != nil || string(content) != probeModel {
		t.Errorf("Read(Probe.sysml) = %q, %v", content, err)
	}
	if _, err := a.Read("missing.sysml"); err == nil {
		t.Error("Read of a missing member succeeded")
	}
	if len(a.Notes) != 0 {
		t.Errorf("notes = %q, want none for a SHA256 checksum", a.Notes)
	}
}

func TestParseArchiveRejectsMalformedArchives(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"not a zip", []byte("package P;"), "not a ZIP archive"},
		{"empty", nil, "not a ZIP archive"},
		{"no project file", zipOf(t, [2]string{"Probe.sysml", probeModel}, [2]string{".meta.json", probeMeta}), "no .project.json at the top level"},
		{"no meta file", zipOf(t, [2]string{"Probe.sysml", probeModel}, [2]string{".project.json", probeProject}), "no .meta.json at the top level"},
		{"nested folder", zipOf(t, [2]string{"probe/.project.json", probeProject}, [2]string{"probe/.meta.json", probeMeta}, [2]string{"probe/Probe.sysml", probeModel}), "holds a folder rather than a project"},
		{"project not json", zipOf(t, [2]string{".project.json", "{"}, [2]string{".meta.json", probeMeta}), ".project.json"},
		{"project not an object", zipOf(t, [2]string{".project.json", "[]"}, [2]string{".meta.json", probeMeta}), "does not hold a JSON object"},
		{"project without name", zipOf(t, [2]string{".project.json", `{"version": "1"}`}, [2]string{".meta.json", probeMeta}), "name"},
		{"project without version", zipOf(t, [2]string{".project.json", `{"name": "Probe"}`}, [2]string{".meta.json", probeMeta}), "version"},
		{"name of the wrong type", zipOf(t, [2]string{".project.json", `{"name": 1, "version": "1"}`}, [2]string{".meta.json", probeMeta}), "name"},
		{"usage without resource", zipOf(t, [2]string{".project.json", `{"name": "Probe", "version": "1", "usage": [{"versionConstraint": "1"}]}`}, [2]string{".meta.json", probeMeta}), "resource"},
		{"meta without created", zipOf(t, [2]string{".project.json", probeProject}, [2]string{".meta.json", `{"index": {}}`}), "created"},
		{"meta without index", zipOf(t, [2]string{".project.json", probeProject}, [2]string{".meta.json", `{"created": "2026-01-01T00:00:00Z"}`}), "index"},
		{"index names a missing member", zipOf(t, [2]string{".project.json", probeProject}, [2]string{".meta.json", `{"index": {"Probe": "Gone.sysml"}, "created": "2026-01-01T00:00:00Z"}`}), `"Gone.sysml", which is not a member`},
		{"checksum of a missing member", zipOf(t, [2]string{".project.json", probeProject}, [2]string{".meta.json", `{"index": {}, "created": "2026-01-01T00:00:00Z", "checksum": {"Gone.sysml": {"value": "00", "algorithm": "SHA256"}}}`}), "checksum is recorded for"},
		{"checksum does not match", zipOf(t, [2]string{"Probe.sysml", "package Changed;"}, [2]string{".project.json", probeProject}, [2]string{".meta.json", probeMeta}), "does not match"},
		{"checksum algorithm outside the schema", zipOf(t, [2]string{"Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", `{"index": {}, "created": "2026-01-01T00:00:00Z", "checksum": {"Probe.sysml": {"value": "00", "algorithm": "CRC32"}}}`}), "not an algorithm the interchange schema allows"},
		{"metamodel not an IRI", zipOf(t, [2]string{".project.json", probeProject}, [2]string{".meta.json", `{"index": {}, "created": "2026-01-01T00:00:00Z", "metamodel": "kerml"}`}), "metamodel"},
		{"parent segment", zipOf(t, [2]string{"../Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", probeMeta}), `member "../Probe.sysml"`},
		{"absolute path", zipOf(t, [2]string{"/etc/Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", probeMeta}), "path is absolute"},
		{"backslash path", zipOf(t, [2]string{`docs\Probe.sysml`, probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", probeMeta}), `separated by /`},
		{"dot segment", zipOf(t, [2]string{"./Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", probeMeta}), "path has a . segment"},
		{"duplicate member", zipOf(t, [2]string{"Probe.sysml", probeModel}, [2]string{"Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", probeMeta}), "appears twice"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseArchive("lib/probe.kpar", c.data)
			var ae *ArchiveError
			if !errors.As(err, &ae) {
				t.Fatalf("err = %v, want an *ArchiveError", err)
			}
			if ae.Path != "lib/probe.kpar" || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %q, want it on lib/probe.kpar mentioning %q", err, c.want)
			}
		})
	}
}

func TestParseArchiveNotesAChecksumItCannotCompute(t *testing.T) {
	meta := `{"index": {}, "created": "2026-01-01T00:00:00Z", "checksum": {"Probe.sysml": [{"value": "00", "algorithm": "SHA256"}, {"value": "00", "algorithm": "BLAKE3"}]}}`
	_, err := ParseArchive("probe.kpar", zipOf(t, [2]string{"Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", meta}))
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("a wrong SHA256 beside a BLAKE3 should still fail: %v", err)
	}
	a, err := ParseArchive("probe.kpar", zipOf(t, [2]string{"Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", strings.Replace(meta, `"00", "algorithm": "SHA256"`, `"4044ca78bfbbcdd72b63cd877ad948c20b758dd30cbb4a0bf8c9e0af30c98bda", "algorithm": "SHA256"`, 1)}))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Notes) != 1 || !strings.Contains(a.Notes[0], "BLAKE3") {
		t.Errorf("notes = %q, want the BLAKE3 checksum noted as unverified", a.Notes)
	}
}

func TestReadFileReportsAMalformedArchiveOnItsPath(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "broken"+ArchiveExt)
	if err := os.WriteFile(bad, []byte("package NotZipped;"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The error is the archive's own; callers report it under the path.
	if _, _, err := ReadFile(bad); err == nil || !strings.Contains(err.Error(), "not a ZIP archive") || strings.Contains(err.Error(), bad) {
		t.Fatalf("ReadFile(%s) err = %v, want the archive failure without the path", bad, err)
	}

	good := filepath.Join(dir, "probe"+ArchiveExt)
	if err := os.WriteFile(good, probeArchive(t), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadFile(good); err == nil || !strings.Contains(err.Error(), "not a model file") {
		t.Errorf("ReadFile of a valid archive = %v, want a refusal naming what it is", err)
	}
	_, content, err := ReadFile(MemberPath(good, "Probe.sysml"))
	if err != nil || string(content) != probeModel {
		t.Errorf("ReadFile of a member = %q, %v", content, err)
	}
	if _, _, err := ReadFile(MemberPath(good, "Gone.sysml")); err == nil {
		t.Error("ReadFile of a missing member succeeded")
	}
}

// A member path is one whose leading directories include an archive that
// exists; the member part is slash-separated whatever the host uses.
func TestSplitMemberPath(t *testing.T) {
	dir := t.TempDir()
	kpar := filepath.Join(dir, "probe"+ArchiveExt)
	if err := os.WriteFile(kpar, probeArchive(t), 0o600); err != nil {
		t.Fatal(err)
	}
	archive, member, ok := SplitMemberPath(filepath.Join(kpar, "sub", "A.sysml"))
	if !ok || archive != kpar || member != "sub/A.sysml" {
		t.Errorf("SplitMemberPath = %q, %q, %v", archive, member, ok)
	}
	if _, _, ok := SplitMemberPath(kpar); ok {
		t.Error("an archive path alone is not a member path")
	}
	if _, _, ok := SplitMemberPath(filepath.Join(dir, "A.sysml")); ok {
		t.Error("a plain file is not a member path")
	}
	if _, _, ok := SplitMemberPath(filepath.Join(dir, "absent.kpar", "A.sysml")); ok {
		t.Error("a .kpar that does not exist is a directory name, not an archive")
	}
}

func TestParseMetaReadsBothChecksumShapes(t *testing.T) {
	const object = `{"index": {}, "created": "2026-01-01T00:00:00Z", "checksum": {"A.sysml": {"value": "00", "algorithm": "SHA256"}}}`
	const list = `{"index": {}, "created": "2026-01-01T00:00:00Z", "checksum": {"A.sysml": [{"value": "00", "algorithm": "SHA256"}, {"value": "11", "algorithm": "MD5"}]}}`
	meta, err := ParseMeta([]byte(object))
	if err != nil {
		t.Fatal(err)
	}
	if got := meta.Checksum["A.sysml"]; len(got) != 1 || got[0] != (Checksum{Value: "00", Algorithm: "SHA256"}) {
		t.Errorf("object shape read as %+v", got)
	}
	meta, err = ParseMeta([]byte(list))
	if err != nil {
		t.Fatal(err)
	}
	if got := meta.Checksum["A.sysml"]; len(got) != 2 || got[1].Algorithm != "MD5" {
		t.Errorf("list shape read as %+v", got)
	}
	// Meta itself is the JSON type, so a plain Unmarshal accepts both shapes too.
	var direct Meta
	if err := json.Unmarshal([]byte(object), &direct); err != nil {
		t.Errorf("json.Unmarshal of the object shape: %v", err)
	}
	if err := json.Unmarshal([]byte(list), &direct); err != nil {
		t.Errorf("json.Unmarshal of the list shape: %v", err)
	}
	for _, bad := range []struct{ name, meta, want string }{
		{"empty list", `{"index": {}, "created": "2026-01-01T00:00:00Z", "checksum": {"A.sysml": []}}`, `checksum of "A.sysml" is an empty list`},
		{"a string", `{"index": {}, "created": "2026-01-01T00:00:00Z", "checksum": {"A.sysml": "00"}}`, `checksum of "A.sysml" is neither`},
		{"null", `{"index": {}, "created": "2026-01-01T00:00:00Z", "checksum": {"A.sysml": null}}`, `checksum of "A.sysml" is neither`},
	} {
		if _, err := ParseMeta([]byte(bad.meta)); err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("%s: err = %v, want %q", bad.name, err, bad.want)
		}
	}
}

func TestChecksumsMarshalAsTheSchemaObject(t *testing.T) {
	meta := Meta{Index: map[string]string{}, Created: "2026-01-01T00:00:00Z", Checksum: map[string]Checksums{
		"A.sysml": {{Value: "00", Algorithm: "SHA256"}},
		"B.sysml": {{Value: "00", Algorithm: "SHA256"}, {Value: "11", Algorithm: "MD5"}},
	}}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"index":{},"created":"2026-01-01T00:00:00Z","checksum":{"A.sysml":{"value":"00","algorithm":"SHA256"},"B.sysml":[{"value":"00","algorithm":"SHA256"},{"value":"11","algorithm":"MD5"}]}}`
	if string(data) != want {
		t.Errorf("marshalled as\n%s\nwant\n%s", data, want)
	}
	var back Meta
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Checksum["A.sysml"]) != 1 || len(back.Checksum["B.sysml"]) != 2 {
		t.Errorf("round trip read %+v", back.Checksum)
	}
}

func TestParseArchiveSkipsAnUnrecordedChecksum(t *testing.T) {
	for _, tc := range []struct{ name, sum string }{
		{"algorithm NONE", `{"value": "", "algorithm": "NONE"}`},
		{"algorithm none in lower case", `{"value": "", "algorithm": "none"}`},
		{"NONE with a value", `{"value": "00", "algorithm": "NONE"}`},
		{"an empty value", `{"value": "", "algorithm": "SHA256"}`},
		{"unrecorded beside a verified one", `[{"value": "", "algorithm": "NONE"}, {"value": "4044ca78bfbbcdd72b63cd877ad948c20b758dd30cbb4a0bf8c9e0af30c98bda", "algorithm": "SHA256"}]`},
	} {
		meta := `{"index": {"Probe": "Probe.sysml"}, "created": "2026-01-01T00:00:00Z", "checksum": {"Probe.sysml": ` + tc.sum + `}}`
		a, err := ParseArchive("probe.kpar", zipOf(t, [2]string{"Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", meta}))
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if len(a.Notes) != 0 {
			t.Errorf("%s: notes = %q, want none: an unrecorded checksum is not a finding", tc.name, a.Notes)
		}
		if sums := a.Meta.Checksum["Probe.sysml"]; len(sums) == 0 || !sums[0].Unrecorded() {
			t.Errorf("%s: checksums kept as %+v", tc.name, sums)
		}
	}
	for _, tc := range []struct{ name, sum, want string }{
		{"no algorithm", `{"value": "", "algorithm": ""}`, "lacks an algorithm"},
		{"an empty value of an algorithm outside the schema", `{"value": "", "algorithm": "CRC32"}`, "not an algorithm the interchange schema allows"},
		{"a wrong value beside an unrecorded one", `[{"value": "", "algorithm": "NONE"}, {"value": "00", "algorithm": "SHA256"}]`, "does not match"},
	} {
		meta := `{"index": {"Probe": "Probe.sysml"}, "created": "2026-01-01T00:00:00Z", "checksum": {"Probe.sysml": ` + tc.sum + `}}`
		_, err := ParseArchive("probe.kpar", zipOf(t, [2]string{"Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", meta}))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
}
