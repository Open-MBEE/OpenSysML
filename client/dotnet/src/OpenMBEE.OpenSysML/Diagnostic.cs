using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>A parse, validation or run finding the service reported about a model.</summary>
public sealed record Diagnostic
{
    /// <summary>Creates a diagnostic.</summary>
    /// <param name="severityName">The severity as the service spells it (<c>error</c>, <c>warning</c>, <c>info</c>).</param>
    /// <param name="message">The finding, as the service words it.</param>
    /// <param name="code">The stable code of what was found; empty when none was assigned.</param>
    /// <param name="span">Where in the source it is; null when the service located none.</param>
    /// <param name="orderDependent">On a <c>choice-point</c>, the features the conflicting alternatives wrote.</param>
    /// <param name="independent">On a <c>choice-point</c>, whether the alternatives commute.</param>
    /// <exception cref="ArgumentNullException">A string or list is null.</exception>
    /// <exception cref="ArgumentException">An entry of <paramref name="orderDependent"/> is null.</exception>
    public Diagnostic(string severityName, string message, string code, SourceSpan? span, IEnumerable<string> orderDependent, bool independent)
    {
        ArgumentNullException.ThrowIfNull(severityName);
        ArgumentNullException.ThrowIfNull(message);
        ArgumentNullException.ThrowIfNull(code);
        SeverityName = severityName;
        Message = message;
        Code = code;
        Span = span;
        OrderDependent = new ValueList<string>(orderDependent);
        Independent = independent;
    }

    /// <summary>How serious the finding is; <see cref="DiagnosticSeverity.Unknown"/> for a severity this client does not know.</summary>
    public DiagnosticSeverity Severity => SeverityName switch
    {
        "error" => DiagnosticSeverity.Error,
        "warning" => DiagnosticSeverity.Warning,
        "info" => DiagnosticSeverity.Info,
        _ => DiagnosticSeverity.Unknown,
    };

    /// <summary>The severity as the service spells it, kept whether or not this client knows it.</summary>
    public string SeverityName { get; }

    /// <summary>The finding, as the service words it.</summary>
    public string Message { get; }

    /// <summary>
    /// The stable identifier of what was found, to branch on instead of the message: a validation code,
    /// <c>syntax</c>, or <c>choice-point</c> and <c>guard-unevaluable</c> on a run; empty when none was assigned.
    /// </summary>
    public string Code { get; }

    /// <summary>Where in the source it is; null when the service located none.</summary>
    public SourceSpan? Span { get; }

    /// <summary>
    /// On a <c>choice-point</c>, the features the conflicting alternatives wrote, whose values the run
    /// holds only up to the order it took; empty otherwise.
    /// </summary>
    public ValueList<string> OrderDependent { get; }

    /// <summary>On a <c>choice-point</c>, whether the alternatives commute, so the run's values stand whichever was taken.</summary>
    public bool Independent { get; }

    /// <summary>The finding, as <c>error at a.sysml:3:5: message [code]</c>.</summary>
    /// <returns>The diagnostic's text.</returns>
    public override string ToString() =>
        $"{SeverityName}{(Span is null ? string.Empty : $" at {Span}")}: {Message}{(Code.Length == 0 ? string.Empty : $" [{Code}]")}";
}
