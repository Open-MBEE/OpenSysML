using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One application an analysis case made of one of its own calcs held as a function value: a trade study's
/// evaluation function applied to one alternative. A service without <see cref="Capabilities.CaseEvaluations"/> reports none.
/// </summary>
public sealed record CaseEvaluation
{
    /// <summary>Creates an evaluation, copying its arguments.</summary>
    /// <param name="functionId">The qualified name of the calc applied.</param>
    /// <param name="arguments">What it was applied to, in parameter order.</param>
    /// <param name="result">What it computed; null when <paramref name="error"/> says why it computed nothing.</param>
    /// <param name="error">Why it computed nothing; empty when it computed a result.</param>
    /// <param name="selected">Whether this is the evaluation whose argument the case selected and returned.</param>
    /// <param name="tied">Whether it computed what the selected one did without being selected.</param>
    /// <exception cref="ArgumentNullException"><paramref name="functionId"/>, <paramref name="arguments"/> or <paramref name="error"/> is null.</exception>
    /// <exception cref="ArgumentException">An argument is null.</exception>
    public CaseEvaluation(string functionId, IEnumerable<Value> arguments, Value? result, string error, bool selected, bool tied)
    {
        ArgumentNullException.ThrowIfNull(functionId);
        ArgumentNullException.ThrowIfNull(error);
        FunctionId = functionId;
        Arguments = new ValueList<Value>(arguments);
        Result = result;
        Error = error;
        Selected = selected;
        Tied = tied;
    }

    /// <summary>The qualified name of the calc applied.</summary>
    public string FunctionId { get; }

    /// <summary>What it was applied to, in parameter order; an alternative is a <see cref="Value.InstanceRefValue"/>.</summary>
    public ValueList<Value> Arguments { get; }

    /// <summary>What it computed; null when <see cref="Error"/> says why it computed nothing.</summary>
    public Value? Result { get; }

    /// <summary>Why it computed nothing; empty when it computed a result.</summary>
    public string Error { get; }

    /// <summary>
    /// Whether this is the evaluation whose argument <c>selectOne</c> picked and the case returned: the alternative
    /// a trade study selected. A result that merely equals an argument selects nothing.
    /// </summary>
    public bool Selected { get; }

    /// <summary>Whether it computed what the selected one did without being selected, an alternative passed over for an earlier one evaluating alike.</summary>
    public bool Tied { get; }
}
