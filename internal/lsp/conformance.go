package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// strictConformanceKey is the setting an editor sets to ask the strict question,
// spelled as the CLI's -strict flag and the REPL's %strict command.
const strictConformanceKey = "strictConformance"

// settingsSection is the section an editor nests this server's settings under.
const settingsSection = "sysml"

// applyConformanceSettings switches the workspace's conformance mode to what a
// settings payload asks for, and reports whether it said anything: a payload
// without the setting leaves the mode alone rather than resetting it.
func (s *Server) applyConformanceSettings(payload any) bool {
	strict, ok := strictConformanceSetting(payload)
	if !ok {
		return false
	}
	s.ws.SetConformanceMode(diag.ConformanceModeOf(strict))
	return true
}

// DidChangeConfiguration applies the settings the client pushed. Only the
// conformance mode is read; a payload that does not mention it changes nothing.
func (s *Server) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) error {
	if params == nil || !s.applyConformanceSettings(params.Settings) {
		return nil
	}
	s.republishOpenDiagnostics(ctx)
	return nil
}

// strictConformanceSetting reads the strict-conformance flag out of an
// initializationOptions or didChangeConfiguration payload. Clients nest their
// settings differently, so all three shapes are accepted:
// {"strictConformance": true}, {"sysml": {"strictConformance": true}} and the
// flat {"sysml.strictConformance": true}.
func strictConformanceSetting(payload any) (bool, bool) {
	settings, ok := payload.(map[string]any)
	if !ok {
		return false, false
	}
	if value, ok := settings[strictConformanceKey]; ok {
		return boolSetting(value)
	}
	if value, ok := settings[settingsSection+"."+strictConformanceKey]; ok {
		return boolSetting(value)
	}
	if nested, ok := settings[settingsSection].(map[string]any); ok {
		if value, ok := nested[strictConformanceKey]; ok {
			return boolSetting(value)
		}
	}
	return false, false
}

// boolSetting reads a JSON boolean, ignoring a value of any other type: a
// malformed setting is not a reason to answer a different question.
func boolSetting(value any) (bool, bool) {
	strict, ok := value.(bool)
	return strict, ok
}

// republishOpenDiagnostics re-analyzes every open document and pushes the
// result, so a setting that changes what counts as an error is answered without
// waiting for the next keystroke.
func (s *Server) republishOpenDiagnostics(ctx context.Context) {
	for _, name := range s.ws.OpenNames() {
		s.publishOpenDiagnostics(ctx, name)
	}
}
