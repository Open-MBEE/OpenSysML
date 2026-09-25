function value = conf_normalize(value, modelHash)
%CONF_NORMALIZE Wire values that cannot be compared literally:
%version -> ${version}, the model hash -> ${model_hash}, absolute paths ->
%${path}; a self_id of 0/"0" is removed. Runtime ids are left for
%conf_label_ids, which runs in one pass over the whole response.

    if isa(value, 'containers.Map')
        isInstance = (isKey(value, 'type_symbol_id') || isKey(value, 'typeSymbolId')) && ...
                     (isKey(value, 'feature_values') || isKey(value, 'featureValues')) && ...
                     isKey(value, 'id');
        isServerInfo = isKey(value, 'capabilities') && isKey(value, 'version');
        for k = {'self_id', 'selfId'}
            key = k{1};
            if isKey(value, key)
                sv = value(key);
                if (isnumeric(sv) && sv == 0) || (ischar(sv) && strcmp(sv, '0'))
                    remove(value, key);
                end
            end
        end
        keys = value.keys;
        for i = 1:numel(keys)
            key = keys{i};
            child = value(key);
            if isServerInfo && strcmp(key, 'version')
                value(key) = '${version}';
            elseif (isInstance && strcmp(key, 'id')) || ismember(key, runtime_id_keys())
                % labelled after normalization, in one pass over the response
            else
                value(key) = conf_normalize(child, modelHash);
            end
        end
    elseif iscell(value)
        for i = 1:numel(value)
            value{i} = conf_normalize(value{i}, modelHash);
        end
    elseif ischar(value)
        if ~isempty(modelHash) && strcmp(value, modelHash)
            value = '${model_hash}';
        elseif isAbsolutePath(value)
            value = '${path}';
        end
    end
end

function tf = isAbsolutePath(s)
    tf = ~isempty(s) && (s(1) == '/' || (numel(s) > 2 && s(2) == ':' && s(3) == '\'));
end

function keys = runtime_id_keys()
    keys = {'instance_id', 'instanceId', 'self_id', 'selfId'};
end
