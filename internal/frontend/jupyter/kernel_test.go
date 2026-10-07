package jupyter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-zeromq/zmq4"
)

// fakeEngine answers the kernel's requests predictably: "block" runs until
// interrupted, "fail" fails, anything else echoes.
type fakeEngine struct {
	mu          sync.Mutex
	interrupted chan struct{}
	shutdowns   []bool
}

func newFakeEngine() *fakeEngine { return &fakeEngine{interrupted: make(chan struct{}, 1)} }

func (e *fakeEngine) Execute(code string, out Output) error {
	switch code {
	case "block":
		out.Stream("stdout", "started\n")
		<-e.interrupted
		return &ExecError{Name: "KeyboardInterrupt", Value: "interrupted", Traceback: []string{"interrupted"}}
	case "fail":
		return &ExecError{Name: "TestError", Value: "as asked", Traceback: []string{"as asked"}}
	case "plain":
		return errors.New("untyped failure")
	}
	out.Stream("stdout", "ran "+code+"\n")
	out.Display(MIMEBundle{"text/plain": "shown"}, map[string]any{"k": "v"})
	out.Result(MIMEBundle{"text/plain": "result of " + code})
	return nil
}

func (e *fakeEngine) Complete(code string, cursor int) Completion {
	return Completion{Matches: []string{code + "!"}, CursorStart: 0, CursorEnd: cursor}
}

func (e *fakeEngine) IsComplete(code string) (IsCompleteStatus, string) {
	if strings.HasSuffix(code, "{") {
		return Incomplete, "  "
	}
	return Complete, ""
}

func (e *fakeEngine) Inspect(code string, cursor int, detail int) Inspection {
	if code == "known" {
		return Inspection{Found: true, Data: MIMEBundle{"text/plain": fmt.Sprintf("known at %d, detail %d", cursor, detail)}}
	}
	return Inspection{}
}

func (e *fakeEngine) Interrupt() {
	select {
	case e.interrupted <- struct{}{}:
	default:
	}
}

func (e *fakeEngine) Shutdown(restart bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shutdowns = append(e.shutdowns, restart)
}

// client is a front end over the five channels of a running kernel.
type client struct {
	t       *testing.T
	conn    ConnectionInfo
	signer  signer
	shell   zmq4.Socket
	control zmq4.Socket
	iopub   zmq4.Socket
	hb      zmq4.Socket
	// pub receives every IOPub message as it arrives.
	pub chan Message
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startKernel runs a kernel over the engine and connects a client to it; the
// kernel's Run result is read from done.
func startKernel(t *testing.T, engine Engine) (*client, *Kernel, chan struct{ restart bool }) {
	t.Helper()
	conn := ConnectionInfo{
		Transport: "tcp", IP: "127.0.0.1", Key: "test-key", SignatureScheme: "hmac-sha256",
		ShellPort: freePort(t), IOPubPort: freePort(t), StdinPort: freePort(t), ControlPort: freePort(t), HBPort: freePort(t),
	}
	kernel := New(Info{Implementation: "test", ImplementationVersion: "0", Language: LanguageInfo{Name: "sysml"}}, conn, engine, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{ restart bool }, 1)
	go func() {
		restart, err := kernel.Run(ctx)
		if err != nil {
			t.Errorf("Run: %v", err)
		}
		done <- struct{ restart bool }{restart}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("the kernel did not stop")
		}
	})

	c := &client{t: t, conn: conn, signer: newSigner(conn), pub: make(chan Message, 256)}
	c.shell = zmq4.NewDealer(ctx, zmq4.WithID(zmq4.SocketIdentity("shell-client")))
	c.control = zmq4.NewDealer(ctx, zmq4.WithID(zmq4.SocketIdentity("control-client")))
	c.iopub = zmq4.NewSub(ctx)
	c.hb = zmq4.NewReq(ctx)
	if err := c.iopub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		t.Fatal(err)
	}
	for _, s := range []struct {
		sock zmq4.Socket
		port int
	}{{c.shell, conn.ShellPort}, {c.control, conn.ControlPort}, {c.iopub, conn.IOPubPort}, {c.hb, conn.HBPort}} {
		if err := dialUntilUp(ctx, s.sock, conn.endpoint(s.port)); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, s := range []zmq4.Socket{c.shell, c.control, c.iopub, c.hb} {
			s.Close()
		}
	})
	go func() {
		for {
			raw, err := c.iopub.Recv()
			if err != nil {
				return
			}
			msg, err := decode(raw.Frames, c.signer)
			if err != nil {
				t.Errorf("iopub: %v", err)
				continue
			}
			c.pub <- msg
		}
	}()
	// A subscription joins late: ask for kernel info until its status shows on IOPub.
	deadline := time.Now().Add(10 * time.Second)
	for {
		id := c.send(c.shell, "kernel_info_request", map[string]any{})
		c.reply(c.shell, id)
		if c.sawStatus(id) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("IOPub never delivered the status of a kernel_info_request")
		}
	}
	return c, kernel, done
}

