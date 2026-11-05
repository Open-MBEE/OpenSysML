using System;
using System.Collections.Generic;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A filter a <see cref="Query"/> applies, the standard's <c>Constraint</c>: a <see cref="Comparison"/> of one
/// property against values, or a <see cref="Composite"/> of constraints. One nested sealed class per form, each
/// immutable and compared by content.
/// </summary>
/// <remarks>
/// A constraint is complete by construction: a comparison always names its property and operator, <c>&lt;</c>,
/// <c>&lt;=</c>, <c>&gt;</c> and <c>&gt;=</c> compare against exactly one value, <c>=</c> and <c>in</c> against at
/// least one, and <c>instanceOf</c> compares <c>@type</c> against at least one metaclass name; a composite combines at
/// least two constraints, as the standard's <c>CompositeConstraint</c> does. What the service still judges is whether
/// the property exists and is ordered, whether a value parses as the property's kind, and whether the library declares
/// each metaclass named.
/// </remarks>
public abstract class QueryConstraint : IEquatable<QueryConstraint>
{
    /// <summary>The only property <see cref="ComparisonOperator.InstanceOf"/> compares: the element's metaclass.</summary>
    private const string TypeProperty = "@type";

    private QueryConstraint()
    {
    }

    /// <summary>Matches an element whose property equals any of the values.</summary>
    /// <param name="property">The property, as the query property table names it (<c>@type</c>, <c>name</c>).</param>
    /// <param name="values">The values; at least one.</param>
    /// <returns>The comparison.</returns>
    /// <exception cref="ArgumentNullException">An argument or a value is null.</exception>
    /// <exception cref="ArgumentException">The property is empty, or there is no value.</exception>
    public static Comparison Equal(string property, params IEnumerable<string> values) => new(property, ComparisonOperator.Equal, values);

    /// <summary>Matches an element whose ordered property is greater than the value.</summary>
    /// <param name="property">The property, as the query property table names it.</param>
    /// <param name="value">The value.</param>
    /// <returns>The comparison.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">The property is empty.</exception>
    public static Comparison Greater(string property, string value) => new(property, ComparisonOperator.Greater, [value]);

    /// <summary>Matches an element whose ordered property is at least the value.</summary>
    /// <param name="property">The property, as the query property table names it.</param>
    /// <param name="value">The value.</param>
    /// <returns>The comparison.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">The property is empty.</exception>
    public static Comparison GreaterOrEqual(string property, string value) => new(property, ComparisonOperator.GreaterOrEqual, [value]);

    /// <summary>Matches an element whose ordered property is less than the value.</summary>
    /// <param name="property">The property, as the query property table names it.</param>
    /// <param name="value">The value.</param>
    /// <returns>The comparison.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">The property is empty.</exception>
    public static Comparison Less(string property, string value) => new(property, ComparisonOperator.Less, [value]);

    /// <summary>Matches an element whose ordered property is at most the value.</summary>
    /// <param name="property">The property, as the query property table names it.</param>
    /// <param name="value">The value.</param>
    /// <returns>The comparison.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">The property is empty.</exception>
    public static Comparison LessOrEqual(string property, string value) => new(property, ComparisonOperator.LessOrEqual, [value]);

    /// <summary>Matches an element whose property is one of the values, the standard's <c>in</c>.</summary>
    /// <param name="property">The property, as the query property table names it.</param>
    /// <param name="values">The values it may be one of; at least one.</param>
    /// <returns>The comparison.</returns>
    /// <exception cref="ArgumentNullException">An argument or a value is null.</exception>
    /// <exception cref="ArgumentException">The property is empty, or there is no value.</exception>
    public static Comparison In(string property, params IEnumerable<string> values) => new(property, ComparisonOperator.In, values);

    /// <summary>
    /// Matches an element whose metaclass conforms to any of the metaclasses named (<c>Usage</c>, <c>Feature</c>,
    /// <c>Element</c>), the standard's <c>instanceOf</c> on <c>@type</c>. The service refuses a name the library
    /// declares no metaclass for.
    /// </summary>
    /// <param name="metaclasses">The metaclass names; at least one, none empty.</param>
    /// <returns>The comparison.</returns>
    /// <exception cref="ArgumentNullException">The names, or one of them, are null.</exception>
    /// <exception cref="ArgumentException">There is no name, or a name is empty.</exception>
    public static Comparison InstanceOf(params IEnumerable<string> metaclasses) => new(TypeProperty, ComparisonOperator.InstanceOf, metaclasses);

