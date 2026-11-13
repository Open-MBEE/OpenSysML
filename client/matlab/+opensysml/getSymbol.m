function sym = getSymbol(model, id)
%GETSYMBOL The GetSymbol answer for a fully qualified name, as a plain record.
%   The name is resolved from the root namespace the way the notation resolves
%   one: through imports, an alias answered by the element it names, and a
%   member not visible from outside its namespace (private, protected) not found.

    answer = opensysml.internal.checkError(opensysml.call(model.connection, 'GetSymbol', ...
        struct('modelHash', model.hash, 'symbolId', char(id), 'resolveName', ...
        model.connection.hasCapability('symbol_name_resolution'))), 'GetSymbol');
    if isfield(answer, 'symbol')
        sym = answer.symbol;
    else
        sym = answer;
    end
end
