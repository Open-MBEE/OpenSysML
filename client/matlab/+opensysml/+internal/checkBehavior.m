function result = checkBehavior(model, actionId, stateMachineId, encodedInputs, options)
%CHECKBEHAVIOR Send a CheckBehavior request and decode its answer.

    capabilities = {'check'};
    if ~isempty(options.performer), capabilities{end + 1} = 'performer'; end
    model.connection.requireAll(capabilities);
    request = struct('modelHash', model.hash, 'actionSymbolId', actionId, ...
        'stateMachineSymbolId', stateMachineId, 'inputs', encodedInputs, ...
        'checkProperty', {strings(options.properties)}, 'checkDiverge', {strings(options.diverge)}, ...
        'checkInput', {strings(options.freeInputs)}, 'checkAssume', {strings(options.assume)}, ...
        'checkWitness', logical(options.witness), 'checkDepth', double(options.depth), ...
        'checkStates', double(options.states), 'checkUnroll', double(options.unroll), ...
        'checkTimeout', double(options.timeout));
    if ~isempty(options.performer), request.performerSymbolId = char(options.performer); end
    if ~isempty(options.engine), request.engine = char(options.engine); end
    raw = opensysml.internal.checkError(opensysml.call(model.connection, ...
        'CheckBehavior', request, capabilities), 'CheckBehavior');
    result = struct('verdict', fieldOr(raw, 'verdict', ''), 'lines', {strings(fieldOr(raw, 'lines', {}))}, ...
        'search', [], 'violations', {{}}, 'divergences', {{}}, 'finals', {{}}, ...
        'claim', fieldOr(raw, 'claim', ''), 'freeInputs', {strings(fieldOr(raw, 'freeInputs', {}))}, ...
        'assumptions', {strings(fieldOr(raw, 'assumptions', {}))}, 'witness', [], 'contrast', [], ...
        'standing', opensysml.internal.decodeStanding(raw), 'reason', fieldOr(raw, 'reason', ''));
    if isfield(raw, 'search'), result.search = search(raw.search); end
    result.violations = cellfun(@violation, cells(fieldOr(raw, 'violations', {})), 'UniformOutput', false);
    result.divergences = cellfun(@divergence, cells(fieldOr(raw, 'divergences', {})), 'UniformOutput', false);
    result.finals = cellfun(@final, cells(fieldOr(raw, 'finals', {})), 'UniformOutput', false);
    if isfield(raw, 'witness'), result.witness = witness(raw.witness); end
    if isfield(raw, 'contrast'), result.contrast = witness(raw.contrast); end
end

function s = search(raw)
    s = struct('states', fieldOr(raw, 'states', 0), 'moves', fieldOr(raw, 'moves', 0), ...
        'maxDepth', fieldOr(raw, 'maxDepth', 0), 'boundsHit', {strings(fieldOr(raw, 'boundsHit', {}))}, ...
        'horizon', fieldOr(raw, 'horizon', 0), 'horizonSet', fieldOr(raw, 'horizonSet', false), ...
        'massBounded', fieldOr(raw, 'massBounded', false), 'notes', {strings(fieldOr(raw, 'notes', {}))}, ...
        'scope', {strings(fieldOr(raw, 'scope', {}))}, 'status', fieldOr(raw, 'status', ''));
end

function w = witness(raw)
    w = struct('schedule', fieldOr(raw, 'schedule', ''), 'draws', {strings(fieldOr(raw, 'draws', {}))}, ...
        'choices', {strings(fieldOr(raw, 'choices', {}))}, 'inputs', {strings(fieldOr(raw, 'inputs', {}))}, ...
        'fileName', fieldOr(raw, 'fileName', ''), 'fileContent', fieldOr(raw, 'fileContent', ''));
end

function v = violation(raw)
    v = struct('kind', fieldOr(raw, 'kind', ''), 'property', fieldOr(raw, 'property', ''), ...
        'error', fieldOr(raw, 'error', ''), 'depth', fieldOr(raw, 'depth', 0), ...
        'probability', fieldOr(raw, 'probability', 0), 'witness', [], 'text', fieldOr(raw, 'text', ''));
    if isfield(raw, 'witness'), v.witness = witness(raw.witness); end
end

function d = divergence(raw)
    d = struct('feature', fieldOr(raw, 'feature', ''), 'values', {{}});
    for value = cells(fieldOr(raw, 'values', {}))
        entry = struct('value', fieldOr(value{1}, 'value', ''), 'witness', []);
        if isfield(value{1}, 'witness'), entry.witness = witness(value{1}.witness); end
        d.values{end + 1} = entry;
    end
end

function f = final(raw)
    f = struct('outcome', fieldOr(raw, 'outcome', ''), 'values', struct(), 'witness', []);
    if isfield(raw, 'values'), f.values = raw.values; end
    if isfield(raw, 'witness'), f.witness = witness(raw.witness); end
end

function out = strings(list)
    out = cellfun(@char, cells(list), 'UniformOutput', false);
end

function out = cells(list)
    if iscell(list), out = list(:)'; elseif isstruct(list), out = num2cell(list(:)'); ...
    elseif isempty(list), out = {}; elseif isstring(list), out = cellstr(list(:)'); else, out = {list}; end
end

function value = fieldOr(s, name, default)
    if isfield(s, name), value = s.(name); else, value = default; end
end
