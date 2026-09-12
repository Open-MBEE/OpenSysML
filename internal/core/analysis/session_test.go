package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis/enginewire"
	"github.com/Open-MBEE/OpenSysML/internal/testutil/gobuild"
)

// The stand-in engine's variables and modes, from testdata/enginestandin.
const (
	engineStandinDescribe = "ENGINE_STANDIN_DESCRIBE"
	engineStandinCovers   = "ENGINE_STANDIN_COVERS"
	engineStandinResult   = "ENGINE_STANDIN_RESULT"
	engineStandinMode     = "ENGINE_STANDIN_MODE"
	engineStandinRecord   = "ENGINE_STANDIN_RECORD"
	engineStandinDelay    = "ENGINE_STANDIN_DELAY"
	engineStandinProgress = "ENGINE_STANDIN_PROGRESS"
)

var (
	engineStandinOnce sync.Once
	engineStandinPath string
	engineStandinErr  error
)

// engineStandin builds the stand-in engine once per test binary and returns its path.
func engineStandin(t *testing.T) string {
	t.Helper()
	engineStandinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "enginestandin")
		if err != nil {
			engineStandinErr = err
			return
		}
		engineStandinPath = filepath.Join(dir, "enginestandin")
		build := exec.Command("go", gobuild.Args(engineStandinPath)...)
		build.Dir = filepath.Join("testdata", "enginestandin")
		if out, err := build.CombinedOutput(); err != nil {
			engineStandinErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if engineStandinErr != nil {
		t.Fatalf("building the stand-in engine: %v", engineStandinErr)
	}
	return engineStandinPath
}

// standinEntry is the manifest entry the stand-in answers describe for by default.
func standinEntry(t *testing.T) EngineEntry {
	t.Helper()
	program := engineStandin(t)
	return EngineEntry{Kind: KindEngine, Name: "standin", Version: "1.0.0", Command: []string{program},
		Executable: program, Dir: filepath.Dir(program), Transport: TransportStdio, Protocol: 1,
		Answers: []Kind{Holds}, Model: []ModelForm{FormSources}, Witness: WitnessSchedule, Authority: Bounded,
		Concurrent: true}
}

// standinSession starts a session with the stand-in under a short timeout.
func standinSession(t *testing.T, entry EngineEntry, timeout time.Duration) *session {
	t.Helper()
	s, err := startSession(entry, DefaultOutputLimit, timeout)
	if err != nil {
		t.Fatalf("startSession: %v", err)
	}
	t.Cleanup(s.close)
	return s
}

// runOnce sends one run to the session and returns its answer.
func runOnce(ctx context.Context, s *session, sink *progressSink) (json.RawMessage, error) {
	return s.call(ctx, enginewire.MethodRun, enginewire.RunParams{Question: enginewire.Question{Kind: "holds", Subject: "A"}}, sink)
}

