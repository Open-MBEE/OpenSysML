function values = verifySatisfaction(model, varargin)
%VERIFYSATISFACTION Ask whether satisfaction assertions hold.

    options = opensysml.internal.nameValueOptions(struct( ...
        'symbol', '', 'engine', '', 'question', ''), varargin, 'verifySatisfaction');
    model.connection.require('verification');
    engine = opensysml.internal.normalizeEngine(options.engine);
    question = opensysml.internal.normalizeQuestion(options.question);
    opensysml.internal.requireEngine(model.connection, options.engine);
    opensysml.internal.requireQuestion(model.connection, options.question);
    request = struct('modelHash', model.hash);
    if ~isempty(options.symbol), request.symbolId = char(options.symbol); end
    if ~isempty(engine), request.engine = engine; end
    if ~isempty(question), request.question = question; end
    capabilities = {'verification'};
    if ~isempty(engine), capabilities{end+1} = 'engines'; end
    if strcmp(char(options.engine), 'explore'), capabilities{end+1} = 'schedule_explore'; end
    if ~isempty(question), capabilities{end+1} = 'verification_questions'; end
    raw = opensysml.call(model.connection, 'VerifySatisfaction', request, capabilities);
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
    values = opensysml.internal.decodeVerdicts(fieldOr(raw, 'verdicts', {}), ...
        instances, diagnostics, verifications);
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
