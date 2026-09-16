package model

import (
	"bytes"
	"sync"
	"testing"
)

// Tokens and the content they index are read under one lock, so a concurrent
// edit never pairs one revision's tokens with another's text. Run with -race.
func TestHighlightTokensIndexTheContentTheyComeWith(t *testing.T) {
	ws := NewWorkspace()
	const name = "flip.sysml"
	revisions := [][]byte{
		[]byte("package P { part def Alpha; part a : Alpha; }\n"),
		[]byte("package LongerName { part def Bravo; part def Charlie; part b : Bravo; }\n"),
	}
	ws.Open(name, revisions[0], 1)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for version := 2; ; version++ {
			select {
			case <-stop:
				return
			default:
			}
			ws.Update(name, revisions[(version-1)%2], version)
		}
	}()
	defer func() { close(stop); <-done }()

	const readers = 4
	var wg sync.WaitGroup
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				content, toks := ws.HighlightTokens(name)
				if !bytes.Equal(content, revisions[0]) && !bytes.Equal(content, revisions[1]) {
					t.Errorf("content %q is no revision", content)
				}
				if len(toks) == 0 {
					t.Error("no tokens for a document with declarations")
				}
				for _, tok := range toks {
					if tok.Span.Offset < 0 || tok.Span.End() > len(content) {
						t.Errorf("token %v outside content of %d bytes", tok.Span, len(content))
					}
				}
			}
		}()
	}
	wg.Wait()
}

func TestHighlightTokensUnknownDocument(t *testing.T) {
	if content, toks := NewWorkspace().HighlightTokens("missing.sysml"); content != nil || toks != nil {
		t.Errorf("HighlightTokens(missing) = %q, %v; want nil, nil", content, toks)
	}
}
