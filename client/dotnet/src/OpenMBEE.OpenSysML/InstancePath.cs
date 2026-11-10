using System;
using System.Collections.Generic;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One root-relative path to an object: the root it starts from, an object the population holds on its own, which
/// <see cref="Model.InstantiateAsync"/> created, and the features walked from it, each with the object it reaches.
/// </summary>
public sealed record InstancePath
{
    /// <summary>Creates a path, copying its steps.</summary>
    /// <param name="rootInstanceId">The id of the root the path starts from.</param>
    /// <param name="rootSymbolId">The qualified name the root was instantiated as; empty when the name no longer binds it.</param>
    /// <param name="path">The steps as an object path (<c>powertrain.engine.fuelIn</c>, <c>wheels[2].hub</c>); empty when the object is the root.</param>
    /// <param name="steps">The steps, one per feature walked.</param>
    /// <exception cref="ArgumentNullException">A string or <paramref name="steps"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="steps"/> holds a null.</exception>
    public InstancePath(long rootInstanceId, string rootSymbolId, string path, IEnumerable<InstancePathStep> steps)
    {
        ArgumentNullException.ThrowIfNull(rootSymbolId);
        ArgumentNullException.ThrowIfNull(path);
        RootInstanceId = rootInstanceId;
        RootSymbolId = rootSymbolId;
        Path = path;
        Steps = new ValueList<InstancePathStep>(steps);
    }

    /// <summary>The id of the root the path starts from.</summary>
    public long RootInstanceId { get; }

    /// <summary>The qualified name the root was instantiated as, the label the path starts with; empty when the name no longer binds it, and <c>#&lt;id&gt;</c> addresses it.</summary>
    public string RootSymbolId { get; }

    /// <summary>The steps as an object path (<c>powertrain.engine.fuelIn</c>, <c>wheels[2].hub</c>); empty when the object is the root.</summary>
    public string Path { get; }

    /// <summary>The steps, one per feature walked, in order.</summary>
    public ValueList<InstancePathStep> Steps { get; }

    /// <summary>
    /// The path as an object reference: the root's label, its qualified name or <c>#&lt;id&gt;</c>, then the steps
    /// (<c>Garage::car.wheels[2]</c>), which <see cref="Model.InstancePathsAtAsync"/> and <see cref="DocumentValue.ObjectRef(string)"/> take back.
    /// </summary>
    /// <returns>The path's text.</returns>
    public override string ToString()
    {
        var root = RootSymbolId.Length > 0 ? RootSymbolId : $"#{ValueText.Whole(RootInstanceId)}";
        return Path.Length > 0 ? $"{root}.{Path}" : root;
    }
}
