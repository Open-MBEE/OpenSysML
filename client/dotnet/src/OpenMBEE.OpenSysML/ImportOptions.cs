namespace OpenMBEE.OpenSysML;

/// <summary>
/// How <see cref="Model.ImportDataAsync"/> imports its data files: what each row is made into, whether the model is
/// changed, and whether the documents of a model of several are accepted.
/// </summary>
public sealed class ImportOptions
{
    /// <summary>Every row's values set, the model changed, its documents accepted.</summary>
    public static ImportOptions Default { get; } = new();

    /// <summary>
    /// What each row is made into (<c>sysml -import-as</c>): <c>values</c>, the service's default and the one form it
    /// supports, sets the features of the element the row names; null for the default. Another is refused by the
    /// service with <see cref="StatusCode.InvalidArgument"/>; empty is refused before sending.
    /// </summary>
    public string? As { get; init; }

    /// <summary>
    /// Whether the imports only report the values they would set (<c>sysml -import-dry-run</c>), changing nothing: the
    /// answer then has no model, notation or documents, and the model is as it was.
    /// </summary>
    public bool DryRun { get; init; }

    /// <summary>
    /// Whether the edited documents of a model of several are accepted, its edited notation then in
    /// <see cref="ImportResult.Documents"/> alone. True by default, since this client reads them; false asks the service
    /// to refuse a model of several documents with <see cref="StatusCode.FailedPrecondition"/>, as
    /// <see cref="EditOptions.AcceptDocuments"/> does.
    /// </summary>
    public bool AcceptDocuments { get; init; } = true;
}
