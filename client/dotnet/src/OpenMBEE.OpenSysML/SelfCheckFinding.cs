using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One thing a self-check reports about a constraint applied to an element, or a constraint or rule package it skipped.</summary>
public sealed record SelfCheckFinding
{
    /// <summary>Creates a finding.</summary>
    /// <param name="status"><c>fails</c>, <c>unevaluated</c>, <c>error</c> or <c>warning</c>.</param>
    /// <param name="subject"><c>&lt;constraint&gt; for &lt;element&gt;</c>, or the constraint or package alone.</param>
    /// <param name="location">The element's <c>document:line:col</c>; empty when it has none.</param>
    /// <param name="constraint">The qualified name of the constraint applied.</param>
    /// <param name="element">The name of the element it was applied to.</param>
    /// <param name="message">The reason, for a warning or an error.</param>
    /// <param name="line">The finding as <c>sysml -self-check</c> prints it.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public SelfCheckFinding(string status, string subject, string location, string constraint, string element, string message, string line)
    {
        ArgumentNullException.ThrowIfNull(status);
        ArgumentNullException.ThrowIfNull(subject);
        ArgumentNullException.ThrowIfNull(location);
        ArgumentNullException.ThrowIfNull(constraint);
        ArgumentNullException.ThrowIfNull(element);
        ArgumentNullException.ThrowIfNull(message);
        ArgumentNullException.ThrowIfNull(line);
        Status = status;
        Subject = subject;
        Location = location;
        Constraint = constraint;
        Element = element;
        Message = message;
        Line = line;
    }

    /// <summary>
    /// What was found, as the service spells it: <c>fails</c> when the element answered the constraint false,
    /// <c>unevaluated</c> when the runtime cannot yet apply the constraint reflectively, <c>error</c> when the evaluation
    /// failed, <c>warning</c> when a rule constraint was skipped or a rule package yielded none.
    /// </summary>
    public string Status { get; }

    /// <summary><c>&lt;constraint&gt; for &lt;element&gt;</c>, or the constraint or package alone.</summary>
    public string Subject { get; }

    /// <summary>The element's <c>document:line:col</c>; empty when it has none.</summary>
    public string Location { get; }

    /// <summary>The qualified name of the constraint applied.</summary>
    public string Constraint { get; }

    /// <summary>The name of the element it was applied to.</summary>
    public string Element { get; }

    /// <summary>The reason, for a warning or an error.</summary>
    public string Message { get; }

    /// <summary>The finding as <c>sysml -self-check</c> prints it.</summary>
    public string Line { get; }
}
