package libs

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/pack"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/graphcmp"
)

// builtSnapshot is the snapshot of the bundled library, built once per test
// binary: the decode path is exercised from it whether or not the tree has a
// generated snapshot to embed.
var builtSnapshot struct {
	once sync.Once
	data []byte
	err  error
}

// testSnapshot returns the snapshot BuildSnapshot writes for the bundled library.
func testSnapshot(tb testing.TB) []byte {
	tb.Helper()
	builtSnapshot.once.Do(func() {
		builtSnapshot.data, builtSnapshot.err = BuildSnapshot(BundledSource())
	})
	if builtSnapshot.err != nil {
		tb.Fatalf("BuildSnapshot: %v", builtSnapshot.err)
	}
	return builtSnapshot.data
}

// testSnapshotIndex is SnapshotIndex over the snapshot testSnapshot builds.
func testSnapshotIndex(tb testing.TB) *symbols.Index {
	tb.Helper()
	idx, err := snapshotIndexFrom(testSnapshot(tb), DefaultSource())
	if err != nil {
		tb.Fatalf("decode the snapshot: %v", err)
	}
	return idx
}

// TestEmbeddedSnapshotIsCurrent fails when a generated snapshot is embedded
// that was not regenerated after the bundled library, or the snapshot format,
// changed. A tree without one — a module-proxy checkout — has nothing to check.
func TestEmbeddedSnapshotIsCurrent(t *testing.T) {
	if stdlibSnapshot == nil {
		t.Skip("no snapshot embedded; `make stdlib-snapshot` generates one")
	}
	if want := testSnapshot(t); !bytes.Equal(want, stdlibSnapshot) {
		t.Fatalf("stdlib.snapshot is stale (%d bytes embedded, %d regenerated); run `go generate ./internal/workspace/libs`",
			len(stdlibSnapshot), len(want))
	}
}

// TestSnapshotIndexTellsAnAbsentSnapshotFromABrokenOne pins the two fallbacks
// apart: a build without a snapshot is expected and only noted at debug level,
// a snapshot that fails to decode is a build defect and warned about.
func TestSnapshotIndexTellsAnAbsentSnapshotFromABrokenOne(t *testing.T) {
	var absent []byte
	if _, err := snapshotIndexFrom(absent, BundledSource()); !errors.Is(err, ErrNoSnapshot) {
		t.Errorf("absent snapshot: got %v, want ErrNoSnapshot", err)
	}
	broken := []byte("not a snapshot")
	if _, err := snapshotIndexFrom(broken, BundledSource()); !errors.Is(err, pack.ErrCorrupt) {
		t.Errorf("broken snapshot: got %v, want ErrCorrupt", err)
	}

	var logs bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(original) })

	if idx := frozenLibraryFrom(absent, BundledSource()); len(idx.LookupQualified("Base::Anything")) == 0 {
		t.Error("absent snapshot: the library files were not loaded")
	}
	if got := logs.String(); !strings.Contains(got, "level=DEBUG") || strings.Contains(got, "level=WARN") {
		t.Errorf("absent snapshot: want one debug record and no warning, got %q", got)
	}
	logs.Reset()
	if idx := frozenLibraryFrom(broken, BundledSource()); len(idx.LookupQualified("Base::Anything")) == 0 {
		t.Error("broken snapshot: the library files were not loaded")
	}
	if got := logs.String(); !strings.Contains(got, "level=WARN") {
		t.Errorf("broken snapshot: want a warning, got %q", got)
	}
}

