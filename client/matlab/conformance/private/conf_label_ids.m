function value = conf_label_ids(value, labels)
%CONF_LABEL_IDS Runtime instance ids -> @1, @2, ... in order of first
%appearance across the whole response: an Instance's id (an object with
%type_symbol_id/typeSymbolId + feature_values/featureValues + id) and every
%instance_id/instanceId and self_id/selfId.

    if nargin < 2, labels = containers.Map('KeyType', 'double', 'ValueType', 'char'); end
    if isa(value, 'containers.Map')
        isInstance = (isKey(value, 'type_symbol_id') || isKey(value, 'typeSymbolId')) && ...
                     (isKey(value, 'feature_values') || isKey(value, 'featureValues')) && ...
                     isKey(value, 'id');
        if isInstance && isKey(value, 'id')
            value('id') = label_id(value('id'), labels);
        end
        for k = {'instance_id', 'instanceId', 'self_id', 'selfId'}
            key = k{1};
            if isKey(value, key)
                value(key) = label_id(value(key), labels);
            end
        end
        keys = value.keys;
        for i = 1:numel(keys)
            key = keys{i};
            if (isInstance && strcmp(key, 'id')) || ...
               ismember(key, {'instance_id', 'instanceId', 'self_id', 'selfId'})
                continue;
            end
            value(key) = conf_label_ids(value(key), labels);
        end
    elseif iscell(value)
        for i = 1:numel(value)
            value{i} = conf_label_ids(value{i}, labels);
        end
    end
end

function labeled = label_id(id, labels)
    n = [];
    if ischar(id)
        n = str2double(id);
        if isnan(n) || n ~= floor(n), n = []; end
    elseif isnumeric(id) && isscalar(id)
        n = double(id);
    end
    if isempty(n)
        labeled = id; return;
    end
    if isKey(labels, n)
        labeled = labels(n);
    else
        labeled = sprintf('@%d', labels.Count + 1);
        labels(n) = labeled;
    end
end
