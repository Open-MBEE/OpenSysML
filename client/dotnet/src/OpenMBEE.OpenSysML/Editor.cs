using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// Collects edits of one model, in the order they are added, and applies them as one batch: all of them or none.
/// Every method names elements as the model reports them and returns this editor, so additions chain; <see cref="Add"/>
/// takes any <see cref="Edit"/>, with every option its record has. Obtained from <see cref="Model.Edit"/>.
/// </summary>
/// <remarks>
/// <para>
/// <see cref="ApplyAsync"/> sends the edits collected when it is called. Once it succeeds the editor is spent: its
/// edits describe the model it was made from, so load the edited notation and start another to edit further. A failed
/// apply leaves the editor as it was, to apply again. An editor may be shared between threads; edits added while it is
/// being applied are refused rather than lost.
/// </para>
/// <example>
/// <code>
/// var result = await model.Edit()
///     .SetValue("Demo::sc::unitMass", "1050.0[SI::kg]")
///     .AddMember("Demo::Vehicle", "part", "engine", m => m with { Type = "Engine" })
///     .ApplyAsync();
/// </code>
/// </example>
/// </remarks>
public sealed class Editor
{
    private const int Open = 0;
    private const int Applying = 1;
    private const int Applied = 2;

    private readonly Lock gate = new();
    private readonly List<Edit> edits = [];
    private int state;

    internal Editor(Model model) => Model = model;

    /// <summary>The model the edits apply to.</summary>
    public Model Model { get; }

    /// <summary>The edits collected so far, in order.</summary>
    public ValueList<Edit> Edits
    {
        get
        {
            lock (gate)
            {
                return new ValueList<Edit>(edits);
            }
        }
    }

    /// <summary>Whether <see cref="ApplyAsync"/> has succeeded, after which the editor takes no more edits.</summary>
    public bool IsApplied
    {
        get
        {
            lock (gate)
            {
                return state == Applied;
            }
        }
    }

    /// <summary>Adds any edit.</summary>
    /// <param name="edit">The edit.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="edit"/> is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor Add(Edit edit)
    {
        ArgumentNullException.ThrowIfNull(edit);
        lock (gate)
        {
            RequireOpen();
            edits.Add(edit);
        }

        return this;
    }

    /// <summary>
    /// Applies the edits collected, as <see cref="Model.ApplyEditsAsync"/> does; an empty editor is sent too, and the
    /// service refuses it as <see cref="EditFailure.NoOperations"/>.
    /// </summary>
    /// <param name="options">The document edited and whether its documents are read; null for <see cref="EditOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The edited notation and what each edit changed.</returns>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    /// <exception cref="InvalidRequestException">An edit or an option is invalid; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks a capability the edits or options need; nothing was sent.</exception>
    /// <exception cref="EditException">The service refused the batch; nothing of it was applied.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<EditResult> ApplyAsync(EditOptions? options = null, CancellationToken cancellationToken = default)
    {
        ValueList<Edit> batch;
        lock (gate)
        {
            RequireOpen();
            state = Applying;
            batch = new ValueList<Edit>(edits);
        }

        try
        {
            var result = await Model.ApplyEditsAsync(batch, options, cancellationToken).ConfigureAwait(false);
            lock (gate)
            {
                state = Applied;
            }

            return result;
        }
        catch
        {
            lock (gate)
            {
                state = Open;
            }

            throw;
        }
    }

    /// <summary>Sets the value of an existing feature, as <see cref="Edit.SetValue"/>.</summary>
    /// <param name="target">The feature.</param>
    /// <param name="value">The new value in notation.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor SetValue(string target, string value) => Add(new Edit.SetValue(target, value));

    /// <summary>Renames a declaration and every reference to it, as <see cref="Edit.Rename"/>.</summary>
    /// <param name="target">The declaration.</param>
    /// <param name="newName">The new name.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor Rename(string target, string newName) => Add(new Edit.Rename(target, newName));

    /// <summary>Removes a declaration, as <see cref="Edit.Delete"/>.</summary>
    /// <param name="target">The declaration.</param>
    /// <param name="cascade">Whether the declarations referring to it go too, rather than the delete refused.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="target"/> is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor Delete(string target, bool cascade = false) => Add(new Edit.Delete(target, cascade));

