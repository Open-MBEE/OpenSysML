package model

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Members of a document held as a record keep their directions, reversed through a conjugated
// typing, and their multiplicities, as those of a parsed document do.
func TestMemberSummaryReadsRecordedDirections(t *testing.T) {
	base := Input{Name: "base.sysml", Version: 1, Content: []byte(
		"package Hull { port def PD { in attribute a; out attribute b; inout attribute c; attribute d; }\n" +
			"requirement def Need { subject s : PD[2]; assume constraint ac[0..1] { true } require constraint rc[1..*] { true } part q[3]; } }")}
	user := func(extra string) Input {
		return Input{Name: "user.sysml", Version: 1, Content: []byte(
			"package Craft { port def Sub :> Hull::PD; part def Sat { port p : ~Hull::PD; } }" + extra)}
	}
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cold := NewWorkspace(WithRecordCache(cache))
	cold.OpenAll([]Input{base, user("")})
	cold.DiagnosticsAll([]string{"base.sysml", "user.sysml"})

	summary := func(ws *Workspace, fqn string) []string {
		t.Helper()
		found := ws.LookupQualified(fqn)
		if len(found) != 1 {
			t.Fatalf("%s names %d symbols", fqn, len(found))
		}
		var out []string
		for _, member := range ws.MemberSummaryOf(found[0], true) {
			if member.Origin != nil && ws.FQNOf(member.Origin) == "Hull::PD" {
				out = append(out, member.Direction.String()+" "+member.Name)
			}
		}
		return out
	}
	warm := NewWorkspace(WithRecordCache(cache))
	warm.OpenAll([]Input{base, user("")})
	warm.Update("user.sysml", user(" package Edited {}").Content, 2)
	if !warm.Recorded("base.sysml") || warm.Recorded("user.sysml") {
		t.Fatal("want base.sysml held as a record beside a parsed user.sysml")
	}
	for fqn, want := range map[string][]string{
		"Craft::Sub":    {"in a", "out b", "inout c", "none d"},
		"Craft::Sat::p": {"out a", "in b", "inout c", "none d"},
	} {
		if got := summary(cold, fqn); !reflect.DeepEqual(got, want) {
			t.Errorf("parsed: %s inherits %q, want %q", fqn, got, want)
		}
		if got := summary(warm, fqn); !reflect.DeepEqual(got, want) {
			t.Errorf("recorded: %s inherits %q, want %q", fqn, got, want)
		}
	}
	for _, ws := range []*Workspace{cold, warm} {
		var got []string
		for _, member := range ws.MemberSummaryOf(ws.LookupQualified("Hull::Need")[0], false) {
			got = append(got, member.Name+member.Multiplicity)
		}
		if want := []string{"s[2]", "ac[0..1]", "rc[1..*]", "q[3]"}; !reflect.DeepEqual(got, want) {
			t.Errorf("recorded %v: Hull::Need lists %q, want %q", ws == warm, got, want)
		}
	}
	if !warm.Recorded("base.sysml") {
		t.Fatal("summarizing hydrated base.sysml: the recorded case was not exercised")
	}
}
