package lsp

import (
	"context"
	"encoding/json"
	"slices"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// strictConformanceKey is the setting an editor sets to ask the strict question,
// spelled as the CLI's -strict flag and the REPL's %strict command.
const strictConformanceKey = "strictConformance"

// disabledLintsKey is the setting naming the lints an editor leaves out of the
// diagnostics, spelled as the CLI's -disable-lint flag.
const disabledLintsKey = "disabledLints"

// enabledLintsKey is the setting naming the opt-in lints an editor keeps in the
// diagnostics, spelled as the CLI's -enable-lint flag.
const enabledLintsKey = "enabledLints"

// libraryRootsKey is the setting naming the directories installed projects are
// looked for under, spelled as the CLI's -library-root flag.
const libraryRootsKey = "libraryRoots"

// settingsSection is the section an editor nests this server's settings under.
const settingsSection = "sysml"

// applyConformanceSettings switches the workspace's conformance mode to what a
// settings payload asks for, and reports whether the mode changed: a payload
// without the setting leaves the mode alone rather than resetting it, and a
// workspace that cannot answer the new mode says why to the client.
// decodeSettings reads a settings payload as the generic JSON the readers walk;
// nothing, when it is absent or malformed.
func decodeSettings(raw protocol.LSPAny) any {
	var payload any
	if len(raw) == 0 || json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	return payload
}

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
// conformance mode, the disabled and enabled lints and the library roots are
// read; a payload that mentions none of them changes nothing.
func (s *Server) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) error {
	if params == nil {
		return nil
	}
	settings := decodeSettings(params.Settings)
	modeChanged := s.applyConformanceSettings(ctx, settings)
	lintsChanged := s.applyLintSettings(ctx, settings)
	if s.applyLibraryRootsSetting(settings) {
		// The walk republishes the open documents, whose imports may now resolve.
		s.reloadDependencies(ctx)
		return nil
	}
	if !modeChanged && !lintsChanged {
		return nil
	}
	s.republishOpenDiagnostics(ctx)
	return nil
}

// applyLibraryRootsSetting replaces the library roots with what the list
// setting names, and reports whether they changed: a payload without the
// setting, or with a value that is not a list of strings, leaves them alone.
func (s *Server) applyLibraryRootsSetting(payload any) bool {
	value, ok := sysmlSetting(payload, libraryRootsKey)
	if !ok {
		return false
	}
	roots, ok := stringListSetting(value)
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if slices.Equal(roots, s.libraryRoots) {
		return false
	}
	s.libraryRoots = roots
	return true
}

// applyLintSettings replaces the lints the workspace leaves out, and the opt-in
// lints it keeps, with the ones a settings payload names, and reports whether it
// named either setting: a payload without one leaves those lints alone.
func (s *Server) applyLintSettings(ctx context.Context, payload any) bool {
	disabled := s.applyLintSetting(ctx, payload, disabledLintsKey, s.ws.SetDisabledLints)
	enabled := s.applyLintSetting(ctx, payload, enabledLintsKey, s.ws.SetEnabledLints)
	return disabled || enabled
}

// applyLintSetting hands set the codes the list setting key names, and reports
// whether it took them: a value that is not a list of strings is ignored, and
// an unknown code is shown the client.
func (s *Server) applyLintSetting(ctx context.Context, payload any, key string, set func([]string) error) bool {
	value, ok := sysmlSetting(payload, key)
	if !ok {
		return false
	}
	codes, ok := stringListSetting(value)
	if !ok {
		return false
	}
	if err := set(codes); err != nil {
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

// stringListSetting reads a JSON list of strings, refusing a value of any
// other shape.
func stringListSetting(value any) ([]string, bool) {
	list, ok := value.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		text, isString := item.(string)
		if !isString {
			return nil, false
		}
		out = append(out, text)
	}
	return out, true
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
