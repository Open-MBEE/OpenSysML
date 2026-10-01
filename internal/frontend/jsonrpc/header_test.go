// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package jsonrpc

import (
	"bufio"
	"io"
	"net/textproto"
	"strings"
	"testing"
)

// headerBlocks are header blocks each followed by a body marker, covering what
// a frame reader may meet: case variation, duplicates, whitespace, folded
// continuations, malformed lines and odd line endings.
var headerBlocks = []struct {
	name  string
	block string
}{
	{"empty input", ""},
	{"only a body", "the body"},
	{"blank line only", "\r\n"},
	{"blank line only, bare LF", "\n"},
	{"one header, CRLF", "Content-Length: 3\r\n\r\nabc"},
	{"one header, bare LF", "Content-Length: 3\n\nabc"},
	{"no terminating blank line", "Content-Length: 3\r\nContent-Type: application/json\r\n"},
	{"lower-case keys", "content-length: 3\r\ncontent-type: application/json\r\n\r\nabc"},
	{"mixed-case keys", "cOnTeNt-LeNgTh: 3\r\nCoNtEnT-tYpE: text/plain\r\n\r\nabc"},
	{"duplicate keys, first wins", "Content-Length: 3\r\nContent-Length: 99\r\n\r\nabc"},
	{"value with surrounding whitespace", "Content-Length:   3  \r\n\r\nabc"},
	{"value with a tab", "Content-Length:\t3\r\n\r\nabc"},
	{"no space after colon", "Content-Length:3\r\n\r\nabc"},
	{"folded continuation", "Content-Type: application/\r\n json\r\nContent-Length: 3\r\n\r\nabc"},
	{"folded continuation with tabs", "Content-Type: application/\r\n\tjson\r\nContent-Length: 3\r\n\r\nabc"},
	{"missing colon", "Content-Length 3\r\n\r\nabc"},
	{"space before colon", "Content-Length : 3\r\n\r\nabc"},
	{"leading-space first line", " Content-Length: 3\r\n\r\nabc"},
	{"leading-tab first line", "\tContent-Length: 3\r\n\r\nabc"},
	{"empty key", ": 3\r\n\r\nabc"},
	{"invalid key byte", "Content\x01Length: 3\r\n\r\nabc"},
	{"invalid value byte", "Content-Length: 3\x00\r\n\r\nabc"},
	{"key of dashes", "Content--Length: 3\r\n\r\nabc"},
	{"CR-only line ending", "Content-Length: 3\r\rabc"},
	{"CR-only between headers", "Content-Length: 3\rContent-Type: application/json\r\rabc"},
	{"colon-less then headers", "garbage\r\nContent-Length: 3\r\n\r\nabc"},
	{"extra headers around the two", "X-One: a\r\nContent-Length: 3\r\nX-Two: b\r\nContent-Type: application/json; charset=utf-8\r\n\r\nabc"},
	{"long folded value", "Content-Type: application/json;\r\n charset=utf-8;\r\n boundary=x\r\nContent-Length: 3\r\n\r\nabc"},
	{"continued first line without colon", "Content-Length\r\n : 3\r\n\r\nabc"},
	{"second line missing colon", "Content-Length: 3\r\nGarbage\r\n\r\nabc"},
	{"unicode in key", "Contént-Length: 3\r\n\r\nabc"},
	{"blank line mid-block ends it", "Content-Type: x\r\n\r\nContent-Length: 3\r\n\r\nabc"},
}

// The private reader must answer exactly what net/textproto's ReadMIMEHeader
// answers for every block: the same values, the same error text, and the same
// bytes left for the body.
func TestReadMIMEHeaderMatchesTextproto(t *testing.T) {
	for _, tc := range headerBlocks {
		t.Run(tc.name, func(t *testing.T) {
			mine := bufio.NewReader(strings.NewReader(tc.block))
			gotHeader, gotErr := readMIMEHeader(mine)
			gotRest, _ := readRest(mine)

			ref := bufio.NewReader(strings.NewReader(tc.block))
			wantHeader, wantErr := textproto.NewReader(ref).ReadMIMEHeader()
			wantRest, _ := readRest(ref)

			errText := func(err error) string {
				if err == nil {
					return ""
				}
				return err.Error()
			}
			if errText(gotErr) != errText(wantErr) {
				t.Errorf("err = %v, want %v", gotErr, wantErr)
			}
			for _, key := range []string{"Content-Length", "Content-Type"} {
				if got, want := headerGet(gotHeader, key), wantHeader.Get(key); got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
			if len(gotHeader) != len(wantHeader) {
				t.Errorf("header = %v, want %v", gotHeader, wantHeader)
			}
			for key, want := range wantHeader {
				if got, ok := gotHeader[key]; !ok || !equalStrings(got, want) {
					t.Errorf("header[%q] = %v, want %v", key, got, want)
				}
			}
			if gotRest != wantRest {
				t.Errorf("remaining bytes = %q, want %q", gotRest, wantRest)
			}
		})
	}
}

func readRest(r *bufio.Reader) (string, error) {
	data, err := io.ReadAll(r)
	return string(data), err
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