// dialUntilUp dials the endpoint, retrying while the kernel is still binding.
func dialUntilUp(ctx context.Context, sock zmq4.Socket, ep string) error {
	var err error
	for i := 0; i < 200; i++ {
		if err = sock.Dial(ep); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	return err
}

// sawStatus drains IOPub for up to half a second and reports whether an idle
// status for the request arrived.
func (c *client) sawStatus(parent string) bool {
	timer := time.After(500 * time.Millisecond)
	for {
		select {
		case m := <-c.pub:
			if m.ParentHeader.MsgID == parent && m.Header.MsgType == "status" && m.Content["execution_state"] == "idle" {
				return true
			}
		case <-timer:
			return false
		}
	}
}

// send sends a request on sock and returns its message id.
func (c *client) send(sock zmq4.Socket, msgType string, content map[string]any) string {
	c.t.Helper()
	msg := Message{Header: newHeader("client", msgType), Metadata: map[string]any{}, Content: content}
	frames, err := msg.encode(c.signer)
	if err != nil {
		c.t.Fatal(err)
	}
	if err := sock.SendMulti(zmq4.NewMsgFrom(frames...)); err != nil {
		c.t.Fatal(err)
	}
	return msg.Header.MsgID
}

// reply reads replies on sock until the one answering parent.
func (c *client) reply(sock zmq4.Socket, parent string) Message {
	c.t.Helper()
	type result struct {
		msg Message
		err error
	}
	for {
		got := make(chan result, 1)
		go func() {
			raw, err := sock.Recv()
			if err != nil {
				got <- result{err: err}
				return
			}
			msg, err := decode(raw.Frames, c.signer)
			got <- result{msg, err}
		}()
		select {
		case r := <-got:
			if r.err != nil {
				c.t.Fatalf("receive: %v", r.err)
			}
			if r.msg.ParentHeader.MsgID == parent {
				return r.msg
			}
		case <-time.After(10 * time.Second):
			c.t.Fatalf("no reply to %s within ten seconds", parent)
		}
	}
}

// published collects the IOPub messages of one request, up to and including
// its idle status.
func (c *client) published(parent string) []Message {
	c.t.Helper()
	var out []Message
	timer := time.After(10 * time.Second)
	for {
		select {
		case m := <-c.pub:
			if m.ParentHeader.MsgID != parent {
				continue
			}
			out = append(out, m)
			if m.Header.MsgType == "status" && m.Content["execution_state"] == "idle" {
				return out
			}
		case <-timer:
			c.t.Fatalf("IOPub did not go idle for %s within ten seconds; saw %s", parent, types(out))
		}
	}
}

func types(msgs []Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Header.MsgType
		if m.Header.MsgType == "status" {
			out[i] += ":" + m.Content["execution_state"].(string)
		}
	}
	return out
}

