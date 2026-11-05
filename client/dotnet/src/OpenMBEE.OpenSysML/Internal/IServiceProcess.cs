using System;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// A started private service as <see cref="ServiceRegistry"/> manages it: where it listens, its process,
/// when that process ends, and how to stop it.
/// </summary>
internal interface IServiceProcess
{
    /// <summary>The base URL it reported, on a loopback address.</summary>
    Uri Address { get; }

    /// <summary>Its process id.</summary>
    int ProcessId { get; }

    /// <summary>Completes once its process has exited, for any reason, with how it ended.</summary>
    Task<string> Exited { get; }

    /// <summary>
    /// Stops it: closes its input, which it exits on, waits out its grace period, then kills it and
    /// every process it started, waits for the exit and drains its output. Calling again waits for the
    /// same stop. Faults with <see cref="OpenSysMLException"/> naming the process when it could not
    /// be made to exit.
    /// </summary>
    /// <returns>When it has exited and its output is drained.</returns>
    Task StopAsync();

    /// <summary>Closes its input without waiting for it to exit, for an unload that cannot wait.</summary>
    void CloseInput();

    /// <summary>The last lines it wrote, as they end an error message; empty when there are none.</summary>
    /// <returns>The lines, each on its own indented line.</returns>
    string DescribeTail();
}
