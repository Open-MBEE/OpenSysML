using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One assumption or omission behind a solver's answer, worded once for every surface as the REPL's solver reports
/// print it.
/// </summary>
public sealed record VerdictNote
{
    /// <summary>The kind of a note about a feature read as holding one value or object while only the default <c>0..*</c> governs it.</summary>
    public const string AssumedOne = "assumed_one";

    /// <summary>The kind of a note about an object-holding feature whose objects' own assertions the question does not state.</summary>
    public const string Uncovered = "uncovered";

    /// <summary>Creates a note.</summary>
    /// <param name="kind">What the note says, <see cref="AssumedOne"/> or <see cref="Uncovered"/>; a newer service may name others.</param>
    /// <param name="subject">The feature the note is about, as the report names it.</param>
    /// <param name="text">The note as a sentence.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public VerdictNote(string kind, string subject, string text)
    {
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(subject);
        ArgumentNullException.ThrowIfNull(text);
        Kind = kind;
        Subject = subject;
        Text = text;
    }

    /// <summary>
    /// What the note says: <see cref="AssumedOne"/> or <see cref="Uncovered"/>. It is the service's word rather than a
    /// closed set, so a kind a newer service adds is kept as it was sent.
    /// </summary>
    public string Kind { get; }

    /// <summary>The feature the note is about, as the report names it.</summary>
    public string Subject { get; }

    /// <summary>The note as a sentence.</summary>
    public string Text { get; }

    /// <summary>The note as a sentence.</summary>
    /// <returns><see cref="Text"/>.</returns>
    public override string ToString() => Text;
}
