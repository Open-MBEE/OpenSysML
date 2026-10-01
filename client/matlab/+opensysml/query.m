function answer = query(model, varargin)
%QUERY Run an OSLC text query or a structured SysML v2 API query.

    if numel(varargin) == 1 && (ischar(varargin{1}) || isStringValue(varargin{1}))
        request = struct('modelHash', model.hash, 'oslcQuery', char(varargin{1}));
        model.connection.require('query');
        raw = opensysml.internal.checkError(opensysml.call(model.connection, ...
            'Query', request, {'query'}), 'Query');
        answer = opensysml.internal.decodeValues(raw);
        return;
    end
    if isempty(varargin)
        opensysml.internal.raise('opensysml:argument', 'a query payload or query options are required');
    end
    if numel(varargin) == 1 && ...
            (isstruct(varargin{1}) || isa(varargin{1}, 'containers.Map'))
        built = opensysml.buildQuery(varargin{1});
    elseif mod(numel(varargin), 2) == 0
        built = opensysml.buildQuery([], varargin{:});
    else
        built = opensysml.buildQuery(varargin{1}, varargin{2:end});
    end
    request = struct('modelHash', model.hash, 'query', built);
    model.connection.require('query');
    raw = opensysml.internal.checkError(opensysml.call(model.connection, ...
        'Query', request, {'query'}), 'Query');
    answer = queryElements(raw);
end

function values = queryElements(raw)
    elements = fieldOr(raw, 'elements', {});
    if isstruct(elements), elements = num2cell(elements(:)'); end
    if ~iscell(elements), elements = {}; end
    values = cell(1, numel(elements));
    for i = 1:numel(elements)
        element = elements{i};
        properties = containers.Map('KeyType', 'char', 'ValueType', 'any');
        [fields, propertyValues] = opensysml.internal.mapEntries( ...
            fieldOr(element, 'properties', struct()));
        for j = 1:numel(fields)
            properties(fields{j}) = propertyValues{j};
        end
        values{i} = struct('id', fieldOr(element, 'id', ''), ...
            'type', fieldOr(element, 'type', ''), 'properties', properties);
    end
end

function value = fieldOr(record, name, fallback)
    if isstruct(record) && isfield(record, name), value = record.(name);
    else, value = fallback;
    end
end

function tf = isStringValue(value)
    tf = false;
    if exist('isstring', 'builtin') || exist('isstring', 'file')
        tf = isstring(value);
    end
end
