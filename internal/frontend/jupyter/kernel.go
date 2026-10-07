package jupyter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-zeromq/zmq4"
)

// Info is what the kernel tells a front end about itself.
type Info struct {
	Implementation        string
	ImplementationVersion string
	Language              LanguageInfo
	Banner                string
	HelpLinks             []HelpLink
}

// HelpLink is one entry of a front end's help menu.
type HelpLink struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// Kernel serves one connection: the five channels, each on its own socket,
// shell requests answered in order and control requests beside them.
type Kernel struct {
	info   Info
	conn   ConnectionInfo
	signer signer
	engine Engine
	logger *log.Logger

	// session is the kernel's own session id, on every message it originates.
	session string

	iopubMu sync.Mutex
	iopub   zmq4.Socket

	// execCount numbers the cells run, as the prompt shows them.
	execCount int

	stop   context.CancelFunc
	stopMu sync.Mutex
	// restart records whether the shutdown asked for a restart.
	restart bool
}

// New prepares a kernel over the connection, answering with the engine.
func New(info Info, conn ConnectionInfo, engine Engine, logger *log.Logger) *Kernel {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Kernel{
		info:    info,
		conn:    conn,
		signer:  newSigner(conn),
		engine:  engine,
		logger:  logger,
		session: newID(),
	}
}

// Run binds the channels and serves them until a shutdown request arrives or
// ctx ends. It reports whether the front end asked for a restart.
func (k *Kernel) Run(ctx context.Context) (restart bool, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	k.stopMu.Lock()
	k.stop = cancel
	k.stopMu.Unlock()

	shell := zmq4.NewRouter(ctx)
	control := zmq4.NewRouter(ctx)
	stdin := zmq4.NewRouter(ctx)
	iopub := zmq4.NewPub(ctx)
	hb := zmq4.NewRep(ctx)
	sockets := []struct {
		name string
		sock zmq4.Socket
		port int
	}{
		{"shell", shell, k.conn.ShellPort},
		{"control", control, k.conn.ControlPort},
		{"stdin", stdin, k.conn.StdinPort},
		{"iopub", iopub, k.conn.IOPubPort},
		{"hb", hb, k.conn.HBPort},
	}
	defer func() {
		for _, s := range sockets {
			_ = s.sock.Close()
		}
	}()
	for _, s := range sockets {
		if err := s.sock.Listen(k.conn.endpoint(s.port)); err != nil {
			return false, fmt.Errorf("listen %s on %s: %w", s.name, k.conn.endpoint(s.port), err)
		}
	}
	k.iopubMu.Lock()
	k.iopub = iopub
	k.iopubMu.Unlock()

	k.publish("status", Header{}, map[string]any{"execution_state": "starting"})

	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); k.heartbeat(ctx, hb) }()
	go func() { defer wg.Done(); k.serve(ctx, control, k.handleControl) }()
	go func() { defer wg.Done(); k.serve(ctx, shell, k.handleShell) }()
	<-ctx.Done()
	// A cell under way is stopped first: closing the sockets cannot reach the
	// goroutine busy in Execute, and Run waits for it below.
	k.engine.Interrupt()
	// Closing the sockets unblocks the receives; the loops then see ctx done.
	for _, s := range sockets {
		_ = s.sock.Close()
	}
	wg.Wait()
	k.stopMu.Lock()
	defer k.stopMu.Unlock()
	return k.restart, nil
}

// Interrupt stops the cell running, as a SIGINT from the front end does.
func (k *Kernel) Interrupt() { k.engine.Interrupt() }

// heartbeat echoes every frame it is sent, so the front end knows the kernel lives.
func (k *Kernel) heartbeat(ctx context.Context, sock zmq4.Socket) {
	for ctx.Err() == nil {
		msg, err := sock.Recv()
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, io.EOF) {
				k.logger.Printf("heartbeat: %v", err)
			}
			continue
		}
		if err := sock.SendMulti(msg); err != nil && ctx.Err() == nil {
			k.logger.Printf("heartbeat: %v", err)
		}
	}
}

