using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One record of a run's trace, in the order the run made it: what an action or state run returns when it is
/// asked for its trace, and the row an <c>Events</c> document query answers.
/// </summary>
public sealed record DocumentEvent
{
    /// <summary>Creates a trace record, copying its lists.</summary>
    /// <param name="kind">What the record is: <c>accept</c>, <c>send</c>, <c>transition</c>, <c>entry</c>, <c>exit</c>, <c>do</c>, <c>choice</c>, <c>guard</c> or <c>terminate</c>.</param>
    /// <param name="time">The clock's instant when the record was made; null when the service reported none.</param>
    /// <param name="subject">The object whose behavior made the record; null for the run's own records.</param>
    /// <param name="machine">The behavior the record is about; empty for the run's own records.</param>
    /// <param name="state">The state entered, exited or stepped by an entry, exit or do record.</param>
    /// <param name="from">A fired transition's source state.</param>
    /// <param name="to">A fired transition's target state.</param>
    /// <param name="target">The object a send was addressed to; null otherwise.</param>
    /// <param name="signal">The signal or event accepted or sent, or the trigger a transition fired on.</param>
    /// <param name="payload">An accept's payload, one <c>name = value</c> entry per parameter in name order.</param>
    /// <param name="alternatives">A choice's alternatives, in the order they were offered.</param>
    /// <param name="taken">The alternative a choice took.</param>
    /// <param name="text">The line the trace prints for the record.</param>
    /// <exception cref="ArgumentNullException">A string or a list is null.</exception>
    /// <exception cref="ArgumentException">A payload entry or an alternative is null.</exception>
    public DocumentEvent(
        string kind,
        DocumentValue? time,
        DocumentObject? subject,
        string machine,
        string state,
        string from,
        string to,
        DocumentObject? target,
        string signal,
        IEnumerable<string> payload,
        IEnumerable<string> alternatives,
        string taken,
        string text)
        : this(kind, time, subject, machine, state, from, to, target, signal, payload, alternatives, taken, text, 0, 0, string.Empty, null)
    {
    }

    /// <summary>Creates a trace record with its action-token and assignment facts, copying its lists.</summary>
    /// <param name="kind">The kind of trace record.</param>
    /// <param name="time">The clock's instant; null when none was reported.</param>
    /// <param name="subject">The object whose behavior made the record; null for the run's own records.</param>
    /// <param name="machine">The behavior the record is about.</param>
    /// <param name="state">The state entered, exited or stepped.</param>
    /// <param name="from">A fired transition's source.</param>
    /// <param name="to">A fired transition's target.</param>
    /// <param name="target">The object a send was addressed to; null otherwise.</param>
    /// <param name="signal">The signal or event.</param>
    /// <param name="payload">The accepted payload.</param>
    /// <param name="alternatives">A choice's alternatives, in order.</param>
    /// <param name="taken">The alternative taken.</param>
    /// <param name="text">The line the trace prints.</param>
    /// <param name="token">The action token's identity; 0 when none was reported.</param>
    /// <param name="step">The action graph's 1-based step index; 0 for a record of neither an action token nor a step.</param>
    /// <param name="feature">The feature assigned by an assignment record; empty otherwise.</param>
    /// <param name="value">The assigned value; null when none was reported.</param>
    /// <exception cref="ArgumentNullException">A string or list is null.</exception>
    /// <exception cref="ArgumentException">A payload entry or alternative is null.</exception>
    public DocumentEvent(
        string kind,
        DocumentValue? time,
        DocumentObject? subject,
        string machine,
        string state,
        string from,
        string to,
        DocumentObject? target,
        string signal,
        IEnumerable<string> payload,
        IEnumerable<string> alternatives,
        string taken,
        string text,
        long token,
        int step,
        string feature,
        Value? value)
    {
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(machine);
        ArgumentNullException.ThrowIfNull(state);
        ArgumentNullException.ThrowIfNull(from);
        ArgumentNullException.ThrowIfNull(to);
        ArgumentNullException.ThrowIfNull(signal);
        ArgumentNullException.ThrowIfNull(taken);
        ArgumentNullException.ThrowIfNull(text);
        ArgumentNullException.ThrowIfNull(feature);
        Kind = kind;
        Time = time;
        Subject = subject;
        Machine = machine;
        State = state;
        From = from;
        To = to;
        Target = target;
        Event = signal;
        Payload = new ValueList<string>(payload);
        Alternatives = new ValueList<string>(alternatives);
        Taken = taken;
        Text = text;
        Token = token;
        Step = step;
        Feature = feature;
        Value = value;
    }

    /// <summary>What the record is: <c>accept</c>, <c>send</c>, <c>transition</c>, <c>entry</c>, <c>exit</c>, <c>do</c>, <c>choice</c>, <c>guard</c> or <c>terminate</c>.</summary>
    public string Kind { get; }

    /// <summary>
    /// The clock's instant when the record was made: a <see cref="DocumentValue.QuantityValue"/> in the
    /// clock's unit when the library reduces one, else a <see cref="DocumentValue.NumberValue"/> of clock
    /// units; null when the service reported none.
    /// </summary>
    public DocumentValue? Time { get; }

    /// <summary>The object whose behavior made the record; null for the run's own records, such as a due-order choice.</summary>
    public DocumentObject? Subject { get; }

    /// <summary>The behavior the record is about, by the name the object exhibits it under, else its declared name; empty for the run's own records.</summary>
    public string Machine { get; }

    /// <summary>The state entered, exited or stepped by an entry, exit or do record; empty otherwise.</summary>
    public string State { get; }

    /// <summary>A fired transition's source state; empty otherwise.</summary>
    public string From { get; }

    /// <summary>A fired transition's target state; empty otherwise.</summary>
    public string To { get; }

    /// <summary>The object a send was addressed to; null for every other kind and for a send addressed to no object.</summary>
    public DocumentObject? Target { get; }

    /// <summary>The signal or event accepted or sent, or the trigger a transition fired on; empty otherwise.</summary>
    public string Event { get; }

    /// <summary>An accept's payload, one <c>name = value</c> entry per parameter in name order.</summary>
    public ValueList<string> Payload { get; }

    /// <summary>A choice's alternatives, in the order they were offered.</summary>
    public ValueList<string> Alternatives { get; }

    /// <summary>The alternative a choice took; empty otherwise.</summary>
    public string Taken { get; }

    /// <summary>The line the trace prints for the record.</summary>
    public string Text { get; }

    /// <summary>The identity of the action token; 0 when none was reported.</summary>
    public long Token { get; }

    /// <summary>The action graph's 1-based step index; 0 for a record of neither an action token nor a step.</summary>
    public int Step { get; }

    /// <summary>The feature assigned by an assignment record; empty otherwise.</summary>
    public string Feature { get; }

    /// <summary>The assigned value; null when none was reported.</summary>
    public Value? Value { get; }

    /// <summary>The record as the trace prints it, <see cref="Text"/>.</summary>
    /// <returns>The record's line.</returns>
    public override string ToString() => Text;
}
