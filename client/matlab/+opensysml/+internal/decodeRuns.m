function result = decodeRuns(model, raw)
%DECODERUNS Decode a RunRuns answer into the repeated-runs struct.

    diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    message = fieldOr(raw, 'error', '');
    if ~isempty(message)
        opensysml.internal.raiseFailure(message, fieldOr(raw, 'failureReason', ''), diagnostics);
    end
    instances = containers.Map('KeyType', 'char', 'ValueType', 'any');
    if isfield(raw, 'instances')
        [~, instances] = opensysml.internal.decodeInstances(model.connection, raw.instances);
    end
    result = struct( ...
        'rows', {opensysml.internal.decodeSweepRows(fieldOr(raw, 'rows', {}), instances, diagnostics)}, ...
        'observables', {strings(fieldOr(raw, 'observables', {}))}, ...
        'distributions', {cellfun(@distribution, cells(fieldOr(raw, 'distributions', {})), 'UniformOutput', false)}, ...
        'outputs', opensysml.internal.decodeOutputEntries(fieldOr(raw, 'outputs', {}), model.connection, instances), ...
        'verdicts', {opensysml.internal.decodeVerdicts(fieldOr(raw, 'verdicts', {}), instances, diagnostics)}, ...
        'unconcluded', fieldOr(raw, 'unconcluded', ''), 'seed', [], ...
        'instances', instances, 'diagnostics', {diagnostics}, ...
        'standing', opensysml.internal.decodeStanding(raw));
    if isfield(raw, 'seed'), result.seed = opensysml.parseUint64(raw.seed); end
end

function d = distribution(raw)
    d = struct('name', fieldOr(raw, 'name', ''), 'unit', fieldOr(raw, 'unit', ''), ...
        'count', double(opensysml.parseInt64(fieldOr(raw, 'count', '0'))), 'numeric', fieldOr(raw, 'numeric', false), ...
        'min', [], 'max', [], 'mean', [], 'deviation', fieldOr(raw, 'deviation', 0), ...
        'percentiles', {{}}, 'histogram', {{}}, 'values', {{}});
    for name = {'min', 'max', 'mean'}
        if isfield(raw, name{1}), d.(name{1}) = opensysml.internal.decodeValues(raw.(name{1})); end
    end
    for p = cells(fieldOr(raw, 'percentiles', {}))
        d.percentiles{end + 1} = struct('percentile', fieldOr(p{1}, 'percentile', 0), ...
            'value', opensysml.internal.decodeValues(fieldOr(p{1}, 'value', struct())));
    end
    for b = cells(fieldOr(raw, 'histogram', {}))
        d.histogram{end + 1} = struct('low', opensysml.internal.decodeValues(fieldOr(b{1}, 'low', struct())), ...
            'high', opensysml.internal.decodeValues(fieldOr(b{1}, 'high', struct())), ...
            'count', double(opensysml.parseInt64(fieldOr(b{1}, 'count', '0'))));
    end
    for v = cells(fieldOr(raw, 'values', {}))
        d.values{end + 1} = struct('value', opensysml.internal.decodeValues(fieldOr(v{1}, 'value', struct())), ...
            'count', double(opensysml.parseInt64(fieldOr(v{1}, 'count', '0'))));
    end
end

function out = strings(list)
    out = cellfun(@char, cells(list), 'UniformOutput', false);
end

function out = cells(list)
    if iscell(list), out = list; elseif isempty(list), out = {}; else, out = num2cell(list); end
end

function value = fieldOr(s, name, default)
    if isstruct(s) && isfield(s, name), value = s.(name); else, value = default; end
end
