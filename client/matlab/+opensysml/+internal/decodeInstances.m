function [values, instances] = decodeInstances(conn, raw)
%DECODEINSTANCES Decode an instance graph returned by the service.

    values = {};
    instances = containers.Map('KeyType', 'char', 'ValueType', 'any');
    if nargin < 2 || isempty(raw), return; end
    if isstruct(raw), raw = num2cell(raw(:)'); end
    if ~iscell(raw), raw = {raw}; end
    conn.require('feature_values');
    rawById = containers.Map('KeyType', 'char', 'ValueType', 'any');
    keysInOrder = cell(1, numel(raw));
    for i = 1:numel(raw)
        id = opensysml.parseInt64(raw{i}.id);
        key = sprintf('%d', id);
        rawById(key) = raw{i};
        keysInOrder{i} = key;
    end
    decoding = containers.Map('KeyType', 'char', 'ValueType', 'logical');
    values = cell(1, numel(raw));
    for i = 1:numel(raw)
        values{i} = decodeOne(keysInOrder{i});
    end

    function value = decodeOne(key)
        if isKey(instances, key)
            value = instances(key);
            return;
        end
        if ~isKey(rawById, key) || (isKey(decoding, key) && decoding(key))
            value = opensysml.parseInt64(key);
            return;
        end
        decoding(key) = true;
        item = rawById(key);
        features = containers.Map('KeyType', 'char', 'ValueType', 'any');
        if isfield(item, 'featureValues')
            names = fieldnames(item.featureValues);
            for j = 1:numel(names)
                features(names{j}) = featureValue( ...
                    names{j}, item.featureValues.(names{j}), @resolveOne);
            end
        end
        id = opensysml.parseInt64(item.id);
        typeId = '';
        if isfield(item, 'typeSymbolId'), typeId = item.typeSymbolId; end
        value = struct('id', id, 'type_symbol_id', typeId, 'feature_values', features);
        instances(key) = value;
        decoding(key) = false;
    end

    function value = resolveOne(id)
        key = sprintf('%d', int64(id));
        if ~isKey(rawById, key) || (isKey(decoding, key) && decoding(key))
            value = int64(id);
        elseif isKey(instances, key)
            value = instances(key);
        else
            value = decodeOne(key);
        end
    end
end

function value = featureValue(name, raw, resolveInstance)
    if isfield(raw, 'error') && ~isempty(raw.error)
        value = featureError(sprintf('feature %s failed to evaluate: %s', name, raw.error));
    elseif isfield(raw, 'value')
        try
            value = opensysml.decodeValue(raw.value, resolveInstance);
        catch e
            value = e;
        end
    elseif isfield(raw, 'values') && ~isempty(raw.values)
        items = toCells(raw.values);
        value = cell(1, numel(items));
        for i = 1:numel(items)
            try
                value{i} = opensysml.decodeValue(items{i}, resolveInstance);
            catch e
                value{i} = e;
            end
        end
    elseif isfield(raw, 'materialized') && raw.materialized
        value = {};
    else
        value = featureError(sprintf('feature %s value is not materialized', name));
    end
end

function value = featureError(message)
    identifier = 'opensysml:featureValue';
    if exist('OCTAVE_VERSION', 'builtin') || exist('OCTAVE_VERSION', 'var')
        value = struct('identifier', identifier, 'message', message);
    else
        value = MException(identifier, '%s', message);
    end
end

function values = toCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif isstruct(raw), values = num2cell(raw(:)');
    else, values = num2cell(raw(:)');
    end
end
