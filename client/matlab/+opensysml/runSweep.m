function result = runSweep(model, symbolId, ranges, varargin)
%RUNSWEEP Run an analysis case or calculation over parameter ranges.

    options = opensysml.internal.nameValueOptions(struct( ...
        'subject', '', 'arguments', {{}}, 'namedArguments', struct(), ...
        'samples', 0, 'seed', 0, 'engine', ''), varargin, 'runSweep');
    model.connection.require('verification');
    opensysml.internal.requireEngine(model.connection, options.engine);
    arguments = opensysml.internal.encodeArguments(options.arguments, model.connection, ...
        'runSweep arguments');
    named = opensysml.internal.encodeNamedArguments(options.namedArguments, ...
        model.connection, 'runSweep namedArguments');
    encodedRanges = sweepRanges(model.connection, ranges);
    request = struct('modelHash', model.hash, 'symbolId', char(symbolId), ...
        'arguments', {arguments}, 'namedArguments', named, 'ranges', {encodedRanges}, ...
        'samples', int64String(options.samples, 'samples'), 'seed', uint64String(options.seed));
    if ~isempty(options.subject), request.subjectSymbolId = char(options.subject); end
    engine = opensysml.internal.normalizeEngine(options.engine);
    if ~isempty(engine), request.engine = engine; end
    capabilities = {'verification', 'complex_values', 'structured_values'};
    if ~isempty(engine), capabilities{end+1} = 'engines'; end
    raw = opensysml.call(model.connection, 'RunSweep', request, capabilities);
    diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    message = fieldOr(raw, 'error', '');
    if ~isempty(message)
        opensysml.internal.raiseFailure(message, fieldOr(raw, 'failureReason', ''), diagnostics);
    end
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

function ranges = sweepRanges(conn, raw)
    ranges = {};
    if isempty(raw), return; end
    if isa(raw, 'containers.Map')
        names = raw.keys;
        for i = 1:numel(names)
            ranges{end+1} = sweepRange(conn, names{i}, raw(names{i}));
        end
    elseif isstruct(raw) && isscalar(raw)
        names = fieldnames(raw);
        for i = 1:numel(names)
            ranges{end+1} = sweepRange(conn, names{i}, raw.(names{i}));
        end
    else
        opensysml.internal.raise('opensysml:argument', ...
            'runSweep ranges must be a scalar struct or containers.Map');
    end
end

function item = sweepRange(conn, name, bounds)
    if iscell(bounds), values = bounds(:)';
    elseif isnumeric(bounds) && isvector(bounds), values = num2cell(bounds(:)');
    elseif isstruct(bounds) && isscalar(bounds) && ...
            isfield(bounds, 'from') && isfield(bounds, 'to')
        values = {bounds.from, bounds.to};
        if isfield(bounds, 'step'), values{end+1} = bounds.step; end
    else
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('range of %s takes (from, to) or (from, to, step), not 0 value(s)', name));
    end
    if ~any(numel(values) == [2 3])
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('range of %s takes (from, to) or (from, to, step), not %d value(s)', ...
            name, numel(values)));
    end
    item = struct('parameter', name, ...
        'start', opensysml.encodeValue(values{1}, conn), ...
        'end', opensysml.encodeValue(values{2}, conn));
    if numel(values) == 3, item.step = opensysml.encodeValue(values{3}, conn); end
end

function text = int64String(value, name)
    if isa(value, 'int64') && isscalar(value) && value >= 0
        text = sprintf('%d', value);
        return;
    end
    if isa(value, 'uint64') && isscalar(value) && value <= uint64(intmax('int64'))
        text = sprintf('%d', int64(value));
        return;
    end
    if ~(isnumeric(value) && ~islogical(value) && isscalar(value) && ...
            isfinite(double(value)) && double(value) >= 0 && ...
            double(value) == fix(double(value)) && double(value) <= flintmax)
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be a non-negative int64', name));
    end
    text = sprintf('%d', int64(value));
end

function text = uint64String(value)
    if isa(value, 'uint64') && isscalar(value)
        text = sprintf('%u', value);
        return;
    end
    if isa(value, 'int64') && isscalar(value) && value >= 0
        text = sprintf('%u', uint64(value));
        return;
    end
    if ~(isnumeric(value) && ~islogical(value) && isscalar(value) && isfinite(double(value)) && ...
            double(value) >= 0 && double(value) == fix(double(value)))
        opensysml.internal.raise('opensysml:argument', 'seed must be a non-negative uint64');
    end
    if double(value) <= flintmax
        text = sprintf('%u', uint64(value));
    else
        opensysml.internal.raise('opensysml:argument', ...
            'seed values above flintmax must be supplied as uint64');
    end
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
