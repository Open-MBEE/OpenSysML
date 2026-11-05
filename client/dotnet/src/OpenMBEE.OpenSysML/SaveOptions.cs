namespace OpenMBEE.OpenSysML;

/// <summary>
/// How <see cref="Conversion.SaveAsync"/>, <see cref="Model.SaveAsync"/>, <see cref="Migration.WriteAsync"/>,
/// <see cref="EditResult.SaveAsync"/> and <see cref="EditedDocument.SaveAsync"/> write a file. Every file is written whole to a temporary file beside it, flushed, and then moved into place, so a write that
/// fails or is cancelled leaves no partial file and an existing destination as it was.
/// </summary>
public sealed class SaveOptions
{
    /// <summary>Refuses to replace an existing file.</summary>
    public static SaveOptions Default { get; } = new();

    /// <summary>
    /// Whether an existing file is replaced, in one atomic move on its own file system; false refuses one with
    /// <see cref="UnwritableException"/>. A migration never replaces the SysML v1 model it came from, even with this set.
    /// </summary>
    public bool Overwrite { get; init; }
}