    /// <summary>Moves a declaration to another namespace, respelling the references the move breaks, as <see cref="Edit.Move"/>.</summary>
    /// <param name="target">The declaration.</param>
    /// <param name="owner">The receiving namespace; empty for the document root.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor Move(string target, string owner) => Add(new Edit.Move(target, owner));

    /// <summary>Adds a declaration of any kind, as <see cref="Edit.AddMember"/>.</summary>
    /// <param name="owner">The receiving namespace; empty for the document root.</param>
    /// <param name="kind">The declaration kind as written, such as <c>part def</c> or <c>attribute</c>.</param>
    /// <param name="name">The declared name.</param>
    /// <param name="configure">What else to write, such as <c>m => m with { Type = "Engine" }</c>; null for nothing.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="configure"/> is null, or <paramref name="configure"/> answers null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddMember(string owner, string kind, string name, Func<Edit.AddMember, Edit.AddMember>? configure = null) =>
        Add(Configured(new Edit.AddMember(owner, kind, name), configure));

    /// <summary>Adds a directed parameter written without a kind, <c>in name : type;</c>.</summary>
    /// <param name="owner">The receiving declaration.</param>
    /// <param name="direction">The direction: <c>in</c>, <c>out</c> or <c>inout</c>.</param>
    /// <param name="name">The parameter's name.</param>
    /// <param name="type">Its type; null for none.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="type"/> is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddParameter(string owner, string direction, string name, string? type = null)
    {
        ArgumentNullException.ThrowIfNull(direction);
        return Add(new Edit.AddMember(owner, string.Empty, name) { Direction = direction, Type = type });
    }

    /// <summary>Adds a usage connecting two features, as <see cref="Edit.AddConnection"/>.</summary>
    /// <param name="owner">The receiving namespace; empty for the document root.</param>
    /// <param name="kind">The usage kind, such as <c>connection</c>, <c>allocation</c>, <c>flow</c> or <c>succession</c>.</param>
    /// <param name="fromEnd">The first end.</param>
    /// <param name="toEnd">The second end.</param>
    /// <param name="configure">What else to write, such as a name or a type; null for nothing.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="configure"/> is null, or <paramref name="configure"/> answers null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddConnection(string owner, string kind, string fromEnd, string toEnd, Func<Edit.AddConnection, Edit.AddConnection>? configure = null) =>
        Add(Configured(new Edit.AddConnection(owner, kind, fromEnd, toEnd), configure));

    /// <summary>Adds a <c>satisfy</c> usage, as <see cref="Edit.AddSatisfy"/>.</summary>
    /// <param name="owner">The receiving namespace or body.</param>
    /// <param name="requirement">The requirement satisfied.</param>
    /// <param name="configure">What else to write, such as the satisfying feature; null for nothing.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="configure"/> is null, or <paramref name="configure"/> answers null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddSatisfy(string owner, string requirement, Func<Edit.AddSatisfy, Edit.AddSatisfy>? configure = null) =>
        Add(Configured(new Edit.AddSatisfy(owner, requirement), configure));

    /// <summary>Adds a <c>require</c> or <c>assume</c> constraint, as <see cref="Edit.AddRequirementConstraint"/>.</summary>
    /// <param name="owner">The requirement-like namespace receiving it.</param>
    /// <param name="kind">The constraint kind: <c>require</c> or <c>assume</c>.</param>
    /// <param name="expression">The constraint expression.</param>
    /// <param name="name">Its declared name; null for an anonymous one.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="name"/> is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddRequirementConstraint(string owner, string kind, string expression, string? name = null) =>
        Add(new Edit.AddRequirementConstraint(owner, kind, expression) { Name = name });

    /// <summary>Adds a transition to a state body, as <see cref="Edit.AddTransition"/>.</summary>
    /// <param name="owner">The state body.</param>
    /// <param name="source">The source state.</param>
    /// <param name="target">The target state.</param>
    /// <param name="configure">What else to write, such as a name, trigger, guard or effect; null for nothing.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="configure"/> is null, or <paramref name="configure"/> answers null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddTransition(string owner, string source, string target, Func<Edit.AddTransition, Edit.AddTransition>? configure = null) =>
        Add(Configured(new Edit.AddTransition(owner, source, target), configure));

