function result = decodeSweepTable(model, raw)
%DECODESWEEPTABLE Build a SweepTable from a RunSweepResponse.
    diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    instances = containers.Map('KeyType', 'char', 'ValueType', 'any');
    if isfield(raw, 'instances')
        [~, instances] = opensysml.internal.decodeInstances(model.connection, raw.instances);
    end
    rows = opensysml.internal.decodeSweepRows(fieldOr(raw, 'rows', {}), instances, diagnostics);
    parameters = textCells(fieldOr(raw, 'parameters', {}));
    result = opensysml.SweepTable(rows, parameters, ...
        fieldOr(raw, 'sampled', false), opensysml.parseUint64(fieldOr(raw, 'seed', '0')), ...
        instances, diagnostics, opensysml.internal.decodeStanding(raw));
end

function values = textCells(raw)
    if isempty(raw), values = {};
    elseif ischar(raw), values = {raw};
    elseif iscell(raw), values = cellfun(@char, raw(:)', 'UniformOutput', false);
    elseif isstruct(raw), values = cellfun(@char, num2cell(raw(:)'), 'UniformOutput', false);
    else, values = cellstr(raw);
    end
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
