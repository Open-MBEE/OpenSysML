using System.Collections.Generic;
using System.Linq;
using Google.Protobuf;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Reads the model messages the service answers into the client's immutable records. Every field
/// is read whatever the service advertises; a value is read by <see cref="ValueWire"/>, which
/// names where in the answer it is when it is malformed or of a kind this client does not know.
/// </summary>
internal static class ModelWire
{
    /// <summary>Reads diagnostics, in report order.</summary>
    internal static ValueList<Diagnostic> Diagnostics(IEnumerable<Wire.Diagnostic> diagnostics) =>
        new(diagnostics.Select(Diagnostic));

    /// <summary>Reads one diagnostic.</summary>
    internal static Diagnostic Diagnostic(Wire.Diagnostic diagnostic) =>
        new(
            diagnostic.Severity,
            diagnostic.Message,
            diagnostic.Code,
            diagnostic.Span is { } span ? new SourceSpan(span.File, span.StartLine, span.StartCol, span.EndLine, span.EndCol) : null,
            diagnostic.OrderDependent,
            diagnostic.Independent);

    /// <summary>Reads a symbol.</summary>
    /// <param name="symbol">The message.</param>
    /// <param name="method">The method answered, for an error.</param>
    /// <param name="path">Where the symbol is in the answer, as <c>symbol</c>.</param>
    internal static Symbol Symbol(Wire.SymbolInfo symbol, string method, string path) =>
        new(
            symbol.Id,
            symbol.Name,
            symbol.Kind,
            symbol.Metadata,
            symbol.ChildIds,
            symbol.Attributes.Select((attribute, i) => Attribute(attribute, method, $"{path}.attributes[{i}]")),
            symbol.TypeInfo is { } type
                ? new TypeFacts(
                    type.Declared,
                    type.ResolvedId,
                    type.ResolvedKind,
                    type.Primitive,
                    type.PrimitiveSource,
                    type.Quantity,
                    type.Unit,
                    type.Types_.Select(derived => new TypeRef(derived.Id, derived.Kind)))
                : null,
            symbol.Multiplicity is { } multiplicity ? new Multiplicity(multiplicity.Lower, multiplicity.Upper) : null,
            symbol.Specializations.Select(edge => new Specialization(edge.Kind, edge.Declared, edge.TargetId, edge.TargetKind)),
            symbol.WithheldLibraryAttributes,
            symbol.Members.Select(member => new SymbolMember(member.Id, member.Name, member.Kind, member.Inherited, member.InheritedFrom, member.Library)),
            symbol.WithheldLibraryMembers);

    /// <summary>Reads an object the service built.</summary>
    /// <param name="instance">The message.</param>
    /// <param name="method">The method answered, for an error.</param>
    /// <param name="path">Where the object is in the answer, as <c>instances[2]</c>.</param>
    internal static Instance Instance(Wire.Instance instance, string method, string path) =>
        new(
            instance.Id,
            instance.TypeSymbolId,
            instance.FeatureValues.Select(entry => KeyValuePair.Create(
                entry.Key,
                FeatureValue(entry.Value, method, $"{path}.feature_values[{ValueText.Quoted(entry.Key)}]"))),
            instance.ReachedBy is { } reach
                ? new InstanceReach(reach.InstanceId, reach.FeatureName, reach.FeatureSymbolId, reach.Index, reach.Path)
                : null);

    /// <summary>Reads what a GetElementProperties call answered without an error.</summary>
    /// <param name="response">The message.</param>
    /// <param name="method">The method answered, for an error.</param>
    /// <exception cref="TransportException">The answer names no element.</exception>
    /// <exception cref="UnsupportedValueException">A value is of no kind this client knows.</exception>
    internal static ElementProperties ElementProperties(Wire.GetElementPropertiesResponse response, string method)
    {
        if (response.Element is not { } element)
        {
            throw new TransportException(StatusCode.Internal, $"{method} answered neither an element nor a failure.", method);
        }

        return new ElementProperties(
            ElementRef(element),
            response.Properties.Select((property, i) => new ElementProperty(
                property.Name,
                property.DefiningClass,
                property.Many,
                property.Derived,
                property.Values.Select((value, j) => PropertyValue(value, method, $"properties[{i}].values[{j}]")),
                property.Unsupported)));
    }

    /// <summary>Reads what an InstancePaths call answered without an error.</summary>
    /// <param name="response">The message.</param>
    /// <param name="method">The method answered, for an error.</param>
    /// <exception cref="TransportException">A path names no root.</exception>
    internal static InstancePaths InstancePaths(Wire.InstancePathsResponse response, string method) =>
        new(
            response.Paths.Select((path, i) => path.Root is { } root
                ? new InstancePath(
                    root.InstanceId,
                    root.SymbolId,
                    path.Path,
                    path.Steps.Select(step => new InstancePathStep(step.FeatureName, step.FeatureSymbolId, step.Index, step.InstanceId)))
                : throw new TransportException(StatusCode.Internal, $"{method} answered a path at paths[{i}] that names no root.", method)),
            response.Truncated);

    private static ElementRef ElementRef(Wire.ElementRef element) => new(element.Id, element.QualifiedName, element.Metaclass);

    private static ElementPropertyValue PropertyValue(Wire.ElementPropertyValue value, string method, string path) => value.ValueCase switch
    {
        Wire.ElementPropertyValue.ValueOneofCase.Element => new ElementPropertyValue.ElementValue(ElementRef(value.Element)),
        Wire.ElementPropertyValue.ValueOneofCase.StringValue => new ElementPropertyValue.TextValue(value.StringValue),
        Wire.ElementPropertyValue.ValueOneofCase.BoolValue => new ElementPropertyValue.BooleanValue(value.BoolValue),
        Wire.ElementPropertyValue.ValueOneofCase.IntValue => new ElementPropertyValue.IntegerValue(value.IntValue),
        Wire.ElementPropertyValue.ValueOneofCase.RealValue => new ElementPropertyValue.RealValue(value.RealValue),
        Wire.ElementPropertyValue.ValueOneofCase.Number => new ElementPropertyValue.NumberValue(value.Number),
        _ => throw Unsupported(value, method, path),
    };

    private static UnsupportedValueException Unsupported(Wire.ElementPropertyValue value, string method, string path)
    {
        var bytes = value.ToByteArray();
        var what = bytes.Length == 0
            ? "names no kind of value"
            : "is of a kind this client does not know; a newer client may read it";
        return new UnsupportedValueException($"The service answered a property value at {path} that {what}.", method, path, bytes);
    }

    private static AttributeInfo Attribute(Wire.AttributeInfo attribute, string method, string path) =>
        new(
            attribute.Name,
            attribute.Type,
            attribute.Value is { } value ? ValueWire.FromWire(value, method, $"{path}.value") : null,
            attribute.Unit);

    private static FeatureValue FeatureValue(Wire.FeatureValue feature, string method, string path) =>
        new(
            feature.FeatureName,
            feature.Value is { } value ? ValueWire.FromWire(value, method, $"{path}.value") : null,
            ValueWire.FromWire(feature.Values, method, $"{path}.values"),
            feature.Materialized,
            feature.Error,
            feature.OrderDependent);
}