    /// <summary>Adds a state body's entry transition, <c>entry; then target;</c>, as <see cref="Edit.AddTransition.Entry"/>.</summary>
    /// <param name="owner">The state body.</param>
    /// <param name="target">The state entered first.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddEntryTransition(string owner, string target) => Add(Edit.AddTransition.Entry(owner, target));

    /// <summary>Adds an import, as <see cref="Edit.AddImport"/>.</summary>
    /// <param name="owner">The receiving namespace; empty for the document root.</param>
    /// <param name="target">The imported name, <c>A::*</c> for a namespace import.</param>
    /// <param name="configure">What else to write, such as the visibility, recursion or filters; null for nothing.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="configure"/> is null, or <paramref name="configure"/> answers null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddImport(string owner, string target, Func<Edit.AddImport, Edit.AddImport>? configure = null) =>
        Add(Configured(new Edit.AddImport(owner, target), configure));

    /// <summary>Documents an existing declaration, as <see cref="Edit.AddDocumentation"/>.</summary>
    /// <param name="target">The documented declaration.</param>
    /// <param name="body">The documentation as plain text.</param>
    /// <param name="configure">What else to write, such as a name or locale, or to replace documentation already there; null for nothing.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="configure"/> is null, or <paramref name="configure"/> answers null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddDocumentation(string target, string body, Func<Edit.AddDocumentation, Edit.AddDocumentation>? configure = null) =>
        Add(Configured(new Edit.AddDocumentation(target, body), configure));

    /// <summary>Adds <c>verify requirement;</c> to a verification case or its objective, as <see cref="Edit.AddVerify"/>.</summary>
    /// <param name="owner">The case or objective.</param>
    /// <param name="requirement">The requirement verified.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddVerify(string owner, string requirement) => Add(new Edit.AddVerify(owner, requirement));

    /// <summary>Adds a metadata usage, as <see cref="Edit.AddMetadata"/>.</summary>
    /// <param name="owner">The receiving namespace or body.</param>
    /// <param name="metadataType">The metadata definition.</param>
    /// <param name="configure">What else to write, such as a name, what it is about, its feature values or the shorthand; null for nothing.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="configure"/> is null, or <paramref name="configure"/> answers null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddMetadata(string owner, string metadataType, Func<Edit.AddMetadata, Edit.AddMetadata>? configure = null) =>
        Add(Configured(new Edit.AddMetadata(owner, metadataType), configure));

    /// <summary>Annotates an existing declaration with a metadata prefix, as <see cref="Edit.AddMetadataPrefix"/>.</summary>
    /// <param name="target">The declaration.</param>
    /// <param name="metadataType">The metadata definition.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddMetadataPrefix(string target, string metadataType) => Add(new Edit.AddMetadataPrefix(target, metadataType));

    /// <summary>Adds a comment, as <see cref="Edit.AddComment"/>.</summary>
    /// <param name="owner">The receiving namespace; empty for the document root.</param>
    /// <param name="body">The comment as plain text.</param>
    /// <param name="configure">What else to write, such as a name, what it is about or its locale; null for nothing.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="configure"/> is null, or <paramref name="configure"/> answers null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddComment(string owner, string body, Func<Edit.AddComment, Edit.AddComment>? configure = null) =>
        Add(Configured(new Edit.AddComment(owner, body), configure));

    /// <summary>Writes the line note <c>// text</c> above a declaration, as <see cref="Edit.AddNote"/>.</summary>
    /// <param name="target">The declaration.</param>
    /// <param name="text">The note, one line.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddNote(string target, string text) => Add(new Edit.AddNote(target, text));

    /// <summary>Adds <c>first ref;</c> to an action body, as <see cref="Edit.AddSequence.First"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="reference">The node it starts at.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddFirst(string owner, string reference) => Add(Edit.AddSequence.First(owner, reference));

    /// <summary>Adds <c>then ref;</c> to an action body, as <see cref="Edit.AddSequence.Then"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="reference">The node sequenced to.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddThen(string owner, string reference) => Add(Edit.AddSequence.Then(owner, reference));

