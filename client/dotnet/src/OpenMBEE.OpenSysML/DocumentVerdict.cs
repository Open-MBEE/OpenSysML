using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One row a <c>Verdicts</c> document query answered: an assertion checked on one object, and how
/// it came out.
/// </summary>
public sealed record DocumentVerdict
{
    /// <summary>Creates a verdict row, copying its verification kinds.</summary>
    /// <param name="assertion">The assertion checked; null when the service reported none.</param>
    /// <param name="kind"><c>constraint</c>, <c>requirement</c>, <c>satisfaction</c> or <c>verification</c>.</param>
    /// <param name="text">The assertion as written.</param>
    /// <param name="path">The object checked, by path from the element the query was bound to.</param>
    /// <param name="verdict"><c>holds</c>, <c>violated</c> or <c>undecided</c>.</param>
    /// <param name="condition">The condition that evaluated to false, as written; empty otherwise.</param>
    /// <param name="reason">Why the assertion is violated or undecided; empty when it holds.</param>
    /// <param name="verification">The verdict kinds of the verification cases verifying the requirement the row is about.</param>
    /// <exception cref="ArgumentNullException">A string or the list is null.</exception>
    /// <exception cref="ArgumentException">A verification kind is null.</exception>
    public DocumentVerdict(
        DocumentValue.ElementValue? assertion,
        string kind,
        string text,
        string path,
        string verdict,
        string condition,
        string reason,
        IEnumerable<string> verification)
    {
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(text);
        ArgumentNullException.ThrowIfNull(path);
        ArgumentNullException.ThrowIfNull(verdict);
        ArgumentNullException.ThrowIfNull(condition);
        ArgumentNullException.ThrowIfNull(reason);
        Assertion = assertion;
        Kind = kind;
        Text = text;
        Path = path;
        Verdict = verdict;
        Condition = condition;
        Reason = reason;
        Verification = new ValueList<string>(verification);
    }

    /// <summary>
    /// The assertion checked, as an element: the constraint, requirement, satisfy usage or verification
    /// case, with no <see cref="DocumentValue.ElementValue.Id"/> when it is anonymous; null when the
    /// service reported none.
    /// </summary>
    public DocumentValue.ElementValue? Assertion { get; }

    /// <summary>What kind of assertion it is: <c>constraint</c>, <c>requirement</c>, <c>satisfaction</c> or <c>verification</c>.</summary>
    public string Kind { get; }

    /// <summary>The assertion as written (<c>satisfy Range by cruise</c>), naming an anonymous one.</summary>
    public string Text { get; }

    /// <summary>The object checked, by path from the element the query was bound to (<c>Garage::car.wheels[2]</c>).</summary>
    public string Path { get; }

    /// <summary>How the assertion came out: <c>holds</c>, <c>violated</c> or <c>undecided</c>.</summary>
    public string Verdict { get; }

    /// <summary>The condition that evaluated to false, as written; empty otherwise.</summary>
    public string Condition { get; }

    /// <summary>Why the assertion is violated or undecided; empty when it holds.</summary>
    public string Reason { get; }

    /// <summary>
    /// The verdict kinds (<c>pass</c>, <c>fail</c>, <c>inconclusive</c>, <c>error</c>) of the verification
    /// cases verifying the requirement the row is about; a verification row's own kind; empty for a constraint.
    /// </summary>
    public ValueList<string> Verification { get; }

    /// <summary>The row as <c>assert constraint massKnown on Garage::car: holds</c>.</summary>
    /// <returns>The row's text.</returns>
    public override string ToString() => $"{Text}{(Path.Length > 0 ? $" on {Path}" : string.Empty)}: {Verdict}";
}
