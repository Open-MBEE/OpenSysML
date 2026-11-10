using System;
using System.Globalization;

namespace OpenMBEE.OpenSysML;

/// <summary>One feature walked along an <see cref="InstancePath"/>, and the object it reaches.</summary>
public sealed record InstancePathStep
{
    /// <summary>Creates a step.</summary>
    /// <param name="featureName">The feature of the object before this step, a key of its <see cref="Instance.FeatureValues"/>.</param>
    /// <param name="featureSymbolId">The qualified name of the feature as declared.</param>
    /// <param name="index">The position among the values of a multi-valued feature, counted from 1 as an object path indexes them; 0 for a single-valued one.</param>
    /// <param name="instanceId">The id of the object the step reaches.</param>
    /// <exception cref="ArgumentNullException">A string is null.</exception>
    public InstancePathStep(string featureName, string featureSymbolId, int index, long instanceId)
    {
        ArgumentNullException.ThrowIfNull(featureName);
        ArgumentNullException.ThrowIfNull(featureSymbolId);
        FeatureName = featureName;
        FeatureSymbolId = featureSymbolId;
        Index = index;
        InstanceId = instanceId;
    }

    /// <summary>The feature of the object before this step, a key of its <see cref="Instance.FeatureValues"/>.</summary>
    public string FeatureName { get; }

    /// <summary>The qualified name of the feature as declared.</summary>
    public string FeatureSymbolId { get; }

    /// <summary>The position among the values of a multi-valued feature, counted from 1 as an object path indexes them; 0 for a single-valued one.</summary>
    public int Index { get; }

    /// <summary>The id of the object the step reaches.</summary>
    public long InstanceId { get; }

    /// <summary>The step, as <c>wheels[2] -&gt; #7</c>.</summary>
    /// <returns>The step's text.</returns>
    public override string ToString() =>
        string.Create(CultureInfo.InvariantCulture, $"{FeatureName}{(Index > 0 ? $"[{Index}]" : string.Empty)} -> #{InstanceId}");
}
