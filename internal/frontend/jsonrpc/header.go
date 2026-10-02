// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.
// Portions derived from Go's net/textproto, Copyright 2010 The Go Authors,
// used under the BSD-style license at https://go.dev/LICENSE.

package jsonrpc

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
)

// This file reimplements net/textproto.Reader.ReadMIMEHeader over a
// *bufio.Reader, accepting and rejecting exactly the inputs textproto does and
// answering with the same values and error strings, so frame headers are read
// without linking the networking packages a WebAssembly build cannot use. The
// logic is Go's own, trimmed of the size limits, the dot reader and the
// common-header lookup, none of which changes an answer.

// mimeHeader is textproto.MIMEHeader: canonical keys to their values in order.
type mimeHeader map[string][]string

// headerGet is MIMEHeader.Get: the first value under the canonical key.
func headerGet(h mimeHeader, key string) string {
	if h == nil {
		return ""
	}
	if v := h[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// protocolError is textproto.ProtocolError: a protocol violation's message.
type protocolError string

func (p protocolError) Error() string { return string(p) }

// headerReader is textproto.Reader reduced to what ReadMIMEHeader needs.
type headerReader struct {
	r   *bufio.Reader
	buf []byte
}

// readMIMEHeader is textproto's ReadMIMEHeader: headers each a continued line,
// ended by a blank line, with the same acceptance of CRLF and bare LF, the same
// folded continuations and the same rejections.
func readMIMEHeader(r *bufio.Reader) (mimeHeader, error) {
	hr := &headerReader{r: r}
	m := make(mimeHeader, hr.upcomingHeaderKeys())

	// The first line cannot start with a leading space.
	if buf, err := r.Peek(1); err == nil && (buf[0] == ' ' || buf[0] == '\t') {
		const errorLimit = 80 // arbitrary limit on how much of the line we'll quote
		line, err := hr.readLineSlice(errorLimit)
		if err != nil {
			return m, err
		}
		return m, protocolError(fmt.Sprintf("malformed MIME header initial line: %q", line))
	}

	for {
		kv, err := hr.readContinuedLineSlice(math.MaxInt64, mustHaveFieldNameColon)
		if len(kv) == 0 {
			return m, err
		}

		// Key ends at first colon.
		k, v, ok := bytes.Cut(kv, colonBytes)
		if !ok {
			return m, protocolError(fmt.Sprintf("malformed MIME header line: %q", kv))
		}
		key, ok := canonicalMIMEHeaderKey(k)
		if !ok {
			return m, protocolError(fmt.Sprintf("malformed MIME header line: %q", kv))
		}
		for _, c := range v {
			if !validHeaderValueByte(c) {
				return m, protocolError(fmt.Sprintf("malformed MIME header line: %q", kv))
			}
		}

		// Skip initial spaces in value.
		value := string(bytes.TrimLeft(v, " \t"))
		m[key] = append(m[key], value)

		if err != nil {
			return m, err
		}
	}
}

// readLineSlice reads a single line, eliding the final \n or \r\n.
func (r *headerReader) readLineSlice(lim int64) ([]byte, error) {
	var line []byte
	for {
		l, more, err := r.r.ReadLine()
		if err != nil {
			return nil, err
		}
		if lim >= 0 && int64(len(line))+int64(len(l)) > lim {
			return nil, errMessageTooLarge
		}
		// Avoid the copy if the first call produced a full line.
		if line == nil && !more {
			return l, nil
		}
		line = append(line, l...)
		if !more {
			break
		}
	}
	return line, nil
}

// readContinuedLineSlice reads a line and its folded continuations, trimming
// each line's trailing whitespace and joining them with single spaces.
func (r *headerReader) readContinuedLineSlice(lim int64, validateFirstLine func([]byte) error) ([]byte, error) {
	if validateFirstLine == nil {
		return nil, fmt.Errorf("missing validateFirstLine func")
	}

	// Read the first line.
	line, err := r.readLineSlice(lim)
	if err != nil {
		return nil, err
	}
	if len(line) == 0 { // blank line - no continuation
		return line, nil
	}

	if err := validateFirstLine(line); err != nil {
		return nil, err
	}

	// Optimistically assume that we have started to buffer the next line
	// and it starts with an ASCII letter (the next header key), or a blank
	// line, so we can avoid copying that buffered data around in memory
	// and skipping over non-existent whitespace.
	if r.r.Buffered() > 1 {
		peek, _ := r.r.Peek(2)
		if len(peek) > 0 && (isASCIILetter(peek[0]) || peek[0] == '\n') ||
			len(peek) == 2 && peek[0] == '\r' && peek[1] == '\n' {
			return trimBytes(line), nil
		}
	}

	// ReadByte or the next readLineSlice will flush the read buffer;
	// copy the slice into buf.
	r.buf = append(r.buf[:0], trimBytes(line)...)

	if lim < 0 {
		lim = math.MaxInt64
	}
	lim -= int64(len(r.buf))

	// Read continuation lines.
	for r.skipSpace() > 0 {
		r.buf = append(r.buf, ' ')
		if int64(len(r.buf)) >= lim {
			return nil, errMessageTooLarge
		}
		line, err := r.readLineSlice(lim - int64(len(r.buf)))
		if err != nil {
			break
		}
		r.buf = append(r.buf, trimBytes(line)...)
	}
	return r.buf, nil
}

// skipSpace skips over all spaces and returns the number of bytes skipped.
func (r *headerReader) skipSpace() int {
	n := 0
	for {
		c, err := r.r.ReadByte()
		if err != nil {
			// Bufio will keep err until next read.
			break
		}
		if c != ' ' && c != '\t' {
			_ = r.r.UnreadByte()
			break
		}
		n++
	}
	return n
}

var errMessageTooLarge = fmt.Errorf("message too large")

var colonBytes = []byte(":")

// mustHaveFieldNameColon ensures that, per RFC 7230, the field-name is on a
// single line, so the first line must contain a colon.
func mustHaveFieldNameColon(line []byte) error {
	if bytes.IndexByte(line, ':') < 0 {
		return protocolError(fmt.Sprintf("malformed MIME header: missing colon: %q", line))
	}
	return nil
}

// trimBytes returns s with leading and trailing spaces and tabs removed.
func trimBytes(s []byte) []byte {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	n := len(s)
	for n > i && (s[n-1] == ' ' || s[n-1] == '\t') {
		n--
	}
	return s[i:n]
}

var newlineBytes = []byte("\n")

// upcomingHeaderKeys returns an approximation of the number of keys the header
// will carry, the hint textproto sizes its map by; confused input yields 0.
func (r *headerReader) upcomingHeaderKeys() (n int) {
	// Try to determine the 'hint' size.
	_, _ = r.r.Peek(1) // force a buffer load if empty
	s := r.r.Buffered()
	if s == 0 {
		return
	}
	peek, _ := r.r.Peek(s)
	for len(peek) > 0 && n < 1000 {
		var line []byte
		line, peek, _ = bytes.Cut(peek, newlineBytes)
		if len(line) == 0 || (len(line) == 1 && line[0] == '\r') {
			// Blank line separating headers from the body.
			break
		}
		if line[0] == ' ' || line[0] == '\t' {
			// Folded continuation of the previous line.
			continue
		}
		n++
	}
	return n
}

// canonicalMIMEHeaderKey is textproto's unexported equivalent: the key with its
// first letter and each letter after a dash upper-cased, the rest lowered; a
// key holding a space or an invalid byte is not canonicalized, and one holding
// an invalid byte is rejected.
func canonicalMIMEHeaderKey(a []byte) (_ string, ok bool) {
	if len(a) == 0 {
		return "", false
	}

	// See if a looks like a header key. If not, return it unchanged.
	noCanon := false
	for _, c := range a {
		if validHeaderFieldByte(c) {
			continue
		}
		// Don't canonicalize.
		if c == ' ' {
			// We accept invalid headers with a space before the
			// colon, but must not canonicalize them.
			// See https://go.dev/issue/34540.
			noCanon = true
			continue
		}
		return string(a), false
	}
	if noCanon {
		return string(a), true
	}

	upper := true
	out := make([]byte, len(a))
	for i, c := range a {
		// Canonicalize: first letter upper case
		// and upper case after each dash.
		// (Host, User-Agent, If-Modified-Since).
		// MIME headers are ASCII only, so no Unicode issues.
		if upper && 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		} else if !upper && 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
		upper = c == '-' // for next time
	}
	return string(out), true
}

// validHeaderFieldByte reports whether c is a valid byte in a header field
// name per RFC 7230: header-field = field-name ":" OWS field-value OWS, where
// field-name is a token of the RFC 7230 tchar bytes.
func validHeaderFieldByte(c byte) bool {
	// mask is a 128-bit bitmap with 1s for allowed bytes,
	// so that the byte c can be tested with a shift and an and.
	// If c >= 128, then 1<<c and 1<<(c-64) will both be zero,
	// and this function will return false.
	const mask = 0 |
		(1<<(10)-1)<<'0' |
		(1<<(26)-1)<<'a' |
		(1<<(26)-1)<<'A' |
		1<<'!' |
		1<<'#' |
		1<<'$' |
		1<<'%' |
		1<<'&' |
		1<<'\'' |
		1<<'*' |
		1<<'+' |
		1<<'-' |
		1<<'.' |
		1<<'^' |
		1<<'_' |
		1<<'`' |
		1<<'|' |
		1<<'~'
	return ((uint64(1)<<c)&(mask&(1<<64-1)) |
		(uint64(1)<<(c-64))&(mask>>64)) != 0
}

// validHeaderValueByte reports whether c is a valid byte in a header field
// value per RFC 7230 and RFC 5234: VCHAR (%x21-7E), obs-text (%x80-FF), SP and
// HTAB.
func validHeaderValueByte(c byte) bool {
	// mask is a 128-bit bitmap with 1s for allowed bytes; here the obs-text
	// range inverts it into 1s for disallowed bytes.
	const mask = 0 |
		(1<<(0x7f-0x21)-1)<<0x21 | // VCHAR: %x21-7E
		1<<0x20 | // SP: %x20
		1<<0x09 // HTAB: %x09
	return ((uint64(1)<<c)&^(mask&(1<<64-1)) |
		(uint64(1)<<(c-64))&^(mask>>64)) == 0
}

func isASCIILetter(b byte) bool {
	b |= 0x20 // make lower case
	return 'a' <= b && b <= 'z'
}
