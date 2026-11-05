using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One analysis engine the service registers, as <see cref="Connection.ListEnginesAsync"/> lists it: what it
/// answers, the strongest evidence it can produce, whether it can run here, and, for one a manifest registered,
/// where it comes from and whether the service serves it.
/// </summary>
public sealed record EngineInfo
{
    /// <summary>Creates an engine listing, copying its collections.</summary>
    /// <param name="name">The engine's name, as an options' <c>Engine</c> selects it.</param>
    /// <param name="authority">The strongest evidence it can produce, spelled as <see cref="Standing.Strength"/> is.</param>
    /// <param name="answers">The kinds of question it answers.</param>
    /// <param name="bounds">The bounds it takes, in the order it reports them.</param>
    /// <param name="process">The external process it needs; empty for one running in the service.</param>
    /// <param name="processFound">Where that process was found; empty when it was not.</param>
    /// <param name="ready">Whether it can run.</param>
    /// <param name="unavailable">Why it cannot run; empty when it can.</param>
    /// <param name="kind">Where it comes from: <c>built-in</c>, or the kind of manifest entry that registered it.</param>
    /// <param name="protocol">How it is spoken to.</param>
    /// <param name="source">The manifest entry it was registered from.</param>
    /// <param name="command">The command that entry resolved to.</param>
    /// <param name="version">The version that entry declares.</param>
    /// <param name="served">Whether the service runs it for a request that reaches it.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public EngineInfo(
        string name,
        string authority,
        IEnumerable<string> answers,
        IEnumerable<string> bounds,
        string process,
        string processFound,
        bool ready,
        string unavailable,
        string kind,
        string protocol,
        string source,
        string command,
        string version,
        bool served)
    {
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(authority);
        ArgumentNullException.ThrowIfNull(process);
        ArgumentNullException.ThrowIfNull(processFound);
        ArgumentNullException.ThrowIfNull(unavailable);
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(protocol);
        ArgumentNullException.ThrowIfNull(source);
        ArgumentNullException.ThrowIfNull(command);
        ArgumentNullException.ThrowIfNull(version);
        Name = name;
        Authority = authority;
        Answers = new ValueList<string>(answers);
        Bounds = new ValueList<string>(bounds);
        Process = process;
        ProcessFound = processFound;
        Ready = ready;
        Unavailable = unavailable;
        Kind = kind;
        Protocol = protocol;
        Source = source;
        Command = command;
        Version = version;
        Served = served;
    }

    /// <summary>The engine's name, as an options' <c>Engine</c> selects it (<c>run</c>, <c>solve</c>, <c>explore</c>, <c>sweep</c>).</summary>
    public string Name { get; }

    /// <summary>The strongest evidence the engine can ever produce, spelled as <see cref="Standing.Strength"/> is.</summary>
    public string Authority { get; }

    /// <summary>The kinds of question it answers: <c>evaluate</c>, <c>outcomes</c>, <c>holds</c>, <c>sensitive</c>, <c>satisfiable</c>, <c>sweep</c>, <c>compute</c>.</summary>
    public ValueList<string> Answers { get; }

    /// <summary>The bounds it takes, in the order it reports them.</summary>
    public ValueList<string> Bounds { get; }

    /// <summary>The external process it needs; empty for one running in the service.</summary>
    public string Process { get; }

    /// <summary>Where <see cref="Process"/> was found; empty when it was not, or none is needed.</summary>
    public string ProcessFound { get; }

    /// <summary>Whether the engine can run: it is served, and it needs no process or its process was found.</summary>
    public bool Ready { get; }

    /// <summary>Why the engine cannot run; empty when it can. After a probe, the outcome of its handshake.</summary>
    public string Unavailable { get; }

    /// <summary>Where it comes from: <c>built-in</c>, or the kind of manifest entry that registered it: <c>tool</c>, <c>engine</c>, <c>policy</c> or <c>sampler</c>.</summary>
    public string Kind { get; }

    /// <summary>How it is spoken to: <c>-</c> for one built in, a tool's exchange and reply format, or an engine entry's <c>&lt;transport&gt;/&lt;protocol&gt;</c>.</summary>
    public string Protocol { get; }

    /// <summary>The manifest entry it was registered from; empty for a built-in engine.</summary>
    public string Source { get; }

    /// <summary>The command that entry resolved to; empty for a built-in engine.</summary>
    public string Command { get; }

    /// <summary>The version that entry declares; empty for a built-in engine.</summary>
    public string Version { get; }

    /// <summary>
    /// Whether the service runs the engine for a request that reaches it; a manifest engine is listed but served only
    /// when the service was started to serve it, and a request naming one it does not serve is refused.
    /// </summary>
    public bool Served { get; }
}
