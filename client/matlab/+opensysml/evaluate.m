function val = evaluate(model, expression, varargin)
%EVALUATE An expression over the model, decoded per the Value rule.
%   'context' and 'subject' take fully qualified names.

    request.modelHash = model.hash;
    request.expression = char(expression);
    for i = 1:2:numel(varargin)
        switch varargin{i}
            case 'context', request.contextSymbolId = char(varargin{i+1});
            case 'subject', request.subjectSymbolId = char(varargin{i+1});
        end
    end
    answer = opensysml.internal.checkError(opensysml.call(model.connection, 'Evaluate', request), 'Evaluate');
    if isfield(answer, 'result')
        val = opensysml.decodeValue(answer.result);
    else
        val = [];
    end
end
