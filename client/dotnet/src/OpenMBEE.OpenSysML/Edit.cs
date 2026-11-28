using System;
using System.Collections.Generic;
using System.Runtime.CompilerServices;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One source-preserving change to a model's notation, applied by <see cref="Model.ApplyEditsAsync"/> or an
/// <see cref="Editor"/>: an immutable variant of the service's <c>EditOperation</c>, one nested sealed record per arm.
/// Derive one from another with a <c>with</c> expression; collections are copied when they are set.
/// </summary>
/// <remarks>
/// <para>
/// Every target and owner names its element as the model reports it (<see cref="Symbol.Id"/>, as
/// <c>Demo::Vehicle</c>), and an owner left empty names the document root. Notation the service writes (a type, a
/// value, an expression) is given as text and judged by the service, which refuses the whole batch, naming why, rather
/// than write a model that does not read back.
/// </para>
/// <para>
/// No type outside this library can derive from it, so the arms are exactly the nested records. An optional part is
/// null when it is not written; an empty one is refused before anything is sent, since the service reads empty as
/// absent.
/// </para>
/// </remarks>
public abstract partial record Edit
{
    private protected Edit()
    {
    }

    /// <summary>The arm of the service's <c>EditOperation</c> the edit is sent as, as the schema names it.</summary>
    internal abstract string Arm { get; }

    /// <summary>Sets the value of a feature that already exists, replacing the expression of its <c>= expr</c> or adding one before the declaration's <c>;</c>.</summary>
    public sealed record SetValue : Edit
    {
        private readonly string target = string.Empty;
        private readonly string text = string.Empty;

        /// <summary>Creates the edit.</summary>
        /// <param name="target">The feature, as <c>Demo::sc::unitMass</c>.</param>
        /// <param name="value">The new value in notation (<c>1050.0[SI::kg]</c>, <c>"m1"</c>, <c>true</c>, <c>mass * 2</c>).</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public SetValue(string target, string value)
        {
            Target = target;
            Value = value;
        }

        /// <summary>The feature, as <c>Demo::sc::unitMass</c>.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Target { get => target; init => target = Required(value); }

        /// <summary>The new value in notation; it must parse as an expression and resolve in the feature's own scope.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Value { get => text; init => text = Required(value); }

        internal override string Arm => "set_value";
    }

    /// <summary>
    /// Rewrites the name of a declaration and every reference to it in the model's documents. A rename reaching a
    /// reference in a document the edit cannot rewrite is refused, naming the referring declarations.
    /// </summary>
    public sealed record Rename : Edit
    {
        private readonly string target = string.Empty;
        private readonly string newName = string.Empty;

        /// <summary>Creates the edit.</summary>
        /// <param name="target">The declaration to rename.</param>
        /// <param name="newName">Its new name, which must lex as an identifier.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public Rename(string target, string newName)
        {
            Target = target;
            NewName = newName;
        }

        /// <summary>The declaration to rename.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Target { get => target; init => target = Required(value); }

        /// <summary>Its new name, which must lex as an identifier.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string NewName { get => newName; init => newName = Required(value); }

        internal override string Arm => "rename";
    }

    /// <summary>
    /// Inserts a declaration into a namespace or the document root: a definition or usage of any kind the document's
    /// language has, with its type, multiplicity, value, specializations, modifiers, metadata, documentation or body.
    /// </summary>
    public sealed record AddMember : Edit
    {
        private readonly string owner = string.Empty;
        private readonly string kind = string.Empty;
        private readonly string name = string.Empty;
        private readonly ValueList<string> specializes;
        private readonly ValueList<string> redefines;
        private readonly ValueList<string> metadataPrefixes;