    /// <summary>Adds <c>then kind name;</c> to an action body, declaring the member sequenced to, as <see cref="Edit.AddSequence.ThenMember"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="kind">The member kind, such as <c>action</c>, <c>merge</c> or <c>fork</c>.</param>
    /// <param name="name">The member's name.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddThenMember(string owner, string kind, string name) => Add(Edit.AddSequence.ThenMember(owner, kind, name));

    /// <summary>Adds <c>then accept payload;</c> to an action body, as <see cref="Edit.AddSequence.Accept"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="payload">The accepted payload's name.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddAccept(string owner, string payload) => Add(Edit.AddSequence.Accept(owner, payload));

    /// <summary>Adds <c>then send payload to receiver;</c> to an action body, as <see cref="Edit.AddSequence.Send"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="payload">The payload expression.</param>
    /// <param name="to">The receiver expression; null for none.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="owner"/> or <paramref name="payload"/> is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddSend(string owner, string payload, string? to = null) => Add(Edit.AddSequence.Send(owner, payload, to));

    /// <summary>Adds <c>then assign target := value;</c> to an action body, as <see cref="Edit.AddSequence.Assign"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="target">The assigned feature.</param>
    /// <param name="value">The assigned expression.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddAssign(string owner, string target, string value) => Add(Edit.AddSequence.Assign(owner, target, value));

    /// <summary>Adds <c>then if condition { body } else { elseBody }</c> to an action body, as <see cref="Edit.AddSequence.If"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="condition">The condition.</param>
    /// <param name="body">The then branch, as collected when this is called.</param>
    /// <param name="elseBody">The else branch; null or empty for none.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="elseBody"/> is null.</exception>
    /// <exception cref="ArgumentException">An item is nested too deep.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddIf(string owner, string condition, Body body, Body? elseBody = null)
    {
        ArgumentNullException.ThrowIfNull(body);
        return Add(Edit.AddSequence.If(owner, condition, body.Items, elseBody?.Items));
    }

    /// <summary>Adds <c>then while condition { body } until until;</c> to an action body, as <see cref="Edit.AddSequence.While"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="condition">The loop condition.</param>
    /// <param name="body">The loop's items, as collected when this is called.</param>
    /// <param name="until">The termination condition; null for none.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="owner"/>, <paramref name="condition"/> or <paramref name="body"/> is null.</exception>
    /// <exception cref="ArgumentException">An item is nested too deep.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddWhile(string owner, string condition, Body body, string? until = null)
    {
        ArgumentNullException.ThrowIfNull(body);
        return Add(Edit.AddSequence.While(owner, condition, body.Items, until));
    }

    /// <summary>Adds <c>then loop { body } until until;</c> to an action body, as <see cref="Edit.AddSequence.Loop"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="body">The loop's items, as collected when this is called.</param>
    /// <param name="until">The termination condition; null for none.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="owner"/> or <paramref name="body"/> is null.</exception>
    /// <exception cref="ArgumentException">An item is nested too deep.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddLoop(string owner, Body body, string? until = null)
    {
        ArgumentNullException.ThrowIfNull(body);
        return Add(Edit.AddSequence.Loop(owner, body.Items, until));
    }

    /// <summary>Adds <c>then for variable in collection { body }</c> to an action body, as <see cref="Edit.AddSequence.For"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="variable">The loop variable.</param>
    /// <param name="collection">The collection expression.</param>
    /// <param name="body">The loop's items, as collected when this is called.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">An item is nested too deep.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddFor(string owner, string variable, string collection, Body body)
    {
        ArgumentNullException.ThrowIfNull(body);
        return Add(Edit.AddSequence.For(owner, variable, collection, body.Items));
    }

    /// <summary>Adds <c>then terminate occurrence;</c> to an action body, as <see cref="Edit.AddSequence.Terminate"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="occurrence">The terminated occurrence; null for the performing one.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="owner"/> is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddTerminate(string owner, string? occurrence = null) => Add(Edit.AddSequence.Terminate(owner, occurrence));

