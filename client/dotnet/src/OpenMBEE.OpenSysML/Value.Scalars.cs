using System;
using System.Diagnostics.CodeAnalysis;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public abstract partial class Value
{
    /// <summary>A Boolean.</summary>
    public sealed class BooleanValue : Value
    {
        /// <summary>Creates a Boolean.</summary>
        /// <param name="value">The Boolean.</param>
        public BooleanValue(bool value) => Value = value;

        /// <summary>The Boolean.</summary>
        public bool Value { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Boolean;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is BooleanValue that && Value == that.Value;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Value);

        /// <summary><c>true</c> or <c>false</c>.</summary>
        /// <returns>The Boolean's text.</returns>
        public override string ToString() => Value ? "true" : "false";
    }

    /// <summary>A String.</summary>
    public sealed class TextValue : Value
    {
        /// <summary>Creates a String.</summary>
        /// <param name="value">The text.</param>
        /// <exception cref="ArgumentNullException"><paramref name="value"/> is null.</exception>
        public TextValue(string value)
        {
            ArgumentNullException.ThrowIfNull(value);
            Value = value;
        }

        /// <summary>The text.</summary>
        public string Value { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Text;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is TextValue that && string.Equals(Value, that.Value, StringComparison.Ordinal);

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, StringComparer.Ordinal.GetHashCode(Value));

        /// <summary>The text as a SysML string literal, <c>"a \"b\""</c>.</summary>
        /// <returns>The quoted text.</returns>
        public override string ToString() => ValueText.Quoted(Value);
    }

    /// <summary>
    /// A reference to an instance by the id the service assigned it, which lives only within the
    /// response that holds it.
    /// </summary>
    public sealed class InstanceRefValue : Value
    {
        /// <summary>Creates an instance reference.</summary>
        /// <param name="instanceId">The instance's id.</param>
        public InstanceRefValue(long instanceId) => InstanceId = instanceId;

        /// <summary>The instance's id within its response.</summary>
        public long InstanceId { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.InstanceRef;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is InstanceRefValue that && InstanceId == that.InstanceId;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, InstanceId);

        /// <summary>The reference, as <c>instance(7)</c>.</summary>
        /// <returns>The reference's text.</returns>
        public override string ToString() => $"instance({ValueText.Whole(InstanceId)})";
    }

    /// <summary>
    /// One literal of an enumeration definition. A literal is its own identity: two are the same
    /// literal exactly when <see cref="LiteralId"/> is, whatever else describes them.
    /// </summary>
    /// <remarks>Only a service advertising <c>enum_values</c> reports a literal as itself rather than as an unsupported null.</remarks>
    public sealed class EnumLiteralValue : Value
    {
        /// <summary>Creates an enumeration literal.</summary>
        /// <param name="literalId">The literal declaration's qualified name, as <c>D::Color::red</c>: its identity.</param>
        /// <param name="enumerationId">The declaring enumeration's qualified name, as <c>D::Color</c>.</param>
        /// <param name="name">The literal as a reader writes it, as <c>Color::red</c>.</param>
        /// <param name="scalar">The scalar the literal equals (<c>3</c> for <c>high = 3</c>), or null for one that is only its identity.</param>
        /// <exception cref="ArgumentNullException">An identifier or the name is null.</exception>
        /// <exception cref="ArgumentException"><paramref name="literalId"/> is empty.</exception>
        public EnumLiteralValue(string literalId, string enumerationId, string name, Value? scalar = null)
        {
            ArgumentException.ThrowIfNullOrEmpty(literalId);
            ArgumentNullException.ThrowIfNull(enumerationId);
            ArgumentNullException.ThrowIfNull(name);
            LiteralId = literalId;
            EnumerationId = enumerationId;
            Name = name;
            Scalar = scalar;
        }

        /// <summary>The literal declaration's qualified name, as <c>D::Color::red</c>: its identity.</summary>
        public string LiteralId { get; }

        /// <summary>The declaring enumeration's qualified name, as <c>D::Color</c>.</summary>
        public string EnumerationId { get; }

        /// <summary>The literal as a reader writes it, as <c>Color::red</c>.</summary>
        public string Name { get; }

        /// <summary>The scalar the literal equals, or null for a literal that is only its identity.</summary>
        public Value? Scalar { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.EnumLiteral;

        /// <summary>Whether another value is the same literal: the same <see cref="LiteralId"/>.</summary>
        /// <param name="other">The other value.</param>
        /// <returns>Whether they are the same literal.</returns>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is EnumLiteralValue that && string.Equals(LiteralId, that.LiteralId, StringComparison.Ordinal);

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, StringComparer.Ordinal.GetHashCode(LiteralId));

        /// <summary>The literal as a reader writes it, or its declaration's name when it has none.</summary>
        /// <returns>The literal's text.</returns>
        public override string ToString() => Name.Length > 0 ? Name : LiteralId;
    }

    /// <summary>
    /// A measurement unit held as a value by itself, with no magnitude: <c>SI::m</c>, <c>km</c>, or
    /// <c>m / s</c> as an operation composed it.
    /// </summary>
    /// <remarks>Only a service advertising <c>measurement_refs</c> reports one as itself rather than as an unsupported null.</remarks>
    public sealed class MeasurementRefValue : Value
    {
        /// <summary>Creates a measurement reference.</summary>
        /// <param name="unit">The unit as written (<c>km</c>); empty for one never written down.</param>
        /// <param name="reduction">What the unit reduces to.</param>
        /// <param name="unitId">The qualified name of the one unit declaration it names (<c>SI::kilometre</c>), or null for a composed unit.</param>
        /// <exception cref="ArgumentNullException"><paramref name="unit"/> or <paramref name="reduction"/> is null.</exception>
        /// <exception cref="ArgumentException"><paramref name="unitId"/> is empty; null stands for no declaration.</exception>
        public MeasurementRefValue(string unit, UnitTerm reduction, string? unitId = null)
        {
            ArgumentNullException.ThrowIfNull(unit);
            ArgumentNullException.ThrowIfNull(reduction);
            if (unitId is { Length: 0 })
            {
                throw new ArgumentException("The unit declaration's name is empty; null stands for a unit naming no declaration.", nameof(unitId));
            }

            Unit = unit;
            Reduction = reduction;
            UnitId = unitId;
        }

        /// <summary>The unit as written (<c>km</c>); empty for one never written down.</summary>
        public string Unit { get; }

        /// <summary>What the unit reduces to.</summary>
        public UnitTerm Reduction { get; }

        /// <summary>The qualified name of the one unit declaration it names (<c>SI::kilometre</c>), or null for a composed unit.</summary>
        public string? UnitId { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.MeasurementRef;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is MeasurementRefValue that
                && string.Equals(Unit, that.Unit, StringComparison.Ordinal)
                && Reduction.Equals(that.Reduction)
                && string.Equals(UnitId, that.UnitId, StringComparison.Ordinal);

        /// <inheritdoc/>
        public override int GetHashCode() =>
            HashCode.Combine(Kind, StringComparer.Ordinal.GetHashCode(Unit), Reduction, UnitId is null ? 0 : StringComparer.Ordinal.GetHashCode(UnitId));

        /// <summary>The unit in brackets, as <c>[km]</c>; its declaration's name, or else its reduction, when it was never written down.</summary>
        /// <returns>The reference's text.</returns>
        public override string ToString() => $"[{(Unit.Length > 0 ? Unit : UnitId ?? Reduction.ToString())}]";
    }

    /// <summary>
    /// A calc held as a value: a calc definition, or a calc usage with an input no read could
    /// supply, as <c>Sq</c> in <c>Fn(Sq, 3.0)</c>. It is the declaration it is a value of: two
    /// functions are equal exactly when <see cref="CalcId"/> and <see cref="SelfId"/> are.
    /// </summary>
    /// <remarks>
    /// Only a service advertising <c>function_values</c> reports one as itself rather than as an
    /// unsupported null; a function closing over the bindings of a behavior body has no wire form
    /// and arrives as an unsupported null too. One closing over an instance cannot be sent back,
    /// since that instance lives only within the response that held it.
    /// </remarks>
    public sealed class FunctionValue : Value
    {
        /// <summary>Creates a function.</summary>
        /// <param name="calcId">The calc declaration's qualified name, as <c>Analysis::Sq</c>.</param>
        /// <param name="selfId">The id of the instance its feature names resolve against, or null for one closing over none.</param>
        /// <exception cref="ArgumentNullException"><paramref name="calcId"/> is null.</exception>
        /// <exception cref="ArgumentException"><paramref name="calcId"/> is empty.</exception>
        /// <exception cref="ArgumentOutOfRangeException"><paramref name="selfId"/> is zero, which the service reads as no instance.</exception>
        public FunctionValue(string calcId, long? selfId = null)
        {
            ArgumentException.ThrowIfNullOrEmpty(calcId);
            if (selfId == 0)
            {
                throw new ArgumentOutOfRangeException(nameof(selfId), selfId, "Instance id zero stands for no instance; pass null.");
            }

            CalcId = calcId;
            SelfId = selfId;
        }

        /// <summary>The calc declaration's qualified name, as <c>Analysis::Sq</c>.</summary>
        public string CalcId { get; }

        /// <summary>The id of the instance its feature names resolve against (<c>holder.scale</c>), or null for one closing over none.</summary>
        public long? SelfId { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Function;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is FunctionValue that && string.Equals(CalcId, that.CalcId, StringComparison.Ordinal) && SelfId == that.SelfId;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, StringComparer.Ordinal.GetHashCode(CalcId), SelfId);

        /// <summary>The calc's name, as <c>Analysis::Sq</c>, followed by <c>of instance(7)</c> for one closing over an instance.</summary>
        /// <returns>The function's text.</returns>
        public override string ToString() =>
            SelfId is { } self ? $"{CalcId} of instance({ValueText.Whole(self)})" : CalcId;
    }

    /// <summary>
    /// An element of the model held as an instance of its reflective metaclass: what
    /// <c>x meta KerML::Feature</c> evaluates to. It is the element it reflects on: two metaobjects
    /// are equal exactly when <see cref="ElementId"/> is, whatever metaclass describes them.
    /// </summary>
    /// <remarks>Only a service advertising <c>metaobject_values</c> reports one as itself rather than as an unsupported null.</remarks>
    public sealed class MetaobjectValue : Value
    {
        /// <summary>Creates a metaobject.</summary>
        /// <param name="elementId">The qualified name of the element reflected on, as <c>Vehicle::seatBelt</c>: its identity.</param>
        /// <param name="metaclassId">The element's own metaclass, as <c>SysML::Systems::PartUsage</c>; empty to let the service use the model's.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        /// <exception cref="ArgumentException"><paramref name="elementId"/> is empty.</exception>
        public MetaobjectValue(string elementId, string metaclassId = "")
        {
            ArgumentException.ThrowIfNullOrEmpty(elementId);
            ArgumentNullException.ThrowIfNull(metaclassId);
            ElementId = elementId;
            MetaclassId = metaclassId;
        }

        /// <summary>The qualified name of the element reflected on: its identity.</summary>
        public string ElementId { get; }

        /// <summary>The element's own metaclass, never the type it was cast to; the service always sends it.</summary>
        public string MetaclassId { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Metaobject;

        /// <summary>Whether another value reflects on the same element: the same <see cref="ElementId"/>.</summary>
        /// <param name="other">The other value.</param>
        /// <returns>Whether they are the same metaobject.</returns>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is MetaobjectValue that && string.Equals(ElementId, that.ElementId, StringComparison.Ordinal);

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, StringComparer.Ordinal.GetHashCode(ElementId));

        /// <summary>The metaobject, as <c>meta(Vehicle::seatBelt : SysML::Systems::PartUsage)</c>.</summary>
        /// <returns>The metaobject's text.</returns>
        public override string ToString() =>
            MetaclassId.Length > 0 ? $"meta({ElementId} : {MetaclassId})" : $"meta({ElementId})";
    }

    /// <summary>
    /// The null value: a feature that resolved to nothing. With <see cref="Unsupported"/> text, it
    /// instead stands for a value the service held but could not send, which cannot be sent back.
    /// </summary>
    public sealed class NullValue : Value
    {
        /// <summary>Creates a null.</summary>
        /// <param name="unsupported">Empty for the model's null; otherwise what the service held and could not send.</param>
        /// <exception cref="ArgumentNullException"><paramref name="unsupported"/> is null.</exception>
        public NullValue(string unsupported = "")
        {
            ArgumentNullException.ThrowIfNull(unsupported);
            Unsupported = unsupported;
        }

        /// <summary>Empty for the model's null; otherwise what the service held and could not send, as <c>unsupported: variant selection</c>.</summary>
        public string Unsupported { get; }

        /// <summary>Whether this stands for a value the service could not send rather than the model's null.</summary>
        public bool IsUnsupported => Unsupported.Length > 0;

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Null;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is NullValue that && string.Equals(Unsupported, that.Unsupported, StringComparison.Ordinal);

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, StringComparer.Ordinal.GetHashCode(Unsupported));

        /// <summary><c>null</c>, followed by what could not be sent in parentheses when it is unsupported.</summary>
        /// <returns>The null's text.</returns>
        public override string ToString() => IsUnsupported ? $"null ({Unsupported})" : "null";
    }

    /// <summary>
    /// A materialized feature of a value type that holds no value. An answer only: the service
    /// refuses it as an argument, so this client does too.
    /// </summary>
    /// <remarks>Only a service advertising <c>unset_value</c> reports one as itself rather than as an unsupported null.</remarks>
    public sealed class UnsetValue : Value
    {
        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Unset;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is UnsetValue;

        /// <inheritdoc/>
        public override int GetHashCode() => Kind.GetHashCode();

        /// <summary><c>&lt;unset&gt;</c>.</summary>
        /// <returns>The value's text.</returns>
        public override string ToString() => "<unset>";
    }

    /// <summary>
    /// A result the model leaves open: it reads a feature no value is given, or counts one whose
    /// multiplicity fixes no count. Evaluation succeeded; the answer is open. An answer only: the
    /// service refuses it as an argument, so this client does too.
    /// </summary>
    /// <remarks>Only a service advertising <c>undetermined_value</c> reports one as itself rather than as an unsupported null.</remarks>
    public sealed class UndeterminedValue : Value
    {
        /// <summary>Creates an undetermined result.</summary>
        /// <param name="reason">Why the model leaves it open, as <c>u has no value in the model</c>.</param>
        /// <param name="countLower">The least number of values the result would hold, as a multiplicity bound is spelled; empty when unknown.</param>
        /// <param name="countUpper">The most, <c>*</c> when unbounded; empty when unknown.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public UndeterminedValue(string reason, string countLower, string countUpper)
        {
            ArgumentNullException.ThrowIfNull(reason);
            ArgumentNullException.ThrowIfNull(countLower);
            ArgumentNullException.ThrowIfNull(countUpper);
            Reason = reason;
            CountLower = countLower;
            CountUpper = countUpper;
        }

        /// <summary>Why the model leaves the result open.</summary>
        public string Reason { get; }

        /// <summary>The least number of values the result would hold, as a multiplicity bound is spelled; empty when unknown.</summary>
        public string CountLower { get; }

        /// <summary>The most number of values the result would hold, <c>*</c> when unbounded; empty when unknown.</summary>
        public string CountUpper { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Undetermined;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is UndeterminedValue that
                && string.Equals(Reason, that.Reason, StringComparison.Ordinal)
                && string.Equals(CountLower, that.CountLower, StringComparison.Ordinal)
                && string.Equals(CountUpper, that.CountUpper, StringComparison.Ordinal);

        /// <inheritdoc/>
        public override int GetHashCode() =>
            HashCode.Combine(Kind, StringComparer.Ordinal.GetHashCode(Reason), StringComparer.Ordinal.GetHashCode(CountLower), StringComparer.Ordinal.GetHashCode(CountUpper));

        /// <summary><c>&lt;undetermined&gt;</c>, as every other surface spells it.</summary>
        /// <returns>The value's text.</returns>
        public override string ToString() => "<undetermined>";
    }

    /// <summary>
    /// The unbounded value <c>*</c>: no number, ordered above every finite magnitude and refused by
    /// arithmetic. It is not a Real infinity, which <see cref="RealValue"/> holds.
    /// </summary>
    /// <remarks>Only a service advertising <c>infinity_value</c> reports it as itself rather than as an unsupported null.</remarks>
    public sealed class InfinityValue : Value
    {
        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Infinity;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is InfinityValue;

        /// <inheritdoc/>
        public override int GetHashCode() => Kind.GetHashCode();

        /// <summary><c>*</c>.</summary>
        /// <returns>The value's text.</returns>
        public override string ToString() => "*";
    }
}
