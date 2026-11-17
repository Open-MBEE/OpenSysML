namespace OpenMBEE.OpenSysML;

/// <summary>How <see cref="Connection.ParseSourcesAsync"/> parses several documents as one model.</summary>
public sealed class SourcesOptions
{
    /// <summary>The service's defaults.</summary>
    public static SourcesOptions Default { get; } = new();

    /// <summary>
    /// Whether to judge every document as conforming SysML v2, as <see cref="ParseOptions.StrictConformance"/>
    /// does. Needs the <see cref="Capabilities.StrictConformance"/> capability.
    /// </summary>
    public bool StrictConformance { get; init; }

    /// <summary>
    /// The <see cref="Model.Hash"/> of a model of these documents whose results the caller holds,
    /// typically the previous parse's, or null. The answer's <see cref="Model.Affected"/> then names
    /// the documents whose results may differ from that model's. Needs the
    /// <see cref="Capabilities.ParseSourcesAffected"/> capability; an empty hash is refused.
    /// </summary>
    public string? BaseModelHash { get; init; }
}
