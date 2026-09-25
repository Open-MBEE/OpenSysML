package libs

import (
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Text answers spans of the library files src serves, by the names an index
// built from src holds them under; nil for a nil src. Each file is read once.
func Text(src Source) source.Lookup {
	switch s := src.(type) {
	case nil:
		return nil
	case *snapshotSource:
		return s.text
	default:
		return newLibraryText(src).text
	}
}

// libraryText reads a Source's files as SourceFiles, each once.
type libraryText struct {
	src   Source
	mu    sync.Mutex
	files map[string]*source.SourceFile // nil for a name the source has no file of
}

func newLibraryText(src Source) *libraryText {
	return &libraryText{src: src, files: map[string]*source.SourceFile{}}
}

func (t *libraryText) text(doc string, span source.Span) string {
	t.mu.Lock()
	sf, ok := t.files[doc]
	if !ok {
		if content, err := t.src.Read(doc); err == nil {
			sf = source.New(doc, content)
		}
		t.files[doc] = sf
	}
	t.mu.Unlock()
	if sf == nil {
		return ""
	}
	return sf.Text(span)
}