        /// <summary>Creates the edit, writing nothing but the declaration's kind and name.</summary>
        /// <param name="owner">The namespace receiving the declaration; empty for the document root.</param>
        /// <param name="kind">
        /// The declaration kind as written, such as <c>part def</c>, <c>attribute</c>, <c>class</c>, <c>perform</c>,
        /// <c>exhibit state</c>, <c>entry action</c>, <c>assert constraint</c>, <c>objective</c> or <c>return</c>; empty
        /// for a directed usage written without one (<c>in x : T;</c>), which needs <see cref="Direction"/>.
        /// </param>
        /// <param name="name">The declared name; empty for an anonymous member such as a <c>return</c> or an <c>objective</c>.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddMember(string owner, string kind, string name)
        {
            Owner = owner;
            Kind = kind;
            Name = name;
        }

        /// <summary>The namespace receiving the declaration; empty for the document root.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Owner { get => owner; init => owner = Required(value); }

        /// <summary>The declaration kind as written; empty for a directed usage written without one.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Kind { get => kind; init => kind = Required(value); }

        /// <summary>The declared name; empty for an anonymous member.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Name { get => name; init => name = Required(value); }

        /// <summary>The type of a usage, written as notation after <c>:</c>; null for none.</summary>
        public string? Type { get; init; }

        /// <summary>The multiplicity, brackets included, such as <c>[0..*]</c>; null for none.</summary>
        public string? Multiplicity { get; init; }

        /// <summary>The value expression, written as notation after <c>=</c> (or <c>default =</c>); null for none.</summary>
        public string? Value { get; init; }

        /// <summary>The specialization targets of a definition, in order.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        /// <exception cref="ArgumentException">A target is null.</exception>
        public IReadOnlyList<string> Specializes { get => specializes; init => specializes = new ValueList<string>(value); }

        /// <summary>Whether the declaration is written <c>abstract</c>.</summary>
        public bool IsAbstract { get; init; }

        /// <summary>The redefinition targets of a usage, in order.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        /// <exception cref="ArgumentException">A target is null.</exception>
        public IReadOnlyList<string> Redefines { get => redefines; init => redefines = new ValueList<string>(value); }

        /// <summary>Whether <see cref="Value"/> is written with the <c>default</c> keyword.</summary>
        public bool IsDefault { get; init; }

        /// <summary>The direction of a usage: <c>in</c>, <c>out</c> or <c>inout</c>; null for none.</summary>
        public string? Direction { get; init; }

        /// <summary>The metadata definitions annotating the new member, written as prefixes, in order.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        /// <exception cref="ArgumentException">A definition is null.</exception>
        public IReadOnlyList<string> MetadataPrefixes { get => metadataPrefixes; init => metadataPrefixes = new ValueList<string>(value); }

        /// <summary>
        /// Documentation as plain text, written as the declaration's first body member <c>doc /* ... */</c>, which reads
        /// back as exactly this text; null for none.
        /// </summary>
        public string? Doc { get; init; }

        /// <summary>
        /// The body expression, written inside <c>{ ... }</c>: a constraint's condition, or the result of a calc, case,
        /// analysis, verification or use case; null for none. It is distinct from <see cref="Value"/>, which writes
        /// <c>= ...</c>.
        /// </summary>
        public string? BodyExpression { get; init; }

        internal override string Arm => "add_member";
    }

    /// <summary>Removes a declaration and its owned trivia, and with <see cref="Cascade"/> the declarations referring to it.</summary>
    public sealed record Delete : Edit
    {
        private readonly string target = string.Empty;

        /// <summary>Creates the edit.</summary>
        /// <param name="target">The declaration to remove.</param>
        /// <param name="cascade">Whether the declarations referring to it are removed too, rather than the delete refused.</param>
        /// <exception cref="ArgumentNullException"><paramref name="target"/> is null.</exception>
        public Delete(string target, bool cascade = false)
        {
            Target = target;
            Cascade = cascade;
        }

        /// <summary>The declaration to remove.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Target { get => target; init => target = Required(value); }

        /// <summary>Whether the declarations referring to it are removed too, in every document, rather than the delete refused.</summary>
        public bool Cascade { get; init; }

