using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>What an <see cref="Instance"/> holds for one feature of its type.</summary>
public sealed record FeatureValue
{
    /// <summary>Creates a feature value.</summary>
    /// <param name="featureName">The feature's name.</param>
    /// <param name="value">The value of a single-valued feature; null for a multi-valued one, one not held, or a failure.</param>
    /// <param name="values">The values of a multi-valued feature; empty for a single-valued one.</param>
    /// <param name="isMaterialized">Whether the object holds the feature at all.</param>
    /// <param name="error">Why evaluating the feature failed; empty when it did not.</param>
    /// <param name="orderDependent">How many of the run's unresolved, conflicting choice points touch the value; 0 for none.</param>
    /// <exception cref="ArgumentNullException"><paramref name="featureName"/>, <paramref name="values"/> or <paramref name="error"/> is null.</exception>
    /// <exception cref="ArgumentException">An element of <paramref name="values"/> is null.</exception>
    public FeatureValue(string featureName, Value? value, IEnumerable<Value> values, bool isMaterialized, string error, int orderDependent)
    {
        ArgumentNullException.ThrowIfNull(featureName);
        ArgumentNullException.ThrowIfNull(error);
        FeatureName = featureName;
        Value = value;
        Values = new ValueList<Value>(values);
        IsMaterialized = isMaterialized;
        Error = error;
        OrderDependent = orderDependent;
    }

    /// <summary>The feature's name.</summary>
    public string FeatureName { get; }

    /// <summary>The value of a single-valued feature; null for a multi-valued one, one not held, or a failure.</summary>
    public Value? Value { get; }

    /// <summary>The values of a multi-valued feature; empty for a single-valued one.</summary>
    public ValueList<Value> Values { get; }

    /// <summary>Whether the object holds the feature at all.</summary>
    public bool IsMaterialized { get; }

    /// <summary>Why evaluating the feature failed, as <c>cyclic …</c>; empty when it did not. The rest of the object stays readable.</summary>
    public string Error { get; }

    /// <summary>
    /// How many of the run's unresolved choice points, whose alternatives conflict, touch this value: it
    /// then stands only up to the order the run took. 0 for a value nothing order-dependent wrote.
    /// Filled under the <see cref="Capabilities.OrderDependence"/> capability.
    /// </summary>
    public int OrderDependent { get; }
}
