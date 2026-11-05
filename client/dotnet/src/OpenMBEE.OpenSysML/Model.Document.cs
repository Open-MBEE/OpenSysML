using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string RunDocumentQueryMethod = "sysml.SysMLService/RunDocumentQuery";
    internal const string RenderDocumentMethod = "sysml.SysMLService/RenderDocument";

    /// <summary>
    /// Runs a document query the model declares (a calc def specializing <c>DocumentQueries::Query</c>) with its entry
    /// parameters bound, answering as <c>%run-query</c> does but with typed columns and rows. The query runs over the
    /// objects the service holds for the model, which <see cref="InstantiateAsync"/> creates and an object binding names.
    /// </summary>
    /// <param name="queryId">The document query's qualified name.</param>
    /// <param name="bindings">The entry parameters' values, in order; a parameter bound more than once takes every value. Null binds none.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The projected columns and the rows, in the engine's order.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="queryId"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="queryId"/> is empty, or a binding is null.</exception>
    /// <exception cref="InvalidRequestException">A bound quantity cannot be sent; nothing was sent.</exception>
    /// <exception cref="CapabilityException">
    /// The service lacks <see cref="Capabilities.DocumentQuery"/>, or <see cref="Capabilities.BigIntValues"/> or
    /// <see cref="Capabilities.RationalValues"/> for a bound Integer beyond <see cref="long"/> or exact Rational, checked
    /// in that order; nothing was sent. A Rational a <see cref="double"/> holds exactly is sent as that double to a
    /// service without <see cref="Capabilities.RationalValues"/>.
    /// </exception>
    /// <exception cref="QueryException">The model's declaration of the query could not be planned or run.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">
    /// The service refused the call: <see cref="StatusCode.NotFound"/> for a query the model does not declare or an
    /// object it does not hold, <see cref="StatusCode.InvalidArgument"/> for something else of that name or a binding
    /// the query does not take, <see cref="StatusCode.ResourceExhausted"/> for a run past the engine's budget.
    /// </exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<DocumentQueryResult> RunDocumentQueryAsync(string queryId, IEnumerable<DocumentBinding>? bindings = null, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(queryId);
        var held = bindings is null ? default : new ValueList<DocumentBinding>(bindings);
        Connection.ThrowIfDisposed();
        var request = new Wire.RunDocumentQueryRequest { ModelHash = Hash, QueryId = queryId, Bindings = { DocumentWire.ToWire(held) } };
        var capabilities = Connection.Capabilities;
        capabilities.Require(Capabilities.DocumentQuery);
        DocumentWire.Fit(request.Bindings, capabilities);
        var response = await DeclarationCallAsync<Wire.RunDocumentQueryRequest, Wire.RunDocumentQueryResponse>(RunDocumentQueryMethod, request, modelFaults: true, cancellationToken)
            .ConfigureAwait(false);
        return DocumentWire.Result(response, RunDocumentQueryMethod);
    }

    /// <summary>
    /// Renders a document the model declares (a part def specializing <c>DocumentQueries::Document</c>), byte-for-byte
    /// what the CLI's <c>-render-document</c> writes in that form. A document binds its queries' parameters in the model,
    /// and reads the objects the service holds for it.
    /// </summary>
    /// <param name="documentId">The document's qualified name.</param>
    /// <param name="form">The form to render it to.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The rendered document, and the file a rendered set writes it to when the service names one.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="documentId"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="documentId"/> is empty.</exception>
    /// <exception cref="InvalidRequestException"><paramref name="form"/> is not a <see cref="DocumentForm"/>; nothing was sent.</exception>
    /// <exception cref="CapabilityException">
    /// The service lacks <see cref="Capabilities.RenderDocument"/>, or <see cref="Capabilities.RenderDocumentHtml"/> for
    /// HTML or <see cref="Capabilities.RenderDocumentAsciidoc"/> for AsciiDoc, checked in that order; nothing was sent.
    /// </exception>
    /// <exception cref="QueryException">The model's declaration of the document, or of a query it runs, could not be planned or run.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">
    /// The service refused the call: <see cref="StatusCode.NotFound"/> for a document the model does not declare,
    /// <see cref="StatusCode.InvalidArgument"/> for something else of that name.
    /// </exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<RenderedDocument> RenderDocumentAsync(string documentId, DocumentForm form = DocumentForm.Markdown, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(documentId);
        Connection.ThrowIfDisposed();
        if (!Enum.IsDefined(form))
        {
            throw new InvalidRequestException($"The form is {form}, which is not a DocumentForm.");
        }

        var capabilities = Connection.Capabilities;
        capabilities.Require(Capabilities.RenderDocument);
        switch (form)
        {
            case DocumentForm.Html:
                capabilities.Require(Capabilities.RenderDocumentHtml);
                break;
            case DocumentForm.AsciiDoc:
                capabilities.Require(Capabilities.RenderDocumentAsciidoc);
                break;
        }

        // Markdown is the form an empty one names, which every service renders.
        var spelled = form switch
        {
            DocumentForm.Html => "html",
            DocumentForm.AsciiDoc => "asciidoc",
            _ => string.Empty,
        };
        var request = new Wire.RenderDocumentRequest { ModelHash = Hash, DocumentId = documentId, Form = spelled };
        var response = await DeclarationCallAsync<Wire.RenderDocumentRequest, Wire.RenderDocumentResponse>(RenderDocumentMethod, request, modelFaults: true, cancellationToken)
            .ConfigureAwait(false);
        (DocumentForm Form, string Content)[] answered = [(DocumentForm.Markdown, response.Markdown), (DocumentForm.Html, response.Html), (DocumentForm.AsciiDoc, response.Asciidoc)];
        if (answered.Any(each => each.Form != form && each.Content.Length > 0))
        {
            throw new TransportException(StatusCode.Internal, $"{RenderDocumentMethod} answered the document in a form other than {form}, which was asked for.", RenderDocumentMethod);
        }

        return new RenderedDocument(form, answered.Single(each => each.Form == form).Content, response.FileName.Length > 0 ? response.FileName : null);
    }

    // A call naming a declaration of the model answers NotFound for the model or for that declaration; the model's is
    // told by the message the service words for this model's hash. With modelFaults, a failed precondition is a fault in
    // the model's own document or query declarations.
    private async Task<TResponse> DeclarationCallAsync<TRequest, TResponse>(string method, TRequest request, bool modelFaults, CancellationToken cancellationToken)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        try
        {
            return await Connection.UnaryAsync<TRequest, TResponse>(method, request, cancellationToken).ConfigureAwait(false);
        }
        catch (ServiceException error) when (error.GetType() == typeof(ServiceException) && error.StatusCode == StatusCode.NotFound && error.ServiceMessage == $"model not found: {Hash}")
        {
            throw new ModelNotFoundException(error.ServiceMessage, error.Method, error.InnerException);
        }
        catch (ServiceException error) when (modelFaults && error.GetType() == typeof(ServiceException) && error.StatusCode == StatusCode.FailedPrecondition)
        {
            throw new QueryException(error.StatusCode, error.ServiceMessage, error.Method, error.InnerException);
        }
    }
}
