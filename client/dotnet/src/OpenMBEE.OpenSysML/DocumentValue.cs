using System;
using System.Diagnostics.CodeAnalysis;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One typed document value, as the service answers it and as a <see cref="DocumentBinding"/> binds it: a model
/// element, an object it holds, a scalar, a quantity, or a row a <c>Verdicts</c>, <c>States</c> or <c>Events</c>
/// query answered. It is an immutable variant of the service's <c>DocumentValue</c>, one nested sealed class per
/// arm; a state run's trace records its <see cref="DocumentEvent.Time"/> as one.
/// </summary>
/// <remarks>
/// <para>No type outside this library can derive from it, so the arms are exactly the nested
/// classes; a switch over them still needs a default, as a later release adds an arm whenever the
/// service does. Values compare by content, numbers as <see cref="Value"/> compares them.</para>
/// <para>An Integer, an Integer beyond <see cref="long"/>, an exact Rational and a Real are one arm,
/// <see cref="NumberValue"/>, holding the <see cref="Value.NumberValue"/> the service answered, read
/// by the same rules as a <see cref="Value"/>'s.</para>
/// </remarks>
public abstract class DocumentValue : IEquatable<DocumentValue>
{
    private DocumentValue()
    {
    }

    /// <summary>A model element bound by its qualified name, as a <see cref="DocumentBinding"/> names one.</summary>
    /// <param name="qualifiedName">The element's qualified name (<c>Observatory::telescope</c>).</param>
    /// <returns>The element value, of no stated type: a type is answered, never bound.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="qualifiedName"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="qualifiedName"/> is empty.</exception>
    public static ElementValue ElementRef(string qualifiedName)
    {
        ArgumentException.ThrowIfNullOrEmpty(qualifiedName);
        return new ElementValue(qualifiedName, string.Empty);
    }

