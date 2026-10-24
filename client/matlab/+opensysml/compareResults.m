function result = compareResults(model, results, varargin)
%COMPARERESULTS Run a SysML v1 tool's results index and compare, as sysml -compare-results does.
%   RESULT = COMPARERESULTS(MODEL, RESULTS, ...) runs the configurations the results
%   index RESULTS (the sidecar's JSON text) names and answers a struct with source,
%   configurations, standing, engine, strength, bounds and diagnostics. Each
%   configuration has id, name, behavior, target, resultLocation, status,
%   unresolved, storedRuns, storedSnapshots, runs, completed, draws, seed (uint64
%   or []), clockStep, rows (as runSweep decodes them), instances, observables
%   (stored, feature, unit, tool, pooled, unstored, run, produced, compared,
%   difference, declared, notes), errors, notes and lines (the REPL's table).
%   Options: actions (cell of names or ids), runs, seed, draws, clockStep,
%   observe (cell of {stored, feature} pairs) and engine.
    options = opensysml.internal.nameValueOptions(struct('actions', {{}}, 'runs', 0, ...
        'seed', [], 'draws', '', 'clockStep', [], 'observe', {{}}, 'engine', ''), ...
        varargin, 'compareResults');
    model.connection.require('compare_results');
    opensysml.internal.requireEngine(model.connection, options.engine);
    request = struct('modelHash', model.hash, 'results', char(results), ...
        'actions', {cellfun(@char, cellstr(options.actions), 'UniformOutput', false)}, ...
        'runs', sprintf('%d', int64(options.runs)), 'draws', char(options.draws));
    if ~isempty(options.seed), request.seed = sprintf('%u', uint64(options.seed)); end
    if ~isempty(options.clockStep), request.clockStep = double(options.clockStep); end
    observe = {};
    for pair = cells(options.observe)
        observe{end + 1} = struct('stored', char(pair{1}{1}), 'feature', char(pair{1}{2}));
    end
    request.observe = observe;
    engine = opensysml.internal.normalizeEngine(options.engine);
    if ~isempty(engine), request.engine = engine; end
    capabilities = {'compare_results'};
    if ~isempty(engine), capabilities{end + 1} = 'engines'; end
    raw = opensysml.internal.checkError(opensysml.call(model.connection, 'CompareResults', ...
        request, capabilities), 'CompareResults');
    diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    standing = opensysml.internal.decodeStanding(raw);
    result = struct('source', fieldOr(raw, 'source', ''), 'configurations', {{}}, ...
        'standing', standing, 'engine', standing.engine, 'strength', standing.strength, ...
        'bounds', {standing.bounds}, 'diagnostics', {diagnostics});
    for c = cells(fieldOr(raw, 'configurations', {}))
        result.configurations{end + 1} = configuration(model.connection, c{1}, diagnostics);
    end
end

function conf = configuration(conn, raw, diagnostics)
    instances = containers.Map('KeyType', 'char', 'ValueType', 'any');
    if isfield(raw, 'instances')
        [~, instances] = opensysml.internal.decodeInstances(conn, raw.instances);
    end
    seed = [];
    if isfield(raw, 'seed'), seed = opensysml.parseUint64(raw.seed); end
    conf = struct('id', fieldOr(raw, 'id', ''), 'name', fieldOr(raw, 'name', ''), ...
        'behavior', fieldOr(raw, 'behavior', ''), 'target', fieldOr(raw, 'target', ''), ...
        'resultLocation', fieldOr(raw, 'resultLocation', ''), 'status', fieldOr(raw, 'status', ''), ...
        'unresolved', fieldOr(raw, 'unresolved', ''), ...
        'storedRuns', double(opensysml.parseInt64(fieldOr(raw, 'storedRuns', '0'))), ...
        'storedSnapshots', double(opensysml.parseInt64(fieldOr(raw, 'storedSnapshots', '0'))), ...
        'runs', double(opensysml.parseInt64(fieldOr(raw, 'runs', '0'))), ...
        'completed', double(opensysml.parseInt64(fieldOr(raw, 'completed', '0'))), ...
        'draws', fieldOr(raw, 'draws', ''), 'seed', seed, ...
        'clockStep', double(fieldOr(raw, 'clockStep', 0)), ...
        'rows', {opensysml.internal.decodeSweepRows(fieldOr(raw, 'rows', {}), instances, diagnostics)}, ...
        'instances', instances, 'observables', {{}}, ...
        'errors', {textCells(fieldOr(raw, 'errors', {}))}, ...
        'notes', {textCells(fieldOr(raw, 'notes', {}))}, ...
        'lines', {textCells(fieldOr(raw, 'lines', {}))});
    for o = cells(fieldOr(raw, 'observables', {}))
        conf.observables{end + 1} = observable(o{1});
    end
end

function obs = observable(raw)
    obs = struct('stored', fieldOr(raw, 'stored', ''), 'feature', fieldOr(raw, 'feature', ''), ...
        'unit', fieldOr(raw, 'unit', ''), 'tool', distribution(fieldOr(raw, 'tool', [])), ...
        'pooled', logical(fieldOr(raw, 'pooled', false)), ...
        'unstored', logical(fieldOr(raw, 'unstored', false)), ...
        'run', distribution(fieldOr(raw, 'run', [])), ...
        'produced', double(opensysml.parseInt64(fieldOr(raw, 'produced', '0'))), ...
        'compared', logical(fieldOr(raw, 'compared', false)), 'difference', [], 'declared', [], ...
        'notes', {textCells(fieldOr(raw, 'notes', {}))});
    d = fieldOr(raw, 'difference', []);
    if isstruct(d)
        obs.difference = struct('min', fieldOr(d, 'min', ''), 'mean', fieldOr(d, 'mean', ''), ...
            'p50', fieldOr(d, 'p50', ''), 'p90', fieldOr(d, 'p90', ''), 'max', fieldOr(d, 'max', ''));
    end
    s = fieldOr(raw, 'declared', []);
    if isstruct(s)
        obs.declared = struct('analysis', fieldOr(s, 'analysis', ''), 'rows', {{}}, ...
            'notes', {textCells(fieldOr(s, 'notes', {}))});
        for r = cells(fieldOr(s, 'rows', {}))
            obs.declared.rows{end + 1} = struct('statistic', fieldOr(r{1}, 'statistic', ''), ...
                'of', fieldOr(r{1}, 'of', ''), 'tool', fieldOr(r{1}, 'tool', ''), ...
                'run', fieldOr(r{1}, 'run', ''), 'difference', fieldOr(r{1}, 'difference', ''));
        end
    end
end

function dist = distribution(raw)
    if ~isstruct(raw), dist = []; return; end
    dist = struct('count', double(opensysml.parseInt64(fieldOr(raw, 'count', '0'))), ...
        'min', double(fieldOr(raw, 'min', 0)), 'mean', double(fieldOr(raw, 'mean', 0)), ...
        'p50', double(fieldOr(raw, 'p50', 0)), 'p90', double(fieldOr(raw, 'p90', 0)), ...
        'max', double(fieldOr(raw, 'max', 0)), 'deviation', double(fieldOr(raw, 'deviation', 0)));
end

function values = textCells(raw)
    values = {};
    for v = cells(raw), values{end + 1} = char(v{1}); end
end

function out = cells(list)
    % A row of cells: jsondecode gives a column (struct array or cell) that a for loop would take whole.
    if iscell(list), out = list; elseif isempty(list), out = {}; else, out = num2cell(list); end
    out = reshape(out, 1, []);
end

function value = fieldOr(s, name, default)
    if isstruct(s) && isfield(s, name), value = s.(name); else, value = default; end
end
