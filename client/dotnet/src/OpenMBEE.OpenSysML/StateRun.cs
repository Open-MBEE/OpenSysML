using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>What one run of a state machine produced: what <see cref="Model.ExecuteStateAsync"/> answers.</summary>
public sealed record StateRun
{
    /// <summary>Creates a state run, copying its collections.</summary>
    /// <param name="statesVisited">The states the machine entered, in order.</param>
    /// <param name="finalContext">The machine's context when the run stopped, by name.</param>
    /// <param name="diagnostics">What the service reported about the run.</param>
    /// <param name="finalTime">The run's simulation clock when it ended, in seconds.</param>
    /// <param name="trace">The run's trace, when it was asked for.</param>
    /// <param name="traceDropped">How many of the oldest trace records the service discarded.</param>
    /// <param name="orderDependent">For each context entry an unresolved choice point touched, how many did.</param>
    /// <exception cref="ArgumentNullException">A collection is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null or repeats a name.</exception>
    public StateRun(
        IEnumerable<string> statesVisited,
        IEnumerable<KeyValuePair<string, Value>> finalContext,
        IEnumerable<Diagnostic> diagnostics,
        double finalTime,
        IEnumerable<DocumentEvent> trace,
        int traceDropped,
        IEnumerable<KeyValuePair<string, int>> orderDependent)
    {
        StatesVisited = new ValueList<string>(statesVisited);
        FinalContext = new ValueDictionary<Value>(finalContext);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
        FinalTime = finalTime;
        Trace = new ValueList<DocumentEvent>(trace);
        TraceDropped = traceDropped;
        OrderDependent = new ValueDictionary<int>(orderDependent);
    }

    /// <summary>
    /// The states the machine entered, in the order it entered them. The last is not where a machine with
    /// parallel regions rests, which is every region's active state; the service answers that only for an
    /// explored <see cref="Outcome.FinalState"/>.
    /// </summary>
    public ValueList<string> StatesVisited { get; }

    /// <summary>The machine's context when the run stopped: its own data and, under <c>this.</c>, the performer's attributes.</summary>
    public ValueDictionary<Value> FinalContext { get; }

    /// <summary>What the service reported about the run: its choice points and the guards it could not evaluate.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>
    /// The run's simulation clock when it ended, in seconds from the 0 it started at: the clock advances
    /// through every time-triggered transition. A service without <see cref="Capabilities.FinalTime"/> reports 0.
    /// </summary>
    public double FinalTime { get; }

    /// <summary>The run's trace in the order the run made it, when <see cref="RunOptions.Trace"/> asked for it; empty otherwise.</summary>
    public ValueList<DocumentEvent> Trace { get; }

    /// <summary>How many of the oldest records the service's bound on held events discarded from <see cref="Trace"/>.</summary>
    public int TraceDropped { get; }

    /// <summary>
    /// For each entry of <see cref="FinalContext"/> an unresolved choice point of the run touched, how many did;
    /// an entry nothing order-dependent wrote is absent. A service without <see cref="Capabilities.OrderDependence"/> reports none.
    /// </summary>
    public ValueDictionary<int> OrderDependent { get; }
}
