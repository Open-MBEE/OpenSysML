using System;
using System.Collections.Generic;
using System.Linq;
using Google.Protobuf;
using DocumentKind = OpenMBEE.OpenSysML.Wire.DocumentValue.KindOneofCase;

namespace OpenMBEE.OpenSysML.Internal.Mapping;

/// <summary>
/// Reads the service's document messages, <c>DocumentValue</c>, <c>DocumentObject</c>,
/// <c>DocumentVerdict</c>, <c>DocumentState</c> and <c>DocumentEvent</c>, into the client's immutable
/// records, and writes the values a document query is bound to. A number or a quantity is read and written
/// by <see cref="ValueWire"/>'s rules; a malformed answer is a <see cref="TransportException"/> naming where
/// it is, and an arm this client does not know is an <see cref="UnsupportedValueException"/>, never an
/// anonymous element.
/// </summary>
internal static class DocumentWire
{
    /// <summary>Reads a document value.</summary>
    /// <param name="value">The message.</param>
    /// <param name="method">The method answered, for an error.</param>
    /// <param name="path">Where the value is in the answer, as <c>trace[0].time</c>.</param>
    internal static DocumentValue Value(Wire.DocumentValue value, string method, string path) => Read(value, method, path, 0);

    /// <summary>Reads a trace record.</summary>
    /// <param name="value">The message.</param>
    /// <param name="method">The method answered, for an error.</param>
    /// <param name="path">Where the record is in the answer, as <c>trace[0]</c>.</param>
    internal static DocumentEvent Event(Wire.DocumentEvent value, string method, string path) => ReadEvent(value, method, path, 0);

    /// <summary>Reads a document query's answer: its columns, and each row's element and cells.</summary>
    /// <exception cref="TransportException">A row selects no element, or a value is malformed.</exception>
    /// <exception cref="UnsupportedValueException">A value is of a kind this client does not know.</exception>
    internal static DocumentQueryResult Result(Wire.RunDocumentQueryResponse response, string method) =>
        new(
            response.Columns.Select(column => column.Name),
            response.Rows.Select((row, i) => new DocumentRow(
                row.Element is { } element
                    ? Value(element, method, $"rows[{i}].element")
                    : throw ValueWire.MalformedAnswer(method, $"rows[{i}]", "it selects no element, which every row stands for"),
                row.Cells.Select((cell, j) => cell.Values.Select((value, k) => Value(value, method, $"rows[{i}].cells[{j}].values[{k}]"))))));

    /// <summary>Writes a document query's bindings, in the order given, each value as it is held.</summary>
    /// <exception cref="ArgumentException">A quantity cannot be sent.</exception>
    internal static IEnumerable<Wire.DocumentQueryBinding> ToWire(IEnumerable<DocumentBinding> bindings) =>
        bindings.Select((binding, i) => new Wire.DocumentQueryBinding
        {
            Parameter = binding.Parameter,
            Values = { binding.Values.Select((value, j) => ToWire(value, $"bindings[{i}].values[{j}]")) },
        });

    /// <summary>
    /// Holds written bindings to what a service advertises, by the service's own predicates and in its order: first, to
    /// a service without <c>rational_values</c>, each Rational a <see cref="double"/> holds exactly becomes that double,
    /// the same number; then an Integer beyond <see cref="long"/> needs <c>big_int_values</c>, and a Rational
    /// <c>rational_values</c>, as a value's own arm or a quantity's magnitude.
    /// </summary>
    /// <exception cref="CapabilityException">The service does not advertise a capability a binding needs.</exception>
    internal static void Fit(IReadOnlyList<Wire.DocumentQueryBinding> bindings, Capabilities capabilities)
    {
        var values = bindings.SelectMany(binding => binding.Values).ToList();
        if (!capabilities.Has(Capabilities.RationalValues))
        {
            values.ForEach(RationalAsReal);
        }

        if (values.Any(value => Holds(value, DocumentKind.BigIntValue, Wire.Quantity.MagnitudeOneofCase.BigIntMagnitude)))
        {
            capabilities.Require(Capabilities.BigIntValues);
        }

        if (values.Any(value => Holds(value, DocumentKind.RationalValue, Wire.Quantity.MagnitudeOneofCase.RationalMagnitude)))
        {
            capabilities.Require(Capabilities.RationalValues);
        }
    }

