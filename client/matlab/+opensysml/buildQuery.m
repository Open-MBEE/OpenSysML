function query = buildQuery(payload, varargin)
%BUILDQUERY Build the standard SysML v2 API query representation.

    options = struct('scope', [], 'select', [], 'where', []);
    if nargin >= 1 && ~isempty(payload)
        if ~isempty(varargin)
            queryError('pass a query payload or scope/select/where keywords, not both');
        end
        if ~isstruct(payload) && ~isa(payload, 'containers.Map')
            queryError(sprintf('a query is an object, not %s', class(payload)));
        end
        declared = payloadGet(payload, '@type', payloadGet(payload, 'type', 'Query'));
        if ~strcmp(declared, 'Query')
            queryError(sprintf('expected a ''Query'' payload, got %s', valueText(declared)));
        end
        keys = payloadKeys(payload);
        known = {'@type','type','@id','id','owningProject','scope','select','where'};
        unknown = setdiff(keys, known);
        if ~isempty(unknown)
            queryError(sprintf(['a query has no %s; the standard''s query is ' ...
                'scope, select and where'], strjoin(sort(unknown), ', ')));
        end
        options.scope = payloadGet(payload, 'scope', []);
        options.select = payloadGet(payload, 'select', []);
        options.where = payloadGet(payload, 'where', []);
    else
        if mod(numel(varargin), 2) ~= 0
            queryError('query options must be name-value pairs');
        end
        for i = 1:2:numel(varargin)
            key = char(varargin{i});
            if ~isfield(options, key)
                queryError(sprintf('a query has no option ''%s''', key));
            end
            options.(key) = varargin{i+1};
        end
    end

    query.scope = cellfun(@scopeId, sequence('scope', options.scope), 'UniformOutput', false);
    query.select = cellfun(@propertyName, sequence('select', options.select), 'UniformOutput', false);
    if ~isempty(options.where)
        query.where = constraint(options.where);
    end
end

function id = scopeId(entry)
    if ischar(entry) || isStringValue(entry)
        id = char(entry);
    elseif isstruct(entry) && isfield(entry, 'id') && ischar(entry.id)
        id = entry.id;
    elseif isstruct(entry) && isfield(entry, 'x_id') && ischar(entry.x_id)
        id = entry.x_id;
    elseif isa(entry, 'containers.Map') && isKey(entry, '@id') && ischar(entry('@id'))
        id = entry('@id');
    else
        queryError(sprintf('a scope entry is an element''s qualified name or a {''@id'': ...} reference, not %s', ...
            valueText(entry)));
    end
end

function name = propertyName(entry)
    if ~ischar(entry) && ~isStringValue(entry)
        queryError(sprintf('a selected property is a name, not %s', valueText(entry)));
    end
    name = char(entry);
end

function out = constraint(payload)
    if ~isstruct(payload) && ~isa(payload, 'containers.Map')
        queryError(sprintf('a constraint is an object, not %s', class(payload)));
    end
    if isstruct(payload) && isscalar(payload)
        fields = fieldnames(payload);
        if numel(fields) == 1 && any(strcmp(fields{1}, {'primitive', 'composite'}))
            out = payload;
            return;
        end
    end
    declared = payloadGet(payload, '@type', payloadGet(payload, 'type', ''));
    if isempty(declared)
        if ~isempty(payloadGet(payload, 'constraint', []))
            declared = 'CompositeConstraint';
        else
            declared = 'PrimitiveConstraint';
        end
    end
    if strcmp(declared, 'PrimitiveConstraint')
        out.primitive = primitive(payload);
    elseif strcmp(declared, 'CompositeConstraint')
        out.composite = composite(payload);
    else
        queryError(sprintf(['unknown constraint type %s; the standard''s constraints ' ...
            'are PrimitiveConstraint and CompositeConstraint'], valueText(declared)));
    end
end

