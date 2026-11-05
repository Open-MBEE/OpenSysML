using System;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string GetDiagnosticsMethod = "sysml.SysMLService/GetDiagnostics";
    internal const string EvaluateMethod = "sysml.SysMLService/Evaluate";
    internal const string InstantiateMethod = "sysml.SysMLService/Instantiate";

    /// <summary>Asks the service for the model's diagnostics, under a lint selection.</summary>
    /// <param name="options">The lints left out and the opt-in lints kept; null for <see cref="DiagnosticsOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The diagnostics, in report order.</returns>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ModelException">The service reported a failure in its answer.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a code that names no lint.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<ValueList<Diagnostic>> GetDiagnosticsAsync(DiagnosticsOptions? options = null, CancellationToken cancellationToken = default)
    {
        Connection.ThrowIfDisposed();
        options ??= DiagnosticsOptions.Default;
        var request = new Wire.DiagnosticsRequest { ModelHash = Hash, DisableLint = { options.DisableLint }, EnableLint = { options.EnableLint } };
        var response = await Connection.ModelCallAsync<Wire.DiagnosticsRequest, Wire.DiagnosticsResponse>(GetDiagnosticsMethod, request, cancellationToken).ConfigureAwait(false);
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        if (response.Error.Length > 0)
        {
            throw new ModelException(response.Error, GetDiagnosticsMethod, diagnostics);
        }

        return diagnostics;
    }

    /// <summary>Evaluates an expression against the model's declarations.</summary>
    /// <param name="expression">A SysML expression, such as <c>2 + 2</c>.</param>
    /// <param name="options">Where to evaluate it; null for <see cref="EvalOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>What it evaluated to.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="expression"/> is null.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty symbol; nothing was sent.</exception>
    /// <exception cref="CapabilityException">A subject is named and the service lacks <see cref="Capabilities.EvaluateSubject"/>; nothing was sent.</exception>
    /// <exception cref="ModelException">The expression did not parse or evaluate, or the subject is unknown or could not be built.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">The value is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Value> EvalAsync(string expression, EvalOptions? options = null, CancellationToken cancellationToken = default) =>
        (await EvaluateAsync(expression, options, cancellationToken).ConfigureAwait(false)).Result;

    /// <summary>Evaluates an expression against the model's declarations, keeping what the service reported with the value.</summary>
    /// <param name="expression">A SysML expression, such as <c>2 + 2</c>.</param>
    /// <param name="options">Where to evaluate it; null for <see cref="EvalOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The value and the diagnostics reported with it.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="expression"/> is null.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty symbol; nothing was sent.</exception>
    /// <exception cref="CapabilityException">A subject is named and the service lacks <see cref="Capabilities.EvaluateSubject"/>; nothing was sent.</exception>
    /// <exception cref="ModelException">The expression did not parse or evaluate, or the subject is unknown or could not be built.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">The value is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Evaluation> EvaluateAsync(string expression, EvalOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(expression);
        Connection.ThrowIfDisposed();
        options ??= EvalOptions.Default;
        if (options.ContextSymbolId is { Length: 0 } || options.SubjectSymbolId is { Length: 0 })
        {
            throw new InvalidRequestException("EvalOptions names an empty symbol; leave ContextSymbolId or SubjectSymbolId null to name none.");
        }

        if (options.SubjectSymbolId is not null)
        {
            Connection.Capabilities.Require(Capabilities.EvaluateSubject);
        }

        var request = new Wire.EvaluateRequest
        {
            ModelHash = Hash,
            Expression = expression,
            ContextSymbolId = options.ContextSymbolId ?? string.Empty,
            SubjectSymbolId = options.SubjectSymbolId ?? string.Empty,
        };
        var response = await Connection.ModelCallAsync<Wire.EvaluateRequest, Wire.EvaluateResponse>(EvaluateMethod, request, cancellationToken).ConfigureAwait(false);
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        if (response.Error.Length > 0)
        {
            throw new ModelException(response.Error, EvaluateMethod, diagnostics);
        }

        if (response.Result is null)
        {
            throw new TransportException(StatusCode.Internal, $"{EvaluateMethod} answered neither a value nor a failure for {expression}.", EvaluateMethod);
        }

        return new Evaluation(ValueWire.FromWire(response.Result, EvaluateMethod, "result"), diagnostics);
    }

    /// <summary>Builds an object of a part definition or usage, and every object reachable from it.</summary>
    /// <param name="symbolId">The qualified name of the definition or usage.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The object built, and those reachable from it.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="symbolId"/> is null.</exception>
    /// <exception cref="ModelException">The symbol is unknown, or the object could not be built.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A feature value is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does past the objects a model may hold.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Instantiation> InstantiateAsync(string symbolId, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(symbolId);
        Connection.ThrowIfDisposed();
        var request = new Wire.InstantiateRequest { ModelHash = Hash, SymbolId = symbolId };
        var response = await Connection.ModelCallAsync<Wire.InstantiateRequest, Wire.InstantiateResponse>(InstantiateMethod, request, cancellationToken).ConfigureAwait(false);
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        if (response.Error.Length > 0)
        {
            throw new ModelException(response.Error, InstantiateMethod, diagnostics);
        }

        if (response.Instance is null)
        {
            throw new TransportException(StatusCode.Internal, $"{InstantiateMethod} answered neither an instance nor a failure for {symbolId}.", InstantiateMethod);
        }

        var root = ModelWire.Instance(response.Instance, InstantiateMethod, "instance");
        var reachable = new Instance[response.Instances.Count];
        for (var i = 0; i < reachable.Length; i++)
        {
            reachable[i] = ModelWire.Instance(response.Instances[i], InstantiateMethod, $"instances[{i}]");
        }

        return new Instantiation(root, reachable, diagnostics);
    }
}
