using System;
using System.Collections.Generic;
using System.Diagnostics.CodeAnalysis;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public abstract partial class Value
{
    /// <summary>An ordered sequence of values; its order is part of its value.</summary>
    public sealed class SequenceValue : Value
    {
        /// <summary>Creates a sequence, copying the elements.</summary>
        /// <param name="elements">The elements, in order.</param>
        /// <exception cref="ArgumentNullException"><paramref name="elements"/> is null.</exception>
        /// <exception cref="ArgumentException">An element is null.</exception>
        public SequenceValue(IEnumerable<Value> elements) => Elements = new ValueList<Value>(elements);

        /// <summary>The elements, in order.</summary>
        public ValueList<Value> Elements { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Sequence;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is SequenceValue that && Elements == that.Elements;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Elements);

        /// <summary>The elements in parentheses, as <c>(1, 2.0, "a")</c>.</summary>
        /// <returns>The sequence's text.</returns>
        public override string ToString() => $"({ValueText.Join(Elements)})";
    }

    /// <summary>
    /// A unique, unordered collection: a <c>Collections::Set</c>'s elements. Two sets are equal, and
    /// hash alike, when they hold equal members in any order.
    /// </summary>
    /// <remarks>
    /// <para>Membership is <see cref="Value"/> equality, so an Integer <c>1</c> and a Real <c>1.0</c>
    /// are two members here. The service judges membership by the model's numeric comparison, so it
    /// never sends both and refuses a set holding both.</para>
    /// <para><see cref="Elements"/> keeps the order the members were given in, which for an answer
    /// is the service's canonical order; it is for reading and rendering, not for equality. Only a
    /// service advertising <c>set_values</c> reports a set as itself rather than as an unsupported null.</para>
    /// </remarks>
    public sealed class SetValue : Value
    {
        private readonly HashSet<Value> members;

        /// <summary>Creates a set, copying the members.</summary>
        /// <param name="elements">The members, each once.</param>
        /// <exception cref="ArgumentNullException"><paramref name="elements"/> is null.</exception>
        /// <exception cref="ArgumentException">A member is null or listed twice.</exception>
        public SetValue(IEnumerable<Value> elements)
        {
            Elements = new ValueList<Value>(elements);
            members = new HashSet<Value>(Elements.Count);
            foreach (var element in Elements)
            {
                if (!members.Add(element))
                {
                    throw new ArgumentException($"The set lists the member {element} twice.", nameof(elements));
                }
            }
        }

        /// <summary>The members, in the order they were given.</summary>
        public ValueList<Value> Elements { get; }

        /// <summary>How many members the set holds.</summary>
        public int Count => Elements.Count;

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Set;

        /// <summary>Whether a value is a member.</summary>
        /// <param name="value">The value.</param>
        /// <returns>Whether the set holds it.</returns>
        public bool Contains(Value value)
        {
            ArgumentNullException.ThrowIfNull(value);
            return members.Contains(value);
        }

        /// <summary>Whether another value is a set of the same members, in any order.</summary>
        /// <param name="other">The other value.</param>
        /// <returns>Whether they are the same set.</returns>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is SetValue that && Count == that.Count && members.SetEquals(that.members);

        /// <inheritdoc/>
        public override int GetHashCode()
        {
            var unordered = 0;
            foreach (var member in Elements)
            {
                unordered = unchecked(unordered + member.GetHashCode());
            }

            return HashCode.Combine(Kind, Count, unordered);
        }

        /// <summary>The members in braces, in the order they were given, as <c>{1, 2, 3}</c>.</summary>
        /// <returns>The set's text.</returns>
        public override string ToString() => $"{{{ValueText.Join(Elements)}}}";
    }

    /// <summary>A magnitude and the unit it is expressed in.</summary>
    public sealed class QuantityValue : Value
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
        public override ValueKind Kind => ValueKind.Quantity;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is QuantityValue that && Quantity.Equals(that.Quantity);

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Quantity);

        /// <summary>The quantity, as <c>5.4 [km/h]</c>.</summary>
        /// <returns>The quantity's text.</returns>
        public override string ToString() => Quantity.ToString();
    }

    /// <summary>
    /// A <c>Collections::Array</c>: its dimensions, and its elements flattened in row-major order
    /// under them, the last dimension varying fastest. A rank-0 array holds exactly one element; an
    /// element is any value, a nested array or a quantity included.
    /// </summary>
    /// <remarks>Only a service advertising <c>structured_values</c> reports one as itself rather than as an unsupported null.</remarks>
    public sealed class ArrayValue : Value
    {
        /// <summary>Creates an array, copying the dimensions and the elements.</summary>
        /// <param name="dimensions">The extent of each dimension, all positive.</param>
        /// <param name="elements">The elements in row-major order, exactly as many as the dimensions multiply to.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        /// <exception cref="ArgumentException">An element is null, a dimension is not positive, or the elements do not fill the dimensions.</exception>
        public ArrayValue(IEnumerable<long> dimensions, IEnumerable<Value> elements)
        {
            Dimensions = new ValueList<long>(dimensions);
            Elements = new ValueList<Value>(elements);
            Shape.RequireFilled(Dimensions, Elements.Count, "An array", nameof(elements));
        }

        /// <summary>The extent of each dimension.</summary>
        public ValueList<long> Dimensions { get; }

        /// <summary>The elements, in row-major order.</summary>
        public ValueList<Value> Elements { get; }

        /// <summary>The number of dimensions.</summary>
        public int Rank => Dimensions.Count;

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Array;

        /// <summary>The element at a multi-index.</summary>
        /// <param name="index">One coordinate per dimension, each within its extent.</param>
        /// <returns>The element there.</returns>
        /// <exception cref="ArgumentException">The index has another rank than the array.</exception>
        /// <exception cref="ArgumentOutOfRangeException">A coordinate is outside its dimension.</exception>
        public Value Get(params ReadOnlySpan<long> index) => Elements[Shape.Offset(Dimensions, index, "array")];

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is ArrayValue that && Dimensions == that.Dimensions && Elements == that.Elements;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Dimensions, Elements);

        /// <summary>The dimensions and the row-major elements, as <c>Array(2, 2)[1, 2, 3, 4]</c>.</summary>
        /// <returns>The array's text.</returns>
        public override string ToString() => $"Array({ValueText.Join(Dimensions)})[{ValueText.Join(Elements)}]";
    }

    /// <summary>
    /// A <c>VectorValues::NumericalVectorValue</c>: its numeric components in order, one value, never
    /// a sequence of numbers.
    /// </summary>
    /// <remarks>Only a service advertising <c>structured_values</c> reports one as itself rather than as an unsupported null.</remarks>
    public sealed class VectorValue : Value
    {
        /// <summary>Creates a vector, copying the components.</summary>
        /// <param name="components">The components, in order.</param>
        /// <exception cref="ArgumentNullException"><paramref name="components"/> is null.</exception>
        /// <exception cref="ArgumentException">A component is null.</exception>
        public VectorValue(IEnumerable<NumberValue> components) => Components = new ValueList<NumberValue>(components);

        /// <summary>The components, in order.</summary>
        public ValueList<NumberValue> Components { get; }

        /// <summary>The number of components.</summary>
        public int Dimension => Components.Count;

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Vector;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is VectorValue that && Components == that.Components;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Components);

        /// <summary>The components, as <c>Vector(3, 4.0)</c>.</summary>
        /// <returns>The vector's text.</returns>
        public override string ToString() => $"Vector({ValueText.Join(Components)})";
    }

    /// <summary>
    /// A <c>Quantities::VectorQuantityValue</c>: one quantity per axis, each with its own unit,
    /// since the axes need not share one.
    /// </summary>
    /// <remarks>Only a service advertising <c>structured_values</c> reports one as itself rather than as an unsupported null.</remarks>
    public sealed class VectorQuantityValue : Value
    {
        /// <summary>Creates a vector quantity, copying the components.</summary>
        /// <param name="components">The components, at least one.</param>
        /// <exception cref="ArgumentNullException"><paramref name="components"/> is null.</exception>
        /// <exception cref="ArgumentException">There is no component, or one is null.</exception>
        public VectorQuantityValue(IEnumerable<Quantity> components)
        {
            Components = new ValueList<Quantity>(components);
            if (Components.IsEmpty)
            {
                throw new ArgumentException("A vector quantity has at least one component.", nameof(components));
            }
        }

        /// <summary>The components, in order.</summary>
        public ValueList<Quantity> Components { get; }

        /// <summary>The number of components.</summary>
        public int Dimension => Components.Count;

        /// <summary>The one unit every component is written in, or null when they differ.</summary>
        public string? Unit => Shape.SharedUnit(Components);

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.VectorQuantity;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is VectorQuantityValue that && Components == that.Components;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Components);

        /// <summary>The components, as <c>VectorQuantity(3.0 [m], 4.0 [m])</c>.</summary>
        /// <returns>The vector quantity's text.</returns>
        public override string ToString() => $"VectorQuantity({ValueText.Join(Components)})";
    }

    /// <summary>
    /// A <c>Quantities::TensorQuantityValue</c> of any rank: its dimensions, and one quantity per
    /// component flattened in row-major order under them. A rank-one tensor stays a tensor, distinct
    /// from a <see cref="VectorQuantityValue"/>.
    /// </summary>
    /// <remarks>Only a service advertising <c>tensor_values</c> reports one as itself rather than as an unsupported null.</remarks>
    public sealed class TensorQuantityValue : Value
    {
        /// <summary>Creates a tensor quantity, copying the dimensions and the components.</summary>
        /// <param name="dimensions">The extent of each dimension, all positive.</param>
        /// <param name="components">The components in row-major order, exactly as many as the dimensions multiply to.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        /// <exception cref="ArgumentException">A component is null, a dimension is not positive, or the components do not fill the dimensions.</exception>
        public TensorQuantityValue(IEnumerable<long> dimensions, IEnumerable<Quantity> components)
        {
            Dimensions = new ValueList<long>(dimensions);
            Components = new ValueList<Quantity>(components);
            Shape.RequireFilled(Dimensions, Components.Count, "A tensor quantity", nameof(components));
        }

        /// <summary>The extent of each dimension.</summary>
        public ValueList<long> Dimensions { get; }

        /// <summary>The components, in row-major order.</summary>
        public ValueList<Quantity> Components { get; }

        /// <summary>The number of dimensions.</summary>
        public int Rank => Dimensions.Count;

        /// <summary>The one unit every component is written in, or null when they differ.</summary>
        public string? Unit => Shape.SharedUnit(Components);

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.TensorQuantity;

        /// <summary>The component at a multi-index.</summary>
        /// <param name="index">One coordinate per dimension, each within its extent.</param>
        /// <returns>The component there.</returns>
        /// <exception cref="ArgumentException">The index has another rank than the tensor.</exception>
        /// <exception cref="ArgumentOutOfRangeException">A coordinate is outside its dimension.</exception>
        public Quantity Get(params ReadOnlySpan<long> index) => Components[Shape.Offset(Dimensions, index, "tensor")];

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is TensorQuantityValue that && Dimensions == that.Dimensions && Components == that.Components;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Dimensions, Components);

        /// <summary>The dimensions and the row-major components, as <c>TensorQuantity(2)[1.0 [m], 2.0 [m]]</c>.</summary>
        /// <returns>The tensor's text.</returns>
        public override string ToString() => $"TensorQuantity({ValueText.Join(Dimensions)})[{ValueText.Join(Components)}]";
    }
}
