function result = runAnalysis(model, symbolId, varargin)
%RUNANALYSIS Run an analysis case once.
%   runAnalysis(model, id, 'toolDryRun', true) runs the case only to its first
%   external tool call and answers what the tool would be given, as
%   sysml -tool-dry-run does, in the result's toolDryRun with no outputs.

    options = opensysml.internal.nameValueOptions(struct( ...
        'subject', '', 'arguments', {{}}, 'namedArguments', struct(), ...
        'schedule', '', 'engine', '', 'toolDryRun', false), varargin, 'runAnalysis');
    if ~(islogical(options.toolDryRun) && isscalar(options.toolDryRun))
        opensysml.internal.raise('opensysml:argument', 'toolDryRun must be true or false');
    end
    schedule = char(options.schedule);
    engineOption = char(options.engine);
    rejectExplore(schedule, 'opensysml.exploreAnalysis');
    if strcmp(engineOption, 'explore')
        opensysml.internal.raise('opensysml:argument', ...
            'engine ''explore'' answers with every outcome, not one run''s result: use opensysml.exploreAnalysis');
    end
    model.connection.require('verification');
    scheduleCapabilities = opensysml.internal.runCapabilities(model.connection, schedule, '');
    opensysml.internal.requireEngine(model.connection, options.engine);
    arguments = opensysml.internal.encodeArguments(options.arguments, model.connection, ...
        'runAnalysis arguments');
    named = opensysml.internal.encodeNamedArguments(options.namedArguments, ...
        model.connection, 'runAnalysis namedArguments');
    request = struct('modelHash', model.hash, 'symbolId', char(symbolId), ...
        'arguments', {arguments}, 'namedArguments', named);
    if ~isempty(options.subject), request.subjectSymbolId = char(options.subject); end
    if ~isempty(schedule), request.schedule = schedule; end
    engine = opensysml.internal.normalizeEngine(options.engine);
    if ~isempty(engine), request.engine = engine; end
    if options.toolDryRun, request.toolDryRun = true; end
    capabilities = [{'verification', 'complex_values', 'structured_values', ...
        'measurement_refs', 'set_values', 'tensor_values', 'metaobject_values'}, ...
        scheduleCapabilities];
    if ~isempty(engine), capabilities{end+1} = 'engines'; end
    raw = opensysml.call(model.connection, 'RunAnalysis', request, capabilities);
    diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    message = fieldOr(raw, 'error', '');
    if ~isempty(message) && ~hasPartialResult(raw)
        opensysml.internal.raiseFailure(message, fieldOr(raw, 'failureReason', ''), diagnostics);
    end
    result = opensysml.internal.decodeAnalysisResult(model, raw);
    if ~isempty(message)
        opensysml.internal.raise('opensysml:diagnostics:analysisRun', message, ...
            diagnostics, struct('result', result, ...
                'failure', fieldOr(raw, 'failureReason', ''), ...
                'diagnostics', {diagnostics}));
    end
end

function rejectExplore(schedule, method)
    if strcmp(schedule, 'explore') || strncmp(schedule, 'explore:', 8)
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('schedule ''%s'' answers with every outcome, not one run''s result: use %s', ...
            schedule, method));
    end
end

function tf = hasPartialResult(raw)
    names = {'outputs', 'verdicts', 'evaluations', 'instances'};
    tf = false;
    for i = 1:numel(names)
        if isfield(raw, names{i}) && ~isempty(raw.(names{i}))
            tf = true;
            return;
        end
    end
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
