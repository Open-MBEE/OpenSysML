using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The service answered a migration but could not migrate the SysML v1 model at all: it would not read as XMI, or the
/// archive held none. An element with no v2 form is reported in <see cref="Migration.Report"/>, never raised.
/// </summary>
public class MigrationException : ModelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">The failure, as the service worded it.</param>
    /// <param name="method">The method whose answer reported it.</param>
    /// <param name="fromFormat">The v1 form read; empty when the answer named none.</param>
    /// <param name="toFormat">The format it was to be written in; empty when the answer named none.</param>
    /// <param name="experimental">Whether the service marked the migration experimental.</param>
    /// <param name="experimentalNotice">What is experimental about it.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public MigrationException(string message, string method, string fromFormat, string toFormat, bool experimental, string experimentalNotice)
        : base(message, method, [])
    {
        ArgumentNullException.ThrowIfNull(fromFormat);
        ArgumentNullException.ThrowIfNull(toFormat);
        ArgumentNullException.ThrowIfNull(experimentalNotice);
        FromFormat = fromFormat;
        ToFormat = toFormat;
        Experimental = experimental;
        ExperimentalNotice = experimentalNotice;
    }

    /// <summary>The v1 form read, canonically <c>xmi</c>; empty when the answer named none.</summary>
    public string FromFormat { get; }

    /// <summary>The format the model was to be written in; empty when the answer named none.</summary>
    public string ToFormat { get; }

    /// <summary>Whether the service marked the migration experimental, as it marks every one.</summary>
    public bool Experimental { get; }

    /// <summary>What is experimental about the migration, in the service's own wording.</summary>
    public string ExperimentalNotice { get; }
}
