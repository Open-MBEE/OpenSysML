using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One data file <see cref="Model.ImportDataAsync"/> imports, as <c>sysml -import</c> reads one: its name, which names
/// its format by extension and the file in what goes wrong, and its content, carried in the request. Each row names an
/// element, by the content's own header or by <see cref="Map"/>, and the values its features are set to.
/// </summary>
public sealed record DataImport
{
    private readonly string name = string.Empty;
    private readonly string content = string.Empty;

    /// <summary>Creates a data file.</summary>
    /// <param name="name">The file's name, whose extension (<c>.csv</c>, <c>.tsv</c>, <c>.json</c>, <c>.jsonl</c>) names its format unless <see cref="Format"/> does.</param>
    /// <param name="content">The file's content.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="name"/> is empty.</exception>
    public DataImport(string name, string content)
    {
        Name = name;
        Content = content;
    }

    /// <summary>
    /// The file's name, whose extension (<c>.csv</c>, <c>.tsv</c>, <c>.json</c>, <c>.jsonl</c>) names its format unless
    /// <see cref="Format"/> does; what goes wrong names the file by it. It is never read as a path.
    /// </summary>
    /// <exception cref="ArgumentNullException">The name is null.</exception>
    /// <exception cref="ArgumentException">The name is empty.</exception>
    public string Name
    {
        get => name;
        init
        {
            ArgumentException.ThrowIfNullOrEmpty(value, nameof(Name));
            name = value;
        }
    }

    /// <summary>The file's content.</summary>
    /// <exception cref="ArgumentNullException">The content is null.</exception>
    public string Content
    {
        get => content;
        init
        {
            ArgumentNullException.ThrowIfNull(value, nameof(Content));
            content = value;
        }
    }

    /// <summary>
    /// The format the content is read as, whatever the name's extension says (<c>sysml -import-format</c>): <c>csv</c>,
    /// <c>tsv</c>, <c>json</c> or <c>jsonl</c>; null reads it as <see cref="Name"/>'s extension says. Empty is refused
    /// before sending, since the service reads it as none.
    /// </summary>
    public string? Format { get; init; }

    /// <summary>
    /// The JSON mapping the content is read by onto elements and features, as the file <c>sysml -import-map</c> names
    /// holds it; null reads the content by its own header. Empty is refused before sending, since the service reads it
    /// as none.
    /// </summary>
    public string? Map { get; init; }
}