    /// <summary>An object the service holds for the model, bound by the label a session reaches it under.</summary>
    /// <param name="path">The qualified name it was instantiated as (<c>Garage::car</c>), its id (<c>#2</c>), or a path through the feature values of either (<c>Garage::car.wheels[2]</c>).</param>
    /// <returns>The object value.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="path"/> is empty.</exception>
    public static ObjectValue ObjectRef(string path)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        return new ObjectValue(new DocumentObject(0, path, null));
    }

    /// <summary>An object the service holds for the model, bound by the id an instantiation answered for it.</summary>
    /// <param name="instanceId">The object's id; ids count up from 1.</param>
    /// <returns>The object value.</returns>
    /// <exception cref="ArgumentOutOfRangeException"><paramref name="instanceId"/> is not positive.</exception>
    public static ObjectValue ObjectRef(long instanceId)
    {
        ArgumentOutOfRangeException.ThrowIfNegativeOrZero(instanceId);
        return new ObjectValue(new DocumentObject(instanceId, string.Empty, null));
    }

    /// <inheritdoc/>
    public abstract bool Equals([NotNullWhen(true)] DocumentValue? other);

    /// <inheritdoc/>
    public sealed override bool Equals([NotNullWhen(true)] object? obj) => obj is DocumentValue other && Equals(other);

    /// <inheritdoc/>
    public abstract override int GetHashCode();

    /// <summary>The value as a reader writes it, culture-invariant.</summary>
    /// <returns>The value's text.</returns>
    public abstract override string ToString();

    /// <summary>Whether two values are equal.</summary>
    /// <param name="left">A value.</param>
    /// <param name="right">Another value.</param>
    /// <returns>Whether they are equal.</returns>
    public static bool operator ==(DocumentValue? left, DocumentValue? right) => left is null ? right is null : left.Equals(right);

    /// <summary>Whether two values differ.</summary>
    /// <param name="left">A value.</param>
    /// <param name="right">Another value.</param>
    /// <returns>Whether they differ.</returns>
    public static bool operator !=(DocumentValue? left, DocumentValue? right) => !(left == right);

    /// <summary>
    /// A model element, by qualified name and metamodel type. An anonymous element, which the service
    /// names by no id, is known by its <see cref="ElementType"/> alone.
    /// </summary>
    public sealed class ElementValue : DocumentValue
    {
        /// <summary>Creates an element value.</summary>
        /// <param name="id">The element's qualified name; null when the service named none.</param>
        /// <param name="elementType">The element's metamodel type (<c>PartUsage</c>); empty when the service reported none.</param>
        /// <exception cref="ArgumentNullException"><paramref name="elementType"/> is null.</exception>
        public ElementValue(string? id, string elementType)
        {
            ArgumentNullException.ThrowIfNull(elementType);
            Id = id;
            ElementType = elementType;
        }

        /// <summary>
        /// The element's qualified name as the service answered it; null when the answer named no element,
        /// as it does for an anonymous assertion or a state the machine declares no element for.
        /// </summary>
        public string? Id { get; }

        /// <summary>The element's metamodel type (<c>PartUsage</c>); empty when the service reported none.</summary>
        public string ElementType { get; }

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] DocumentValue? other) =>
            other is ElementValue that && Id == that.Id && ElementType == that.ElementType;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(typeof(ElementValue), Id, ElementType);

        /// <summary>The element as <c>Demo::car (PartUsage)</c>; an anonymous one by its type alone.</summary>
        /// <returns>The element's text.</returns>
        public override string ToString() => (Id, ElementType) switch
        {
            (null, "") => "(no element)",
            (null, var type) => $"({type})",
            (var id, "") => id,
            var (id, type) => $"{id} ({type})",
        };
    }

    /// <summary>An object the service holds for the model, as <see cref="DocumentObject"/> names it.</summary>
    public sealed class ObjectValue : DocumentValue
    {
        /// <summary>Creates an object value.</summary>
        /// <param name="value">The object.</param>
        /// <exception cref="ArgumentNullException"><paramref name="value"/> is null.</exception>
        public ObjectValue(DocumentObject value)
        {
            ArgumentNullException.ThrowIfNull(value);
            Value = value;
        }

        /// <summary>The object.</summary>
        public DocumentObject Value { get; }

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] DocumentValue? other) => other is ObjectValue that && Value == that.Value;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(typeof(ObjectValue), Value);

        /// <inheritdoc/>
        public override string ToString() => Value.ToString();
    }

    /// <summary>A string.</summary>
    public sealed class TextValue : DocumentValue
    {
        /// <summary>Creates a string value.</summary>
        /// <param name="text">The text.</param>
        /// <exception cref="ArgumentNullException"><paramref name="text"/> is null.</exception>
        public TextValue(string text)
        {
            ArgumentNullException.ThrowIfNull(text);
            Text = text;
        }

        /// <summary>The text.</summary>
        public string Text { get; }

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] DocumentValue? other) => other is TextValue that && Text == that.Text;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(typeof(TextValue), Text);

        /// <summary>The text itself.</summary>
        /// <returns>The text.</returns>
        public override string ToString() => Text;
    }

    /// <summary>
    /// A number: an Integer, an Integer beyond <see cref="long"/>, an exact Rational or a Real, each
    /// kept as the service answered it.
    /// </summary>
    public sealed class NumberValue : DocumentValue
    {
        /// <summary>Creates a number value.</summary>
        /// <param name="number">The number.</param>
        /// <exception cref="ArgumentNullException"><paramref name="number"/> is null.</exception>
        public NumberValue(Value.NumberValue number)
        {
            ArgumentNullException.ThrowIfNull(number);
            Number = number;
        }

        /// <summary>The number.</summary>
        public Value.NumberValue Number { get; }

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] DocumentValue? other) => other is NumberValue that && Number.Equals(that.Number);

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(typeof(NumberValue), Number);

        /// <inheritdoc/>
        public override string ToString() => Number.ToString();
    }

    /// <summary>A boolean.</summary>
    public sealed class BooleanValue : DocumentValue
    {
        /// <summary>Creates a boolean value.</summary>
        /// <param name="value">The boolean.</param>
        public BooleanValue(bool value) => Value = value;

        /// <summary>The boolean.</summary>
        public bool Value { get; }

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] DocumentValue? other) => other is BooleanValue that && Value == that.Value;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(typeof(BooleanValue), Value);

        /// <summary>The boolean, as <c>true</c> or <c>false</c>.</summary>
        /// <returns>The boolean's text.</returns>
        public override string ToString() => Value ? "true" : "false";
    }

    /// <summary>The unbounded value <c>*</c> of a multiplicity; answered, never bound.</summary>
    public sealed class InfinityValue : DocumentValue
    {
        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] DocumentValue? other) => other is InfinityValue;

        /// <inheritdoc/>
        public override int GetHashCode() => typeof(InfinityValue).GetHashCode();

        /// <summary>The value as <c>*</c>.</summary>
        /// <returns>The text <c>*</c>.</returns>
        public override string ToString() => "*";
    }

    /// <summary>A magnitude in a unit, <c>2290000 [kg]</c>.</summary>
    public sealed class QuantityValue : DocumentValue
    {
        /// <summary>Creates a quantity value.</summary>
        /// <param name="quantity">The quantity.</param>
        /// <exception cref="ArgumentNullException"><paramref name="quantity"/> is null.</exception>
        public QuantityValue(Quantity quantity)
        {
            ArgumentNullException.ThrowIfNull(quantity);
            Quantity = quantity;
        }

        /// <summary>The quantity.</summary>
        public Quantity Quantity { get; }

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] DocumentValue? other) => other is QuantityValue that && Quantity == that.Quantity;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(typeof(QuantityValue), Quantity);

        /// <inheritdoc/>
        public override string ToString() => Quantity.ToString();
    }

    /// <summary>A row a <c>Verdicts</c> query answered; answered, never bound.</summary>
    public sealed class VerdictValue : DocumentValue
    {
        /// <summary>Creates a verdict value.</summary>
        /// <param name="verdict">The verdict row.</param>
        /// <exception cref="ArgumentNullException"><paramref name="verdict"/> is null.</exception>
        public VerdictValue(DocumentVerdict verdict)
        {
            ArgumentNullException.ThrowIfNull(verdict);
            Verdict = verdict;
        }

        /// <summary>The verdict row.</summary>
        public DocumentVerdict Verdict { get; }

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] DocumentValue? other) => other is VerdictValue that && Verdict == that.Verdict;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(typeof(VerdictValue), Verdict);

        /// <inheritdoc/>
        public override string ToString() => Verdict.ToString();
    }

    /// <summary>A row a <c>States</c> query answered; answered, never bound.</summary>
    public sealed class StateValue : DocumentValue
    {
        /// <summary>Creates a state value.</summary>
        /// <param name="state">The state row.</param>
        /// <exception cref="ArgumentNullException"><paramref name="state"/> is null.</exception>
        public StateValue(DocumentState state)
        {
            ArgumentNullException.ThrowIfNull(state);
            State = state;
        }

        /// <summary>The state row.</summary>
        public DocumentState State { get; }

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] DocumentValue? other) => other is StateValue that && State == that.State;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(typeof(StateValue), State);

        /// <inheritdoc/>
        public override string ToString() => State.ToString();
    }

    /// <summary>A row an <c>Events</c> query answered, one record of a run's trace; answered, never bound.</summary>
    public sealed class EventValue : DocumentValue
    {
        /// <summary>Creates an event value.</summary>
        /// <param name="value">The event row.</param>
        /// <exception cref="ArgumentNullException"><paramref name="value"/> is null.</exception>
        public EventValue(DocumentEvent value)
        {
            ArgumentNullException.ThrowIfNull(value);
            Event = value;
        }

        /// <summary>The event row.</summary>
        public DocumentEvent Event { get; }

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] DocumentValue? other) => other is EventValue that && Event == that.Event;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(typeof(EventValue), Event);

        /// <inheritdoc/>
        public override string ToString() => Event.ToString();
    }
}
