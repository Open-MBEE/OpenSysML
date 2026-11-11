package baseline

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	oracle "github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
)

// The toy referee: a file's summary counts its elements, a row is two buckets,
// and the aggregate sums both over the fragments.
type (
	summary   struct{ Elements int }
	row       [2]int
	aggregate struct{ Elements, Rows int }
	fragment  = Fragment[summary, row]
	toy       = Baseline[summary, row, aggregate]
)

func fold(fragments []fragment) aggregate {
	var out aggregate
	for _, f := range fragments {
		out.Elements += f.Summary.Elements
		out.Rows += len(f.Rows)
	}
	return out
}

var commands = Commands{Refresh: "toy -update", Reindex: "toy -reindex"}

func provenance() oracle.Record {
	return oracle.Record{
		PilotTag: "2026-09", PilotCommit: "abc", PilotArtifact: "0.63.0", Errata: "sha256:errata",
		Inputs: []oracle.Input{{Name: "corpus", Digest: "sha256:corpus"}, {Name: "a", Digest: "sha256:a"}, {Name: "b", Digest: "sha256:b"}},
	}
}

func fixture() toy {
	fragments := []fragment{
		{Root: "b", File: "deep/nested/z.sysml", Summary: summary{Elements: 1}, Rows: map[string]row{"Zed.name": {1, 0}}},
		{Root: "a", File: "y.sysml", Summary: summary{Elements: 2}, Rows: map[string]row{"Why.name": {2, 0}, "Why.type": {1, 1}}},
		{Root: "a", File: "x.sysml", Summary: summary{Elements: 3}, Rows: map[string]row{"Ex.name": {3, 0}}},
	}
	return toy{Provenance: provenance(), Summary: fold(fragments), Fragments: fragments}
}

// at finds a fragment of the run by file; Check sorts the run, so indices do not name a file.
func at(b *toy, file string) *fragment {
	for i := range b.Fragments {
		if b.Fragments[i].File == file {
			return &b.Fragments[i]
		}
	}
	return nil
}

