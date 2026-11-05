using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A run the service reported failed in its answer: an action without an initial node, a symbol that
/// names no behavior, a performer that could not be made, a run still waiting when its
/// <see cref="RunOptions.Advance"/> ran out. It keeps what the answer reported of the run up to the failure.
/// </summary>
public class ExecutionException : ModelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">The failure, as the service worded it.</param>
    /// <param name="method">The method whose answer reported it.</param>
    /// <param name="diagnostics">The diagnostics the answer carried.</param>
    /// <param name="finalTime">The run's simulation clock when it stopped, in seconds.</param>
    /// <param name="trace">The run's trace up to the failure, when it was asked for.</param>
    /// <param name="traceDropped">How many of the oldest trace records the service discarded.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A diagnostic or a trace record is null.</exception>
    public ExecutionException(
        string message,
        string method,
        IEnumerable<Diagnostic> diagnostics,
        double finalTime,
        IEnumerable<DocumentEvent> trace,
        int traceDropped)
        : base(message, method, diagnostics)
    {
        FinalTime = finalTime;
        Trace = new ValueList<DocumentEvent>(trace);
        TraceDropped = traceDropped;
    }

    /// <summary>
    /// The run's simulation clock when it stopped, in seconds: the instant a run cut short by
    /// <see cref="RunOptions.Advance"/> waits past. A service without <see cref="Capabilities.FinalTime"/> reports 0.
    /// </summary>
    public double FinalTime { get; }

    /// <summary>The run's trace up to the failure, when <see cref="RunOptions.Trace"/> asked for it; empty otherwise.</summary>
    public ValueList<DocumentEvent> Trace { get; }

    /// <summary>How many of the oldest records the service's bound on held events discarded from <see cref="Trace"/>.</summary>
    public int TraceDropped { get; }
}
