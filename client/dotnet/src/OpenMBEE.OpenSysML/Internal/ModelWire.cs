using System.Collections.Generic;
using System.Linq;

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
                ? new TypeFacts(type.Declared, type.ResolvedId, type.ResolvedKind, type.Primitive, type.PrimitiveSource, type.Quantity, type.Unit)
                : null,
            symbol.Multiplicity is { } multiplicity ? new Multiplicity(multiplicity.Lower, multiplicity.Upper) : null,
            symbol.Specializations.Select(edge => new Specialization(edge.Kind, edge.Declared, edge.TargetId, edge.TargetKind)),
            symbol.WithheldLibraryAttributes);

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
                FeatureValue(entry.Value, method, $"{path}.feature_values[{ValueText.Quoted(entry.Key)}]"))));

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
