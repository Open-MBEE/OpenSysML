using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>What one run of an action produced: what <see cref="Model.ExecuteActionAsync"/> answers.</summary>
public sealed record ActionRun
{
    /// <summary>Creates an action run, copying its collections.</summary>
    /// <param name="outputs">The action's output parameters, by name.</param>
    /// <param name="diagnostics">What the service reported about the run.</param>
    /// <param name="finalTime">The run's simulation clock when it ended, in seconds.</param>
    /// <param name="performerAttributes">The attributes the performing object holds when the run ends, keyed as <c>this.level</c>.</param>
    /// <param name="orderDependent">For each output an unresolved choice point touched, how many did.</param>
    /// <exception cref="ArgumentNullException">A collection is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null or repeats a name.</exception>
    public ActionRun(
        IEnumerable<KeyValuePair<string, Value>> outputs,
        IEnumerable<Diagnostic> diagnostics,
        double finalTime,
        IEnumerable<KeyValuePair<string, Value>> performerAttributes,
        IEnumerable<KeyValuePair<string, int>> orderDependent)
        : this(outputs, diagnostics, finalTime, performerAttributes, orderDependent, [], 0)
    {
    }

    /// <summary>Creates an action run with its trace, copying its collections.</summary>
    /// <param name="outputs">The action's output parameters, by name.</param>
    /// <param name="diagnostics">What the service reported about the run.</param>
    /// <param name="finalTime">The run's simulation clock when it ended, in seconds.</param>
    /// <param name="performerAttributes">The attributes the performing object holds when the run ends.</param>
    /// <param name="orderDependent">For each output an unresolved choice point touched, how many did.</param>
    /// <param name="trace">The trace records, in the order the run made them.</param>
    /// <param name="traceDropped">How many oldest records the service's held-events bound discarded.</param>
    /// <exception cref="ArgumentNullException">A collection is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null or repeats a name.</exception>
    public ActionRun(
        IEnumerable<KeyValuePair<string, Value>> outputs,
        IEnumerable<Diagnostic> diagnostics,
        double finalTime,
        IEnumerable<KeyValuePair<string, Value>> performerAttributes,
        IEnumerable<KeyValuePair<string, int>> orderDependent,
        IEnumerable<DocumentEvent> trace,
        int traceDropped)
    {
        Outputs = new ValueDictionary<Value>(outputs);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
        FinalTime = finalTime;
        PerformerAttributes = new ValueDictionary<Value>(performerAttributes);
        OrderDependent = new ValueDictionary<int>(orderDependent);
        Trace = new ValueList<DocumentEvent>(trace);
        TraceDropped = traceDropped;
    }

    /// <summary>The action's output parameters, by name; empty for an action producing none.</summary>
    public ValueDictionary<Value> Outputs { get; }

    /// <summary>What the service reported about the run: its choice points and the guards it could not evaluate.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>
    /// The run's simulation clock when it ended, in seconds from the 0 it started at: the clock advances
    /// through every wait. A service without <see cref="Capabilities.FinalTime"/> reports 0 whatever the run waited on.
    /// </summary>
    public double FinalTime { get; }

    /// <summary>
    /// The attributes the object named by <see cref="RunOptions.PerformerSymbolId"/> holds when the run ends,
    /// keyed as an explored <see cref="Outcome"/>'s outputs key them (<c>this.level</c>); empty without a performer.
    /// </summary>
    public ValueDictionary<Value> PerformerAttributes { get; }

    /// <summary>
    /// For each output an unresolved choice point of the run touched, how many did, so its value stands only
    /// up to the order the run took; an output nothing order-dependent wrote is absent. A service without
    /// <see cref="Capabilities.OrderDependence"/> reports none.
    /// </summary>
    public ValueDictionary<int> OrderDependent { get; }

    /// <summary>
    /// The run's trace in the order the run made it, when <see cref="RunOptions.Trace"/> asked for it: the token steps,
    /// waits, accepts, sends, decisions, assignments and completion of the action; empty otherwise.
    /// </summary>
    public ValueList<DocumentEvent> Trace { get; }

    /// <summary>How many of the oldest records the service's bound on held events discarded from <see cref="Trace"/>.</summary>
    public int TraceDropped { get; }
}
