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

// A bundled library file is highlighted whether the workspace holds a document
// under its name or has just let it go: both are looked up under one lock, so a
// close racing the request never answers "unknown". Run with -race.
func TestHighlightTokensSurviveClosingTheBundledName(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	own := []byte("package ScalarValues { datatype Real; }\n")

	// Each cycle reindexes the library, so a few cycles are all the readers get.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for version := 1; version <= 3; version++ {
			ws.Open(scalarValues, own, version)
			ws.Close(scalarValues)
		}
	}()

	const readers = 4
	var wg sync.WaitGroup
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for reads := 0; ; reads++ {
				select {
				case <-done:
					if reads >= 10 {
						return
					}
				default:
				}
				content, toks := ws.HighlightTokens(scalarValues)
				if !bytes.Equal(content, own) && !bytes.Equal(content, lib.Content) {
					t.Errorf("content of %d bytes is neither the workspace's nor the bundled file", len(content))
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
