function answer = executeState(model, stateId, varargin)
%EXECUTESTATE Run one state-machine outcome.

    options = opensysml.internal.nameValueOptions(struct( ...
        'events', {{}}, 'schedule', '', 'performer', ''), varargin, 'executeState');
    schedule = char(options.schedule);
    rejectExplore(schedule, 'explore_state');
    capabilities = opensysml.internal.runCapabilities(model.connection, schedule, options.performer);
    events = toTextCells(options.events);
    request = struct('modelHash', model.hash, ...
        'stateMachineSymbolId', char(stateId), 'events', {events});
    if ~isempty(schedule), request.schedule = schedule; end
    if ~isempty(options.performer), request.performerSymbolId = char(options.performer); end
    raw = opensysml.internal.checkError(opensysml.call(model.connection, ...
        'ExecuteState', request, capabilities), 'ExecuteState');
    statesVisited = toTextCells(fieldOr(raw, 'statesVisited', {}));
    statesVisited = statesVisited(:);
    answer = struct('statesVisited', {statesVisited}, ...
        'finalContext', struct(), 'finalTime', fieldOr(raw, 'finalTime', 0));
    if isfield(raw, 'finalContext')
        answer.finalContext = opensysml.internal.decodeValueMap(raw.finalContext);
    end
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
