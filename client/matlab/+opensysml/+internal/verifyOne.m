function result = verifyOne(model, symbolId, method, kind, varargin)
%VERIFYONE Request one constraint or requirement verdict.

    label = ['verify' upper(kind(1)) kind(2:end)];
    options = opensysml.internal.nameValueOptions(struct( ...
        'subject', '', 'engine', '', 'question', '', ...
        'arguments', {{}}, 'namedArguments', struct()), varargin, label);
    model.connection.require('verification');
    engine = opensysml.internal.normalizeEngine(options.engine);
    question = opensysml.internal.normalizeQuestion(options.question);
    opensysml.internal.requireEngine(model.connection, options.engine);
    opensysml.internal.requireQuestion(model.connection, options.question);
    request = struct('modelHash', model.hash, 'symbolId', char(symbolId));
    if ~isempty(options.subject), request.subjectSymbolId = char(options.subject); end
    if ~isempty(engine), request.engine = engine; end
    if ~isempty(question), request.question = question; end
    methodName = ['Verify' kind];
    capabilities = {'verification'};
    if ~isempty(engine), capabilities{end+1} = 'engines'; end
    if strcmp(char(options.engine), 'explore'), capabilities{end+1} = 'schedule_explore'; end
    if ~isempty(question), capabilities{end+1} = 'verification_questions'; end
    named = opensysml.internal.encodeNamedArguments( ...
        options.namedArguments, model.connection, [label ' namedArguments']);
    if ~isempty(options.arguments) || hasEntries(named)
        model.connection.require('verification_arguments');
        request.arguments = opensysml.internal.encodeArguments(options.arguments, ...
            model.connection, [label ' arguments']);
        request.namedArguments = named;
        capabilities = [capabilities, {'verification_arguments', 'complex_values', ...
            'structured_values', 'measurement_refs', 'set_values', 'tensor_values', ...
            'metaobject_values'}];
    end
    raw = opensysml.internal.checkError(opensysml.call(model.connection, ...
        methodName, request, capabilities), methodName);
    diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    instances = decodeInstanceMap(model, raw);
    verifications = opensysml.internal.decodeVerificationVerdicts( ...
        fieldOr(raw, 'verificationVerdicts', {}));
    if ~isfield(raw, 'verdict')
        opensysml.internal.raise('opensysml:diagnostics:execution', ...
            sprintf('%s carried no verdict', methodName), diagnostics);
    end
    values = opensysml.internal.decodeVerdicts(raw.verdict, instances, diagnostics, verifications);
    if isempty(values)
        opensysml.internal.raise('opensysml:diagnostics:execution', ...
            sprintf('%s carried no verdict', methodName), diagnostics);
    end
    result = values{1};
end

function instances = decodeInstanceMap(model, raw)
    instances = containers.Map('KeyType', 'char', 'ValueType', 'any');
    if isfield(raw, 'instances')
        [~, instances] = opensysml.internal.decodeInstances(model.connection, raw.instances);
    end
end

function bound = hasEntries(named)
    if isa(named, 'containers.Map'), bound = named.Count > 0;
    else, bound = ~isempty(fieldnames(named));
    end
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
