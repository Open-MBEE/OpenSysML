function result = runDocumentQuery(model, queryId, varargin)
%RUNDOCUMENTQUERY Run one of the model's named document queries.

    bindings = [];
    for i = 1:2:numel(varargin)
        if i == numel(varargin)
            opensysml.internal.raise('opensysml:argument', ...
                'runDocumentQuery options must be name-value pairs');
        end
        if strcmp(varargin{i}, 'bindings'), bindings = varargin{i+1};
        else
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('runDocumentQuery has no option ''%s''', varargin{i}));
        end
    end
    model.connection.require('document_query');
    wire = opensysml.buildDocumentBindings(bindings);
    if any(cellfun(@bindingHoldsBigInt, wire))
        model.connection.require('big_int_values');
    end
    request = struct('modelHash', model.hash, 'queryId', char(queryId), ...
        'bindings', {wire});
    answer = opensysml.call(model.connection, 'RunDocumentQuery', request, ...
        {'document_query'});
    result = opensysml.internal.decodeDocumentResult(answer);
end

function wide = bindingHoldsBigInt(binding)
%BINDINGHOLDSBIGINT Whether a wire binding sends an Integer beyond int64.
    wide = any(cellfun(@(value) isfield(value, 'bigIntValue') || ...
        (isfield(value, 'quantity') && isfield(value.quantity, 'bigIntMagnitude')), ...
        binding.values));
end
