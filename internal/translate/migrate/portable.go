package migrate

import (
	"bytes"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// openSysMLLibraryDir is the directory of the library source holding the
// OpenSysML extension libraries, one library package per file named after it.
const openSysMLLibraryDir = "OpenSysML Libraries/"

// LibrarySummary accounts for the OpenSysML library packages a portable
// migration appended to its output, and those it could not.
type LibrarySummary struct {
	// Inlined names the library packages appended, in the order appended.
	Inlined []string `json:"inlined,omitempty"`
	// NotInlined explains, by package, why a referenced library package was
	// not appended, so the output still refers to it.
	NotInlined map[string]string `json:"notInlined,omitempty"`
}

// libraryText is one OpenSysML library package as the source ships it.
type libraryText struct {
	name  string
	kerml bool
	text  []byte
}

// openSysMLLibraries reads the OpenSysML extension libraries from the
// embedded library source, by package name.
func openSysMLLibraries() map[string]libraryText {
	src := libs.EmbeddedSource()
	out := map[string]libraryText{}
	for _, path := range src.List() {
		if !strings.HasPrefix(path, openSysMLLibraryDir) {
			continue
		}
		base := strings.TrimPrefix(path, openSysMLLibraryDir)
		name, kerml := strings.TrimSuffix(base, ".sysml"), false
		if name == base {
			name, kerml = strings.TrimSuffix(base, ".kerml"), true
			if name == base {
				continue
			}
		}
		text, err := src.Read(path)
		if err != nil {
			continue
		}
		out[name] = libraryText{name: name, kerml: kerml, text: text}
	}
	return out
}

// inlineLibraries appends to notation the OpenSysML library packages it
// refers to, and those they refer to in turn, so the file loads with the
// standard library alone. A KerML library is written in its SysML spelling;
// one that has none is left referenced and accounted for.
func inlineLibraries(notation []byte) ([]byte, *LibrarySummary) {
	available := openSysMLLibraries()
	summary := &LibrarySummary{}
	seen := map[string]bool{}
	var appended [][]byte
	pending := referencedLibraries(notation, available)
	for len(pending) > 0 {
		name := pending[0]
		pending = pending[1:]
		if seen[name] {
			continue
		}
		seen[name] = true
		lib := available[name]
		text := lib.text
		if lib.kerml {
			spelled, ok := sysmlSpelling(name, text)
			if !ok {
				if summary.NotInlined == nil {
					summary.NotInlined = map[string]string{}
				}
				summary.NotInlined[name] = "a KerML library with no SysML spelling"
				continue
			}
			text = spelled
		}
		text = unmarkedStandard(name, text)
		summary.Inlined = append(summary.Inlined, name)
		appended = append(appended, text)
		pending = append(pending, referencedLibraries(text, available)...)
	}
	if len(appended) == 0 {
		return notation, summary
	}
	var out bytes.Buffer
	out.Write(notation)
	if !bytes.HasSuffix(notation, []byte("\n")) {
		out.WriteByte('\n')
	}
	out.WriteString("\n// OpenSysML library packages the model refers to, inlined so the file\n// loads with the standard library alone.\n")
	for _, text := range appended {
		out.WriteByte('\n')
		out.Write(text)
		if !bytes.HasSuffix(text, []byte("\n")) {
			out.WriteByte('\n')
		}
	}
	return out.Bytes(), summary
}

// referencedLibraries names the OpenSysML library packages notation refers
// to, each as the first segment of a qualified name, sorted.
func referencedLibraries(notation []byte, available map[string]libraryText) []string {
	found := map[string]bool{}
	lx := lexer.New(source.New("migration.sysml", notation))
	var prev lexer.Token
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.IsTrivia() {
			continue
		}
		if tok.Kind == lexer.ColonColon && prev.Kind == lexer.Identifier {
			if name := string(notation[prev.Span.Offset : prev.Span.Offset+prev.Span.Len]); available[name].text != nil {
				found[name] = true
			}
		}
		prev = tok
	}
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// unmarkedStandard is a library package declared `standard library package`
// without the `standard`, which only the standard library's own files may carry.
func unmarkedStandard(name string, text []byte) []byte {
	lx := lexer.New(source.New(name+".sysml", text))
	var prev lexer.Token
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.IsTrivia() {
			continue
		}
		if tok.Kind == lexer.Keyword && string(text[tok.Span.Offset:tok.Span.Offset+tok.Span.Len]) == "library" && prev.Kind == lexer.Keyword && string(text[prev.Span.Offset:prev.Span.Offset+prev.Span.Len]) == "standard" {
			out := append([]byte(nil), text[:prev.Span.Offset]...)
			return append(out, text[tok.Span.Offset:]...)
		}
		prev = tok
	}
	return text
}

// sysmlSpelling writes a KerML library in SysML notation: its functions as
// calc defs. ok is false when a word of it is KerML's alone, as `feature` or
// `featured by` are, so no SysML file can say what it says.
func sysmlSpelling(name string, kerml []byte) (spelled []byte, ok bool) {
	var out bytes.Buffer
	lx := lexer.New(source.New(name+".kerml", kerml))
	pos := 0
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Kind != lexer.Keyword {
			continue
		}
		end := tok.Span.Offset + tok.Span.Len
		word := string(kerml[tok.Span.Offset:end])
		switch {
		case word == "function":
			out.Write(kerml[pos:tok.Span.Offset])
			out.WriteString("calc def")
			pos = end
		case !source.IsKeywordIn(word, source.KindSysML):
			return nil, false
		}
	}
	out.Write(kerml[pos:])
	return out.Bytes(), true
}
