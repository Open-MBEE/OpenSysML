using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// <see cref="Connection.StopSharedServicesAsync"/> attempted to stop every shared private service,
/// and some could not be stopped cleanly. <see cref="Failures"/> holds one failure per such service,
/// each naming its process; the others were stopped.
/// </summary>
public class ServiceStopException : OpenSysMLException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="failures">Why each service that was not stopped cleanly was not; at least one.</param>
    /// <exception cref="ArgumentNullException"><paramref name="failures"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="failures"/> is empty or holds null.</exception>
    public ServiceStopException(IEnumerable<Exception> failures)
        : this(Copy(failures))
    {
    }

    private ServiceStopException(Exception[] failures)
        : base(Describe(failures), failures.Length == 1 ? failures[0] : new AggregateException(failures))
    {
        Failures = Array.AsReadOnly(failures);
    }

    /// <summary>A read-only snapshot of why each service that was not stopped cleanly was not.</summary>
    public IReadOnlyList<Exception> Failures { get; }

    private static Exception[] Copy(IEnumerable<Exception> failures)
    {
        ArgumentNullException.ThrowIfNull(failures);
        var copied = failures.ToArray();
        if (copied.Length == 0 || copied.Contains(null))
        {
            throw new ArgumentException("A stop failure needs at least one cause, and no null one.", nameof(failures));
        }

        return copied;
    }

    private static string Describe(Exception[] failures) =>
        $"{failures.Length} private sysml-grpc service{(failures.Length == 1 ? " was" : "s were")} not stopped cleanly: "
        + string.Join("; ", failures.Select(failure => failure.Message));
}
