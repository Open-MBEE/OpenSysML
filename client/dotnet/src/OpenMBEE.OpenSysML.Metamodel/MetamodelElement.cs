using System;
using System.Collections;
using System.Collections.Generic;
using System.Text.Json;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// The base of every generated element class in <c>OpenMBEE.OpenSysML.Metamodel.Elements</c>: one sealed class per concrete
/// metaclass, implementing that metaclass's interface and so the interface of every ancestor. A property
/// the document supplies reads as its value; one it does not supply throws <see cref="NotSuppliedException"/>
/// rather than reading as an invented default, and one whose value does not fit it or whose reference the
/// document does not resolve throws <see cref="MalformedValueException"/> or
/// <see cref="UnresolvedReferenceException"/>.
/// </summary>
/// <remarks>
/// Elements are created and supplied only while a document is read, and are immutable afterwards, so
/// any number of threads may read them.
/// </remarks>
public abstract class MetamodelElement : IMetamodelElement
{
    private readonly Dictionary<MetaProperty, object?> values = [];
    private IReadOnlyDictionary<string, JsonElement>? extensions;

    private protected MetamodelElement(MetaclassInfo metaclass, string id, string jsonType)
    {
        ArgumentNullException.ThrowIfNull(metaclass);
        ArgumentNullException.ThrowIfNull(id);
        ArgumentNullException.ThrowIfNull(jsonType);
        MetaclassInfo = metaclass;
        Id = id;
        JsonType = jsonType;
    }

    /// <inheritdoc/>
    public string Id { get; }

    /// <inheritdoc/>
    public string JsonType { get; }

    /// <inheritdoc/>
    public MetaclassInfo MetaclassInfo { get; }

    /// <inheritdoc/>
    public IReadOnlyDictionary<string, JsonElement> Extensions =>
        extensions ?? throw new InvalidOperationException(
            $"{this} was read without its extensions; read the document with ReadOptions.KeepExtensions to keep the keys its metaclass has no property for");

    /// <inheritdoc/>
    public object? GetValue(string jsonKey)
    {
        var property = MetaclassInfo.GetProperty(jsonKey);
        if (!values.TryGetValue(property, out var value))
        {
            throw new NotSuppliedException(property, Id);
        }

        return value switch
        {
            Unresolved unresolved => throw unresolved.Failure(property, Id),
            Malformed malformed => throw new MalformedValueException(property, Id, malformed.Reason),
            OutOfRange outOfRange => outOfRange.Value,
            null when property.IsMany => TypedList.Empty(property.ValueType),
            _ => value,
        };
    }

    /// <summary>The metaclass name and <c>@id</c>, for diagnostics.</summary>
    /// <returns>For example <c>PartUsage 'a1b2'</c>.</returns>
    public override string ToString() => $"{MetaclassInfo.Name} '{Id}'";

    /// <summary>
    /// Records the value a document supplies for one of this element's properties: null, a value of the
    /// property's <see cref="MetaProperty.ValueType"/>, or for a many-valued property an immutable
    /// <see cref="IReadOnlyList{T}"/> of them. Null supplied to a many-valued property reads as empty.
    /// </summary>
    /// <exception cref="ArgumentException">The property is not one of this element's metaclass, or the value does not fit it.</exception>
    /// <exception cref="InvalidOperationException">The property was supplied already.</exception>
    internal void Supply(MetaProperty property, object? value)
    {
        Admit(property);
        if (value is not null && !Fits(property, value))
        {
            throw new ArgumentException(
                $"{property.QualifiedName} of {this} holds {property.ValueType.Name}{(property.IsMany ? " lists" : string.Empty)}, not {value.GetType().Name}",
                nameof(value));
        }

        values.Add(property, value);
    }

    /// <summary>
    /// Records a reference the document does not resolve, naming exactly one of an <c>@id</c> and an
    /// <c>@ref</c>: every read of the property, typed or through <see cref="GetValue"/>, throws
    /// <see cref="UnresolvedReferenceException"/>.
    /// </summary>
    /// <exception cref="ArgumentException">The property is not one of this element's metaclass or holds no references, or not exactly one target is named.</exception>
    /// <exception cref="InvalidOperationException">The property was supplied already.</exception>
    internal void SupplyUnresolved(MetaProperty property, string? targetId, string? targetRef)
    {
        Admit(property);
        if (property.RangeKind != PropertyRangeKind.Metaclass || (targetId is null) == (targetRef is null))
        {
            throw new ArgumentException($"{property.QualifiedName} of {this} cannot hold an unresolved reference to {targetId ?? targetRef}", nameof(property));
        }

        values.Add(property, new Unresolved(targetId, targetRef));
    }

    /// <summary>
    /// Records a value the document supplies that does not fit the property: every read of the property, typed
    /// or through <see cref="GetValue"/>, throws <see cref="MalformedValueException"/> with <paramref name="reason"/>.
    /// </summary>
    /// <exception cref="ArgumentException">The property is not one of this element's metaclass.</exception>
    /// <exception cref="InvalidOperationException">The property was supplied already.</exception>
    internal void SupplyMalformed(MetaProperty property, string reason)
    {
        Admit(property);
        values.Add(property, new Malformed(reason));
    }

