package lsp

import (
	"context"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// baseServer implements protocol.Server with method-not-found defaults for
// requests and no-op defaults for notifications. Server embeds it and overrides
// only the handlers this plan implements.
type baseServer struct{}

var _ protocol.Server = (*baseServer)(nil)

func (baseServer) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	return nil
}
func (baseServer) Shutdown(ctx context.Context) error { return nil }
func (baseServer) Exit(ctx context.Context) error     { return nil }
func (baseServer) WorkDoneProgressCancel(ctx context.Context, params *protocol.WorkDoneProgressCancelParams) error {
	return nil
}
func (baseServer) LogTrace(ctx context.Context, params *protocol.LogTraceParams) error { return nil }
func (baseServer) SetTrace(ctx context.Context, params *protocol.SetTraceParams) error { return nil }
func (baseServer) CodeAction(ctx context.Context, params *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) CodeLens(ctx context.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) CodeLensResolve(ctx context.Context, params *protocol.CodeLens) (*protocol.CodeLens, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) ColorPresentation(ctx context.Context, params *protocol.ColorPresentationParams) ([]protocol.ColorPresentation, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) CompletionResolve(ctx context.Context, params *protocol.CompletionItem) (*protocol.CompletionItem, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) Declaration(ctx context.Context, params *protocol.DeclarationParams) ([]protocol.Location, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) Definition(ctx context.Context, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	return nil
}
func (baseServer) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) error {
	return nil
}
func (baseServer) DidChangeWatchedFiles(ctx context.Context, params *protocol.DidChangeWatchedFilesParams) error {
	return nil
}
func (baseServer) DidChangeWorkspaceFolders(ctx context.Context, params *protocol.DidChangeWorkspaceFoldersParams) error {
	return nil
}
func (baseServer) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	return nil
}
func (baseServer) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	return nil
}
func (baseServer) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) error {
	return nil
}
func (baseServer) DocumentColor(ctx context.Context, params *protocol.DocumentColorParams) ([]protocol.ColorInformation, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) DocumentHighlight(ctx context.Context, params *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) DocumentLink(ctx context.Context, params *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) DocumentLinkResolve(ctx context.Context, params *protocol.DocumentLink) (*protocol.DocumentLink, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) ([]interface{}, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (interface{}, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) FoldingRanges(ctx context.Context, params *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) Implementation(ctx context.Context, params *protocol.ImplementationParams) ([]protocol.Location, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) OnTypeFormatting(ctx context.Context, params *protocol.DocumentOnTypeFormattingParams) ([]protocol.TextEdit, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) PrepareRename(ctx context.Context, params *protocol.PrepareRenameParams) (*protocol.Range, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) RangeFormatting(ctx context.Context, params *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) References(ctx context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) Rename(ctx context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) SignatureHelp(ctx context.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) Symbols(ctx context.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) TypeDefinition(ctx context.Context, params *protocol.TypeDefinitionParams) ([]protocol.Location, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) WillSave(ctx context.Context, params *protocol.WillSaveTextDocumentParams) error {
	return nil
}
func (baseServer) WillSaveWaitUntil(ctx context.Context, params *protocol.WillSaveTextDocumentParams) ([]protocol.TextEdit, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) ShowDocument(ctx context.Context, params *protocol.ShowDocumentParams) (*protocol.ShowDocumentResult, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) WillCreateFiles(ctx context.Context, params *protocol.CreateFilesParams) (*protocol.WorkspaceEdit, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) DidCreateFiles(ctx context.Context, params *protocol.CreateFilesParams) error {
	return nil
}
func (baseServer) WillRenameFiles(ctx context.Context, params *protocol.RenameFilesParams) (*protocol.WorkspaceEdit, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) DidRenameFiles(ctx context.Context, params *protocol.RenameFilesParams) error {
	return nil
}
func (baseServer) WillDeleteFiles(ctx context.Context, params *protocol.DeleteFilesParams) (*protocol.WorkspaceEdit, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) DidDeleteFiles(ctx context.Context, params *protocol.DeleteFilesParams) error {
	return nil
}
func (baseServer) CodeLensRefresh(ctx context.Context) error { return nil }
func (baseServer) PrepareCallHierarchy(ctx context.Context, params *protocol.CallHierarchyPrepareParams) ([]protocol.CallHierarchyItem, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) IncomingCalls(ctx context.Context, params *protocol.CallHierarchyIncomingCallsParams) ([]protocol.CallHierarchyIncomingCall, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) OutgoingCalls(ctx context.Context, params *protocol.CallHierarchyOutgoingCallsParams) ([]protocol.CallHierarchyOutgoingCall, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) SemanticTokensFull(ctx context.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) SemanticTokensFullDelta(ctx context.Context, params *protocol.SemanticTokensDeltaParams) (interface{}, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) SemanticTokensRange(ctx context.Context, params *protocol.SemanticTokensRangeParams) (*protocol.SemanticTokens, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) SemanticTokensRefresh(ctx context.Context) error { return nil }
func (baseServer) LinkedEditingRange(ctx context.Context, params *protocol.LinkedEditingRangeParams) (*protocol.LinkedEditingRanges, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) Moniker(ctx context.Context, params *protocol.MonikerParams) ([]protocol.Moniker, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
func (baseServer) Request(ctx context.Context, method string, params interface{}) (interface{}, error) {
	return nil, jsonrpc2.ErrMethodNotFound
}
