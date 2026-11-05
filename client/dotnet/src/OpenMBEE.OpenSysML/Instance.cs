using System;
using System.Collections.Generic;
using System.Globalization;

namespace OpenMBEE.OpenSysML;

/// <summary>A runtime object the service built for a part or usage.</summary>
public sealed record Instance
{
    /// <summary>Creates an instance, copying its feature values.</summary>
    /// <param name="id">The id the service gave it, which a <see cref="Value.InstanceRefValue"/> refers to.</param>
    /// <param name="typeSymbolId">The qualified name of the definition or usage it is an object of.</param>
    /// <param name="featureValues">What it holds for each feature of its type, by feature name.</param>
    /// <exception cref="ArgumentNullException"><paramref name="typeSymbolId"/> or <paramref name="featureValues"/> is null.</exception>
    /// <exception cref="ArgumentException">A feature name repeats, or a name or value is null.</exception>
    public Instance(long id, string typeSymbolId, IEnumerable<KeyValuePair<string, FeatureValue>> featureValues)
    {
        ArgumentNullException.ThrowIfNull(typeSymbolId);
        Id = id;
        TypeSymbolId = typeSymbolId;
        FeatureValues = new ValueDictionary<FeatureValue>(featureValues);
    }

    /// <summary>The id the service gave it, which a <see cref="Value.InstanceRefValue"/> refers to.</summary>
    public long Id { get; }

    /// <summary>The qualified name of the definition or usage it is an object of.</summary>
    public string TypeSymbolId { get; }

    /// <summary>
    /// What it holds for each feature of its type, by feature name, filled under the
    /// <see cref="Capabilities.FeatureValues"/> capability.
    /// </summary>
    public ValueDictionary<FeatureValue> FeatureValues { get; }

    /// <summary>The instance, as <c>Demo::Vehicle#1</c>.</summary>
    /// <returns>The instance's text.</returns>
    public override string ToString() => string.Create(CultureInfo.InvariantCulture, $"{TypeSymbolId}#{Id}");
}
