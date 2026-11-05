namespace OpenMBEE.OpenSysML;

/// <summary>
/// How a SysML v1 model is migrated, as <c>sysml -migrate</c> migrates one. The report's summary and counts come back
/// with every migration; the rest is asked for here.
/// </summary>
public sealed class MigrateOptions
{
    /// <summary>The form inferred from the file's extension, the summary alone, no layout, and every extension the migration writes.</summary>
    public static MigrateOptions Default { get; } = new();

    /// <summary>
    /// The v1 form: <see cref="ConversionFormat.Xmi"/>, <see cref="ConversionFormat.Uml"/> or
    /// <see cref="ConversionFormat.Mdzip"/>; null infers it from the file's extension. Inline content must name it. A v2
    /// format is refused by the service, pointing at <see cref="Connection.ConvertAsync"/>.
    /// </summary>
    public string? FromFormat { get; init; }

    /// <summary>Carries every element's verdict and the report's text in <see cref="Migration.Report"/>, as <c>-migration-report</c> writes it.</summary>
    public bool Report { get; init; }

    /// <summary>
    /// Carries the result snapshots the v1 tool stored with the model in <see cref="Migration.Results"/>, as
    /// <c>-migration-results</c> writes them, so the migrated runs can be compared with the tool's.
    /// </summary>
    public bool Results { get; init; }

    /// <summary>
    /// An MTIP export of the model's diagrams (<c>Model_mtip.xml</c>) the service reads, in its own file system, laying
    /// the migrated views out as the v1 tool drew them; null for none. At most one of this and <see cref="LayoutContent"/>.
    /// </summary>
    public string? LayoutPath { get; init; }

    /// <summary>The MTIP export itself, carried inline; null for none. At most one of this and <see cref="LayoutPath"/>.</summary>
    public string? LayoutContent { get; init; }

    /// <summary>
    /// The absolute http(s) URL a server-relative image reference in a v1 comment (<c>&lt;img src="/projects/…"&gt;</c>)
    /// is resolved against; null for none.
    /// </summary>
    public string? ImageBaseUrl { get; init; }

    /// <summary>
    /// Writes only notation a pinned SysML v2 production admits: a construct whose only v2 form is an OpenSysML
    /// extension (a deferred event, a choice, junction or history pseudostate) is reported unmapped instead of written.
    /// </summary>
    public bool Strict { get; init; }
}
