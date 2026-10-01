function result = calc(model, symbolId, varargin)
%CALC Invoke a calculation or compute a calc usage's outputs.

    options = opensysml.internal.nameValueOptions(struct( ...
        'arguments', {{}}, 'engine', ''), varargin, 'calc');
    model.connection.require('verification');
    engine = opensysml.internal.normalizeEngine(options.engine);
    opensysml.internal.requireEngine(model.connection, options.engine);
    arguments = opensysml.internal.encodeArguments(options.arguments, model.connection, 'calc arguments');
    request = struct('modelHash', model.hash, 'symbolId', char(symbolId), ...
        'arguments', {arguments});
    if ~isempty(engine), request.engine = engine; end
    capabilities = {'verification', 'complex_values', 'structured_values', ...
        'measurement_refs', 'function_values', 'set_values', ...
        'tensor_values', 'metaobject_values'};
    if ~isempty(engine), capabilities{end+1} = 'engines'; end
    raw = opensysml.call(model.connection, 'EvaluateCalc', request, capabilities);
    diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    message = fieldOr(raw, 'error', '');
    if ~isempty(message)
        opensysml.internal.raiseFailure(message, fieldOr(raw, 'failureReason', ''), diagnostics);
    end
    outputs = opensysml.internal.decodeOutputEntries(fieldOr(raw, 'outputs', {}), model.connection);
    value = [];
    [outputNames, ~] = opensysml.internal.mapEntries(outputs);
    if isempty(outputNames) && isfield(raw, 'result')
        try, value = opensysml.decodeValue(raw.result);
        catch e, value = e;
        end
    end
    result = opensysml.CalcResult(value, outputs, diagnostics, ...
        opensysml.internal.decodeStanding(raw));
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
