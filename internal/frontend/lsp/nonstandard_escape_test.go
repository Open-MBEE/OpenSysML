package lsp

import (
	"context"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"go.lsp.dev/protocol"
)

// lspNonstandardEscape spells `\r` in a name on line 0 and in a string on line 1.
const lspNonstandardEscape = "package P { part def 'a\\rb';\n  attribute s = \"c\\rd\"; attribute t = \"\\t\\n\"; }"

// The editor is published one nonstandard-notation warning per `\r`, ranged
// over the escape, and the same two as errors once it asks strictly.
func TestNonstandardEscapeIsPublishedAndEscalates(t *testing.T) {
	for _, name := range []string{"a.sysml", "a.kerml"} {
		t.Run(name, func(t *testing.T) {
			text := lspNonstandardEscape
			if name == "a.kerml" {
				text = strings.NewReplacer("part def", "class", "attribute", "feature").Replace(text)
			}
			ws := model.NewWorkspace()
			s := NewServer(ws)
			fc := &fakeClient{}
			s.client = fc
			ws.Open(name, []byte(text), 1)

			check := func(severity protocol.DiagnosticSeverity) {
				t.Helper()
				published := fc.all()
				if len(published) == 0 {
					t.Fatal("nothing published")
				}
				got := published[len(published)-1].Diagnostics
				if len(got) != 2 {
					t.Fatalf("published %+v, want the two `\\r` findings", got)
				}
				lines := strings.Split(text, "\n")
				for i, d := range got {
					if diagnosticCode(d) != passes.CodeNonstandardNotation || d.Severity != severity {
						t.Errorf("diagnostic %+v, want %s at severity %v", d, passes.CodeNonstandardNotation, severity)
					}
					r := d.Range
					if int(r.Start.Line) != i || r.End.Line != r.Start.Line || lines[i][r.Start.Character:r.End.Character] != `\r` {
						t.Errorf("range %+v on line %d, want the escape `\\r`", r, i)
					}
					if !strings.Contains(diagnosticMessage(t, d), "Table 4") {
						t.Errorf("message = %q, want it to cite Table 4", diagnosticMessage(t, d))
					}
				}
			}

			s.publishDiagnostics(context.Background(), name)
			check(protocol.DiagnosticSeverityWarning)
			if err := s.DidChangeConfiguration(context.Background(), &protocol.DidChangeConfigurationParams{
				Settings: mustLSPAny(t, map[string]any{"sysml": map[string]any{"strictConformance": true}}),
			}); err != nil {
				t.Fatal(err)
			}
			check(protocol.DiagnosticSeverityError)
		})
	}
}
