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
    internal const string InstancePathsMethod = "sysml.SysMLService/InstancePaths";

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

    /// <summary>
    /// Every root-relative usage path that reaches the object of an id, from each object
    /// <see cref="InstantiateAsync"/> created in this model: the paths by which a client can address it, and so the
    /// parts that share it, aliases through connector ends and reference usages included. An object's
    /// <see cref="Instance.ReachedBy"/> is one of them.
    /// </summary>
    /// <param name="instanceId">The object's id, as an instantiation answered it (<see cref="Instance.Id"/>); ids count up from 1.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The paths in traversal order, and whether the service bounded its search.</returns>
    /// <exception cref="ArgumentOutOfRangeException"><paramref name="instanceId"/> is not positive; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.InstancePaths"/>; nothing was sent.</exception>
    /// <exception cref="ModelException">The service could not list the paths.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused the call, with <see cref="StatusCode.NotFound"/> when the model's population holds no object of the id.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public Task<InstancePaths> InstancePathsAsync(long instanceId, CancellationToken cancellationToken = default)
    {
        ArgumentOutOfRangeException.ThrowIfNegativeOrZero(instanceId);
        return ReadInstancePathsAsync(new Wire.DocumentObject { InstanceId = instanceId }, cancellationToken);
    }

    /// <summary>
    /// Every root-relative usage path that reaches the object at an object path, as <see cref="InstancePathsAsync"/>
    /// answers for an id.
    /// </summary>
    /// <param name="objectPath">The qualified name the object was instantiated as (<c>Garage::car</c>), its id (<c>#2</c>), or a path through feature values from either (<c>Garage::car.wheels[2]</c>; indexes count from 1), as <see cref="InstancePath.ToString"/> writes one.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The paths in traversal order, and whether the service bounded its search.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="objectPath"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="objectPath"/> is empty; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.InstancePaths"/>; nothing was sent.</exception>
    /// <exception cref="ModelException">The service could not list the paths.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">
    /// The service refused the call: <see cref="StatusCode.NotFound"/> when the model's population holds no object of
    /// the name or id, <see cref="StatusCode.InvalidArgument"/> when the path does not read or reaches no object.
    /// </exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public Task<InstancePaths> InstancePathsAtAsync(string objectPath, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(objectPath);
        return ReadInstancePathsAsync(new Wire.DocumentObject { Path = objectPath }, cancellationToken);
    }

    private async Task<InstancePaths> ReadInstancePathsAsync(Wire.DocumentObject target, CancellationToken cancellationToken)
    {
        Connection.ThrowIfDisposed();
        Connection.Capabilities.Require(Capabilities.InstancePaths);
        var request = new Wire.InstancePathsRequest { ModelHash = Hash, Object = target };
        var response = await DeclarationCallAsync<Wire.InstancePathsRequest, Wire.InstancePathsResponse>(InstancePathsMethod, request, modelFaults: false, cancellationToken).ConfigureAwait(false);
        if (response.Error.Length > 0)
        {
            throw new ModelException(response.Error, InstancePathsMethod, ModelWire.Diagnostics(response.Diagnostics));
        }

        return ModelWire.InstancePaths(response, InstancePathsMethod);
    }
}