// channel is one request channel: its socket, and the requests read from it,
// which a reader goroutine hands over one at a time.
type channel struct {
	sock zmq4.Socket
	msgs chan zmq4.Msg

	// aborting is set after a cell fails with stop_on_error, and the execute
	// requests already queued behind it are answered aborted. Only the
	// goroutine serving the channel touches it.
	aborting bool
}

// serve reads requests from a channel and answers each with handle, in order.
// A message that does not decode is logged and dropped, as the protocol says.
func (k *Kernel) serve(ctx context.Context, sock zmq4.Socket, handle func(*channel, Message)) {
	ch := &channel{sock: sock, msgs: make(chan zmq4.Msg)}
	go func() {
		defer close(ch.msgs)
		for ctx.Err() == nil {
			raw, err := sock.Recv()
			if err != nil {
				if ctx.Err() == nil && !errors.Is(err, io.EOF) {
					k.logger.Printf("%s: receive: %v", sock.Type(), err)
					// A socket that keeps failing would spin; pause before trying again.
					time.Sleep(10 * time.Millisecond)
				}
				continue
			}
			select {
			case ch.msgs <- raw:
			case <-ctx.Done():
				return
			}
		}
	}()
	for raw := range ch.msgs {
		k.dispatch(ch, raw, handle)
		if ch.aborting {
			k.abortQueued(ch, handle)
		}
	}
}

// dispatch decodes one wire message and hands it to handle.
func (k *Kernel) dispatch(ch *channel, raw zmq4.Msg, handle func(*channel, Message)) {
	msg, err := decode(raw.Frames, k.signer)
	if err != nil {
		k.logger.Printf("dropped message: %v", err)
		return
	}
	handle(ch, msg)
}

// handleControl answers the control channel: interrupts and shutdowns, which
// must not wait behind a cell on the shell channel.
func (k *Kernel) handleControl(ch *channel, req Message) {
	switch req.Header.MsgType {
	case "interrupt_request":
		k.engine.Interrupt()
		k.reply(ch.sock, req, "interrupt_reply", map[string]any{"status": "ok"})
	case "shutdown_request":
		k.shutdown(ch.sock, req)
	case "kernel_info_request":
		k.reply(ch.sock, req, "kernel_info_reply", k.kernelInfo())
	case "debug_request":
		k.reply(ch.sock, req, "debug_reply", map[string]any{})
	default:
		k.logger.Printf("control: unhandled %s", req.Header.MsgType)
	}
}

// handleShell answers the shell channel, bracketing each request in busy and
// idle status messages.
func (k *Kernel) handleShell(ch *channel, req Message) {
	k.publish("status", req.Header, map[string]any{"execution_state": "busy"})
	defer k.publish("status", req.Header, map[string]any{"execution_state": "idle"})
	switch req.Header.MsgType {
	case "kernel_info_request":
		k.reply(ch.sock, req, "kernel_info_reply", k.kernelInfo())
	case "execute_request":
		k.execute(ch, req)
	case "complete_request":
		k.complete(ch.sock, req)
	case "is_complete_request":
		status, indent := k.engine.IsComplete(req.string("code"))
		content := map[string]any{"status": string(status)}
		if status == Incomplete {
			content["indent"] = indent
		}
		k.reply(ch.sock, req, "is_complete_reply", content)
	case "inspect_request":
		k.inspect(ch.sock, req)
	case "history_request":
		k.reply(ch.sock, req, "history_reply", map[string]any{"status": "ok", "history": []any{}})
	case "comm_info_request":
		k.reply(ch.sock, req, "comm_info_reply", map[string]any{"status": "ok", "comms": map[string]any{}})
	case "comm_open", "comm_msg", "comm_close":
		// No comm targets are offered, so a comm opened is closed again.
		if req.Header.MsgType == "comm_open" {
			k.publish("comm_close", req.Header, map[string]any{"comm_id": req.string("comm_id"), "data": map[string]any{}})
		}
	case "shutdown_request":
		k.shutdown(ch.sock, req)
	case "interrupt_request":
		k.engine.Interrupt()
		k.reply(ch.sock, req, "interrupt_reply", map[string]any{"status": "ok"})
	default:
		k.logger.Printf("shell: unhandled %s", req.Header.MsgType)
	}
}

