function result = exploreState(model, stateId, varargin)
%EXPLORESTATE Explore every outcome admitted for a state machine.

    options = opensysml.internal.nameValueOptions(struct( ...
        'events', {{}}, 'schedule', 'explore', 'performer', ''), varargin, 'exploreState');
    schedule = char(options.schedule);
    if ~(strcmp(schedule, 'explore') || strncmp(schedule, 'explore:', 8))
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('schedule ''%s'' answers one run''s result, not every outcome: spell it ''explore'' or ''explore:runs=<n>,depth=<d>''', ...
            schedule));
    end
    capabilities = opensysml.internal.runCapabilities(model.connection, schedule, options.performer);
    events = toTextCells(options.events);
    request = struct('modelHash', model.hash, ...
        'stateMachineSymbolId', char(stateId), 'events', {events}, 'schedule', schedule);
    if ~isempty(options.performer), request.performerSymbolId = char(options.performer); end
    raw = opensysml.internal.checkError(opensysml.call(model.connection, ...
        'ExecuteState', request, capabilities), 'ExecuteState');
    result = opensysml.internal.decodeExplorationResponse(raw, model.connection);
end

function values = toTextCells(raw)
    if isempty(raw), values = {};
    elseif ischar(raw), values = {raw};
    elseif iscell(raw), values = cellfun(@char, raw(:)', 'UniformOutput', false);
    else, values = {char(raw)};
    end
end
