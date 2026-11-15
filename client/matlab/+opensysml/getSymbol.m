function sym = getSymbol(model, id, varargin)
%GETSYMBOL The GetSymbol answer for a fully qualified name, as a plain record.
%   The name is resolved from the root namespace the way the notation resolves
%   one: through imports, an alias answered by the element it names, and a
%   member not visible from outside its namespace (private, protected) not found.

    options = opensysml.internal.nameValueOptions(struct('lookup', 'either'), varargin, 'getSymbol');
    request = opensysml.internal.lookupKind(model.connection, ...
        struct('modelHash', model.hash, 'symbolId', char(id)), options.lookup);
    request.resolveName = ~strcmp(options.lookup, 'element_id') && ...
        model.connection.hasCapability('symbol_name_resolution');
    answer = opensysml.internal.checkError(opensysml.call(model.connection, 'GetSymbol', ...
        request), 'GetSymbol');
    if isfield(answer, 'symbol')
        sym = answer.symbol;
    else
        sym = answer;
    end
end
