// Command enginestandin stands in for an external engine in the tests of the engine
// protocol: it speaks JSON-RPC lines on standard input and output, describes itself as
// ENGINE_STANDIN_DESCRIBE says, answers covers with ENGINE_STANDIN_COVERS and run with
// ENGINE_STANDIN_RESULT, and misbehaves as ENGINE_STANDIN_MODE says. It is built by the
// tests that run it and lives under testdata, out of every build.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis/enginewire"
)

// The stand-in's variables.
const (
	// DescribeEnv is the description JSON the stand-in answers describe with.
	DescribeEnv = "ENGINE_STANDIN_DESCRIBE"
	// CoversEnv is the covers result JSON; unset answers covers true.
	CoversEnv = "ENGINE_STANDIN_COVERS"
	// ResultEnv is the result JSON every run answers with; unset answers a `none` result.
	ResultEnv = "ENGINE_STANDIN_RESULT"
	// ModeEnv selects a misbehavior; the empty mode answers every request.
	ModeEnv = "ENGINE_STANDIN_MODE"
	// RecordEnv names a file every event is appended to, one line each: `start`,
	// `<method> <id>`, `open <n>` at each run's start, `cancel <id>`, `exit`.
	RecordEnv = "ENGINE_STANDIN_RECORD"
	// DelayEnv is how long a run takes before it answers, a duration.
	DelayEnv = "ENGINE_STANDIN_DELAY"
	// ProgressEnv is how many progress notifications a run sends before it answers.
	ProgressEnv = "ENGINE_STANDIN_PROGRESS"
)

// The modes.
const (
	ModeAnswer              = ""
	ModeSilentDescribe      = "silent-describe"
	ModeExitAtStart         = "exit-at-start"
	ModeExitDuringRun       = "exit-during-run"
	ModeCancelAnswers       = "cancel-answers"
	ModeCancelIgnored       = "cancel-ignored"
	ModeErrorUnsupported    = "error-unsupported"
	ModeErrorBudget         = "error-budget"
	ModeErrorInternal       = "error-internal"
	ModeErrorNoCode         = "error-no-code"
	ModeNotJSON             = "not-json"
	ModeWrongJSONRPC        = "wrong-jsonrpc"
	ModeNoID                = "no-id"
	ModeNullID              = "null-id"
	ModeStringID            = "string-id"
	ModeUnknownID           = "unknown-id"
	ModeResultAndError      = "result-and-error"
	ModeNeitherResultNorErr = "neither"
	ModeEngineRequest       = "engine-request"
	ModeUnknownNotification = "unknown-notification"
	ModeProgressUnknownRun  = "progress-unknown-run"
	ModeOverflow            = "overflow"
	ModeStderr              = "stderr"
	ModeSlowRun             = "slow-run"
)

func main() {
	if err := serve(); err != nil {
		fmt.Fprintln(os.Stderr, "enginestandin:", err)
		os.Exit(3)
	}
}

// engine is the stand-in's state: its output, the runs cancelled, the runs open.
type engine struct {
	mode    string
	out     *bufio.Writer
	outMu   sync.Mutex
	record  *os.File
	recMu   sync.Mutex
	cancels sync.Map
	open    atomic.Int64
	wg      sync.WaitGroup
}

func serve() error {
	e := &engine{mode: os.Getenv(ModeEnv), out: bufio.NewWriter(os.Stdout)}
	if path := os.Getenv(RecordEnv); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- the test names the record file
		if err != nil {
			return err
		}
		e.record = f
		defer f.Close()
	}
	e.log("start")
	if e.mode == ModeExitAtStart {
		fmt.Fprintln(os.Stderr, "cannot load formalism")
		os.Exit(2)
	}
	r := bufio.NewReaderSize(os.Stdin, 1<<20)
	for {
		line, err := readLine(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var msg enginewire.Message
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Errorf("host sent a line that is not a message: %v", err)
		}
		if err := e.take(msg); err != nil {
			return err
		}
	}
	e.wg.Wait()
	e.log("exit")
	return nil
}

// readLine reads one line of any length.
func readLine(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		line = append(line, part...)
		switch {
		case err == nil:
			return line, nil
		case err == bufio.ErrBufferFull:
			continue
		case err == io.EOF && len(line) > 0:
			return line, nil
		default:
			return nil, err
		}
	}
}

// take dispatches one message from the host.
func (e *engine) take(msg enginewire.Message) error {
	if msg.JSONRPC != enginewire.JSONRPC {
		return fmt.Errorf("host sent jsonrpc %q", msg.JSONRPC)
	}
	e.log(msg.Method + " " + string(msg.ID))
	switch msg.Method {
	case enginewire.MethodDescribe:
		return e.describe(msg)
	case enginewire.MethodCovers:
		return e.covers(msg)
	case enginewire.MethodRun:
		var p enginewire.RunParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return fmt.Errorf("run params: %v", err)
		}
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			e.run(msg.ID, p)
		}()
		return nil
	case enginewire.MethodCancel:
		var p enginewire.CancelParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return fmt.Errorf("cancel params: %v", err)
		}
		e.log("cancel " + strconv.FormatInt(p.ID, 10))
		if ch, ok := e.cancels.Load(p.ID); ok {
			close(ch.(chan struct{}))
		}
		return nil
	}
	return fmt.Errorf("host sent method %q", msg.Method)
}

// describe answers with the description the environment gives, or a default one.
func (e *engine) describe(msg enginewire.Message) error {
	if e.mode == ModeSilentDescribe {
		return nil
	}
	if e.mode == ModeStderr {
		fmt.Fprintln(os.Stderr, "the formalism is missing")
		os.Exit(4)
	}
	text := os.Getenv(DescribeEnv)
	if text == "" {
		text = `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds"]}`
	}
	return e.write(enginewire.Message{JSONRPC: enginewire.JSONRPC, ID: msg.ID, Result: json.RawMessage(text)})
}