// TestSnapshotIndexMatchesFreshLoad locks the decoded snapshot to the index a
// load of the same files builds, on the same surface the concurrent load is
// held to, and checks that re-encoding it gives the snapshot back.
func TestSnapshotIndexMatchesFreshLoad(t *testing.T) {
	fresh := symbols.NewIndex()
	loader := NewLoader(BundledSource(), nil)
	if err := loader.LoadAll(fresh); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	fresh.Freeze()

	decoded, err := DecodeSnapshot(testSnapshot(t), loader.setDigest())
	if err != nil {
		t.Fatalf("DecodeSnapshot: %v", err)
	}
	if !decoded.Frozen() {
		t.Fatal("decoded index is not frozen")
	}
	// The whole object graph, less the lookup caches an index fills lazily and
	// the inline storage a multi-part name leaves behind (see symbols' tests).
	if err := graphcmp.Equal(fresh, decoded, graphcmp.SkipFields(
		"memberCache.generation", "libraryIdentityMemo.gen", "memberCache.children",
		"memberCache.byName", "memberCache.shortNamed", "QualifiedName.part0",
	)); err != nil {
		t.Errorf("decoded index differs from a fresh load: %v", err)
	}
	want, got := indexFingerprint(fresh), indexFingerprint(decoded)
	if want != got {
		t.Errorf("decoded index differs from a fresh load:\n%s", firstDifference(want, got))
	}
	if want, got := indexRecords(fresh), indexRecords(decoded); !reflect.DeepEqual(want, got) {
		t.Errorf("derived records differ from a fresh load")
	}

	w := pack.NewWriter()
	if err := decoded.WriteSnapshot(w); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}
	if again, err := BuildSnapshot(BundledSource()); err != nil {
		t.Fatal(err)
	} else if !bytes.HasSuffix(again, w.Bytes()) {
		t.Errorf("re-encoding the decoded index does not reproduce the snapshot stream")
	}
}

// snapshotKindDigest pins the SymbolKind and PseudostateKind numbering the snapshot
// stream persists under snapshotFormatVersion; a kind added or moved renumbers the ones after it.
const snapshotKindDigest = "405b5e666a52ba75"

func TestSnapshotFormatVersionPinsSymbolKinds(t *testing.T) {
	var b strings.Builder
	fmt.Fprintf(&b, "format %d\n", snapshotFormatVersion)
	for k := symbols.SymbolUnknown; k <= symbols.SymbolMultiplicity; k++ {
		fmt.Fprintf(&b, "%d=%s\n", int(k), k)
	}
	for k := ast.PseudostateChoice; k <= ast.PseudostateDeepHistory; k++ {
		fmt.Fprintf(&b, "pseudostate %d=%s\n", int(k), k)
	}
	sum := sha256.Sum256([]byte(b.String()))
	if got := hex.EncodeToString(sum[:8]); got != snapshotKindDigest {
		t.Fatalf("SymbolKind or PseudostateKind numbering changed under snapshot format %d (digest %s, want %s).\n"+
			"The snapshot stream persists kinds as integers: bump snapshotFormatVersion so a "+
			"blob in the old numbering is refused, then update snapshotKindDigest.",
			snapshotFormatVersion, got, snapshotKindDigest)
	}
}

func TestDecodeSnapshotRefusesOtherFiles(t *testing.T) {
	snapshot := testSnapshot(t)
	if _, err := DecodeSnapshot(snapshot, "0000"); !errors.Is(err, ErrSnapshotStale) {
		t.Errorf("digest mismatch: got %v, want ErrSnapshotStale", err)
	}
	// A blob in the previous or a later format is refused before its stream
	// is read, so a layout change never decodes into shifted fields. Format 8
	// numbered the kinds before SymbolCrossFeature was added; format 9 wrote a
	// CrossFeatureMember without its prefix fields, format 10 without ordered/nonunique,
	// format 11 a member without how it came by its name, format 12 a Usage with a
	// ValueMultiplicity field, format 13 the pseudostate kinds with entry and exit
	// points numbered before the histories, format 14 a TransitionMember without
	// IsSuccession and an Invariant without IsNegated, format 15 an InitialNode's
	// first end as a name and span rather than a QualifiedName, format 16 a Usage
	// without IsTerminate.
	digest := NewLoader(BundledSource(), nil).setDigest()
	for _, version := range []uint64{8, 9, 10, 11, 12, 13, 14, snapshotFormatVersion - 1, snapshotFormatVersion + 1} {
		other := binary.AppendUvarint([]byte(snapshotMagic), version)
		other = binary.AppendUvarint(other, uint64(len(digest)))
		other = append(other, digest...)
		other = append(other, snapshot[len(other):]...)
		if _, err := DecodeSnapshot(other, digest); !errors.Is(err, ErrSnapshotStale) {
			t.Errorf("format %d: got %v, want ErrSnapshotStale", version, err)
		}
	}
}

