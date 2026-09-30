function answer = query(model, queryText)
%QUERY An OSLC query text over the model's elements; rows with every Value
%inside decoded.

    answer = opensysml.internal.checkError(opensysml.call(model.connection, 'Query', ...
        struct('modelHash', model.hash, 'oslcQuery', char(queryText))), 'Query');
    answer = opensysml.internal.decodeValues(answer);
end
