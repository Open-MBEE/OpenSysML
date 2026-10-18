function result = recordRun(model, analysisId, varargin)
%RECORDRUN Run an analysis case and record its result into the model, as sysml -record-run does.
%   RESULT = RECORDRUN(MODEL, ANALYSISID, ...) runs the case once, over sweep
%   'ranges' or as a Monte Carlo sample of 'runs', and answers a struct with run
%   (an AnalysisResult), sweep (a SweepTable) or sample (the runs; whichever was made), package, definition, records, source,
%   content, documents, skipped, reasons, diagnostics and modelHash (the recorded
%   model, which the service holds). Options: subject, arguments, namedArguments,
%   into (the records' package, Records by default), ranges, runs, seed, engine.

    options = opensysml.internal.nameValueOptions(struct('subject', '', 'arguments', {{}}, ...
        'namedArguments', struct(), 'into', '', 'ranges', struct(), 'runs', 0, 'seed', [], ...
        'engine', ''), varargin, 'recordRun');
    model.connection.require('record_run');
    request = struct('modelHash', model.hash, 'analysisSymbolId', char(analysisId), ...
        'subjectSymbolId', char(options.subject), ...
        'arguments', {opensysml.internal.encodeArguments(options.arguments, model.connection, 'recordRun arguments')}, ...
        'namedArguments', opensysml.internal.encodeNamedArguments(options.namedArguments, model.connection, 'recordRun namedArguments'), ...
        'recordInto', char(options.into), 'ranges', {ranges(model.connection, options.ranges)}, ...
        'runs', double(options.runs), 'acceptDocuments', true);
    if ~isempty(options.seed), request.seed = sprintf('%d', uint64(options.seed)); end
    if ~isempty(options.engine), request.engine = char(options.engine); end
    raw = opensysml.call(model.connection, 'RecordRun', request, {'record_run'});
    diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    message = fieldOr(raw, 'error', '');
    if ~isempty(message)
        opensysml.internal.raiseFailure(message, fieldOr(raw, 'failureReason', ''), diagnostics);
    end
    result = struct('run', [], 'sweep', [], 'sample', [], 'package', fieldOr(raw, 'package', ''), ...
        'definition', fieldOr(raw, 'definition', ''), ...
        'records', {cellfun(@char, cells(fieldOr(raw, 'records', {})), 'UniformOutput', false)}, ...
        'source', fieldOr(raw, 'source', ''), 'content', fieldOr(raw, 'content', ''), ...
        'documents', {{}}, 'skipped', double(opensysml.parseInt64(fieldOr(raw, 'skipped', '0'))), ...
        'reasons', {cellfun(@char, cells(fieldOr(raw, 'reasons', {})), 'UniformOutput', false)}, ...
        'diagnostics', {diagnostics}, 'modelHash', fieldOr(raw, 'modelHash', ''));
    if isfield(raw, 'run'), result.run = opensysml.internal.decodeAnalysisResult(model, raw.run); end
    if isfield(raw, 'sweep'), result.sweep = opensysml.internal.decodeSweepTable(model, raw.sweep); end
    if isfield(raw, 'sample'), result.sample = opensysml.internal.decodeRuns(model, raw.sample); end
    for d = cells(fieldOr(raw, 'documents', {}))
        result.documents{end + 1} = struct('name', fieldOr(d{1}, 'name', ''), 'content', fieldOr(d{1}, 'content', ''));
    end
end

function encoded = ranges(conn, raw)
    encoded = {};
    for name = fieldnames(raw)'
        bounds = raw.(name{1});
        if ~(iscell(bounds) && numel(bounds) >= 2 && numel(bounds) <= 3)
            error('opensysml:invalidArgument', 'range %s takes {from, to} or {from, to, step}', name{1});
        end
        item = struct('parameter', name{1}, 'start', opensysml.internal.encodeArguments(bounds(1), conn, 'recordRun ranges'), ...
            'end', opensysml.internal.encodeArguments(bounds(2), conn, 'recordRun ranges'));
        item.start = item.start{1}; item.end = item.end{1};
        if numel(bounds) == 3
            step = opensysml.internal.encodeArguments(bounds(3), conn, 'recordRun ranges');
            item.step = step{1};
        end
        encoded{end + 1} = item; %#ok<AGROW>
    end
end

function out = cells(list)
    % A row of cells: jsondecode gives a column (struct array or cell) that a for loop would take whole.
    if iscell(list), out = list; elseif isempty(list), out = {}; else, out = num2cell(list); end
    out = reshape(out, 1, []);
end

function value = fieldOr(s, name, default)
    if isstruct(s) && isfield(s, name), value = s.(name); else, value = default; end
end
