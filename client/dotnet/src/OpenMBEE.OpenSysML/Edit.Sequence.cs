using System;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.CompilerServices;

namespace OpenMBEE.OpenSysML;

public abstract partial record Edit
{
    /// <summary>
    /// Inserts a sequencing member or action-body statement into an action body: <c>first start;</c>,
    /// <c>then done;</c>, <c>then action b;</c>, a guarded <c>if g then t;</c> and its <c>else t;</c>, or an
    /// <c>accept</c>, <c>send</c>, <c>assign</c>, <c>if</c>, <c>while</c>, <c>loop</c>, <c>for</c> or <c>terminate</c>
    /// statement, whose <see cref="Body"/> and <see cref="ElseBody"/> nest further items.
    /// </summary>
    /// <remarks>
    /// The factories write each form as the notation does; <see cref="Editor.Body"/> collects the items of a nested
    /// body, writing its first statement without <c>then</c>. An item nested inside another names no owner: the body it
    /// is in places it. The service reads items nested at most <see cref="MaxDepth"/> levels below their edit, so an item
    /// nested deeper cannot be built.
    /// </remarks>
    public sealed record AddSequence : Edit
    {
        /// <summary>The deepest an item can be nested below its edit, the most the service reads.</summary>
        public const int MaxDepth = 128;

        private readonly string owner = string.Empty;
        private readonly string keyword = string.Empty;
        private readonly ValueList<AddSequence> body;
        private readonly ValueList<AddSequence> elseBody;
        private readonly int height = 1;

        /// <summary>Creates the edit, writing nothing but its keyword; the factories write each form.</summary>
        /// <param name="owner">The action body receiving the item; empty for an item nested in another's body.</param>
        /// <param name="keyword">The sequencing keyword: <c>first</c>, <c>then</c>, <c>if</c> or <c>else</c>, or empty for a plain body item.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddSequence(string owner, string keyword)
        {
            Owner = owner;
            Keyword = keyword;
        }

        /// <summary>The action body receiving the item; empty for an item nested in another's body.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Owner { get => owner; init => owner = Required(value); }

        /// <summary>The sequencing keyword: <c>first</c>, <c>then</c>, <c>if</c> or <c>else</c>, or empty for a plain body item.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Keyword { get => keyword; init => keyword = Required(value); }

        /// <summary>The node a bare <c>first ref;</c>, <c>then ref;</c>, <c>if g then ref;</c> or <c>else ref;</c> names; null when the item declares a member instead.</summary>
        public string? Ref { get; init; }

        /// <summary>
        /// The kind of item declared: <c>action</c>, <c>perform action</c>, <c>state</c>, <c>merge</c>, <c>decide</c>,
        /// <c>join</c>, <c>fork</c>, <c>accept</c>, <c>send</c>, <c>assign</c>, <c>if</c>, <c>while</c>, <c>loop</c>,
        /// <c>for</c> or <c>terminate</c>; null for a bare reference.
        /// </summary>
        public string? MemberKind { get; init; }

        /// <summary>The declared member's name; null for an anonymous one.</summary>
        public string? MemberName { get; init; }

        /// <summary>The declared member's type, or an accepted payload's; null for none.</summary>
        public string? Type { get; init; }

        /// <summary>The name of the body member the new item follows; null to append it.</summary>
        public string? After { get; init; }

        /// <summary>The condition of an <c>if</c> or <c>while</c>, or the guard of a guarded succession; null for none.</summary>
        public string? Condition { get; init; }

        /// <summary>A sent payload, an assigned value, a <c>for</c> collection or a terminated occurrence; null for none.</summary>
        public string? Value { get; init; }

        /// <summary>The feature an <c>assign</c> assigns, or a <c>send</c>'s receiver; null for none.</summary>
        public string? Target { get; init; }

        /// <summary>The port an <c>accept</c> or <c>send</c> goes through; null for none.</summary>
        public string? Via { get; init; }

        /// <summary>The termination condition of a <c>while</c> or <c>loop</c>, written after <c>until</c>; null for none.</summary>
        public string? Until { get; init; }