// kernelInfo is the content of a kernel_info_reply.
func (k *Kernel) kernelInfo() map[string]any {
	links := k.info.HelpLinks
	if links == nil {
		links = []HelpLink{}
	}
	return map[string]any{
		"status":                 "ok",
		"protocol_version":       ProtocolVersion,
		"implementation":         k.info.Implementation,
		"implementation_version": k.info.ImplementationVersion,
		"language_info":          k.info.Language,
		"banner":                 k.info.Banner,
		"debugger":               false,
		"help_links":             links,
	}
}

// execute runs a cell: its input is echoed to every front end, what it produces
// is published as it is produced, and the reply says how it ended.
func (k *Kernel) execute(ch *channel, req Message) {
	code := req.string("code")
	silent := req.bool("silent")
	storeHistory := !silent && (req.Content["store_history"] == nil || req.bool("store_history"))
	stopOnError := req.Content["stop_on_error"] == nil || req.bool("stop_on_error")

	if ch.aborting {
		k.reply(ch.sock, req, "execute_reply", map[string]any{
			"status": "aborted", "execution_count": k.execCount,
			"user_expressions": map[string]any{}, "payload": []any{},
		})
		return
	}
	if storeHistory {
		k.execCount++
	}
	count := k.execCount
	if !silent {
		k.publish("execute_input", req.Header, map[string]any{"code": code, "execution_count": count})
	}

	out := &publisher{kernel: k, parent: req.Header, count: count, silent: silent}
	err := k.engine.Execute(code, out)
	content := map[string]any{
		"execution_count":  count,
		"user_expressions": map[string]any{},
		"payload":          []any{},
	}
	if err == nil {
		content["status"] = "ok"
		k.reply(ch.sock, req, "execute_reply", content)
		return
	}
	var failure *ExecError
	if !errors.As(err, &failure) {
		failure = &ExecError{Name: "Error", Value: err.Error(), Traceback: []string{err.Error()}}
	}
	if !silent {
		out.Error(failure.Name, failure.Value, failure.Traceback)
	}
	content["status"] = "error"
	content["ename"] = failure.Name
	content["evalue"] = failure.Value
	content["traceback"] = failure.Traceback
	k.reply(ch.sock, req, "execute_reply", content)
	// The requests queued behind the failure are answered aborted once this
	// one's idle status is out, by the loop serving the channel.
	ch.aborting = stopOnError && !silent
}

// abortQueued answers the execute requests already queued behind a failed cell
// as aborted, as a front end running every cell expects, then takes requests
// normally again. The queue is drained for a short while, since the requests
// behind one being read may still be in flight; other requests arriving in
// that while are answered as usual.
func (k *Kernel) abortQueued(ch *channel, handle func(*channel, Message)) {
	defer func() { ch.aborting = false }()
	timer := time.NewTimer(abortGrace)
	defer timer.Stop()
	for {
		select {
		case raw, ok := <-ch.msgs:
			if !ok {
				return
			}
			k.dispatch(ch, raw, handle)
		case <-timer.C:
			return
		}
	}
}

// abortGrace is how long after a failed cell the requests arriving are taken
// as queued behind it, since those behind one being read may still be in
// flight; the same window the reference kernel keeps.
var abortGrace = 50 * time.Millisecond

// complete answers a completion request.
func (k *Kernel) complete(sock zmq4.Socket, req Message) {
	code := req.string("code")
	cursor := req.int("cursor_pos", len([]rune(code)))
	c := k.engine.Complete(code, cursor)
	matches := c.Matches
	if matches == nil {
		matches = []string{}
	}
	k.reply(sock, req, "complete_reply", map[string]any{
		"status":       "ok",
		"matches":      matches,
		"cursor_start": c.CursorStart,
		"cursor_end":   c.CursorEnd,
		"metadata":     map[string]any{},
	})
}

