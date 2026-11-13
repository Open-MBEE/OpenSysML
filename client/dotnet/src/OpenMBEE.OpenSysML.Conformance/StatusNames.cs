using System;
using System.Globalization;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Status codes as scenarios spell them: the canonical gRPC names (<c>NOT_FOUND</c>), which are the
/// protocol's, not any one language's enum spelling.
/// </summary>
internal static class StatusNames
{
    /// <summary>A status code's canonical name; one the protocol does not define is <c>Code(n)</c>.</summary>
    /// <param name="code">The code.</param>
    /// <returns>Its name.</returns>
    internal static string Name(StatusCode code) => code switch
    {
        StatusCode.Ok => "OK",
        StatusCode.Cancelled => "CANCELLED",
        StatusCode.Unknown => "UNKNOWN",
        StatusCode.InvalidArgument => "INVALID_ARGUMENT",
        StatusCode.DeadlineExceeded => "DEADLINE_EXCEEDED",
        StatusCode.NotFound => "NOT_FOUND",
        StatusCode.AlreadyExists => "ALREADY_EXISTS",
        StatusCode.PermissionDenied => "PERMISSION_DENIED",
        StatusCode.ResourceExhausted => "RESOURCE_EXHAUSTED",
        StatusCode.FailedPrecondition => "FAILED_PRECONDITION",
        StatusCode.Aborted => "ABORTED",
        StatusCode.OutOfRange => "OUT_OF_RANGE",
        StatusCode.Unimplemented => "UNIMPLEMENTED",
        StatusCode.Internal => "INTERNAL",
        StatusCode.Unavailable => "UNAVAILABLE",
        StatusCode.DataLoss => "DATA_LOSS",
        StatusCode.Unauthenticated => "UNAUTHENTICATED",
        _ => Numbered(code),
    };

    /// <summary>
    /// Whether a code is the one a scenario names: by its canonical name or the reference runner's
    /// camel-case spelling of it (<c>NotFound</c>), in either case.
    /// </summary>
    /// <param name="want">The name the scenario states.</param>
    /// <param name="got">The code the call ended with.</param>
    /// <returns>Whether they are the same status.</returns>
    internal static bool Same(string want, StatusCode got)
    {
        ArgumentNullException.ThrowIfNull(want);
        return string.Equals(want, Name(got), StringComparison.OrdinalIgnoreCase)
            || string.Equals(want, CamelName(got), StringComparison.OrdinalIgnoreCase);
    }

    private static string CamelName(StatusCode code) => code switch
    {
        StatusCode.Cancelled => "Canceled",
        StatusCode.Ok => "OK",
        _ when Enum.IsDefined(code) => code.ToString(),
        _ => Numbered(code),
    };

    private static string Numbered(StatusCode code) =>
        $"Code({((int)code).ToString(CultureInfo.InvariantCulture)})";
}
