using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// Which lints <see cref="Model.GetDiagnosticsAsync"/> keeps, as <c>sysml -disable-lint</c> and
/// <c>-enable-lint</c> select them. Lists are copied when they are set. A code that names no lint
/// is refused by the service with <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class DiagnosticsOptions
{
    private readonly ValueList<string> disableLint;
    private readonly ValueList<string> enableLint;

    /// <summary>The service's default selection: every lint on by default, no opt-in lint.</summary>
    public static DiagnosticsOptions Default { get; } = new();

    /// <summary>Codes of lints left out of the answer; this wins over <see cref="EnableLint"/>.</summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A code is null.</exception>
    public IReadOnlyList<string> DisableLint
    {
        get => disableLint;
        init => disableLint = new ValueList<string>(value);
    }

    /// <summary>Codes of opt-in lints kept in the answer.</summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A code is null.</exception>
    public IReadOnlyList<string> EnableLint
    {
        get => enableLint;
        init => enableLint = new ValueList<string>(value);
    }
}