        internal override string Arm => "delete";
    }

    /// <summary>
    /// Re-parents a declaration, respelling the references the move breaks so the model stays valid. A move whose
    /// references cannot be respelled is refused, naming them, rather than leaving them dangling.
    /// </summary>
    public sealed record Move : Edit
    {
        private readonly string target = string.Empty;
        private readonly string owner = string.Empty;

        /// <summary>Creates the edit.</summary>
        /// <param name="target">The declaration to move.</param>
        /// <param name="owner">The namespace receiving it; empty for the document root.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public Move(string target, string owner)
        {
            Target = target;
            Owner = owner;
        }

        /// <summary>The declaration to move.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Target { get => target; init => target = Required(value); }

        /// <summary>The namespace receiving it; empty for the document root.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Owner { get => owner; init => owner = Required(value); }

        internal override string Arm => "move";
    }

    /// <summary>Inserts a usage whose ends connect two features: a connection, interface, allocation, binding, flow, succession or transition.</summary>
    public sealed record AddConnection : Edit
    {
        private readonly string owner = string.Empty;
        private readonly string kind = string.Empty;
        private readonly string fromEnd = string.Empty;
        private readonly string toEnd = string.Empty;

        /// <summary>Creates the edit.</summary>
        /// <param name="owner">The namespace receiving the usage; empty for the document root.</param>
        /// <param name="kind">
        /// The usage kind: in SysML <c>connection</c>, <c>interface</c>, <c>allocation</c>, <c>binding</c>, <c>flow</c>,
        /// <c>succession</c> or <c>transition</c>; in KerML <c>connector</c>, <c>binding</c>, <c>flow</c> or
        /// <c>succession</c>.
        /// </param>
        /// <param name="fromEnd">The first end, a feature reference resolved from the owner.</param>
        /// <param name="toEnd">The second end, a feature reference resolved from the owner.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddConnection(string owner, string kind, string fromEnd, string toEnd)
        {
            Owner = owner;
            Kind = kind;
            FromEnd = fromEnd;
            ToEnd = toEnd;
        }

        /// <summary>The namespace receiving the usage; empty for the document root.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Owner { get => owner; init => owner = Required(value); }

        /// <summary>The usage kind.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Kind { get => kind; init => kind = Required(value); }

        /// <summary>The first end, a feature reference resolved from the owner.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string FromEnd { get => fromEnd; init => fromEnd = Required(value); }

        /// <summary>The second end, a feature reference resolved from the owner.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string ToEnd { get => toEnd; init => toEnd = Required(value); }

        /// <summary>The declared name; null for an anonymous usage.</summary>
        public string? Name { get; init; }

        /// <summary>The typing target, which only the typed connection kinds take; null for none.</summary>
        public string? Type { get; init; }

        internal override string Arm => "add_connection";
    }

    /// <summary>Inserts a <c>satisfy</c> usage into any package or body that admits behavior usages.</summary>
    public sealed record AddSatisfy : Edit
    {
        private readonly string owner = string.Empty;
        private readonly string requirement = string.Empty;

        /// <summary>Creates the edit.</summary>
        /// <param name="owner">The namespace or body receiving the usage.</param>
        /// <param name="requirement">The requirement satisfied, as a feature reference.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddSatisfy(string owner, string requirement)
        {
            Owner = owner;
            Requirement = requirement;
        }

        /// <summary>The namespace or body receiving the usage.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Owner { get => owner; init => owner = Required(value); }

        /// <summary>The requirement satisfied, as a feature reference.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Requirement { get => requirement; init => requirement = Required(value); }

        /// <summary>The satisfying feature, written after <c>by</c>; null for none.</summary>
        public string? SatisfyingFeature { get; init; }

        /// <summary>Whether the usage is written <c>assert satisfy</c>.</summary>
        public bool IsAsserted { get; init; }

        /// <summary>Whether the usage is written <c>not satisfy</c>.</summary>
        public bool IsNegated { get; init; }