    /// <summary>Adds the guarded succession <c>if guard then ref;</c> to an action body, as <see cref="Edit.AddSequence.GuardedThen"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="guard">The guard expression.</param>
    /// <param name="reference">The node sequenced to.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddGuardedThen(string owner, string guard, string reference) => Add(Edit.AddSequence.GuardedThen(owner, guard, reference));

    /// <summary>Adds <c>else ref;</c>, closing a run of guarded successions, as <see cref="Edit.AddSequence.Else"/>.</summary>
    /// <param name="owner">The action body.</param>
    /// <param name="reference">The node sequenced to.</param>
    /// <returns>This editor.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="InvalidOperationException">The editor is being applied, or was applied.</exception>
    public Editor AddElse(string owner, string reference) => Add(Edit.AddSequence.Else(owner, reference));

    private static T Configured<T>(T edit, Func<T, T>? configure)
        where T : Edit
    {
        if (configure is null)
        {
            return edit;
        }

        var configured = configure(edit);
        ArgumentNullException.ThrowIfNull(configured, nameof(configure));
        return configured;
    }

    private void RequireOpen()
    {
        switch (state)
        {
            case Applying:
                throw new InvalidOperationException("This editor is being applied; wait for its answer, or start another editor.");
            case Applied:
                throw new InvalidOperationException(
                    "This editor was applied: its edits describe the model it was made from, so load the edited notation and start another editor to edit further.");
        }
    }

    /// <summary>
    /// The items of a nested <c>if</c>, <c>while</c>, <c>loop</c> or <c>for</c> body, in the order they are added. The
    /// first statement is written without <c>then</c> and later ones with it; <see cref="Add"/> takes an item as built,
    /// for options the shorthands do not take. A body is read when it is passed to an editor or a factory, so changing
    /// it afterwards changes nothing already built from it.
    /// </summary>
    public sealed class Body
    {
        private readonly Lock gate = new();
        private readonly List<Edit.AddSequence> items = [];

        /// <summary>The items collected so far, in order.</summary>
        public ValueList<Edit.AddSequence> Items
        {
            get
            {
                lock (gate)
                {
                    return new ValueList<Edit.AddSequence>(items);
                }
            }
        }

        /// <summary>Adds an item as built; its owner is not read.</summary>
        /// <param name="item">The item.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="item"/> is null.</exception>
        public Body Add(Edit.AddSequence item)
        {
            ArgumentNullException.ThrowIfNull(item);
            lock (gate)
            {
                items.Add(item);
            }

            return this;
        }

        /// <summary>Adds <c>first ref;</c>.</summary>
        /// <param name="reference">The node the body starts at.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="reference"/> is null.</exception>
        public Body AddFirst(string reference) => Add(Edit.AddSequence.First(string.Empty, reference));

        /// <summary>Adds <c>then ref;</c>.</summary>
        /// <param name="reference">The node sequenced to.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="reference"/> is null.</exception>
        public Body AddThen(string reference) => Add(Edit.AddSequence.Then(string.Empty, reference));

        /// <summary>Adds <c>then kind name;</c>.</summary>
        /// <param name="kind">The member kind, such as <c>action</c>.</param>
        /// <param name="name">The member's name.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public Body AddThenMember(string kind, string name) => Add(Edit.AddSequence.ThenMember(string.Empty, kind, name));

        /// <summary>Adds an <c>action name;</c> node.</summary>
        /// <param name="name">The action's name.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="name"/> is null.</exception>
        public Body AddAction(string name) => Add(Edit.AddSequence.Node(string.Empty, "action", name));

        /// <summary>Adds <c>accept payload;</c>, after <c>then</c> unless it is the first statement.</summary>
        /// <param name="payload">The accepted payload's name.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="payload"/> is null.</exception>
        public Body AddAccept(string payload) => Statement(Edit.AddSequence.Accept(string.Empty, payload));

        /// <summary>Adds <c>send payload to receiver;</c>, after <c>then</c> unless it is the first statement.</summary>
        /// <param name="payload">The payload expression.</param>
        /// <param name="to">The receiver expression; null for none.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="payload"/> is null.</exception>
        public Body AddSend(string payload, string? to = null) => Statement(Edit.AddSequence.Send(string.Empty, payload, to));

