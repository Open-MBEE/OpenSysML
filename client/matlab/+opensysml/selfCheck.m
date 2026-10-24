function result = selfCheck(model, varargin)
%SELFCHECK Apply every SysMLValidation rule constraint to the model, as sysml -self-check does.
%   RESULT = SELFCHECK(MODEL, 'packages', {...}) answers a struct with status
%   ('holds' or 'fails'), findings (status, subject, location, constraint,
%   element, message, line), elements, checks, violations, unevaluated,
%   evaluationErrors, applications (constraint and count), summary and
%   diagnostics. Option: packages, further rule packages to apply.

    options = opensysml.internal.nameValueOptions(struct('packages', {{}}), varargin, 'selfCheck');
    model.connection.require('self_check');
    request = struct('modelHash', model.hash, ...
        'selfCheckPackage', {cellfun(@char, cellstr(options.packages), 'UniformOutput', false)});
    raw = opensysml.internal.checkError(opensysml.call(model.connection, 'SelfCheck', request, ...
        {'self_check'}), 'SelfCheck');
    result = struct('status', fieldOr(raw, 'status', ''), 'findings', {{}}, ...
        'elements', double(opensysml.parseInt64(fieldOr(raw, 'elements', '0'))), 'checks', double(opensysml.parseInt64(fieldOr(raw, 'checks', '0'))), ...
        'violations', double(opensysml.parseInt64(fieldOr(raw, 'violations', '0'))), 'unevaluated', double(opensysml.parseInt64(fieldOr(raw, 'unevaluated', '0'))), ...
        'evaluationErrors', double(opensysml.parseInt64(fieldOr(raw, 'evaluationErrors', '0'))), 'applications', {{}}, ...
        'summary', fieldOr(raw, 'summary', ''), ...
        'diagnostics', {opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}))});
    for f = cells(fieldOr(raw, 'findings', {}))
        result.findings{end + 1} = struct('status', fieldOr(f{1}, 'status', ''), ...
            'subject', fieldOr(f{1}, 'subject', ''), 'location', fieldOr(f{1}, 'location', ''), ...
            'constraint', fieldOr(f{1}, 'constraint', ''), 'element', fieldOr(f{1}, 'element', ''), ...
            'message', fieldOr(f{1}, 'message', ''), 'line', fieldOr(f{1}, 'line', ''));
    end
    for a = cells(fieldOr(raw, 'applications', {}))
        result.applications{end + 1} = struct('constraint', fieldOr(a{1}, 'constraint', ''), ...
            'count', double(opensysml.parseInt64(fieldOr(a{1}, 'count', '0'))));
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
