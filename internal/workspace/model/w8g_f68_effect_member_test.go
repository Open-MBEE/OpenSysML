package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// effectMemberChainModel reaches through a transition's effect action into the
// message that action sends, the chain the corpus writes as
// `serverBehavior.delivering.effect.sentMessage`.
const effectMemberChainModel = `package F68 {
	item def Msg;
	action def Send { in item m : Msg; }
	state def Behavior {
		state idle;
		state busy;
		transition delivering first idle do send Send() to busy then busy;
	}
	part server {
		exhibit state serverBehavior : Behavior;
		attribute a = serverBehavior.delivering.effect.sentMessage;
	}
}`

// F101: a transition's effect action is the `effect` its own scope names, so a
// chain through it reads that action rather than the library's abstract feature.
func TestW8GEffectMemberOfACachedLibrarySymbol(t *testing.T) {
	ws := NewWorkspace()
	uri := "file:///w8g_f68.sysml"
	ws.Open(uri, []byte(effectMemberChainModel), 1)
	defer ws.Close(uri)

	if unresolved := lookupFailures(diagnosticMessages(ws, uri)); len(unresolved) != 0 {
		t.Fatalf("expected the effect member chain to resolve, got %v", unresolved)
	}
}

// The chain reaches members of library symbols (`Actions::TransitionAction`,
// `Actions::SendAction`), so it resolves the same whether the load parsed the
// library or restored its records from the cache.
func TestW8GEffectMemberChainIsTheSameColdAndWarm(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	uri := "file:///w8g_f68.sysml"

	cold, coldHits := stdlibLibraryIndex(t)
	if coldHits != 0 {
		t.Fatalf("the first load restored %d records, so it is not a cold load", coldHits)
	}
	warm, warmHits := stdlibLibraryIndex(t)
	if warmHits == 0 {
		t.Fatal("the second load restored no record, so it is not a warm load")
	}

	for _, tc := range []struct {
		state string
		base  *symbols.Index
	}{{"cold", cold}, {"warm", warm}} {
		ws := NewWorkspaceWithIndex(symbols.NewOverlay(tc.base))
		ws.Open(uri, []byte(effectMemberChainModel), 1)
		if got := lookupFailures(diagnosticMessages(ws, uri)); len(got) != 0 {
			t.Errorf("the %s library reports %v, want the chain resolved", tc.state, got)
		}
		ws.Close(uri)
	}
}

// stdlibLibraryIndex loads the standard library into a frozen index of its own,
// reporting how many of its documents the cache supplied the facts of.
func stdlibLibraryIndex(t *testing.T) (*symbols.Index, int) {
	t.Helper()
	cache, err := libs.NewCache()
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	idx := symbols.NewIndex()
	loader := libs.NewLoader(libs.DefaultSource(), cache)
	if err := loader.LoadAll(idx); err != nil {
		t.Fatalf("load the standard library: %v", err)
	}
	idx.Freeze()
	return idx, loader.Hits()
}

// diagnosticMessages is the text of every diagnostic ws reports for uri.
func diagnosticMessages(ws *Workspace, uri string) []string {
	var out []string
	for _, d := range ws.Diagnostics(uri) {
		out = append(out, d.Message)
	}
	return out
}

// lookupFailures is the member-lookup failures among messages, the class a
// scope-less chain segment produces.
func lookupFailures(messages []string) []string {
	var out []string
	for _, msg := range messages {
		if strings.Contains(msg, "no scope for member lookup") ||
			strings.Contains(msg, "no members in") ||
			strings.Contains(msg, "unresolved member") {
			out = append(out, msg)
		}
	}
	return out
}
