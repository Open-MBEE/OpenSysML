namespace OpenMBEE.OpenSysML;

/// <summary>How a batch of edits is applied: the document whose declarations the edits target, and whether the answer's documents are read.</summary>
public sealed class EditOptions
{
    /// <summary>The model's first document edited, its documents accepted.</summary>
    public static EditOptions Default { get; } = new();

    /// <summary>
    /// The document whose declarations the edits target, named as the parse named it (<see cref="Model.Documents"/>);
    /// null for the model's first document, the only one of a loaded model. An edit targeting a declaration of another
    /// document is refused as <see cref="EditFailure.UnknownTarget"/>; a name no document of the model has is refused
    /// by the service with <see cref="StatusCode.InvalidArgument"/>. Naming one needs
    /// <see cref="Capabilities.EditDocuments"/>; empty is refused before sending.
    /// </summary>
    public string? InDocument { get; init; }

    /// <summary>
    /// Whether the answer's documents are read, which a model of several documents is edited only with: its edited
    /// notation is in <see cref="EditResult.Documents"/> alone. True by default, since this client reads them; false
    /// asks for a single document's notation alone, and the service then refuses a model of several documents with
    /// <see cref="StatusCode.FailedPrecondition"/> rather than answer one document as the model.
    /// </summary>
    public bool AcceptDocuments { get; init; } = true;
}
