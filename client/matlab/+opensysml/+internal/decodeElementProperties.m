function answer = decodeElementProperties(raw)
%DECODEELEMENTPROPERTIES Decode a GetElementPropertiesResponse answered without error.
%   Each value is an element record (struct with id, qualifiedName, metaclass), a char,
%   a logical, an int64, a double, or the decimal text of a number beyond int64 and
%   double. A field the wire left at its default (false, empty) decodes as that default.

    answer = struct();
    answer.element = elementRecord(fieldOr(raw, 'element', struct()));
    properties = fieldOr(raw, 'properties', {});
    if isstruct(properties), properties = num2cell(properties(:)'); end
    if ~iscell(properties), properties = {}; end
    answer.properties = cell(1, numel(properties));
    answer.byName = containers.Map('KeyType', 'char', 'ValueType', 'any');
    for i = 1:numel(properties)
        p = properties{i};
        record = struct();
        record.name = char(fieldOr(p, 'name', ''));
        record.definingClass = char(fieldOr(p, 'definingClass', ''));
        record.many = logical(fieldOr(p, 'many', false));
        record.derived = logical(fieldOr(p, 'derived', false));
        record.unsupported = char(fieldOr(p, 'unsupported', ''));
        values = fieldOr(p, 'values', {});
        if isstruct(values), values = num2cell(values(:)'); end
        if ~iscell(values), values = {}; end
        record.values = cell(1, numel(values));
        for j = 1:numel(values)
            record.values{j} = propertyValue(values{j});
        end
        answer.properties{i} = record;
        answer.byName(record.name) = record;
    end
end

function record = elementRecord(raw)
    record = struct('id', char(fieldOr(raw, 'id', '')), ...
        'qualifiedName', char(fieldOr(raw, 'qualifiedName', '')), ...
        'metaclass', char(fieldOr(raw, 'metaclass', '')));
end

function value = propertyValue(raw)
    if isfield(raw, 'element')
        value = elementRecord(raw.element);
    elseif isfield(raw, 'stringValue')
        value = char(raw.stringValue);
    elseif isfield(raw, 'boolValue')
        value = logical(raw.boolValue);
    elseif isfield(raw, 'intValue')
        value = opensysml.parseInt64(valueText(raw.intValue));
    elseif isfield(raw, 'realValue')
        value = double(raw.realValue);
    elseif isfield(raw, 'number')
        value = char(raw.number);
    else
        opensysml.internal.raise('opensysml:transport', ...
            'an ElementPropertyValue carries no value', {}, struct());
    end
end

function text = valueText(value)
    if ischar(value)
        text = value;
    else
        text = sprintf('%d', value);
    end
end

function value = fieldOr(record, name, default)
    if isstruct(record) && isfield(record, name) && ~isempty(record.(name))
        value = record.(name);
    else
        value = default;
    end
end
