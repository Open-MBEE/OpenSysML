function result = importData(model, imports, varargin)
%IMPORTDATA Set the feature values tabular or JSON data assigns, as sysml -import does.
%   RESULT = IMPORTDATA(MODEL, IMPORTS, 'dryRun', true, 'acceptDocuments', true) imports
%   each of IMPORTS (a struct or cell of structs with fields name, content and
%   optionally format and map) in order and answers a struct with files (one per
%   import: name, values, elements and assignments with element, feature, value,
%   redefines, where and description), documents (name and content, multi-document
%   models), content (one-document models), modelHash (the imported model's hash;
%   '' on a dry run) and diagnostics. An import the model refuses raises an error
%   and leaves the model unchanged.
    options = opensysml.internal.nameValueOptions(struct('importAs', 'values', 'dryRun', false, ...
        'acceptDocuments', false), varargin, 'importData');
    model.connection.require('import_data');
    encoded = {};
    for item = cells(imports)
        entry = item{1};
        encoded{end + 1} = struct('name', char(fieldOr(entry, 'name', '')), ...
            'content', char(fieldOr(entry, 'content', '')), ...
            'importFormat', char(fieldOr(entry, 'format', '')), ...
            'importMap', char(fieldOr(entry, 'map', '')));
    end
    request = struct('modelHash', model.hash, 'imports', {encoded}, 'importAs', char(options.importAs), ...
        'importDryRun', logical(options.dryRun), 'acceptDocuments', logical(options.acceptDocuments));
    raw = opensysml.internal.checkError(opensysml.call(model.connection, 'ImportData', ...
        request, {'import_data'}), 'ImportData');
    result = struct('files', {{}}, 'documents', {{}}, 'content', fieldOr(raw, 'content', ''), ...
        'modelHash', fieldOr(raw, 'modelHash', ''), ...
        'diagnostics', {opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}))});
    for f = cells(fieldOr(raw, 'files', {}))
        file = struct('name', fieldOr(f{1}, 'name', ''), ...
            'values', double(opensysml.parseInt64(fieldOr(f{1}, 'values', '0'))), ...
            'elements', double(opensysml.parseInt64(fieldOr(f{1}, 'elements', '0'))), ...
            'assignments', {{}});
        for a = cells(fieldOr(f{1}, 'assignments', {}))
            file.assignments{end + 1} = struct('element', fieldOr(a{1}, 'element', ''), ...
                'feature', fieldOr(a{1}, 'feature', ''), 'value', fieldOr(a{1}, 'value', ''), ...
                'redefines', fieldOr(a{1}, 'redefines', ''), 'where', fieldOr(a{1}, 'where', ''), ...
                'description', fieldOr(a{1}, 'description', ''));
        end
        result.files{end + 1} = file;
    end
    for d = cells(fieldOr(raw, 'documents', {}))
        result.documents{end + 1} = struct('name', fieldOr(d{1}, 'name', ''), ...
            'content', fieldOr(d{1}, 'content', ''));
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
