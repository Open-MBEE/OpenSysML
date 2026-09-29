package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// strictConformanceKey is the setting an editor sets to ask the strict question,
// spelled as the CLI's -strict flag and the REPL's %strict command.
const strictConformanceKey = "strictConformance"

// disabledLintsKey is the setting naming the lints an editor leaves out of the
// diagnostics, spelled as the CLI's -disable-lint flag.
const disabledLintsKey = "disabledLints"

// settingsSection is the section an editor nests this server's settings under.
const settingsSection = "sysml"

// applyConformanceSettings switches the workspace's conformance mode to what a
// settings payload asks for, and reports whether the mode changed: a payload
// without the setting leaves the mode alone rather than resetting it, and a
// workspace that cannot answer the new mode says why to the client.
func (s *Server) applyConformanceSettings(ctx context.Context, payload any) bool {
	strict, ok := strictConformanceSetting(payload)
	if !ok {
		return false
	}
	if err := s.ws.SetConformanceMode(diag.ConformanceModeOf(strict)); err != nil {
		if s.client != nil {
			_ = s.client.ShowMessage(ctx, &protocol.ShowMessageParams{Type: protocol.MessageTypeError, Message: err.Error()})
		}
		return false
	}
	return true
}

// DidChangeConfiguration applies the settings the client pushed. Only the
// conformance mode and the disabled lints are read; a payload that mentions
// neither changes nothing.
func (s *Server) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) error {
	if params == nil {
		return nil
	}
	modeChanged := s.applyConformanceSettings(ctx, params.Settings)
	lintsChanged := s.applyLintSettings(ctx, params.Settings)
	if !modeChanged && !lintsChanged {
		return nil
	}
	s.republishOpenDiagnostics(ctx)
	return nil
}

// applyLintSettings replaces the lints the workspace leaves out with the ones a
// settings payload names, and reports whether it named any setting: a payload
// without it leaves the lints alone, and an unknown code is shown the client.
func (s *Server) applyLintSettings(ctx context.Context, payload any) bool {
	value, ok := sysmlSetting(payload, disabledLintsKey)
	if !ok {
		return false
	}
	list, ok := value.([]any)
	if !ok {
		return false
	}
	codes := make([]string, 0, len(list))
	for _, item := range list {
		code, isString := item.(string)
		if !isString {
			return false
		}
		codes = append(codes, code)
	}
	if err := s.ws.SetDisabledLints(codes); err != nil {
		if s.client != nil {
			_ = s.client.ShowMessage(ctx, &protocol.ShowMessageParams{Type: protocol.MessageTypeError, Message: err.Error()})
		}
		return false
	}
	return true
}

// strictConformanceSetting reads the strict-conformance flag out of an
// initializationOptions or didChangeConfiguration payload. Clients nest their
// settings differently, so all three shapes are accepted:
// {"strictConformance": true}, {"sysml": {"strictConformance": true}} and the
// flat {"sysml.strictConformance": true}.
func strictConformanceSetting(payload any) (bool, bool) {
	value, ok := sysmlSetting(payload, strictConformanceKey)
	if !ok {
		return false, false
	}
	return boolSetting(value)
}

// sysmlSetting reads the setting key out of a payload in any of the three shapes.
func sysmlSetting(payload any, key string) (any, bool) {
	settings, ok := payload.(map[string]any)
	if !ok {
		return nil, false
	}
	if value, ok := settings[key]; ok {
		return value, true
	}
	if value, ok := settings[settingsSection+"."+key]; ok {
		return value, true
	}
	if nested, ok := settings[settingsSection].(map[string]any); ok {
		if value, ok := nested[key]; ok {
			return value, true
		}
	}
	return nil, false
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