// covers answers with the covers result the environment gives, or true.
func (e *engine) covers(msg enginewire.Message) error {
	text := os.Getenv(CoversEnv)
	if text == "" {
		text = `{"covers":true}`
	}
	return e.write(enginewire.Message{JSONRPC: enginewire.JSONRPC, ID: msg.ID, Result: json.RawMessage(text)})
}

// run answers one run as the mode says, after the progress and the delay.
func (e *engine) run(id json.RawMessage, p enginewire.RunParams) {
	runID, _ := strconv.ParseInt(string(id), 10, 64)
	cancelled := make(chan struct{})
	e.cancels.Store(runID, cancelled)
	defer e.cancels.Delete(runID)
	e.log(fmt.Sprintf("open %d", e.open.Add(1)))
	defer e.open.Add(-1)

	reply := func(result json.RawMessage, err *enginewire.Error) {
		e.write(enginewire.Message{JSONRPC: enginewire.JSONRPC, ID: id, Result: result, Error: err})
	}
	fault := func(code string) {
		reply(nil, &enginewire.Error{Code: code, Message: "the stand-in was told to fail with " + code})
	}
	if n, _ := strconv.Atoi(os.Getenv(ProgressEnv)); n > 0 {
		for i := 1; i <= n; i++ {
			e.notify(enginewire.MethodProgress, enginewire.ProgressParams{ID: runID, Runs: int64(i), Depth: int64(i % 7)})
		}
	}
	if d, err := time.ParseDuration(os.Getenv(DelayEnv)); err == nil && d > 0 {
		select {
		case <-time.After(d):
		case <-cancelled:
			if e.mode != ModeCancelIgnored {
				reply(json.RawMessage(`{"claim":"none","strength":"not covered","bounds":[{"name":"steps","limit":1,"reached":true}],"reason":"cancelled"}`), nil)
				return
			}
		}
	}
	switch e.mode {
	case ModeExitDuringRun:
		fmt.Fprintln(os.Stderr, "segmentation fault (stand-in)")
		os.Exit(11)
	case ModeCancelAnswers:
		<-cancelled
		reply(json.RawMessage(`{"claim":"none","strength":"not covered","bounds":[{"name":"steps","limit":1,"reached":true}],"reason":"cancelled"}`), nil)
	case ModeCancelIgnored:
		select {}
	case ModeErrorUnsupported:
		fault(enginewire.CodeUnsupported)
	case ModeErrorBudget:
		fault(enginewire.CodeBudget)
	case ModeErrorInternal:
		fault(enginewire.CodeInternal)
	case ModeErrorNoCode:
		e.raw(`{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"message":"no code"}}`)
	case ModeNotJSON:
		e.raw(`this is not JSON`)
	case ModeWrongJSONRPC:
		e.raw(`{"jsonrpc":"1.0","id":` + string(id) + `,"result":{"claim":"none","strength":"not covered"}}`)
	case ModeNoID:
		e.raw(`{"jsonrpc":"2.0","result":{"claim":"none","strength":"not covered"}}`)
	case ModeNullID:
		e.raw(`{"jsonrpc":"2.0","id":null,"result":{"claim":"none","strength":"not covered"}}`)
	case ModeStringID:
		e.raw(`{"jsonrpc":"2.0","id":"` + string(id) + `","result":{"claim":"none","strength":"not covered"}}`)
	case ModeUnknownID:
		e.raw(`{"jsonrpc":"2.0","id":987654,"result":{"claim":"none","strength":"not covered"}}`)
	case ModeResultAndError:
		e.raw(`{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"claim":"none","strength":"not covered"},"error":{"code":"internal","message":"both"}}`)
	case ModeNeitherResultNorErr:
		e.raw(`{"jsonrpc":"2.0","id":` + string(id) + `}`)
	case ModeEngineRequest:
		e.raw(`{"jsonrpc":"2.0","id":1,"method":"describe","params":{}}`)
	case ModeUnknownNotification:
		e.raw(`{"jsonrpc":"2.0","method":"log","params":{"text":"hello"}}`)
	case ModeProgressUnknownRun:
		e.notify(enginewire.MethodProgress, enginewire.ProgressParams{ID: runID + 1000, Runs: 1})
	case ModeOverflow:
		e.raw(`{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"claim":"none","strength":"not covered","reason":"` + strings.Repeat("x", 1<<20) + `"}}`)
	case ModeSlowRun:
		time.Sleep(time.Hour)
	default:
		text := os.Getenv(ResultEnv)
		if text == "" {
			text = `{"claim":"none","strength":"not covered","reason":"the stand-in has no answer"}`
		}
		reply(json.RawMessage(text), nil)
	}
}

// notify writes one notification.
func (e *engine) notify(method string, params any) {
	raw, _ := json.Marshal(params)
	e.write(enginewire.Message{JSONRPC: enginewire.JSONRPC, Method: method, Params: raw})
}

// write frames one message as a line.
func (e *engine) write(msg enginewire.Message) error {
	line, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return e.raw(string(line))
}

// raw writes one line as it is.
func (e *engine) raw(line string) error {
	e.outMu.Lock()
	defer e.outMu.Unlock()
	if _, err := e.out.WriteString(line + "\n"); err != nil {
		return err
	}
	return e.out.Flush()
}

// log appends one event to the record file, when one is named.
func (e *engine) log(event string) {
	if e.record == nil {
		return
	}
	e.recMu.Lock()
	defer e.recMu.Unlock()
	fmt.Fprintf(e.record, "%d %s\n", os.Getpid(), event)
}
