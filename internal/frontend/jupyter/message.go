package jupyter

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ProtocolVersion is the version of the Jupyter messaging protocol the kernel
// speaks.
const ProtocolVersion = "5.3"

// delimiter separates the routing identities of a wire message from its signed
// parts.
var delimiter = []byte("<IDS|MSG>")

// Header identifies a message: who sent it, in which session, and what kind it is.
type Header struct {
	MsgID    string `json:"msg_id"`
	Session  string `json:"session"`
	Username string `json:"username"`
	Date     string `json:"date"`
	MsgType  string `json:"msg_type"`
	Version  string `json:"version"`
}

// Message is one Jupyter message, decoded from or encoded to its wire frames.
type Message struct {
	// Identities are the routing frames a ROUTER socket prepends, echoed back on
	// the reply so it reaches the sender.
	Identities   [][]byte
	Header       Header
	ParentHeader Header
	Metadata     map[string]any
	Content      map[string]any
	Buffers      [][]byte
}

// Errors a wire message is refused for.
var (
	ErrNoDelimiter  = errors.New("no <IDS|MSG> delimiter")
	ErrShortMessage = errors.New("fewer than four frames after the delimiter")
	ErrBadSignature = errors.New("signature does not verify")
)

// signer signs the four JSON parts of a message with HMAC-SHA256, or leaves them
// unsigned when the connection has no key.
type signer struct{ key []byte }

func newSigner(info ConnectionInfo) signer { return signer{key: []byte(info.Key)} }

// sign is the hex HMAC over the parts in order, empty without a key.
func (s signer) sign(parts ...[]byte) string {
	if len(s.key) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, s.key)
	for _, part := range parts {
		mac.Write(part)
	}
	return hex.EncodeToString(mac.Sum(nil))
}

// verify reports whether signature is the one the parts sign to. Without a key
// every signature is accepted, as the protocol says.
func (s signer) verify(signature []byte, parts ...[]byte) bool {
	if len(s.key) == 0 {
		return true
	}
	return hmac.Equal(signature, []byte(s.sign(parts...)))
}

// decode reads a wire message: the identities before the delimiter, the
// signature, then header, parent header, metadata and content, then any buffers.
func decode(frames [][]byte, s signer) (Message, error) {
	at := -1
	for i, frame := range frames {
		if bytes.Equal(frame, delimiter) {
			at = i
			break
		}
	}
	if at < 0 {
		return Message{}, ErrNoDelimiter
	}
	rest := frames[at+1:]
	if len(rest) < 5 {
		return Message{}, ErrShortMessage
	}
	signature, parts := rest[0], rest[1:5]
	if !s.verify(signature, parts...) {
		return Message{}, ErrBadSignature
	}
	msg := Message{Identities: frames[:at], Buffers: rest[5:]}
	if err := json.Unmarshal(parts[0], &msg.Header); err != nil {
		return Message{}, fmt.Errorf("header: %w", err)
	}
	if err := json.Unmarshal(parts[1], &msg.ParentHeader); err != nil {
		return Message{}, fmt.Errorf("parent header: %w", err)
	}
	if err := json.Unmarshal(parts[2], &msg.Metadata); err != nil {
		return Message{}, fmt.Errorf("metadata: %w", err)
	}
	if err := json.Unmarshal(parts[3], &msg.Content); err != nil {
		return Message{}, fmt.Errorf("content: %w", err)
	}
	if msg.Header.MsgType == "" {
		return Message{}, errors.New("header names no msg_type")
	}
	return msg, nil
}

// encode writes the message as wire frames, signed.
func (m Message) encode(s signer) ([][]byte, error) {
	parts := make([][]byte, 0, 4)
	for _, v := range []any{m.Header, m.ParentHeader, orEmpty(m.Metadata), orEmpty(m.Content)} {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		parts = append(parts, raw)
	}
	frames := make([][]byte, 0, len(m.Identities)+2+len(parts)+len(m.Buffers))
	frames = append(frames, m.Identities...)
	frames = append(frames, delimiter, []byte(s.sign(parts...)))
	frames = append(frames, parts...)
	frames = append(frames, m.Buffers...)
	return frames, nil
}

func orEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// newHeader is the header of a message the kernel sends: a fresh id, the
// kernel's session, and the time now.
func newHeader(session, msgType string) Header {
	return Header{
		MsgID:    newID(),
		Session:  session,
		Username: "kernel",
		Date:     time.Now().UTC().Format(time.RFC3339Nano),
		MsgType:  msgType,
		Version:  ProtocolVersion,
	}
}

// newID is a random identifier, as a UUID is spelled.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// string reads a string field of the content, empty when absent or not a string.
func (m Message) string(field string) string {
	s, _ := m.Content[field].(string)
	return s
}

// bool reads a boolean field of the content, false when absent or not one.
func (m Message) bool(field string) bool {
	b, _ := m.Content[field].(bool)
	return b
}

// int reads an integer field of the content, which JSON delivers as a float;
// absent or not a number, it reads as the fallback.
func (m Message) int(field string, fallback int) int {
	f, ok := m.Content[field].(float64)
	if !ok {
		return fallback
	}
	return int(f)
}
