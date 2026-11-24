package analysis

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/enginewire"
)

// hookedInput is a session's standard input that hands each request line to a hook, which
// plays the engine's part from inside the write.
type hookedInput struct{ hook func(line []byte) }

func (h hookedInput) Write(line []byte) (int, error) {
	h.hook(line)
	return len(line), nil
}

func (hookedInput) Close() error { return nil }

// hookedSession is a session with no process, whose requests go to engine.
func hookedSession(engine func(s *session, id string)) *session {
	s := newSession(EngineEntry{Name: "standin"}, DefaultOutputLimit, time.Minute)
	s.stdin = hookedInput{hook: func(line []byte) {
		var msg enginewire.Message
		if err := decodeOne(line, &msg); err != nil {
			panic(err)
		}
		engine(s, string(msg.ID))
	}}
	return s
}

const lifecycleAnswer = `{"claim":"none","strength":"not covered"}`

func answerLine(id string) []byte {
	return []byte(`{"jsonrpc":"2.0","id":` + id + `,"result":` + lifecycleAnswer + `}`)
}

// An answer the reader delivered before the session ended reaches its caller, however the
// caller's select falls: with the answer and the end both ready, the answer wins every time.
func TestSessionAnswerDeliveredBeforeTheEndIsReturned(t *testing.T) {
	for i := range 200 {
		s := hookedSession(func(s *session, id string) {
			if err := s.take(answerLine(id)); err != nil {
				t.Errorf("take: %v", err)
			}
			s.finish(&EngineExitedError{Engine: "standin", Started: true})
		})
		raw, err := runOnce(context.Background(), s, nil)
		if err != nil || string(raw) != lifecycleAnswer {
			t.Fatalf("iteration %d: got %s, %v; want the answer", i, raw, err)
		}
	}
}

// An answer that arrived by the time a round trip's context ended is returned rather than
// ending the session as a timeout.
func TestSessionAnswerDeliveredBeforeTheDeadlineIsReturned(t *testing.T) {
	for i := range 200 {
		ctx, cancel := context.WithCancel(context.Background())
		s := hookedSession(func(s *session, id string) {
			if err := s.take(answerLine(id)); err != nil {
				t.Errorf("take: %v", err)
			}
			cancel()
		})
		raw, err := s.call(ctx, enginewire.MethodCovers, enginewire.CoversParams{Question: questionA()}, nil)
		if err != nil || string(raw) != lifecycleAnswer || !s.alive() {
			t.Fatalf("iteration %d: got %s, %v, alive %v; want the answer on a live session", i, raw, err, s.alive())
		}
	}
}

// A second response to one request breaks the protocol at once, before the caller has read
// the first: the reader is never left blocked on a full reply.
func TestSessionSecondAnswerBreaksProtocolWithoutBlocking(t *testing.T) {
	second := make(chan error, 1)
	s := hookedSession(func(s *session, id string) {
		if err := s.take(answerLine(id)); err != nil {
			t.Errorf("take: %v", err)
		}
		go func() { second <- s.take(answerLine(id)) }()
		select {
		case err := <-second:
			second <- err
		case <-time.After(10 * time.Second):
			t.Error("the second answer blocked the reader")
		}
	})
	raw, err := runOnce(context.Background(), s, nil)
	if err != nil || string(raw) != lifecycleAnswer {
		t.Fatalf("got %s, %v; want the first answer", raw, err)
	}
	select {
	case err := <-second:
		if !errors.Is(err, ErrProtocol) {
			t.Fatalf("the second answer got %v; want a protocol error", err)
		}
	default:
		t.Fatal("the second answer was never taken")
	}
	if s.open != 0 {
		t.Fatalf("%d requests left open", s.open)
	}
}

// An engine that answers a run and exits at once has its answer returned, every time.
func TestSessionAnswerThenExitIsAnswered(t *testing.T) {
	t.Setenv(engineStandinMode, "answer-and-exit")
	for i := range 50 {
		s := standinSession(t, standinEntry(t), 5*time.Second)
		raw, err := runOnce(context.Background(), s, nil)
		if err != nil || !strings.Contains(string(raw), "not covered") {
			t.Fatalf("iteration %d: got %.80s, %v; want the answer", i, raw, err)
		}
	}
}

// An engine that answers a run twice has its first answer returned and its session ended as
// a protocol break, with the reader gone.
func TestSessionDuplicateAnswerEndsTheSession(t *testing.T) {
	t.Setenv(engineStandinMode, "duplicate-answer")
	for i := range 20 {
		s := standinSession(t, standinEntry(t), 5*time.Second)
		raw, err := runOnce(context.Background(), s, nil)
		if err != nil || !strings.Contains(string(raw), "twice") {
			t.Fatalf("iteration %d: got %s, %v; want the first answer", i, raw, err)
		}
		select {
		case <-s.drained:
		case <-time.After(10 * time.Second):
			t.Fatalf("iteration %d: the reader is still running after the second answer", i)
		}
		if err := s.failed(); !errors.Is(err, ErrProtocol) {
			t.Fatalf("iteration %d: the session ended with %v; want a protocol error", i, err)
		}
	}
}

