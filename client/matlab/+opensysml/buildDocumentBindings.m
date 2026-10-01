function out = buildDocumentBindings(bindings)
%BUILDDOCUMENTBINDINGS Encode document-query bindings for a request.

    if nargin < 1 || isempty(bindings), out = {}; return; end
    if isa(bindings, 'containers.Map')
        names = keys(bindings);
        rawValues = cellfun(@(key) bindings(key), names, 'UniformOutput', false);
    elseif isstruct(bindings) && isscalar(bindings)
        names = fieldnames(bindings)';
        rawValues = cellfun(@(key) bindings.(key), names, 'UniformOutput', false);
    else
        opensysml.internal.raise('opensysml:argument', ...
            'document bindings must be a struct or containers.Map');
    end
    out = cell(1, numel(names));
    for i = 1:numel(names)
        values = rawValues{i};
        if ~iscell(values), values = {values}; end
        encoded = cellfun(@(value) boundValue(names{i}, value), ...
            values(:)', 'UniformOutput', false);
        out{i} = struct('parameter', names{i}, 'values', {encoded});
    end
end

function value = boundValue(parameter, item)
    if isstruct(item) && isfield(item, 'type')
        switch item.type
            case 'element'
                value = struct('elementId', item.id);
                return;
            case 'object'
                if (~isfield(item, 'id') || item.id == 0) && ...
                        (~isfield(item, 'path') || isempty(item.path))
                    documentError(parameter, item, ...
                        'an object is bound by id or by path; neither was given');
                end
                object = struct();
                if isfield(item, 'id') && item.id ~= 0
                    object.instanceId = signedInt64Text(parameter, item, item.id);
                end
                if isfield(item, 'path') && ~isempty(item.path), object.path = item.path; end
                value = struct('object', object);
                return;
            case 'verdict'
                documentError(parameter, item, 'a verdict is answered by queries, not bound to them');
            case 'state'
                documentError(parameter, item, 'a state row is answered by queries, not bound to them');
            case 'event'
                documentError(parameter, item, 'an event row is answered by queries, not bound to them');
        end
    end
    if islogical(item) && isscalar(item)
        value = struct('boolValue', item);
    elseif ischar(item) || (isStringValue(item) && isscalar(item))
        value = struct('stringValue', char(item));
    elseif isnumeric(item) && isscalar(item) && isinteger(item) && ~islogical(item)
        value = struct('intValue', signedInt64Text(parameter, item, item));
    elseif isnumeric(item) && isscalar(item) && isreal(item)
        value = struct('realValue', docReal(double(item)));
    elseif isstruct(item) && isfield(item, 'magnitude') && isfield(item, 'unit')
        quantity = opensysml.encodeValue(item);
        value = struct('quantity', quantity.quantity);
    else
        documentError(parameter, item, ...
            'a binding is a str, int, float, bool, Quantity, ElementRef or ObjectRef');
    end
end

function text = signedInt64Text(parameter, original, value)
    if isa(value, 'uint64')
        if value > uint64(intmax('int64'))
            documentError(parameter, original, 'integer is outside the signed 64-bit range');
        end
        text = sprintf('%u', value);
    elseif isinteger(value) && isscalar(value) && ~islogical(value)
        text = sprintf('%d', int64(value));
    elseif isnumeric(value) && isscalar(value) && isreal(value) && ...
            isfinite(value) && fix(value) == value
        limit = 9223372036854775808;
        if value < -limit || value >= limit
            documentError(parameter, original, 'integer is outside the signed 64-bit range');
        end
        text = sprintf('%d', int64(value));
    else
        documentError(parameter, original, 'integer is outside the signed 64-bit range');
    end
end

function documentError(parameter, value, reason)
    opensysml.internal.raise('opensysml:argument', ...
        sprintf('binding ''%s'' cannot carry %s: %s', parameter, ...
        valueText(value), reason));
end

function text = valueText(value)
    if ischar(value), text = ['''' value ''''];
    elseif isnumeric(value) || islogical(value), text = num2str(value);
    elseif isstruct(value) && isfield(value, 'type')
        if strcmp(value.type, 'element'), text = value.id;
        elseif strcmp(value.type, 'object')
            if isfield(value, 'path') && ~isempty(value.path), text = value.path;
            elseif isfield(value, 'id'), text = ['#' num2str(value.id)];
            else, text = value.type;
            end
        else, text = value.type;
        end
    else, text = class(value);
    end
end

function value = docReal(value)
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
