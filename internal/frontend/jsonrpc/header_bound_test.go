package jsonrpc

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

func TestReadHeaderRefusesHeadersOverTheBound(t *testing.T) {
	for name, input := range map[string]string{
		"one long line":    "X-Pad: " + strings.Repeat("a", MaxHeaderBytes) + "\r\n\r\n",
		"many short lines": strings.Repeat("X-Pad: "+strings.Repeat("a", 1000)+"\r\n", 100) + "\r\n",
		"folded line":      "X-Pad: a\r\n" + strings.Repeat(" "+strings.Repeat("a", 1000)+"\r\n", 100) + "\r\n",
	} {
		if _, err := ReadHeader(bufio.NewReader(strings.NewReader(input))); !errors.Is(err, errMessageTooLarge) {
			t.Errorf("%s: err = %v, want errMessageTooLarge", name, err)
		}
	}
}

func TestReadHeaderAcceptsAFoldedHeaderUnderTheBound(t *testing.T) {
	first, rest := strings.Repeat("a", MaxHeaderBytes/2+1000), strings.Repeat("b", 1000)
	input := "X-Pad: " + first + "\r\n " + rest + "\r\n\r\n"
	header, err := ReadHeader(bufio.NewReader(strings.NewReader(input)))
	if err != nil {
		t.Fatal(err)
	}
	if got := header["X-Pad"]; len(got) != 1 || got[0] != first+" "+rest {
		t.Errorf("X-Pad holds %d values; the first line or its continuation was lost", len(got))
	}
}

func TestReadHeaderAcceptsHeadersUnderTheBound(t *testing.T) {
	input := "Content-Length: 2\r\nX-Pad: " + strings.Repeat("a", MaxHeaderBytes/2) + "\r\n\r\n{}"
	header, err := ReadHeader(bufio.NewReader(strings.NewReader(input)))
	if err != nil {
		t.Fatal(err)
	}
	if got := header["Content-Length"]; len(got) != 1 || got[0] != "2" {
		t.Errorf("Content-Length = %v", got)
	}
}
