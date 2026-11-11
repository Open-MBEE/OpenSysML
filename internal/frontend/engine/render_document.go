// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package engine

import (
	"context"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docset"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// renderDocument renders a document as the service does, over a run holding no objects (the
// engine keeps no Instantiate store) and without Graphviz, so positioned diagrams fall back to Mermaid.
func (e *Engine) renderDocument(ctx context.Context, req *JRenderDocumentRequest) ([]byte, error) {
	form := docset.Form(req.Form)
	if form == "" {
		form = docset.FormMarkdown
	}
	if !slices.Contains(docset.Forms(), form) {
		return nil, statusError(codeInvalidArgument, (&docset.UnknownFormError{Form: req.Form}).Error())
	}
	cached, ok := e.get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(codeNotFound, msgModelNotFound, req.ModelHash)
	}
	if req.DocumentId == "" {
		return nil, statusError(codeInvalidArgument, "a document to render must be named in documentId")
	}
	syms := lookupNamed(cached.Index, req.DocumentId)
	if len(syms) == 0 {
		return nil, statusErrorf(codeNotFound, "symbol not found: %s", req.DocumentId)
	}
	sem := cached.typedModel()
	qctx := queryexec.Context{
		Index:     cached.Index,
		Resolver:  cached.resolver,
		Model:     sem,
		Runtime:   e.newRuntime(ctx, cached),
		ElementID: cached.elementIDs(),
	}
	rendered, err := docset.Render(qctx, syms[0], cached.sourceText(), docset.Options{Form: form, WithoutGraphviz: true})
	if err != nil {
		return nil, documentError(err)
	}
	resp := &JRenderDocumentResponse{FileName: rendered.FileName}
	switch form {
	case docset.FormHTML:
		resp.Html = rendered.Text
	case docset.FormAsciiDoc:
		resp.Asciidoc = rendered.Text
	default:
		resp.Markdown = rendered.Text
	}
	return marshal(resp)
}

// documentError reports a document failure under the code the service reports
// it, naming the source declaration behind it when the pipeline reports one.
func documentError(err error) *Error {
	message := err.Error()
	if origin := docset.OriginOf(err); origin.Located() {
		message = fmt.Sprintf("%s (declared in %s)", message, origin.Doc)
	}
	return statusError(faultCode(docset.FaultOf(err)), message)
}

// faultCode is the status code a document fault is reported under, as the
// service's faultCode numbers it.
func faultCode(fault docset.Fault) uint32 {
	switch fault {
	case docset.FaultArgument:
		return codeInvalidArgument
	case docset.FaultModel:
		return codeFailedPrecondition
	case docset.FaultBudget:
		return codeResourceExhausted
	default:
		return codeInternal
	}
}

// elementIDs reads the elementId a conversion writes for each element through the registered
// converter, as the service does; nil when the binary links none (the js build), and elementId is absent.
func (m *cachedModel) elementIDs() func(*symbols.Symbol) (string, error) {
	documents := make([]queryexec.ElementIDDocument, 0, len(m.Documents))
	for _, doc := range m.Documents {
		documents = append(documents, queryexec.ElementIDDocument{File: doc.Source, Root: doc.Root, ParseDiags: doc.Errors})
	}
	read := queryexec.OpenElementIDs(documents)
	if read == nil {
		return nil
	}
	return func(sym *symbols.Symbol) (string, error) {
		return queryexec.ElementIDOf(read, m.Index, sym)
	}
}
