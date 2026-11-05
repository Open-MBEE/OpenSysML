using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One byte range of a document that an edit replaced, so a caller can report or locate what changed.</summary>
public sealed record AppliedEdit
{
    /// <summary>Creates an applied edit.</summary>
    /// <param name="operationIndex">The index, in the batch, of the edit that made the change.</param>
    /// <param name="target">The element edited, as the edit named it.</param>
    /// <param name="offset">Where the replaced bytes start.</param>
    /// <param name="length">How many bytes were replaced; zero for text inserted.</param>
    /// <param name="oldText">What was there; empty for an insertion.</param>
    /// <param name="newText">What was written.</param>
    /// <param name="document">The document the bytes are in, named as the parse named it; null from a service that names none.</param>
    /// <exception cref="ArgumentNullException">A string other than <paramref name="document"/> is null.</exception>
    /// <exception cref="ArgumentOutOfRangeException">The index, offset or length is negative.</exception>
    /// <exception cref="ArgumentException"><paramref name="document"/> is empty.</exception>
    public AppliedEdit(int operationIndex, string target, int offset, int length, string oldText, string newText, string? document)
    {
        ArgumentOutOfRangeException.ThrowIfNegative(operationIndex);
        ArgumentNullException.ThrowIfNull(target);
        ArgumentOutOfRangeException.ThrowIfNegative(offset);
        ArgumentOutOfRangeException.ThrowIfNegative(length);
        ArgumentNullException.ThrowIfNull(oldText);
        ArgumentNullException.ThrowIfNull(newText);
        if (document is { Length: 0 })
        {
            throw new ArgumentException("The document is empty; leave it null for an edit the service named no document of.", nameof(document));
        }

        OperationIndex = operationIndex;
        Target = target;
        Offset = offset;
        Length = length;
        OldText = oldText;
        NewText = newText;
        Document = document;
    }

    /// <summary>The index, in the batch, of the edit that made the change, so an answer maps back to its ask.</summary>
    public int OperationIndex { get; }

    /// <summary>The element edited, as the edit named it.</summary>
    public string Target { get; }

    /// <summary>
    /// Where the replaced bytes start, in UTF-8 bytes from the start of the document: of the source as parsed for a
    /// batch whose edits do not depend on one another, of the source an earlier edit of the batch left for one that does.
    /// </summary>
    public int Offset { get; }

    /// <summary>How many bytes were replaced; zero for text inserted.</summary>
    public int Length { get; }

    /// <summary>What was there; empty for an insertion.</summary>
    public string OldText { get; }

    /// <summary>What was written.</summary>
    public string NewText { get; }

    /// <summary>
    /// The document the bytes are in, named as the parse named it and as <see cref="EditResult.Documents"/> lists it;
    /// null from a service without <see cref="Capabilities.EditDocuments"/>, which names none.
    /// </summary>
    public string? Document { get; }
}
