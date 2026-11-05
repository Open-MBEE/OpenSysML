using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The lowered graph of an action or a state machine, and of every behavior it performs, in the canonical
/// <c>graphs:&lt;version&gt;</c> JSON form external analysis engines read: the same text an engine is sent, kept
/// exactly as the service wrote it.
/// </summary>
public sealed record Graphs
{
    /// <summary>Creates the graphs.</summary>
    /// <param name="content">The form, as canonical JSON with one trailing newline.</param>
    /// <param name="version">The version of the form, the <c>version</c> field of the JSON.</param>
    /// <param name="subject">The subject's qualified name as the service resolved it.</param>
    /// <exception cref="ArgumentNullException">A string is null.</exception>
    public Graphs(string content, int version, string subject)
    {
        ArgumentNullException.ThrowIfNull(content);
        ArgumentNullException.ThrowIfNull(subject);
        Content = content;
        Version = version;
        Subject = subject;
    }

    /// <summary>
    /// The form, as canonical JSON with one trailing newline, exactly as the service wrote it: the subject's
    /// <c>ActionGraph</c> or <c>StateGraph</c> and those of every behavior it performs. The client reads nothing out of it.
    /// </summary>
    public string Content { get; }

    /// <summary>The version of the form, the <c>version</c> field of <see cref="Content"/>.</summary>
    public int Version { get; }

    /// <summary>The subject's qualified name as the service resolved it, the <c>subject</c> field of <see cref="Content"/>.</summary>
    public string Subject { get; }

    /// <summary>The form itself.</summary>
    /// <returns><see cref="Content"/>.</returns>
    public override string ToString() => Content;
}
