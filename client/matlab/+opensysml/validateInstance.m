function result = validateInstance(model, symbolId, varargin)
%VALIDATEINSTANCE Evaluate assertions about an instance and its parts.

    options = opensysml.internal.nameValueOptions(struct('engine', ''), varargin, ...
        'validateInstance');
    model.connection.require('verification');
    engine = opensysml.internal.normalizeEngine(options.engine);
    opensysml.internal.requireEngine(model.connection, options.engine);
    request = struct('modelHash', model.hash, 'symbolId', char(symbolId));
    if ~isempty(engine), request.engine = engine; end
    capabilities = {'verification'};
    if ~isempty(engine), capabilities{end+1} = 'engines'; end
    raw = opensysml.call(model.connection, 'ValidateInstance', request, capabilities);
    diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    if ~isempty(fieldOr(raw, 'error', ''))
        opensysml.internal.raiseFailure(raw.error, fieldOr(raw, 'failureReason', ''), diagnostics);
    end
    instances = containers.Map('KeyType', 'char', 'ValueType', 'any');
    if isfield(raw, 'instances')
        [~, instances] = opensysml.internal.decodeInstances(model.connection, raw.instances);
    end
    verifications = opensysml.internal.decodeVerificationVerdicts( ...
        fieldOr(raw, 'verificationVerdicts', {}));
    verdicts = opensysml.internal.decodeVerdicts(fieldOr(raw, 'verdicts', {}), ...
        instances, diagnostics, verifications);
    summary = [];
    if isfield(raw, 'summary') && isstruct(raw.summary) && ~isempty(fieldnames(raw.summary))
        item = opensysml.internal.decodeVerdicts(raw.summary, instances, ...
            diagnostics, verifications);
        if ~isempty(item), summary = item{1}; end
    end
    result = opensysml.Validation(verdicts, summary, instances, diagnostics, ...
        verifications, fieldOr(raw, 'bounded', false));
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
