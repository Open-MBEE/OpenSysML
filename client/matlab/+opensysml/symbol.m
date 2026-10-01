function sym = symbol(model, id)
%SYMBOL The GetSymbol answer for a fully qualified name, as a plain record.

    answer = opensysml.internal.checkError(opensysml.call(model.connection, 'GetSymbol', ...
        struct('modelHash', model.hash, 'symbolId', char(id))), 'GetSymbol');
    if isfield(answer, 'symbol')
        sym = answer.symbol;
    else
        sym = answer;
    end
end
