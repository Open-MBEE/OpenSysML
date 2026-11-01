function answer = executeAction(model, actionId, varargin)
%EXECUTEACTION Run one action outcome.
%   Options: inputs, schedule, performer, trace (the run's token steps, waits,
%   accepts, decisions and assignments as answer.trace), events (signals posted
%   to the performer in order, each once every token waits), and the run
%   options of the sysml command: advance (simulated seconds to run for,
%   reported back in finalTime), seed (the model's own draws), draws
%   ('random', 'min', 'max' or 'average') and clockStep.

    options = opensysml.internal.nameValueOptions(struct( ...
        'inputs', struct(), 'schedule', '', 'performer', '', 'trace', false, 'events', {{}}, ...
        'advance', [], 'seed', [], 'draws', '', 'clockStep', 0), varargin, 'executeAction');
    schedule = char(options.schedule);
    rejectExplore(schedule, 'opensysml.exploreAction');
    events = toTextCells(options.events);
    if ~isempty(events) && isempty(options.performer)
        opensysml.internal.raise('opensysml:argument', ...
            'executeAction events are posted to the object performing the action: name a performer');
    end
    capabilities = opensysml.internal.runCapabilities(model.connection, schedule, ...
        options.performer, options.trace, 'action_trace');
    request = struct('modelHash', model.hash, 'actionSymbolId', char(actionId), ...
        'inputs', encodeInputs(options.inputs, model.connection));
    if ~isempty(schedule), request.schedule = schedule; end
    if ~isempty(options.performer), request.performerSymbolId = char(options.performer); end
    if options.trace, request.trace = true; end
    if ~isempty(events), request.events = events; end
    request = opensysml.internal.runOptions(request, options, 'executeAction');
    raw = opensysml.call(model.connection, 'ExecuteAction', request, capabilities);
    traceRaw = fieldOr(raw, 'trace', {});
    if iscell(traceRaw)
        trace = cellfun(@decodeTraceEvent, traceRaw(:), 'UniformOutput', false);
    elseif isstruct(traceRaw)
        trace = arrayfun(@decodeTraceEvent, traceRaw(:), 'UniformOutput', false);
    else
        trace = {};
    end
    details = struct('trace', {trace}, 'traceDropped', fieldOr(raw, 'traceDropped', 0));
    raw = opensysml.internal.checkError(raw, 'ExecuteAction', details);
    answer = struct('outputs', struct(), ...
        'performerAttributes', containers.Map('KeyType', 'char', 'ValueType', 'any'), ...
        'finalTime', fieldOr(raw, 'finalTime', 0), ...
        'trace', {trace}, 'traceDropped', fieldOr(raw, 'traceDropped', 0));
    if isfield(raw, 'outputs')
        answer.outputs = opensysml.internal.decodeNamedValues(raw.outputs, model.connection, false);
    end
    if isfield(raw, 'performerAttributes')
        answer.performerAttributes = opensysml.internal.decodeNamedValues( ...
            raw.performerAttributes, model.connection, true);
    end
end

function event = decodeTraceEvent(raw)
    event = opensysml.internal.decodeDocumentValue(struct('event', raw));
end

function values = toTextCells(raw)
    if isempty(raw), values = {};
    elseif ischar(raw), values = {raw};
    elseif iscell(raw), values = cellfun(@char, raw(:)', 'UniformOutput', false);
    elseif isstruct(raw), values = cellfun(@char, num2cell(raw(:)'), 'UniformOutput', false);
    else, values = {char(raw)};
    end
end

function encoded = encodeInputs(inputs, conn)
    encoded = opensysml.internal.encodeNamedArguments(inputs, conn, 'executeAction inputs');
end

function rejectExplore(schedule, method)
    if strcmp(schedule, 'explore') || strncmp(schedule, 'explore:', 8)
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('schedule ''%s'' answers with every outcome, not one run''s result: use %s', ...
            schedule, method));
    end
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