// TestDecodeSnapshotRejectsCorruption feeds truncated and altered snapshots to
// the decoder, which must report each rather than panic or return an index.
func TestDecodeSnapshotRejectsCorruption(t *testing.T) {
	snapshot := testSnapshot(t)
	digest := NewLoader(BundledSource(), nil).setDigest()
	if _, err := DecodeSnapshot([]byte("not a snapshot"), digest); !errors.Is(err, pack.ErrCorrupt) {
		t.Errorf("garbage: got %v, want ErrCorrupt", err)
	}
	for _, n := range []int{len(snapshotMagic) + 3, len(snapshot) / 2, len(snapshot) - 1} {
		if _, err := DecodeSnapshot(snapshot[:n], digest); err == nil {
			t.Errorf("truncated to %d bytes: decoded without error", n)
		}
	}
	// Damage that still parses — a byte inside a string, a run in the string
	// table — is caught by the checksum, so no altered blob is decoded.
	header := len(snapshotMagic) + len(digest) + 3
	for _, at := range []int{header, 200000, len(snapshot) * 3 / 4, len(snapshot) - 200} {
		data := bytes.Clone(snapshot)
		data[at] ^= 0xff
		if _, err := DecodeSnapshot(data, digest); !errors.Is(err, pack.ErrCorrupt) {
			t.Errorf("byte %d flipped: got %v, want ErrCorrupt", at, err)
		}
	}
	data := bytes.Clone(snapshot)
	for i := 200000; i < 200064; i++ {
		data[i] ^= 0xff
	}
	if _, err := DecodeSnapshot(data, digest); !errors.Is(err, pack.ErrCorrupt) {
		t.Errorf("64 bytes flipped: got %v, want ErrCorrupt", err)
	}
	if _, err := DecodeSnapshot(append(bytes.Clone(snapshot), 0), digest); !errors.Is(err, pack.ErrCorrupt) {
		t.Errorf("trailing byte: got %v, want ErrCorrupt", err)
	}
}

// TestSnapshotIndexFollowsLibraryPath checks that a library override with
// other content is loaded from its files, while one with the bundled content
// is served from the snapshot.
func TestSnapshotIndexFollowsLibraryPath(t *testing.T) {
	dir := t.TempDir()
	writeLibraryTo(t, BundledSource(), dir)
	t.Setenv(LibraryPathEnvVar, dir)
	snapshot := testSnapshot(t)
	if _, err := snapshotIndexFrom(snapshot, DefaultSource()); err != nil {
		t.Errorf("identical override: %v, want the snapshot", err)
	}

	edited := filepath.Join(dir, "Kernel Libraries", "Kernel Data Type Library", "ScalarValues.kerml")
	if err := os.WriteFile(edited, []byte("standard library package ScalarValues { datatype Edited; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotIndexFrom(snapshot, DefaultSource()); !errors.Is(err, ErrSnapshotStale) {
		t.Errorf("edited override: got %v, want ErrSnapshotStale", err)
	}
	idx := symbols.NewIndex()
	if err := NewLoader(DefaultSource(), nil).LoadAll(idx); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(idx.LookupQualified("ScalarValues::Edited")) == 0 {
		t.Error("the edited library was not loaded from the override directory")
	}
}

// writeLibraryTo copies every file of src under dir, as a library override.
func writeLibraryTo(t *testing.T, src Source, dir string) {
	t.Helper()
	for _, name := range src.List() {
		data, err := src.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// BenchmarkDecodeSnapshot times what a process pays to start from the
// snapshot instead of from the files.
func BenchmarkDecodeSnapshot(b *testing.B) {
	snapshot := testSnapshot(b)
	digest := NewLoader(BundledSource(), nil).setDigest()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeSnapshot(snapshot, digest); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSetDigest times the check that the snapshot matches the files.
func BenchmarkSetDigest(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NewLoader(BundledSource(), nil).setDigest()
	}
}