// An engine that stops reading requests but keeps its output open fails the request it
// refused at once, as an exit carrying the write's error, not after the whole timeout.
func TestSessionRefusedWriteDoesNotWaitTheTimeout(t *testing.T) {
	t.Setenv(engineStandinMode, "stdin-closed")
	record := t.TempDir() + "/record"
	t.Setenv(engineStandinRecord, record)
	s := standinSession(t, standinEntry(t), time.Minute)
	for closed := false; !closed; time.Sleep(10 * time.Millisecond) {
		log, _ := os.ReadFile(record)
		closed = strings.Contains(string(log), "stdin closed")
	}
	began := time.Now()
	_, err := s.call(context.Background(), enginewire.MethodCovers, enginewire.CoversParams{Question: questionA()}, nil)
	var exited *EngineExitedError
	if !errors.As(err, &exited) || !errors.Is(err, syscall.EPIPE) {
		t.Fatalf("got %v; want the engine's exit with the broken pipe", err)
	}
	if took := time.Since(began); took > 30*time.Second {
		t.Fatalf("the refused write took %s under a one-minute timeout", took)
	}
}

// What the engine wrote before it exited is still readable after its exit was collected:
// collecting the exit does not close the pipe under the reader.
func TestSessionOutputSurvivesTheCollectedExit(t *testing.T) {
	t.Setenv(engineStandinMode, "describe-and-exit")
	entry := standinEntry(t)
	s := newSession(entry, DefaultOutputLimit, 5*time.Second)
	s.cmd = exec.Command(entry.Executable)
	s.cmd.Dir = entry.Dir
	stdout, err := s.launch()
	if err != nil {
		t.Fatal(err)
	}
	defer close(s.drained)
	if err := s.write(enginewire.Message{JSONRPC: enginewire.JSONRPC, ID: []byte("1"), Method: enginewire.MethodDescribe},
		enginewire.DescribeParams{Protocols: ProtocolVersions}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.waited:
	case <-time.After(10 * time.Second):
		t.Fatal("the engine did not exit")
	}
	line, err := readLine(bufio.NewReader(stdout), DefaultOutputLimit)
	if err != nil || !strings.Contains(string(line), `"name":"standin"`) {
		t.Fatalf("after the exit read %q, %v; want the description", line, err)
	}
}

// A start whose caller gives up ends with the caller's error well inside the timeout, and a
// caller that had given up already starts no process.
func TestSessionStartStopsWithItsCaller(t *testing.T) {
	t.Setenv(engineStandinMode, "silent-describe")
	t.Run("during the handshake", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(100*time.Millisecond, cancel)
		began := time.Now()
		_, err := startSession(ctx, standinEntry(t), DefaultOutputLimit, time.Minute)
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrEngineTimeout) {
			t.Fatalf("got %v; want the caller's cancellation", err)
		}
		if took := time.Since(began); took > 30*time.Second {
			t.Fatalf("the start took %s after a cancellation at 100ms", took)
		}
	})
	t.Run("before the start", func(t *testing.T) {
		record := t.TempDir() + "/record"
		t.Setenv(engineStandinRecord, record)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := startSession(ctx, standinEntry(t), DefaultOutputLimit, time.Minute); !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v; want the caller's cancellation", err)
		}
		if _, err := os.Stat(record); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("a process started for a caller that had given up (record: %v)", err)
		}
	})
}

// A start given up with its caller's context does not end the engine for the plan: the next
// request starts it, and a request waiting on another's start stops with its own context.
func TestEnginePoolSurvivesAnAbandonedStart(t *testing.T) {
	for _, concurrent := range []bool{true, false} {
		t.Run(map[bool]string{true: "concurrent", false: "nonconcurrent"}[concurrent], func(t *testing.T) {
			entry := EngineEntry{Name: "standin", Concurrent: concurrent}
			pool := newEnginePool(entry, DefaultOutputLimit, time.Minute)
			var starts atomic.Int32
			entered := make(chan struct{}, 8)
			pool.start = func(ctx context.Context, entry EngineEntry, limit int, timeout time.Duration) (*session, error) {
				if starts.Add(1) > 1 {
					return newSession(entry, limit, timeout), nil
				}
				entered <- struct{}{}
				<-ctx.Done()
				return nil, abandoned(ctx, entry.Name)
			}
			first, giveUp := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := pool.acquire(first); !errors.Is(err, context.Canceled) {
					t.Errorf("the abandoned start got %v", err)
				}
			}()
			<-entered
			if concurrent {
				waiting, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				if _, err := pool.acquire(waiting); !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("a request waiting on the start got %v; want its own deadline", err)
				}
			}
			giveUp()
			wg.Wait()
			s, err := pool.acquire(context.Background())
			if err != nil || s == nil {
				t.Fatalf("the request after an abandoned start got %v, %v; want a session", s, err)
			}
		})
	}
}
