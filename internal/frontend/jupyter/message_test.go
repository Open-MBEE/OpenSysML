package jupyter

import (
	"bytes"
	"errors"
	"testing"
)

func testSigner() signer {
	return newSigner(ConnectionInfo{Key: "secret", SignatureScheme: "hmac-sha256"})
}

func sampleMessage() Message {
	return Message{
		Identities:   [][]byte{[]byte("client-1")},
		Header:       newHeader("sess", "execute_request"),
		ParentHeader: Header{},
		Metadata:     map[string]any{"m": "v"},
		Content:      map[string]any{"code": "1 + 2", "silent": false},
		Buffers:      [][]byte{{1, 2, 3}},
	}
}

func TestAMessageRoundTripsThroughTheWire(t *testing.T) {
	s := testSigner()
	in := sampleMessage()
	frames, err := in.encode(s)
	if err != nil {
		t.Fatal(err)
	}
	// identities, delimiter, signature, four parts, one buffer
	if len(frames) != 1+1+1+4+1 {
		t.Fatalf("encoded into %d frames, want 8", len(frames))
	}
	if !bytes.Equal(frames[1], delimiter) {
		t.Errorf("frame 1 = %q, want the delimiter", frames[1])
	}
	out, err := decode(frames, s)
	if err != nil {
		t.Fatal(err)
	}
	if string(out.Identities[0]) != "client-1" {
		t.Errorf("identities = %q", out.Identities)
	}
	if out.Header != in.Header {
		t.Errorf("header = %+v, want %+v", out.Header, in.Header)
	}
	if out.Content["code"] != "1 + 2" || out.Content["silent"] != false {
		t.Errorf("content = %v", out.Content)
	}
	if out.Metadata["m"] != "v" {
		t.Errorf("metadata = %v", out.Metadata)
	}
	if len(out.Buffers) != 1 || !bytes.Equal(out.Buffers[0], []byte{1, 2, 3}) {
		t.Errorf("buffers = %v", out.Buffers)
	}
}

func TestTheSignatureCoversTheFourPartsInOrder(t *testing.T) {
	s := testSigner()
	sig := s.sign([]byte("a"), []byte("b"))
	if len(sig) != 64 {
		t.Errorf("signature %q is not hex SHA-256", sig)
	}
	if !s.verify([]byte(sig), []byte("a"), []byte("b")) {
		t.Error("a signature does not verify against the parts it signed")
	}
	if s.verify([]byte(sig), []byte("b"), []byte("a")) {
		t.Error("a signature verifies against the parts reordered")
	}
	if newSigner(ConnectionInfo{Key: "other"}).verify([]byte(sig), []byte("a"), []byte("b")) {
		t.Error("a signature verifies under another key")
	}
}

func TestWithoutAKeyMessagesAreUnsignedAndAnySignatureIsAccepted(t *testing.T) {
	s := newSigner(ConnectionInfo{})
	frames, err := sampleMessage().encode(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames[2]) != 0 {
		t.Errorf("signature frame = %q, want empty without a key", frames[2])
	}
	frames[2] = []byte("anything")
	if _, err := decode(frames, s); err != nil {
		t.Errorf("decode with a key-less signer refused a signature: %v", err)
	}
}

func TestATamperedMessageIsRefused(t *testing.T) {
	s := testSigner()
	frames, err := sampleMessage().encode(s)
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([][]byte{}, frames...)
	tampered[6] = []byte(`{"code":"%quit"}`)
	if _, err := decode(tampered, s); !errors.Is(err, ErrBadSignature) {
		t.Errorf("decode of altered content = %v, want ErrBadSignature", err)
	}
	wrongKey := newSigner(ConnectionInfo{Key: "other"})
	if _, err := decode(frames, wrongKey); !errors.Is(err, ErrBadSignature) {
		t.Errorf("decode under another key = %v, want ErrBadSignature", err)
	}
}

func TestMalformedWireMessagesAreRefusedWithoutPanicking(t *testing.T) {
	s := testSigner()
	good, err := sampleMessage().encode(s)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		frames [][]byte
		want   error
	}{
		{"no frames", nil, ErrNoDelimiter},
		{"no delimiter", [][]byte{[]byte("id"), []byte("x")}, ErrNoDelimiter},
		{"delimiter alone", [][]byte{delimiter}, ErrShortMessage},
		{"signature only", [][]byte{delimiter, good[2]}, ErrShortMessage},
		{"three parts", good[:6], ErrShortMessage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decode(tc.frames, s); !errors.Is(err, tc.want) {
				t.Errorf("decode = %v, want %v", err, tc.want)
			}
		})
	}
	t.Run("header is not JSON", func(t *testing.T) {
		frames := append([][]byte{}, good...)
		frames[3] = []byte("{not json")
		frames[2] = []byte(s.sign(frames[3], frames[4], frames[5], frames[6]))
		if _, err := decode(frames, s); err == nil {
			t.Error("a header that is not JSON decoded")
		}
	})
	t.Run("header without a type", func(t *testing.T) {
		frames := append([][]byte{}, good...)
		frames[3] = []byte(`{"msg_id":"1"}`)
		frames[2] = []byte(s.sign(frames[3], frames[4], frames[5], frames[6]))
		if _, err := decode(frames, s); err == nil {
			t.Error("a header without msg_type decoded")
		}
	})
}

func TestHeadersAreFreshAndVersioned(t *testing.T) {
	a, b := newHeader("s", "status"), newHeader("s", "status")
	if a.MsgID == b.MsgID || a.MsgID == "" {
		t.Errorf("message ids %q and %q are not distinct", a.MsgID, b.MsgID)
	}
	if a.Version != ProtocolVersion || a.Session != "s" || a.MsgType != "status" || a.Date == "" {
		t.Errorf("header = %+v", a)
	}
}

func TestContentAccessorsReadWhatJSONDecodesTo(t *testing.T) {
	m := Message{Content: map[string]any{"code": "x", "silent": true, "cursor_pos": float64(7)}}
	if m.string("code") != "x" || m.string("missing") != "" {
		t.Error("string accessor")
	}
	if !m.bool("silent") || m.bool("missing") {
		t.Error("bool accessor")
	}
	if m.int("cursor_pos", 0) != 7 || m.int("missing", 3) != 3 {
		t.Error("int accessor")
	}
}
