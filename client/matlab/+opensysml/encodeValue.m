function v = encodeValue(x, conn)
%ENCODEVALUE Write a Value for a request.

    if nargin < 2, conn = []; end
    try
        v = encodeValueInner(x, conn);
    catch e
        recordClientError(e);
        rethrow(e);
    end
end

function v = encodeValueInner(x, conn)
    if islogical(x) && isscalar(x)
        v = struct('boolValue', x);
    elseif isnumeric(x) && isscalar(x) && isreal(x)
        if isinteger(x)
            v = struct('intValue', sprintf('%d', integerValue(x, 'integer value')));
        else
            v = struct('realValue', wireReal(double(x)));
        end
    elseif isnumeric(x) && isscalar(x) && ~isreal(x)
        requireCapability(conn, 'complex_values');
        v = struct('complex', struct('real', wireReal(double(real(x))), ...
                                     'imaginary', wireReal(double(imag(x)))));
    elseif ischar(x) || isStringValue(x)
        v = struct('stringValue', char(x));
    elseif isempty(x) && ~iscell(x)
        v = struct('null', '');
    elseif isstruct(x)
        v = encodeStruct(x, conn);
    elseif iscell(x) || (isnumeric(x) && ~isscalar(x)) || ...
            (islogical(x) && ~isscalar(x))
        elements = cellfun(@(item) opensysml.encodeValue(item, conn), ...
            valueList(x), 'UniformOutput', false);
        v = struct('sequence', struct('elements', {elements}));
    else
        opensysml.internal.raise('opensysml:encode', ...
            sprintf('no wire encoding for a value of class %s', class(x)));
    end
end