        internal override string Arm => "add_satisfy";
    }

    /// <summary>Inserts a <c>require</c> or <c>assume</c> constraint into a requirement-like namespace.</summary>
    public sealed record AddRequirementConstraint : Edit
    {
        private readonly string owner = string.Empty;
        private readonly string kind = string.Empty;
        private readonly string expression = string.Empty;

        /// <summary>Creates the edit.</summary>
        /// <param name="owner">The requirement-like namespace receiving the constraint.</param>
        /// <param name="kind">The constraint kind: <c>require</c> or <c>assume</c>.</param>
        /// <param name="expression">The constraint expression, written as notation.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddRequirementConstraint(string owner, string kind, string expression)
        {
            Owner = owner;
            Kind = kind;
            Expression = expression;
        }

        /// <summary>The requirement-like namespace receiving the constraint.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Owner { get => owner; init => owner = Required(value); }

        /// <summary>The constraint kind: <c>require</c> or <c>assume</c>.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Kind { get => kind; init => kind = Required(value); }

        /// <summary>The constraint expression, written as notation.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Expression { get => expression; init => expression = Required(value); }

        /// <summary>The constraint's declared name; null for an anonymous one.</summary>
        public string? Name { get; init; }

        internal override string Arm => "add_requirement_constraint";
    }

    /// <summary>
    /// Inserts a transition usage into a state body, or, with <see cref="Initial"/>, its entry transition
    /// <c>entry; then target;</c>.
    /// </summary>
    public sealed record AddTransition : Edit
    {
        private readonly string owner = string.Empty;
        private readonly string source = string.Empty;
        private readonly string target = string.Empty;

        /// <summary>Creates the edit of a regular transition.</summary>
        /// <param name="owner">The state body receiving the transition.</param>
        /// <param name="source">The source state, as a feature reference.</param>
        /// <param name="target">The target state, as a feature reference.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddTransition(string owner, string source, string target)
        {
            Owner = owner;
            Source = source;
            Target = target;
        }

        /// <summary>The state body receiving the transition.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Owner { get => owner; init => owner = Required(value); }

        /// <summary>The source state, as a feature reference; empty only for an entry transition.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Source { get => source; init => source = Required(value); }

        /// <summary>The target state, as a feature reference.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Target { get => target; init => target = Required(value); }

        /// <summary>The transition's declared name; null for an anonymous one.</summary>
        public string? Name { get; init; }

        /// <summary>The text written after <c>accept</c>; null for none.</summary>
        public string? Trigger { get; init; }

        /// <summary>The boolean expression written after <c>if</c>; null for none.</summary>
        public string? Guard { get; init; }

        /// <summary>The effect written after <c>do</c>; null for none.</summary>
        public string? Effect { get; init; }

        /// <summary>Whether <c>entry; then target;</c> is written instead of a regular transition.</summary>
        public bool Initial { get; init; }

        internal override string Arm => "add_transition";

        /// <summary>The entry transition of a state body, <c>entry; then target;</c>.</summary>
        /// <param name="owner">The state body receiving it.</param>
        /// <param name="target">The state entered first.</param>
        /// <returns>The edit.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public static AddTransition Entry(string owner, string target) => new(owner, string.Empty, target) { Initial = true };
    }

    /// <summary>Inserts an import into a namespace body or the document root.</summary>
    public sealed record AddImport : Edit
    {
        private readonly string owner = string.Empty;
        private readonly string target = string.Empty;
        private readonly ValueList<string> filters;

        /// <summary>Creates the edit.</summary>
        /// <param name="owner">The namespace receiving the import; empty for the document root.</param>
        /// <param name="target">The imported qualified name, optionally rooted <c>$::</c>, suffixed <c>::*</c> for a namespace import.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddImport(string owner, string target)
        {
            Owner = owner;
            Target = target;
        }