    private static Wire.DocumentValue ToWire(DocumentValue value, string path) => value switch
    {
        DocumentValue.ElementValue element => Element(element),
        DocumentValue.ObjectValue held => new Wire.DocumentValue
        {
            Object = new Wire.DocumentObject
            {
                InstanceId = held.Value.InstanceId,
                Path = held.Value.Path,
                Element = held.Value.Element is { } element ? Element(element) : null,
            },
        },
        DocumentValue.TextValue text => new Wire.DocumentValue { StringValue = text.Text },
        DocumentValue.NumberValue number => ValueWire.ToWire(number.Number, path) switch
        {
            { KindCase: Wire.Value.KindOneofCase.IntValue } written => new Wire.DocumentValue { IntValue = written.IntValue },
            { KindCase: Wire.Value.KindOneofCase.BigIntValue } written => new Wire.DocumentValue { BigIntValue = written.BigIntValue },
            { KindCase: Wire.Value.KindOneofCase.RationalValue } written => new Wire.DocumentValue { RationalValue = written.RationalValue },
            var written => new Wire.DocumentValue { RealValue = written.RealValue },
        },
        DocumentValue.BooleanValue boolean => new Wire.DocumentValue { BoolValue = boolean.Value },
        DocumentValue.QuantityValue quantity => new Wire.DocumentValue { Quantity = ValueWire.ToWire(quantity.Quantity, path + ".quantity") },
        _ => throw new InvalidOperationException($"No binding is written for a {value.GetType().FullName}, which a DocumentBinding refuses."),
    };

    // An element named by no id is sent, as the service answers one, as the value of no arm.
    private static Wire.DocumentValue Element(DocumentValue.ElementValue element) =>
        element.Id is { } id ? new Wire.DocumentValue { ElementId = id, ElementType = element.ElementType } : new Wire.DocumentValue { ElementType = element.ElementType };

    private static bool Holds(Wire.DocumentValue value, DocumentKind arm, Wire.Quantity.MagnitudeOneofCase magnitude) =>
        value.KindCase == DocumentKind.Quantity ? value.Quantity.MagnitudeCase == magnitude : value.KindCase == arm;

    private static void RationalAsReal(Wire.DocumentValue value)
    {
        if (value.KindCase == DocumentKind.RationalValue && ValueWire.ExactDouble(value.RationalValue) is { } real)
        {
            value.RealValue = real;
        }
        else if (value.KindCase == DocumentKind.Quantity)
        {
            ValueWire.RationalAsReal(value.Quantity);
        }
    }

