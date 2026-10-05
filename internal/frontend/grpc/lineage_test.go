package grpc

import (
	"context"
	"fmt"
	"sync"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"google.golang.org/protobuf/proto"
)

// A sequence of document sets, each the next state of one client's model: the
// library renames what the application uses and renames it back, the
// application gains a reference and loses it, and a document stops parsing and
// parses again.
var lineageSteps = [][]string{
	{"lib.sysml", sourcesLibrary, "top.sysml", sourcesTop},
	{"lib.sysml", sourcesLibrary, "top.sysml", sourcesTop}, // the same again: cached
	{"lib.sysml", "package Lib {\n\tpart def Motor;\n}\n", "top.sysml", sourcesTop},
	{"lib.sysml", sourcesLibrary, "top.sysml", sourcesTop},
	{"lib.sysml", sourcesLibrary, "top.sysml", "package Top {\n\tprivate import Lib::*;\n\tpart def Car {\n\t\tpart motor : Engine;\n\t\tpart spare : Wheel;\n\t}\n}\n"},
	{"lib.sysml", "package Lib {\n\tpart def Engine {\n", "top.sysml", sourcesTop},
	{"lib.sysml", sourcesLibrary, "top.sysml", sourcesTop},
	{"lib.sysml", sourcesLibrary + "// a comment\n", "top.sysml", sourcesTop},
}

// Incremental equals fresh: every step answered by the service that parsed the
// steps before it has the diagnostics, and converts to the api-json, that a
// service seeing the documents for the first time gives them.
func TestParseSourcesFromALineageEqualsAFreshParse(t *testing.T) {
	for _, strict := range []bool{false, true} {
		incremental := mustNewService(t, 10)
		for step, named := range lineageSteps {
			request := &pb.ParseSourcesRequest{Documents: inlineDocuments(named...), StrictConformance: strict}
			got, err := incremental.ParseSources(context.Background(), request)
			if err != nil {
				t.Fatalf("strict=%v step %d: ParseSources: %v", strict, step, err)
			}
			fresh := mustNewService(t, 10)
			want, err := fresh.ParseSources(context.Background(), request)
			if err != nil {
				t.Fatalf("strict=%v step %d: fresh ParseSources: %v", strict, step, err)
			}
			if got.ModelHash != want.ModelHash {
				t.Errorf("strict=%v step %d: model hash %s, fresh %s", strict, step, got.ModelHash, want.ModelHash)
			}
			if len(got.Diagnostics) != len(want.Diagnostics) {
				t.Fatalf("strict=%v step %d: %d diagnostics, fresh %d:\n%v\n%v", strict, step,
					len(got.Diagnostics), len(want.Diagnostics), got.Diagnostics, want.Diagnostics)
			}
			for i := range got.Diagnostics {
				if !proto.Equal(got.Diagnostics[i], want.Diagnostics[i]) {
					t.Errorf("strict=%v step %d: diagnostic %d is %v, fresh %v", strict, step, i, got.Diagnostics[i], want.Diagnostics[i])
				}
			}
			convert := func(srv *Service, hash string) string {
				resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
					Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}, ToFormat: "api-json",
				})
				if err != nil {
					t.Fatalf("strict=%v step %d: Convert: %v", strict, step, err)
				}
				return resp.GetContent() + resp.GetError()
			}
			if a, b := convert(incremental, got.ModelHash), convert(fresh, want.ModelHash); a != b {
				t.Errorf("strict=%v step %d: api-json differs from a fresh conversion", strict, step)
			}
			fresh.Close()
		}
		incremental.Close()
	}
}

// The first parse of a document set builds no workspace: a one-off parse pays
// for nothing it will not reuse. The second builds one, and later ones reuse it.
func TestParseSourcesBuildsALineageOnlyForASetParsedAgain(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	parse := func(named ...string) {
		t.Helper()
		if _, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: inlineDocuments(named...)}); err != nil {
			t.Fatalf("ParseSources: %v", err)
		}
	}
	held := func() *lineage {
		t.Helper()
		key := lineageKey([]sourceInput{{name: "lib.sysml"}, {name: "top.sysml"}}, 0)
		el, ok := srv.lineages.byKey[key]
		if !ok {
			return nil
		}
		return el.Value.(*lineageEntry).l
	}

	parse("lib.sysml", sourcesLibrary, "top.sysml", sourcesTop)
	if l := held(); l == nil || l.ws != nil {
		t.Fatalf("after one parse the set is noted and holds no workspace: %+v", l)
	}
	parse("lib.sysml", sourcesLibrary+"// edited\n", "top.sysml", sourcesTop)
	l := held()
	if l == nil || l.ws == nil {
		t.Fatal("a set parsed again holds a workspace")
	}
	ws := l.ws
	parse("lib.sysml", sourcesLibrary+"// edited again\n", "top.sysml", sourcesTop)
	if held().ws != ws {
		t.Error("a later parse of the set builds another workspace instead of updating its own")
	}
}

// The service keeps a few lineages at most, dropping the least recently used.
func TestLineagesAreBounded(t *testing.T) {
	ls := newLineages(2)
	for _, key := range []string{"a", "b", "a", "c"} {
		ls.get(key, func() *lineage { return &lineage{held: map[string]string{}} })
	}
	if _, ok := ls.byKey["b"]; ok {
		t.Error("the least recently used lineage was kept")
	}
	for _, key := range []string{"a", "c"} {
		if _, ok := ls.byKey[key]; !ok {
			t.Errorf("lineage %q was dropped", key)
		}
	}
}

// Requests for states of one document set that arrive at once each get the
// model of their own documents: the lineage serves them one at a time.
func TestParseSourcesFromALineageUnderConcurrentRequests(t *testing.T) {
	srv := mustNewService(t, 32)
	defer srv.Close()
	want := make([]string, len(lineageSteps))
	for step, named := range lineageSteps {
		fresh := mustNewService(t, 10)
		resp, err := fresh.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: inlineDocuments(named...)})
		if err != nil {
			t.Fatal(err)
		}
		want[step] = fmt.Sprint(resp.Diagnostics)
		fresh.Close()
	}
	var wg sync.WaitGroup
	for round := 0; round < 4; round++ {
		for step, named := range lineageSteps {
			wg.Add(1)
			go func(step int, named []string) {
				defer wg.Done()
				resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: inlineDocuments(named...)})
				if err != nil {
					t.Error(err)
					return
				}
				if got := fmt.Sprint(resp.Diagnostics); got != want[step] {
					t.Errorf("step %d: diagnostics %s, fresh %s", step, got, want[step])
				}
			}(step, named)
		}
	}
	wg.Wait()
}