function v = encodeStruct(x, conn)
    if isfield(x, 'unset')
        opensysml.internal.raise('opensysml:encode', 'an unset value cannot be sent');
    elseif isfield(x, 'reason') && isfield(x, 'lower') && isfield(x, 'upper')
        opensysml.internal.raise('opensysml:encode', ...
            sprintf('an undetermined value cannot be sent: %s', x.reason));
    elseif isfield(x, 'infinity')
        requireCapability(conn, 'infinity_value');
        v = struct('infinity', true);
    elseif isfield(x, 'bigInteger')
        v = struct('bigIntValue', char(x.bigInteger));
    elseif isfield(x, 'instanceRef')
        id = integerValue(x.instanceRef, 'instance reference id');
        v = struct('instanceId', sprintf('%d', id));
    elseif isfield(x, 'magnitude') && isfield(x, 'unit')
        v = struct('quantity', encodeQuantityBody(x));
    elseif isfield(x, 'literalId')
        body = struct('literalId', x.literalId, ...
            'enumerationId', getField(x, 'enumerationId', ''), ...
            'name', getField(x, 'name', ''));
        literalValue = getField(x, 'value', []);
        if ~isempty(literalValue)
            body.value = opensysml.encodeValue(literalValue, conn);
        end
        v = struct('enumLiteral', body);
    elseif isfield(x, 'calcId')
        self = getField(x, 'self', []);
        if ~isempty(self)
            opensysml.internal.raise('opensysml:encode', ['a function read off an object cannot be sent: ' ...
                'selfId names no instance in another call']);
        end
        requireCapability(conn, 'function_values');
        v = struct('function', struct('calcId', x.calcId));
    elseif isfield(x, 'set')
        requireCapability(conn, 'set_values');
        values = valueList(x.set);
        elements = cellfun(@(item) opensysml.encodeValue(item, conn), ...
            values, 'UniformOutput', false);
        for i = 1:numel(elements)
            for j = i+1:numel(elements)
                if isequaln(elements{i}, elements{j})
                    opensysml.internal.raise('opensysml:encode', ...
                        'set lists a member more than once');
                end
            end
        end
        v = struct('set', struct('elements', {elements}));
    elseif isfield(x, 'dimensions') && isfield(x, 'elements')
        requireCapability(conn, 'structured_values');
        dims = encodeDimensions(x.dimensions);
        elements = cellfun(@(item) opensysml.encodeValue(item, conn), ...
            valueList(x.elements), 'UniformOutput', false);
        validateShape(dims, numel(elements), 'array');
        v = struct('array', struct('dimensions', {dims}, 'elements', {elements}));
    elseif isfield(x, 'dimensions') && isfield(x, 'components')
        components = valueList(x.components);
        if isempty(components) || isQuantity(components{1})
            requireCapability(conn, 'tensor_values');
            dims = encodeDimensions(x.dimensions);
            quantities = cellfun(@encodeQuantityBody, components, 'UniformOutput', false);
            validateShape(dims, numel(quantities), 'tensorQuantity');
            v = struct('tensorQuantity', ...
                struct('dimensions', {dims}, 'components', {quantities}));
        else
            opensysml.internal.raise('opensysml:encode', ...
                'tensorQuantity components must be quantities');
        end
    elseif isfield(x, 'components') && hasQuantityComponents(x.components)
        requireCapability(conn, 'structured_values');
        components = valueList(x.components);
        quantities = cellfun(@encodeQuantityBody, components, 'UniformOutput', false);
        if isempty(quantities)
            opensysml.internal.raise('opensysml:encode', 'vectorQuantity has no components');
        end
        v = struct('vectorQuantity', struct('components', {quantities}));
    elseif isfield(x, 'components')
        requireCapability(conn, 'structured_values');
        components = valueList(x.components);
        wireComponents = cell(1, numel(components));
        for i = 1:numel(components)
            component = components{i};
            if ~isnumeric(component) || ~isscalar(component) || ~isreal(component)
                opensysml.internal.raise('opensysml:encode', ...
                    'vector components must be real numeric scalars');
            end
            wireComponents{i} = opensysml.encodeValue(component, conn);
            if ~isfield(wireComponents{i}, 'intValue') && ~isfield(wireComponents{i}, 'realValue')
                opensysml.internal.raise('opensysml:encode', ...
                    'vector components must be real numeric scalars');
            end
        end
        v = struct('vector', struct('components', {wireComponents}));
    elseif isfield(x, 'unit') && isfield(x, 'unitTerm')
        requireCapability(conn, 'measurement_refs');
        body = struct();
        if ~isempty(x.unit), body.unit = x.unit; end
        if isfield(x, 'unitId') && ~isempty(x.unitId), body.unitId = x.unitId; end
        if isempty(x.unitTerm) && (~isempty(getField(x, 'unit', '')) || ...
                ~isempty(getField(x, 'unitId', '')))
            opensysml.internal.raise('opensysml:encode', ...
                'measurementRef naming a unit requires its unitTerm');
        end
        if isempty(x.unitTerm) && isempty(getField(x, 'unit', '')) && ...
                isempty(getField(x, 'unitId', ''))
            opensysml.internal.raise('opensysml:encode', ...
                'measurementRef carries neither unit nor unitId');
        end
        if ~isempty(x.unitTerm), body.unitTerm = wireUnitTerm(x.unitTerm); end
        v = struct('measurementRef', body);
    elseif isfield(x, 'elementId')
        body = struct('elementId', x.elementId);
        metaclassId = getField(x, 'metaclassId', '');
        if ~isempty(metaclassId), body.metaclassId = metaclassId; end
        requireCapability(conn, 'metaobject_values');
        v = struct('metaobject', body);
    else
        opensysml.internal.raise('opensysml:encode', 'no wire encoding for this struct');
    end
end