func record(t *testing.T, b toy) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "baseline")
	if err := Write(dir, b, io.Discard); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestWriteLaysOutOneFragmentPerFileAndReadsThemBack(t *testing.T) {
	fresh := fixture()
	dir := record(t, fresh)
	for _, want := range []string{"index.json", "a/x.sysml.json", "a/y.sysml.json", "b/deep/nested/z.sysml.json"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(want))); err != nil {
			t.Errorf("%s: %v", want, err)
		}
	}
	content, err := os.ReadFile(filepath.Join(dir, "a", "y.sysml.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"root\": \"a\",\n  \"file\": \"y.sysml\",\n  \"summary\": {\n    \"Elements\": 2\n  },\n  \"rows\": {\n    \"Why.name\": [2,0],\n    \"Why.type\": [1,1]\n  }\n}\n"
	if string(content) != want {
		t.Errorf("fragment encoding:\n%s\nwant:\n%s", content, want)
	}
	var index Index[aggregate]
	indexContent, err := os.ReadFile(filepath.Join(dir, IndexFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(indexContent, &index); err != nil {
		t.Fatal(err)
	}
	if want := map[string][]string{"a": {"x.sysml", "y.sysml"}, "b": {"deep/nested/z.sysml"}}; !reflect.DeepEqual(index.Files, want) {
		t.Errorf("index files %v, want %v", index.Files, want)
	}
	if index.Summary != (aggregate{Elements: 6, Rows: 4}) {
		t.Errorf("index summary %+v", index.Summary)
	}
	read, err := Read[summary, row, aggregate](dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read, fresh) {
		t.Errorf("read back %+v\nwant %+v", read, fresh)
	}
	if err := Verify(dir, fold); err != nil {
		t.Error(err)
	}
	if err := Check(dir, fixture(), commands); err != nil {
		t.Error(err)
	}
}

func TestWriteRemovesFragmentsOfFilesNoLongerRecorded(t *testing.T) {
	dir := record(t, fixture())
	fresh := fixture()
	fresh.Fragments = fresh.Fragments[1:] // the fixture lists b/deep/nested/z.sysml first
	fresh.Summary = fold(fresh.Fragments)
	if err := Write(dir, fresh, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b")); !os.IsNotExist(err) {
		t.Errorf("root b's directory survived its last fragment: %v", err)
	}
	if err := Verify(dir, fold); err != nil {
		t.Error(err)
	}
}

func TestWriteRefusesTwoFragmentsForOneFileAndEscapingPaths(t *testing.T) {
	for name, fragments := range map[string][]fragment{
		"duplicate":  {{Root: "a", File: "x.sysml"}, {Root: "a", File: "x.sysml"}},
		"parent":     {{Root: "a", File: "../x.sysml"}},
		"absolute":   {{Root: "a", File: "/x.sysml"}},
		"empty":      {{Root: "a", File: ""}},
		"nestedRoot": {{Root: "a/b", File: "x.sysml"}},
	} {
		b := toy{Provenance: provenance(), Fragments: fragments}
		if err := Write(filepath.Join(t.TempDir(), "baseline"), b, io.Discard); err == nil {
			t.Errorf("%s: Write accepted %+v", name, fragments)
		}
	}
	b := fixture()
	b.Provenance.PilotTag = ""
	if err := Write(filepath.Join(t.TempDir(), "baseline"), b, io.Discard); err == nil || !strings.Contains(err.Error(), "no pilot pin") {
		t.Errorf("Write accepted a baseline without a pin: %v", err)
	}
}

// Each mutation of the committed directory, or of the run held against it, must fail Check
// with a line that names the fragment and the kind of movement.
func TestCheckNamesEveryKindOfMovement(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, dir string, fresh *toy)
		want   []string
	}{
		{"moved row", func(t *testing.T, dir string, fresh *toy) {
			at(fresh, "y.sysml").Rows["Why.type"] = row{0, 2}
		}, []string{"a/y.sysml.json: row Why.type moved: [1,1] -> [0,2]", "implementation movement"}},
		{"added row", func(t *testing.T, dir string, fresh *toy) {
			at(fresh, "y.sysml").Rows["Why.owner"] = row{1, 0}
		}, []string{"a/y.sysml.json: row Why.owner added: [1,0]"}},
		{"removed row", func(t *testing.T, dir string, fresh *toy) {
			delete(at(fresh, "y.sysml").Rows, "Why.name")
		}, []string{"a/y.sysml.json: row Why.name removed: was [2,0]"}},
		{"moved per-file summary", func(t *testing.T, dir string, fresh *toy) {
			at(fresh, "x.sysml").Summary.Elements = 9
		}, []string{"a/x.sysml.json: summary.Elements: 3 -> 9"}},
		{"missing fragment", func(t *testing.T, dir string, fresh *toy) {
			fresh.Fragments = append(fresh.Fragments, fragment{Root: "a", File: "w.sysml", Rows: map[string]row{}})
		}, []string{"a/w.sysml.json: no fragment recorded for a file the run compares"}},
		{"fragment of a file no longer compared", func(t *testing.T, dir string, fresh *toy) {
			fresh.Fragments = fresh.Fragments[1:]
		}, []string{"b/deep/nested/z.sysml.json: records a file the run no longer compares"}},
		{"unlisted fragment on disk", func(t *testing.T, dir string, fresh *toy) {
			if err := os.WriteFile(filepath.Join(dir, "a", "stray.sysml.json"), []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, []string{"a/stray.sysml.json: a fragment", "index.json does not list"}},
		{"stale index figure", func(t *testing.T, dir string, fresh *toy) {
			editIndex(t, dir, func(index map[string]any) {
				index["summary"].(map[string]any)["Elements"] = 7.0
			})
		}, []string{"index.json: summary.Elements: 7 -> 6", "only the index figures are stale: rebuild them with toy -reindex"}},
		{"stale provenance", func(t *testing.T, dir string, fresh *toy) {
			editIndex(t, dir, func(index map[string]any) {
				index["provenance"].(map[string]any)["pilotTag"] = "2026-03"
			})
		}, []string{"index.json no longer describes this repository", `provenance.pilotTag: baseline records "2026-03", repository now has "2026-09"`, "re-record it with: toy -update"}},
		{"stale input digest", func(t *testing.T, dir string, fresh *toy) {
			fresh.Provenance.Inputs[0].Digest = "sha256:moved"
		}, []string{"provenance.inputs[corpus].digest"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := record(t, fixture())
			fresh := fixture()
			tc.mutate(t, dir, &fresh)
			err := Check(dir, fresh, commands)
			if err == nil {
				t.Fatal("Check passed")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Check failed without %q:\n%v", want, err)
				}
			}
		})
	}
}

func editIndex(t *testing.T, dir string, edit func(index map[string]any)) {
	t.Helper()
	path := filepath.Join(dir, IndexFile)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var index map[string]any
	if err := json.Unmarshal(content, &index); err != nil {
		t.Fatal(err)
	}
	edit(index)
	encoded, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyHoldsTheIndexToItsFragmentsWithoutTheToolchain(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, dir string)
		want   string
	}{
		{"stale figure", func(t *testing.T, dir string) {
			editIndex(t, dir, func(index map[string]any) { index["summary"].(map[string]any)["Rows"] = 5.0 })
		}, "does not sum its fragments (recorded -> folded):\n  summary.Rows: 5 -> 4"},
		{"listed fragment missing", func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "a", "x.sysml.json")); err != nil {
				t.Fatal(err)
			}
		}, "x.sysml.json is listed in index.json but missing"},
		{"unlisted fragment", func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "a", "w.sysml.json"), []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "w.sysml.json is not listed in"},
		{"fragment filed under another file", func(t *testing.T, dir string) {
			content, err := os.ReadFile(filepath.Join(dir, "a", "y.sysml.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "a", "x.sysml.json"), content, 0o600); err != nil {
				t.Fatal(err)
			}
		}, "x.sysml.json records a/y.sysml, not the file it is filed under"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := record(t, fixture())
			tc.mutate(t, dir)
			err := Verify(dir, fold)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Verify: %v\nwant a line with %q", err, tc.want)
			}
		})
	}
}

