package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Contextual words are syntax the lexer does not reserve, so completion has to
// offer them from the second list or they are never offered at all.
func TestCompletionOffersContextualWords(t *testing.T) {
	for _, tc := range []struct {
		file string
		kind source.Kind
	}{
		{"/tmp/f9_completion.sysml", source.KindSysML},
		{"/tmp/f9_completion.kerml", source.KindKerML},
	} {
		items := f9CompletionAt(t, tc.file, "package P { }\n")
		for _, w := range lexer.ContextualWords(tc.kind) {
			item, ok := items[w]
			if !ok {
				t.Errorf("%s: completion does not offer contextual word %q", tc.file, w)
				continue
			}
			if item.Kind != protocol.CompletionItemKindKeyword {
				t.Errorf("%s: %q kind = %v, want Keyword", tc.file, w, item.Kind)
			}
			if item.Detail != "contextual keyword" {
				t.Errorf("%s: %q detail = %q, want \"contextual keyword\"", tc.file, w, item.Detail)
			}
		}
	}
}

// `var` is a KerML literal and no SysML production, so a `.sysml` document must
// not be offered it while a `.kerml` one is.
func TestCompletionOffersVarInKerMLOnly(t *testing.T) {
	if _, ok := f9CompletionAt(t, "/tmp/f9_var.sysml", "package P { }\n")["var"]; ok {
		t.Error("a .sysml document is offered `var`, which no SysML production takes")
	}
	if _, ok := f9CompletionAt(t, "/tmp/f9_var.kerml", "package P { }\n")["var"]; !ok {
		t.Error("a .kerml document is not offered `var`")
	}
}

// The regression the second list exists to avoid: a contextual word must stay
// usable as an ordinary name, which reserving it would break.
func TestContextualWordsRemainUsableAsNames(t *testing.T) {
	src := `package P {
	attribute point : ScalarValues::Real;
	attribute region : ScalarValues::Real;
	attribute defer : ScalarValues::Real;
	attribute chain : ScalarValues::Real;
	part def Sensor { attribute initial : ScalarValues::Real; }
	part sensor : Sensor { attribute redefines initial = 1.0; }
}
`
	ws := model.NewWorkspace()
	name := uri.File("/tmp/f9_names.sysml").Filename()
	ws.Open(name, []byte(src), 1)
	_, diags, ok := ws.AnalyzedContent(name)
	if !ok {
		t.Fatal("document not analysed")
	}
	for _, d := range diags {
		t.Errorf("naming a contextual word diagnosed: %s", d.Message)
	}
	// And the words are still offered as syntax, the declarations above winning
	// the label where they collide.
	if _, ok := f9CompletionAt(t, "/tmp/f9_names2.sysml", src)["defer"]; !ok {
		t.Error("`defer` is not offered at all")
	}
}

// f9CompletionAt opens a document and returns the completion items at its end,
// keyed by label.
func f9CompletionAt(t *testing.T, path, src string) map[string]protocol.CompletionItem {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File(path).Filename()
	ws.Open(name, []byte(src), 1)

	list, err := s.Completion(context.Background(), &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), len(src)),
		},
	})
	if err != nil {
		t.Fatalf("Completion(%s) err = %v", path, err)
	}
	if list == nil {
		t.Fatalf("Completion(%s) returned nil list", path)
	}
	out := make(map[string]protocol.CompletionItem, len(list.Items))
	for _, it := range list.Items {
		out[it.Label] = it
	}
	return out
}
