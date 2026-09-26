package libs

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func indexOf(t *testing.T, name, src string) *symbols.Index {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndex()
	idx.AddDocument(name, root)
	idx.MarkLibrary(name)
	return idx
}

// recordOf is recordFromIndex with a resolver over idx, as the loader builds it.
func recordOf(name string, idx *symbols.Index) *IndexRecord {
	r := resolve.New(idx)
	rec, _ := recordFromIndex(name, idx, semantics.NewModel(r))
	return rec
}

// supersByFQN is the supertype facts the record holds, keyed by qualified name,
// each rendered as refString does.
func supersByFQN(rec *IndexRecord) map[string][]string {
	out := map[string][]string{}
	for _, f := range rec.Facts {
		var supers []string
		for _, ref := range f.Supers {
			supers = append(supers, refString(ref))
		}
		out[f.FQN] = supers
	}
	return out
}

// refString renders a reference as its name, then "#k" per member ordinal and
// "@doc" for a document-qualified one.
func refString(ref symbols.ElementRef) string {
	s := ref.FQN
	for _, at := range ref.Path {
		s += fmt.Sprintf("#%d", at)
	}
	if ref.Doc != "" {
		s += "@" + ref.Doc
	}
	return s
}

// A record memoizes derived facts only: a symbol whose declaration yields
// everything about it is persisted no facts at all.
func TestRecordFromIndexPersistsDerivedFactsOnly(t *testing.T) {
	const name = "Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml"
	idx := indexOf(t, name, "standard library package ScalarValues { classifier Boolean; classifier Real :> Boolean; }")
	rec := recordOf(name, idx)
	if rec == nil {
		t.Fatal("recordFromIndex returned nil")
	}
	if rec.Name != name {
		t.Fatalf("Name = %q", rec.Name)
	}
	supers := supersByFQN(rec)
	if want := []string{"ScalarValues::Boolean"}; !slices.Equal(supers["ScalarValues::Real"], want) {
		t.Errorf("Supers of ScalarValues::Real = %v, want %v", supers["ScalarValues::Real"], want)
	}
	if _, recorded := supers["ScalarValues::Boolean"]; recorded {
		t.Error("a symbol with no derived fact is persisted anyway")
	}
}

func TestIndexRecordGobRoundTrip(t *testing.T) {
	idx := indexOf(t, "a.kerml", "package P { classifier N; classifier M :> N; }")
	rec := recordOf("a.kerml", idx)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(rec); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var got IndexRecord
	if err := gob.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != rec.Name || len(got.Facts) != len(rec.Facts) {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, rec)
	}
	for i := range rec.Facts {
		a, b := got.Facts[i], rec.Facts[i]
		if a.FQN != b.FQN || !reflect.DeepEqual(a.Supers, b.Supers) {
			t.Errorf("fact[%d] = %+v, want %+v", i, a, b)
		}
	}
}

func TestRecordSupersFromSpecializationEdges(t *testing.T) {
	idx := indexOf(t, "lib", "part def Car specializes Vehicle, Machine; part def Vehicle; part def Machine;")
	rec := recordOf("lib", idx)
	if rec == nil {
		t.Fatalf("expected a record")
	}
	got := supersByFQN(rec)["Car"]
	if want := []string{"Vehicle", "Machine"}; !slices.Equal(got, want) {
		t.Fatalf("Supers = %v, want %v", got, want)
	}
}

// Supers memoizes the semantic supertype edges, so it records exactly the edge
// kinds semantics.GeneralizationKind accepts — typing included, references excluded.
func TestRecordSupersCoversGeneralizationEdges(t *testing.T) {
	idx := indexOf(t, "lib", "part def Engine; part e : Engine subsets Engine; part def Chassis; part c ::> Chassis;")
	got := supersByFQN(recordOf("lib", idx))
	// Typing and subsetting name the same target here, recorded once.
	if want := []string{"Engine"}; !slices.Equal(got["e"], want) {
		t.Fatalf("Supers of e = %v, want %v", got["e"], want)
	}
	// `::>` is reference subsetting: it contributes members, not conformance.
	if len(got["c"]) != 0 {
		t.Fatalf("Supers of c = %v, want none (reference subsetting)", got["c"])
	}
}

// A result parameter implicitly redefines the nameless result of the behavior
// its owner specializes. That edge has no qualified name of its own, so the
// record reaches it by its owner's name and its member ordinal.
func TestRecordSupersReachNamelessTarget(t *testing.T) {
	idx := indexOf(t, "lib.kerml", `package P {
		datatype Boolean;
		abstract function Check { return : Boolean; }
		function Named specializes Check { return result : Boolean; }
		function Plain specializes Check { return result = true; }
	}`)
	got := supersByFQN(recordOf("lib.kerml", idx))
	if want := []string{"P::Check"}; !slices.Equal(got["P::Named"], want) {
		t.Fatalf("Supers of P::Named = %v, want %v", got["P::Named"], want)
	}
	for _, fqn := range []string{"P::Named::result", "P::Plain::result"} {
		want := []string{"P::Boolean", "P::Check#0"}
		if fqn == "P::Plain::result" {
			want = want[1:]
		}
		if !slices.Equal(got[fqn], want) {
			t.Errorf("Supers of %s = %v, want %v", fqn, got[fqn], want)
		}
	}
}