        /// <summary>Adds <c>assign target := value;</c>, after <c>then</c> unless it is the first statement.</summary>
        /// <param name="target">The assigned feature.</param>
        /// <param name="value">The assigned expression.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public Body AddAssign(string target, string value) => Statement(Edit.AddSequence.Assign(string.Empty, target, value));

        /// <summary>Adds <c>if condition { body } else { elseBody }</c>, after <c>then</c> unless it is the first statement.</summary>
        /// <param name="condition">The condition.</param>
        /// <param name="body">The then branch, as collected when this is called.</param>
        /// <param name="elseBody">The else branch; null or empty for none.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="condition"/> or <paramref name="body"/> is null.</exception>
        /// <exception cref="ArgumentException">An item is nested too deep.</exception>
        public Body AddIf(string condition, Body body, Body? elseBody = null)
        {
            ArgumentNullException.ThrowIfNull(body);
            return Statement(Edit.AddSequence.If(string.Empty, condition, body.Items, elseBody?.Items));
        }

        /// <summary>Adds <c>while condition { body } until until;</c>, after <c>then</c> unless it is the first statement.</summary>
        /// <param name="condition">The loop condition.</param>
        /// <param name="body">The loop's items, as collected when this is called.</param>
        /// <param name="until">The termination condition; null for none.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="condition"/> or <paramref name="body"/> is null.</exception>
        /// <exception cref="ArgumentException">An item is nested too deep.</exception>
        public Body AddWhile(string condition, Body body, string? until = null)
        {
            ArgumentNullException.ThrowIfNull(body);
            return Statement(Edit.AddSequence.While(string.Empty, condition, body.Items, until));
        }

        /// <summary>Adds <c>loop { body } until until;</c>, after <c>then</c> unless it is the first statement.</summary>
        /// <param name="body">The loop's items, as collected when this is called.</param>
        /// <param name="until">The termination condition; null for none.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="body"/> is null.</exception>
        /// <exception cref="ArgumentException">An item is nested too deep.</exception>
        public Body AddLoop(Body body, string? until = null)
        {
            ArgumentNullException.ThrowIfNull(body);
            return Statement(Edit.AddSequence.Loop(string.Empty, body.Items, until));
        }

        /// <summary>Adds <c>for variable in collection { body }</c>, after <c>then</c> unless it is the first statement.</summary>
        /// <param name="variable">The loop variable.</param>
        /// <param name="collection">The collection expression.</param>
        /// <param name="body">The loop's items, as collected when this is called.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        /// <exception cref="ArgumentException">An item is nested too deep.</exception>
        public Body AddFor(string variable, string collection, Body body)
        {
            ArgumentNullException.ThrowIfNull(body);
            return Statement(Edit.AddSequence.For(string.Empty, variable, collection, body.Items));
        }

        /// <summary>Adds <c>terminate occurrence;</c>, after <c>then</c> unless it is the first statement.</summary>
        /// <param name="occurrence">The terminated occurrence; null for the performing one.</param>
        /// <returns>This body.</returns>
        public Body AddTerminate(string? occurrence = null) => Statement(Edit.AddSequence.Terminate(string.Empty, occurrence));

        /// <summary>Adds <c>if guard then ref;</c>.</summary>
        /// <param name="guard">The guard expression.</param>
        /// <param name="reference">The node sequenced to.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException">An argument is null.</exception>
        public Body AddGuardedThen(string guard, string reference) => Add(Edit.AddSequence.GuardedThen(string.Empty, guard, reference));

        /// <summary>Adds <c>else ref;</c>.</summary>
        /// <param name="reference">The node sequenced to.</param>
        /// <returns>This body.</returns>
        /// <exception cref="ArgumentNullException"><paramref name="reference"/> is null.</exception>
        public Body AddElse(string reference) => Add(Edit.AddSequence.Else(string.Empty, reference));

        // The first statement of a body is written without then, as the notation writes it.
        private Body Statement(Edit.AddSequence item)
        {
            lock (gate)
            {
                items.Add(items.Count == 0 ? item.WithoutThen() : item);
            }

            return this;
        }
    }
}
