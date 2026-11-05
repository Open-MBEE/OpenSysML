using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string VerifyConstraintMethod = "sysml.SysMLService/VerifyConstraint";
    internal const string VerifyRequirementMethod = "sysml.SysMLService/VerifyRequirement";
    internal const string VerifySatisfactionMethod = "sysml.SysMLService/VerifySatisfaction";
    internal const string ValidateInstanceMethod = "sysml.SysMLService/ValidateInstance";

    /// <summary>Asks whether a constraint holds, as the REPL's <c>%constraint</c> does.</summary>
    /// <param name="symbolId">The qualified name of the constraint definition or usage.</param>
    /// <param name="options">The question, subject, arguments and engine; null for <see cref="VerifyOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The verdict and the objects it is about. A condition the model answers false is a decided verdict, and one that could not be evaluated an undecided one; neither is an exception.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="symbolId"/> is null.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty string, or an argument is a value no service reads as one; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Verification"/>, <see cref="Capabilities.VerificationQuestions"/> for a solver question, <see cref="Capabilities.Engines"/> for an engine, <see cref="Capabilities.ScheduleExplore"/> for the explore engine, <see cref="Capabilities.VerificationArguments"/> for an argument, or a capability an argument needs; nothing was sent.</exception>
    /// <exception cref="VerificationException">The service could not answer at all: the symbol or the subject does not resolve, an argument does not read.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a question or an engine it does not know, or arguments with a solver question.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Verification> VerifyConstraintAsync(string symbolId, VerifyOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(symbolId);
        var request = new Wire.VerifyConstraintRequest { ModelHash = Hash, SymbolId = symbolId };
        var (positional, named) = Verifying(options ??= VerifyOptions.Default);
        request.SubjectSymbolId = options.SubjectSymbolId ?? string.Empty;
        request.Question = options.Question ?? string.Empty;
        request.Engine = options.Engine ?? string.Empty;
        request.Arguments.Add(positional);
        AddInputs(request.NamedArguments, named);
        var response = await Connection.ModelCallAsync<Wire.VerifyConstraintRequest, Wire.VerifyConstraintResponse>(VerifyConstraintMethod, request, cancellationToken).ConfigureAwait(false);
        return Verified(VerifyConstraintMethod, response.Verdict, response.Instances, [], response.Error, response.Diagnostics);
    }

    /// <summary>Asks whether a requirement is satisfied, as the REPL's <c>%requirement</c> does.</summary>
    /// <param name="symbolId">The qualified name of the requirement definition or usage.</param>
    /// <param name="options">The question, subject, arguments and engine; null for <see cref="VerifyOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The verdict, what the verification cases verifying the requirement answered beside it, and the objects it is about.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="symbolId"/> is null.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty string, or an argument is a value no service reads as one; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Verification"/>, <see cref="Capabilities.VerificationQuestions"/> for a solver question, <see cref="Capabilities.Engines"/> for an engine, <see cref="Capabilities.ScheduleExplore"/> for the explore engine, <see cref="Capabilities.VerificationArguments"/> for an argument, or a capability an argument needs; nothing was sent.</exception>
    /// <exception cref="VerificationException">The service could not answer at all: the symbol or the subject does not resolve, an argument does not read.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a question or an engine it does not know, or arguments with a solver question.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Verification> VerifyRequirementAsync(string symbolId, VerifyOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(symbolId);
        var request = new Wire.VerifyRequirementRequest { ModelHash = Hash, SymbolId = symbolId };
        var (positional, named) = Verifying(options ??= VerifyOptions.Default);
        request.SubjectSymbolId = options.SubjectSymbolId ?? string.Empty;
        request.Question = options.Question ?? string.Empty;
        request.Engine = options.Engine ?? string.Empty;
        request.Arguments.Add(positional);
        AddInputs(request.NamedArguments, named);
        var response = await Connection.ModelCallAsync<Wire.VerifyRequirementRequest, Wire.VerifyRequirementResponse>(VerifyRequirementMethod, request, cancellationToken).ConfigureAwait(false);
        return Verified(VerifyRequirementMethod, response.Verdict, response.Instances, response.VerificationVerdicts, response.Error, response.Diagnostics);
    }

    /// <summary>
    /// Asks whether the satisfaction assertions the model states hold, as the REPL's <c>%satisfy</c> does, each against
    /// an object of its own subject built for the call.
    /// </summary>
    /// <param name="scopeSymbolId">
    /// The qualified name limiting the assertions asked about: those stated within that element, or that element when it
    /// is a named satisfy assertion; null asks about every assertion the model states.
    /// </param>
    /// <param name="options">The question and engine; null for <see cref="VerifyOptions.Default"/>. A satisfaction takes no subject and no arguments.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>One verdict per assertion, in declaration order; none for a model stating none.</returns>
    /// <exception cref="InvalidRequestException">The scope or an option names an empty string, or the options name a subject or arguments; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Verification"/>, <see cref="Capabilities.VerificationQuestions"/> for a solver question, <see cref="Capabilities.Engines"/> for an engine, or <see cref="Capabilities.ScheduleExplore"/> for the explore engine; nothing was sent.</exception>
    /// <exception cref="VerificationException">The service could not answer at all: the scope does not resolve, or states no assertion (<see cref="FailureReason.WrongKind"/>).</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a question or an engine it does not know.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Satisfaction> VerifySatisfactionAsync(string? scopeSymbolId = null, VerifyOptions? options = null, CancellationToken cancellationToken = default)
    {
        Connection.ThrowIfDisposed();
        options ??= VerifyOptions.Default;
        if (scopeSymbolId is { Length: 0 })
        {
            throw new InvalidRequestException("The scope is empty; leave it null to ask about every satisfaction assertion of the model.");
        }

        if (options.SubjectSymbolId is not null || options.BindsArguments)
        {
            throw new InvalidRequestException(
                "A satisfaction assertion is asked about an object of its own subject, built for the call, so VerifyOptions.SubjectSymbolId and the arguments must be left unset.");
        }

        Verifying(options);
        var request = new Wire.VerifySatisfactionRequest
        {
            ModelHash = Hash,
            SymbolId = scopeSymbolId ?? string.Empty,
            Engine = options.Engine ?? string.Empty,
            Question = options.Question ?? string.Empty,
        };
        var response = await Connection.ModelCallAsync<Wire.VerifySatisfactionRequest, Wire.VerifySatisfactionResponse>(VerifySatisfactionMethod, request, cancellationToken).ConfigureAwait(false);
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        var instances = VerificationWire.Instances(response.Instances, VerifySatisfactionMethod, "instances");
        var verifications = VerificationWire.Verifications(response.VerificationVerdicts);
        if (response.Error.Length > 0)
        {
            Unanswered(VerifySatisfactionMethod, ("verdicts", response.Verdicts.Count > 0));
            throw new VerificationException(
                response.Error, VerifySatisfactionMethod, diagnostics, VerificationWire.Reason(response.FailureReason), instances, verifications, Standing.None);
        }

        return new Satisfaction(VerificationWire.Verdicts(response.Verdicts, VerifySatisfactionMethod, "verdicts"), verifications, instances, diagnostics);
    }

    /// <summary>
    /// Whether every satisfaction assertion asked about holds, as <see cref="VerifySatisfactionAsync"/> answers: an
    /// undecided one does not, and a model stating none is satisfied, so read <see cref="Satisfaction.Verdicts"/>
    /// where that matters.
    /// </summary>
    /// <param name="scopeSymbolId">The qualified name limiting the assertions asked about; null for every one.</param>
    /// <param name="options">The question and engine; null for <see cref="VerifyOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>Whether every verdict is decided and holds.</returns>
    /// <exception cref="InvalidRequestException">The scope or an option names an empty string, or the options name a subject or arguments; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks a capability the call needs; nothing was sent.</exception>
    /// <exception cref="VerificationException">The service could not answer at all.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<bool> SatisfiedAsync(string? scopeSymbolId = null, VerifyOptions? options = null, CancellationToken cancellationToken = default) =>
        (await VerifySatisfactionAsync(scopeSymbolId, options, cancellationToken).ConfigureAwait(false)).Holds;

    /// <summary>
    /// Validates an object of a part as a whole, as the REPL's <c>%validate</c> does: every assertion about it and about
    /// the objects it holds, each answered on the object it is about, and a summary about the object.
    /// </summary>
    /// <param name="symbolId">The qualified name of the part definition or usage an object of which is validated.</param>
    /// <param name="options">The engine; null for <see cref="ValidateOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>One verdict per assertion, the summary, and every object reached.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="symbolId"/> is null.</exception>
    /// <exception cref="InvalidRequestException">The engine is an empty string; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Verification"/>, <see cref="Capabilities.Engines"/> for an engine, or <see cref="Capabilities.ScheduleExplore"/> for the explore engine; nothing was sent.</exception>
    /// <exception cref="VerificationException">The service could not answer at all: the symbol does not resolve, declares no object (<see cref="FailureReason.WrongKind"/>), or the object could not be validated.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does an engine it does not know.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Validation> ValidateInstanceAsync(string symbolId, ValidateOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(symbolId);
        Connection.ThrowIfDisposed();
        options ??= ValidateOptions.Default;
        RefuseEmptyEngine(options.Engine, nameof(ValidateOptions));
        Connection.Capabilities.Require(Capabilities.Verification);
        Engines.Require(options.Engine, Connection.Capabilities);
        var request = new Wire.ValidateInstanceRequest { ModelHash = Hash, SymbolId = symbolId, Engine = options.Engine ?? string.Empty };
        var response = await Connection.ModelCallAsync<Wire.ValidateInstanceRequest, Wire.ValidateInstanceResponse>(ValidateInstanceMethod, request, cancellationToken).ConfigureAwait(false);
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        var instances = VerificationWire.Instances(response.Instances, ValidateInstanceMethod, "instances");
        var verifications = VerificationWire.Verifications(response.VerificationVerdicts);
        if (response.Error.Length > 0)
        {
            Unanswered(ValidateInstanceMethod, ("summary", response.Summary is not null), ("verdicts", response.Verdicts.Count > 0), ("bounded", response.Bounded));
            throw new VerificationException(
                response.Error, ValidateInstanceMethod, diagnostics, VerificationWire.Reason(response.FailureReason), instances, verifications, Standing.None);
        }

        if (response.Summary is null)
        {
            throw new TransportException(StatusCode.Internal, $"{ValidateInstanceMethod} answered neither a summary nor a failure for {symbolId}.", ValidateInstanceMethod);
        }

        return new Validation(
            VerificationWire.Verdict(response.Summary, ValidateInstanceMethod, "summary"),
            VerificationWire.Verdicts(response.Verdicts, ValidateInstanceMethod, "verdicts"),
            verifications,
            instances,
            diagnostics,
            response.Bounded);
    }

    // Checks a verification's options in the order the service does, and writes its arguments: positional, then by name.
    private (List<Wire.Value> Positional, List<KeyValuePair<string, Wire.Value>> Named) Verifying(VerifyOptions options)
    {
        Connection.ThrowIfDisposed();
        if (options.Question is { Length: 0 } || options.SubjectSymbolId is { Length: 0 } || options.Engine is { Length: 0 })
        {
            throw new InvalidRequestException("VerifyOptions names an empty question, subject or engine; leave Question, SubjectSymbolId or Engine null to name none.");
        }

        var positional = ValueWire.ToWire(new ValueList<Value>(options.Arguments), "arguments");
        var named = ValueWire.ToWire(new ValueDictionary<Value>(options.NamedArguments), "named_arguments");
        var capabilities = Connection.Capabilities;
        capabilities.Require(Capabilities.Verification);
        if (options.Question is VerifyOptions.Holds or VerifyOptions.Satisfiable)
        {
            capabilities.Require(Capabilities.VerificationQuestions);
        }

        Engines.Require(options.Engine, capabilities);
        if (options.BindsArguments)
        {
            capabilities.Require(Capabilities.VerificationArguments);
        }

        Fit(positional);
        Fit(named);
        return (positional, named);
    }

    // One verdict's answer: the failure it reports when it could not be given, else the verdict, which the answer must hold.
    private static Verification Verified(
        string method,
        Wire.Verdict? verdict,
        Google.Protobuf.Collections.RepeatedField<Wire.Instance> instances,
        Google.Protobuf.Collections.RepeatedField<Wire.VerificationVerdict> verifications,
        string error,
        Google.Protobuf.Collections.RepeatedField<Wire.Diagnostic> diagnostics)
    {
        var read = ModelWire.Diagnostics(diagnostics);
        var objects = VerificationWire.Instances(instances, method, "instances");
        var cases = VerificationWire.Verifications(verifications);
        if (error.Length > 0)
        {
            Unanswered(method, ("verdict", verdict is not null));
            throw new VerificationException(error, method, read, FailureReason.Unspecified, objects, cases, Standing.None);
        }

        if (verdict is null)
        {
            throw new TransportException(StatusCode.Internal, $"{method} answered neither a verdict nor a failure.", method);
        }

        return new Verification(VerificationWire.Verdict(verdict, method, "verdict"), cases, objects, read);
    }

    private static void RefuseEmptyEngine(string? engine, string options)
    {
        if (engine is { Length: 0 })
        {
            throw new InvalidRequestException($"{options} names an empty engine; leave Engine null to let the service choose.");
        }
    }

    // An answer reporting that nothing could be answered carries none of the answer's own fields; one that does is malformed.
    private static void Unanswered(string method, params (string Field, bool Set)[] answer)
    {
        foreach (var (field, set) in answer)
        {
            if (set)
            {
                throw new TransportException(StatusCode.Internal, $"{method} answered a failure beside {field}, which only an answer reports.", method);
            }
        }
    }

    private void Fit(List<Wire.Value> sent)
    {
        foreach (var value in sent)
        {
            ValueWire.Fit(value, Connection.Capabilities);
        }
    }
}
