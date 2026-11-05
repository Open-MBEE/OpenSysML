using System;
using System.Globalization;

namespace OpenMBEE.OpenSysML;

/// <summary>A range of a source document, 1-based, as the service reports it.</summary>
public sealed record SourceSpan
{
    /// <summary>Creates a span.</summary>
    /// <param name="file">The document: a path, or the name inline content is reported under.</param>
    /// <param name="startLine">The first line.</param>
    /// <param name="startColumn">The first column.</param>
    /// <param name="endLine">The last line.</param>
    /// <param name="endColumn">The last column.</param>
    /// <exception cref="ArgumentNullException"><paramref name="file"/> is null.</exception>
    public SourceSpan(string file, int startLine, int startColumn, int endLine, int endColumn)
    {
        ArgumentNullException.ThrowIfNull(file);
        File = file;
        StartLine = startLine;
        StartColumn = startColumn;
        EndLine = endLine;
        EndColumn = endColumn;
    }

    /// <summary>The document: a path, or the name inline content is reported under.</summary>
    public string File { get; }

    /// <summary>The first line, 1-based.</summary>
    public int StartLine { get; }

    /// <summary>The first column, 1-based.</summary>
    public int StartColumn { get; }

    /// <summary>The last line, 1-based.</summary>
    public int EndLine { get; }

    /// <summary>The last column, 1-based.</summary>
    public int EndColumn { get; }

    /// <summary>The span, as <c>file:line:column</c>.</summary>
    /// <returns>The span's text.</returns>
    public override string ToString() =>
        string.Create(CultureInfo.InvariantCulture, $"{File}:{StartLine}:{StartColumn}");
}