func TestRewriteReplacesOneRootAndReindexesTheRest(t *testing.T) {
	dir := record(t, fixture())
	replacement := []fragment{{Root: "a", File: "v.sysml", Summary: summary{Elements: 10}, Rows: map[string]row{"Vee.name": {1, 0}}}}
	if err := Rewrite(dir, []string{"a"}, replacement, provenance(), fold, commands, io.Discard); err != nil {
		t.Fatal(err)
	}
	read, err := Read[summary, row, aggregate](dir)
	if err != nil {
		t.Fatal(err)
	}
	want := toy{Provenance: provenance(), Summary: aggregate{Elements: 11, Rows: 2}, Fragments: []fragment{replacement[0], fixture().Fragments[0]}}
	if !reflect.DeepEqual(read, want) {
		t.Errorf("after Rewrite %+v\nwant %+v", read, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "x.sysml.json")); !os.IsNotExist(err) {
		t.Errorf("the replaced root's old fragment survived: %v", err)
	}
	if err := Rewrite(dir, nil, replacement, provenance(), fold, commands, io.Discard); err == nil || !strings.Contains(err.Error(), "not of a replaced root") {
		t.Errorf("Rewrite accepted a fragment outside the replaced roots: %v", err)
	}

	editIndex(t, dir, func(index map[string]any) { index["summary"].(map[string]any)["Elements"] = 0.0 })
	if err := Verify(dir, fold); err == nil {
		t.Fatal("Verify passed a stale index")
	}
	if err := Rewrite(dir, nil, nil, provenance(), fold, commands, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := Verify(dir, fold); err != nil {
		t.Errorf("reindexing did not mend the index: %v", err)
	}
}

func TestCheckRootsHoldsTheNamedRootsAndTheIndex(t *testing.T) {
	dir := record(t, fixture())
	fresh := func() []fragment {
		var out []fragment
		for _, f := range fixture().Fragments {
			if f.Root == "a" {
				out = append(out, f)
			}
		}
		return out
	}
	if err := CheckRoots(dir, fresh(), provenance(), []string{"a"}, fold, commands); err != nil {
		t.Fatalf("an unchanged root does not reproduce: %v", err)
	}
	moved := fresh()
	moved[0].Rows["Why.name"] = row{1, 1}
	err := CheckRoots(dir, moved, provenance(), []string{"a"}, fold, commands)
	if err == nil || !strings.Contains(err.Error(), "row Why.name moved") {
		t.Errorf("a moved row of a named root passed: %v", err)
	}
	if err := CheckRoots(dir, fixture().Fragments, provenance(), []string{"a"}, fold, commands); err == nil || !strings.Contains(err.Error(), "did not name") {
		t.Errorf("a fragment outside the named roots was accepted: %v", err)
	}
	editIndex(t, dir, func(index map[string]any) { index["summary"].(map[string]any)["Elements"] = 0.0 })
	err = CheckRoots(dir, fresh(), provenance(), []string{"a"}, fold, commands)
	if err == nil || !strings.Contains(err.Error(), commands.Reindex) {
		t.Errorf("a stale index was not named with the reindex command: %v", err)
	}
}

// Rewrite stamps the provenance it is given on fragments it did not re-measure, so a
// moved input is accepted for a replaced root only; a moved pin, errata digest or other
// input is refused, for the index alone and for a root, naming the refresh command.
func TestRewriteVouchesOnlyForTheReplacedRoots(t *testing.T) {
	dir := record(t, fixture())
	pinned := provenance()
	pinned.PilotTag = "2026-10"
	for _, replaced := range [][]string{nil, {"a"}} {
		err := Rewrite(dir, replaced, nil, pinned, fold, commands, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "pilotTag") || !strings.Contains(err.Error(), commands.Refresh) {
			t.Errorf("Rewrite of %v vouched for a moved pin: %v", replaced, err)
		}
	}
	errata := provenance()
	errata.Errata = "sha256:errata2"
	if err := Rewrite(dir, nil, nil, errata, fold, commands, io.Discard); err == nil || !strings.Contains(err.Error(), "errataRegistry") {
		t.Errorf("reindexing vouched for a moved errata digest: %v", err)
	}
	other := provenance()
	other.Inputs[2].Digest = "sha256:b2"
	if err := Rewrite(dir, []string{"a"}, nil, other, fold, commands, io.Discard); err == nil || !strings.Contains(err.Error(), "inputs[b]") {
		t.Errorf("re-recording a vouched for b's moved input: %v", err)
	}
	own := provenance()
	own.Inputs[1].Digest = "sha256:a2"
	if err := Rewrite(dir, []string{"a"}, nil, own, fold, commands, io.Discard); err != nil {
		t.Fatalf("re-recording a refused its own moved input: %v", err)
	}
	read, err := Read[summary, row, aggregate](dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read.Provenance, own) {
		t.Errorf("provenance after Rewrite %+v, want %+v", read.Provenance, own)
	}
	if read.Summary != fold(read.Fragments) || len(read.Fragments) != 1 || read.Fragments[0].Root != "b" {
		t.Errorf("re-recording a with no fragments left %+v", read)
	}
}

// Two branches that each added a file of one root merge into an index listing one
// side's file beside the other side's unlisted fragment, or listing a fragment the merge
// removed; re-recording that root resolves it, the same disagreement elsewhere is refused.
func TestRewriteIgnoresIndexDisagreementInsideTheReplacedRoots(t *testing.T) {
	dir := record(t, fixture())
	theirs := fragment{Root: "a", File: "w.sysml", Summary: summary{Elements: 4}, Rows: map[string]row{"Dub.name": {1, 0}}}
	if err := os.WriteFile(FragmentPath(dir, "a", "w.sysml"), encodeFragment(theirs), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(FragmentPath(dir, "a", "y.sysml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Read[summary, row, aggregate](dir); err == nil {
		t.Fatal("Read accepted a directory disagreeing with its index")
	}
	if err := Rewrite(dir, nil, nil, provenance(), fold, commands, io.Discard); err == nil {
		t.Error("reindexing accepted a directory disagreeing with its index")
	}
	merged := provenance()
	merged.Inputs[1].Digest = "sha256:a-merged"
	ours := []fragment{theirs, fixture().Fragments[2], fixture().Fragments[1]}
	if err := Rewrite(dir, []string{"a"}, ours, merged, fold, commands, io.Discard); err != nil {
		t.Fatalf("re-recording the root both branches added to: %v", err)
	}
	read, err := Read[summary, row, aggregate](dir)
	if err != nil {
		t.Fatal(err)
	}
	want := toy{Provenance: merged, Summary: aggregate{Elements: 10, Rows: 5}, Fragments: []fragment{theirs, fixture().Fragments[2], fixture().Fragments[1], fixture().Fragments[0]}}
	if !reflect.DeepEqual(read, want) {
		t.Errorf("after the re-record %+v\nwant %+v", read, want)
	}
	if err := Verify(dir, fold); err != nil {
		t.Error(err)
	}
	if err := os.Remove(FragmentPath(dir, "b", "deep/nested/z.sysml")); err != nil {
		t.Fatal(err)
	}
	if err := Rewrite(dir, []string{"a"}, ours, merged, fold, commands, io.Discard); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("re-recording a overlooked b's missing fragment: %v", err)
	}
}
