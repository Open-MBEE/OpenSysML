// Package source owns file content and byte-offset span types.
package source

import (
	"path/filepath"
	"strings"
	"sync"
)

// Span is a byte range within a SourceFile: [Offset, Offset+Len).
type Span struct {
	Offset int
	Len    int
}

// End returns the exclusive end offset.
func (s Span) End() int { return s.Offset + s.Len }

// Contains reports whether other lies within s.
func (s Span) Contains(other Span) bool {
	return other.Offset >= s.Offset && other.End() <= s.End()
}

// Pos is a 1-based line/column location.
type Pos struct {
	Line int
	Col  int
}

// Lookup answers the text a span of the named document covers, or "" for a
// document it does not hold.
type Lookup func(doc string, span Span) string

// TextOf answers spans from files by document name, shadowing next; a name not
// among files goes to next, "" when next is nil.
func TextOf(files map[string]*SourceFile, next Lookup) Lookup {
	return func(doc string, span Span) string {
		if sf, ok := files[doc]; ok {
			return sf.Text(span)
		}
		if next == nil {
			return ""
		}
		return next(doc, span)
	}
}

// SourceFile owns the raw bytes of one source file.
type SourceFile struct {
	name    string
	content []byte
	kind    Kind
	lines   sync.Once
	index   *LineIndex
	text    sync.Once
	whole   string
}

// New creates a SourceFile from a name and its raw bytes.
func New(name string, content []byte) *SourceFile {
	return &SourceFile{name: name, content: content, kind: KindOf(name)}
}

// NewWithKind creates a source file whose language is explicit rather than
// inferred from its name. This is used for inline KerML content.
func NewWithKind(name string, content []byte, kind Kind) *SourceFile {
	return &SourceFile{name: name, content: content, kind: kind}
}

// Name returns the file name.
func (sf *SourceFile) Name() string { return sf.name }

// Dir is the directory of a source file's name when the name is a path on
// disk; "" for a name that is no file: empty, a `<placeholder>` such as
// standard input's or the REPL's, or a URI.
func Dir(name string) string {
	if name == "" || strings.HasPrefix(name, "<") || strings.Contains(name, "://") {
		return ""
	}
	return filepath.Dir(name)
}

// Len returns the byte length of the content.
func (sf *SourceFile) Len() int { return len(sf.content) }

// Bytes returns the raw content (do not mutate).
func (sf *SourceFile) Bytes() []byte { return sf.content }

// Text returns the substring covered by the span. Spans are taken from one
// cached copy of the content, so a span costs no allocation of its own.
func (sf *SourceFile) Text(sp Span) string {
	sf.text.Do(func() {
		sf.whole = string(sf.content)
	})
	return sf.whole[sp.Offset:sp.End()]
}

// Lines returns the cached line index for this file, building it on first use.
// Col is a byte column (1-based). LSP requires UTF-16 code-unit columns; that
// conversion is an LSP-layer concern (Plan 06), not the source package.
func (sf *SourceFile) Lines() *LineIndex {
	sf.lines.Do(func() {
		sf.index = NewLineIndex(sf.content)
	})
	return sf.index
}