    /// <summary>Matches an element every constraint matches.</summary>
    /// <param name="constraints">The constraints; at least two.</param>
    /// <returns>The composite.</returns>
    /// <exception cref="ArgumentNullException">The constraints, or one of them, are null.</exception>
    /// <exception cref="ArgumentException">There are fewer than two constraints.</exception>
    public static Composite And(params IEnumerable<QueryConstraint> constraints) => new(CompositeOperator.And, constraints);

    /// <summary>Matches an element at least one constraint matches.</summary>
    /// <param name="constraints">The constraints; at least two.</param>
    /// <returns>The composite.</returns>
    /// <exception cref="ArgumentNullException">The constraints, or one of them, are null.</exception>
    /// <exception cref="ArgumentException">There are fewer than two constraints.</exception>
    public static Composite Or(params IEnumerable<QueryConstraint> constraints) => new(CompositeOperator.Or, constraints);

    /// <inheritdoc/>
    public abstract bool Equals([NotNullWhen(true)] QueryConstraint? other);

    /// <inheritdoc/>
    public sealed override bool Equals([NotNullWhen(true)] object? obj) => obj is QueryConstraint other && Equals(other);

    /// <inheritdoc/>
    public abstract override int GetHashCode();

    /// <summary>The constraint as a reader writes it: <c>@type = "PartUsage"</c>, <c>(a and b)</c>.</summary>
    /// <returns>The constraint's text.</returns>
    public abstract override string ToString();

    /// <summary>Whether two constraints are equal.</summary>
    /// <param name="left">A constraint.</param>
    /// <param name="right">Another constraint.</param>
    /// <returns>Whether they are equal.</returns>
    public static bool operator ==(QueryConstraint? left, QueryConstraint? right) => left is null ? right is null : left.Equals(right);

    /// <summary>Whether two constraints differ.</summary>
    /// <param name="left">A constraint.</param>
    /// <param name="right">Another constraint.</param>
    /// <returns>Whether they differ.</returns>
    public static bool operator !=(QueryConstraint? left, QueryConstraint? right) => !(left == right);

    /// <summary>One property of an element compared against values, the standard's <c>PrimitiveConstraint</c>.</summary>
    public sealed class Comparison : QueryConstraint
    {
        /// <summary>Creates a comparison.</summary>
        /// <param name="property">
        /// The property, as the query property table names it (<c>@type</c>, <c>name</c>); <c>@type</c> for
        /// <see cref="ComparisonOperator.InstanceOf"/>.
        /// </param>
        /// <param name="operator">How the property is compared.</param>
        /// <param name="values">
        /// The values: exactly one for an ordered comparison (<see cref="ComparisonOperator.Less"/>,
        /// <see cref="ComparisonOperator.LessOrEqual"/>, <see cref="ComparisonOperator.Greater"/>,
        /// <see cref="ComparisonOperator.GreaterOrEqual"/>), at least one otherwise, and for
        /// <see cref="ComparisonOperator.InstanceOf"/> metaclass names, none empty.
        /// </param>
        /// <param name="inverse">Whether the comparison's verdict is negated.</param>
        /// <exception cref="ArgumentNullException">The property, the values or a value is null.</exception>
        /// <exception cref="ArgumentException">
        /// The property is empty, there are not as many values as the operator compares against, or an
        /// <see cref="ComparisonOperator.InstanceOf"/> comparison is on a property other than <c>@type</c> or names an
        /// empty metaclass.
        /// </exception>
        /// <exception cref="ArgumentOutOfRangeException">The operator is not a <see cref="ComparisonOperator"/>.</exception>
        public Comparison(string property, ComparisonOperator @operator, IEnumerable<string> values, bool inverse = false)
        {
            ArgumentException.ThrowIfNullOrEmpty(property);
            if (!Enum.IsDefined(@operator))
            {
                throw new ArgumentOutOfRangeException(
                    nameof(@operator), @operator, "A comparison names its operator: Equal, Greater, Less, GreaterOrEqual, LessOrEqual, In or InstanceOf.");
            }

            var held = new ValueList<string>(values);
            if (held.IsEmpty)
            {
                throw new ArgumentException("A comparison compares against at least one value.", nameof(values));
            }

            if (IsOrdered(@operator) && held.Count != 1)
            {
                throw new ArgumentException($"A {@operator} comparison compares against exactly one value, not {held.Count}.", nameof(values));
            }

            if (@operator == ComparisonOperator.InstanceOf)
            {
                if (property != TypeProperty)
                {
                    throw new ArgumentException(
                        $"An InstanceOf comparison compares {TypeProperty}, the element's metaclass, not {ValueText.Quoted(property)}.", nameof(property));
                }

                if (held.Any(string.IsNullOrEmpty))
                {
                    throw new ArgumentException("An InstanceOf comparison compares against metaclass names, and one is empty.", nameof(values));
                }
            }

            Property = property;
            Operator = @operator;
            Values = held;
            Inverse = inverse;
        }

