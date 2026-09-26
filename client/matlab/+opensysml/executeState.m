function answer = executeState(model, stateId, varargin)
%EXECUTESTATE Run a state machine; 'events' is a cell of names and
%'schedule' the policy spelling. Values in the answer arrive decoded.

    request.modelHash = model.hash;
    request.stateMachineSymbolId = char(stateId);
    events = getArg(varargin, 'events', {});
    request.events = cellfun(@char, events, 'UniformOutput', false);
    schedule = getArg(varargin, 'schedule', '');
    if ~isempty(schedule), request.schedule = schedule; end
    answer = opensysml.internal.checkError(opensysml.call(model.connection, 'ExecuteState', request), 'ExecuteState');
    if isfield(answer, 'finalContext')
        answer.finalContext = opensysml.internal.decodeValueMap(answer.finalContext);
    end
end

function v = getArg(args, name, default)
    v = default;
    for i = 1:2:numel(args)
        if strcmp(args{i}, name), v = args{i+1}; end
    end
end