    /// <summary>
    /// Records references to elements outside the property's range, kept when range checks are off: an
    /// element, or for a many-valued property an immutable list of elements. Its typed read throws
    /// <see cref="MalformedValueException"/> with <paramref name="reason"/>; <see cref="GetValue"/> reads it.
    /// </summary>
    /// <exception cref="ArgumentException">The property is not one of this element's metaclass or holds no references, or the value does not fit it.</exception>
    /// <exception cref="InvalidOperationException">The property was supplied already.</exception>
    internal void SupplyOutOfRange(MetaProperty property, object value, string reason)
    {
        Admit(property);
        var fits = property.IsMany
            ? value is IReadOnlyList<IMetamodelElement> list && !Contains(list, null)
            : value is IMetamodelElement;
        if (property.RangeKind != PropertyRangeKind.Metaclass || !fits)
        {
            throw new ArgumentException($"{property.QualifiedName} of {this} cannot hold {value.GetType().Name} out of its range", nameof(value));
        }

        values.Add(property, new OutOfRange(value, reason));
    }

    /// <summary>Records the members of the element's object its metaclass has no property for, <c>@id</c> and <c>@type</c> aside.</summary>
    /// <exception cref="InvalidOperationException">They were recorded already.</exception>
    internal void KeepExtensions(IReadOnlyDictionary<string, JsonElement> kept)
    {
        ArgumentNullException.ThrowIfNull(kept);
        if (extensions is not null)
        {
            throw new InvalidOperationException($"the extensions of {this} are kept already");
        }

        extensions = kept;
    }

    /// <summary>Whether the document supplied the property, even as null.</summary>
    internal bool IsSupplied(MetaProperty property) => values.ContainsKey(property);

    /// <summary>A single string-valued property: null when the document supplies null.</summary>
    /// <exception cref="NotSuppliedException">The document does not supply the property.</exception>
    /// <exception cref="MalformedValueException">The document's value does not fit the property.</exception>
    private protected string? ReadText(MetaProperty property) => (string?)Read(property);

    /// <summary>A single Boolean, integer, real or enumeration property: null when the document supplies null.</summary>
    /// <exception cref="NotSuppliedException">The document does not supply the property.</exception>
    /// <exception cref="MalformedValueException">The document's value does not fit the property.</exception>
    private protected T? ReadValue<T>(MetaProperty property)
        where T : struct => Read(property) is { } value ? (T)value : null;

    /// <summary>A single reference to another element: null when the document supplies null.</summary>
    /// <exception cref="NotSuppliedException">The document does not supply the property.</exception>
    /// <exception cref="UnresolvedReferenceException">The document does not resolve the reference.</exception>
    /// <exception cref="MalformedValueException">The document's value does not fit the property, or the reference is outside its range.</exception>
    private protected T? ReadReference<T>(MetaProperty property)
        where T : class => (T?)Read(property);

    /// <summary>A many-valued property: empty when the document supplies null.</summary>
    /// <exception cref="NotSuppliedException">The document does not supply the property.</exception>
    /// <exception cref="UnresolvedReferenceException">The document does not resolve one of the references.</exception>
    /// <exception cref="MalformedValueException">The document's value does not fit the property, or a reference is outside its range.</exception>
    private protected IReadOnlyList<T> ReadList<T>(MetaProperty property) =>
        Read(property) is { } value ? (IReadOnlyList<T>)value : [];

    private object? Read(MetaProperty property)
    {
        if (!values.TryGetValue(property, out var value))
        {
            throw new NotSuppliedException(property, Id);
        }

        return value switch
        {
            Unresolved unresolved => throw unresolved.Failure(property, Id),
            Malformed malformed => throw new MalformedValueException(property, Id, malformed.Reason),
            OutOfRange outOfRange => throw new MalformedValueException(
                property, Id, $"{outOfRange.Reason}; read with range checks off, GetValue(\"{property.JsonKey}\") returns it"),
            _ => value,
        };
    }

    private void Admit(MetaProperty property)
    {
        ArgumentNullException.ThrowIfNull(property);
        if (!MetaclassInfo.TryGetProperty(property.JsonKey, out var effective) || !ReferenceEquals(effective, property))
        {
            throw new ArgumentException($"{property.QualifiedName} is not a property of {MetaclassInfo.Name}", nameof(property));
        }

        if (values.ContainsKey(property))
        {
            throw new InvalidOperationException($"{property.QualifiedName} of {this} is supplied already");
        }
    }

    private static bool Fits(MetaProperty property, object value)
    {
        if (!property.IsMany)
        {
            return property.ValueType.IsInstanceOfType(value);
        }

        return typeof(IReadOnlyList<>).MakeGenericType(property.ValueType).IsInstanceOfType(value) && !Contains((IEnumerable)value, null);
    }

    private static bool Contains(IEnumerable items, object? sought)
    {
        foreach (var item in items)
        {
            if (Equals(item, sought))
            {
                return true;
            }
        }

        return false;
    }

    /// <summary>A reference the document does not resolve.</summary>
    private sealed record Unresolved(string? TargetId, string? TargetRef)
    {
        internal UnresolvedReferenceException Failure(MetaProperty property, string elementId) => new(property, elementId, TargetId, TargetRef);
    }

    /// <summary>A value that does not fit the property.</summary>
    private sealed record Malformed(string Reason);

    /// <summary>References outside the property's range, kept with range checks off.</summary>
    private sealed record OutOfRange(object Value, string Reason);
}
