package symbols

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/pack"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/graphcmp"
)

// unobservable is state two equal indexes may hold differently: lookup caches
// a decoded index rebuilds lazily, and the inline segment storage a multi-part
// name leaves behind once its parts are appended elsewhere.
var unobservable = graphcmp.SkipFields(
	"Index.directChildrenGeneration", "libraryIdentityMemo.gen",
	"Index.directChildrenCache", "Index.directChildrenByName", "Index.shortNamedCache",
	"QualifiedName.part0",
)

// snapshotDocs are library shapes whose expansion state crosses documents:
// re-exports, private imports, filters, aliases, cycles and effective names.
var snapshotDocs = map[string]string{
	"a.sysml": "package A { part def Widget; part def <sn> Short; private part def Hidden; " +
		"public import B::*; private import C::*; alias W for Widget; }",
	"b.sysml": "package B { part def Gadget; public import A::*; public import C::*[@Safety]; " +
		"package Inner { public import A::*; } }",
	"c.sysml": "package C { part def Safety; part def Loose; public import D::*; }",
	"d.sysml": "package D { public import A::*; part def Cyclic; part x : Widget; part :>> Gadget; " +
		"metadata def Safety; metadata s : Safety about Widget; }",
	"e.kerml": "package E { classifier K; feature f : K; import A::*; }",
	"f.sysml": "package F { action def Run { action a; action b; first a then b; then done; } }",
}

func roundTrip(t *testing.T, idx *Index) *Index {
	t.Helper()
	w := pack.NewWriter()
	if err := idx.WriteSnapshot(w); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}
	r, err := pack.NewReader(w.Bytes())
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	got, err := ReadSnapshot(r)
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}
	return got
}

// A decoded snapshot must be the frozen index it was written from: the same
// object graph, down to which tables share which symbols and scopes.
func TestSnapshotRoundTripKeepsTheGraph(t *testing.T) {
	want := frozenBase(t, snapshotDocs)
	got := roundTrip(t, want)
	if err := graphcmp.Equal(want, got, unobservable); err != nil {
		t.Fatal(err)
	}
	if got, want := indexState(got), indexState(want); got != want {
		t.Errorf("index state differs:\n%s", diffLines(want, got))
	}
}

// Writing the decoded index again must give the bytes it was read from.
func TestSnapshotReencodesIdentically(t *testing.T) {
	first := pack.NewWriter()
	if err := frozenBase(t, snapshotDocs).WriteSnapshot(first); err != nil {
		t.Fatal(err)
	}
	r, err := pack.NewReader(first.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	idx, err := ReadSnapshot(r)
	if err != nil {
		t.Fatal(err)
	}
	second := pack.NewWriter()
	if err := idx.WriteSnapshot(second); err != nil {
		t.Fatal(err)
	}
	if string(first.Bytes()) != string(second.Bytes()) {
		t.Fatal("re-encoding the decoded index changed the bytes")
	}
}

// An overlay over a decoded base must behave as one over the base it was
// written from, through additions, removals and re-additions of documents.
func TestOverlayOverADecodedBaseMatchesTheOriginal(t *testing.T) {
	cases := []struct {
		name string
		user string
	}{
		{"user imports the base", "package User { public import A::*; part u : Widget; }"},
		{"base imports a namespace the user declares", "package Late { part def Arrived; } " +
			"package Also { public import D::*; }"},
		{"user re-declares a base namespace", "package A { part def FromUser; }"},
		{"cycle across the boundary", "package Loop { public import B::*; } package B { public import Loop::*; }"},
		{"filtered import of base content", "package User { public import C::*[@Safety]; }"},
		{"document-root import", "public import A::*; part top : Gadget;"},
	}
	const userDoc = "user.sysml"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fresh := frozenBase(t, snapshotDocs)
			decoded := roundTrip(t, fresh)
			user := map[string]string{userDoc: tc.user}
			overFresh := overlayOver(t, fresh, user)
			overDecoded := overlayOver(t, decoded, user)
			if got, want := indexState(overDecoded), indexState(overFresh); got != want {
				t.Errorf("overlay over the decoded base differs:\n%s", diffLines(want, got))
			}
			if err := graphcmp.Equal(overFresh, overDecoded, unobservable); err != nil {
				t.Errorf("overlay graphs differ: %v", err)
			}

			overFresh.RemoveDocument(userDoc)
			overDecoded.RemoveDocument(userDoc)
			overFresh.ExpandWildcardImports()
			overDecoded.ExpandWildcardImports()
			if got, want := indexState(overDecoded), indexState(overFresh); got != want {
				t.Errorf("after removing the user document:\n%s", diffLines(want, got))
			}

			addDoc(t, overFresh, userDoc, tc.user)
			addDoc(t, overDecoded, userDoc, tc.user)
			overFresh.ExpandWildcardImports()
			overDecoded.ExpandWildcardImports()
			if got, want := indexState(overDecoded), indexState(overFresh); got != want {
				t.Errorf("after re-adding the user document:\n%s", diffLines(want, got))
			}
			if err := graphcmp.Equal(fresh, decoded, unobservable); err != nil {
				t.Errorf("the base changed under its overlay: %v", err)
			}
		})
	}
}

