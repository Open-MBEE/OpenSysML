function answer = executeState(model, stateId, varargin)
%EXECUTESTATE Run one state-machine outcome.

    options = opensysml.internal.nameValueOptions(struct( ...
        'events', {{}}, 'schedule', '', 'performer', '', 'trace', false), varargin, 'executeState');
    schedule = char(options.schedule);
    rejectExplore(schedule, 'opensysml.exploreState');
    capabilities = opensysml.internal.runCapabilities(model.connection, schedule, ...
        options.performer, options.trace);
    events = toTextCells(options.events);
    request = struct('modelHash', model.hash, ...
        'stateMachineSymbolId', char(stateId), 'events', {events});
    if ~isempty(schedule), request.schedule = schedule; end
    if ~isempty(options.performer), request.performerSymbolId = char(options.performer); end
    if options.trace, request.trace = true; end
    raw = opensysml.internal.checkError(opensysml.call(model.connection, ...
        'ExecuteState', request, capabilities), 'ExecuteState');
    statesVisited = toTextCells(fieldOr(raw, 'statesVisited', {}));
    statesVisited = statesVisited(:);
    traceRaw = fieldOr(raw, 'trace', {});
    if iscell(traceRaw)
        trace = cellfun(@decodeTraceEvent, traceRaw(:), 'UniformOutput', false);
    elseif isstruct(traceRaw)
        trace = arrayfun(@decodeTraceEvent, traceRaw(:), 'UniformOutput', false);
    else
        trace = {};
    end
    answer = struct('statesVisited', {statesVisited}, ...
        'finalContext', struct(), 'finalTime', fieldOr(raw, 'finalTime', 0), ...
        'trace', {trace}, 'traceDropped', fieldOr(raw, 'traceDropped', 0));
    if isfield(raw, 'finalContext')
        answer.finalContext = opensysml.internal.decodeValueMap(raw.finalContext);
    end
end

function event = decodeTraceEvent(raw)
    event = opensysml.internal.decodeDocumentValue(struct('event', raw));
end

function rejectExplore(schedule, method)
    if strcmp(schedule, 'explore') || strncmp(schedule, 'explore:', 8)
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('schedule ''%s'' answers with every outcome, not one run''s result: use %s', ...
            schedule, method));
    end
end

function values = toTextCells(raw)
    if isempty(raw), values = {};
    elseif ischar(raw), values = {raw};
    elseif iscell(raw), values = cellfun(@char, raw(:)', 'UniformOutput', false);
    elseif isstruct(raw), values = cellfun(@char, num2cell(raw(:)'), 'UniformOutput', false);
    else, values = {char(raw)};
    end
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