    private static DocumentValue Read(Wire.DocumentValue value, string method, string path, int depth)
    {
        if (depth > ValueWire.MaxDepth)
        {
            throw ValueWire.MalformedAnswer(method, path, $"it nests deeper than {ValueWire.MaxDepth} values");
        }

        if (value.KindCase is not (DocumentKind.ElementId or DocumentKind.None) && value.ElementType.Length > 0)
        {
            throw ValueWire.MalformedAnswer(method, path, $"element_type {ValueText.Quoted(value.ElementType)} is set on a {value.KindCase} value, which names no element");
        }

        return value.KindCase switch
        {
            DocumentKind.ElementId => new DocumentValue.ElementValue(value.ElementId, value.ElementType),
            DocumentKind.None => Anonymous(value, method, path),
            DocumentKind.StringValue => new DocumentValue.TextValue(value.StringValue),
            DocumentKind.IntValue => new DocumentValue.NumberValue(new Value.IntegerValue(value.IntValue)),
            DocumentKind.BigIntValue => new DocumentValue.NumberValue(new Value.BigIntegerValue(ValueWire.BigIntegerFromWire(value.BigIntValue, method, path))),
            DocumentKind.RationalValue => new DocumentValue.NumberValue(new Value.RationalValue(ValueWire.RationalFromWire(value.RationalValue, method, path))),
            DocumentKind.RealValue => new DocumentValue.NumberValue(new Value.RealValue(value.RealValue)),
            DocumentKind.BoolValue => new DocumentValue.BooleanValue(value.BoolValue),
            DocumentKind.Infinity => value.Infinity
                ? new DocumentValue.InfinityValue()
                : throw ValueWire.MalformedAnswer(method, path, "the infinity arm states no value unless it is true"),
            DocumentKind.Quantity => new DocumentValue.QuantityValue(ValueWire.FromWire(value.Quantity, method, path + ".quantity")),
            DocumentKind.Object => new DocumentValue.ObjectValue(ReadObject(value.Object, method, path + ".object", depth)),
            DocumentKind.Verdict => new DocumentValue.VerdictValue(ReadVerdict(value.Verdict, method, path + ".verdict", depth)),
            DocumentKind.State => new DocumentValue.StateValue(ReadState(value.State, method, path + ".state", depth)),
            DocumentKind.Event => new DocumentValue.EventValue(ReadEvent(value.Event, method, path + ".event", depth)),
            _ => throw Unsupported(value, method, path),
        };
    }

    // A value of no known arm: an anonymous element named by its type alone, unless it carries
    // fields this client does not know, which are a newer arm rather than no element.
    private static DocumentValue.ElementValue Anonymous(Wire.DocumentValue value, string method, string path) =>
        new Wire.DocumentValue { ElementType = value.ElementType }.CalculateSize() == value.CalculateSize()
            ? new DocumentValue.ElementValue(null, value.ElementType)
            : throw Unsupported(value, method, path);

    // A field the schema says holds an element value: one naming an element, or none.
    private static DocumentValue.ElementValue? Element(Wire.DocumentValue? value, string method, string path, int depth) => value is null
        ? null
        : Read(value, method, path, depth + 1) as DocumentValue.ElementValue
            ?? throw ValueWire.MalformedAnswer(method, path, $"it is a {value.KindCase} value where the schema holds an element");

    private static DocumentObject ReadObject(Wire.DocumentObject value, string method, string path, int depth) =>
        new(value.InstanceId, value.Path, Element(value.Element, method, path + ".element", depth));

    private static DocumentVerdict ReadVerdict(Wire.DocumentVerdict value, string method, string path, int depth) =>
        new(
            Element(value.Assertion, method, path + ".assertion", depth),
            value.Kind,
            value.Text,
            value.Path,
            value.Verdict,
            value.Condition,
            value.Reason,
            value.Verification);

    private static DocumentState ReadState(Wire.DocumentState value, string method, string path, int depth) =>
        new(
            value.Object is { } held ? ReadObject(held, method, path + ".object", depth + 1) : null,
            value.Machine,
            value.Name,
            value.StatePath,
            Element(value.State, method, path + ".state", depth),
            value.Region,
            value.Enclosing);

    private static DocumentEvent ReadEvent(Wire.DocumentEvent value, string method, string path, int depth) =>
        new(
            value.Kind,
            value.Time is { } time ? Read(time, method, path + ".time", depth + 1) : null,
            value.Object is { } held ? ReadObject(held, method, path + ".object", depth + 1) : null,
            value.Machine,
            value.State,
            value.From,
            value.To,
            value.Target is { } target ? ReadObject(target, method, path + ".target", depth + 1) : null,
            value.Event,
            value.Payload.ToList(),
            value.Alternatives.ToList(),
            value.Taken,
            value.Text,
            value.Token,
            value.Step,
            value.Feature,
            value.Value is { } assigned ? ValueWire.FromWire(assigned, method, path + ".value") : null);

    private static UnsupportedValueException Unsupported(Wire.DocumentValue value, string method, string path) =>
        new($"The service answered a document value at {path} of a kind this client does not know; a newer client may read it.", method, path, value.ToByteArray());
}
