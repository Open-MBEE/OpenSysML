package grpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// countingContext is done from its live-th look on, with the error it was
// given: a test cancels at an exact check instead of racing a timer.
type countingContext struct {
	context.Context
	live  int64
	looks atomic.Int64
	err   error
}

func cancelAfter(live int, err error) *countingContext {
	return &countingContext{Context: context.Background(), live: int64(live), err: err}
}

func (c *countingContext) Err() error {
	if c.looks.Add(1) > c.live {
		return c.err
	}
	return nil
}

// cachedModels is how many models the service's cache holds.
func cachedModels(srv *Service) int {
	srv.cache.mu.RLock()
	defer srv.cache.mu.RUnlock()
	return len(srv.cache.items)
}

// cancellationDocuments is a model of three documents that reference each other.
func cancellationDocuments(prefix string) []*pb.SourceDocument {
	var named []string
	for d := range 3 {
		text := fmt.Sprintf("package %s%d {\n", prefix, d)
		if d > 0 {
			text += fmt.Sprintf("\tprivate import %s%d::*;\n", prefix, d-1)
		}
		text += fmt.Sprintf("\tpart def D%d { attribute mass : ScalarValues::Real = %d; }\n\tpart p%d : D%d;\n}\n", d, d, d, d)
		named = append(named, fmt.Sprintf("%s%d.sysml", prefix, d), text)
	}
	return inlineDocuments(named...)
}

// cancelAtEveryCheck calls with a context done one look later each time until the call
// answers; it returns the answer and its looks, and fails on an earlier partial answer.
func cancelAtEveryCheck[Resp any](t *testing.T, cause error, call func(context.Context) (*Resp, error)) (*Resp, int) {
	t.Helper()
	for live := 0; live < 10_000; live++ {
		ctx := cancelAfter(live, cause)
		resp, err := call(ctx)
		if ctx.looks.Load() <= int64(live) {
			if err != nil {
				t.Fatalf("a call whose context stayed live failed: %v", err)
			}
			return resp, live
		}
		if !errors.Is(err, cause) {
			t.Fatalf("context done at look %d: error = %v, want %v", live+1, err, cause)
		}
		if resp != nil {
			t.Fatalf("context done at look %d: a response came back with the error", live+1)
		}
	}
	t.Fatal("the call never finished")
	return nil, 0
}

func TestParseSourcesStopsAtEveryCheck(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			srv := mustNewService(t, 8)
			defer srv.Close()
			docs := cancellationDocuments("P")
			req := &pb.ParseSourcesRequest{Documents: docs}
			resp, looks := cancelAtEveryCheck(t, cause, func(ctx context.Context) (*pb.ParseSourcesResponse, error) {
				before := cachedModels(srv)
				resp, err := srv.ParseSources(ctx, req)
				if err != nil && cachedModels(srv) != before {
					t.Fatalf("a cancelled parse cached a model")
				}
				return resp, err
			})
			// One look per document read, one per document parsed and one per document analyzed, at least.
			if looks < 3*len(docs) {
				t.Errorf("the parse looked at its context %d times, want at least %d", looks, 3*len(docs))
			}
			fresh := mustNewService(t, 8)
			defer fresh.Close()
			want, err := fresh.ParseSources(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(resp, want) {
				t.Errorf("the parse after the cancelled ones differs from an undisturbed one:\n got %v\nwant %v", resp, want)
			}
		})
	}
}

// The second and later parses of a document set go through its lineage's
// workspace, which a cancelled call leaves for the next.
func TestParseSourcesLineageSurvivesCancellation(t *testing.T) {
	srv := mustNewService(t, 8)
	defer srv.Close()
	fresh := mustNewService(t, 8)
	defer fresh.Close()
	edit := func(round int) *pb.ParseSourcesRequest {
		docs := cancellationDocuments("L")
		docs[1].Source = &pb.SourceDocument_Content{Content: docs[1].GetContent() + fmt.Sprintf("package Extra%d { part def E; }\n", round)}
		return &pb.ParseSourcesRequest{Documents: docs}
	}
	for round := range 3 {
		req := edit(round)
		resp, _ := cancelAtEveryCheck(t, context.Canceled, func(ctx context.Context) (*pb.ParseSourcesResponse, error) {
			return srv.ParseSources(ctx, req)
		})
		want, err := fresh.ParseSources(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(resp, want) {
			t.Errorf("round %d: the parse after the cancelled ones differs from an undisturbed one:\n got %v\nwant %v", round, resp, want)
		}
	}
}

func TestParseFileStopsWhenCancelled(t *testing.T) {
	srv := mustNewService(t, 8)
	defer srv.Close()
	req := &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: "package P { part def A; part a : A; }"}}
	_, looks := cancelAtEveryCheck(t, context.DeadlineExceeded, func(ctx context.Context) (*pb.ParseFileResponse, error) {
		before := cachedModels(srv)
		resp, err := srv.ParseFile(ctx, req)
		if err != nil && cachedModels(srv) != before {
			t.Fatalf("a cancelled parse cached a model")
		}
		return resp, err
	})
	if looks < 2 {
		t.Errorf("the parse looked at its context %d times, want at least 2", looks)
	}
}