        /// <summary>The namespace receiving the import; empty for the document root.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Owner { get => owner; init => owner = Required(value); }

        /// <summary>The imported qualified name, optionally rooted <c>$::</c>, suffixed <c>::*</c> for a namespace import.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Target { get => target; init => target = Required(value); }

        /// <summary>The visibility written: <c>private</c>, <c>public</c> or <c>protected</c>; null writes <c>private</c>.</summary>
        public string? Visibility { get; init; }

        /// <summary>Whether <c>::**</c> is written, importing recursively.</summary>
        public bool IsRecursive { get; init; }

        /// <summary>Whether <c>import all</c> is written, importing non-public members too.</summary>
        public bool IsImportAll { get; init; }

        /// <summary>Filter conditions, each written as <c>[expression]</c>, in order.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        /// <exception cref="ArgumentException">A filter is null.</exception>
        public IReadOnlyList<string> Filters { get => filters; init => filters = new ValueList<string>(value); }

        internal override string Arm => "add_import";
    }

    /// <summary>
    /// Adds <c>doc /* ... */</c> as the first body member of an existing declaration, opening a body for one ended by
    /// <c>;</c>.
    /// </summary>
    public sealed record AddDocumentation : Edit
    {
        private readonly string target = string.Empty;
        private readonly string body = string.Empty;

        /// <summary>Creates the edit.</summary>
        /// <param name="target">The documented declaration.</param>
        /// <param name="body">The documentation as plain text, which reads back exactly, whitespace included; it may not contain <c>*/</c> or a carriage return.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddDocumentation(string target, string body)
        {
            Target = target;
            Body = body;
        }

        /// <summary>The documented declaration.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Target { get => target; init => target = Required(value); }

        /// <summary>The documentation as plain text, which reads back exactly, whitespace included.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Body { get => body; init => body = Required(value); }

        /// <summary>The documentation's declared name; null for none.</summary>
        public string? Name { get; init; }

        /// <summary>The locale, written as <c>locale "..."</c>; null for none.</summary>
        public string? Locale { get; init; }

        /// <summary>Whether the documentation the target already owns is rewritten rather than the edit refused.</summary>
        public bool Replace { get; init; }

        internal override string Arm => "add_documentation";
    }

    /// <summary>Inserts <c>verify requirement;</c> into a verification case or its objective.</summary>
    public sealed record AddVerify : Edit
    {
        private readonly string owner = string.Empty;
        private readonly string requirement = string.Empty;

        /// <summary>Creates the edit.</summary>
        /// <param name="owner">The verification case or objective receiving it.</param>
        /// <param name="requirement">The requirement verified, as a feature reference.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddVerify(string owner, string requirement)
        {
            Owner = owner;
            Requirement = requirement;
        }

        /// <summary>The verification case or objective receiving it.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Owner { get => owner; init => owner = Required(value); }

        /// <summary>The requirement verified, as a feature reference.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Requirement { get => requirement; init => requirement = Required(value); }

        internal override string Arm => "add_verify";
    }

    /// <summary>Inserts a metadata usage, <c>metadata T about a { f = v; }</c> or its shorthand <c>@T</c>.</summary>
    public sealed record AddMetadata : Edit
    {
        private readonly string owner = string.Empty;
        private readonly string metadataType = string.Empty;
        private readonly ValueList<string> about;
        private readonly ValueList<MetadataFeatureValue> values;

        /// <summary>Creates the edit.</summary>
        /// <param name="owner">The namespace or body receiving the usage.</param>
        /// <param name="metadataType">The metadata definition, as a reference.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddMetadata(string owner, string metadataType)
        {
            Owner = owner;
            MetadataType = metadataType;
        }

        /// <summary>The namespace or body receiving the usage.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Owner { get => owner; init => owner = Required(value); }

        /// <summary>The metadata definition, as a reference.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string MetadataType { get => metadataType; init => metadataType = Required(value); }