        /// <summary>The items nested in the statement's body, in order; their owners are not read.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        /// <exception cref="ArgumentException">An item is null, or nested more than <see cref="MaxDepth"/> levels deep.</exception>
        public IReadOnlyList<AddSequence> Body
        {
            get => body;
            init
            {
                body = Nested(value);
                height = HeightOf(body, elseBody);
            }
        }

        /// <summary>
        /// The items of an <c>if</c>'s else branch, in order; empty writes no else branch, since an empty one performs
        /// nothing.
        /// </summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        /// <exception cref="ArgumentException">An item is null, or nested more than <see cref="MaxDepth"/> levels deep.</exception>
        public IReadOnlyList<AddSequence> ElseBody
        {
            get => elseBody;
            init
            {
                elseBody = Nested(value);
                height = HeightOf(body, elseBody);
            }
        }

        /// <summary>The source-end multiplicity of a <c>then</c>, brackets included, as <c>[1]</c>; null for none.</summary>
        public string? Multiplicity { get; init; }

        /// <summary>An accepted payload's name, or a <c>for</c> loop's variable; null for none.</summary>
        public string? Parameter { get; init; }

        /// <summary>How many levels the item spans, itself included: one for an item nesting none.</summary>
        internal int Height => height;

        internal override string Arm => "add_sequence";

        /// <summary><c>first ref;</c>: the node an action body starts at.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="reference">The node.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public static AddSequence First(string owner, string reference) => new(owner, "first") { Ref = Required(reference) };

        /// <summary><c>then ref;</c>: the node sequenced to.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="reference">The node.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public static AddSequence Then(string owner, string reference) => new(owner, "then") { Ref = Required(reference) };

        /// <summary><c>then kind name;</c>: a member declared where it is sequenced to.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="kind">The member kind, such as <c>action</c>, <c>merge</c> or <c>fork</c>.</param>
        /// <param name="name">The member's name.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public static AddSequence ThenMember(string owner, string kind, string name) =>
            new(owner, "then") { MemberKind = Required(kind), MemberName = Required(name) };

        /// <summary><c>kind name;</c>: a body item declared without <c>then</c>, such as an action node.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="kind">The item kind, such as <c>action</c>.</param>
        /// <param name="name">The item's name.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public static AddSequence Node(string owner, string kind, string name) =>
            new(owner, string.Empty) { MemberKind = Required(kind), MemberName = Required(name) };

        /// <summary><c>then accept payload;</c>; <see cref="Type"/> and <see cref="Via"/> add to it.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="payload">The accepted payload's name.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public static AddSequence Accept(string owner, string payload) => Statement(owner, "accept") with { Parameter = Required(payload) };

        /// <summary><c>then send payload to receiver;</c>; <see cref="Via"/> adds to it.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="payload">The payload expression.</param>
        /// <param name="to">The receiver expression; null for none.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="owner"/> or <paramref name="payload"/> is null.</exception>
        public static AddSequence Send(string owner, string payload, string? to = null) => Statement(owner, "send") with { Value = Required(payload), Target = to };

        /// <summary><c>then assign target := value;</c>.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="target">The assigned feature.</param>
        /// <param name="value">The assigned expression.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public static AddSequence Assign(string owner, string target, string value) => Statement(owner, "assign") with { Target = Required(target), Value = Required(value) };

        /// <summary><c>then if condition { body } else { elseBody }</c>.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="condition">The condition.</param>
        /// <param name="body">The items of the then branch.</param>
        /// <param name="elseBody">The items of the else branch; null or empty for none.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException">An argument other than <paramref name="elseBody"/> is null.</exception>
        /// <exception cref="ArgumentException">An item is null, or nested too deep.</exception>
        public static AddSequence If(string owner, string condition, IEnumerable<AddSequence> body, IEnumerable<AddSequence>? elseBody = null) =>
            Statement(owner, "if") with { Condition = Required(condition), Body = Listed(body), ElseBody = Listed(elseBody ?? []) };

