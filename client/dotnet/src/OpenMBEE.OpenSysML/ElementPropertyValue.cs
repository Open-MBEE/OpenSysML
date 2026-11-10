using System;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One value of a metamodel property, in the shape the model's API element form spells it: an immutable variant of the
/// service's <c>ElementPropertyValue</c>, one nested sealed record per arm. An element is an <see cref="ElementValue"/>,
/// whose <see cref="ElementRef.Id"/> <see cref="Model.ElementPropertiesAsync"/> accepts back; a literal is a string, a
/// boolean, an integer <see cref="long"/> holds, a real <see cref="double"/> holds, or a <see cref="NumberValue"/>
/// beyond both, as its decimal text.
/// </summary>
/// <remarks>
/// No type outside this library can derive from it, so the arms are exactly the nested records; a switch over them
/// still needs a default, as a later release adds an arm whenever the service does. Values compare by content.
/// </remarks>
public abstract record ElementPropertyValue
{
    private protected ElementPropertyValue()
    {
    }

    /// <summary>The arm of the service's <c>ElementPropertyValue</c> the value was answered as, as the schema names it.</summary>
    internal abstract string Arm { get; }

    /// <summary>A reference to an element.</summary>
    public sealed record ElementValue : ElementPropertyValue
    {
        /// <summary>Creates an element value.</summary>
        /// <param name="element">The element referenced.</param>
        /// <exception cref="ArgumentNullException"><paramref name="element"/> is null.</exception>
        public ElementValue(ElementRef element)
        {
            ArgumentNullException.ThrowIfNull(element);
            Element = element;
        }

        /// <summary>The element referenced.</summary>
        public ElementRef Element { get; }


        internal override string Arm => "element";

        /// <summary>The element as it is best named.</summary>
        /// <returns>The value's text.</returns>
        public override string ToString() => Element.ToString();
    }

    /// <summary>A string, an enumeration literal included.</summary>
    public sealed record TextValue : ElementPropertyValue
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


        internal override string Arm => "string_value";

        /// <summary>The text itself.</summary>
        /// <returns>The value's text.</returns>
        public override string ToString() => Text;
    }

    /// <summary>A boolean.</summary>
    public sealed record BooleanValue : ElementPropertyValue
    {
        /// <summary>Creates the value.</summary>
        /// <param name="value">The truth value.</param>
        public BooleanValue(bool value)
        {
            Value = value;
        }

        /// <summary>The truth value.</summary>
        public bool Value { get; }


        internal override string Arm => "bool_value";

        /// <summary>The truth value, as <c>true</c> or <c>false</c>.</summary>
        /// <returns>The value's text.</returns>
        public override string ToString() => Value ? "true" : "false";
    }

    /// <summary>An integer a <see cref="long"/> holds.</summary>
    public sealed record IntegerValue : ElementPropertyValue
    {
        /// <summary>Creates the value.</summary>
        /// <param name="value">The integer.</param>
        public IntegerValue(long value)
        {
            Value = value;
        }

        /// <summary>The integer.</summary>
        public long Value { get; }


        internal override string Arm => "int_value";

        /// <summary>The integer, culture-invariant.</summary>
        /// <returns>The value's text.</returns>
        public override string ToString() => ValueText.Whole(Value);
    }

    /// <summary>A real a <see cref="double"/> holds.</summary>
    public sealed record RealValue : ElementPropertyValue
    {
        /// <summary>Creates the value.</summary>
        /// <param name="value">The real.</param>
        public RealValue(double value)
        {
            Value = value;
        }

        /// <summary>The real.</summary>
        public double Value { get; }


        internal override string Arm => "real_value";

        /// <summary>The real, culture-invariant.</summary>
        /// <returns>The value's text.</returns>
        public override string ToString() => ValueText.Real(Value);
    }

    /// <summary>A number beyond <see cref="long"/> and <see cref="double"/>, kept as the decimal text the service wrote.</summary>
    public sealed record NumberValue : ElementPropertyValue
    {
        /// <summary>Creates a number value.</summary>
        /// <param name="digits">The number in decimal.</param>
        /// <exception cref="ArgumentNullException"><paramref name="digits"/> is null.</exception>
        public NumberValue(string digits)
        {
            ArgumentNullException.ThrowIfNull(digits);
            Digits = digits;
        }

        /// <summary>The number in decimal.</summary>
        public string Digits { get; }


        internal override string Arm => "number";

        /// <summary>The number in decimal.</summary>
        /// <returns>The value's text.</returns>
        public override string ToString() => Digits;
    }
}