        /// <summary>The property compared.</summary>
        public string Property { get; }

        /// <summary>How the property is compared.</summary>
        public ComparisonOperator Operator { get; }

        /// <summary>
        /// The values compared against: with <see cref="ComparisonOperator.Equal"/>, <see cref="ComparisonOperator.In"/>
        /// and <see cref="ComparisonOperator.InstanceOf"/>, an element matches any of them.
        /// </summary>
        public ValueList<string> Values { get; }

        /// <summary>Whether the comparison's verdict is negated.</summary>
        public bool Inverse { get; }

        /// <summary>The same comparison with its verdict negated, or no longer negated.</summary>
        /// <returns>The inverted comparison.</returns>
        public Comparison Inverted() => new(Property, Operator, Values, !Inverse);

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] QueryConstraint? other) =>
            other is Comparison that && Property == that.Property && Operator == that.Operator && Values == that.Values && Inverse == that.Inverse;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(typeof(Comparison), Property, Operator, Values, Inverse);

        /// <inheritdoc/>
        public override string ToString()
        {
            var symbol = Operator switch
            {
                ComparisonOperator.Equal => "=",
                ComparisonOperator.Greater => ">",
                ComparisonOperator.Less => "<",
                ComparisonOperator.GreaterOrEqual => ">=",
                ComparisonOperator.LessOrEqual => "<=",
                ComparisonOperator.In => "in",
                _ => "instanceOf",
            };
            var operands = Values.Count == 1 && Operator != ComparisonOperator.In
                ? ValueText.Quoted(Values[0])
                : $"({string.Join(", ", Values.Select(ValueText.Quoted))})";
            return $"{(Inverse ? "not " : string.Empty)}{Property} {symbol} {operands}";
        }

        private static bool IsOrdered(ComparisonOperator @operator) => @operator is
            ComparisonOperator.Greater or ComparisonOperator.Less or ComparisonOperator.GreaterOrEqual or ComparisonOperator.LessOrEqual;
    }

    /// <summary>Constraints combined by one operator, the standard's <c>CompositeConstraint</c>.</summary>
    public sealed class Composite : QueryConstraint
    {
        /// <summary>Creates a composite.</summary>
        /// <param name="operator">How the constraints are combined.</param>
        /// <param name="constraints">The constraints; at least two, as the standard combines.</param>
        /// <exception cref="ArgumentNullException">The constraints, or one of them, are null.</exception>
        /// <exception cref="ArgumentException">There are fewer than two constraints.</exception>
        /// <exception cref="ArgumentOutOfRangeException">The operator is not a <see cref="CompositeOperator"/>.</exception>
        public Composite(CompositeOperator @operator, IEnumerable<QueryConstraint> constraints)
        {
            if (!Enum.IsDefined(@operator))
            {
                throw new ArgumentOutOfRangeException(nameof(@operator), @operator, "A composite names its operator: And or Or.");
            }

            var held = new ValueList<QueryConstraint>(constraints);
            if (held.Count < 2)
            {
                throw new ArgumentException(
                    held.IsEmpty
                        ? "A composite combines at least two constraints, got none."
                        : "A composite combines at least two constraints, got one; write the constraint on its own.",
                    nameof(constraints));
            }

            Operator = @operator;
            Constraints = held;
        }

        /// <summary>How the constraints are combined.</summary>
        public CompositeOperator Operator { get; }

        /// <summary>The constraints combined, in order.</summary>
        public ValueList<QueryConstraint> Constraints { get; }

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] QueryConstraint? other) =>
            other is Composite that && Operator == that.Operator && Constraints == that.Constraints;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(typeof(Composite), Operator, Constraints);

        /// <inheritdoc/>
        public override string ToString() =>
            $"({string.Join(Operator == CompositeOperator.And ? " and " : " or ", Constraints.Select(constraint => constraint.ToString()))})";
    }
}
