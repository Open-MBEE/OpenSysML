function result = exploreAction(model, actionId, varargin)
%EXPLOREACTION Explore every outcome admitted for an action.

    options = opensysml.internal.nameValueOptions(struct( ...
        'inputs', struct(), 'schedule', 'explore', 'performer', ''), varargin, 'exploreAction');
    schedule = char(options.schedule);
    requireExploreSchedule(schedule);
    capabilities = opensysml.internal.runCapabilities(model.connection, schedule, options.performer);
    request = struct('modelHash', model.hash, 'actionSymbolId', char(actionId), ...
        'inputs', encodeInputs(options.inputs, model.connection), 'schedule', schedule);
    if ~isempty(options.performer), request.performerSymbolId = char(options.performer); end
    raw = opensysml.internal.checkError(opensysml.call(model.connection, ...
        'ExecuteAction', request, capabilities), 'ExecuteAction');
    result = opensysml.internal.decodeExplorationResponse(raw, model.connection);
end

function requireExploreSchedule(schedule)
    if ~(strcmp(schedule, 'explore') || strncmp(schedule, 'explore:', 8))
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('schedule ''%s'' answers one run''s result, not every outcome: spell it ''explore'' or ''explore:runs=<n>,depth=<d>''', ...
            schedule));
    end
end

function encoded = encodeInputs(inputs, conn)
    encoded = struct();
    if isa(inputs, 'containers.Map')
        names = inputs.keys;
        for i = 1:numel(names)
            encoded.(names{i}) = opensysml.encodeValue(inputs(names{i}), conn);
        end
    elseif isstruct(inputs) && isscalar(inputs)
        names = fieldnames(inputs);
        for i = 1:numel(names)
            encoded.(names{i}) = opensysml.encodeValue(inputs.(names{i}), conn);
        end
    elseif ~isempty(inputs)
        opensysml.internal.raise('opensysml:argument', ...
            'exploreAction inputs must be a scalar struct or containers.Map');
    end
end
