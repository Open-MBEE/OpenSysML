package libs

import (
	"fmt"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// snapshotSource serves a Source's files as first read through it, so an index
// built from it and its text share one set of bytes; sealed, it serves only those.
type snapshotSource struct {
	inner  Source
	mu     sync.Mutex
	names  []string
	files  map[string]*snapshotFile
	sealed bool
}

type snapshotFile struct {
	content []byte
	err     error
	sf      *source.SourceFile // the content as a SourceFile, built on first text read
}

func newSnapshotSource(inner Source) *snapshotSource {
	return &snapshotSource{inner: inner, files: map[string]*snapshotFile{}}
}

func (s *snapshotSource) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.names == nil {
		s.names = append([]string{}, s.inner.List()...)
	}
	return append([]string(nil), s.names...)
}

func (s *snapshotSource) Read(name string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.fileLocked(name)
	return f.content, f.err
}

// text answers a span of the named file from the bytes the index was built
// over, "" for a name that is no file of the loaded library.
func (s *snapshotSource) text(doc string, span source.Span) string {
	s.mu.Lock()
	f := s.fileLocked(doc)
	if f.err == nil && f.sf == nil {
		f.sf = source.New(doc, f.content)
	}
	s.mu.Unlock()
	if f.err != nil {
		return ""
	}
	return f.sf.Text(span)
}

// fileLocked is the named file, read through on first use while unsealed.
// Caller holds the lock.
func (s *snapshotSource) fileLocked(name string) *snapshotFile {
	f, ok := s.files[name]
	if ok {
		return f
	}
	f = &snapshotFile{}
	if s.sealed {
		f.err = fmt.Errorf("libs: %q is not a file of the loaded library", name)
		return f
	}
	f.content, f.err = s.inner.Read(name)
	s.files[name] = f
	return f
}

// seal ends reading through to the underlying source.
func (s *snapshotSource) seal() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sealed = true
}
