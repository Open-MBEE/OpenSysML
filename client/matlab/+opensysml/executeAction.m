function answer = executeAction(model, actionId, varargin)
%EXECUTEACTION Run an action; the answer is the wire record with every
%Value inside decoded. 'inputs' is a containers.Map or struct name->value;
%'schedule' is the policy spelling sysml -schedule takes.

    request.modelHash = model.hash;
    request.actionSymbolId = char(actionId);
    request.inputs = encodeInputs(getArg(varargin, 'inputs', containers.Map()));
    schedule = getArg(varargin, 'schedule', '');
    if ~isempty(schedule), request.schedule = schedule; end
    answer = opensysml.internal.checkError(opensysml.call(model.connection, 'ExecuteAction', request), 'ExecuteAction');
    answer = opensysml.internal.decodeValues(answer);
end

function inputs = encodeInputs(map)
    inputs = struct();
    if isa(map, 'containers.Map')
        keys = map.keys;
        for i = 1:numel(keys)
            inputs.(keys{i}) = opensysml.encodeValue(map(keys{i}));
        end
    elseif isstruct(map)
        names = fieldnames(map);
        for i = 1:numel(names)
            inputs.(names{i}) = opensysml.encodeValue(map.(names{i}));
        end
    end
end

function v = getArg(args, name, default)
    v = default;
    for i = 1:2:numel(args)
        if strcmp(args{i}, name), v = args{i+1}; end
    end
end
