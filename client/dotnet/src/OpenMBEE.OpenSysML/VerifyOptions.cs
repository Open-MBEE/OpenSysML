using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.VerifyConstraintAsync"/>, <see cref="Model.VerifyRequirementAsync"/> and
/// <see cref="Model.VerifySatisfactionAsync"/> ask: the question, the object the verdict is about, the values
/// bound to the element's <c>in</c> parameters, and the engine asked. Collections are copied when they are set.
/// A satisfaction is asked about an object of each assertion's own subject, so it takes neither a subject nor
/// arguments. A question or an engine the service does not know is refused by the service with
/// <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class VerifyOptions
{
    /// <summary>The question that evaluates the declared values, as the service asks when none is named.</summary>
    public const string Evaluate = "evaluate";

    /// <summary>The question whether the claim holds over every assignment of the free features, which a solver answers.</summary>
    public const string Holds = "holds";

    /// <summary>The question whether one assignment of the free features satisfies the claim, which a solver answers.</summary>
    public const string Satisfiable = "satisfiable";

    private readonly ValueList<Value> arguments;
    private readonly ValueDictionary<Value> namedArguments;

    /// <summary>An evaluation of the declared values, outside any object, by the engine the service chooses.</summary>
    public static VerifyOptions Default { get; } = new();

    /// <summary>
    /// The question asked: <see cref="Evaluate"/>, <see cref="Holds"/> or <see cref="Satisfiable"/>; null asks none,
    /// which the service answers as an evaluation. Either solver question needs <see cref="Capabilities.VerificationQuestions"/>.
    /// </summary>
    public string? Question { get; init; }

    /// <summary>
    /// The object the verdict is about: the qualified name of a part definition or usage the service builds an
    /// object of, or a path from one to a nested object (<c>Mission::mission.vehicle</c>, <c>convoy.escorts[2]</c>);
    /// null judges the declared values.
    /// </summary>
    public string? SubjectSymbolId { get; init; }

    /// <summary>
    /// Values for the element's <c>in</c> parameters, in declaration order. Binding any needs
    /// <see cref="Capabilities.VerificationArguments"/>, and a solver question takes none.
    /// </summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A value is null.</exception>
    public IReadOnlyList<Value> Arguments
    {
        get => arguments;
        init => arguments = new ValueList<Value>(value);
    }

    /// <summary>Values for the element's <c>in</c> parameters, by name; see <see cref="Arguments"/>.</summary>
    /// <exception cref="ArgumentNullException">The dictionary is null.</exception>
    /// <exception cref="ArgumentException">A name or a value is null.</exception>
    public IReadOnlyDictionary<string, Value> NamedArguments
    {
        get => namedArguments;
        init => namedArguments = new ValueDictionary<Value>(value);
    }

    /// <summary>
    /// The engine the question is put to: one <see cref="Connection.ListEnginesAsync"/> names, <c>auto</c> for the
    /// strongest covering one or <c>all</c> for every covering one; null leaves the choice to the service. Naming
    /// one needs <see cref="Capabilities.Engines"/>, and <c>explore</c> also <see cref="Capabilities.ScheduleExplore"/>.
    /// </summary>
    public string? Engine { get; init; }

    /// <summary>Whether any argument is bound.</summary>
    internal bool BindsArguments => arguments.Count > 0 || namedArguments.Count > 0;
}
