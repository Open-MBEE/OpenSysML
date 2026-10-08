function rows = decodeSweepRows(raw, instances, diagnostics)
%DECODESWEEPROWS Decode the repeated rows in a sweep result.

    raw = toCells(raw);
    rows = cell(1, numel(raw));
    for i = 1:numel(raw)
        item = raw{i};
        rowDiagnostics = diagnostics;
        outputs = opensysml.internal.decodeOutputEntries(fieldOr(item, 'outputs', {}), [], instances);
        inputs = opensysml.internal.decodeOutputEntries(fieldOr(item, 'inputs', {}), [], instances);
        verdicts = opensysml.internal.decodeVerdicts(fieldOr(item, 'verdicts', {}), ...
            instances, rowDiagnostics, {});
        evaluations = opensysml.internal.decodeEvaluations(fieldOr(item, 'evaluations', {}), instances);
        elapsed = opensysml.parseInt64(fieldOr(item, 'elapsedMicros', '0'));
        message = fieldOr(item, 'error', '');
        selected = {};
        for j = 1:numel(evaluations)
            if evaluations{j}.selected, selected{end+1} = evaluations{j}; end
        end
        rows{i} = struct('inputs', inputs, 'outputs', outputs, ...
            'verdicts', {verdicts}, 'seconds', double(elapsed) / 1e6, ...
            'error', message, 'evaluations', {evaluations}, ...
            'selected', {selected}, 'failed', ~isempty(message));
    end
end

function values = toCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif isstruct(raw), values = num2cell(raw(:)');
    else, values = {};
    end
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