function out = primitive(payload)
    rejectUnknown(payload, {'@type','type','@id','id','inverse','property','operator','value'}, ...
        'PrimitiveConstraint');
    operator = payloadGet(payload, 'operator', '');
    switch operator
        case '=', wireOperator = 'PRIMITIVE_OPERATOR_EQUAL';
        case '>', wireOperator = 'PRIMITIVE_OPERATOR_GREATER';
        case '<', wireOperator = 'PRIMITIVE_OPERATOR_LESS';
        otherwise
            queryError(sprintf(['unknown primitive operator %s; expected one of =, >, <'], ...
                valueText(operator)));
    end
    name = payloadGet(payload, 'property', '');
    if ~ischar(name) || isempty(name)
        queryError(sprintf('a primitive constraint names one property, not %s', valueText(name)));
    end
    values = payloadGet(payload, 'value', []);
    if iscell(values), values = values(:)';
    elseif isempty(values), values = {};
    elseif (isnumeric(values) || islogical(values)) && ~isscalar(values)
        values = num2cell(values(:)');
    else, values = {values};
    end
    out = struct('inverse', logical(payloadGet(payload, 'inverse', false)), ...
        'property', name, 'operator', wireOperator, ...
        'value', {cellfun(@comparisonValue, values, 'UniformOutput', false)});
end

function out = composite(payload)
    rejectUnknown(payload, {'@type','type','@id','id','constraint','operator'}, ...
        'CompositeConstraint');
    operator = payloadGet(payload, 'operator', '');
    switch operator
        case 'and', wireOperator = 'COMPOSITE_OPERATOR_AND';
        case 'or', wireOperator = 'COMPOSITE_OPERATOR_OR';
        otherwise
            queryError(sprintf(['unknown composite operator %s; expected one of and, or'], ...
                valueText(operator)));
    end
    nested = payloadGet(payload, 'constraint', []);
    if isstruct(nested), nested = num2cell(nested(:)'); end
    if ~iscell(nested) || isempty(nested)
        queryError(sprintf(['a composite constraint combines a non-empty list of constraints, ' ...
            'not %s'], valueText(nested)));
    end
    constraints = cellfun(@constraint, nested(:)', 'UniformOutput', false);
    out = struct('operator', wireOperator, 'constraint', {constraints});
end

function rejectUnknown(payload, known, what)
    unknown = setdiff(payloadKeys(payload), known);
    if ~isempty(unknown)
        queryError(sprintf('a %s has no %s', what, strjoin(sort(unknown), ', ')));
    end
end

function values = sequence(field, raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif ischar(raw) || isa(raw, 'containers.Map') || ...
            (isStringValue(raw) && isscalar(raw)) || (isstruct(raw) && isscalar(raw))
        values = {raw};
    elseif isStringValue(raw)
        values = cellstr(raw(:))';
    elseif isstruct(raw) || isnumeric(raw) || islogical(raw)
        if isscalar(raw)
            queryError(sprintf('%s is a list, not %s', field, pythonType(raw)));
        end
        values = num2cell(raw(:)');
    else
        queryError(sprintf('%s is a list, not %s', field, pythonType(raw)));
    end
end

function value = comparisonValue(raw)
    if islogical(raw) && isscalar(raw)
        if raw, value = 'true'; else, value = 'false'; end
    elseif ischar(raw) || (isStringValue(raw) && isscalar(raw))
        value = char(raw);
    elseif isa(raw, 'int64') && isscalar(raw)
        value = sprintf('%d', raw);
    elseif isnumeric(raw) && isscalar(raw)
        value = num2str(raw, 17);
        if isfinite(raw) && fix(raw) == raw && isempty(strfind(value, '.')) && ...
                isempty(strfind(lower(value), 'e'))
            value = [value '.0'];
        end
    else
        queryError(sprintf('cannot compare against %s', valueText(raw)));
    end
end

function value = payloadGet(payload, key, fallback)
    value = fallback;
    if isa(payload, 'containers.Map')
        if isKey(payload, key), value = payload(key); end
        return;
    end
    if isfield(payload, key), value = payload.(key);
    elseif strcmp(key, '@type') && isfield(payload, 'type'), value = payload.type;
    elseif strcmp(key, '@id') && isfield(payload, 'id'), value = payload.id;
    end
end

function names = payloadKeys(payload)
    if isa(payload, 'containers.Map'), names = keys(payload);
    else, names = fieldnames(payload)';
    end
end

function queryError(message)
    opensysml.internal.raise('opensysml:argument', message);
end

function text = valueText(value)
    if ischar(value), text = ['''' value ''''];
    elseif isnumeric(value) || islogical(value), text = num2str(value);
    else, text = class(value);
    end
end

function name = pythonType(value)
    if islogical(value), name = 'bool';
    elseif isnumeric(value) && isinteger(value), name = 'int';
    elseif isnumeric(value), name = 'float';
    elseif iscell(value), name = 'list';
    elseif isstruct(value) || isa(value, 'containers.Map'), name = 'dict';
    elseif ischar(value) || isStringValue(value), name = 'str';
    else, name = class(value);
    end
end

function tf = isStringValue(value)
    tf = false;
    if exist('isstring', 'builtin') || exist('isstring', 'file')
        tf = isstring(value);
    end
end
