using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How the traversal that serialized an object graph reached one object: the object holding it, the feature of that
/// holder it is a value of, and the path from the root. The traversal is depth-first from the root, over the features
/// of an object in name order (those the model declares first, then those the standard library derives, such as
/// <c>subparts</c>) and the values of a feature in their order, and reaches an object once, the first time. One object
/// is often a value of several features (a port and the connector end referencing it are one object), and
/// <see cref="Model.InstancePathsAsync"/> lists every path to it.
/// </summary>
public sealed record InstanceReach
{
    /// <summary>Creates a reach.</summary>
    /// <param name="instanceId">The id of the object holding this one.</param>
    /// <param name="featureName">The holder's feature, a key of its <see cref="Instance.FeatureValues"/>.</param>
    /// <param name="featureSymbolId">The qualified name of the feature as declared.</param>
    /// <param name="index">The position among the values of a multi-valued feature, counted from 1 as an object path indexes them (<c>wheels[2]</c>); 0 for a single-valued feature.</param>
    /// <param name="path">The path from the root: the feature names, indexed within a multi-valued feature, joined by dots (<c>powertrain.engine.fuelIn</c>, <c>wheels[2].hub</c>).</param>
    /// <exception cref="ArgumentNullException">A string is null.</exception>
    public InstanceReach(long instanceId, string featureName, string featureSymbolId, int index, string path)
    {
        ArgumentNullException.ThrowIfNull(featureName);
        ArgumentNullException.ThrowIfNull(featureSymbolId);
        ArgumentNullException.ThrowIfNull(path);
        InstanceId = instanceId;
        FeatureName = featureName;
        FeatureSymbolId = featureSymbolId;
        Index = index;
        Path = path;
    }

    /// <summary>The id of the object holding this one, which <see cref="Instantiation.FindInstance"/> resolves.</summary>
    public long InstanceId { get; }

    /// <summary>The holder's feature, a key of its <see cref="Instance.FeatureValues"/>.</summary>
    public string FeatureName { get; }

    /// <summary>The qualified name of the feature as declared.</summary>
    public string FeatureSymbolId { get; }

    /// <summary>The position among the values of a multi-valued feature, counted from 1 as an object path indexes them (<c>wheels[2]</c>); 0 for a single-valued feature.</summary>
    public int Index { get; }

    /// <summary>The path from the root: the feature names, indexed within a multi-valued feature, joined by dots (<c>powertrain.engine.fuelIn</c>, <c>wheels[2].hub</c>).</summary>
    public string Path { get; }

    /// <summary>The reach, as its path from the root.</summary>
    /// <returns>The reach's text.</returns>
    public override string ToString() => Path;
}
