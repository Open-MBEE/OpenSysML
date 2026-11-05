using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One distinct outcome an exploration reached: what the runs reaching it ended with, how many
/// orders of the choice points reached it, and the choices of one run that did. Runs agreeing on
/// their outputs are one outcome; runs failing alike are an outcome too, with its <see cref="Error"/>.
/// </summary>
public sealed record Outcome
{
    /// <summary>Creates an outcome, copying its collections.</summary>
    /// <param name="outputs">What the runs ended with, by name.</param>
    /// <param name="finalState">The state a machine rests in; empty for an action.</param>
    /// <param name="statesVisited">The states a machine entered, in order.</param>
    /// <param name="error">What the runs failed with; empty when they completed.</param>
    /// <param name="linearizations">How many orders within the budget reached the outcome.</param>
    /// <param name="witness">One run's choices, in run order.</param>
    /// <param name="diagnostics">What the witness run noted about itself.</param>
    /// <param name="probability">The exact model-draw probability; zero when inexact or absent.</param>
    /// <param name="probabilityRange">The probability bounds over schedulers; null when the exploration made no weighted choice.</param>
    /// <exception cref="ArgumentNullException">A string or a collection is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null or repeats a name.</exception>
    public Outcome(
        IEnumerable<KeyValuePair<string, Value>> outputs,
        string finalState,
        IEnumerable<string> statesVisited,
        string error,
        int linearizations,
        IEnumerable<string> witness,
        IEnumerable<Diagnostic> diagnostics,
        double probability,
        ProbabilityRange? probabilityRange)
    {
        ArgumentNullException.ThrowIfNull(finalState);
        ArgumentNullException.ThrowIfNull(error);
        Outputs = new ValueDictionary<Value>(outputs);
        FinalState = finalState;
        StatesVisited = new ValueList<string>(statesVisited);
        Error = error;
        Linearizations = linearizations;
        Witness = new ValueList<string>(witness);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
        Probability = probability;
        ProbabilityRange = probabilityRange;
    }

    /// <summary>
    /// The values the runs ended with: an action's outputs and, under <c>this.</c>, its performer's attributes,
    /// or a state machine's final context; empty for runs that failed.
    /// </summary>
    public ValueDictionary<Value> Outputs { get; }

    /// <summary>The state a machine rests in; empty for an action.</summary>
    public string FinalState { get; }

    /// <summary>The states a machine entered, in order; empty for an action.</summary>
    public ValueList<string> StatesVisited { get; }

    /// <summary>What the runs reaching this outcome failed with; empty when they completed.</summary>
    public string Error { get; }

    /// <summary>Whether the runs reaching this outcome completed rather than failed.</summary>
    public bool Completed => Error.Length == 0;

    /// <summary>How many linearizations within the budget reached this outcome.</summary>
    public int Linearizations { get; }

    /// <summary>
    /// One run's choice sequence in run order, one entry per choice point it resolved, each spelling the
    /// alternatives and the one taken; empty when the run faced no choice point.
    /// </summary>
    public ValueList<string> Witness { get; }

    /// <summary>What the witness run noted about itself: its choice points and the guards it could not evaluate.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>The exact model-draw probability when <see cref="ProbabilityRange"/> is exact; zero otherwise.</summary>
    public double Probability { get; }

    /// <summary>The model-draw probability bounds over schedulers; null when the exploration made no weighted choice.</summary>
    public ProbabilityRange? ProbabilityRange { get; }
}
