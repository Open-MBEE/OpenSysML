function v = encodeValue(x)
%ENCODEVALUE Write a Value for a request. Request lists are cell arrays —
%jsonencode writes a 1x1 struct array as one object; {} is the empty list.

    if islogical(x) && isscalar(x)
        v = struct('boolValue', x);
    elseif isa(x, 'int64') && isscalar(x)
        v = struct('intValue', sprintf('%d', x));
    elseif isinteger(x) && isscalar(x)
        v = struct('intValue', sprintf('%d', int64(x)));
    elseif isfloat(x) && isscalar(x) && isreal(x)
        v = struct('realValue', double(x));
    elseif isnumeric(x) && isscalar(x) && ~isreal(x)
        v = struct('complex', struct('real', double(real(x)), 'imaginary', double(imag(x))));
    elseif ischar(x) || (exist('isstring', 'builtin') && isstring(x))
        v = struct('stringValue', char(x));
    elseif isempty(x) && ~iscell(x)
        v = struct('null', '');
    elseif isstruct(x)
        v = encodeStruct(x);
    elseif iscell(x) || (isnumeric(x) && ~isscalar(x))
        elements = cellfun(@opensysml.encodeValue, mat2cell_list(x), 'UniformOutput', false);
        v = struct('sequence', struct('elements', {elements}));
    else
        error('opensysml:encode', 'no wire encoding for a value of class %s', class(x));
    end
end

function list = mat2cell_list(x)
    if iscell(x)
        list = x(:)';
    else
        list = num2cell(x(:)');
    end
end

function v = encodeStruct(x)
    if isfield(x, 'unset')
        error('opensysml:encode', 'an unset value cannot be sent');
    elseif isfield(x, 'reason') && isfield(x, 'lower') && isfield(x, 'upper')
        error('opensysml:encode', 'an undetermined value cannot be sent: %s', x.reason);
    elseif isfield(x, 'infinity')
        v = struct('infinity', true);
    elseif isfield(x, 'instanceRef')
        v = struct('instanceId', sprintf('%d', x.instanceRef));
    elseif isfield(x, 'magnitude') && isfield(x, 'unit')
        v = struct('quantity', encodeQuantityBody(x));
    elseif isfield(x, 'literalId')
        body = struct('literalId', x.literalId, 'enumerationId', x.enumerationId, 'name', x.name);
        if ~isempty(x.value)
            body.value = opensysml.encodeValue(x.value);
        end
        v = struct('enumLiteral', body);
    elseif isfield(x, 'calcId')
        if ~isempty(x.self)
            error('opensysml:encode', ['a function read off an object cannot be sent: ' ...
                'selfId names no instance in another call']);
        end
        v = struct('function', struct('calcId', x.calcId));
    elseif isfield(x, 'set')
        elements = cellfun(@opensysml.encodeValue, x.set(:)', 'UniformOutput', false);
        v = struct('set', struct('elements', {elements}));
    elseif isfield(x, 'elementId')
        v = struct('metaobject', struct('elementId', x.elementId, 'metaclassId', x.metaclassId));
    else
        error('opensysml:encode', 'no wire encoding for this struct');
    end
end

function body = encodeQuantityBody(q)
    if isa(q.magnitude, 'int64') || isinteger(q.magnitude)
        body.intMagnitude = sprintf('%d', int64(q.magnitude));
    else
        body.realMagnitude = double(q.magnitude);
    end
    if ~isempty(q.unit), body.unit = q.unit; end
    if ~isempty(q.unitTerm), body.unitTerm = q.unitTerm; end
end
