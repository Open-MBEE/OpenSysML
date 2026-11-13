using System;
using System.Linq;

namespace OpenMBEE.OpenSysML.Typed;

/// <summary>
/// The accessors generated properties read an instance's feature values with, one per multiplicity: a
/// required single value, an optional one, and a list. Each reads the feature by name, decodes what it holds
/// with the property's decoder, and reports a value of another type or number with
/// <see cref="TypeMismatchException"/> rather than returning it as the declared type.
/// </summary>
/// <remarks>
/// A feature whose evaluation failed, or that the service never materialized, raises
/// <see cref="FeatureValueException"/> from every accessor. A feature the instance does not have at all,
/// and one that holds null or no value, holds nothing: the empty list or null for a list or optional
/// feature, a mismatch for a required one.
/// </remarks>
public static class TypedFeatures
{
    /// <summary>The value of a feature of multiplicity one.</summary>
    /// <typeparam name="T">The declared type.</typeparam>
    /// <param name="owner">The view whose instance holds the feature.</param>
    /// <param name="featureName">The feature's name.</param>
    /// <param name="decode">Decodes the value into the declared type.</param>
    /// <returns>The decoded value.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="FeatureValueException">Evaluating the feature failed, or it was never materialized.</exception>
    /// <exception cref="TypeMismatchException">The feature holds no value, several, or one of another type.</exception>
    public static T Required<T>(ITypedObject owner, string featureName, FeatureDecoder<T> decode)
        where T : notnull
    {
        var held = Read(owner, featureName, decode);
        if (held.Absent)
        {
            throw new TypeMismatchException(featureName, "a value", null);
        }

        if (held.Many is { } many)
        {
            throw new TypeMismatchException(featureName, "a single value", new Value.SequenceValue(many));
        }

        return IsEmpty(held.Single!)
            ? throw new TypeMismatchException(featureName, "a value", held.Single)
            : decode(featureName, held.Single!, owner.Instantiation);
    }

    /// <summary>The value of a feature of multiplicity <c>0..1</c> whose type is a reference type, or null when it holds none.</summary>
    /// <typeparam name="T">The declared type.</typeparam>
    /// <param name="owner">The view whose instance holds the feature.</param>
    /// <param name="featureName">The feature's name.</param>
    /// <param name="decode">Decodes the value into the declared type.</param>
    /// <returns>The decoded value, or null.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="FeatureValueException">Evaluating the feature failed, or it was never materialized.</exception>
    /// <exception cref="TypeMismatchException">The feature holds several values, or one of another type.</exception>
    public static T? Optional<T>(ITypedObject owner, string featureName, FeatureDecoder<T> decode)
        where T : class =>
        Single(owner, featureName, decode) is { } value ? decode(featureName, value, owner.Instantiation) : null;

    /// <summary>The value of a feature of multiplicity <c>0..1</c> whose type is a value type, or null when it holds none.</summary>
    /// <typeparam name="T">The declared type.</typeparam>
    /// <param name="owner">The view whose instance holds the feature.</param>
    /// <param name="featureName">The feature's name.</param>
    /// <param name="decode">Decodes the value into the declared type.</param>
    /// <returns>The decoded value, or null.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="FeatureValueException">Evaluating the feature failed, or it was never materialized.</exception>
    /// <exception cref="TypeMismatchException">The feature holds several values, or one of another type.</exception>
    public static T? OptionalValue<T>(ITypedObject owner, string featureName, FeatureDecoder<T> decode)
        where T : struct =>
        Single(owner, featureName, decode) is { } value ? decode(featureName, value, owner.Instantiation) : null;

    /// <summary>The values of a feature that may hold several, in the order the service reported them.</summary>
    /// <typeparam name="T">The declared element type.</typeparam>
    /// <param name="owner">The view whose instance holds the feature.</param>
    /// <param name="featureName">The feature's name.</param>
    /// <param name="decode">Decodes each value into the declared type.</param>
    /// <returns>The decoded values; empty when the feature holds none.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="FeatureValueException">Evaluating the feature failed, or it was never materialized.</exception>
    /// <exception cref="TypeMismatchException">A value is of another type.</exception>
    public static ValueList<T> List<T>(ITypedObject owner, string featureName, FeatureDecoder<T> decode)
        where T : notnull
    {
        var held = Read(owner, featureName, decode);
        if (held.Absent)
        {
            return default;
        }

        if (held.Many is { } many)
        {
            return new ValueList<T>(many.Select(value => decode(featureName, value, owner.Instantiation)));
        }

        return IsEmpty(held.Single!) ? default : new ValueList<T>([decode(featureName, held.Single!, owner.Instantiation)]);
    }

    // The one value an optional feature holds, or null when it holds none.
    private static Value? Single<T>(ITypedObject owner, string featureName, FeatureDecoder<T> decode)
    {
        var held = Read(owner, featureName, decode);
        if (held.Absent)
        {
            return null;
        }

        var value = held.Single;
        if (held.Many is { } many)
        {
            if (many.Count > 1)
            {
                throw new TypeMismatchException(featureName, "at most one value", new Value.SequenceValue(many));
            }

            value = many.Count == 0 ? null : many[0];
        }

        return value is null || IsEmpty(value) ? null : value;
    }

    private static bool IsEmpty(Value value) => value is Value.NullValue or Value.UnsetValue;

    // What the instance holds for the feature: nothing at all, one value, or several (a sequence counts as several).
    private static Held Read<T>(ITypedObject owner, string featureName, FeatureDecoder<T> decode)
    {
        ArgumentNullException.ThrowIfNull(owner);
        ArgumentNullException.ThrowIfNull(featureName);
        ArgumentNullException.ThrowIfNull(decode);
        if (!owner.Instance.FeatureValues.TryGetValue(featureName, out var feature))
        {
            return new Held(true, null, null);
        }

        if (feature.Error.Length > 0)
        {
            throw new FeatureValueException(featureName, feature.Error);
        }

        if (feature.Value is { } value)
        {
            if (value is Value.SequenceValue sequence)
            {
                return new Held(false, null, Supported(featureName, sequence.Elements));
            }

            ThrowIfUnsupported(featureName, value);
            return new Held(false, value, null);
        }

        if (!feature.Values.IsEmpty || feature.IsMaterialized)
        {
            return new Held(false, null, Supported(featureName, feature.Values));
        }

        throw new FeatureValueException(featureName, "the feature value is not materialized");
    }

    private static ValueList<Value> Supported(string featureName, ValueList<Value> values)
    {
        foreach (var value in values)
        {
            ThrowIfUnsupported(featureName, value);
        }

        return values;
    }

    // A null carrying a reason stands for a value the service could not send, not for no value.
    private static void ThrowIfUnsupported(string featureName, Value value)
    {
        if (value is Value.NullValue { Unsupported.Length: > 0 } unsupported)
        {
            throw new FeatureValueException(featureName, unsupported.Unsupported);
        }
    }

    private readonly record struct Held(bool Absent, Value? Single, ValueList<Value>? Many);
}
