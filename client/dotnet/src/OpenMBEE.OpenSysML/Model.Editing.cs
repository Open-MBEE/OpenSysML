using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string ApplyEditsMethod = "sysml.SysMLService/ApplyEdits";

    /// <summary>Starts a batch of edits of this model, applied in one call by <see cref="Editor.ApplyAsync"/>.</summary>
    /// <returns>An empty editor.</returns>
    public Editor Edit() => new(this);

    /// <summary>
    /// Applies edits to the notation the model was parsed from, all of them or none, and answers the notation they
    /// produce: byte-identical to the source outside what the edits changed, comments and layout kept. Edits apply in
    /// order to the source the parse read, so they edit the model that was inspected. The model itself is unchanged;
    /// load the edited notation to read the edited model.
    /// </summary>
    /// <remarks>
    /// A model of several documents (<see cref="Connection.ParseSourcesAsync"/>) is edited as one: the edits target
    /// declarations of <see cref="EditOptions.InDocument"/>, and a rename or cascade delete follows references into
    /// every other document, rewriting those too. Its edited notation is in <see cref="EditResult.Documents"/> alone. An
    /// empty batch is sent too, and refused as <see cref="EditFailure.NoOperations"/>.
    /// </remarks>
    /// <param name="edits">The edits, applied in order as one batch.</param>
    /// <param name="options">The document edited and whether its documents are read; null for <see cref="EditOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The edited notation, the documents the edits rewrote, and what each edit changed.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="edits"/> is null.</exception>
    /// <exception cref="ArgumentException">An edit is null.</exception>
    /// <exception cref="InvalidRequestException">An edit sets an optional part to empty text, or <see cref="EditOptions.InDocument"/> is empty; nothing was sent.</exception>
    /// <exception cref="CapabilityException">
    /// The service lacks <see cref="Capabilities.ApplyEdits"/>; or a capability an edit needs, checked over the whole
    /// batch in the service's order: <see cref="Capabilities.Authoring"/> for any edit but a value or a rename, then
    /// the dedicated one of a connection, satisfy, requirement constraint, transition, verification, metadata, prefix,
    /// sequence, action-body statement or import, then <see cref="Capabilities.MemberModifiers"/>,
    /// <see cref="Capabilities.ImplicitParameters"/>, <see cref="Capabilities.ConstraintBodyAuthoring"/> and
    /// <see cref="Capabilities.StateActionAuthoring"/> for the member forms that need them, then documentation and
    /// comments; or, last, <see cref="Capabilities.EditDocuments"/> to name a document. Nothing was sent.
    /// </exception>
    /// <exception cref="EditException">The service refused the batch, saying why; nothing of it was applied.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">
    /// The service refused the call: <see cref="StatusCode.InvalidArgument"/> for a document the model does not have,
    /// <see cref="StatusCode.FailedPrecondition"/> for a model of several documents when
    /// <see cref="EditOptions.AcceptDocuments"/> is false or the service edits one alone.
    /// </exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<EditResult> ApplyEditsAsync(IEnumerable<Edit> edits, EditOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(edits);
        var batch = new ValueList<Edit>(edits);
        Connection.ThrowIfDisposed();
        options ??= EditOptions.Default;
        if (options.InDocument is { Length: 0 })
        {
            throw new InvalidRequestException("EditOptions.InDocument is empty; leave it null to edit the model's first document.");
        }

        EditWire.Validate(batch);
        var capabilities = Connection.Capabilities;
        capabilities.Require(Capabilities.ApplyEdits);
        foreach (var capability in EditWire.Needed(batch))
        {
            capabilities.Require(capability);
        }

        if (options.InDocument is not null)
        {
            capabilities.Require(Capabilities.EditDocuments);
        }

        var request = new Wire.ApplyEditsRequest
        {
            ModelHash = Hash,
            Document = options.InDocument ?? string.Empty,
            AcceptDocuments = options.AcceptDocuments,
        };
        foreach (var edit in batch)
        {
            request.Operations.Add(EditWire.ToWire(edit));
        }

        var response = await Connection.ModelCallAsync<Wire.ApplyEditsRequest, Wire.ApplyEditsResponse>(ApplyEditsMethod, request, cancellationToken).ConfigureAwait(false);
        return EditWire.Result(response, ApplyEditsMethod, batch.Count, DocumentCount, capabilities.Has(Capabilities.EditDocuments));
    }
}
