function value = decodeDocumentValue(raw)
%DECODEDOCUMENTVALUE Decode one answered DocumentValue.

    arms = {'elementId','stringValue','intValue','bigIntValue','realValue','boolValue', ...
        'infinity','quantity','object','verdict','state','event'};
    arm = '';
    for i = 1:numel(arms)
        if isfield(raw, arms{i}), arm = arms{i}; break; end
    end
    if isempty(arm)
        opensysml.internal.raise('opensysml:decode', ...
            'the service answered a document value without a known kind');
    end
    switch arm
        case 'elementId'
            elementType = fieldOr(raw, 'elementType', '');
            value = struct('type', 'element', 'id', raw.elementId, ...
                'elementType', elementType);
        case 'stringValue'
            value = raw.stringValue;
        case 'intValue'
            value = opensysml.parseInt64(valueString(raw.intValue));
        case 'bigIntValue'
            value = opensysml.decodeValue(struct('bigIntValue', raw.bigIntValue));
        case 'realValue'
            value = realValue(raw.realValue);
        case 'boolValue'
            value = logical(raw.boolValue);
        case 'infinity'
            value = opensysml.infinity();
        case 'quantity'
            value = opensysml.decodeValue(struct('quantity', raw.quantity));
        case 'object'
            object = raw.object;
            value = struct('type', 'object', ...
                'id', parseId(fieldOr(object, 'instanceId', '0')), ...
                'path', fieldOr(object, 'path', ''), ...
                'element', decodeElement(fieldOr(object, 'element', struct()), ...
                    fieldOr(object, 'elementType', '')));
        case 'verdict'
            value = decodeVerdict(raw.verdict);
        case 'state'
            value = decodeState(raw.state);
        case 'event'
            value = decodeEvent(raw.event);
    end
end

function value = decodeVerdict(raw)
    value = struct('type', 'verdict', ...
        'assertion', decodeElement(fieldOr(raw, 'assertion', struct()), ''), ...
        'kind', fieldOr(raw, 'kind', ''), 'text', fieldOr(raw, 'text', ''), ...
        'path', fieldOr(raw, 'path', ''), 'status', fieldOr(raw, 'verdict', ''), ...
        'condition', fieldOr(raw, 'condition', ''), 'reason', fieldOr(raw, 'reason', ''), ...
        'verification', {toCells(fieldOr(raw, 'verification', {}))});
end

function value = decodeState(raw)
    stateValue = [];
    if isfield(raw, 'state') && ~isempty(fieldnames(raw.state))
        stateValue = decodeElement(raw.state, '');
    end
    object = [];
    if isfield(raw, 'object') && isstruct(raw.object) && ~isempty(fieldnames(raw.object))
        object = opensysml.internal.decodeDocumentValue(struct('object', raw.object));
    end
    value = struct('type', 'state', ...
        'object', object, ...
        'machine', fieldOr(raw, 'machine', ''), 'name', fieldOr(raw, 'name', ''), ...
        'path', fieldOr(raw, 'statePath', ''), 'state', stateValue, ...
        'region', fieldOr(raw, 'region', ''), ...
        'enclosing', {toCells(fieldOr(raw, 'enclosing', {}))});
end

function value = decodeEvent(raw)
    object = [];
    if isfield(raw, 'object') && ~isempty(fieldnames(raw.object))
        object = opensysml.internal.decodeDocumentValue(struct('object', raw.object));
    end
    target = [];
    if isfield(raw, 'target') && ~isempty(fieldnames(raw.target))
        target = opensysml.internal.decodeDocumentValue(struct('object', raw.target));
    end
    time = opensysml.internal.decodeDocumentValue(raw.time);
    value = struct('type', 'event', 'kind', fieldOr(raw, 'kind', ''), ...
        'time', time, 'text', fieldOr(raw, 'text', ''), 'object', object, ...
        'machine', fieldOr(raw, 'machine', ''), 'state', fieldOr(raw, 'state', ''), ...
        'fromState', fieldOr(raw, 'from', ''), 'toState', fieldOr(raw, 'to', ''), ...
        'target', target, 'event', fieldOr(raw, 'event', ''), ...
        'payload', {toCells(fieldOr(raw, 'payload', {}))}, ...
        'alternatives', {toCells(fieldOr(raw, 'alternatives', {}))}, ...
        'taken', fieldOr(raw, 'taken', ''));
end

function value = parseId(raw)
    value = opensysml.parseInt64(valueString(raw));
end

function value = decodeElement(raw, elementType)
    if isfield(raw, 'elementId')
        value = opensysml.internal.decodeDocumentValue(raw);
    else
        value = opensysml.elementRef('', fieldOr(raw, 'elementType', elementType));
    end
end

function value = realValue(raw)
    if ischar(raw)
        switch raw
            case 'NaN', value = NaN;
            case 'Infinity', value = Inf;
            case '-Infinity', value = -Inf;
            otherwise
                opensysml.internal.raise('opensysml:decode', ...
                    sprintf('invalid real value ''%s''', raw));
        end
    else, value = double(raw);
    end
end

function text = valueString(raw)
    if ischar(raw), text = raw; else, text = sprintf('%d', int64(raw)); end
end

function value = fieldOr(record, name, fallback)
    if isstruct(record) && isfield(record, name), value = record.(name);
    else, value = fallback;
    end
end

function cells = toCells(raw)
    if isempty(raw), cells = {};
    elseif iscell(raw), cells = raw(:)';
    elseif isstruct(raw), cells = num2cell(raw(:)');
    elseif ischar(raw), cells = {raw};
    else, cells = num2cell(raw(:)');
    end
end