func TestConvertModelStopsAtEveryCheck(t *testing.T) {
	for _, format := range []string{"ttl", "api-json"} {
		for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
			t.Run(format+"/"+cause.Error(), func(t *testing.T) {
				srv := mustNewService(t, 8)
				defer srv.Close()
				docs := cancellationDocuments("C")
				parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: docs})
				if err != nil {
					t.Fatal(err)
				}
				req := &pb.ConvertRequest{Source: &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash}, ToFormat: format, IncludeCheckDiagnostics: true}
				resp, looks := cancelAtEveryCheck(t, cause, func(ctx context.Context) (*pb.ConvertResponse, error) {
					return srv.Convert(ctx, req)
				})
				// One look before each document is written, at least.
				if looks < len(docs) {
					t.Errorf("the conversion looked at its context %d times, want at least %d", looks, len(docs))
				}
				if resp.Error != "" || resp.Content == "" {
					t.Fatalf("the conversion after the cancelled ones: error %q, %d bytes", resp.Error, len(resp.Content))
				}
				fresh := mustNewService(t, 8)
				defer fresh.Close()
				if _, err := fresh.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: docs}); err != nil {
					t.Fatal(err)
				}
				want, err := fresh.Convert(context.Background(), req)
				if err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(resp, want) {
					t.Errorf("the conversion after the cancelled ones differs from an undisturbed one")
				}
			})
		}
	}
}

func TestCancelledCallLeavesOtherModelsAlone(t *testing.T) {
	srv := mustNewService(t, 8)
	defer srv.Close()
	other := &pb.ParseSourcesRequest{Documents: cancellationDocuments("O")}
	want, err := srv.ParseSources(context.Background(), other)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			req := &pb.ParseSourcesRequest{Documents: cancellationDocuments(fmt.Sprintf("X%d_", i))}
			if resp, err := srv.ParseSources(cancelAfter(4+i, context.Canceled), req); !errors.Is(err, context.Canceled) || resp != nil {
				t.Errorf("cancelled parse %d: response %v, error %v", i, resp, err)
			}
		}()
		go func() {
			defer wg.Done()
			got, err := srv.ParseSources(context.Background(), other)
			if err != nil {
				t.Errorf("undisturbed parse %d: %v", i, err)
				return
			}
			if !proto.Equal(got, want) {
				t.Errorf("undisturbed parse %d differs from the one before the cancelled calls", i)
			}
		}()
	}
	wg.Wait()
}

func TestPreCancelledLongCallsDoNoWork(t *testing.T) {
	srv := mustNewService(t, 8)
	defer srv.Close()
	docs := cancellationDocuments("N")
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: docs})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if resp, err := srv.ParseSources(ctx, &pb.ParseSourcesRequest{Documents: cancellationDocuments("M")}); !errors.Is(err, context.Canceled) || resp != nil {
		t.Errorf("ParseSources: response %v, error %v", resp, err)
	}
	convert := &pb.ConvertRequest{Source: &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash}, ToFormat: "ttl"}
	if resp, err := srv.Convert(ctx, convert); !errors.Is(err, context.Canceled) || resp != nil {
		t.Errorf("Convert: response %v, error %v", resp, err)
	}
	cached, _ := srv.cache.Get(parsed.ModelHash)
	if cached.exportModel != nil {
		t.Error("a pre-cancelled Convert prepared the model for export")
	}
}

// A parse stopped while it analyzes writes no record of the model, so no later
// process reads part of a report; one stopped while a record answers is as whole.
func TestCancelledParseRecordsNoAnalysis(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			req := &pb.ParseSourcesRequest{Documents: inlineDocuments(recordedModels[2]...)}
			want, err := mustNewService(t, 8).ParseSources(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if len(want.Diagnostics) == 0 {
				t.Fatal("the model reports nothing, so a partial report would not show")
			}
			cache, dir := recordCacheIn(t)
			// A service per attempt: a second parse of the documents goes through their lineage, which records nothing.
			written, analyzing := cancelAtEveryCheck(t, cause, func(ctx context.Context) (*pb.ParseSourcesResponse, error) {
				resp, err := recordingService(t, cache).ParseSources(ctx, req)
				if n := len(recordFiles(t, dir)); err != nil && n != 0 {
					t.Fatalf("a cancelled parse wrote %d records", n)
				}
				return resp, err
			})
			if n := len(recordFiles(t, dir)); n != 1 {
				t.Fatalf("the finished parse wrote %d records, want 1", n)
			}
			read, reading := cancelAtEveryCheck(t, cause, func(ctx context.Context) (*pb.ParseSourcesResponse, error) {
				return recordingService(t, cache).ParseSources(ctx, req)
			})
			if reading >= analyzing {
				t.Errorf("reading the record looked at the context %d times, analyzing %d: the record was not read", reading, analyzing)
			}
			for label, got := range map[string]*pb.ParseSourcesResponse{"writing the record": written, "reading the record": read} {
				if !proto.Equal(got, want) {
					t.Errorf("%s after cancelled parses differs from an undisturbed parse:\n got %v\nwant %v", label, got, want)
				}
			}
		})
	}
}
