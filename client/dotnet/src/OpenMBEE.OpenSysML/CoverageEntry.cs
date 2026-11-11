using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One state entered or one transition taken, and how many runs did so.</summary>
public sealed record CoverageEntry
{
    /// <summary>Creates an entry.</summary>
    /// <param name="path">The state or transition qualified by the instance running the machine.</param>
    /// <param name="declaration">The state or transition qualified by its declaration.</param>
    /// <param name="runs">How many runs, or schedules under the check engine, entered or took it.</param>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> or <paramref name="declaration"/> is null.</exception>
    internal CoverageEntry(string path, string declaration, int runs)
    {
        ArgumentNullException.ThrowIfNull(path);
        ArgumentNullException.ThrowIfNull(declaration);
        Path = path;
        Declaration = declaration;
        Runs = runs;
    }

    /// <summary>
    /// The state or transition qualified by the instance running the machine: <c>Cov::Sys#1.m.run: viaA</c>,
    /// <c>Cov::Sys#1.m.run: s0 -> viaB</c>.
    /// </summary>
    public string Path { get; }

    /// <summary>The state or transition qualified by its declaration: <c>Cov::M::run::viaA</c>, <c>Cov::M::run::s0 -> viaB</c>.</summary>
    public string Declaration { get; }

    /// <summary>How many runs, or schedules under the check engine, entered or took it.</summary>
    public int Runs { get; }
}
