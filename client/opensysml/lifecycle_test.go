package opensysml_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// bothImplementations runs a check through the in-process client and a remote
// one, since the two are held to the same lifetime and context semantics.
func bothImplementations(t *testing.T, check func(*testing.T, opensysml.Client)) {
	t.Helper()
	t.Run("in process", func(t *testing.T) {
		check(t, newClient(t))
	})
	t.Run("remote", func(t *testing.T) {
		check(t, dialClient(t, startService(t)))
	})
}

func TestACancelledContextRefusesTheCall(t *testing.T) {
	bothImplementations(t, func(t *testing.T, client opensysml.Client) {
		model := parseVehicle(t, client)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := client.Evaluate(ctx, model, "2 + 2"); !errors.Is(err, opensysml.CodeCanceled) {
			t.Errorf("Evaluate err = %v, want CodeCanceled", err)
		}
		if _, err := client.ParseSource(ctx, vehicleSource); !errors.Is(err, opensysml.CodeCanceled) {
			t.Errorf("ParseSource err = %v, want CodeCanceled", err)
		}
		documents := []opensysml.Document{opensysml.Source("v.sysml", vehicleSource)}
		if _, err := client.ParseDocuments(ctx, documents); !errors.Is(err, opensysml.CodeCanceled) {
			t.Errorf("ParseDocuments err = %v, want CodeCanceled", err)
		}
		if _, err := client.LookupSymbol(ctx, model, "Demo::Vehicle"); !errors.Is(err, opensysml.CodeCanceled) {
			t.Errorf("LookupSymbol err = %v, want CodeCanceled", err)
		}
		if _, err := client.Instantiate(ctx, model, "Demo::Vehicle"); !errors.Is(err, opensysml.CodeCanceled) {
			t.Errorf("Instantiate err = %v, want CodeCanceled", err)
		}
		if _, err := client.ServerInfo(ctx); !errors.Is(err, opensysml.CodeCanceled) {
			t.Errorf("ServerInfo err = %v, want CodeCanceled", err)
		}
	})
}

// doneAfter is done, with err, from its live-th look on.
type doneAfter struct {
	context.Context
	live  int64
	looks atomic.Int64
	err   error
}

func (c *doneAfter) Err() error {
	if c.looks.Add(1) > c.live {
		return c.err
	}
	return nil
}

// The boundary looks at the context once before the engine and once after; a
// parse that takes more looks than those to finish is one the engine can stop.
func TestAContextDoneMidParseStopsTheEngine(t *testing.T) {
	client := newClient(t)
	for i, c := range []struct {
		cause error
		code  opensysml.Code
	}{{context.Canceled, opensysml.CodeCanceled}, {context.DeadlineExceeded, opensysml.CodeDeadlineExceeded}} {
		cause, code := c.cause, c.code
		// Each cause parses a model of its own, so neither is answered from the cache.
		documents := []opensysml.Document{
			opensysml.Source("a.sysml", fmt.Sprintf("package A { part def Vehicle%d; }", i)),
			opensysml.Source("b.sysml", fmt.Sprintf("package B { private import A::*; part car : Vehicle%d; }", i)),
		}
		for live := int64(0); ; live++ {
			ctx := &doneAfter{Context: context.Background(), live: live, err: cause}
			model, err := client.ParseDocuments(ctx, documents)
			if ctx.looks.Load() <= live {
				if err != nil {
					t.Fatalf("a parse whose context stayed live failed: %v", err)
				}
				if live <= 2+int64(len(documents)) {
					t.Errorf("%v: the parse took %d looks at its context, want more than the boundary's two and one per document", cause, live)
				}
				break
			}
			if !errors.Is(err, code) {
				t.Fatalf("%v at look %d: err = %v, want %v", cause, live+1, err, code)
			}
			if model != nil {
				t.Fatalf("%v at look %d: a model came back with the error", cause, live+1)
			}
		}
	}
}

func TestAnExpiredDeadlineIsDeadlineExceeded(t *testing.T) {
	bothImplementations(t, func(t *testing.T, client opensysml.Client) {
		model := parseVehicle(t, client)
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		if _, err := client.Evaluate(ctx, model, "2 + 2"); !errors.Is(err, opensysml.CodeDeadlineExceeded) {
			t.Errorf("err = %v, want CodeDeadlineExceeded", err)
		}
	})
}

func TestAClosedClientAnswersNoFurtherCalls(t *testing.T) {
	bothImplementations(t, func(t *testing.T, client opensysml.Client) {
		model := parseVehicle(t, client)
		if err := client.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		for name, call := range map[string]func() error{
			"ServerInfo":   func() error { _, err := client.ServerInfo(context.Background()); return err },
			"ParseSource":  func() error { _, err := client.ParseSource(context.Background(), vehicleSource); return err },
			"ParseFile":    func() error { _, err := client.ParseFile(context.Background(), "vehicle.sysml"); return err },
			"Diagnostics":  func() error { _, err := client.Diagnostics(context.Background(), model); return err },
			"LookupSymbol": func() error { _, err := client.LookupSymbol(context.Background(), model, "Demo::Vehicle"); return err },
			"Evaluate":     func() error { _, err := client.Evaluate(context.Background(), model, "2 + 2"); return err },
			"Instantiate":  func() error { _, err := client.Instantiate(context.Background(), model, "Demo::Vehicle"); return err },
			"ParseFiles": func() error {
				_, err := client.ParseFiles(context.Background(), []string{"vehicle.sysml"})
				return err
			},
			"ParseDocuments": func() error {
				_, err := client.ParseDocuments(context.Background(), []opensysml.Document{opensysml.Source("v.sysml", vehicleSource)})
				return err
			},
		} {
			if err := call(); !errors.Is(err, opensysml.CodeUnavailable) {
				t.Errorf("%s after Close: err = %v, want CodeUnavailable", name, err)
			}
		}
		if err := client.Close(); err != nil {
			t.Errorf("second Close: %v", err)
		}
	})
}

func TestCallsAreSafeFromManyGoroutines(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			model, err := client.ParseSource(ctx, fmt.Sprintf("package P%d { part def A { attribute x = %d; } }", i, i))
			if err != nil {
				t.Errorf("ParseSource: %v", err)
				return
			}
			value, err := client.Evaluate(ctx, model, "x", opensysml.WithContextSymbol(fmt.Sprintf("P%d::A", i)))
			if err != nil {
				t.Errorf("Evaluate: %v", err)
				return
			}
			if got, ok := value.(opensysml.Int); !ok || int(got) != i {
				t.Errorf("value = %#v, want Int(%d)", value, i)
			}
		}()
	}
	wg.Wait()
}

func TestServerInfoReportsTheEngineVersion(t *testing.T) {
	client := newClient(t)
	info, err := client.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	// The test binary's own module is OpenSysML, so the version is the main
	// module's; what must never happen is an empty one.
	if info.Version == "" {
		t.Error("ServerInfo reported no version")
	}
}