// The handshake takes the description and the session answers covers and run by id.
func TestSessionHandshakeCoversAndRun(t *testing.T) {
	t.Setenv(engineStandinResult, `{"claim":"holds","strength":"bounded","bounds":[{"name":"depth","limit":10}]}`)
	s := standinSession(t, standinEntry(t), 5*time.Second)
	if s.described.Name != "standin" || s.described.Protocol != 1 {
		t.Fatalf("described %+v", s.described)
	}
	raw, err := s.call(context.Background(), enginewire.MethodCovers, enginewire.CoversParams{}, nil)
	if err != nil {
		t.Fatalf("covers: %v", err)
	}
	var covers enginewire.CoversResult
	if err := json.Unmarshal(raw, &covers); err != nil || !covers.Covers {
		t.Fatalf("covers answered %s (%v)", raw, err)
	}
	raw, err = runOnce(context.Background(), s, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var result enginewire.Result
	if err := json.Unmarshal(raw, &result); err != nil || result.Claim != "holds" {
		t.Fatalf("run answered %s (%v)", raw, err)
	}
}

// Every field of describe is checked against the manifest entry.
func TestSessionHandshakeMismatchIsTypedPerField(t *testing.T) {
	cases := []struct {
		field    string
		describe string
	}{
		{"name", `{"name":"other","version":"1.0.0","protocol":1,"answers":["holds"]}`},
		{"version", `{"name":"standin","version":"2.0.0","protocol":1,"answers":["holds"]}`},
		{"protocol", `{"name":"standin","version":"1.0.0","protocol":2,"answers":["holds"]}`},
		{"answers", `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds","outcomes"]}`},
		{"witness", `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds"],"witness":"assignment"}`},
		{"model", `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds"],"model":["sources","graphs:1"]}`},
		{"authority", `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds"],"authority":"proved"}`},
		{"concurrent", `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds"],"concurrent":false}`},
	}
	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			t.Setenv(engineStandinDescribe, c.describe)
			_, err := startSession(standinEntry(t), DefaultOutputLimit, 5*time.Second)
			var mismatch *HandshakeError
			if !errors.As(err, &mismatch) || mismatch.Field != c.field {
				t.Fatalf("got %v, want a HandshakeError on %s", err, c.field)
			}
			if !errors.Is(err, ErrHandshake) {
				t.Fatalf("%v is not ErrHandshake", err)
			}
		})
	}
}

