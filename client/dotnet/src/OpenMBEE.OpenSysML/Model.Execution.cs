using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string ExecuteActionMethod = "sysml.SysMLService/ExecuteAction";
    internal const string ExecuteStateMethod = "sysml.SysMLService/ExecuteState";
    internal const string CheckBehaviorMethod = "sysml.SysMLService/CheckBehavior";

    /// <summary>Runs an action once, under one schedule, as <c>sysml -action</c> does.</summary>
    /// <param name="actionSymbolId">The qualified name of the action definition or usage.</param>
    /// <param name="inputs">Values bound to the action's parameters before it runs, by name; null for none.</param>
    /// <param name="options">The schedule, the performer, the trace, the events posted to the performer and the run's clock; null for <see cref="RunOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The run's outputs, the performer's attributes, the trace when asked for, and what the service reported about the run.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="actionSymbolId"/> is null.</exception>
    /// <exception cref="ArgumentException">An input or an event is null. An option names an empty string, the schedule explores, which <see cref="ExploreActionAsync"/> answers, events are posted with no performer to post them to, or an input is a value no service reads as an argument; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Schedule"/> for a schedule, <see cref="Capabilities.ActionTrace"/> for a trace, <see cref="Capabilities.Performer"/> for a performer, or a capability an input needs; nothing was sent.</exception>
    /// <exception cref="ExecutionException">The action could not be run, or did not complete within its advance; a traced run's failure keeps its trace up to the failure.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a schedule or draw policy it cannot read.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<ActionRun> ExecuteActionAsync(
        string actionSymbolId,
        IReadOnlyDictionary<string, Value>? inputs = null,
        RunOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        var response = await ActionAsync(actionSymbolId, inputs, options, explore: false, cancellationToken).ConfigureAwait(false);
        return ExecutionWire.Run(response, ExecuteActionMethod);
    }

    /// <summary>
    /// Runs an action once per order of its choice points the library admits, within a budget, as
    /// <c>sysml -schedule explore</c> does; a run failing under some order is an outcome of its own.
    /// </summary>
    /// <param name="actionSymbolId">The qualified name of the action definition or usage.</param>
    /// <param name="inputs">Values bound to the action's parameters before each run, by name; null for none.</param>
    /// <param name="options">The exploring schedule, <see cref="RunOptions.Explore"/> when null, and the performer, made anew for each run; null for <see cref="RunOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>Every distinct outcome reached, and how the exploration ended.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="actionSymbolId"/> is null.</exception>
    /// <exception cref="ArgumentException">An input or an event is null. An option names an empty string, the schedule runs once, which <see cref="ExecuteActionAsync"/> answers, a trace or events are asked for, which only one run has, or an input is a value no service reads as an argument; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Schedule"/> or <see cref="Capabilities.ScheduleExplore"/>, <see cref="Capabilities.Performer"/> for a performer, or a capability an input needs; nothing was sent.</exception>
    /// <exception cref="ExecutionException">The action could not be explored at all.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a budget it cannot read.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Exploration> ExploreActionAsync(
        string actionSymbolId,
        IReadOnlyDictionary<string, Value>? inputs = null,
        RunOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        var response = await ActionAsync(actionSymbolId, inputs, options, explore: true, cancellationToken).ConfigureAwait(false);
        return ExecutionWire.Exploration(response.Outcomes, response.Exploration, Standing.None, ExecuteActionMethod);
    }

    /// <summary>Runs a state machine once, under one schedule, as <c>sysml -state</c> does.</summary>
    /// <param name="stateMachineSymbolId">The qualified name of the state definition or usage.</param>
    /// <param name="events">The names of the events sent to it, in order; null for none.</param>
    /// <param name="options">The schedule, the performer, the trace and the run's clock; null for <see cref="RunOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The states visited, the final context, the trace when asked for, and what the service reported about the run.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="stateMachineSymbolId"/> is null.</exception>
    /// <exception cref="ArgumentException">An event is null. An option names an empty string, the schedule explores, which <see cref="ExploreStateAsync"/> answers, or <see cref="RunOptions.Events"/> is set, where a state run takes <paramref name="events"/>; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Schedule"/> for a schedule, <see cref="Capabilities.StateTrace"/> for a trace, or <see cref="Capabilities.Performer"/> for a performer; nothing was sent.</exception>
    /// <exception cref="ExecutionException">The machine could not be run; a traced run's failure keeps its trace up to the failure.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a schedule or draw policy it cannot read.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<StateRun> ExecuteStateAsync(
        string stateMachineSymbolId,
        IEnumerable<string>? events = null,
        RunOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        var response = await StateAsync(stateMachineSymbolId, events, options, explore: false, cancellationToken).ConfigureAwait(false);
        return ExecutionWire.Run(response, ExecuteStateMethod);
    }

    /// <summary>
    /// Runs a state machine once per order of its choice points the library admits, within a budget, as
    /// <c>sysml -schedule explore</c> does; a run failing under some order is an outcome of its own.
    /// </summary>
    /// <param name="stateMachineSymbolId">The qualified name of the state definition or usage.</param>
    /// <param name="events">The names of the events sent to each run, in order; null for none.</param>
    /// <param name="options">The exploring schedule, <see cref="RunOptions.Explore"/> when null, and the performer, made anew for each run; null for <see cref="RunOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>Every distinct outcome reached, and how the exploration ended.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="stateMachineSymbolId"/> is null.</exception>
    /// <exception cref="ArgumentException">An event is null. An option names an empty string, the schedule runs once, which <see cref="ExecuteStateAsync"/> answers, a trace is asked for, which an exploration has none of, or <see cref="RunOptions.Events"/> is set, where a state machine takes <paramref name="events"/>; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Schedule"/> or <see cref="Capabilities.ScheduleExplore"/>, or <see cref="Capabilities.Performer"/> for a performer; nothing was sent.</exception>
    /// <exception cref="ExecutionException">The machine could not be explored at all.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a budget it cannot read.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Exploration> ExploreStateAsync(
        string stateMachineSymbolId,
        IEnumerable<string>? events = null,
        RunOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        var response = await StateAsync(stateMachineSymbolId, events, options, explore: true, cancellationToken).ConfigureAwait(false);
        return ExecutionWire.Exploration(response.Outcomes, response.Exploration, Standing.None, ExecuteStateMethod);
    }

    /// <summary>
    /// Searches every schedule of an action for a violated property, a deadlock, a failure or a divergent
    /// feature, as <c>sysml -action … -check-property</c> does; what it finds is witnessed and replayed.
    /// </summary>
    /// <param name="actionSymbolId">The qualified name of the action definition or usage.</param>
    /// <param name="inputs">Values bound to the action's parameters before the search, by name; null for none.</param>
    /// <param name="options">What to look for, the bounds and the engine; null for <see cref="CheckOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The verdict, with every violation, divergence and final outcome witnessed; a failing property is a verdict, not an exception.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="actionSymbolId"/> is null.</exception>
    /// <exception cref="ArgumentException">An input is null. An option names an empty string, or an input is a value no service reads as an argument; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Check"/>, <see cref="Capabilities.Performer"/> for a performer, <see cref="Capabilities.Engines"/> for an engine, <see cref="Capabilities.ScheduleExplore"/> for the explore engine, or a capability an input needs; nothing was sent.</exception>
    /// <exception cref="CheckException">The service refused the check in its answer: the behavior, a property or the performer does not resolve.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a negative bound or an engine it does not know.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public Task<CheckResult> CheckActionAsync(
        string actionSymbolId,
        IReadOnlyDictionary<string, Value>? inputs = null,
        CheckOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(actionSymbolId);
        return CheckAsync(new Wire.CheckBehaviorRequest { ActionSymbolId = actionSymbolId }, inputs, options, cancellationToken);
    }

    /// <summary>
    /// Searches every schedule of a state machine for a violated property, a deadlock, a failure or a
    /// divergent feature, as <c>sysml -state … -check-property</c> does; see <see cref="CheckActionAsync"/>.
    /// </summary>
    /// <param name="stateMachineSymbolId">The qualified name of the state definition or usage.</param>
    /// <param name="options">What to look for, the bounds and the engine; null for <see cref="CheckOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The verdict, with every violation, divergence and final outcome witnessed.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="stateMachineSymbolId"/> is null.</exception>
    /// <exception cref="ArgumentException">An option names an empty string; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Check"/>, <see cref="Capabilities.Performer"/> for a performer, <see cref="Capabilities.Engines"/> for an engine, or <see cref="Capabilities.ScheduleExplore"/> for the explore engine; nothing was sent.</exception>
    /// <exception cref="CheckException">The service refused the check in its answer: the behavior, a property or the performer does not resolve.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a negative bound or an engine it does not know.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public Task<CheckResult> CheckStateAsync(string stateMachineSymbolId, CheckOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(stateMachineSymbolId);
        return CheckAsync(new Wire.CheckBehaviorRequest { StateMachineSymbolId = stateMachineSymbolId }, null, options, cancellationToken);
    }

    private async Task<Wire.ExecuteActionResponse> ActionAsync(
        string actionSymbolId,
        IReadOnlyDictionary<string, Value>? inputs,
        RunOptions? options,
        bool explore,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(actionSymbolId);
        Connection.ThrowIfDisposed();
        options ??= RunOptions.Default;
        var schedule = Schedule(options, explore, explore ? nameof(ExecuteActionAsync) : nameof(ExploreActionAsync));
        if (options.Trace && explore)
        {
            throw new ArgumentException("RunOptions.Trace asks for one run's trace, which an exploration describes none of; leave it false.");
        }

        var events = new ValueList<string>(options.Events ?? []);
        if (events.Count > 0 && explore)
        {
            throw new ArgumentException("RunOptions.Events posts to one run, which an exploration does not make; leave it null.");
        }

        if (events.Count > 0 && options.PerformerSymbolId is null)
        {
            throw new ArgumentException("RunOptions.Events are posted to the object performing the action; name one in PerformerSymbolId.");
        }

        var sent = Inputs(inputs);
        RequireRun(schedule, options, Capabilities.ActionTrace);
        Fit(sent);
        var request = new Wire.ExecuteActionRequest
        {
            ModelHash = Hash,
            ActionSymbolId = actionSymbolId,
            Schedule = schedule ?? string.Empty,
            PerformerSymbolId = options.PerformerSymbolId ?? string.Empty,
            Draws = options.Draws ?? string.Empty,
            ClockStep = options.ClockStep,
            Trace = options.Trace,
            Events = { events },
        };
        AddInputs(request.Inputs, sent);
        if (options.Advance is { } advance)
        {
            request.Advance = advance;
        }

        if (options.Seed is { } seed)
        {
            request.Seed = seed;
        }

        var response = await Connection.ModelCallAsync<Wire.ExecuteActionRequest, Wire.ExecuteActionResponse>(ExecuteActionMethod, request, cancellationToken).ConfigureAwait(false);
        if (response.Error.Length > 0)
        {
            throw new ExecutionException(
                response.Error,
                ExecuteActionMethod,
                ModelWire.Diagnostics(response.Diagnostics),
                response.FinalTime,
                ExecutionWire.Trace(response.Trace, ExecuteActionMethod),
                response.TraceDropped);
        }

        RequireShape(
            ExecuteActionMethod,
            explore,
            response.Exploration,
            response.Outcomes.Count,
            ("outputs", response.Outputs.Count > 0),
            ("diagnostics", response.Diagnostics.Count > 0),
            ("final_time", BitConverter.DoubleToInt64Bits(response.FinalTime) != 0),
            ("performer_attributes", response.PerformerAttributes.Count > 0),
            ("order_dependent", response.OrderDependent.Count > 0),
            ("trace", response.Trace.Count > 0),
            ("trace_dropped", response.TraceDropped != 0));
        return response;
    }

    private async Task<Wire.ExecuteStateResponse> StateAsync(
        string stateMachineSymbolId,
        IEnumerable<string>? events,
        RunOptions? options,
        bool explore,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(stateMachineSymbolId);
        Connection.ThrowIfDisposed();
        options ??= RunOptions.Default;
        var sentEvents = new ValueList<string>(events ?? []);
        var schedule = Schedule(options, explore, explore ? nameof(ExecuteStateAsync) : nameof(ExploreStateAsync));
        if (options.Trace && explore)
        {
            throw new ArgumentException("RunOptions.Trace asks for one run's trace, which an exploration describes none of; leave it false.");
        }

        if (options.Events is { Count: > 0 })
        {
            throw new ArgumentException("RunOptions.Events posts to an action's performer; a state machine takes its events as the events argument.");
        }

        RequireRun(schedule, options, Capabilities.StateTrace);
        var request = new Wire.ExecuteStateRequest
        {
            ModelHash = Hash,
            StateMachineSymbolId = stateMachineSymbolId,
            Events = { sentEvents },
            Schedule = schedule ?? string.Empty,
            PerformerSymbolId = options.PerformerSymbolId ?? string.Empty,
            Trace = options.Trace,
            Draws = options.Draws ?? string.Empty,
            ClockStep = options.ClockStep,
        };
        if (options.Advance is { } advance)
        {
            request.Advance = advance;
        }

        if (options.Seed is { } seed)
        {
            request.Seed = seed;
        }

        var response = await Connection.ModelCallAsync<Wire.ExecuteStateRequest, Wire.ExecuteStateResponse>(ExecuteStateMethod, request, cancellationToken).ConfigureAwait(false);
        if (response.Error.Length > 0)
        {
            throw new ExecutionException(
                response.Error,
                ExecuteStateMethod,
                ModelWire.Diagnostics(response.Diagnostics),
                response.FinalTime,
                ExecutionWire.Trace(response.Trace, ExecuteStateMethod),
                response.TraceDropped);
        }

        RequireShape(
            ExecuteStateMethod,
            explore,
            response.Exploration,
            response.Outcomes.Count,
            ("states_visited", response.StatesVisited.Count > 0),
            ("final_context", response.FinalContext.Count > 0),
            ("diagnostics", response.Diagnostics.Count > 0),
            ("final_time", BitConverter.DoubleToInt64Bits(response.FinalTime) != 0),
            ("trace", response.Trace.Count > 0),
            ("trace_dropped", response.TraceDropped != 0),
            ("order_dependent", response.OrderDependent.Count > 0));
        return response;
    }

    private async Task<CheckResult> CheckAsync(
        Wire.CheckBehaviorRequest request,
        IReadOnlyDictionary<string, Value>? inputs,
        CheckOptions? options,
        CancellationToken cancellationToken)
    {
        Connection.ThrowIfDisposed();
        options ??= CheckOptions.Default;
        if (options.Engine is { Length: 0 } || options.PerformerSymbolId is { Length: 0 })
        {
            throw new ArgumentException("CheckOptions names an empty engine or performer; leave Engine or PerformerSymbolId null to name none.");
        }

        var sent = Inputs(inputs);
        var capabilities = Connection.Capabilities;
        capabilities.Require(Capabilities.Check);
        if (options.PerformerSymbolId is not null)
        {
            capabilities.Require(Capabilities.Performer);
        }

        if (options.Engine is not null)
        {
            capabilities.Require(Capabilities.Engines);
        }

        if (options.Explores)
        {
            capabilities.Require(Capabilities.ScheduleExplore);
        }

        Fit(sent);
        request.ModelHash = Hash;
        AddInputs(request.Inputs, sent);
        request.PerformerSymbolId = options.PerformerSymbolId ?? string.Empty;
        request.CheckProperty.Add(options.Properties);
        request.CheckDiverge.Add(options.Diverge);
        request.CheckInput.Add(options.FreeInputs);
        request.CheckAssume.Add(options.Assume);
        request.CheckWitness = options.Witness;
        request.CheckDepth = options.Depth;
        request.CheckStates = options.States;
        request.CheckUnroll = options.Unroll;
        request.CheckTimeout = options.Timeout.TotalSeconds;
        request.Engine = options.Engine ?? string.Empty;
        var response = await Connection.ModelCallAsync<Wire.CheckBehaviorRequest, Wire.CheckBehaviorResponse>(CheckBehaviorMethod, request, cancellationToken).ConfigureAwait(false);
        var result = ExecutionWire.Check(response);
        if (response.Error.Length > 0)
        {
            throw new CheckException(response.Error, CheckBehaviorMethod, result);
        }

        if (response.Verdict.Length == 0)
        {
            throw TransportException.Malformed(CheckBehaviorMethod, "neither a verdict nor a failure");
        }

        return result;
    }

    // The schedule sent: the one named, or explore for an exploration naming none. A run and an
    // exploration answer differently, so each call takes only its own kind of schedule.
    private static string? Schedule(RunOptions options, bool explore, string other)
    {
        if (options.Schedule is { Length: 0 } || options.PerformerSymbolId is { Length: 0 } || options.Draws is { Length: 0 })
        {
            throw new ArgumentException("RunOptions names an empty schedule, performer or draw policy; leave Schedule, PerformerSymbolId or Draws null for the default.");
        }

        if (explore && options.Schedule is { } once && !options.Explores)
        {
            throw new ArgumentException($"The schedule {once} runs once, which {other} answers; an exploration takes {RunOptions.Explore} or {RunOptions.Explore}:runs=<n>,depth=<d>.");
        }

        if (!explore && options.Explores)
        {
            throw new ArgumentException($"The schedule {options.Schedule} explores every order, which {other} answers; a run takes one order.");
        }

        return explore ? options.Schedule ?? RunOptions.Explore : options.Schedule;
    }

    // The capabilities a run's options need, in the order the service checks them.
    private void RequireRun(string? schedule, RunOptions options, string traceCapability)
    {
        var capabilities = Connection.Capabilities;
        if (schedule is not null)
        {
            capabilities.Require(Capabilities.Schedule);
            if (RunOptions.IsExploring(schedule))
            {
                capabilities.Require(Capabilities.ScheduleExplore);
            }
        }

        if (options.Trace)
        {
            capabilities.Require(traceCapability);
        }

        if (options.PerformerSymbolId is not null)
        {
            capabilities.Require(Capabilities.Performer);
        }
    }

    // The inputs as sent, in ordinal order of their names, refusing a value no service reads as an argument.
    private static List<KeyValuePair<string, Wire.Value>> Inputs(IReadOnlyDictionary<string, Value>? inputs) =>
        inputs is null ? [] : ValueWire.ToWire(new ValueDictionary<Value>(inputs), "inputs");

    private static void AddInputs(Google.Protobuf.Collections.MapField<string, Wire.Value> field, List<KeyValuePair<string, Wire.Value>> sent)
    {
        foreach (var (name, value) in sent)
        {
            field.Add(name, value);
        }
    }

    private void Fit(List<KeyValuePair<string, Wire.Value>> sent)
    {
        foreach (var (_, value) in sent)
        {
            ValueWire.Fit(value, Connection.Capabilities);
        }
    }

    // A run answers one run and an exploration its outcomes and status; an answer of the other shape is malformed.
    private static void RequireShape(string method, bool explore, Wire.ExplorationStatus? exploration, int outcomes, params (string Field, bool Set)[] run)
    {
        if (explore)
        {
            if (exploration is null)
            {
                throw TransportException.Malformed(method, "an exploring schedule with no exploration status");
            }

            foreach (var (field, set) in run)
            {
                if (set)
                {
                    throw TransportException.Malformed(method, $"an exploring schedule with {field}, which only one run reports");
                }
            }
        }
        else if (exploration is not null || outcomes > 0)
        {
            throw TransportException.Malformed(method, "one run with an exploration's outcomes or status");
        }
    }
}
