function answer = executeAction(model, actionId, varargin)
%EXECUTEACTION Run one action outcome.

    options = opensysml.internal.nameValueOptions(struct( ...
        'inputs', struct(), 'schedule', '', 'performer', ''), varargin, 'executeAction');
    schedule = char(options.schedule);
    rejectExplore(schedule, 'opensysml.exploreAction');
    capabilities = opensysml.internal.runCapabilities(model.connection, schedule, options.performer);
    request = struct('modelHash', model.hash, 'actionSymbolId', char(actionId), ...
        'inputs', encodeInputs(options.inputs, model.connection));
    if ~isempty(schedule), request.schedule = schedule; end
    if ~isempty(options.performer), request.performerSymbolId = char(options.performer); end
    raw = opensysml.internal.checkError(opensysml.call(model.connection, ...
        'ExecuteAction', request, capabilities), 'ExecuteAction');
    answer = struct('outputs', struct(), ...
        'performerAttributes', containers.Map('KeyType', 'char', 'ValueType', 'any'), ...
        'finalTime', fieldOr(raw, 'finalTime', 0));
    if isfield(raw, 'outputs')
        answer.outputs = opensysml.internal.decodeNamedValues(raw.outputs, model.connection, false);
    end
    if isfield(raw, 'performerAttributes')
        answer.performerAttributes = opensysml.internal.decodeNamedValues( ...
            raw.performerAttributes, model.connection, true);
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