        /// <summary>The usage's declared name; null for an anonymous one.</summary>
        public string? Name { get; init; }

        /// <summary>The elements the usage is about, each as a reference, in order.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        /// <exception cref="ArgumentException">A reference is null.</exception>
        public IReadOnlyList<string> About { get => about; init => about = new ValueList<string>(value); }

        /// <summary>The feature values, in written order.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        /// <exception cref="ArgumentException">A value is null.</exception>
        public IReadOnlyList<MetadataFeatureValue> Values { get => values; init => values = new ValueList<MetadataFeatureValue>(value); }

        /// <summary>Whether the shorthand <c>@</c> notation is written instead of <c>metadata</c>.</summary>
        public bool Shorthand { get; init; }

        internal override string Arm => "add_metadata";
    }

    /// <summary>Annotates an existing declaration with a metadata prefix, <c>#T</c>.</summary>
    public sealed record AddMetadataPrefix : Edit
    {
        private readonly string target = string.Empty;
        private readonly string metadataType = string.Empty;

        /// <summary>Creates the edit.</summary>
        /// <param name="target">The declaration annotated.</param>
        /// <param name="metadataType">The metadata definition added as a prefix, as a reference.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddMetadataPrefix(string target, string metadataType)
        {
            Target = target;
            MetadataType = metadataType;
        }

        /// <summary>The declaration annotated.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Target { get => target; init => target = Required(value); }

        /// <summary>The metadata definition added as a prefix, as a reference.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string MetadataType { get => metadataType; init => metadataType = Required(value); }

        internal override string Arm => "add_metadata_prefix";
    }

    /// <summary>Inserts <c>comment name about a, b locale "..." /* ... */</c> where a new member of the owner goes.</summary>
    public sealed record AddComment : Edit
    {
        private readonly string owner = string.Empty;
        private readonly string body = string.Empty;
        private readonly ValueList<string> about;

        /// <summary>Creates the edit.</summary>
        /// <param name="owner">The namespace receiving the comment; empty for the document root.</param>
        /// <param name="body">The comment as plain text, which reads back exactly; it may not contain <c>*/</c> or a carriage return.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddComment(string owner, string body)
        {
            Owner = owner;
            Body = body;
        }

        /// <summary>The namespace receiving the comment; empty for the document root.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Owner { get => owner; init => owner = Required(value); }

        /// <summary>The comment as plain text, which reads back exactly.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Body { get => body; init => body = Required(value); }

        /// <summary>The comment's declared name; null for none.</summary>
        public string? Name { get; init; }

        /// <summary>The elements annotated, each a qualified name written after <c>about</c>, in order.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        /// <exception cref="ArgumentException">A name is null.</exception>
        public IReadOnlyList<string> About { get => about; init => about = new ValueList<string>(value); }

        /// <summary>The locale, written as <c>locale "..."</c>; null for none.</summary>
        public string? Locale { get; init; }

        internal override string Arm => "add_comment";
    }

    /// <summary>Writes the line note <c>// text</c> on its own line above a declaration. A note is lexical trivia, not a model element.</summary>
    public sealed record AddNote : Edit
    {
        private readonly string target = string.Empty;
        private readonly string text = string.Empty;

        /// <summary>Creates the edit.</summary>
        /// <param name="target">The declaration the note precedes.</param>
        /// <param name="text">The note, one line: it may not contain a line break.</param>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public AddNote(string target, string text)
        {
            Target = target;
            Text = text;
        }

        /// <summary>The declaration the note precedes.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Target { get => target; init => target = Required(value); }

        /// <summary>The note, one line.</summary>
        /// <exception cref="ArgumentNullException">It is set to null.</exception>
        public string Text { get => text; init => text = Required(value); }

        internal override string Arm => "add_note";
    }

    private static string Required(string value, [CallerArgumentExpression(nameof(value))] string? name = null)
    {
        ArgumentNullException.ThrowIfNull(value, name);
        return value;
    }
}