// A process that does not start, or exits before describing itself, is typed with its stderr.
func TestSessionStartupFailuresAreTyped(t *testing.T) {
	t.Run("exit at start", func(t *testing.T) {
		t.Setenv(engineStandinMode, "exit-at-start")
		_, err := startSession(standinEntry(t), DefaultOutputLimit, 5*time.Second)
		var exited *EngineExitedError
		if !errors.As(err, &exited) || exited.Started || !strings.Contains(exited.Stderr, "cannot load formalism") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("silent describe", func(t *testing.T) {
		t.Setenv(engineStandinMode, "silent-describe")
		_, err := startSession(standinEntry(t), DefaultOutputLimit, 200*time.Millisecond)
		var timeout *EngineTimeoutError
		if !errors.As(err, &timeout) || timeout.Method != enginewire.MethodDescribe {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("absent program", func(t *testing.T) {
		entry := standinEntry(t)
		entry.Executable = filepath.Join(t.TempDir(), "missing")
		_, err := startSession(entry, DefaultOutputLimit, time.Second)
		if !errors.Is(err, ErrProcessAbsent) {
			t.Fatalf("got %v", err)
		}
	})
}

// The three error codes come back as the engine's fault, typed with the code.
func TestSessionErrorCodesAreTyped(t *testing.T) {
	for _, code := range []string{enginewire.CodeUnsupported, enginewire.CodeBudget, enginewire.CodeInternal} {
		t.Run(code, func(t *testing.T) {
			t.Setenv(engineStandinMode, "error-"+code)
			s := standinSession(t, standinEntry(t), 5*time.Second)
			_, err := runOnce(context.Background(), s, nil)
			var fault *EngineFaultError
			if !errors.As(err, &fault) || fault.Code != code || !errors.Is(err, ErrEngineFault) {
				t.Fatalf("got %v", err)
			}
			if !s.alive() {
				t.Fatal("an error answer ends the session; it must not")
			}
		})
	}
}

// Every break of the protocol ends the session with a ProtocolError, and the next request
// is refused as ended.
func TestSessionProtocolBreaksEndTheSession(t *testing.T) {
	cases := []struct {
		mode   string
		detail string
	}{
		{"not-json", "not a JSON-RPC message"},
		{"wrong-jsonrpc", `jsonrpc "1.0"`},
		{"no-id", "no id"},
		{"null-id", "no id"},
		{"string-id", "not the number of a request"},
		{"unknown-id", "no open request"},
		{"result-and-error", "neither result nor error, or both"},
		{"neither", "neither result nor error, or both"},
		{"error-no-code", "no code"},
		{"engine-request", "takes no requests"},
		{"unknown-notification", "takes progress alone"},
		{"progress-unknown-run", "no open request"},
	}
	for _, c := range cases {
		t.Run(c.mode, func(t *testing.T) {
			t.Setenv(engineStandinMode, c.mode)
			s := standinSession(t, standinEntry(t), 5*time.Second)
			_, err := runOnce(context.Background(), s, nil)
			if !errors.Is(err, ErrProtocol) || !strings.Contains(err.Error(), c.detail) {
				t.Fatalf("got %v, want a protocol break naming %q", err, c.detail)
			}
			if s.alive() {
				t.Fatal("the session is still alive after a protocol break")
			}
			_, err = runOnce(context.Background(), s, nil)
			if !errors.Is(err, ErrProtocol) {
				t.Fatalf("a request after the break got %v", err)
			}
		})
	}
}

// A line over the output bound is a protocol break naming the bound.
func TestSessionLineOverTheBoundEndsTheSession(t *testing.T) {
	t.Setenv(engineStandinMode, "overflow")
	entry := standinEntry(t)
	s, err := startSession(entry, 64<<10, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.close)
	_, err = runOnce(context.Background(), s, nil)
	if !errors.Is(err, ErrProtocol) || !strings.Contains(err.Error(), OutputLimitEnv) {
		t.Fatalf("got %v", err)
	}
}

// A process that exits during a run is typed with its exit and stderr.
func TestSessionExitDuringRunIsTyped(t *testing.T) {
	t.Setenv(engineStandinMode, "exit-during-run")
	s := standinSession(t, standinEntry(t), 5*time.Second)
	_, err := runOnce(context.Background(), s, nil)
	var exited *EngineExitedError
	if !errors.As(err, &exited) || !exited.Started || !strings.Contains(exited.Stderr, "segmentation fault") {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "exit 11") {
		t.Fatalf("%v does not name the exit code", err)
	}
}

// Cancel is sent when the run's context ends; an engine that answers it is heard.
func TestSessionCancelIsAnswered(t *testing.T) {
	t.Setenv(engineStandinMode, "cancel-answers")
	s := standinSession(t, standinEntry(t), 5*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	raw, err := runOnce(ctx, s, nil)
	if err != nil {
		t.Fatalf("got %v, want the cancelled run's answer", err)
	}
	var result enginewire.Result
	if err := json.Unmarshal(raw, &result); err != nil || result.Reason != "cancelled" {
		t.Fatalf("answered %s", raw)
	}
	if !s.alive() {
		t.Fatal("the session ended after an answered cancel")
	}
}

// An engine that ignores cancel is ended at the grace deadline, with its last progress.
func TestSessionIgnoredCancelEndsTheProcess(t *testing.T) {
	t.Setenv(engineStandinMode, "cancel-ignored")
	t.Setenv(engineStandinProgress, "3")
	s := standinSession(t, standinEntry(t), 300*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	sink := newProgressSink("standin", nil)
	started := time.Now()
	_, err := runOnce(ctx, s, sink)
	if !errors.Is(err, ErrEngineTimeout) || !errors.Is(err, ErrUnansweredRun) {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "runs 3") {
		t.Fatalf("%v does not carry the last progress", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("the process was not ended at the grace deadline")
	}
	select {
	case <-s.waited:
	case <-time.After(5 * time.Second):
		t.Fatal("the process is still running")
	}
}

// A thousand progress notifications in a second reach the reporter a handful of times, the
// latest last.
func TestSessionProgressIsCoalesced(t *testing.T) {
	t.Setenv(engineStandinProgress, "1000")
	t.Setenv(engineStandinDelay, "600ms")
	s := standinSession(t, standinEntry(t), 5*time.Second)
	var reports []ProgressReport
	var mu sync.Mutex
	sink := newProgressSink("standin", func(r ProgressReport) {
		mu.Lock()
		defer mu.Unlock()
		reports = append(reports, r)
	})
	if _, err := runOnce(context.Background(), s, sink); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(reports) == 0 || len(reports) > 8 {
		t.Fatalf("%d reports of a thousand notifications; want a handful", len(reports))
	}
	if last := reports[len(reports)-1]; last.Progress.Runs != 1000 || last.Engine != "standin" {
		t.Fatalf("the last report is %+v, not the latest progress", last)
	}
	if got := ProgressText(reports[len(reports)-1].Progress); !strings.HasPrefix(got, "runs 1000") {
		t.Fatalf("ProgressText %q", got)
	}
}

// Several runs stay open on one concurrent process, each answered by its own id.
func TestSessionMatchesConcurrentAnswersByID(t *testing.T) {
	t.Setenv(engineStandinDelay, "50ms")
	record := filepath.Join(t.TempDir(), "record")
	t.Setenv(engineStandinRecord, record)
	s := standinSession(t, standinEntry(t), 5*time.Second)
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = runOnce(context.Background(), s, nil)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if got := maxOpen(t, record); got < 2 {
		t.Fatalf("at most %d runs were open at once; want several on one process", got)
	}
}

// maxOpen reads the most runs the stand-in had open at once from its record.
func maxOpen(t *testing.T, record string) int {
	t.Helper()
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	most := 0
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[1] == "open" {
			n, _ := strconv.Atoi(fields[2])
			most = max(most, n)
		}
	}
	return most
}

// processes counts the stand-in processes the record saw start.
func processes(t *testing.T, record string) int {
	t.Helper()
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), " start")
}

// A pool of a nonconcurrent entry never gives one process two open requests, and ends every
// process with the plan; a concurrent one shares a single process.
func TestEnginePoolHonorsConcurrent(t *testing.T) {
	for _, concurrent := range []bool{true, false} {
		t.Run(strconv.FormatBool(concurrent), func(t *testing.T) {
			t.Setenv(engineStandinDelay, "50ms")
			record := filepath.Join(t.TempDir(), "record")
			t.Setenv(engineStandinRecord, record)
			entry := standinEntry(t)
			entry.Concurrent = concurrent
			if !concurrent {
				t.Setenv(engineStandinDescribe, `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds"],"concurrent":false}`)
			}
			pool := newEnginePool(entry, DefaultOutputLimit, 5*time.Second)
			var wg sync.WaitGroup
			var failures atomic.Int32
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					s, err := pool.acquire()
					if err != nil {
						failures.Add(1)
						return
					}
					defer pool.put(s)
					if _, err := runOnce(context.Background(), s, nil); err != nil {
						failures.Add(1)
					}
				}()
			}
			wg.Wait()
			if failures.Load() != 0 {
				t.Fatalf("%d runs failed", failures.Load())
			}
			started, most := processes(t, record), maxOpen(t, record)
			switch {
			case concurrent && started != 1:
				t.Fatalf("a concurrent entry started %d processes; want one", started)
			case !concurrent && most != 1:
				t.Fatalf("a nonconcurrent process held %d requests open at once", most)
			case !concurrent && started < 2:
				t.Fatalf("a nonconcurrent entry started %d processes for 8 concurrent requests", started)
			}
			pool.close()
			if _, err := pool.acquire(); !errors.Is(err, ErrSessionEnded) {
				t.Fatalf("acquire after close got %v", err)
			}
			data, _ := os.ReadFile(record)
			if strings.Count(string(data), " exit") != started {
				t.Fatalf("%d of %d processes exited cleanly at close:\n%s", strings.Count(string(data), " exit"), started, data)
			}
		})
	}
}

// A pool refuses every request after one of its processes ends by fault.
func TestEnginePoolEndsTheEngineForThePlanAtTheFirstFault(t *testing.T) {
	t.Setenv(engineStandinMode, "not-json")
	pool := newEnginePool(standinEntry(t), DefaultOutputLimit, 5*time.Second)
	s, err := pool.acquire()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runOnce(context.Background(), s, nil); !errors.Is(err, ErrProtocol) {
		t.Fatalf("got %v", err)
	}
	pool.put(s)
	if _, err := pool.acquire(); !errors.Is(err, ErrSessionEnded) || !errors.Is(err, ErrProtocol) {
		t.Fatalf("acquire after the fault got %v", err)
	}
	pool.close()
}
