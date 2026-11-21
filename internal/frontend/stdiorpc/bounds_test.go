package stdiorpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// heldService answers GetServerInfo only once release is closed.
type heldService struct {
	pb.UnimplementedSysMLServiceServer
	release chan struct{}
}

func (h *heldService) GetServerInfo(ctx context.Context, _ *pb.ServerInfoRequest) (*pb.ServerInfoResponse, error) {
	select {
	case <-h.release:
		return &pb.ServerInfoResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func infoFrame(id int) string {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"GetServerInfo"}`, id)
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

func served(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("Serve did not return")
		return nil
	}
}

func TestServeGivesUpOnAPeerThatStopsReading(t *testing.T) {
	svc := &heldService{release: make(chan struct{})}
	close(svc.release)
	server := NewServer(svc)
	server.writeStall = 100 * time.Millisecond

	inR, inW := io.Pipe()
	defer inW.Close()
	_, outW := io.Pipe() // never read: every write to it blocks
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), inR, outW) }()

	if _, err := io.WriteString(inW, infoFrame(1)+infoFrame(2)); err != nil {
		t.Fatal(err)
	}
	if err := served(t, done); !errors.Is(err, ErrPeerStalled) {
		t.Fatalf("Serve = %v, want ErrPeerStalled while the input is still open", err)
	}
}

func TestServeRefusesCallsPastTheInFlightBound(t *testing.T) {
	svc := &heldService{release: make(chan struct{})}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- NewServer(svc).Serve(context.Background(), inR, outW) }()

	var frames strings.Builder
	for id := 1; id <= maxInFlight+1; id++ {
		frames.WriteString(infoFrame(id))
	}
	go func() { _, _ = io.WriteString(inW, frames.String()) }()

	answers := bufio.NewReader(outR)
	read := func() response {
		f, err := readFrame(answers)
		if err != nil {
			t.Fatalf("read an answer: %v", err)
		}
		var res response
		if err := json.Unmarshal(f.body, &res); err != nil {
			t.Fatal(err)
		}
		return res
	}
	refused := read()
	if string(refused.ID) != fmt.Sprint(maxInFlight+1) || refused.Error == nil ||
		refused.Error.Code != uint32(connect.CodeResourceExhausted) {
		t.Fatalf("first answer = id %s, error %+v; want call %d refused as resource-exhausted",
			refused.ID, refused.Error, maxInFlight+1)
	}

	close(svc.release)
	for range maxInFlight {
		if res := read(); res.Error != nil {
			t.Errorf("held call %s failed: %+v", res.ID, res.Error)
		}
	}
	_ = inW.Close()
	if err := served(t, done); err != nil {
		t.Errorf("Serve: %v", err)
	}
}

// A client that writes its whole batch before it reads any answer must still get
// every answer: refusing a call may not stop the server reading the calls behind it.
func TestServeReadsOnWhileRefusalsWaitForThePeer(t *testing.T) {
	svc := &heldService{release: make(chan struct{})}
	server := NewServer(svc)
	server.writeStall = 2 * time.Second
	inR, inW := io.Pipe()
	outR, outW := io.Pipe() // unbuffered: no answer is accepted until the client reads
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), inR, outW) }()

	const refusedCalls = maxRefusalsWaiting
	var frames strings.Builder
	for id := 1; id <= maxInFlight+refusedCalls; id++ {
		frames.WriteString(infoFrame(id))
	}
	sent := make(chan error, 1)
	go func() {
		_, err := io.WriteString(inW, frames.String())
		sent <- err
	}()
	select {
	case err := <-sent:
		if err != nil {
			t.Fatal(err)
		}
	case err := <-done:
		t.Fatalf("Serve = %v before the batch was read", err)
	}

	answers := bufio.NewReader(outR)
	read := func() response {
		t.Helper()
		f, err := readFrame(answers)
		if err != nil {
			t.Fatalf("read an answer: %v", err)
		}
		var res response
		if err := json.Unmarshal(f.body, &res); err != nil {
			t.Fatal(err)
		}
		return res
	}
	for id := maxInFlight + 1; id <= maxInFlight+refusedCalls; id++ {
		res := read()
		if string(res.ID) != fmt.Sprint(id) || res.Error == nil ||
			res.Error.Code != uint32(connect.CodeResourceExhausted) {
			t.Fatalf("answer = id %s, error %+v; want call %d refused as resource-exhausted",
				res.ID, res.Error, id)
		}
	}
	close(svc.release)
	for range maxInFlight {
		if res := read(); res.Error != nil {
			t.Errorf("held call %s failed: %+v", res.ID, res.Error)
		}
	}
	_ = inW.Close()
	if err := served(t, done); err != nil {
		t.Errorf("Serve: %v", err)
	}
}

// A peer that takes longer than the stall bound over a whole answer, but not over
// any one write of it, is still reading and keeps its session.
func TestServeKeepsAPeerThatDrainsSlowly(t *testing.T) {
	svc := &heldService{release: make(chan struct{})}
	close(svc.release)
	server := NewServer(svc)
	server.writeStall = 200 * time.Millisecond

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), inR, outW) }()

	const calls = 12
	go func() {
		for id := 1; id <= calls; id++ {
			_, _ = io.WriteString(inW, infoFrame(id))
		}
		_ = inW.Close()
	}()
	answers := bufio.NewReaderSize(outR, 16)
	for range calls {
		time.Sleep(server.writeStall / 4) // 12 answers take three stall bounds in all
		if _, err := readFrame(answers); err != nil {
			t.Fatalf("read an answer: %v", err)
		}
	}
	if err := served(t, done); err != nil {
		t.Errorf("Serve = %v, want the slow reader served to the end", err)
	}
}

func TestServeRefusesAnEndlessHeaderLine(t *testing.T) {
	input := io.MultiReader(strings.NewReader("X-Pad: "), endless('a'))
	err := NewServer(&heldService{}).Serve(context.Background(), input, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "message too large") {
		t.Fatalf("Serve = %v, want the header refused", err)
	}
}

// endless is a reader of one byte repeated without end.
type endless byte

func (e endless) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(e)
	}
	return len(p), nil
}