// inspect answers an inspection request.
func (k *Kernel) inspect(sock zmq4.Socket, req Message) {
	code := req.string("code")
	cursor := req.int("cursor_pos", len([]rune(code)))
	in := k.engine.Inspect(code, cursor, req.int("detail_level", 0))
	data := in.Data
	if data == nil {
		data = MIMEBundle{}
	}
	k.reply(sock, req, "inspect_reply", map[string]any{
		"status":   "ok",
		"found":    in.Found,
		"data":     data,
		"metadata": map[string]any{},
	})
}

// shutdown answers a shutdown request and ends Run.
func (k *Kernel) shutdown(sock zmq4.Socket, req Message) {
	restart := req.bool("restart")
	k.engine.Shutdown(restart)
	k.reply(sock, req, "shutdown_reply", map[string]any{"status": "ok", "restart": restart})
	k.stopMu.Lock()
	k.restart = restart
	stop := k.stop
	k.stopMu.Unlock()
	if stop != nil {
		stop()
	}
}

// reply sends a reply to req on its channel, routed back to its sender.
func (k *Kernel) reply(sock zmq4.Socket, req Message, msgType string, content map[string]any) {
	msg := Message{
		Identities:   req.Identities,
		Header:       newHeader(k.session, msgType),
		ParentHeader: req.Header,
		Metadata:     map[string]any{},
		Content:      content,
	}
	frames, err := msg.encode(k.signer)
	if err != nil {
		k.logger.Printf("%s: encode: %v", msgType, err)
		return
	}
	if err := sock.SendMulti(zmq4.NewMsgFrom(frames...)); err != nil {
		k.logger.Printf("%s: send: %v", msgType, err)
	}
}

// publish broadcasts a message on IOPub under its type's topic.
func (k *Kernel) publish(msgType string, parent Header, content map[string]any) {
	k.iopubMu.Lock()
	defer k.iopubMu.Unlock()
	if k.iopub == nil {
		return
	}
	msg := Message{
		Identities:   [][]byte{[]byte("kernel." + k.session + "." + msgType)},
		Header:       newHeader(k.session, msgType),
		ParentHeader: parent,
		Metadata:     map[string]any{},
		Content:      content,
	}
	frames, err := msg.encode(k.signer)
	if err != nil {
		k.logger.Printf("iopub %s: encode: %v", msgType, err)
		return
	}
	if err := k.iopub.SendMulti(zmq4.NewMsgFrom(frames...)); err != nil {
		k.logger.Printf("iopub %s: send: %v", msgType, err)
	}
}

// publisher is the Output of one cell: everything it writes goes to IOPub under
// the cell's request.
type publisher struct {
	kernel *Kernel
	parent Header
	count  int
	silent bool
}

func (p *publisher) Stream(name, text string) {
	if p.silent || text == "" {
		return
	}
	p.kernel.publish("stream", p.parent, map[string]any{"name": name, "text": text})
}

func (p *publisher) Display(data MIMEBundle, metadata map[string]any) {
	if p.silent {
		return
	}
	p.kernel.publish("display_data", p.parent, map[string]any{
		"data": data, "metadata": orEmpty(metadata), "transient": map[string]any{},
	})
}

func (p *publisher) Result(data MIMEBundle) {
	if p.silent {
		return
	}
	p.kernel.publish("execute_result", p.parent, map[string]any{
		"data": data, "metadata": map[string]any{}, "execution_count": p.count,
	})
}

func (p *publisher) Error(name, value string, traceback []string) {
	if p.silent {
		return
	}
	if traceback == nil {
		traceback = []string{}
	}
	p.kernel.publish("error", p.parent, map[string]any{"ename": name, "evalue": value, "traceback": traceback})
}

// Stderr is where the kernel logs when asked to; the launcher decides.
func Stderr() *log.Logger { return log.New(os.Stderr, "sysml-jupyter-kernel: ", log.LstdFlags) }
