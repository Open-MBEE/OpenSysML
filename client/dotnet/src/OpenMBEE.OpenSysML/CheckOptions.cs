using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.CheckActionAsync"/> and <see cref="Model.CheckStateAsync"/> look for, the bounds
/// of the search and the engine that searches, each as the <c>sysml</c> command's <c>-check-*</c> flag of
/// the same name sets it. Lists are copied when they are set. A negative bound or timeout and an engine
/// the service does not know are refused by the service with <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class CheckOptions
{
    private readonly ValueList<string> properties;
    private readonly ValueList<string> diverge;
    private readonly ValueList<string> freeInputs;
    private readonly ValueList<string> assume;

    /// <summary>No property, the engine's default bounds and the check engine, outside any object.</summary>
    public static CheckOptions Default { get; } = new();

    /// <summary>
    /// Constraints or requirements evaluated at every stable state, by qualified name (<c>-check-property</c>);
    /// one false at a state is a violation.
    /// </summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A name is null.</exception>
    public IReadOnlyList<string> Properties
    {
        get => properties;
        init => properties = new ValueList<string>(value);
    }

    /// <summary>
    /// Features whose final value the schedule may decide (<c>-check-diverge</c>): the action's own bare, a
    /// performed node's as <c>node.pin</c>, the performer's as <c>this.&lt;name&gt;</c>.
    /// </summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A feature is null.</exception>
    public IReadOnlyList<string> Diverge
    {
        get => diverge;
        init => diverge = new ValueList<string>(value);
    }

    /// <summary>Inputs left free for the symbolic engine to range over (<c>-check-input</c>).</summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">An input is null.</exception>
    public IReadOnlyList<string> FreeInputs
    {
        get => freeInputs;
        init => freeInputs = new ValueList<string>(value);
    }

    /// <summary>Constraints or requirements the symbolic engine assumes (<c>-check-assume</c>).</summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A name is null.</exception>
    public IReadOnlyList<string> Assume
    {
        get => assume;
        init => assume = new ValueList<string>(value);
    }

    /// <summary>
    /// Whether each violation and divergent value carries its witness file, the replay
    /// <c>-schedule replay:&lt;file&gt;</c> follows (<c>-check-witness</c>), in <see cref="CheckWitness.FileContent"/>.
    /// </summary>
    public bool Witness { get; init; }

    /// <summary>The bound on moves per schedule (<c>-check-depth</c>); 0 for the engine's default.</summary>
    public int Depth { get; init; }

    /// <summary>The bound on states visited (<c>-check-states</c>); 0 for the engine's default.</summary>
    public int States { get; init; }

    /// <summary>The bound on loop unrollings (<c>-check-unroll</c>); 0 for the engine's default.</summary>
    public int Unroll { get; init; }

    /// <summary>The seconds the search, and each solver query, may take (<c>-check-timeout</c>); 0 for none.</summary>
    public double Timeout { get; init; }

    /// <summary>
    /// The engine that answers: <c>check</c>, the service's default, <c>smt</c>, or another engine the service
    /// lists; null for the default. Naming one needs <see cref="Capabilities.Engines"/>, and <c>explore</c>,
    /// which answers what the explore schedule does, also <see cref="Capabilities.ScheduleExplore"/>.
    /// </summary>
    public string? Engine { get; init; }

    /// <summary>
    /// The object the behavior is performed by, as <see cref="RunOptions.PerformerSymbolId"/> names it; null
    /// for none. Needs <see cref="Capabilities.Performer"/>.
    /// </summary>
    public string? PerformerSymbolId { get; init; }

    /// <summary>Whether <see cref="Engine"/> selects the explore engine, as the service reads its name.</summary>
    internal bool Explores => Engine is { } engine && engine.Trim() == RunOptions.Explore;
}