func TestSnapshotAnswersEveryLookupAsTheOriginal(t *testing.T) {
	fresh := frozenBase(t, snapshotDocs)
	decoded := roundTrip(t, fresh)
	froms := append([]string{"", "A", "B", "B::Inner", "C", "D", "E"}, fresh.FQNs()...)
	for _, fqn := range fresh.FQNs() {
		if got, want := symNames(decoded.LookupQualified(fqn)), symNames(fresh.LookupQualified(fqn)); !equalStrings(got, want) {
			t.Errorf("LookupQualified(%q) = %v, want %v", fqn, got, want)
		}
		if got, want := symNames(decoded.LookupDirectChildren(fqn)), symNames(fresh.LookupDirectChildren(fqn)); !equalStrings(got, want) {
			t.Errorf("LookupDirectChildren(%q) = %v, want %v", fqn, got, want)
		}
		for _, from := range froms {
			if got, want := symNames(decoded.LookupQualifiedFrom(fqn, from)), symNames(fresh.LookupQualifiedFrom(fqn, from)); !equalStrings(got, want) {
				t.Errorf("LookupQualifiedFrom(%q, %q) = %v, want %v", fqn, from, got, want)
			}
			if got, want := symNames(decoded.LookupDirectChildrenFrom(fqn, from)), symNames(fresh.LookupDirectChildrenFrom(fqn, from)); !equalStrings(got, want) {
				t.Errorf("LookupDirectChildrenFrom(%q, %q) = %v, want %v", fqn, from, got, want)
			}
		}
		for _, doc := range fresh.Documents() {
			for i, sym := range fresh.LookupQualified(fqn) {
				other := decoded.LookupQualified(fqn)[i]
				if got, want := decoded.ReexportVisible(doc, fqn, other), fresh.ReexportVisible(doc, fqn, sym); got != want {
					t.Errorf("ReexportVisible(%q, %q, %s) = %v, want %v", doc, fqn, sym.Name, got, want)
				}
				if got, want := decoded.GetFQN(other), fresh.GetFQN(sym); got != want {
					t.Errorf("GetFQN(%s) = %q, want %q", sym.Name, got, want)
				}
				if got, want := decoded.Library(other), fresh.Library(sym); got != want {
					t.Errorf("Library(%s) = %v, want %v", sym.Name, got, want)
				}
			}
		}
	}
	for _, doc := range fresh.Documents() {
		want, _ := fresh.FrozenAboutUsages(doc)
		got, _ := decoded.FrozenAboutUsages(doc)
		if !equalStrings(symNames(got), symNames(want)) {
			t.Errorf("FrozenAboutUsages(%q) = %v, want %v", doc, symNames(got), symNames(want))
		}
	}
}

func TestWriteSnapshotRejectsWhatItCannotRebuild(t *testing.T) {
	writable := buildIndex(t, snapshotDocs)
	if err := writable.WriteSnapshot(pack.NewWriter()); !errors.Is(err, ErrNotSnapshottable) {
		t.Errorf("writable index: err = %v, want ErrNotSnapshottable", err)
	}
	over := overlayOver(t, frozenBase(t, snapshotDocs), map[string]string{"u.sysml": "package U;"})
	over.Freeze()
	if err := over.WriteSnapshot(pack.NewWriter()); !errors.Is(err, ErrNotSnapshottable) {
		t.Errorf("overlay: err = %v, want ErrNotSnapshottable", err)
	}
}

// Every truncation of a snapshot must be reported, and no corruption of one
// may panic: the reader sees the bytes before anything has vouched for them.
func TestReadSnapshotRejectsDamagedBytes(t *testing.T) {
	w := pack.NewWriter()
	if err := frozenBase(t, snapshotDocs).WriteSnapshot(w); err != nil {
		t.Fatal(err)
	}
	data := w.Bytes()
	read := func(b []byte) error {
		r, err := pack.NewReader(b)
		if err != nil {
			return err
		}
		_, err = ReadSnapshot(r)
		return err
	}
	for n := 0; n < len(data); n += 7 {
		if err := read(data[:n]); err == nil {
			t.Errorf("truncated to %d bytes: no error", n)
		}
	}
	for i := 0; i < len(data); i++ {
		damaged := append([]byte(nil), data...)
		damaged[i] ^= 0x5b
		_ = read(damaged)
	}
	if err := read(append(append([]byte(nil), data...), 0)); err == nil || !strings.Contains(err.Error(), "after") {
		t.Errorf("trailing byte: err = %v", err)
	}
}
