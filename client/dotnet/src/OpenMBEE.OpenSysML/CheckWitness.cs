using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One schedule the interpreter replays: the policy that replays it, what the run drew and chose,
/// and the witness file when the check asked for one.
/// </summary>
public sealed record CheckWitness
{
    /// <summary>Creates a witness, copying its lists.</summary>
    /// <param name="schedule">The schedule policy the replay runs under, as <c>-schedule</c> spells it.</param>
    /// <param name="draws">The random values the run drew, in the order drawn.</param>
    /// <param name="choices">The choice points the run resolved, in run order.</param>
    /// <param name="inputs">Free inputs the symbolic engine pinned, as <c>name = value</c>.</param>
    /// <param name="fileName">The witness file's name, as <c>sysml -check-witness</c> writes it; empty when none was asked for.</param>
    /// <param name="fileContent">The witness file's content; empty when none was asked for.</param>
    /// <exception cref="ArgumentNullException">A string or a list is null.</exception>
    /// <exception cref="ArgumentException">A list holds a null.</exception>
    public CheckWitness(
        string schedule,
        IEnumerable<string> draws,
        IEnumerable<string> choices,
        IEnumerable<string> inputs,
        string fileName,
        string fileContent)
    {
        ArgumentNullException.ThrowIfNull(schedule);
        ArgumentNullException.ThrowIfNull(fileName);
        ArgumentNullException.ThrowIfNull(fileContent);
        Schedule = schedule;
        Draws = new ValueList<string>(draws);
        Choices = new ValueList<string>(choices);
        Inputs = new ValueList<string>(inputs);
        FileName = fileName;
        FileContent = fileContent;
    }

    /// <summary>The schedule policy the replay runs under, as <c>-schedule</c> spells it.</summary>
    public string Schedule { get; }

    /// <summary>The random values the run drew, in the order drawn.</summary>
    public ValueList<string> Draws { get; }

    /// <summary>The choice points the run resolved, in run order, each spelling the alternatives and the one taken.</summary>
    public ValueList<string> Choices { get; }

    /// <summary>Free inputs the symbolic engine pinned, as <c>name = value</c>.</summary>
    public ValueList<string> Inputs { get; }

    /// <summary>The witness file's name, as <c>sysml -check-witness</c> writes it; empty unless <see cref="CheckOptions.Witness"/> asked for it.</summary>
    public string FileName { get; }

    /// <summary>The witness file's content, which a replay reads; empty unless <see cref="CheckOptions.Witness"/> asked for it.</summary>
    public string FileContent { get; }
}