        /// <summary><c>then while condition { body } until until;</c>.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="condition">The loop condition.</param>
        /// <param name="body">The loop's items.</param>
        /// <param name="until">The termination condition; null for none.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="owner"/>, <paramref name="condition"/> or <paramref name="body"/> is null.</exception>
        /// <exception cref="ArgumentException">An item is null, or nested too deep.</exception>
        public static AddSequence While(string owner, string condition, IEnumerable<AddSequence> body, string? until = null) =>
            Statement(owner, "while") with { Condition = Required(condition), Body = Listed(body), Until = until };

        /// <summary><c>then loop { body } until until;</c>.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="body">The loop's items.</param>
        /// <param name="until">The termination condition; null for none.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="owner"/> or <paramref name="body"/> is null.</exception>
        /// <exception cref="ArgumentException">An item is null, or nested too deep.</exception>
        public static AddSequence Loop(string owner, IEnumerable<AddSequence> body, string? until = null) =>
            Statement(owner, "loop") with { Body = Listed(body), Until = until };

        /// <summary><c>then for variable in collection { body }</c>.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="variable">The loop variable.</param>
        /// <param name="collection">The collection expression.</param>
        /// <param name="body">The loop's items.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        /// <exception cref="ArgumentException">An item is null, or nested too deep.</exception>
        public static AddSequence For(string owner, string variable, string collection, IEnumerable<AddSequence> body) =>
            Statement(owner, "for") with { Parameter = Required(variable), Value = Required(collection), Body = Listed(body) };

        /// <summary><c>then terminate occurrence;</c>.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="occurrence">The terminated occurrence; null for the performing one.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="owner"/> is null.</exception>
        public static AddSequence Terminate(string owner, string? occurrence = null) => Statement(owner, "terminate") with { Value = occurrence };

        /// <summary><c>if guard then ref;</c>: a guarded succession.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="guard">The guard expression.</param>
        /// <param name="reference">The node sequenced to.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public static AddSequence GuardedThen(string owner, string guard, string reference) => new(owner, "if") { Condition = Required(guard), Ref = Required(reference) };

        /// <summary><c>else ref;</c>: the succession closing a run of guarded ones.</summary>
        /// <param name="owner">The action body.</param>
        /// <param name="reference">The node sequenced to.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public static AddSequence Else(string owner, string reference) => new(owner, "else") { Ref = Required(reference) };

        /// <summary>The same item without its <c>then</c>, as the first statement of a body is written.</summary>
        /// <returns>The item, its <see cref="Keyword"/> empty.</returns>
        /// <exception cref="InvalidOperationException">The item is not a <c>then</c> item, or carries a source multiplicity, which is written with <c>then</c>.</exception>
        public AddSequence WithoutThen()
        {
            if (Keyword != "then")
            {
                throw new InvalidOperationException($"Only a then item can drop its then; this one's keyword is \"{Keyword}\".");
            }

            return Multiplicity is null
                ? this with { Keyword = string.Empty }
                : throw new InvalidOperationException("A source multiplicity is written with then, so this item cannot drop it.");
        }

        private static AddSequence Statement(string owner, string kind) => new(owner, "then") { MemberKind = kind };

        private static ValueList<AddSequence> Listed(IEnumerable<AddSequence> items, [CallerArgumentExpression(nameof(items))] string? name = null)
        {
            ArgumentNullException.ThrowIfNull(items, name);
            return new ValueList<AddSequence>(items);
        }

        private static ValueList<AddSequence> Nested(IEnumerable<AddSequence> items)
        {
            var nested = new ValueList<AddSequence>(items);
            for (var i = 0; i < nested.Count; i++)
            {
                if (nested[i].Height > MaxDepth)
                {
                    throw new ArgumentException(
                        $"Item {i} spans {nested[i].Height} levels, so nested here it would reach deeper than the {MaxDepth} levels the service reads.", nameof(items));
                }
            }

            return nested;
        }

        private static int HeightOf(ValueList<AddSequence> body, ValueList<AddSequence> elseBody) =>
            1 + body.Concat(elseBody).Select(item => item.Height).DefaultIfEmpty(0).Max();
    }
}