function body = encodeQuantityBody(q)
    magnitude = q.magnitude;
    if isstruct(magnitude) && isfield(magnitude, 'bigInteger')
        body.bigIntMagnitude = char(magnitude.bigInteger);
    elseif ~isnumeric(magnitude) || ~isscalar(magnitude) || ~isreal(magnitude)
        opensysml.internal.raise('opensysml:encode', 'quantity magnitude must be a real scalar');
    elseif isinteger(magnitude)
        body.intMagnitude = sprintf('%d', integerValue(magnitude, 'quantity magnitude'));
    else
        body.realMagnitude = wireReal(double(magnitude));
    end
    if ~isempty(q.unit), body.unit = q.unit; end
    if isfield(q, 'unitTerm') && ~isempty(q.unitTerm)
        body.unitTerm = wireUnitTerm(q.unitTerm);
    end
end

function body = wireUnitTerm(term)
    body = term;
    if ~isstruct(body)
        opensysml.internal.raise('opensysml:encode', 'unitTerm must be a struct');
    end
    if isfield(body, 'factors') && isstruct(body.factors)
        body.factors = num2cell(body.factors(:)');
    end
end

function dims = encodeDimensions(raw)
    values = valueList(raw);
    dims = cell(1, numel(values));
    for i = 1:numel(values)
        value = integerValue(values{i}, 'dimension');
        if value <= 0
            opensysml.internal.raise('opensysml:encode', 'array dimension is not positive');
        end
        dims{i} = sprintf('%d', value);
    end
end

function validateShape(dimensions, count, kind)
    expected = 1;
    for i = 1:numel(dimensions)
        extent = str2double(dimensions{i});
        expected = expected * extent;
    end
    if expected ~= count
        opensysml.internal.raise('opensysml:encode', ...
            sprintf('%s has %d elements for its dimensions', kind, count));
    end
end

function value = integerValue(raw, what)
    if isa(raw, 'int64') && isscalar(raw)
        value = raw;
    elseif isinteger(raw) && isscalar(raw) && ~islogical(raw)
        if isa(raw, 'uint64') && raw > uint64(intmax('int64'))
            opensysml.internal.raise('opensysml:encode', ...
                sprintf('%s is outside the signed 64-bit range', what));
        end
        value = int64(raw);
    elseif ischar(raw)
        value = opensysml.parseInt64(raw);
    elseif isnumeric(raw) && isscalar(raw) && isfinite(raw) && fix(raw) == raw
        limit = 9223372036854775808;
        if raw < -limit || raw >= limit
            opensysml.internal.raise('opensysml:encode', ...
                sprintf('%s is outside the signed 64-bit range', what));
        end
        value = int64(raw);
    else
        opensysml.internal.raise('opensysml:encode', ...
            sprintf('%s must be an integer scalar', what));
    end
end

function list = valueList(raw)
    if iscell(raw)
        list = raw(:)';
    elseif isstruct(raw)
        list = num2cell(raw(:)');
    elseif isempty(raw)
        list = {};
    else
        list = num2cell(raw(:)');
    end
end

function tf = isQuantity(value)
    tf = isstruct(value) && isfield(value, 'magnitude') && isfield(value, 'unit');
end

function tf = hasQuantityComponents(raw)
    values = valueList(raw);
    tf = ~isempty(values) && isQuantity(values{1});
end

function value = getField(record, name, fallback)
    if isfield(record, name), value = record.(name);
    else, value = fallback;
    end
end

function requireCapability(conn, capability)
    if ~isempty(conn), conn.require(capability); end
end

function value = wireReal(value)
    if isnan(value), value = 'NaN';
    elseif isinf(value) && value > 0, value = 'Infinity';
    elseif isinf(value), value = '-Infinity';
    end
end

function tf = isStringValue(value)
    tf = false;
    if exist('isstring', 'builtin') || exist('isstring', 'file')
        tf = isstring(value);
    end
end

function recordClientError(e)
    if isempty(e.identifier) || ~strncmp(e.identifier, 'opensysml:', 10), return; end
    latest = opensysml.lastError();
    if strcmp(latest.identifier, e.identifier) && strcmp(latest.message, e.message), return; end
    opensysml.internal.raise(e.identifier, e.message);
end
