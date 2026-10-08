function outcome = decodeOutcome(raw, conn)
%DECODEOUTCOME Decode one exploration outcome.

    outputs = opensysml.internal.decodeNamedValues(fieldOr(raw, 'outputs', struct()), conn, false);
    outcome = struct();
    outcome.outputs = outputs;
    outcome.finalState = fieldOr(raw, 'finalState', '');
    outcome.statesVisited = cellstrValue(fieldOr(raw, 'statesVisited', {}));
    outcome.error = fieldOr(raw, 'error', '');
    outcome.linearizations = fieldOr(raw, 'linearizations', 0);
    outcome.probability = fieldOr(raw, 'probability', 0);
    outcome.witness = cellstrValue(fieldOr(raw, 'witness', {}));
    outcome.diagnostics = decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    outcome.failed = ~isempty(outcome.error);
end

function diagnostics = decodeDiagnostics(raw)
    if isempty(raw), diagnostics = {};
    elseif isstruct(raw), diagnostics = arrayfun(@opensysml.internal.decodeDiagnostic, ...
            raw, 'UniformOutput', false);
    elseif iscell(raw), diagnostics = cellfun(@opensysml.internal.decodeDiagnostic, ...
            raw, 'UniformOutput', false);
    else, diagnostics = {};
    end
end

function values = cellstrValue(raw)
    if isempty(raw), values = {};
    elseif ischar(raw), values = {raw};
    elseif iscell(raw), values = cellfun(@char, raw(:)', 'UniformOutput', false);
    elseif isString(raw), values = cellstr(raw(:))';
    else, values = cellstr(raw);
    end
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isfield(raw, name), value = raw.(name); end
end

function tf = isString(value)
    tf = false;
    if exist('isstring', 'builtin') || exist('isstring', 'file')
        tf = isstring(value);
    end
end