func TestTheHeartbeatEchoesWhatItIsSent(t *testing.T) {
	c, _, _ := startKernel(t, newFakeEngine())
	for _, ping := range []string{"ping", "", "another"} {
		if err := c.hb.Send(zmq4.NewMsgString(ping)); err != nil {
			t.Fatal(err)
		}
		got := make(chan zmq4.Msg, 1)
		go func() {
			m, err := c.hb.Recv()
			if err != nil {
				t.Errorf("heartbeat receive: %v", err)
			}
			got <- m
		}()
		select {
		case m := <-got:
			if string(m.Bytes()) != ping {
				t.Errorf("heartbeat echoed %q for %q", m.Bytes(), ping)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no heartbeat echo within five seconds")
		}
	}
}

func TestKernelInfoDescribesTheKernelOnBothChannels(t *testing.T) {
	c, _, _ := startKernel(t, newFakeEngine())
	for _, sock := range []zmq4.Socket{c.shell, c.control} {
		id := c.send(sock, "kernel_info_request", map[string]any{})
		reply := c.reply(sock, id)
		if reply.Header.MsgType != "kernel_info_reply" || reply.Content["status"] != "ok" {
			t.Fatalf("reply = %s %v", reply.Header.MsgType, reply.Content)
		}
		if reply.Content["protocol_version"] != ProtocolVersion || reply.Content["implementation"] != "test" {
			t.Errorf("content = %v", reply.Content)
		}
		lang, _ := reply.Content["language_info"].(map[string]any)
		if lang["name"] != "sysml" {
			t.Errorf("language_info = %v", lang)
		}
		if reply.Header.Session == "" || reply.Header.Version != ProtocolVersion {
			t.Errorf("header = %+v", reply.Header)
		}
	}
	// The shell brackets its work in status messages; the control channel does not.
	id := c.send(c.shell, "kernel_info_request", map[string]any{})
	c.reply(c.shell, id)
	if got := types(c.published(id)); strings.Join(got, ",") != "status:busy,status:idle" {
		t.Errorf("shell kernel_info published %v", got)
	}
}

func TestExecutePublishesTheCellAndItsOutputInOrder(t *testing.T) {
	c, _, _ := startKernel(t, newFakeEngine())
	id := c.send(c.shell, "execute_request", map[string]any{"code": "one", "silent": false})
	reply := c.reply(c.shell, id)
	if reply.Content["status"] != "ok" || reply.Content["execution_count"] != float64(1) {
		t.Errorf("reply = %v", reply.Content)
	}
	msgs := c.published(id)
	want := "status:busy,execute_input,stream,display_data,execute_result,status:idle"
	if got := strings.Join(types(msgs), ","); got != want {
		t.Fatalf("published %s, want %s", got, want)
	}
	if msgs[1].Content["code"] != "one" || msgs[1].Content["execution_count"] != float64(1) {
		t.Errorf("execute_input = %v", msgs[1].Content)
	}
	if msgs[2].Content["name"] != "stdout" || msgs[2].Content["text"] != "ran one\n" {
		t.Errorf("stream = %v", msgs[2].Content)
	}
	if meta, _ := msgs[3].Content["metadata"].(map[string]any); meta["k"] != "v" {
		t.Errorf("display_data metadata = %v", msgs[3].Content["metadata"])
	}
	if data, _ := msgs[4].Content["data"].(map[string]any); data["text/plain"] != "result of one" || msgs[4].Content["execution_count"] != float64(1) {
		t.Errorf("execute_result = %v", msgs[4].Content)
	}
	for _, m := range msgs {
		if !strings.HasPrefix(string(m.Identities[0]), "kernel.") || !strings.HasSuffix(string(m.Identities[0]), "."+m.Header.MsgType) {
			t.Errorf("IOPub topic %q does not name the message type", m.Identities[0])
		}
	}

	// The count advances per cell, and a silent cell neither counts nor echoes.
	id = c.send(c.shell, "execute_request", map[string]any{"code": "two"})
	if reply := c.reply(c.shell, id); reply.Content["execution_count"] != float64(2) {
		t.Errorf("second reply = %v", reply.Content)
	}
	c.published(id)
	id = c.send(c.shell, "execute_request", map[string]any{"code": "three", "silent": true})
	if reply := c.reply(c.shell, id); reply.Content["execution_count"] != float64(2) {
		t.Errorf("silent reply = %v", reply.Content)
	}
	if got := strings.Join(types(c.published(id)), ","); got != "status:busy,status:idle" {
		t.Errorf("silent cell published %s", got)
	}
}

func TestAFailedCellIsReportedAndAbortsTheCellsQueuedBehindIt(t *testing.T) {
	c, _, _ := startKernel(t, newFakeEngine())
	failing := c.send(c.shell, "execute_request", map[string]any{"code": "fail"})
	queued := c.send(c.shell, "execute_request", map[string]any{"code": "after"})
	reply := c.reply(c.shell, failing)
	if reply.Content["status"] != "error" || reply.Content["ename"] != "TestError" || reply.Content["evalue"] != "as asked" {
		t.Errorf("failed reply = %v", reply.Content)
	}
	msgs := c.published(failing)
	if got := strings.Join(types(msgs), ","); got != "status:busy,execute_input,error,status:idle" {
		t.Errorf("failed cell published %s", got)
	}
	if reply := c.reply(c.shell, queued); reply.Content["status"] != "aborted" {
		t.Errorf("queued reply = %v, want aborted", reply.Content)
	}
	c.published(queued)
	// Once the grace period passes, cells run again, and the count skipped the failures' successors only.
	time.Sleep(3 * abortGrace)
	id := c.send(c.shell, "execute_request", map[string]any{"code": "later"})
	if reply := c.reply(c.shell, id); reply.Content["status"] != "ok" || reply.Content["execution_count"] != float64(2) {
		t.Errorf("later reply = %v", reply.Content)
	}
	c.published(id)

	// An untyped failure is reported as a plain Error, and stop_on_error false keeps the queue.
	time.Sleep(3 * abortGrace)
	plain := c.send(c.shell, "execute_request", map[string]any{"code": "plain", "stop_on_error": false})
	next := c.send(c.shell, "execute_request", map[string]any{"code": "next"})
	if reply := c.reply(c.shell, plain); reply.Content["ename"] != "Error" || reply.Content["evalue"] != "untyped failure" {
		t.Errorf("plain reply = %v", reply.Content)
	}
	c.published(plain)
	if reply := c.reply(c.shell, next); reply.Content["status"] != "ok" {
		t.Errorf("reply after a non-stopping failure = %v", reply.Content)
	}
	c.published(next)
}

func TestCompletionInspectionAndCompletenessAreAnswered(t *testing.T) {
	c, _, _ := startKernel(t, newFakeEngine())
	id := c.send(c.shell, "complete_request", map[string]any{"code": "abc", "cursor_pos": 2})
	reply := c.reply(c.shell, id)
	if matches, _ := reply.Content["matches"].([]any); len(matches) != 1 || matches[0] != "abc!" || reply.Content["cursor_end"] != float64(2) {
		t.Errorf("complete_reply = %v", reply.Content)
	}
	c.published(id)

	id = c.send(c.shell, "is_complete_request", map[string]any{"code": "package P {"})
	if reply := c.reply(c.shell, id); reply.Content["status"] != "incomplete" || reply.Content["indent"] != "  " {
		t.Errorf("is_complete_reply = %v", reply.Content)
	}
	c.published(id)
	id = c.send(c.shell, "is_complete_request", map[string]any{"code": "done"})
	if reply := c.reply(c.shell, id); reply.Content["status"] != "complete" || reply.Content["indent"] != nil {
		t.Errorf("is_complete_reply = %v", reply.Content)
	}
	c.published(id)

	id = c.send(c.shell, "inspect_request", map[string]any{"code": "known", "cursor_pos": 3, "detail_level": 1})
	reply = c.reply(c.shell, id)
	if data, _ := reply.Content["data"].(map[string]any); reply.Content["found"] != true || data["text/plain"] != "known at 3, detail 1" {
		t.Errorf("inspect_reply = %v", reply.Content)
	}
	c.published(id)
	id = c.send(c.shell, "inspect_request", map[string]any{"code": "other"})
	if reply := c.reply(c.shell, id); reply.Content["found"] != false || len(reply.Content["data"].(map[string]any)) != 0 {
		t.Errorf("inspect_reply for an unknown name = %v", reply.Content)
	}
	c.published(id)

	id = c.send(c.shell, "history_request", map[string]any{})
	if reply := c.reply(c.shell, id); reply.Header.MsgType != "history_reply" {
		t.Errorf("history reply = %s", reply.Header.MsgType)
	}
	c.published(id)
	id = c.send(c.shell, "comm_info_request", map[string]any{})
	if reply := c.reply(c.shell, id); reply.Header.MsgType != "comm_info_reply" {
		t.Errorf("comm_info reply = %s", reply.Header.MsgType)
	}
	c.published(id)
}

func TestAnInterruptOnTheControlChannelStopsTheRunningCell(t *testing.T) {
	c, _, _ := startKernel(t, newFakeEngine())
	blocked := c.send(c.shell, "execute_request", map[string]any{"code": "block"})
	// Wait for the cell to report it started before interrupting it.
	deadline := time.After(10 * time.Second)
	for started := false; !started; {
		select {
		case m := <-c.pub:
			started = m.ParentHeader.MsgID == blocked && m.Header.MsgType == "stream"
		case <-deadline:
			t.Fatal("the blocking cell never started")
		}
	}
	id := c.send(c.control, "interrupt_request", map[string]any{})
	if reply := c.reply(c.control, id); reply.Header.MsgType != "interrupt_reply" || reply.Content["status"] != "ok" {
		t.Errorf("interrupt reply = %s %v", reply.Header.MsgType, reply.Content)
	}
	reply := c.reply(c.shell, blocked)
	if reply.Content["status"] != "error" || reply.Content["ename"] != "KeyboardInterrupt" {
		t.Errorf("interrupted cell's reply = %v", reply.Content)
	}
	msgs := c.published(blocked)
	if got := strings.Join(types(msgs), ","); got != "error,status:idle" {
		t.Errorf("after the interrupt the cell published %s", got)
	}
	// An interrupt aborts what was queued behind the cell, as a failure does; a cell sent later runs.
	time.Sleep(3 * abortGrace)
	id = c.send(c.shell, "execute_request", map[string]any{"code": "again"})
	if reply := c.reply(c.shell, id); reply.Content["status"] != "ok" {
		t.Errorf("the cell after the interrupt = %v", reply.Content)
	}
	c.published(id)
}

func TestAMessageWithABadSignatureIsDroppedAndTheKernelGoesOn(t *testing.T) {
	c, _, _ := startKernel(t, newFakeEngine())
	forged := Message{Header: newHeader("intruder", "execute_request"), Metadata: map[string]any{}, Content: map[string]any{"code": "forged"}}
	frames, err := forged.encode(newSigner(ConnectionInfo{Key: "wrong"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.shell.SendMulti(zmq4.NewMsgFrom(frames...)); err != nil {
		t.Fatal(err)
	}
	if err := c.shell.SendMulti(zmq4.NewMsgFrom([]byte("junk"))); err != nil {
		t.Fatal(err)
	}
	id := c.send(c.shell, "execute_request", map[string]any{"code": "genuine"})
	reply := c.reply(c.shell, id)
	if reply.Content["status"] != "ok" || reply.Content["execution_count"] != float64(1) {
		t.Errorf("the genuine request got %v; the forged one must not have counted", reply.Content)
	}
	for _, m := range c.published(id) {
		if m.Header.MsgType == "execute_input" && m.Content["code"] != "genuine" {
			t.Errorf("executed %v", m.Content)
		}
	}
	select {
	case m := <-c.pub:
		if m.ParentHeader.Session == "intruder" {
			t.Errorf("the forged request produced %s", m.Header.MsgType)
		}
	case <-time.After(200 * time.Millisecond):
	}
}

func TestShutdownEndsRunAndReportsTheRestart(t *testing.T) {
	for _, restart := range []bool{false, true} {
		t.Run(fmt.Sprintf("restart=%v", restart), func(t *testing.T) {
			engine := newFakeEngine()
			c, _, done := startKernel(t, engine)
			id := c.send(c.control, "shutdown_request", map[string]any{"restart": restart})
			reply := c.reply(c.control, id)
			if reply.Header.MsgType != "shutdown_reply" || reply.Content["restart"] != restart {
				t.Errorf("reply = %s %v", reply.Header.MsgType, reply.Content)
			}
			select {
			case r := <-done:
				if r.restart != restart {
					t.Errorf("Run returned restart=%v, want %v", r.restart, restart)
				}
				done <- r
			case <-time.After(10 * time.Second):
				t.Fatal("Run did not return after the shutdown")
			}
			engine.mu.Lock()
			defer engine.mu.Unlock()
			if len(engine.shutdowns) != 1 || engine.shutdowns[0] != restart {
				t.Errorf("engine shutdowns = %v", engine.shutdowns)
			}
		})
	}
}

func TestShutdownStopsTheCellRunningAndEndsRun(t *testing.T) {
	engine := newFakeEngine()
	c, _, done := startKernel(t, engine)
	blocked := c.send(c.shell, "execute_request", map[string]any{"code": "block"})
	deadline := time.After(10 * time.Second)
	for started := false; !started; {
		select {
		case m := <-c.pub:
			started = m.ParentHeader.MsgID == blocked && m.Header.MsgType == "stream"
		case <-deadline:
			t.Fatal("the blocking cell never started")
		}
	}
	id := c.send(c.control, "shutdown_request", map[string]any{"restart": false})
	if reply := c.reply(c.control, id); reply.Header.MsgType != "shutdown_reply" {
		t.Errorf("reply = %s %v", reply.Header.MsgType, reply.Content)
	}
	select {
	case r := <-done:
		done <- r
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return while a cell was running: the shutdown must stop it")
	}
}

func TestKernelInterruptReachesTheEngine(t *testing.T) {
	engine := newFakeEngine()
	kernel := New(Info{}, ConnectionInfo{}, engine, nil)
	kernel.Interrupt()
	select {
	case <-engine.interrupted:
	default:
		t.Error("Interrupt did not reach the engine")
	}
}
