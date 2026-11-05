namespace OpenMBEE.OpenSysML;

/// <summary>How <see cref="Connection.LoadAsync"/> and <see cref="Connection.LoadContentAsync"/> parse a source.</summary>
public sealed class ParseOptions
{
    /// <summary>The service's defaults: SysML notation, judged as the service judges it by default.</summary>
    public static ParseOptions Default { get; } = new();

    /// <summary>
    /// The notation inline content is written in, or null for the service's default, SysML. Naming
    /// one needs the <see cref="Capabilities.InlineLanguage"/> capability. A file's notation comes
    /// from its extension, so loading a file with a language named is refused.
    /// </summary>
    public SourceLanguage? Language { get; init; }

    /// <summary>
    /// Whether to judge the source as conforming SysML v2, making notation no pinned production
    /// admits an error rather than a warning. Needs the <see cref="Capabilities.StrictConformance"/> capability.
    /// </summary>
    public bool StrictConformance { get; init; }
}
