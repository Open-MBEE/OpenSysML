using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The service answered a conversion but could not make it: the source would not parse, or the mapping refused what
/// it cannot write. <see cref="ModelException.Diagnostics"/> holds the syntax errors that failed it, and the semantic
/// check's when asked for.
/// </summary>
public class ConversionException : ModelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">The failure, as the service worded it.</param>
    /// <param name="method">The method whose answer reported it.</param>
    /// <param name="diagnostics">The diagnostics the answer carried.</param>
    /// <param name="fromFormat">The format the source was read as; empty when the answer named none.</param>
    /// <param name="toFormat">The format it was to be written in; empty when the answer named none.</param>
    /// <param name="experimental">Whether the conversion went through the experimental mapping.</param>
    /// <param name="experimentalNotice">What is experimental about it; empty when it is not.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A diagnostic is null.</exception>
    public ConversionException(
        string message,
        string method,
        IEnumerable<Diagnostic> diagnostics,
        string fromFormat,
        string toFormat,
        bool experimental,
        string experimentalNotice)
        : base(message, method, diagnostics)
    {
        ArgumentNullException.ThrowIfNull(fromFormat);
        ArgumentNullException.ThrowIfNull(toFormat);
        ArgumentNullException.ThrowIfNull(experimentalNotice);
        FromFormat = fromFormat;
        ToFormat = toFormat;
        Experimental = experimental;
        ExperimentalNotice = experimentalNotice;
    }

    /// <summary>The format the source was read as, canonically named; empty when the answer named none.</summary>
    public string FromFormat { get; }

    /// <summary>The format it was to be written in, canonically named; empty when the answer named none.</summary>
    public string ToFormat { get; }

    /// <summary>Whether the conversion went through the experimental RDF mapping, whose refusals are part of what is experimental.</summary>
    public bool Experimental { get; }

    /// <summary>What is experimental about the conversion, in the service's own wording; empty when it is not.</summary>
    public string ExperimentalNotice { get; }
}
