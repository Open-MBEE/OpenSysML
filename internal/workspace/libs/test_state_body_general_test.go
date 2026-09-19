package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestStateBodyGeneralMembers verifies that state bodies can contain
// general body members (binding, features, connectors), not just state-specific keywords
func TestStateBodyGeneralMembers(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "state with binding",
			input: `
namespace Test {
    state MyState {
        binding bind payload = accepter.payload;
    }
}`,
			wantErr: false,
		},
		{
			name: "state with feature declaration",
			input: `
namespace Test {
    state MyState {
        feature myFeature: String;
    }
}`,
			wantErr: false,
		},
		{
			name: "state with connector",
			input: `
namespace Test {
    state TestState {
        connector myConn from [1] a to [1] b;
    }
}`,
			wantErr: false,
		},
		{
			name: "state with attribute",
			input: `
namespace Test {
    state TestState {
        attribute count: Integer;
    }
}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := source.New("test.sysml", []byte(tt.input))
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
