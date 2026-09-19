package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// KerML notation (`namespace`, `connector from/to`, the `all` prefix), so the
// fixture is a `.kerml` source: `all` is not a SysML.xtext declaration prefix.
func TestConnectorReferencesKeyword(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "connector from/to with references",
			input: `
namespace Test {
    connector all MyConnector
        from [1] sourceFeature references sourceContext
        to [1] targetFeature references targetContext;
}`,
			wantErr: false,
		},
		{
			name: "connector with references on one end only",
			input: `
namespace Test {
    connector TestConn
        from [1] x references ctx
        to [1] y;
}`,
			wantErr: false,
		},
		{
			name: "connector connect with references",
			input: `
namespace Test {
    connector MyConnection connect [1] a references ctx to [1] b;
}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := source.New("test.kerml", []byte(tt.input))
			p := parser.New(src)
			_ = p.ParseFile()

			hasErrors := len(p.Diagnostics) > 0
			if hasErrors != tt.wantErr {
				t.Errorf("wantErr=%v but got %d diagnostics:", tt.wantErr, len(p.Diagnostics))
				for _, d := range p.Diagnostics {
					t.Logf("  %s", d.Message)
				}
			}
		})
	}
}
