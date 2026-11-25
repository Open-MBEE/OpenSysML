package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// nonstandardEscapes returns the escapes a document spells beyond KerML §8.2.2.3 Table 4.
func nonstandardEscapes(name string, text []byte) []string {
	src := source.New(name, text)
	var found []string
	lx := lexer.New(src)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if !tok.NonstandardEscape {
			continue
		}
		for _, sp := range lexer.NonstandardEscapes(tok.Span, src.Text(tok.Span)) {
			found = append(found, src.Text(sp))
		}
	}
	return found
}

// The OMG corpora and the standard library are standard notation, so none
// spells an escape outside Table 4; a twin that does proves the scan can see one.
func TestOMGCorporaSpellNoNonstandardEscape(t *testing.T) {
	if got := nonstandardEscapes("a.sysml", []byte(`package P { part def 'a\rb'; attribute s = "\t\r"; }`)); len(got) != 2 {
		t.Fatalf("the scan found %v in the positive twin, want its two `\\r`", got)
	}

	scan := func(t *testing.T, dir string, files []string) {
		t.Helper()
		if len(files) == 0 {
			t.Fatalf("%s holds no model file: the scan would prove nothing", dir)
		}
		for _, rel := range files {
			text, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			if got := nonstandardEscapes(rel, text); len(got) != 0 {
				t.Errorf("%s/%s spells %v, which KerML §8.2.2.3 Table 4 does not list", dir, rel, got)
			}
		}
	}

	t.Run("stdlib", func(t *testing.T) {
		const dir = "../../internal/workspace/libs/stdlib"
		var files []string
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			if strings.HasSuffix(path, ".sysml") || strings.HasSuffix(path, ".kerml") {
				files = append(files, filepath.ToSlash(strings.TrimPrefix(path, dir+string(filepath.Separator))))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		scan(t, dir, files)
	})
	for _, gate := range []corpusGate{trainingGate, pilotCorporaGate, specExamplesGate} {
		t.Run(gate.name, func(t *testing.T) {
			files := gate.files(t)
			for _, root := range gate.roots {
				scan(t, root.dir, files[root.name])
			}
		})
	}
}
