function result = runDocumentQuery(model, queryId, varargin)
%RUNDOCUMENTQUERY Run one of the model's named document queries, or a query
%   expression written in place of a name: runDocumentQuery(model, '',
%   'inlineQuery', 'Project(Descendants(Named("Lander")), ("name"))'), which
%   needs the inline_document_query capability.

    bindings = [];
    inlineQuery = '';
    for i = 1:2:numel(varargin)
        if i == numel(varargin)
            opensysml.internal.raise('opensysml:argument', ...
                'runDocumentQuery options must be name-value pairs');
        end
        if strcmp(varargin{i}, 'bindings'), bindings = varargin{i+1};
        elseif strcmp(varargin{i}, 'inlineQuery'), inlineQuery = char(varargin{i+1});
        else
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('runDocumentQuery has no option ''%s''', varargin{i}));
        end
    end
    if isempty(queryId) == isempty(inlineQuery)
        opensysml.internal.raise('opensysml:argument', ...
            'runDocumentQuery takes a queryId or an inlineQuery, not both and not neither');
    end
    model.connection.require('document_query');
    if ~isempty(inlineQuery)
        model.connection.require('inline_document_query');
    end
    wire = opensysml.buildDocumentBindings(bindings);
    if ~model.connection.hasCapability('rational_values')
        wire = cellfun(@opensysml.internal.bindingRationalsAsReals, wire, 'UniformOutput', false);
    end
    if any(cellfun(@bindingHoldsBigInt, wire))
        model.connection.require('big_int_values');
    end
    if any(cellfun(@bindingHoldsRational, wire))
        model.connection.require('rational_values');
    end
    request = struct('modelHash', model.hash, 'bindings', {wire});
    if isempty(inlineQuery)
        request.queryId = char(queryId);
    else
        request.inlineQuery = inlineQuery;
    end
    answer = opensysml.call(model.connection, 'RunDocumentQuery', request, ...
        {'document_query', 'inline_document_query'});
    result = opensysml.internal.decodeDocumentResult(answer);
end

function exact = bindingHoldsRational(binding)
%BINDINGHOLDSRATIONAL Whether a wire binding sends an exact Rational.
    exact = any(cellfun(@(value) isfield(value, 'rationalValue') || ...
        (isfield(value, 'quantity') && isfield(value.quantity, 'rationalMagnitude')), ...
        binding.values));
end

function wide = bindingHoldsBigInt(binding)
%BINDINGHOLDSBIGINT Whether a wire binding sends an Integer beyond int64.
    wide = any(cellfun(@(value) isfield(value, 'bigIntValue') || ...
        (isfield(value, 'quantity') && isfield(value.quantity, 'bigIntMagnitude')), ...
        binding.values));
end
