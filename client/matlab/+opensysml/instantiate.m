function inst = instantiate(model, typeId)
%INSTANTIATE An Instance record: id (int64), type_symbol_id, feature_values
%(containers.Map name -> decoded Value); a failed feature carries its error text.

    answer = opensysml.internal.checkError(opensysml.call(model.connection, 'Instantiate', ...
        struct('modelHash', model.hash, 'symbolId', char(typeId))), 'Instantiate');
    if ~isfield(answer, 'instance')
        error('opensysml:diagnostics', 'Instantiate carried no instance');
    end
    raw = answer.instance;
    values = containers.Map();
    if isfield(raw, 'featureValues')
        fv = raw.featureValues;
        names = fieldnames(fv);
        for i = 1:numel(names)
            entry = fv.(names{i});
            if isfield(entry, 'value')
                values(names{i}) = opensysml.decodeValue(entry.value);
            elseif isfield(entry, 'values')
                elems = entry.values;
                if isstruct(elems), elems = num2cell(elems); end
                if ~iscell(elems), elems = num2cell(elems); end
                values(names{i}) = cellfun(@opensysml.decodeValue, elems, 'UniformOutput', false);
            elseif isfield(entry, 'error')
                values(names{i}) = entry.error;
            end
        end
    end
    inst = struct('id', opensysml.parseInt64(raw.id), ...
                  'type_symbol_id', raw.typeSymbolId, ...
                  'feature_values', values);
end
